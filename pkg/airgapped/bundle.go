package airgapped

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/weka/kubectl-weka/pkg/docker"
	"github.com/weka/kubectl-weka/pkg/helm"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/progress"
	"github.com/weka/kubectl-weka/pkg/targzutils"
	"github.com/weka/kubectl-weka/pkg/utils"
	"helm.sh/helm/v3/pkg/chart"
	"os"
	"path/filepath"
	"time"
)

type Bundle struct {
	components []*ComponentManifest
	charts     []*helm.HelmChartArchive
	failed     []string
	size       int64

	processedSize int64

	bundleSize     int64
	bundleFilename string
	opts           *DownloadOptions
	tw             *targzutils.TgzWriter
}

func (b *Bundle) isOpen() bool {
	return b.tw != nil
}

func (b *Bundle) createWriter(bundleFile string) error {
	if b.tw != nil {
		return fmt.Errorf("bundle is already created")
	}

	tw, err := targzutils.NewTgzWriter(bundleFile)
	if err != nil {
		return fmt.Errorf("create tar.gz writer: %w", err)
	}
	b.tw = tw
	return nil
}

func (b *Bundle) Close() error {
	if b.tw == nil {
		return fmt.Errorf("bundle is already closed")
	}
	if b.tw != nil {
		err := b.tw.Close()
		if err != nil {
			return err
		}
	}
	b.tw = nil
	b.Account()
	return nil
}

func (b *Bundle) Sign(ctx context.Context) error {
	if b.isOpen() {
		return fmt.Errorf("bundle is open")
	}
	logger := logging.GetLogger(ctx)
	if b.bundleFilename == "" {
		return fmt.Errorf("bundle is empty")
	}

	logger.Debug("Creating SHA256 signature file", "file", b.bundleFilename)
	if err := utils.CreateSHA256Signature(b.bundleFilename); err != nil {
		logger.Error("Failed to create signature file", "file", b.bundleFilename, "error", err)
		return fmt.Errorf("create signature file: %w", err)
	}
	return nil
}

func (b *Bundle) RegisterComponent(component *ComponentManifest) {
	b.components = append(b.components, component)
}

func (b *Bundle) RegisterChart(chart *helm.HelmChartArchive) {
	b.charts = append(b.charts, chart)
}

func (b *Bundle) AddFailed(failed string) {
	b.failed = append(b.failed, failed)
}

func (b *Bundle) Size() int64 {
	return b.size
}

// Account recalculates the total size of the bundle based on registered components and charts, and updates bundleSize
// if the bundle file exists on disk. This should be called after all components and charts are registered.
// If bundleFile was created already (after calling Create) the bundleSize will be populated as well.
// If called just after Download, the bundleSize (actual Tar archive will not be set)
func (b *Bundle) Account() {
	b.size = 0 // always reset upon Account invocation to avoid doubling numbers

	for _, comp := range b.components {
		if comp != nil {
			b.size += comp.Size
		}
	}
	for _, chartData := range b.charts {
		if chartData != nil {
			b.size += chartData.Size
		}
	}
	if b.bundleFilename != "" && !b.isOpen() {
		b.bundleSize = 0
		if info, err := os.Stat(b.bundleFilename); err == nil {
			b.bundleSize = info.Size()
		}
	}
}

func (b *Bundle) Failed() []string {
	return b.failed
}

func (b *Bundle) Opts() DownloadOptions {
	return *b.opts
}

func (b *Bundle) Charts() []*helm.HelmChartArchive {
	return b.charts
}

func (b *Bundle) Components() []*ComponentManifest {
	return b.components
}

