package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/printer"
)

var (
	flagUnhealthy bool
)

var getCSIInstancesCmd = &cobra.Command{
	Use:   "csi-instances [DRIVER_NAME]",
	Short: "List CSI driver pods (controller and node instances)",
	Long: `Lists CSI driver pods showing deployment status, restart counts, and pod information.

Arguments:
  DRIVER_NAME      Optional: Show only a specific CSI driver by name

Flags:
  -n, --namespace <string>  Filter by Kubernetes namespace (shows all namespaces if not set)
  -r, --role <string>       Filter by pod role: 'controller' or 'node' (shows both if not set)
  -o, --output <string>     Output format. Supported: json, yaml, wide, custom-columns=<COLS...>
  --unhealthy              Show only pods with frequent restarts (>1 restart in last 5 minutes)

Output Columns (default):
  CSI DRIVER     - CSI driver name
  NAMESPACE      - Kubernetes namespace where pod is deployed
  NODE           - Kubernetes node where pod is running
  ROLE           - Pod role: 'controller' or 'node'
  POD NAME       - Name of the CSI pod
  STATUS         - Pod status from container state (Running, CrashLoopBackoff, ImagePullBackOff, etc.)
  RESTARTS       - Number of pod container restarts
  AGE            - Time since pod was created

Wide columns (--wide):
  LAST RESTART   - Time since last pod container restart (N/A if never restarted)
`,
	RunE: runGetCSIInstances,
}

func init() {
	getCmd.AddCommand(getCSIInstancesCmd)

	getCSIInstancesCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "", "Filter by Kubernetes namespace")
	getCSIInstancesCmd.Flags().StringVarP(&flagRole, "role", "r", "", "Filter by pod role (controller or node)")
	getCSIInstancesCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")
	getCSIInstancesCmd.Flags().BoolVar(&flagUnhealthy, "unhealthy", false, "Show only pods with frequent restarts (>1 restart in last 5 minutes)")

	getCSIInstancesCmd.RegisterFlagCompletionFunc("namespace", completionListNamespaces)
	getCSIInstancesCmd.RegisterFlagCompletionFunc("output", completionGetCsiInstancesOutput)
	getCSIInstancesCmd.RegisterFlagCompletionFunc("role", completionGetCsiInstancesRoles)
	getCSIInstancesCmd.ValidArgsFunction = completionListWekaCsiDriversAsArgs

	getCSIInstancesCmd.SilenceUsage = true
}

func runGetCSIInstances(_ *cobra.Command, args []string) error {
	ctx := context.Background()

	// Extract optional driver name argument
	var driverName string
	if len(args) > 0 {
		driverName = args[0]
	}

	// Validate role filter if provided
	if flagRole != "" && flagRole != "controller" && flagRole != "node" {
		return fmt.Errorf("invalid role: must be 'controller' or 'node'")
	}

	p, _ := printer.GetPrinterFromFlags(flagOutput, true, nil, false, 0, printer.TableStyleMinimal)
	output, err := getters.GenerateCSIInstancesOutput(ctx, KubeClients, driverName, flagNamespace, flagRole, flagUnhealthy, p)
	if err != nil {
		return err
	}

	// Print the output
	fmt.Print(output)
	return nil
}
