package cmd

import (
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/completion"
	"github.com/weka/kubectl-weka/pkg/supportbundle"
	"github.com/weka/kubectl-weka/pkg/types"
)

var supportBundleCSICmd = &cobra.Command{
	Use:   "csi",
	Short: "Collect CSI-related resources and logs",
	Long: `Collects diagnostic information for Weka CSI components including:
  - CSI driver pods and logs
  - CSI controller pods and logs
  - Storage classes
  - Persistent volumes and claims
  - CSI driver deployment information`,
	RunE: runSupportBundleCSI,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleCSICmd)

	supportBundleCSICmd.Flags().StringVarP(&flagOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleCSICmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "", "Namespace (defaults to current kubeconfig namespace)")
	supportBundleCSICmd.Flags().BoolVarP(&flagAllNamespaces, "all-namespaces", "A", false, "Collect CSI resources from all namespaces")

	supportBundleCSICmd.RegisterFlagCompletionFunc("namespace", completionListNamespaces)
	supportBundleCSICmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completion.SuggestAllUnusedFlagsWithUsageForCompletion(cmd, args, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	supportBundleCSICmd.SilenceUsage = true
}

func runSupportBundleCSI(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args
	return supportbundle.RunSupportBundleByMode(KubeClients, types.CollectionModeCSI, "", flagNamespace, flagAllNamespaces, flagIncludeSensitive)
}