func (b *Bundle) PrintSummary(ctx context.Context) {
	logger := logging.GetLogger(ctx)

	// Print header
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           WEKA Air-Gapped Bundle Summary                       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Components Summary Table
	if len(b.components) > 0 {
		fmt.Println("📦 Components Downloaded:")
		componentsCols := []printer.TableColumn{
			{Name: "Component", VisibleInWide: false},
			{Name: "Version", VisibleInWide: false},
			{Name: "Images", VisibleInWide: false},
			{Name: "Size", VisibleInWide: false},
		}

		componentRows := []printer.TableRow{}
		for _, comp := range b.components {
			if comp == nil {
				continue
			}
			componentRows = append(componentRows, printer.TableRow{
				Values: map[string]interface{}{
					"Component": comp.Name,
					"Version":   comp.Version,
					"Images":    len(comp.Images),
					"Size":      utils.HumanBytes(comp.Size),
				},
			})
			logger.Debug("Component in summary", "name", comp.Name, "version", comp.Version, "size", comp.Size)
		}

		tp := &printer.TablePrinter{}
		tp.SetOptions(printer.PrinterOptions{
			ShowHeader: true,
			TableStyle: printer.TableStyleRoundedBox,
		})
		_ = tp.Print(componentsCols, componentRows, os.Stdout)
		fmt.Println()
	}

	// Helm Charts Summary Table
	if len(b.charts) > 0 {
		fmt.Println("📊 Helm Charts Packaged:")
		chartsCols := []printer.TableColumn{
			{Name: "Chart Name", VisibleInWide: false},
			{Name: "Version", VisibleInWide: false},
			{Name: "Filename", VisibleInWide: false},
			{Name: "Size", VisibleInWide: false},
		}

		chartRows := []printer.TableRow{}
		for _, ch := range b.charts {
			if ch == nil {
				continue
			}
			logger.Debug("Chart in summary", "name", ch.Name, "version", ch.Version, "filename", ch.Filename, "size", ch.Size)
			chartRows = append(chartRows, printer.TableRow{
				Values: map[string]interface{}{
					"Chart Name": ch.Name,
					"Version":    ch.Version,
					"Filename":   filepath.Base(ch.Filename),
					"Size":       utils.HumanBytes(ch.Size),
				},
			})
		}

		tp := &printer.TablePrinter{}
		tp.SetOptions(printer.PrinterOptions{
			ShowHeader: true,
			TableStyle: printer.TableStyleRoundedBox,
		})
		_ = tp.Print(chartsCols, chartRows, os.Stdout)
		fmt.Println()
	}

	// Failed Components (if any)
	if len(b.failed) > 0 {
		fmt.Println("⚠️  Failed to Download:")
		failedCols := []printer.TableColumn{
			{Name: "Component", VisibleInWide: false},
		}

		failedRows := []printer.TableRow{}
		for _, comp := range b.failed {
			failedRows = append(failedRows, printer.TableRow{
				Values: map[string]interface{}{
					"Component": comp,
				},
			})
		}

		tp := &printer.TablePrinter{}
		tp.SetOptions(printer.PrinterOptions{
			ShowHeader: true,
			TableStyle: printer.TableStyleRoundedBox,
		})
		_ = tp.Print(failedCols, failedRows, os.Stdout)
		fmt.Println()
	}

	// Bundle Summary Box
	fmt.Println("📦 Bundle Summary:")
	summaryRows := []printer.TableRow{
		{
			Values: map[string]interface{}{
				"Metric": "Total Components",
				"Value":  len(b.Components()),
			},
		},
		{
			Values: map[string]interface{}{
				"Metric": "Total Charts",
				"Value":  len(b.Charts()),
			},
		},
		{
			Values: map[string]interface{}{
				"Metric": "Total Bundle Size",
				"Value":  utils.HumanBytes(b.Size()),
			},
		},
		{
			Values: map[string]interface{}{
				"Metric": "Bundle File",
				"Value":  filepath.Base(b.bundleFilename),
			},
		},
	}

	summaryCols := []printer.TableColumn{
		{Name: "Metric", VisibleInWide: false},
		{Name: "Value", VisibleInWide: false},
	}

	tp := &printer.TablePrinter{}
	tp.SetOptions(printer.PrinterOptions{
		ShowHeader: true,
		TableStyle: printer.TableStyleRoundedBox,
	})
	_ = tp.Print(summaryCols, summaryRows, os.Stdout)
	fmt.Println()

	// Success message
	fmt.Println("✅ Bundle created successfully!")
	logger.Debug("Bundle file details", "path", b.bundleFilename, "size", b.bundleSize)
}

