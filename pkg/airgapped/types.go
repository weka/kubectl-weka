package airgapped

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/docker"
	"github.com/weka/kubectl-weka/pkg/helm"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/utils"
	"helm.sh/helm/v3/pkg/chart"
	"strings"
)

// DownloadOptions specifies options for downloading images
type DownloadOptions struct {
	Ctx context.Context

	// OutputFile is the path to the tar.gz bundle file (auto-generated if empty)
	OutputFile string

	// WEKA Software
	WekaVersion string // e.g., "5.3.0"

	// WEKA Operator
	OperatorVersion  string // e.g., "1.2.0", ignored if OperatorHelmPath is set
	OperatorHelmPath string // Local path, archive, or remote URL

	// WEKA CSI Driver
	CSIVersion  string // e.g., "2.1.0", ignored if CSIHelmPath is set
	CSIHelmPath string // Local path, archive, or remote URL

	// Architecture specifies which architectures to download (default: "amd64,arm64")
	Archs []string // "amd64", "arm64", or "amd64,arm64"

	OperatorChart *chart.Chart
	CSIChart      *chart.Chart
}

func NewDownloadOptions(ctx context.Context, outputFile, wekaVersion, operatorVersion, csiVersion, operatorHelmPath, csiHelmPath, archs string) *DownloadOptions {
	// 		Ctx:              ctx,
	//		OutputFile:       flagDownloadOutput,
	//		WekaVersion:      flagWekaVersion,
	//		OperatorVersion:  flagOperatorVersion,
	//		CSIVersion:       flagCSIVersion,
	//		OperatorHelmPath: flagOperatorHelmPath,
	//		CSIHelmPath:      flagCSIHelmPath,
	//		Archs:            archs,
	opts := &DownloadOptions{
		Ctx:              ctx,
		OutputFile:       outputFile,
		WekaVersion:      wekaVersion,
		OperatorVersion:  operatorVersion,
		OperatorHelmPath: operatorHelmPath,
		CSIVersion:       csiVersion,
		CSIHelmPath:      csiHelmPath,
		Archs:            utils.SplitAndTrim(archs, ","),
	}
	opts.generateOutputFilename()
	return opts
}

func (opts *DownloadOptions) Validate() error {
	if opts.WekaVersion == "" && opts.OperatorHelmPath == "" && opts.OperatorVersion == "" &&
		opts.CSIHelmPath == "" && opts.CSIVersion == "" {
		return fmt.Errorf("must specify at least one of: --weka-version, --operator-version, --operator-helm-path, --csi-version, --csi-helm-path")
	}
	return nil
}

// generateOutputFilename generates a default output filename based on components and versions
// Format: weka-{version}-operator-{version}-csi-{version}-offline-bundle.tar.gz
// Example: weka-5.3.0-operator-1.2.0-offline-bundle.tar.gz
func (opts *DownloadOptions) generateOutputFilename() {
	if opts.OutputFile != "" {
		return
	}
	var parts []string

	// Add WEKA version if specified
	if opts.WekaVersion != "" {
		parts = append(parts, "weka-"+opts.WekaVersion)
	}

	// Add Operator version if specified or will be downloaded
	operatorVersion := opts.OperatorVersion
	if operatorVersion == "" && opts.OperatorHelmPath != "" {
		// If local path is specified but no version, try to extract from path
		// For now, we'll use a placeholder - full implementation would parse the path
		operatorVersion = "local"
	}
	if operatorVersion != "" {
		parts = append(parts, "operator-"+operatorVersion)
	}

	// Add CSI version if specified or will be downloaded
	csiVersion := opts.CSIVersion
	if csiVersion == "" && opts.CSIHelmPath != "" {
		// If local path is specified but no version, try to extract from path
		// For now, we'll use a placeholder - full implementation would parse the path
		csiVersion = "local"
	}
	if csiVersion != "" {
		parts = append(parts, "csi-"+csiVersion)
	}

	// Join all parts and add suffix
	if len(parts) == 0 {
		// Fallback if no components specified (shouldn't happen due to validation)
		opts.OutputFile = "weka-offline-bundle.tar.gz"
	}

	opts.OutputFile = fmt.Sprintf("%s-offline-bundle.tar.gz", strings.Join(parts, "-"))
}

// Normalize processes DownloadOptions to ensure consistent state
// It performs the following operations:
// 1. If version is specified but helm path is not, constructs helm path from version
// 2. If helm path is specified but version is not, extracts version from chart
// After this function, all relevant fields should be populated
func (opts *DownloadOptions) Normalize() error {
	// Normalize versions to always be in "x.y.z" format (remove leading 'v' if present)
	logger := logging.GetLogger(opts.Ctx)
	logger.Info("Normalizing options and downloading Helm charts")
	opts.WekaVersion = utils.NormalizeVersion(opts.WekaVersion)
	opts.OperatorVersion = utils.NormalizeVersion(opts.OperatorVersion)
	opts.CSIVersion = utils.NormalizeVersion(opts.CSIVersion)

	// Handle Operator
	if opts == nil {
		return fmt.Errorf("download options cannot be nil")
	}
	if err := opts.normalizeOperator(); err != nil {
		return err
	}

	// Handle CSI
	if err := opts.normalizeCSI(); err != nil {
		return err
	}

	return nil
}

