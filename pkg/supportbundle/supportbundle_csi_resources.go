package supportbundle

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// StorageClassCollector collects StorageClasses with weka.io CSI driver
type StorageClassCollector struct{}

func (c *StorageClassCollector) Name() string {
	return "CSI StorageClasses"
}

func (c *StorageClassCollector) Start(ctx context.Context) {
	logger := GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "StorageClasses with weka.io CSI driver")
}

func (c *StorageClassCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := GetLogger(ctx)
	clients := getClients(ctx)
	bundlePath := getBundlePath(ctx)

	logger.Debug("Collecting StorageClasses with weka.io CSI driver")

	// Get all StorageClasses
	var scList storagev1.StorageClassList
	if err := clients.CRClient.List(ctx, &scList); err != nil {
		return CollectorResult{
			Status: StatusFailure,
			Error:  fmt.Errorf("failed to list StorageClasses: %w", err),
		}
	}

	// Filter for weka.io CSI driver
	var wekaStorageClasses []storagev1.StorageClass
	for _, sc := range scList.Items {
		if kubernetes.IsWekaCSI(sc.Provisioner) {
			wekaStorageClasses = append(wekaStorageClasses, sc)
		}
	}

	logger.Debug("Found StorageClasses with weka.io CSI driver", "count", len(wekaStorageClasses))

	// Check if we exceeded the limit
	if len(wekaStorageClasses) > 100 {
		warning := fmt.Sprintf("Found %d StorageClasses with weka.io CSI driver, collecting only first 100", len(wekaStorageClasses))
		warnings = append(warnings, warning)
		logger.Debug("⚠️  " + warning)
		wekaStorageClasses = wekaStorageClasses[:100]
	}

	// Collect each StorageClass
	for _, sc := range wekaStorageClasses {
		yamlData, err := yaml.Marshal(&sc)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to marshal StorageClass %s: %v", sc.Name, err))
			logger.Debug("⚠️  Failed to marshal StorageClass", "name", sc.Name, "error", err)
			continue
		}

		filePath := filepath.Join("csi", "storageclasses", GenerateSafeFileName("StorageClass", "", sc.Name, "yaml"))
		if err := WriteToFile(bundlePath, filePath, string(yamlData)); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write StorageClass file for %s: %v", sc.Name, err))
			logger.Debug("⚠️  Failed to write StorageClass file", "name", sc.Name, "error", err)
			continue
		}
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected StorageClass", "name", sc.Name)
	}

	// Determine status
	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}
}

func (c *StorageClassCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := GetLogger(ctx)

	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files", len(result.FilesCreated))
		if len(result.Warnings) > 0 {
			logger.Info("Non-fatal warnings found", "count", len(result.Warnings))
			for _, warning := range result.Warnings {
				logger.Info("Warning", "message", warning)
			}
		}
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

// PersistentVolumeClaimCollector collects PVCs with weka.io CSI driver
type PersistentVolumeClaimCollector struct{}

func (c *PersistentVolumeClaimCollector) Name() string {
	return "CSI PersistentVolumeClaims"
}

func (c *PersistentVolumeClaimCollector) Start(ctx context.Context) {
	logger := GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "PersistentVolumeClaims with weka.io CSI driver")
}