// downloadWekaComponent downloads WEKA software component images
// Downloads multi-arch images from quay.io/weka.io/weka-in-container
func (b *Bundle) downloadWekaComponent(ctx context.Context) error {
	if b.opts == nil {
		return fmt.Errorf("no options provided")
	}
	logger := logging.GetLogger(ctx)
	if b.opts.WekaVersion == "" {
		logger.Debug("Weka version not provided")
		return nil
	}

	logger.Info("Downloading WEKA software image", "version", b.opts.WekaVersion, "architecture", b.opts.Archs)
	archive, err := docker.Download(ctx, WekaInContainerImageBase, b.opts.WekaVersion, b.opts.Archs, "linux", nil)
	if err != nil {
		return err
	}

	b.RegisterComponent(&ComponentManifest{
		Name:    "weka",
		Version: b.opts.WekaVersion,
		Images:  []*docker.ImageArchive{archive},
		Size:    archive.Size,
	})
	return nil
}

// downloadOperatorImages downloads all images from Operator Helm chart
func (b *Bundle) downloadOperatorImages(ctx context.Context, ch *chart.Chart, auth authn.Authenticator) (*ComponentManifest, error) {
	logger := logging.GetLogger(ctx)
	ret := &ComponentManifest{
		Name:    "operator-images",
		Version: ch.Metadata.Version,
		Images:  []*docker.ImageArchive{},
		Size:    0,
	}

	if ch.Values == nil {
		return ret, nil
	}

	// Image paths to extract: simple root-level paths
	simplePaths := []string{
		"sign-drive-image",
		"kubeProxyImage",
		"maintenanceImage",
	}

	for _, path := range simplePaths {
		if img, ok := ch.Values[path].(string); ok && img != "" {
			logger.Debug("Downloading image", "path", path, "image", img)
			archive, err := docker.Download(ctx, img, "", b.opts.Archs, "linux", auth)
			if err != nil {
				logger.Warn("failed to download image", "path", path, "image", img, "error", err)
				continue
			}
			ret.Images = append(ret.Images, archive)
			ret.Size += archive.Size
		}
	}

	// Nested image paths: navigate through the values map hierarchy
	nestedPaths := map[string]string{
		"taskmon.defaultImage":   "taskmon default image",
		"metrics.clusters.image": "metrics clusters image",
		"csi.image":              "CSI driver image",
		"csi.provisionerImage":   "CSI provisioner image",
		"csi.attacherImage":      "CSI attacher image",
		"csi.livenessProbeImage": "CSI liveness probe image",
		"csi.resizerImage":       "CSI resizer image",
		"csi.snapshotterImage":   "CSI snapshotter image",
		"csi.registrarImage":     "CSI registrar image",
	}

	for pathKey, description := range nestedPaths {
		if img := helm.GetNestedValue(ch.Values, pathKey); img != "" {
			logger.Debug("Downloading image", "path", pathKey, "description", description, "image", img)
			archive, err := docker.Download(ctx, img, "", b.opts.Archs, "linux", auth)
			if err != nil {
				logger.Warn("failed to download image", "path", pathKey, "image", img, "error", err)
				continue
			}
			ret.Images = append(ret.Images, archive)
			ret.Size += archive.Size
		}
	}

	// Special case: operatorImage built from image.repository + chart.appVersion
	if repo := helm.GetNestedValue(ch.Values, "image.repository"); repo != "" {
		tag := ch.Metadata.AppVersion
		if tag == "" {
			tag = ch.Metadata.Version
		}
		fullImage := repo + ":" + tag

		logger.Debug("Downloading operator image", "repository", repo, "tag", tag, "full_image", fullImage)
		archive, err := docker.Download(ctx, repo, tag, b.opts.Archs, "linux", auth)
		if err != nil {
			logger.Warn("failed to download operator image", "image", fullImage, "error", err)
		} else {
			ret.Images = append(ret.Images, archive)
			ret.Size += archive.Size
		}
	}

	return ret, nil
}

