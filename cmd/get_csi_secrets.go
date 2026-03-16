package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var getCSISecretsCmd = &cobra.Command{
	Use:   "csi-secrets",
	Short: "List and validate CSI-related secrets from storage classes",
	Long: `Lists all secrets referenced by WEKA CSI storage classes with validation status.

Shows:
  - Secret name and namespace
  - Number of storage classes referencing the secret
  - Validation status (VALID or FAILED)
  - Details of any validation errors

Validation checks for:
  - Required parameters: username, password, organization, endpoints, scheme
  - Scheme must be either 'http' or 'https'
  - No leading or trailing whitespace on parameters
`,
	RunE: runGetCSISecrets,
}

func init() {
	getCmd.AddCommand(getCSISecretsCmd)
	getCSISecretsCmd.SilenceUsage = true
}

func runGetCSISecrets(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	printer, _ := GetPrinterFromFlags(flagOutput, true, nil, false, 0, TableStyleMinimal)
	output, err := generateCSISecretsOutput(ctx, KubeClients, printer)
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}

// SecretInfo holds information about a CSI secret
type SecretInfo struct {
	Name              string
	Namespace         string
	StorageClassCount int
	Valid             bool
	ValidationErrors  []string
}

// generateCSISecretsOutput generates the CSI secrets table as a string
func generateCSISecretsOutput(ctx context.Context, clients *K8sClients, printer ResourcePrinter) (string, error) {
	crClient := clients.CRClient

	// Get all WEKA CSI drivers
	var csiDriverList storagev1.CSIDriverList
	if err := crClient.List(ctx, &csiDriverList); err != nil {
		return "", fmt.Errorf("failed to list CSI drivers: %w", err)
	}

	// Build a map of Weka CSI drivers
	driverMap := make(map[string]bool)
	for _, driver := range csiDriverList.Items {
		if isWekaCSI(driver.Name) {
			driverMap[driver.Name] = true
		}
	}

	if len(driverMap) == 0 {
		return "No CSI drivers found.\n", nil
	}

	// Get all storage classes that use WEKA CSI drivers
	var scList storagev1.StorageClassList
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
		secretRefs := extractSecretReferencesFromStorageClass(&sc)
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
		secret, err := clients.Clientset.CoreV1().Secrets(secretInfo.Namespace).Get(ctx, secretInfo.Name, metav1.GetOptions{})
		if err != nil {
			secretInfo.Valid = false
			secretInfo.ValidationErrors = []string{fmt.Sprintf("failed to get secret: %v", err)}
			continue
		}

		// Validate secret content
		validationErrors := validateSecretContent(secret)
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
	columns := []TableColumn{
		{Name: "NAME", VisibleInWide: false},
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "STORAGECLASS COUNT", VisibleInWide: false},
		{Name: "VALIDITY", VisibleInWide: false},
		{Name: "DETAIL", VisibleInWide: false},
	}

	// Build rows
	var rows []TableRow
	for _, secret := range secrets {
		row := TableRow{Values: map[string]interface{}{
			"NAME":               secret.Name,
			"NAMESPACE":          secret.Namespace,
			"STORAGECLASS COUNT": secret.StorageClassCount,
			"VALIDITY":           boolToOkError(secret.Valid),
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
	_ = printer.Print(columns, rows, &sb)
	return sb.String() + "\n", nil
}
