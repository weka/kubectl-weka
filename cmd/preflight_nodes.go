package cmd

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/preflight"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	preflightSummaryOnly   bool
	preflightFailedOnly    bool
	preflightWekaDirFailGB int64
	preflightWekaDirWarnGB int64
)

var preflightNodesCmd = &cobra.Command{
	Use:   "nodes [NODE...]",
	Short: "Preflight node checks (OS, hugepages, free resources, host readiness)",
	Args:  cobra.ArbitraryArgs,
	RunE:  runPreflightNodes,
}

func init() {
	preflightCmd.AddCommand(preflightNodesCmd)
	preflightNodesCmd.Flags().StringVar(&flagNodeSelector, "node-selector", "", "Label selector to filter nodes, e.g. if only part of nodes are targeted for WEKA")
	preflightNodesCmd.Flags().BoolVar(&flagFailFast, "fail-fast", false, "Stop on first failed node")
	preflightNodesCmd.Flags().BoolVar(&preflightSummaryOnly, "summary-only", false, "Only print summary (no per-node details)")
	preflightNodesCmd.Flags().BoolVar(&preflightFailedOnly, "failed-only", false, "Only show failed nodes")
	preflightNodesCmd.Flags().Int64Var(&preflightWekaDirFailGB, "weka-dir-min-fail", 100, "Minimum GB for weka directory (FAIL if below, default 100)")
	preflightNodesCmd.Flags().Int64Var(&preflightWekaDirWarnGB, "weka-dir-min-warn", 300, "Minimum GB for weka directory (WARN if below, default 300)")
	preflightNodesCmd.SilenceUsage = true

}

func runPreflightNodes(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Setup signal handling for graceful shutdown (cleanup pods on Ctrl-C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle signals in background
	go func() {
		sig := <-sigChan
		fmt.Printf("\n\nReceived signal %v, cleaning up pods...\n", sig)
		cancel() // Cancel context to stop operations
	}()

	// Create output that writes to stdout in real-time
	output := preflight.NewPreflightOutput(os.Stdout)
	defer output.Close()

	// Run the preflight checks
	result := preflight.GeneratePreflightNodesOutput(
		ctx,
		KubeClients,
		args,
		flagNodeSelector,
		flagFailFast,
		preflightSummaryOnly,
		preflightFailedOnly,
		preflightWekaDirFailGB,
		preflightWekaDirWarnGB,
		output,
	)

	if !result.Success {
		return fmt.Errorf("preflight nodes failed")
	}

	if result.Error != nil {
		return result.Error
	}

	return nil
}
