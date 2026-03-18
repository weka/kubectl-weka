package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/printer"
)

var getClusterInstancesCmd = &cobra.Command{
	Use:   "cluster-instances [WEKACLUSTER]",
	Short: "Show WEKA cluster instances (WekaContainers) and their status, optionally filtered by WekaCluster",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGetClusterInstances,
}

func init() {
	getCmd.AddCommand(getClusterInstancesCmd)

	getClusterInstancesCmd.Flags().BoolVarP(&flagAllNamespaces, "all-namespaces", "A", false, "If present, list WekaCluster resources across all namespaces")
	getClusterInstancesCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "", "Namespace. Defaults to current kubeconfig namespace")
	getClusterInstancesCmd.Flags().BoolVar(&flagNoHeaders, "no-headers", false, "Don't print headers")
	getClusterInstancesCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")

	getClusterInstancesCmd.SilenceUsage = true
}

func runGetClusterInstances(_ *cobra.Command, args []string) error {
	ctx := context.Background()

	currentNS, _, err := kubernetes.GetNamespaceFromFlags(flagAllNamespaces, flagNamespace)
	var targetCluster string

	p, _ := printer.GetPrinterFromFlags(flagOutput, !flagNoHeaders, nil, false, 0, printer.TableStyleMinimal)
	output, err := getters.GetClusterInstancesOutput(
		ctx,
		KubeClients,
		currentNS,
		flagAllNamespaces,
		targetCluster,
		p,
	)
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}
