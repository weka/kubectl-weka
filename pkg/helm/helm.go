package helm

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/targzutils"
	"github.com/weka/kubectl-weka/pkg/utils"
	yamlv3 "gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"os"
	"path/filepath"
	"strings"
)

const helmChartExtension = ".tgz"

// GetVersion extracts the version from a Helm chart
// The chart can be:
// - Local directory path (e.g., "./charts/operator")
// - Local .tgz file (e.g., "operator-1.2.0.tgz")
// - HTTP/HTTPS URL (e.g., "https://github.com/.../operator-1.2.0.tgz")
// - OCI URL (e.g., "oci://quay.io/weka.io/helm/weka-operator")
//
// Returns the appVersion if available, otherwise falls back to chart version
func GetVersion(ch *chart.Chart) (string, error) {
	if ch == nil || ch.Metadata == nil {
		return "", fmt.Errorf("chart metadata is empty for %s", ch.Name())
	}

	// Prefer appVersion (actual software version) over chart version
	if ch.Metadata.AppVersion != "" {
		return ch.Metadata.AppVersion, nil
	}

	if ch.Metadata.Version != "" {
		return ch.Metadata.Version, nil
	}

	return "", fmt.Errorf("no version or appVersion found in chart %s", ch.Name())
}

// LoadChart loads a Helm chart from local or remote sources
// Uses Helm SDK for all operations (local dirs, .tgz files, HTTP, OCI)
// Returns the parsed *chart.Chart with metadata
func LoadChart(chartRef string) (*chart.Chart, error) {
	// Local files: directories and .tgz archives
	if isLocalPath(chartRef) {
		return loader.Load(chartRef)
	}

	// Remote sources: HTTP/HTTPS and OCI registries
	return loadRemoteChartWithHelm(chartRef)
}

// GetLocalPackageFromPath returns a path to local Helm archive (.tgz) for a given chart reference
// can accept URL, local path to Helm chart (dir or .tgz) and returns path to .tgz archive
// if URL is provided, the package is downloaded into local temporary directory
// if local directory is provided, a Helm chart is archived into tgz in temporary directory
// if the path is a local archive, just returns it as is
// this sometimes could be preferred to using Archive, since LoadChart + Archive may break things, especially strip comments
func GetLocalPackageFromPath(ctx context.Context, chartPath string) (string, error) {
	if isLocalPath(chartPath) {
		// Check if it's an existing tar.gz archive (not a directory)
		info, err := os.Stat(chartPath)
		if err == nil && !info.IsDir() && (strings.HasSuffix(chartPath, ".tgz") || strings.HasSuffix(chartPath, ".tar.gz")) {
			// this is already an archive, just return it's path
			return chartPath, nil
		}
		if err == nil && info.IsDir() {
			dir, err := utils.MkdirTemp("", "helm-chart-*")
			if err != nil {
				return "", err
			}
			archiveBaseName := filepath.Base(chartPath)
			newPath := filepath.Join(dir, archiveBaseName)
			err = targzutils.PackDirectory(ctx, chartPath, newPath)
			if err != nil {
				return "", err
			}
			return newPath, nil
		}
		return "", err
	}
	if !(strings.HasSuffix(chartPath, ".tgz") || strings.HasSuffix(chartPath, ".tar.gz")) {
		return "", fmt.Errorf("%s is not a helm chart archive", chartPath) // a case where this is URL but not archive
	}
	return downloadHelmChart(chartPath)
}

