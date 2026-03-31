package cmd

import (
	"context"
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/logs"
)

// Local namespace variable for operator logs to avoid global conflict
var flagLogOperatorNamespace string

var logsOperatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "Show logs of the WEKA operator controller manager",
	RunE:  runLogsOperator,
}

func init() {
	logsCmd.AddCommand(logsOperatorCmd)

	logsOperatorCmd.Flags().StringVarP(&flagLogOperatorNamespace, "namespace", "n", "weka-operator-system",
		"Namespace where the WEKA operator is running")

	logsOperatorCmd.Flags().BoolVarP(&flagLogsFollow, "follow", "f", false,
		"Specify if the logs should be streamed")

	// kubectl default is usually -1 (all lines). We'll match that.
	logsOperatorCmd.Flags().Int64Var(&flagLogsTail, "tail", 50,
		"Lines of recent log file to display, or -1 (all logs). Defaults to 50 last lines.")

	logsOperatorCmd.Flags().DurationVar(&flagLogsSince, "since", 0,
		"Only return logs newer than a relative duration like 5s, 2m, or 3h")

	logsOperatorCmd.Flags().BoolVarP(&flagLogsPrevious, "previous", "p", false,
		"If true, print the logs for the previous instance of the container in a pod if it exists")

	logsOperatorCmd.SilenceUsage = true
}

func runLogsOperator(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	opts := logs.OperatorLogsOptions{
		Namespace:   flagLogOperatorNamespace,
		Follow:      flagLogsFollow,
		Tail:        flagLogsTail,
		Since:       flagLogsSince,
		Previous:    flagLogsPrevious,
		TailFlagSet: cmd.Flags().Changed("tail"),
	}

	return logs.StreamOperatorLogs(ctx, KubeClients, opts)
}