// normalizeOperator handles operator version/helm-path normalization
func (opts *DownloadOptions) normalizeOperator() error {
	logger := logging.GetLogger(opts.Ctx)

	// If version is specified but helm path is not
	if opts.OperatorVersion != "" && opts.OperatorHelmPath == "" {
		// Construct helm path from version
		// Example: https://charts.weka.io/weka-operator-1.2.0.tgz
		opts.OperatorHelmPath = helm.ConstructHelmChartURL(
			defaultOperatorHelmURL,
			operatorChartPattern,
			opts.OperatorVersion,
		)
		logger.Debug("Downloading operator chart", "url", opts.OperatorHelmPath)
	}

	// If helm path is specified but version is not
	if opts.OperatorHelmPath != "" {
		// Extract version from helm chart
		ch, err := helm.LoadChart(opts.OperatorHelmPath)
		if err != nil {
			return fmt.Errorf("failed to extract operator version from chart %s: %w", opts.OperatorHelmPath, err)
		}
		opts.OperatorChart = ch
		version, err := helm.GetVersion(opts.OperatorChart)
		version = utils.NormalizeVersion(version)
		logger.Info("Operator chart", "version", version)
		if version != "" && opts.OperatorVersion != "" && version != opts.OperatorVersion {
			return fmt.Errorf("version mismatch: Operator version from helm chart (%s) does not match specified version (%s)", version, opts.OperatorVersion)
		}
		opts.OperatorVersion = version
	}

	return nil
}

// normalizeCSI handles CSI version/helm-path normalization
func (opts *DownloadOptions) normalizeCSI() error {
	logger := logging.GetLogger(opts.Ctx)
	// If version is specified but helm path is not
	if opts.CSIVersion != "" && opts.CSIHelmPath == "" {
		// Construct helm path from version
		// Example: https://charts.weka.io/weka-csi-2.1.0.tgz
		opts.CSIHelmPath = helm.ConstructHelmChartURL(
			defaultCSIHelmURL,
			csiChartPattern,
			opts.CSIVersion,
		)
		logger.Debug("Downloading CSI plugin chart", "path", opts.CSIHelmPath)
	}

	// If helm path is specified but version is not
	if opts.CSIHelmPath != "" {
		logger.Debug("Downloading CSI chart", "path", opts.CSIHelmPath)
		// Extract version from helm chart
		ch, err := helm.LoadChart(opts.CSIHelmPath)
		if err != nil {
			return fmt.Errorf("failed to extract operator version from chart %s: %w", opts.OperatorHelmPath, err)
		}
		opts.CSIChart = ch
		version, err := helm.GetVersion(opts.CSIChart)
		version = utils.NormalizeVersion(version)
		logger.Info("CSI chart version", "version", version)
		if version != "" && opts.CSIVersion != "" && version != opts.CSIVersion {
			return fmt.Errorf("version mismatch: CSI plugin version from helm chart (%s) does not match specified version (%s)", version, opts.CSIVersion)
		}
		opts.CSIVersion = version
	}
	return nil
}

// UploadOptions specifies options for uploading images
type UploadOptions struct {
	// BundleFile is the path to the tar.gz bundle from the download command
	BundleFile string

	// RegistryURL is the target registry URL
	RegistryURL string

	// Username for registry authentication (optional)
	Username string

	// Password for registry authentication (optional)
	Password string

	// Architecture specifies which architectures to upload from bundle (optional, default: all)
	Architecture string // "amd64", "arm64", or empty for all
}

// BundleManifest describes the contents of a download bundle
type BundleManifest struct {
	// Version of the manifest format
	Version string `json:"version"`

	// Timestamp of bundle creation
	CreatedAt string `json:"createdAt"`

	// Components describes each downloaded component
	Components map[string]*ComponentManifest `json:"components"`

	// HelmCharts describes included Helm charts
	HelmCharts map[string]*helm.HelmChartArchive `json:"helmCharts"`

	// Architectures in this bundle
	Architectures []string `json:"architectures"`

	// Total size of all images in bytes
	TotalSize int64 `json:"totalSize"`
}

// buildImageMapping creates a mapping of original image references to their rewritten versions
// This allows us to update charts with new registry URLs
func (bm *BundleManifest) buildImageMapping(registryURL string) map[string]string {
	mapping := make(map[string]string)

	// Iterate through all components and their images
	for _, component := range bm.Components {
		if component == nil || len(component.Images) == 0 {
			continue
		}

		for _, imgArchive := range component.Images {
			if imgArchive == nil || imgArchive.OriginalReference == "" {
				continue
			}

			// Map original reference to rewritten reference
			rewritten := docker.UpdateTagForNewRegistry(imgArchive.OriginalReference, registryURL)
			mapping[imgArchive.OriginalReference] = rewritten
		}
	}

	return mapping
}

// ComponentManifest describes a single component in the bundle
type ComponentManifest struct {
	// Component name (operator, csi, weka)
	Name string `json:"name"`

	// Version of the component
	Version string `json:"version"`

	// Images archive
	Images []*docker.ImageArchive `json:"image"`

	// Total size of this component's images
	Size int64 `json:"size"`
}