func GetLocalDirectoryFromPath(ctx context.Context, chartPath string) (string, error) {
	cp := chartPath
	// first, try to download the file if it is a remote helm archive
	if !isLocalPath(chartPath) {
		if !(strings.HasSuffix(chartPath, ".tgz") || strings.HasSuffix(chartPath, ".tar.gz")) {
			return "", fmt.Errorf("%s is not a helm chart archive", chartPath) // a case where this is URL but not archive
		}
		p, err := downloadHelmChart(chartPath)
		if err != nil {
			return "", err
		}
		cp = p
	}

	// Check if it's an existing tar.gz archive (not a directory)
	info, err := os.Stat(cp)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		// this is already a directory, just return it's path
		return chartPath, nil
	} else if strings.HasSuffix(chartPath, ".tgz") || strings.HasSuffix(chartPath, ".tar.gz") {
		// this is an archive, extract to temporary directory and return its path
		dir, err := utils.MkdirTemp("", "helm-chart-*")
		if err != nil {
			return "", err
		}
		err = targzutils.Extract(ctx, chartPath, dir)
		if err != nil {
			return "", err
		}
		// return the actual path of the chart, by adding the internal directory inside archive
		d, err := os.Open(dir)
		if err != nil {
			return "", err
		}
		defer d.Close()
		entries, err := d.ReadDir(10) // Read at most 10 entries, since we expect only one root directory in the chart archive
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			// the directory is already a root of the chart
			if !entry.IsDir() && strings.ToLower(entry.Name()) == "chart.yaml" {
				return dir, nil
			}
			if entry.IsDir() && strings.ToLower(entry.Name()) != "templates" {
				// assume that this is the root directory of the chart, since it contains Chart.yaml and is not templates/ directory
				chartYamlPath := filepath.Join(dir, entry.Name(), "Chart.yaml")
				if _, err := os.Stat(chartYamlPath); err == nil {
					return filepath.Join(dir, entry.Name()), nil
				}
			}
		}
		return "", fmt.Errorf("%s is not a helm chart archive", cp)
	}
	return "", err
}

// loadRemoteChartWithHelm downloads and loads a chart from HTTP/HTTPS or OCI
func loadRemoteChartWithHelm(chartRef string) (*chart.Chart, error) {
	savedPath, err := downloadHelmChart(chartRef)
	if err != nil {
		return nil, err
	}
	// Load the downloaded chart
	return loader.Load(savedPath)
}

func downloadHelmChart(chartRef string) (string, error) {
	// Initialize Helm settings
	settings := cli.New()

	// Create temporary directory for download
	tmpDir, err := utils.MkdirTemp("", "helm-chart-*")
	if err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}

	// Create OCI registry client for OCI:// schemes
	regClient, err := registry.NewClient()
	if err != nil {
		return "", fmt.Errorf("create OCI registry client: %w", err)
	}

	// Create chart downloader with all supported getters
	chartDownloader := downloader.ChartDownloader{
		Out:              os.Stdout,
		Getters:          getter.All(settings),
		RegistryClient:   regClient,
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
	}

	// Download chart to temporary directory
	savedPath, _, err := chartDownloader.DownloadTo(chartRef, "", tmpDir)
	if err != nil {
		return "", fmt.Errorf("download chart %q: %w", chartRef, err)
	}
	return savedPath, nil
}

// isLocalPath checks if the path is a local file or directory
func isLocalPath(path string) bool {
	// Not a remote URL
	return !isHTTP(path) && !isOCI(path)
}

