package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/completion"
	"github.com/weka/kubectl-weka/pkg/preflight"
)

var preflightK8sClusterCmd = &cobra.Command{
	Use:   "cluster [NODE...]",
	Short: "Preflight cluster checks (platform, permissions, kubelet configuration)",
	Args:  cobra.ArbitraryArgs,
	RunE:  runPreflightK8sCluster,
}

func init() {
	preflightCmd.AddCommand(preflightK8sClusterCmd)
	preflightK8sClusterCmd.Flags().StringVar(&flagNodeSelector, "node-selector", "", "Label selector to filter nodes for node-scoped cluster checks")

	preflightK8sClusterCmd.RegisterFlagCompletionFunc("node-selector", completionListNodeSelectors)
	preflightK8sClusterCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.SuggestAllUnusedFlagsWithUsageForCompletion(cmd, args, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	preflightK8sClusterCmd.SilenceUsage = true

}

func runPreflightK8sCluster(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Create output that writes to stdout in real-time
	output := preflight.NewPreflightOutput(os.Stdout)
	defer output.Close()

	// Run the preflight checks
	result := preflight.GeneratePreflightK8sClusterOutput(ctx, KubeClients, args, flagNodeSelector, output)

	if result.Error != nil {
		return result.Error
	}

	if !result.Success {
		return fmt.Errorf("preflight cluster failed")
	}

	return nil
}
