package airgapped

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/docker"
	"github.com/weka/kubectl-weka/pkg/helm"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/targzutils"
	"github.com/weka/kubectl-weka/pkg/utils"
	"os"
	"path/filepath"
	"strings"
)

// Upload handles uploading Docker images to a custom registry
// It extracts images from the download bundle and uploads them to the specified registry
func Upload(ctx context.Context, opts UploadOptions) error {
	logger := logging.GetLogger(ctx)
	if opts.BundleFile == "" {
		return fmt.Errorf("bundle file is required")
	}
	if !utils.FileExists(opts.BundleFile) {
		return fmt.Errorf("bundle file does not exist: %q", opts.BundleFile)
	}
	if opts.RegistryURL == "" {
		return fmt.Errorf("registry URL is required")
	}
	// Step 1: Extract and validate bundle
	logger.Debug("Extracting and validating bundle")
	manifest, _, err := extractAndValidateBundle(ctx, opts.BundleFile)
	if err != nil {
		return fmt.Errorf("failed to extract bundle: %w", err)
	}

	logger.Debug("Bundle contains components", "count", len(manifest.Components), "charts", len(manifest.HelmCharts))
	for name, comp := range manifest.Components {
		logger.Info("Component", "name", name, "version", comp.Version, "images", len(comp.Images))
	}

	// Step 2: UploadDockerImage images
	logger.Info("Starting image upload", "bundle", opts.BundleFile, "registry", opts.RegistryURL)

	uploadedCount := 0
	failedCount := 0

	for compName, component := range manifest.Components {
		if component == nil || len(component.Images) == 0 {
			continue
		}

		logger.Info("Uploading component", "name", compName, "version", component.Version)

		for _, imgArchive := range component.Images {
			if imgArchive == nil || imgArchive.Filename == "" {
				continue
			}

			// Skip if architecture filter is specified and doesn't match
			if opts.Architecture != "" && imgArchive.Architecture != opts.Architecture {
				logger.Info("Skipping image", "architecture", imgArchive.Architecture, "filter", opts.Architecture)
				continue
			}

			// Use the first image reference from the archive
			if len(imgArchive.ImageReferences) == 0 {
				logger.Warn("no image references found in archive", "component", compName, "file", imgArchive.Filename)
				failedCount++
				continue
			}

			imageRef := imgArchive.OriginalReference // Original reference must be stored in imgArchive

			// Rewrite image reference to use the new registry
			rewrittenRef := docker.UpdateTagForNewRegistry(imageRef, opts.RegistryURL)
			logger.Debug("Uploading image archive", "component", compName, "architecture", imgArchive.Architecture, "file", filepath.Base(imgArchive.Filename), "original_image", imageRef, "target_image", rewrittenRef)

			// Upload Docker images from archive to registry with rewritten reference
			if err := docker.UploadDockerImage(ctx, imgArchive.Filename, rewrittenRef, opts.Username, opts.Password); err != nil {
				logger.Warn("failed to upload image archive", "component", compName, "file", imgArchive.Filename, "error", err)
				failedCount++
				return fmt.Errorf("failed to upload image archive: %w", err)
			}

			uploadedCount++
		}
	}

	// Summary
	logger.Info("UploadDockerImage Summary", "uploaded", uploadedCount, "failed", failedCount)
	if failedCount > 0 {
		return fmt.Errorf("some images failed to upload")
	}

	logger.Info("Images successfully uploaded to registry")

	// Step 3: Update Helm charts with new image URLs
	logger.Info("Updating Helm charts with new image references")
	if err := updateHelmCharts(ctx, manifest, opts.RegistryURL, opts.BundleFile); err != nil {
		logger.Warn("failed to update Helm charts", "error", err)
		// Non-fatal: continue even if chart updates fail
	}

	logger.Info("Upload completed successfully")

	// Print summary
	printUploadSummary(ctx, manifest, opts.RegistryURL, opts.BundleFile)

	return nil
}

