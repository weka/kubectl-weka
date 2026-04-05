package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/logs"
)

var logsWekaClientCmd = &cobra.Command{
	Use:   "wekaclient <client-name>",
	Short: "Show WekaClient logs",
	Long: `Show logs from all (or filtered list of) containers of a particular WEKA client.

By default, shows logs from all containers. You can filter by:
  --wekacontainer <name>                - Filter by container name
  --wekacontainer-id <int>              - Filter by container ID
  --node-selector <key=value,...>       - Filter by node labels (comma-separated key=value pairs)
  --limitconcurrent <int>               - Limit the number of log streams processed in parallel (default 10, 0 for unlimited)
  --no-prefix                           - Do not prepend log steams with container names`,
	Args: cobra.ExactArgs(1),
	RunE: runLogsWekaClient,
}

func init() {
	logsCmd.AddCommand(logsWekaClientCmd)

	logsWekaClientCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "default",
		"Namespace where the WEKA client containers are running")

	logsWekaClientCmd.Flags().StringVarP(&flagContainerName, "wekacontainer", "c", "",
		"Filter by specific WekaContainer name")

	logsWekaClientCmd.Flags().IntVarP(&flagContainerID, "wekacontainer-id", "i", -1,
		"Filter by specific WekaContainer ID (WekaContainer.Status.ClusterContainerID). Defaults to -1, no limits")

	logsWekaClientCmd.Flags().StringVarP(&flagNodeSelector, "node-selector", "s", "",
		"Filter by node labels (comma-separated key=value pairs, e.g., disk=ssd,region=us-west)")

	logsWekaClientCmd.Flags().BoolVarP(&flagLogsFollow, "follow", "f", false,
		"Specify if the logs should be streamed")

	logsWekaClientCmd.Flags().Int64VarP(&flagLogsTail, "tail", "t", 50,
		"Lines of recent log file to display, or -1 (all logs). Defaults to 50 last lines.")

	logsWekaClientCmd.Flags().DurationVar(&flagLogsSince, "since", 0,
		"Only return logs newer than a relative duration like 5s, 2m, or 3h")

	logsWekaClientCmd.Flags().BoolVarP(&flagLogsPrevious, "previous", "p", false,
		"If true, print the logs for the previous instance of the container in a pod if it exists")

	logsWekaClientCmd.Flags().IntVarP(&flagLogsLimitConcurrent, "limit-concurrent", "l", 10,
		"Maximum number of log files to process in parallel. If set to 0 –unlimited")

	logsWekaClientCmd.Flags().BoolVar(&flagLogsNoPrefix, "no-prefix", false, "Do not prepend log steams with container names")

	logsWekaClientCmd.SilenceUsage = true
}

func runLogsWekaClient(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	clientName := args[0]

	opts := logs.WekaLogsOptions{
		OwnerName:     clientName,
		OwnerKind:     "WekaClient",
		Namespace:     flagNamespace,
		Role:          "",
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
