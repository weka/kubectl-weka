package cmd

import (
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/supportbundle"
	"github.com/weka/kubectl-weka/pkg/types"
)

var supportBundleClientCmd = &cobra.Command{
	Use:   "client [CLIENT_NAME]",
	Short: "Collect client-related resources and logs",
	Long: `Collects diagnostic information for Weka clients including:
  - WekaClient resources
  - Associated pods and their logs

If client name is not specified:
  - With -n: collects all clients in the specified namespace
  - With --all-namespaces: collects all clients
  - Otherwise: collects all clients in the default namespace

If client name is specified:
  - Without -n or --all-namespaces: searches in default namespace
  - With -n: searches in specified namespace
  - --all-namespaces: searches across all namespaces`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSupportBundleClient,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleClientCmd)

	supportBundleClientCmd.Flags().StringVarP(&flagOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleClientCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "", "Namespace (defaults to current kubeconfig namespace)")
	supportBundleClientCmd.Flags().BoolVarP(&flagAllNamespaces, "all-namespaces", "A", false, "Collect clients from all namespaces")
	supportBundleClientCmd.RegisterFlagCompletionFunc("namespace", completionListNamespaces)

	supportBundleClientCmd.ValidArgsFunction = completionListWekaClientsAsArgs
}

func runSupportBundleClient(cmd *cobra.Command, args []string) error {
	_ = cmd
	var clientName string
	if len(args) > 0 {
		clientName = args[0]
	}
	return supportbundle.RunSupportBundleByMode(KubeClients, types.CollectionModeClient, clientName, flagNamespace, flagAllNamespaces, flagIncludeSensitive)
}