// extractAndValidateBundle extracts and validates a download bundle
// It performs the following:
// 1. Verifies SHA256 signature if present
// 2. Extracts the tar.gz file to a temporary directory
// 3. Parses and validates the manifest
// 4. Validates SHA256 for all component images and charts
// 5. Returns the manifest with updated absolute paths and the temporary directory path
func extractAndValidateBundle(ctx context.Context, bundlePath string) (*BundleManifest, string, error) {
	logger := logging.GetLogger(ctx)

	// Step 1: Verify SHA256 signature
	logger.Info("Verifying bundle signature", "bundle", bundlePath)
	sigValid, err := utils.VerifySHA256Signature(bundlePath)
	if err != nil {
		return nil, "", fmt.Errorf("verify SHA256 signature: %w", err)
	}
	if !sigValid {
		logger.Warn("SHA256 signature file not found - proceeding without verification")
	} else {
		logger.Info("BundleManifest signature verified successfully")
	}

	// Step 2: Create temporary directory for extraction
	tempDir, err := utils.MkdirTemp("", "weka-bundle-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temporary directory: %w", err)
	}

	// Step 3: Extract tar.gz
	logger.Info("Extracting bundle", "destination", tempDir)
	if err := targzutils.Extract(ctx, bundlePath, tempDir); err != nil {
		_ = os.RemoveAll(tempDir) // Cleanup on error
		return nil, "", fmt.Errorf("extract bundle: %w", err)
	}
	logger.Debug("BundleManifest extracted successfully")

	// Step 4: Load and parse manifest
	manifestPath := filepath.Join(tempDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		_ = os.RemoveAll(tempDir) // Cleanup on error
		return nil, "", fmt.Errorf("read manifest file: %w", err)
	}

	var manifest BundleManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		_ = os.RemoveAll(tempDir) // Cleanup on error
		return nil, "", fmt.Errorf("parse manifest JSON: %w", err)
	}

	logger.Info("Manifest loaded", "version", manifest.Version, "created", manifest.CreatedAt)

	// Step 5: Validate all files in manifest and update paths
	logger.Debug("Validating bundle contents")

	// Validate component images
	for compName, comp := range manifest.Components {
		if comp == nil || len(comp.Images) == 0 {
			continue
		}

		for _, img := range comp.Images {
			if img == nil || img.Filename == "" {
				continue
			}

			// Convert relative path to absolute
			absPath := filepath.Join(tempDir, img.Filename)
			logger.Debug("Validating image archive", "component", compName, "filename", img.Filename, "absolute_path", absPath)

			// Check if file exists
			if _, err := os.Stat(absPath); err != nil {
				_ = os.RemoveAll(tempDir) // Cleanup on error
				return nil, "", fmt.Errorf("image file not found: %q (expected at %q)", img.Filename, absPath)
			}

			// Verify SHA256 if present
			if img.SHA256 != "" {
				_, actualHash, err := utils.FileSizeAndSHA256(absPath)
				if err != nil {
					_ = os.RemoveAll(tempDir) // Cleanup on error
					return nil, "", fmt.Errorf("calculate SHA256 for image %q: %w", img.Filename, err)
				}

				if actualHash != img.SHA256 {
					_ = os.RemoveAll(tempDir) // Cleanup on error
					return nil, "", fmt.Errorf("SHA256 mismatch for image %q: expected %s, got %s", img.Filename, img.SHA256, actualHash)
				}
				logger.Debug("Image SHA256 verified", "filename", img.Filename)
			}

			// Update filename to absolute path
			img.Filename = absPath
		}
	}
	logger.Info("BundleManifest extracted successfully and passed integrity checks", "bundle", bundlePath)

	// Validate Helm charts
	for chartName, chartData := range manifest.HelmCharts {
		if chartData == nil || chartData.Filename == "" {
			continue
		}

		// Convert relative path to absolute
		absPath := filepath.Join(tempDir, chartData.Filename)
		logger.Debug("Validating Helm chart", "name", chartName, "filename", chartData.Filename, "absolute_path", absPath)

		// Check if file exists
		if _, err := os.Stat(absPath); err != nil {
			_ = os.RemoveAll(tempDir) // Cleanup on error
			return nil, "", fmt.Errorf("Helm chart file not found: %q (expected at %q)", chartData.Filename, absPath)
		}

		// Verify SHA256 if present
		if chartData.SHA256 != "" {
			_, actualHash, err := utils.FileSizeAndSHA256(absPath)
			if err != nil {
				_ = os.RemoveAll(tempDir) // Cleanup on error
				return nil, "", fmt.Errorf("calculate SHA256 for chart %q: %w", chartData.Filename, err)
			}

			if actualHash != chartData.SHA256 {
				_ = os.RemoveAll(tempDir) // Cleanup on error
				return nil, "", fmt.Errorf("SHA256 mismatch for chart %q: expected %s, got %s", chartData.Filename, chartData.SHA256, actualHash)
			}
			logger.Debug("Chart SHA256 verified", "filename", chartData.Filename)
		}

		// Update filename to absolute path
		chartData.Filename = absPath
	}

	logger.Debug("BundleManifest validation completed successfully", "components", len(manifest.Components), "charts", len(manifest.HelmCharts))

	return &manifest, tempDir, nil
}

