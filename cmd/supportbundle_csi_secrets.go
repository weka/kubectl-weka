package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CSISecretsCollector collects CSI-related secrets from storage classes
type CSISecretsCollector struct{}

func (c *CSISecretsCollector) Name() string {
	return "CSI Secrets"
}

func (c *CSISecretsCollector) Start(ctx context.Context) {
	logger := getLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	if supportBundleIncludeSensitive {
		logger.Info("Will collect", "items", "CSI secrets (unredacted - sensitive data included)")
	} else {
		logger.Info("Will collect", "items", "CSI secrets (redacted)")
	}
}

func (c *CSISecretsCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := getLogger(ctx)
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
	logger := getLogger(ctx)
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
		secretRefs := extractSecretReferencesFromStorageClass(sc)
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

func (c *CSISecretsCollector) collectSecret(ctx context.Context, baseDir, driverName, storageClassName string, secretRef SecretReference) secretResult {
	logger := getLogger(ctx)
	result := secretResult{}

	// Get the secret from the cluster
	secret, err := KubeClients.Clientset.CoreV1().Secrets(secretRef.Namespace).Get(ctx, secretRef.Name, metav1.GetOptions{})
	if err != nil {
		result.err = fmt.Errorf("failed to get secret %s/%s: %w", secretRef.Namespace, secretRef.Name, err)
		return result
	}

	// Create directory structure: csi/secrets/drivername/storageClassName/secretName
	secretDir := filepath.Join(baseDir, driverName, storageClassName, secretRef.Name)
	if err := os.MkdirAll(secretDir, 0755); err != nil {
		result.err = fmt.Errorf("failed to create secret directory %s: %w", secretDir, err)
		return result
	}

	// Validate and process secret data
	secretFile := filepath.Join(secretDir, "secret.txt")
	var content string
	var validationErrors []string

	if supportBundleIncludeSensitive {
		// Include full secret data
		content = formatSecretContent(secret, false)
		logger.Debug("Collected secret (unredacted)", "secret", secretRef.Name, "namespace", secretRef.Namespace, "file", secretFile)
	} else {
		// Redact sensitive data
		content = formatSecretContent(secret, true)
		logger.Debug("Collected secret (redacted)", "secret", secretRef.Name, "namespace", secretRef.Namespace, "file", secretFile)
	}

	// Validate secret content
	validationErrors = validateSecretContent(secret)
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

// SecretReference represents a reference to a secret
type SecretReference struct {
	Name      string
	Namespace string
}

// getWekaCSIDrivers returns all WEKA CSI drivers
func getWekaCSIDrivers(ctx context.Context) ([]storagev1.CSIDriver, error) {
	crClient := KubeClients.CRClient

	var csiDriverList storagev1.CSIDriverList
	if err := crClient.List(ctx, &csiDriverList); err != nil {
		return nil, fmt.Errorf("failed to list CSI drivers: %w", err)
	}

	var wekaCsiDrivers []storagev1.CSIDriver
	for _, driver := range csiDriverList.Items {
		if isWekaCSI(driver.Name) {
			wekaCsiDrivers = append(wekaCsiDrivers, driver)
		}
	}

	return wekaCsiDrivers, nil
}

// getStorageClassesForCSIDrivers returns all storage classes that use WEKA CSI drivers
func getStorageClassesForCSIDrivers(ctx context.Context, driverMap map[string]bool) ([]storagev1.StorageClass, error) {
	crClient := KubeClients.CRClient

	var scList storagev1.StorageClassList
	if err := crClient.List(ctx, &scList); err != nil {
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

// extractSecretReferencesFromStorageClass extracts all secret references from a storage class
// Returns secrets with their explicit namespaces from storage class parameters.
// StorageClasses are cluster-wide resources and don't have a namespace themselves,
// so the secret namespace MUST be explicitly specified in the parameters.
func extractSecretReferencesFromStorageClass(sc *storagev1.StorageClass) []SecretReference {
	var secretRefs []SecretReference

	if sc.Parameters == nil {
		return secretRefs
	}

	// Common secret reference parameter names in storage classes
	secretParams := []string{
		"csi.storage.k8s.io/provisioner-secret-name",
		"csi.storage.k8s.io/provisioner-secret-namespace",
		"csi.storage.k8s.io/controller-expand-secret-name",
		"csi.storage.k8s.io/controller-expand-secret-namespace",
		"csi.storage.k8s.io/controller-publish-secret-name",
		"csi.storage.k8s.io/controller-publish-secret-namespace",
		"csi.storage.k8s.io/node-stage-secret-name",
		"csi.storage.k8s.io/node-stage-secret-namespace",
		"csi.storage.k8s.io/node-publish-secret-name",
		"csi.storage.k8s.io/node-publish-secret-namespace",
	}

	// Extract secret names and their corresponding namespaces
	secretMap := make(map[string]string) // secretName -> namespace

	for _, param := range secretParams {
		if strings.Contains(param, "-secret-name") {
			secretName := sc.Parameters[param]
			if secretName != "" {
				// Find corresponding namespace parameter
				namespaceParam := strings.Replace(param, "-secret-name", "-secret-namespace", 1)
				namespace := sc.Parameters[namespaceParam]

				// If namespace is not explicitly specified, it's an error - we cannot assume default
				// because secrets could be in any namespace
				if namespace == "" {
					// Skip secrets without explicit namespace specification
					continue
				}

				secretMap[secretName] = namespace
			}
		}
	}

	// Convert map to slice
	for secretName, namespace := range secretMap {
		secretRefs = append(secretRefs, SecretReference{
			Name:      secretName,
			Namespace: namespace,
		})
	}

	return secretRefs
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

// validateSecretContent validates that a secret contains required CSI parameters
func validateSecretContent(secret *corev1.Secret) []string {
	var errors []string

	requiredParams := []string{"username", "password", "organization", "endpoints", "scheme"}

	for _, param := range requiredParams {
		// Check if parameter exists in secret data
		found := false
		for key := range secret.Data {
			if key == param {
				found = true
				break
			}
		}

		if !found {
			errors = append(errors, fmt.Sprintf("Secret %s/%s is missing required parameter: %s", secret.Namespace, secret.Name, param))
		}
	}

	// Check for whitespace issues and scheme validity
	for key, value := range secret.Data {
		strValue := string(value)

		// Check for leading/trailing whitespace
		if strings.HasPrefix(strValue, " ") || strings.HasSuffix(strValue, " ") {
			errors = append(errors, fmt.Sprintf("Secret %s/%s parameter '%s' has leading or trailing whitespace", secret.Namespace, secret.Name, key))
		}

		// Validate scheme
		if key == "scheme" {
			if strValue != "http" && strValue != "https" {
				errors = append(errors, fmt.Sprintf("Secret %s/%s parameter 'scheme' has invalid value '%s' (must be 'http' or 'https')", secret.Namespace, secret.Name, strValue))
			}
		}
	}

	return errors
}