// downloadCSIImages downloads all images from CSI plugin Helm chart
func (b *Bundle) downloadCSIImages(ctx context.Context, ch *chart.Chart, auth authn.Authenticator) (*ComponentManifest, error) {
	logger := logging.GetLogger(ctx)
	ret := &ComponentManifest{
		Name:    "csi-images",
		Version: ch.Metadata.Version,
		Images:  []*docker.ImageArchive{},
		Size:    0,
	}

	if ch.Values == nil {
		return ret, nil
	}

	// Nested image paths: navigate through the values map hierarchy
	// Structure from values.yaml:
	//  images:
	//    livenessprobesidecar: registry.k8s.io/sig-storage/livenessprobe:v2.16.0
	//    attachersidecar: registry.k8s.io/sig-storage/csi-attacher:v4.9.0
	//    provisionersidecar: registry.k8s.io/sig-storage/csi-provisioner:v5.3.0
	//    registrarsidecar: registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.14.0
	//    resizersidecar: registry.k8s.io/sig-storage/csi-resizer:v1.14.0
	//    snapshottersidecar: registry.k8s.io/sig-storage/csi-snapshotter:v8.3.0
	//    csidriver: quay.io/weka.io/csi-wekafs
	//    csidriverTag: <version>

	// Sidecar images
	sidecarPaths := map[string]string{
		"images.livenessprobesidecar": "CSI liveness probe sidecar",
		"images.attachersidecar":      "CSI attacher sidecar",
		"images.provisionersidecar":   "CSI provisioner sidecar",
		"images.registrarsidecar":     "CSI registrar sidecar",
		"images.resizersidecar":       "CSI resizer sidecar",
		"images.snapshottersidecar":   "CSI snapshotter sidecar",
	}

	for pathKey, description := range sidecarPaths {
		if img := helm.GetNestedValue(ch.Values, pathKey); img != "" {
			logger.Debug("Downloading CSI sidecar image", "path", pathKey, "description", description, "image", img)
			archive, err := docker.Download(ctx, img, "", b.opts.Archs, "linux", auth)
			if err != nil {
				logger.Warn("failed to download sidecar image", "path", pathKey, "image", img, "error", err)
				continue
			}
			ret.Images = append(ret.Images, archive)
			ret.Size += archive.Size
		}
	}

	// CSI driver main image: built from images.csidriver + images.csidriverTag
	if driver := helm.GetNestedValue(ch.Values, "images.csidriver"); driver != "" {
		tag := helm.GetNestedValue(ch.Values, "images.csidriverTag")
		if tag == "" {
			tag = ch.Metadata.AppVersion
		}
		if tag == "" {
			tag = ch.Metadata.Version
		}

		fullImage := driver + ":v" + tag
		logger.Debug("Downloading CSI driver image", "driver", driver, "tag", tag, "full_image", fullImage)
		archive, err := docker.Download(ctx, fullImage, "", b.opts.Archs, "linux", auth)
		if err != nil {
			logger.Warn("failed to download CSI driver image", "image", fullImage, "error", err)
		} else {
			ret.Images = append(ret.Images, archive)
			ret.Size += archive.Size
		}
	}

	return ret, nil
}

func (b *Bundle) Download(ctx context.Context) error {
	logger := logging.GetLogger(ctx)
	// WEKA Operator - Helm chart + images
	if err := b.processOperator(ctx); err != nil {
		logger.Error("failed to package Operator Helm chart", "error", err)
		b.AddFailed("weka-operator")
	}

	// WEKA CSI Driver - Helm chart + images
	if err := b.processCSI(ctx); err != nil {
		logger.Error("failed to package CSI driver Helm chart", "error", err)
		b.AddFailed("csi-wekafsplugin")
	}

	// WEKA Software images - multi-arch images from quay.io/weka.io/weka-in-container based on version
	if err := b.downloadWekaComponent(ctx); err != nil {
		logger.Warn("failed to download WEKA software images", "error", err)
		b.AddFailed("weka")
	}
	if len(b.failed) > 0 {
		logger.Error("Failed to download airgapped components", "error", b.failed)
		return fmt.Errorf("airgapped components failed to download")
	}
	return nil
}

