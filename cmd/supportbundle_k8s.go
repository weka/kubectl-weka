package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

var supportBundleK8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Collect Kubernetes preflight check results",
	Long: `Runs Kubernetes preflight checks and stores the results as log files.
This includes all cluster-level and node-level checks that would normally
be performed before deploying Weka resources.`,
	RunE: runSupportBundleK8s,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleK8sCmd)

	supportBundleK8sCmd.Flags().StringVar(&supportBundleCaseID, "case-id", "", "Case ID (Salesforce/Jira) to include in bundle name")
	supportBundleK8sCmd.Flags().StringVarP(&supportBundleOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleK8sCmd.Flags().StringVarP(&supportBundleNodeSel, "node-selector", "l", "", "Node selector for node-level checks (e.g., 'node-role=weka')")
	supportBundleK8sCmd.Flags().BoolVar(&supportBundleIncludeSensitive, "include-sensitive-data", false, "Include sensitive data such as Secrets and credentials (⚠️  INSECURE - use with caution)")

	supportBundleK8sCmd.SilenceUsage = true
}

func runSupportBundleK8s(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args
	return runSupportBundleByMode(ModeK8s, "", supportBundleNamespace, supportBundleAllNS)
}

// K8sPreflightCollector runs preflight checks and stores results
type K8sPreflightCollector struct {
	NodeSelector string
}

func (c *K8sPreflightCollector) Name() string {
	return "Kubernetes Preflight Checks"
}

func (c *K8sPreflightCollector) Start(ctx context.Context) {
	logger := getLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "cluster and node preflight checks")
}

func (c *K8sPreflightCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := getLogger(ctx)
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files", len(result.FilesCreated))
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
		if len(result.Warnings) > 0 {
			logger.Info("Non-fatal warnings found", "count", len(result.Warnings))
			for _, warning := range result.Warnings {
				logger.Info("Warning", "message", warning)
			}
		}
	}
}

func (c *K8sPreflightCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := getLogger(ctx)
	logger.Debug("=== K8sPreflightCollector Debug Mode", "enabled", supportBundleDebug)

	// Run cluster-level preflight checks
	logger.Debug("✓ Running cluster-level preflight checks...")
	clusterResults, err := c.runClusterPreflightChecks(ctx)
	if err != nil {
		logger.Debug("⚠️  Failed to run cluster preflight checks", "error", err)
		return CollectorResult{Status: StatusFailure, Error: fmt.Errorf("failed to run cluster preflight checks: %w", err), Warnings: warnings}
	}

	bundlePath := getBundlePath(ctx)
	clusterPath := filepath.Join("k8s-preflight", "cluster-checks.log")
	if err := writeToFile(bundlePath, clusterPath, clusterResults); err != nil {
		logger.Debug("⚠️  Failed to write cluster checks file", "error", err)
		return CollectorResult{Status: StatusFailure, Error: err, Warnings: warnings}
	}
	filesCreated = append(filesCreated, clusterPath)
	logger.Debug("✓ Collected cluster-level preflight checks")

	// Run node-level preflight checks
	logger.Debug("✓ Running node-level preflight checks...")

	nodeResults, err := c.runNodePreflightChecks(ctx)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to run node preflight checks: %v", err))
		logger.Debug("⚠️  Failed to run node preflight checks", "error", err)
	} else {
		nodePath := filepath.Join("k8s-preflight", "node-checks.log")
		if err := writeToFile(bundlePath, nodePath, nodeResults); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write node checks file: %v", err))
			logger.Debug("⚠️  Failed to write node checks file", "error", err)
		} else {
			filesCreated = append(filesCreated, nodePath)
			logger.Debug("✓ Collected node-level preflight checks")
		}
	}

	// Determine overall status
	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{Status: status, FilesCreated: filesCreated, Warnings: warnings}
}

func (c *K8sPreflightCollector) runClusterPreflightChecks(ctx context.Context) (string, error) {
	// Create output without a writer (we'll capture it in the buffer)
	output := NewPreflightOutput(nil)
	defer output.Close()

	// Run the preflight checks using the new function
	result := generatePreflightK8sClusterOutput(ctx, nil, c.NodeSelector, output)

	// Return the captured output (even if there was an error)
	return result.Output, nil
}

func (c *K8sPreflightCollector) runNodePreflightChecks(ctx context.Context) (string, error) {
	// Create output without a writer (we'll capture it in the buffer)
	output := NewPreflightOutput(nil)
	defer output.Close()

	// Run the preflight checks using the new function
	result := generatePreflightNodesOutput(
		ctx,
		nil,            // no specific nodes (use all or selector)
		c.NodeSelector, // use the collector's node selector
		false,          // failFast
		false,          // summaryOnly
		false,          // failedOnly
		100,            // wekaDirFailGB (default)
		300,            // wekaDirWarnGB (default)
		output,
	)

	// Return the captured output (even if there was an error)
	return result.Output, nil
}