// updateHelmCharts updates Helm charts with new image URLs after upload
// It performs the following:
// 1. Loads Helm charts from the manifest
// 2. Updates image references with new registry URLs
// 3. Creates new Helm archives with updated values
// 4. Generates override values files for convenient deployment
func updateHelmCharts(ctx context.Context, manifest *BundleManifest, registryURL string, bundleFile string) error {
	logger := logging.GetLogger(ctx)

	// Create output directory for updated charts (in same directory as bundle)
	bundleDir := filepath.Dir(bundleFile)
	chartsOutputDir := filepath.Join(bundleDir, "charts-updated")
	valuesOutputDir := filepath.Join(bundleDir, "values-overrides")

	if err := os.MkdirAll(chartsOutputDir, 0755); err != nil {
		return fmt.Errorf("create charts output directory: %w", err)
	}

	if err := os.MkdirAll(valuesOutputDir, 0755); err != nil {
		return fmt.Errorf("create values output directory: %w", err)
	}

	logger.Info("Updated Helm charts will be saved to", "directory", chartsOutputDir)
	logger.Info("Override values files will be saved to", "directory", valuesOutputDir)

	// Build mapping of original → rewritten image references for all components
	imageMapping := manifest.buildImageMapping(registryURL)
	logger.Info("Built image mapping", "count", len(imageMapping))

	// Process Operator chart if present
	if chartData, ok := manifest.HelmCharts["weka-operator"]; ok && chartData != nil {
		logger.Info("Updating Operator Helm chart")
		if err := updateOperatorChart(ctx, chartData.Filename, imageMapping, chartsOutputDir, valuesOutputDir); err != nil {
			logger.Warn("failed to update Operator chart", "error", err)
		}
	}

	// Process CSI chart if present
	if chartData, ok := manifest.HelmCharts["csi-wekafsplugin"]; ok && chartData != nil {
		logger.Info("Updating CSI Helm chart")
		if err := updateCSIChart(ctx, chartData.Filename, imageMapping, chartsOutputDir, valuesOutputDir); err != nil {
			logger.Warn("failed to update CSI chart", "error", err)
		}
	}

	logger.Info("Helm chart updates completed")
	return nil
}

// updateOperatorChart loads the operator Helm chart, updates image references, and creates updated archives
func updateOperatorChart(ctx context.Context, chartPath string, imageMapping map[string]string, chartsOutputDir string, valuesOutputDir string) error {
	logger := logging.GetLogger(ctx)

	// Load the Helm chart
	chart, err := helm.LoadChart(chartPath)
	if err != nil {
		return fmt.Errorf("load operator chart: %w", err)
	}

	if chart.Values == nil {
		chart.Values = make(map[string]interface{})
	}

	// Create updated values map with new image references
	updatedValues := updateOperatorChartValues(chart.Values, imageMapping)

	logger.Debug("Operator chart values updated", "values_count", len(updatedValues))

	// Create new chart archive with updated values
	archivePath := filepath.Join(chartsOutputDir, fmt.Sprintf("weka-operator-%s-air-gapped.tgz", chart.Metadata.Version))
	if err := helm.CreateUpdatedChartArchive(ctx, chart, updatedValues, archivePath); err != nil {
		return fmt.Errorf("create updated operator chart archive: %w", err)
	}
	logger.Info("Created updated Operator chart archive", "path", archivePath)

	// Create override values file
	overridePath := filepath.Join(valuesOutputDir, "values-operator.yaml")
	if err := helm.CreateOverrideValuesFile(updatedValues, overridePath); err != nil {
		return fmt.Errorf("create operator override values file: %w", err)
	}
	logger.Info("Created operator override values file", "path", overridePath)

	return nil
}

