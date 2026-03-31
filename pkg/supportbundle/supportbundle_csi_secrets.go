package supportbundle

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	types2 "github.com/weka/kubectl-weka/pkg/types"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CSISecretsCollector collects CSI-related secrets from storage classes
type CSISecretsCollector struct{}

func (c *CSISecretsCollector) Name() string {
	return "CSI Secrets"
}

func (c *CSISecretsCollector) Start(ctx context.Context) {
	logger := GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	includeSensitive := getCollectSensitiveData(ctx)
	if includeSensitive {
		logger.Info("Will collect", "items", "CSI secrets (unredacted - sensitive data included)")
	} else {
		logger.Info("Will collect", "items", "CSI secrets (redacted)")
	}
}

func (c *CSISecretsCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := GetLogger(ctx)
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files_created", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files_created", len(result.FilesCreated), "warnings", len(result.Warnings))
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

func (c *CSISecretsCollector) Collect(ctx context.Context) CollectorResult {
	logger := GetLogger(ctx)
	bundlePath := getBundlePath(ctx)
	var filesCreated []string
	var warnings []string

	// Create secrets directory in the bundle under csi
	secretsDir := filepath.Join(bundlePath, "csi", "secrets")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		logger.Error("Failed to create secrets directory", "error", err)
		return CollectorResult{
			Status:       StatusFailure,
			FilesCreated: filesCreated,
			Error:        err,
		}
	}

	// Get all WEKA CSI drivers
	logger.Debug("Fetching WEKA CSI drivers")
	csiDrivers, err := getWekaCSIDrivers(ctx)
	if err != nil {
		logger.Error("Failed to fetch CSI drivers", "error", err)
		return CollectorResult{
			Status:       StatusFailure,
			FilesCreated: filesCreated,
			Error:        err,
		}
	}

	if len(csiDrivers) == 0 {
		logger.Info("No WEKA CSI drivers found")
		return CollectorResult{
			Status:       StatusSuccess,
			FilesCreated: filesCreated,
			Warnings:     []string{"No WEKA CSI drivers found"},
		}
	}

	logger.Debug("Found WEKA CSI drivers", "count", len(csiDrivers))

	// Build a map of CSI driver names
	driverMap := make(map[string]bool)
	driverNameMap := make(map[string]string) // maps driver name to driver object name for directory
	for _, driver := range csiDrivers {
		driverMap[driver.Name] = true
		driverNameMap[driver.Name] = driver.Name
	}

	// Get all storage classes that use WEKA CSI drivers
	logger.Debug("Fetching storage classes")
	storageClasses, err := getStorageClassesForCSIDrivers(ctx, driverMap)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to fetch storage classes: %v", err))
		logger.Warn("Failed to fetch storage classes", "error", err)
	}

	if len(storageClasses) == 0 {
		logger.Info("No storage classes found for WEKA CSI drivers")
		return CollectorResult{
			Status:       StatusSuccess,
			FilesCreated: filesCreated,
			Warnings:     []string{"No storage classes found for WEKA CSI drivers"},
		}
	}

	logger.Debug("Found storage classes", "count", len(storageClasses))

	// Collect secrets for each storage class
	var secretErrors []string
	for i := range storageClasses {
		sc := &storageClasses[i]
		logger.Debug("Processing storage class", "name", sc.Name)

		// Get secrets referenced in this storage class
		secretRefs := kubernetes.ExtractSecretReferencesFromStorageClass(sc)
		if len(secretRefs) == 0 {
			logger.Debug("No secrets found in storage class", "name", sc.Name)
			continue
		}

		logger.Debug("Found secrets in storage class", "name", sc.Name, "count", len(secretRefs))

		// Collect each secret
		for _, secretRef := range secretRefs {
			result := c.collectSecret(ctx, secretsDir, driverNameMap[sc.Provisioner], sc.Name, secretRef)
			if result.err != nil {
				secretErrors = append(secretErrors, result.err.Error())
				warnings = append(warnings, result.err.Error())
				logger.Warn("Failed to collect secret", "storage_class", sc.Name, "secret", secretRef.Name, "namespace", secretRef.Namespace, "error", result.err)
			} else {
				filesCreated = append(filesCreated, result.filesCreated...)
				if len(result.validationErrors) > 0 {
					for _, valErr := range result.validationErrors {
						secretErrors = append(secretErrors, valErr)
						warnings = append(warnings, valErr)
						logger.Warn("Secret validation error", "secret", secretRef.Name, "namespace", secretRef.Namespace, "error", valErr)
					}
				}
			}
		}
	}

	// Write any secret errors to a file
	if len(secretErrors) > 0 {
		errFile := filepath.Join(secretsDir, "errors.txt")
		errContent := strings.Join(secretErrors, "\n")
		if err := os.WriteFile(errFile, []byte(errContent), 0644); err != nil {
			logger.Warn("Failed to write secrets errors file", "error", err)
		} else {
			filesCreated = append(filesCreated, errFile)
			logger.Debug("Wrote secrets errors file", "file", errFile)
		}
	}

	// Determine overall status
	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	logger.Info("Collected secrets", "files", len(filesCreated), "warnings", len(warnings))

	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}
}

