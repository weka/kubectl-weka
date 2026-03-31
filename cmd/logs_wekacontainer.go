package cmd

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"strings"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/logs"
)

var logsWekaContainerCmd = &cobra.Command{
	Use:   "wekacontainer <client-name>",
	Short: "Show WekaContainer logs",
	Long: `Show logs from all or arbitrary WekaContainers.

By default, shows logs from all containers. You can filter by:
  --wekacontainer <name>                - Filter by container name
  --namespace <namespace>               - Filter by namespace
  --all-namespaces                      - Filter by all namespaces
  --wekacontainer-id <int>              - Filter by container ID
  --node-selector <key=value,...>       - Filter by node labels (comma-separated key=value pairs)
  --limitconcurrent <int>               - Limit the number of log streams processed in parallel (default 10, 0 for unlimited)
  --no-prefix                           - Do not prepend log steams with container names`,
	RunE: runLogsWekaContainer,
}

func init() {
	logsCmd.AddCommand(logsWekaContainerCmd)

	logsWekaContainerCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "default",
		"Namespace where the WEKA client containers are running")

	logsWekaContainerCmd.Flags().BoolVarP(&flagAllNamespaces, "all-namespaces", "A", false,
		"If true, search for WekaContainers across all namespaces. If set, --namespace is ignored.")

	logsWekaContainerCmd.Flags().StringVarP(&flagContainerName, "wekacontainer", "c", "",
		"Filter by specific WekaContainer name")

	logsWekaContainerCmd.Flags().IntVarP(&flagContainerID, "wekacontainer-id", "i", -1,
		"Filter by specific WekaContainer ID (WekaContainer.Status.ClusterContainerID). Defaults to -1, no limits")

	logsWekaContainerCmd.Flags().StringVarP(&flagNodeSelector, "node-selector", "s", "",
		"Filter by node labels (comma-separated key=value pairs, e.g., disk=ssd,region=us-west)")

	logsWekaContainerCmd.Flags().BoolVarP(&flagLogsFollow, "follow", "f", false,
		"Specify if the logs should be streamed")

	logsWekaContainerCmd.Flags().Int64VarP(&flagLogsTail, "tail", "t", 50,
		"Lines of recent log file to display, or -1 (all logs). Defaults to 50 last lines.")

	logsWekaContainerCmd.Flags().DurationVar(&flagLogsSince, "since", 0,
		"Only return logs newer than a relative duration like 5s, 2m, or 3h")

	logsWekaContainerCmd.Flags().BoolVarP(&flagLogsPrevious, "previous", "p", false,
		"If true, print the logs for the previous instance of the container in a pod if it exists")

	logsWekaContainerCmd.Flags().IntVarP(&flagLogsLimitConcurrent, "limit-concurrent", "l", 10,
		"Maximum number of log files to process in parallel. If set to 0 –unlimited")

	logsWekaContainerCmd.Flags().BoolVar(&flagLogsNoPrefix, "no-prefix", false, "Do not prepend log steams with container names")

	logsWekaContainerCmd.SilenceUsage = true
}

func runLogsWekaContainer(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate parameters
	if strings.HasPrefix(flagRole, "-") {
		return fmt.Errorf("--role can not start with a prefix, check syntax")
	}

	ns, nsAll, err := kubernetes.GetNamespaceFromFlags(flagAllNamespaces, flagNamespace)
	if err != nil {
		return err
	}

	opts := logs.WekaLogsOptions{
		OwnerName:     "",
		OwnerKind:     "", // do not filter logs for owner at all
		Namespace:     ns,
		AllNamespaces: nsAll,
		Role:          flagRole,
		ContainerName: flagContainerName,
		ContainerID:   flagContainerID,
		Aggregation: logs.AggregatedLogOptions{
			Follow:             flagLogsFollow,
			Tail:               flagLogsTail,
			Since:              flagLogsSince,
			Previous:           flagLogsPrevious,
			TailFlagSet:        cmd.Flags().Changed("tail"),
			LimitConcurrent:    flagLogsLimitConcurrent,
			AddContainerPrefix: !flagLogsNoPrefix,
			NodeSelector:       flagNodeSelector,
		},
	}

	return logs.StreamWekaObjectLogs(ctx, KubeClients, opts)
}