func (b *Bundle) updateChartPaths() {
	// NOW update the filenames in components and charts to reflect paths within tar archive
	for _, comp := range b.Components() {
		if comp != nil {
			for _, img := range comp.Images {
				if img != nil && img.Filename != "" {
					img.Filename = "images/" + filepath.Base(img.Filename)
				}
			}
		}
	}

	for _, chartData := range b.Charts() {
		if chartData != nil {
			if chartData.Filename != "" {
				chartData.Filename = "charts/" + filepath.Base(chartData.Filename)
			}
		}
	}
}

func (b *Bundle) addFileContent(path string, content []byte) error {
	if !b.isOpen() {
		return fmt.Errorf("bundle is not open")
	}
	err := b.tw.WriteFile(path, content)
	if err != nil {
		return err
	}
	b.processedSize += int64(len(content))
	progress.RenderProgress(b.processedSize, b.size, "compress", "Packaging "+filepath.Base(path))
	return nil
}

// Create creates the tar.gz bundle with manifest
// The bundle contains:
// - All downloaded image archives
// - All downloaded Helm chart archives
// - manifest.json describing the bundle contents
// returns size of the created bundle in bytes and error
func (b *Bundle) Create(ctx context.Context, bundleFile string) error {
	// perform sanity checks for the reqeust
	if b == nil {
		return fmt.Errorf("bundle is nil")
	}

	if bundleFile == "" {
		return fmt.Errorf("bundle file name must be specified")
	}

	if b.bundleFilename == bundleFile {
		return nil // TODO: fix this, probably should report error
	}
	if b.bundleFilename == "" {
		b.bundleFilename = bundleFile
	}
	logger := logging.GetLogger(ctx)

	// reset bundle size TODO: implement Reset()
	b.size = 0
	b.processedSize = 0
	b.bundleSize = 0

	b.bundleFilename = bundleFile

	b.Account()
	// Add estimate for manifest.json
	b.size += 10000

	// Create the output bundle file
	if err := b.createWriter(bundleFile); err != nil {
		return fmt.Errorf("create bundle writer: %w", err)
	}

	// Add all image archives to tar FIRST (before updating filenames)
	if err := b.addComponents(ctx); err != nil {
		return fmt.Errorf("add components: %w", err)
	}

	// Add all Helm charts to tar
	if err := b.addCharts(ctx); err != nil {
		return fmt.Errorf("add charts: %w", err)
	}
	// Update chart paths to create a valid manifest with local refs
	b.updateChartPaths()

	manifest := b.buildManifest()

	// Marshal manifest to JSON
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	// Add manifest.json to tar after updating paths to relative
	if err := b.addFileContent("manifest.json", manifestJSON); err != nil {
		return fmt.Errorf("write manifest to bundle: %w", err)
	}
	// close archive
	b.Close()
	b.Sign(ctx)

	logger.Info("BundleManifest created successfully", "path", bundleFile, "total_size", b.processedSize)
	b.PrintSummary(ctx)

	// create also a SHA256 for the whole file

	return nil
}

func (b *Bundle) buildManifest() *BundleManifest {
	// Build manifest with updated filenames
	componentsMap := make(map[string]*ComponentManifest)
	chartsMap := make(map[string]*helm.HelmChartArchive)

	for _, comp := range b.Components() {
		if comp != nil {
			componentsMap[comp.Name] = comp
		}
	}

	for _, chartData := range b.Charts() {
		if chartData != nil {
			chartsMap[chartData.Name] = chartData
		}
	}

	// Parse architecture string
	if len(b.opts.Archs) == 0 {
		b.opts.Archs = []string{"amd64", "arm64"}
	}

	// Create manifest
	manifest := &BundleManifest{
		Version:       "1.0",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Components:    componentsMap,
		HelmCharts:    chartsMap,
		Architectures: b.opts.Archs,
		TotalSize:     b.size,
	}
	return manifest
}

