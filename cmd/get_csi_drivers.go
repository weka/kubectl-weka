package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/printer"
)

var (
	flagOnlyHelm     bool
	flagOnlyOperator bool
)

var getCSIDriversCmd = &cobra.Command{
	Use:   "csi-drivers [DRIVER_NAME]",
	Short: "Display CSI driver deployment information (controller and node components)",
	Long: `Lists CSI driver deployments with their controller and node components.

Arguments:
  DRIVER_NAME      Optional: Show only a specific CSI driver by name

Filters:
  --only-helm        Show only CSI drivers installed via Helm chart
  --only-operator    Show only CSI drivers installed by Weka operator
  --wide, -w         Show additional columns (PVs, PVCs, Bound PVs)

Columns (default):
  CSI DRIVER       - CSI driver name
  MANAGED BY       - Installation method (Helm or weka-operator)
  NAMESPACE        - Namespace where CSI components are deployed
  CONTROLLER       - Controller component deployment name
  NODE DAEMONSET   - Node component daemonset name
  STORAGECLASSES   - Number of StorageClasses that refer to this driver
  AGE              - Time since CSI driver was installed

Wide columns (--wide):
  PVS              - Total number of PersistentVolumes using this driver
  PVCS             - Total number of PersistentVolumeClaims using this driver
  BOUND PVS        - Number of PersistentVolumes in Bound state
`,
	RunE: runGetCSIDrivers,
}

func init() {
	getCmd.AddCommand(getCSIDriversCmd)

	getCSIDriversCmd.Flags().BoolVar(&flagOnlyHelm, "only-helm", false, "Only show CSI drivers installed via Helm chart")
	getCSIDriversCmd.Flags().BoolVar(&flagOnlyOperator, "only-operator", false, "Only show CSI drivers installed by Weka operator")
	getCSIDriversCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")
	getCSIDriversCmd.Flags().BoolVar(&flagNoHeaders, "no-headers", false, "Don't print headers")

	getCSIDriversCmd.ValidArgsFunction = completionListWekaCsiDriversAsArgs
	getCSIDriversCmd.RegisterFlagCompletionFunc("output", completionGetCsiDriversOutput)
	getCSIDriversCmd.SilenceUsage = true
}

func runGetCSIDrivers(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Extract optional driver name argument
	var driverName string
	if len(args) > 0 {
		driverName = args[0]
	}

	p, _ := printer.GetPrinterFromFlags(flagOutput, !flagNoHeaders, nil, false, 0, printer.TableStyleMinimal)
	driverOutput, err := getters.GenerateCSIDriversOutput(ctx, KubeClients, flagOnlyHelm, flagOnlyOperator, driverName, p)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), driverOutput)
	_, _ = fmt.Fprintln(cmd.OutOrStdout()) // Add newline after table
	return nil
}
