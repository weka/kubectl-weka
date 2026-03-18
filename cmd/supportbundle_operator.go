package cmd

import (
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/supportbundle"
	"github.com/weka/kubectl-weka/pkg/types"
)

var supportBundleOperatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "Collect operator logs and resources",
	Long: `Collects diagnostic information for the Weka operator including:
  - Operator controller manager logs
  - Node-agent pod logs
  - WekaPolicy resources
  - Jobs created by policies`,
	RunE: runSupportBundleOperator,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleOperatorCmd)

	supportBundleOperatorCmd.Flags().StringVarP(&flagOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleOperatorCmd.Flags().StringVarP(&flagLogOperatorNamespace, "namespace", "n", "weka-operator-system", "Namespace where the operator is running")

	supportBundleOperatorCmd.SilenceUsage = true
}

func runSupportBundleOperator(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args
	return supportbundle.RunSupportBundleByMode(KubeClients, types.CollectionModeOperator, "", flagNamespace, flagAllNamespaces, flagIncludeSensitive)
}