// updateCSIChart loads the CSI Helm chart, updates image references, and creates updated archives
func updateCSIChart(ctx context.Context, chartPath string, imageMapping map[string]string, chartsOutputDir string, valuesOutputDir string) error {
	logger := logging.GetLogger(ctx)

	// Load the Helm chart
	chart, err := helm.LoadChart(chartPath)
	if err != nil {
		return fmt.Errorf("load CSI chart: %w", err)
	}

	if chart.Values == nil {
		chart.Values = make(map[string]interface{})
	}

	// Create updated values map with new image references
	updatedValues := updateCSIChartValues(chart.Values, imageMapping)

	logger.Debug("CSI chart values updated", "values_count", len(updatedValues))

	// Create new chart archive with updated values
	archivePath := filepath.Join(chartsOutputDir, fmt.Sprintf("csi-wekafsplugin-%s-air-gapped.tgz", chart.Metadata.Version))
	if err := helm.CreateUpdatedChartArchive(ctx, chart, updatedValues, archivePath); err != nil {
		return fmt.Errorf("create updated CSI chart archive: %w", err)
	}
	logger.Info("Created updated CSI chart archive", "path", archivePath)

	// Create override values file
	overridePath := filepath.Join(valuesOutputDir, "values-csi.yaml")
	if err := helm.CreateOverrideValuesFile(updatedValues, overridePath); err != nil {
		return fmt.Errorf("create CSI override values file: %w", err)
	}
	logger.Info("Created CSI override values file", "path", overridePath)

	return nil
}

// updateOperatorChartValues updates image references in operator chart values
// Returns a map containing only the updated values (not the full values map)
func updateOperatorChartValues(values map[string]interface{}, imageMapping map[string]string) map[string]interface{} {
	updatedValues := make(map[string]interface{})

	// Image paths that need updating for the operator chart
	simplePaths := []string{"sign-drive-image", "kubeProxyImage", "maintenanceImage"}
	for _, path := range simplePaths {
		if imgRef, ok := values[path].(string); ok && imgRef != "" {
			if newRef, found := imageMapping[imgRef]; found {
				updatedValues[path] = newRef
			}
		}
	}

	// Nested image paths
	nestedPaths := map[string]string{
		"taskmon.defaultImage":   "image",
		"metrics.clusters.image": "image",
		"csi.image":              "image",
		"csi.provisionerImage":   "image",
		"csi.attacherImage":      "image",
		"csi.livenessProbeImage": "image",
		"csi.resizerImage":       "image",
		"csi.snapshotterImage":   "image",
		"csi.registrarImage":     "image",
	}

	for dotPath := range nestedPaths {
		if imgRef := helm.GetNestedValue(values, dotPath); imgRef != "" {
			if newRef, found := imageMapping[imgRef]; found {
				helm.SetNestedValue(updatedValues, dotPath, newRef)
			}
		}
	}

	// Special case: operator image built from image.repository + tag
	if repo := helm.GetNestedValue(values, "image.repository"); repo != "" {
		// Try to find matching entry in imageMapping by partial match
		for origRef, newRef := range imageMapping {
			if helm.GetNestedValue(values, "image.tag") != "" {
				fullOriginal := repo + ":" + helm.GetNestedValue(values, "image.tag")
				if origRef == fullOriginal {
					// Extract new registry and repo
					helm.SetNestedValue(updatedValues, "image.repository", docker.ExtractRepositoryFromImage(newRef))
					helm.SetNestedValue(updatedValues, "image.tag", docker.ExtractTagFromImage(newRef))
					break
				}
			}
		}
	}

	return updatedValues
}

// updateCSIChartValues updates image references in CSI chart values
// Returns a map containing only the updated values (not the full values map)
func updateCSIChartValues(values map[string]interface{}, imageMapping map[string]string) map[string]interface{} {
	updatedValues := make(map[string]interface{})

	// Sidecar image paths
	sidecarPaths := map[string]string{
		"images.livenessprobesidecar": "image",
		"images.attachersidecar":      "image",
		"images.provisionersidecar":   "image",
		"images.registrarsidecar":     "image",
		"images.resizersidecar":       "image",
		"images.snapshottersidecar":   "image",
	}

	for dotPath := range sidecarPaths {
		if imgRef := helm.GetNestedValue(values, dotPath); imgRef != "" {
			if newRef, found := imageMapping[imgRef]; found {
				helm.SetNestedValue(updatedValues, dotPath, newRef)
			}
		}
	}

	// CSI driver image
	if driverImg := helm.GetNestedValue(values, "images.csidriver"); driverImg != "" {
		// Find the mapped image for this driver
		for origRef, newRef := range imageMapping {
			// Check if this mapping entry is for the csi-wekafs driver
			if strings.HasPrefix(origRef, driverImg) {
				// Extract just the repository part (without tag) from the new reference
				newRepo := docker.StripTagFromImage(newRef)
				helm.SetNestedValue(updatedValues, "images.csidriver", newRepo)
				break
			}
		}
	}

	return updatedValues
}

