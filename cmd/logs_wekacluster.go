package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/logs"
)

var logsWekaClusterCmd = &cobra.Command{
	Use:   "wekacluster <cluster-name>",
	Short: "Show WekaCluster logs",
	Long: `Show logs from all (or filtered list of) containers of a particular WEKA cluster.

By default, shows logs from all containers. You can filter by:
  --role=<compute|s3|drive|envoy|nfs>   - Filter by container role
  --wekacontainer <name>                - Filter by container name
  --wekacontainer-id <int>              - Filter by container ID
  --node-selector <key=value,...>       - Filter by node labels (comma-separated key=value pairs)
  --limitconcurrent <int>               - Limit the number of log streams processed in parallel (default 10, 0 for unlimited)
  --no-prefix                           - Do not prepend log steams with container names`,
	Args: cobra.ExactArgs(1),
	RunE: runLogsWekaCluster,
}

func init() {
	logsCmd.AddCommand(logsWekaClusterCmd)

	logsWekaClusterCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "default",
		"Namespace where the WEKA cluster containers are running")

	logsWekaClusterCmd.Flags().StringVarP(&flagRole, "role", "r", "",
		"Filter containers by role (compute|s3|drive|envoy|nfs)")

	logsWekaClusterCmd.Flags().StringVarP(&flagContainerName, "wekacontainer", "c", "",
		"Filter by specific WekaContainer name")

	logsWekaClusterCmd.Flags().IntVarP(&flagContainerID, "wekacontainer-id", "i", -1,
		"Filter by specific WekaContainer ID (WekaContainer.Status.ClusterContainerID). Defaults to -1, no limits")

	logsWekaClusterCmd.Flags().StringVarP(&flagNodeSelector, "node-selector", "s", "",
		"Filter by node labels (comma-separated key=value pairs, e.g., disk=ssd,region=us-west)")

	logsWekaClusterCmd.Flags().BoolVarP(&flagLogsFollow, "follow", "f", false,
		"Specify if the logs should be streamed")

	logsWekaClusterCmd.Flags().Int64VarP(&flagLogsTail, "tail", "t", 50,
		"Lines of recent log file to display, or -1 (all logs). Defaults to 50 last lines.")

	logsWekaClusterCmd.Flags().DurationVar(&flagLogsSince, "since", 0,
		"Only return logs newer than a relative duration like 5s, 2m, or 3h")

	logsWekaClusterCmd.Flags().BoolVarP(&flagLogsPrevious, "previous", "p", false,
		"If true, print the logs for the previous instance of the container in a pod if it exists")

	logsWekaClusterCmd.Flags().IntVarP(&flagLogsLimitConcurrent, "limit-concurrent", "l", 10,
		"Maximum number of log files to process in parallel. If set to 0 –unlimited")

	logsWekaClusterCmd.Flags().BoolVar(&flagLogsNoPrefix, "no-prefix", false, "Do not prepend log steams with container names")

	logsWekaClusterCmd.RegisterFlagCompletionFunc("namespace", completionListNamespaces)
	logsWekaClusterCmd.RegisterFlagCompletionFunc("wekacontainer", completionListWekaContainers)
	logsWekaClusterCmd.RegisterFlagCompletionFunc("role", completionListWekaContainerRoles)
	logsWekaClusterCmd.ValidArgsFunction = completionListWekaClustersAsArgs

	logsWekaClusterCmd.SilenceUsage = true
}

func runLogsWekaCluster(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	var clusterName string
	if len(args) > 0 {
		clusterName = args[0]
	}
	// make validations to parameters sanity
	if strings.HasPrefix(flagRole, "-") {
		return fmt.Errorf("--role can not start with a prefix, check syntax")
	}

	opts := logs.WekaLogsOptions{
		OwnerName:     clusterName,
		OwnerKind:     "WekaCluster",
		Namespace:     flagNamespace,
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
