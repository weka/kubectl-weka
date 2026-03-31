package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/printer"
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

	p, _ := printer.GetPrinterFromFlags(flagOutput, true, nil, false, 0, printer.TableStyleMinimal)
	output, err := getters.GenerateCSISecretsOutput(ctx, KubeClients, p)
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}
