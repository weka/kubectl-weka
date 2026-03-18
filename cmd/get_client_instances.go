package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/printer"
)

var getClientInstancesCmd = &cobra.Command{
	Use:   "client-instances [WEKACLIENT]",
	Short: "Display WEKA client instances and status (derived from WekaClient configuration)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGetClientInstances,
}

func init() {
	getCmd.AddCommand(getClientInstancesCmd)

	getClientInstancesCmd.Flags().BoolVarP(&flagAllNamespaces, "all-namespaces", "A", false, "If present, list WekaClient resources across all namespaces")
	getClientInstancesCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "", "Namespace. Defaults to current kubeconfig namespace")
	getClientInstancesCmd.Flags().BoolVar(&flagNoHeaders, "no-headers", false, "Don't print headers")
	getClientInstancesCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")

	getClientInstancesCmd.SilenceUsage = true
}

func runGetClientInstances(_ *cobra.Command, args []string) error {
	ctx := context.Background()

	currentNS, _, err := kubernetes.GetNamespaceFromFlags(flagAllNamespaces, flagNamespace)
	if err != nil {
		return err
	}
	var targetName string
	if len(args) == 1 {
		targetName = args[0]
		if flagAllNamespaces {
			return fmt.Errorf("cannot use -A/--all-namespaces when specifying a WekaClient name; use -n to choose namespace")
		}
	}
	var hideColumnsList []string
	if !flagAllNamespaces {
		hideColumnsList = append(hideColumnsList, "NAMESPACE")
	}
	p, _ := printer.GetPrinterFromFlags(flagOutput, !flagNoHeaders, hideColumnsList, false, 0, printer.TableStyleMinimal)
	output, err := getters.GenerateClientInstancesOutput(
		ctx,
		KubeClients,
		currentNS,
		flagAllNamespaces,
		targetName,
		p,
	)
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}