func (c *PersistentVolumeClaimCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := GetLogger(ctx)
	clients := getClients(ctx)
	bundlePath := getBundlePath(ctx)

	logger.Debug("Collecting PersistentVolumeClaims with weka.io CSI driver")

	// Get all PVCs across all namespaces
	var pvcList corev1.PersistentVolumeClaimList
	if err := clients.CRClient.List(ctx, &pvcList); err != nil {
		return CollectorResult{
			Status: StatusFailure,
			Error:  fmt.Errorf("failed to list PersistentVolumeClaims: %w", err),
		}
	}

	// Filter for weka.io CSI driver - check StorageClassName
	var wekaPVCs []corev1.PersistentVolumeClaim
	scNameToDriver := make(map[string]bool) // Cache for StorageClass drivers

	for _, pvc := range pvcList.Items {
		scName := pvc.Spec.StorageClassName
		if scName == nil || *scName == "" {
			continue
		}

		// Check cache first
		if cached, ok := scNameToDriver[*scName]; ok {
			if cached {
				wekaPVCs = append(wekaPVCs, pvc)
			}
			continue
		}

		// Fetch StorageClass to check driver
		var sc storagev1.StorageClass
		if err := clients.CRClient.Get(ctx, client.ObjectKey{Name: *scName}, &sc); err != nil {
			// If we can't find the StorageClass, skip this PVC
			scNameToDriver[*scName] = false
			continue
		}

		isWeka := kubernetes.IsWekaCSI(sc.Provisioner)
		scNameToDriver[*scName] = isWeka
		if isWeka {
			wekaPVCs = append(wekaPVCs, pvc)
		}
	}

	logger.Debug("Found PersistentVolumeClaims with weka.io CSI driver", "count", len(wekaPVCs))

	// Check if we exceeded the limit
	if len(wekaPVCs) > 100 {
		warning := fmt.Sprintf("Found %d PersistentVolumeClaims with weka.io CSI driver, collecting only first 100", len(wekaPVCs))
		warnings = append(warnings, warning)
		logger.Debug("⚠️  " + warning)
		wekaPVCs = wekaPVCs[:100]
	}

	// Collect each PVC
	for _, pvc := range wekaPVCs {
		yamlData, err := yaml.Marshal(&pvc)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to marshal PVC %s/%s: %v", pvc.Namespace, pvc.Name, err))
			logger.Debug("⚠️  Failed to marshal PVC", "namespace", pvc.Namespace, "name", pvc.Name, "error", err)
			continue
		}

		filePath := filepath.Join("csi", "pvcs", GenerateSafeFileName("PVC", pvc.Namespace, pvc.Name, "yaml"))
		if err := WriteToFile(bundlePath, filePath, string(yamlData)); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write PVC file for %s/%s: %v", pvc.Namespace, pvc.Name, err))
			logger.Debug("⚠️  Failed to write PVC file", "namespace", pvc.Namespace, "name", pvc.Name, "error", err)
			continue
		}
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected PVC", "namespace", pvc.Namespace, "name", pvc.Name)
	}

	// Determine status
	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}
}

func (c *PersistentVolumeClaimCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := GetLogger(ctx)

	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files", len(result.FilesCreated))
		if len(result.Warnings) > 0 {
			logger.Info("Non-fatal warnings found", "count", len(result.Warnings))
			for _, warning := range result.Warnings {
				logger.Info("Warning", "message", warning)
			}
		}
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

// PersistentVolumeCollector collects PVs with weka.io CSI driver
type PersistentVolumeCollector struct{}

func (c *PersistentVolumeCollector) Name() string {
	return "CSI PersistentVolumes"
}

func (c *PersistentVolumeCollector) Start(ctx context.Context) {
	logger := GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "PersistentVolumes with weka.io CSI driver")
}

func (c *PersistentVolumeCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := GetLogger(ctx)
	clients := getClients(ctx)
	bundlePath := getBundlePath(ctx)

	logger.Debug("Collecting PersistentVolumes with weka.io CSI driver")

	// Get all PVs
	var pvList corev1.PersistentVolumeList
	if err := clients.CRClient.List(ctx, &pvList); err != nil {
		return CollectorResult{
			Status: StatusFailure,
			Error:  fmt.Errorf("failed to list PersistentVolumes: %w", err),
		}
	}

	// Filter for weka.io CSI driver
	var wekaPVs []corev1.PersistentVolume
	for _, pv := range pvList.Items {
		if pv.Spec.CSI != nil && kubernetes.IsWekaCSI(pv.Spec.CSI.Driver) {
			wekaPVs = append(wekaPVs, pv)
		}
	}

	logger.Debug("Found PersistentVolumes with weka.io CSI driver", "count", len(wekaPVs))

	// Check if we exceeded the limit
	if len(wekaPVs) > 100 {
		warning := fmt.Sprintf("Found %d PersistentVolumes with weka.io CSI driver, collecting only first 100", len(wekaPVs))
		warnings = append(warnings, warning)
		logger.Debug("⚠️  " + warning)
		wekaPVs = wekaPVs[:100]
	}

	// Collect each PV
	for _, pv := range wekaPVs {
		yamlData, err := yaml.Marshal(&pv)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to marshal PV %s: %v", pv.Name, err))
			logger.Debug("⚠️  Failed to marshal PV", "name", pv.Name, "error", err)
			continue
		}

		filePath := filepath.Join("csi", "persistentvolumes", GenerateSafeFileName("PV", "", pv.Name, "yaml"))
		if err := WriteToFile(bundlePath, filePath, string(yamlData)); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write PV file for %s: %v", pv.Name, err))
			logger.Debug("⚠️  Failed to write PV file", "name", pv.Name, "error", err)
			continue
		}
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected PersistentVolume", "name", pv.Name)
	}

	// Determine status
	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}
}

