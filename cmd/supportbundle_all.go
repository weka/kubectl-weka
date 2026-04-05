package cmd

import (
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/supportbundle"
	"github.com/weka/kubectl-weka/pkg/types"
)

var supportBundleAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Collect all support bundle information (operator, clusters, clients, CSI, k8s)",
	Long: `Collects comprehensive diagnostic information including:
  - Operator logs and resources
  - All WekaCluster resources and their logs
  - All WekaClient resources and their logs
  - CSI components
  - Kubernetes preflight checks
  NOTE: this collection automatically is performed in ALL namespaces
`,
	RunE: runSupportBundleAll,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleAllCmd)

	supportBundleAllCmd.Flags().StringVarP(&flagOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleAllCmd.Flags().BoolVarP(&flagAllNamespaces, "all-namespaces", "A", false, "Collect resources from all namespaces")
	supportBundleAllCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "", "Namespace (defaults to current kubeconfig namespace)")

	supportBundleAllCmd.SilenceUsage = true
}

func runSupportBundleAll(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args
	return supportbundle.RunSupportBundleByMode(KubeClients, types.CollectionModeAll, "", flagNamespace, flagAllNamespaces, flagIncludeSensitive)
}
