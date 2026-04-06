package cmd

import (
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/completion"
	"github.com/weka/kubectl-weka/pkg/supportbundle"
	"github.com/weka/kubectl-weka/pkg/types"
)

var supportBundleK8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Collect Kubernetes preflight check results",
	Long: `Runs Kubernetes preflight checks and stores the results as log files.
This includes all cluster-level and node-level checks that would normally
be performed before deploying Weka resources.`,
	RunE: runSupportBundleK8s,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleK8sCmd)

	supportBundleK8sCmd.Flags().StringVarP(&flagOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleK8sCmd.Flags().StringVarP(&flagNodeSelector, "node-selector", "l", "", "Node selector for node-level checks (e.g., 'node-role=weka')")

	supportBundleK8sCmd.RegisterFlagCompletionFunc("node-selector", completionListNodeSelectors)
	supportBundleK8sCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.SuggestAllUnusedFlagsWithUsageForCompletion(cmd, args, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
	supportBundleK8sCmd.SilenceUsage = true
}

func runSupportBundleK8s(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args
	return supportbundle.RunSupportBundleByMode(KubeClients, types.CollectionModeK8s, "", flagNamespace, flagAllNamespaces, flagIncludeSensitive)
}
