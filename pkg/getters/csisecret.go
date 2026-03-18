package getters

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/utils"
	"k8s.io/api/storage/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"
	"strings"
)

// SecretInfo holds information about a CSI secret
type SecretInfo struct {
	Name              string
	Namespace         string
	StorageClassCount int
	Valid             bool
	ValidationErrors  []string
}

// GenerateCSISecretsOutput generates the CSI secrets table as a string
func GenerateCSISecretsOutput(ctx context.Context, clients *kubernetes.K8sClients, printerObj printer.ResourcePrinter) (string, error) {
	crClient := clients.CRClient

	// Get all WEKA CSI drivers
	var csiDriverList v1.CSIDriverList
	if err := crClient.List(ctx, &csiDriverList); err != nil {
		return "", fmt.Errorf("failed to list CSI drivers: %w", err)
	}

	// Build a map of Weka CSI drivers
	driverMap := make(map[string]bool)
	for _, driver := range csiDriverList.Items {
		if kubernetes.IsWekaCSI(driver.Name) {
			driverMap[driver.Name] = true
		}
	}

	if len(driverMap) == 0 {
		return "No CSI drivers found.\n", nil
	}

	// Get all storage classes that use WEKA CSI drivers
	var scList v1.StorageClassList
	if err := crClient.List(ctx, &scList); err != nil {
		return "", fmt.Errorf("failed to list storage classes: %w", err)
	}

	// Extract all secrets and their references
	secretMap := make(map[string]*SecretInfo) // "namespace/name" -> SecretInfo
	scCountMap := make(map[string]int)        // "namespace/name" -> storage class count

	for _, sc := range scList.Items {
		if !driverMap[sc.Provisioner] {
			continue
		}

		// Extract secrets from this storage class
		secretRefs := kubernetes.ExtractSecretReferencesFromStorageClass(&sc)
		for _, secretRef := range secretRefs {
			key := secretRef.Namespace + "/" + secretRef.Name
			scCountMap[key]++

			// Create entry if not exists
			if _, exists := secretMap[key]; !exists {
				secretMap[key] = &SecretInfo{
					Name:      secretRef.Name,
					Namespace: secretRef.Namespace,
				}
			}
		}
	}

	if len(secretMap) == 0 {
		return "No CSI secrets found.\n", nil
	}

	// Validate each secret
	for key, secretInfo := range secretMap {
		secretInfo.StorageClassCount = scCountMap[key]

		// Get and validate the secret
		secret, err := clients.Clientset.CoreV1().Secrets(secretInfo.Namespace).Get(ctx, secretInfo.Name, v2.GetOptions{})
		if err != nil {
			secretInfo.Valid = false
			secretInfo.ValidationErrors = []string{fmt.Sprintf("failed to get secret: %v", err)}
			continue
		}

		// Validate secret content
		validationErrors := kubernetes.ValidateSecretContent(secret)
		if len(validationErrors) == 0 {
			secretInfo.Valid = true
		} else {
			secretInfo.Valid = false
			secretInfo.ValidationErrors = validationErrors
		}
	}

	// Convert to slice and sort
	var secrets []*SecretInfo
	for _, secretInfo := range secretMap {
		secrets = append(secrets, secretInfo)
	}

	sort.Slice(secrets, func(i, j int) bool {
		if secrets[i].Namespace != secrets[j].Namespace {
			return secrets[i].Namespace < secrets[j].Namespace
		}
		return secrets[i].Name < secrets[j].Name
	})

	// Define columns
	columns := []printer.TableColumn{
		{Name: "NAME", VisibleInWide: false},
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "STORAGECLASS COUNT", VisibleInWide: false},
		{Name: "VALIDITY", VisibleInWide: false},
		{Name: "DETAIL", VisibleInWide: false},
	}

	// Build rows
	var rows []printer.TableRow
	for _, secret := range secrets {
		row := printer.TableRow{Values: map[string]interface{}{
			"NAME":               secret.Name,
			"NAMESPACE":          secret.Namespace,
			"STORAGECLASS COUNT": secret.StorageClassCount,
			"VALIDITY":           utils.BoolToOkError(secret.Valid),
			"DETAIL": func() string {
				if len(secret.ValidationErrors) > 0 {
					return secret.ValidationErrors[0]
				}
				return ""
			}(),
		}}
		rows = append(rows, row)
	}

	// Render output
	var sb strings.Builder
	_ = printerObj.Print(columns, rows, &sb)
	return sb.String(), nil
}