type secretResult struct {
	filesCreated     []string
	validationErrors []string
	err              error
}

func (c *CSISecretsCollector) collectSecret(ctx context.Context, baseDir, driverName, storageClassName string, secretRef types2.SecretReference) secretResult {
	logger := GetLogger(ctx)
	clients := getClients(ctx)
	result := secretResult{}

	// Get the secret from the cluster using controller-runtime client (cached)
	var secret corev1.Secret
	if err := clients.CRClient.Get(ctx, types.NamespacedName{Namespace: secretRef.Namespace, Name: secretRef.Name}, &secret); err != nil {
		result.err = fmt.Errorf("failed to get secret %s/%s: %w", secretRef.Namespace, secretRef.Name, err)
		return result
	}

	// Create directory structure: csi/secrets/drivername/storageClassName/secretName
	secretDir := filepath.Join(baseDir, driverName)
	if err := os.MkdirAll(secretDir, 0755); err != nil {
		result.err = fmt.Errorf("failed to create secret directory %s: %w", secretDir, err)
		return result
	}

	// Validate and process secret data
	secretFile := filepath.Join(secretDir, fmt.Sprintf("Secret_%s-%s.yaml", kubernetes.SanitizeName(secretRef.Namespace), kubernetes.SanitizeName(secretRef.Name)))
	var content string
	var validationErrors []string
	includeSensitive := getCollectSensitiveData(ctx)
	if includeSensitive {
		// Include full secret data
		content = formatSecretContent(&secret, false)
		logger.Debug("Collected secret (unredacted)", "secret", secretRef.Name, "namespace", secretRef.Namespace, "file", secretFile)
	} else {
		// Redact sensitive data
		content = formatSecretContent(&secret, true)
		logger.Debug("Collected secret (redacted)", "secret", secretRef.Name, "namespace", secretRef.Namespace, "file", secretFile)
	}

	// Validate secret content
	validationErrors = kubernetes.ValidateSecretContent(&secret)
	if len(validationErrors) > 0 {
		result.validationErrors = validationErrors
	}

	// Write secret to file
	if err := os.WriteFile(secretFile, []byte(content), 0600); err != nil {
		result.err = fmt.Errorf("failed to write secret file %s: %w", secretFile, err)
		return result
	}

	result.filesCreated = append(result.filesCreated, secretFile)
	return result
}

// getWekaCSIDrivers returns all WEKA CSI drivers
func getWekaCSIDrivers(ctx context.Context) ([]storagev1.CSIDriver, error) {
	clients := getClients(ctx)

	var csiDriverList storagev1.CSIDriverList
	if err := clients.CRClient.List(ctx, &csiDriverList); err != nil {
		return nil, fmt.Errorf("failed to list CSI drivers: %w", err)
	}

	var wekaCsiDrivers []storagev1.CSIDriver
	for _, driver := range csiDriverList.Items {
		if kubernetes.IsWekaCSI(driver.Name) {
			wekaCsiDrivers = append(wekaCsiDrivers, driver)
		}
	}

	return wekaCsiDrivers, nil
}

// getStorageClassesForCSIDrivers returns all storage classes that use WEKA CSI drivers
func getStorageClassesForCSIDrivers(ctx context.Context, driverMap map[string]bool) ([]storagev1.StorageClass, error) {
	clients := getClients(ctx)

	var scList storagev1.StorageClassList
	if err := clients.CRClient.List(ctx, &scList); err != nil {
		return nil, fmt.Errorf("failed to list storage classes: %w", err)
	}

	var wekaStorageClasses []storagev1.StorageClass
	for _, sc := range scList.Items {
		if driverMap[sc.Provisioner] {
			wekaStorageClasses = append(wekaStorageClasses, sc)
		}
	}

	return wekaStorageClasses, nil
}

// formatSecretContent formats secret content for display
func formatSecretContent(secret *corev1.Secret, redact bool) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("Name: %s", secret.Name))
	lines = append(lines, fmt.Sprintf("Namespace: %s", secret.Namespace))
	lines = append(lines, fmt.Sprintf("Type: %s", secret.Type))
	lines = append(lines, "")
	lines = append(lines, "Data:")

	for key, value := range secret.Data {
		if redact {
			lines = append(lines, fmt.Sprintf("  %s: [REDACTED]", key))
		} else {
			lines = append(lines, fmt.Sprintf("  %s: %s", key, string(value)))
		}
	}

	return strings.Join(lines, "\n")
}