// printUploadSummary prints a formatted summary of the upload operation
func printUploadSummary(ctx context.Context, manifest *BundleManifest, registryURL string, bundleFile string) {
	logger := logging.GetLogger(ctx)

	// Print header
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           WEKA Air-Gapped Upload Summary                       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Images uploaded table
	if len(manifest.Components) > 0 {
		fmt.Println("📤 Images Uploaded:")
		imageCols := []printer.TableColumn{
			{Name: "Component", VisibleInWide: false},
			{Name: "Architecture", VisibleInWide: false},
			{Name: "Size", VisibleInWide: false},
			{Name: "New Registry", VisibleInWide: false},
		}

		imageRows := []printer.TableRow{}
		totalUploadSize := int64(0)

		for compName, comp := range manifest.Components {
			if comp == nil || len(comp.Images) == 0 {
				continue
			}

			for _, img := range comp.Images {
				if img == nil {
					continue
				}

				totalUploadSize += img.Size
				newRef := docker.UpdateTagForNewRegistry(img.OriginalReference, registryURL)
				newRepo := docker.ExtractRepositoryFromImage(newRef)

				imageRows = append(imageRows, printer.TableRow{
					Values: map[string]interface{}{
						"Component":    compName,
						"Architecture": img.Architecture,
						"Size":         utils.HumanBytes(img.Size),
						"New Registry": newRepo,
					},
				})

				logger.Debug("Image in upload summary", "component", compName, "architecture", img.Architecture, "size", img.Size)
			}
		}

		tp := &printer.TablePrinter{}
		tp.SetOptions(printer.PrinterOptions{
			ShowHeader: true,
			TableStyle: printer.TableStyleRoundedBox,
		})
		_ = tp.Print(imageCols, imageRows, os.Stdout)
		fmt.Println()

		// Total uploaded size
		fmt.Printf("Total images uploaded: %s\n", utils.HumanBytes(totalUploadSize))
		fmt.Println()
	}

	// Architectures table
	if len(manifest.Architectures) > 0 {
		fmt.Println("🏗️  Supported Architectures:")
		archCols := []printer.TableColumn{
			{Name: "Architecture", VisibleInWide: false},
		}

		archRows := []printer.TableRow{}
		for _, arch := range manifest.Architectures {
			archRows = append(archRows, printer.TableRow{
				Values: map[string]interface{}{
					"Architecture": arch,
				},
			})
		}

		tp := &printer.TablePrinter{}
		tp.SetOptions(printer.PrinterOptions{
			ShowHeader: true,
			TableStyle: printer.TableStyleRoundedBox,
		})
		_ = tp.Print(archCols, archRows, os.Stdout)
		fmt.Println()
	}

	// Output directories
	bundleDir := filepath.Dir(bundleFile)
	chartsOutputDir := filepath.Join(bundleDir, "charts-updated")
	valuesOutputDir := filepath.Join(bundleDir, "values-overrides")

	fmt.Println("📁 Output Files and Directories:")
	outputRows := []printer.TableRow{
		{
			Values: map[string]interface{}{
				"Item":     "Updated Helm Charts",
				"Location": chartsOutputDir,
			},
		},
		{
			Values: map[string]interface{}{
				"Item":     "Override Values Files",
				"Location": valuesOutputDir,
			},
		},
	}

	outputCols := []printer.TableColumn{
		{Name: "Item", VisibleInWide: false},
		{Name: "Location", VisibleInWide: false},
	}

	tp := &printer.TablePrinter{}
	tp.SetOptions(printer.PrinterOptions{
		ShowHeader: true,
		TableStyle: printer.TableStyleRoundedBox,
	})
	_ = tp.Print(outputCols, outputRows, os.Stdout)
	fmt.Println()

	// Next steps
	fmt.Println("✅ Upload completed successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Deploy using updated charts:  helm install -f %s/values-*.yaml <chart-path>\n", valuesOutputDir)
	fmt.Printf("  2. Or use the all-in-one charts from: %s\n", chartsOutputDir)
}
