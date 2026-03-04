package cmd

import (
	"github.com/spf13/cobra"
)

var supportBundleAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Collect all support bundle information (operator, clusters, clients, CSI, k8s)",
	Long: `Collects comprehensive diagnostic information including:
  - Operator logs and resources
  - All WekaCluster resources and their logs
  - All WekaClient resources and their logs
  - CSI components
  - Kubernetes preflight checks`,
	RunE: runSupportBundleAll,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleAllCmd)

	supportBundleAllCmd.Flags().StringVar(&supportBundleCaseID, "case-id", "", "Case ID (Salesforce/Jira) to include in bundle name")
	supportBundleAllCmd.Flags().StringVarP(&supportBundleOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleAllCmd.Flags().BoolVarP(&supportBundleAllNS, "all-namespaces", "A", false, "Collect resources from all namespaces")
	supportBundleAllCmd.Flags().StringVarP(&supportBundleNamespace, "namespace", "n", "", "Namespace (defaults to current kubeconfig namespace)")
	supportBundleAllCmd.Flags().BoolVar(&supportBundleIncludeSensitive, "include-sensitive-data", false, "Include sensitive data such as Secrets and credentials (⚠️  INSECURE - use with caution)")

	supportBundleAllCmd.SilenceUsage = true
}

func runSupportBundleAll(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args
	return runSupportBundleByMode(ModeAll, "", supportBundleNamespace, supportBundleAllNS)
}