func (c *PersistentVolumeCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := GetLogger(ctx)

	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files", len(result.FilesCreated))
		if len(result.Warnings) > 0 {
			logger.Info("Non-fatal warnings found", "count", len(result.Warnings))
			for _, warning := range result.Warnings {
				logger.Info("Warning", "message", warning)
			}
		}
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

// CSIDriverCollector collects CSIDriver resources with weka.io in the name
type CSIDriverCollector struct{}

func (c *CSIDriverCollector) Name() string {
	return "CSI Drivers"
}

func (c *CSIDriverCollector) Start(ctx context.Context) {
	logger := GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "CSIDriver resources with weka.io in the name")
}

func (c *CSIDriverCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := GetLogger(ctx)
	clients := getClients(ctx)
	bundlePath := getBundlePath(ctx)

	logger.Debug("Collecting CSIDriver resources with weka.io in the name")

	// Get all CSIDrivers
	var driverList storagev1.CSIDriverList
	if err := clients.CRClient.List(ctx, &driverList); err != nil {
		return CollectorResult{
			Status: StatusFailure,
			Error:  fmt.Errorf("failed to list CSIDrivers: %w", err),
		}
	}

	// Filter for weka.io in the name
	var wekaDrivers []storagev1.CSIDriver
	for _, driver := range driverList.Items {
		if kubernetes.IsWekaCSI(driver.Name) {
			wekaDrivers = append(wekaDrivers, driver)
		}
	}

	logger.Debug("Found WEKA CSIDriver resources", "count", len(wekaDrivers))

	// Check if we exceeded the limit
	if len(wekaDrivers) > 100 {
		warning := fmt.Sprintf("Found %d WEKA CSIDriver resources, collecting only first 100", len(wekaDrivers))
		warnings = append(warnings, warning)
		logger.Debug("⚠️  " + warning)
		wekaDrivers = wekaDrivers[:100]
	}

	// Collect each CSIDriver
	for _, driver := range wekaDrivers {
		yamlData, err := yaml.Marshal(&driver)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to marshal CSIDriver %s: %v", driver.Name, err))
			logger.Debug("⚠️  Failed to marshal CSIDriver", "name", driver.Name, "error", err)
			continue
		}

		filePath := filepath.Join("csi", "csidrivers", GenerateSafeFileName("CSIDriver", "", driver.Name, "yaml"))
		if err := WriteToFile(bundlePath, filePath, string(yamlData)); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write CSIDriver file for %s: %v", driver.Name, err))
			logger.Debug("⚠️  Failed to write CSIDriver file", "name", driver.Name, "error", err)
			continue
		}
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected CSIDriver", "name", driver.Name)
	}

	// Determine status
	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}
}

func (c *CSIDriverCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := GetLogger(ctx)

	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files", len(result.FilesCreated))
		if len(result.Warnings) > 0 {
			logger.Info("Non-fatal warnings found", "count", len(result.Warnings))
			for _, warning := range result.Warnings {
				logger.Info("Warning", "message", warning)
			}
		}
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}
