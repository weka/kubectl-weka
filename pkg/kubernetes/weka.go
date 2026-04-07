package kubernetes

import (
	"fmt"
	"strings"

	"github.com/weka/kubectl-weka/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/storage/v1"
)

// IsWekaCSI checks if a string contains "weka.io" (for provisioners, drivers, or names)
func IsWekaCSI(s string) bool {
	if s == "" || len(s) < len("weka.io") {
		return false
	}
	// Check if "weka.io" appears anywhere in the string
	for i := 0; i < len(s)-len("weka.io")+1; i++ {
		if s[i:i+len("weka.io")] == "weka.io" {
			return true
		}
	}
	return false
}

// ExtractSecretReferencesFromStorageClass extracts all secret references from a storage class
// Returns secrets with their explicit namespaces from storage class parameters.
// StorageClasses are cluster-wide resources and don't have a namespace themselves,
// so the secret namespace MUST be explicitly specified in the parameters.
func ExtractSecretReferencesFromStorageClass(sc *v1.StorageClass) []types.SecretReference {
	var secretRefs []types.SecretReference

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
		secretRefs = append(secretRefs, types.SecretReference{
			Name:      secretName,
			Namespace: namespace,
		})
	}

	return secretRefs
}

// ValidateSecretContent validates that a secret contains required CSI parameters
func ValidateSecretContent(secret *corev1.Secret) []string {
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