func (b *Bundle) addCharts(ctx context.Context) error {
	if !b.isOpen() {
		return fmt.Errorf("bundle is not open for writing")
	}

	logger := logging.GetLogger(ctx)
	for _, chartData := range b.Charts() {
		if chartData == nil || chartData.Filename == "" {
			continue
		}

		// Read chart archive file and add to tar
		targetPath := "charts/" + filepath.Base(chartData.Filename)
		if err := b.tw.AddFile(chartData.Filename, targetPath); err != nil {
			logger.Warn("failed to add chart archive to bundle", "file", chartData.Filename, "error", err)
			continue
		}
		b.processedSize += chartData.Size
		progress.RenderProgress(b.processedSize, b.size, "compress", "Packaging "+filepath.Base(chartData.Filename))
		logger.Debug("Added chart archive to bundle", "name", chartData.Filename, "size", chartData.Size)
	}
	return nil
}

func (b *Bundle) addComponents(ctx context.Context) error {
	if !b.isOpen() {
		return fmt.Errorf("bundle is not open for writing")
	}

	logger := logging.GetLogger(ctx)
	for _, comp := range b.Components() {
		if comp == nil {
			continue
		}
		for _, img := range comp.Images {
			if img == nil || img.Filename == "" {
				continue
			}

			// Read image archive file and add to tar with progress tracking
			targetPath := "images/" + filepath.Base(img.Filename)

			// Progress callback: update total progress as we copy file chunks
			// Capture the current processedSize for accurate progress calculation
			processedBeforeFile := b.processedSize
			progressCallback := func(copiedInFile int64) {
				// Total progress = already processed before this file + current file progress
				currentProgress := processedBeforeFile + copiedInFile
				progress.RenderProgress(currentProgress, b.size, "compress", "Packaging "+filepath.Base(img.Filename))
			}

			if err := b.tw.AddFileWithProgress(img.Filename, targetPath, progressCallback); err != nil {
				logger.Warn("failed to add image archive to bundle", "file", img.Filename, "error", err)
				continue
			}
			b.processedSize += img.Size
			logger.Debug("Added image archive to bundle", "name", filepath.Base(img.Filename), "size", img.Size)
		}
	}
	return nil
}

func (b *Bundle) processOperator(ctx context.Context) error {
	if b.opts == nil {
		return fmt.Errorf("no options provided")
	}
	logger := logging.GetLogger(ctx)
	if b.opts.OperatorChart == nil {
		logger.Info("No operator version supplied in options")
		return nil
	}

	logger.Info("Processing WEKA Operator")
	// Create temporary directory for Helm chart archive
	workDir, err := utils.MkdirTemp("", "helm-operator-*")
	if err != nil {
		return fmt.Errorf("create temp dir for operator chart: %w", err)
	}

	// Archive the Helm chart
	ch, err := helm.Archive(ctx, b.opts.OperatorChart, workDir, b.opts.OperatorHelmPath)
	if err != nil {
		return fmt.Errorf("failed to package Operator Helm chart: %w", err)
	}

	b.RegisterChart(ch)

	logger.Info("Processing WEKA Operator Docker images")
	comp, err := b.downloadOperatorImages(ctx, b.opts.OperatorChart, nil)
	if err != nil {
		return fmt.Errorf("failed to download operator images: %w", err)
	}
	b.RegisterComponent(comp)
	return nil
}

func (b *Bundle) processCSI(ctx context.Context) error {
	if b.opts == nil {
		return fmt.Errorf("no options provided")
	}
	logger := logging.GetLogger(ctx)
	if b.opts.CSIChart == nil {
		logger.Info("No standalone CSI driver version supplied in options")
		return nil
	}

	logger.Info("Processing WEKA CSI Driver")

	// Create temporary directory for Helm chart archive
	workDir, err := utils.MkdirTemp("", "helm-csi-*")
	if err != nil {
		return fmt.Errorf("create temp dir for CSI chart: %w", err)
	}

	// Archive the Helm chart
	ch, err := helm.Archive(ctx, b.opts.CSIChart, workDir, b.opts.CSIHelmPath)
	if err != nil {
		return fmt.Errorf("failed to package CSI Chart: %w", err)
	}

	b.RegisterChart(ch)

	logger.Info("Processing WEKA CSI Plugin Docker images", "path", workDir)
	comp, err := b.downloadCSIImages(ctx, b.opts.CSIChart, nil)
	if err != nil {
		return fmt.Errorf("failed to download CSI Images: %w", err)
	}
	b.RegisterComponent(comp)
	return nil
}
