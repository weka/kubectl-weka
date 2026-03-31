package cmd

import (
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/supportbundle"
	"github.com/weka/kubectl-weka/pkg/types"
)

var supportBundleClusterCmd = &cobra.Command{
	Use:   "cluster [CLUSTER_NAME]",
	Short: "Collect cluster-related resources and logs",
	Long: `Collects diagnostic information for Weka clusters including:
  - WekaCluster resources
  - WekaContainer resources and logs
  - Associated pods and their logs

If cluster name is not specified:
  - With -n: collects all clusters in the specified namespace
  - With --all-namespaces: collects all clusters
  - Otherwise: collects all clusters in the default namespace

If cluster name is specified:
  - Without -n or --all-namespaces: searches in default namespace
  - With -n: searches in specified namespace
  - --all-namespaces: searches across all namespaces`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSupportBundleCluster,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleClusterCmd)

	supportBundleClusterCmd.Flags().StringVarP(&flagOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleClusterCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "", "Namespace (defaults to current kubeconfig namespace)")
	supportBundleClusterCmd.Flags().BoolVarP(&flagAllNamespaces, "all-namespaces", "A", false, "Collect clusters from all namespaces")

	supportBundleClusterCmd.SilenceUsage = true
}

func runSupportBundleCluster(cmd *cobra.Command, args []string) error {
	_ = cmd
	var clusterName string
	if len(args) > 0 {
		clusterName = args[0]
	}
	return supportbundle.RunSupportBundleByMode(KubeClients, types.CollectionModeCluster, clusterName, flagNamespace, flagAllNamespaces, flagIncludeSensitive)
}
