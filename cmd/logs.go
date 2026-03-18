package cmd

import (
	"github.com/spf13/cobra"
	"time"
)

var (
	flagLogsFollow          bool
	flagLogsTail            int64
	flagLogsSince           time.Duration
	flagLogsPrevious        bool
	flagLogsNoPrefix        bool
	flagLogsLimitConcurrent int
	flagContainerName       string
	flagContainerID         int
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View WEKA related logs",
}