// isHTTP checks if the string is an HTTP or HTTPS URL
func isHTTP(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// isOCI checks if the string is an OCI registry URL
func isOCI(s string) bool {
	return strings.HasPrefix(s, "oci://")
}

// ConstructHelmChartURL builds a Helm chart URL from base URL, chart name pattern, and version
// Handles three patterns:
// 1. OCI format (Quay.io): oci://quay.io/weka.io/helm/weka-operator:v1.10.0
// 2. Standard Helm repo: https://charts.example.com/chart-name-1.2.0.tgz
// 3. GitHub releases: https://github.com/org/repo/releases/download/v1.2.0/chart-name-1.2.0.tgz
//
// Examples:
//   - OCI: ConstructHelmChartURL("oci://quay.io/weka.io/helm/weka-operator", "", "1.10.0")
//     Returns: "oci://quay.io/weka.io/helm/weka-operator:v1.10.0"
//   - Standard: ConstructHelmChartURL("https://charts.weka.io", "weka-operator", "1.2.0")
//     Returns: "https://charts.weka.io/weka-operator-1.2.0.tgz"
//   - GitHub: ConstructHelmChartURL("https://github.com/weka/csi-wekafs/releases/download", "csi-wekafsplugin", "2.8.2")
//     Returns: "https://github.com/weka/csi-wekafs/releases/download/v2.8.2/csi-wekafsplugin-2.8.2.tgz"
func ConstructHelmChartURL(baseURL string, chartPattern string, version string) string {
	// Remove trailing slash from base URL if present
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	// Detect if this is an OCI URL
	if isOCIURL(baseURL) {
		// OCI pattern: {baseURL}:v{version}
		return baseURL + ":v" + version
	}

	// Detect if this is a GitHub releases URL
	if isGitHubReleasesURL(baseURL) {
		// GitHub releases pattern: {baseURL}/v{version}/{chartPattern}-{version}.tgz
		return baseURL + "/v" + version + "/" + chartPattern + "-" + version + helmChartExtension
	}

	// Standard Helm repository pattern: {baseURL}/{chartPattern}-{version}.tgz
	return baseURL + "/" + chartPattern + "-" + version + helmChartExtension
}

// isOCIURL checks if a URL is an OCI (Open Container Initiative) URL
func isOCIURL(url string) bool {
	return contains(url, "oci://")
}

// isGitHubReleasesURL checks if a URL is a GitHub releases URL
func isGitHubReleasesURL(url string) bool {
	return contains(url, "github.com") && contains(url, "releases/download")
}

// contains checks if a string contains a substring (helper function)
func contains(str string, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func Archive(ctx context.Context, ch *chart.Chart, path string, originalRepo string) (*HelmChartArchive, error) {
	logger := logging.GetLogger(ctx)
	logger.Debug("Processing Helm chart", "output", path)

	if ch == nil {
		return nil, fmt.Errorf("chart is nil")
	}
	if ch.Metadata == nil {
		return nil, fmt.Errorf("chart metadata is nil")
	}
	if strings.TrimSpace(ch.Metadata.Name) == "" {
		return nil, fmt.Errorf("chart metadata.name is empty")
	}
	if strings.TrimSpace(ch.Metadata.Version) == "" {
		return nil, fmt.Errorf("chart metadata.version is empty")
	}
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("path is empty")
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("create archive directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.tgz", ch.Metadata.Name, ch.Metadata.Version)
	fullPath := filepath.Join(path, filename)

	tw, err := targzutils.NewTgzWriter(fullPath)
	if err != nil {
		return nil, err
	}

	root := fmt.Sprintf("%s/", ch.Metadata.Name)

	if err := WriteChartToTar(tw, ch, root); err != nil {
		_ = tw.Close()
		return nil, fmt.Errorf("write chart archive: %w", err)
	}

	// IMPORTANT: Close the tar writer BEFORE getting file size to ensure all data is flushed to disk
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar writer: %w", err)
	}

	size, sum, err := utils.FileSizeAndSHA256(fullPath)
	if err != nil {
		return nil, err
	}
	logger.Info("Chart archived successfully", "name", ch.Metadata.Name, "version", ch.Metadata.Version, "size", size, "filename", filename)
	return &HelmChartArchive{
		Name:       ch.Metadata.Name,
		Version:    ch.Metadata.Version,
		Filename:   fullPath,
		Size:       size,
		SHA256:     sum,
		Repository: originalRepo,
	}, nil
}

// CreateUpdatedChartArchive creates a new Helm chart archive with updated values
// It takes the original chart, merges in the updated values, and writes it to a tar.gz archive
// Reuses Archive to avoid code duplication
func CreateUpdatedChartArchive(ctx context.Context, chart *chart.Chart, updatedValues map[string]interface{}, outputPath string) error {
	logger := logging.GetLogger(ctx)

	// Merge updated values into chart
	if chart.Values == nil {
		chart.Values = make(map[string]interface{})
	}

	// Deep merge: apply updates to existing values
	for key, value := range updatedValues {
		chart.Values[key] = value
	}

	// Create archive directory
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Use Archive to create the tar.gz (reuses existing logic)
	// Temporarily modify the output path to be a directory instead of a file
	tmpDir, err := utils.MkdirTemp("", "chart-archive-*")
	if err != nil {
		return fmt.Errorf("create temporary directory: %w", err)
	}

	// Call Archive which handles all the tar.gz creation
	result, err := Archive(ctx, chart, tmpDir, "")
	if err != nil {
		return fmt.Errorf("archive updated chart: %w", err)
	}

	// Move the created archive to the target location
	if err := os.Rename(result.Filename, outputPath); err != nil {
		return fmt.Errorf("move chart archive to target location: %w", err)
	}

	logger.Info("Created updated Helm chart archive", "path", outputPath)
	return nil
}

// marshalToYAML converts a Go map to YAML bytes
func marshalToYAML(data interface{}) ([]byte, error) {
	yamlBytes, err := yamlv3.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal to YAML: %w", err)
	}
	return yamlBytes, nil
}
