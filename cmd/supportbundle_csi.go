package cmd

import (
	"context"
	"github.com/spf13/cobra"
)

var supportBundleCSICmd = &cobra.Command{
	Use:   "csi",
	Short: "Collect CSI-related resources and logs",
	Long: `Collects diagnostic information for Weka CSI components including:
  - CSI driver pods and logs
  - CSI controller pods and logs
  - Storage classes
  - Persistent volumes and claims

Note: CSI component detection logic is a placeholder and will be implemented later.`,
	RunE: runSupportBundleCSI,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleCSICmd)

	supportBundleCSICmd.Flags().StringVar(&supportBundleCaseID, "case-id", "", "Case ID (Salesforce/Jira) to include in bundle name")
	supportBundleCSICmd.Flags().StringVarP(&supportBundleOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleCSICmd.Flags().StringVarP(&supportBundleNamespace, "namespace", "n", "", "Namespace (defaults to current kubeconfig namespace)")
	supportBundleCSICmd.Flags().BoolVarP(&supportBundleAllNS, "all-namespaces", "A", false, "Collect CSI resources from all namespaces")
	supportBundleCSICmd.Flags().BoolVar(&supportBundleIncludeSensitive, "include-sensitive-data", false, "Include sensitive data such as Secrets and credentials (⚠️  INSECURE - use with caution)")

	supportBundleCSICmd.SilenceUsage = true
}

func runSupportBundleCSI(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args
	return runSupportBundleByMode(ModeCSI, "", supportBundleNamespace, supportBundleAllNS)
}

// CSIResourcesCollector collects CSI driver and controller resources
type CSIResourcesCollector struct{}

func (c *CSIResourcesCollector) Name() string {
	return "CSI Resources"
}

func (c *CSIResourcesCollector) Start(ctx context.Context) {
	logger := getLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "status", "CSI collection is a placeholder")
}

func (c *CSIResourcesCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := getLogger(ctx)
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success")
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial")
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

func (c *CSIResourcesCollector) Collect(ctx context.Context) CollectorResult {
	logger := getLogger(ctx)
	var filesCreated []string
	var warnings []string

	logger.Debug("=== CSIResourcesCollector Debug Mode", "enabled", supportBundleDebug)
	logger.Debug("⚠️  CSI collection is not yet implemented (placeholder)")

	// TODO: Implement CSI component collection
	// This is a placeholder implementation
	//
	// The actual implementation should:
	// 1. Find CSI driver pods (typically have labels like app=csi-wekafsplugin)
	// 2. Find CSI controller pods
	// 3. Collect logs from all CSI-related pods
	// 4. Collect storage classes related to Weka
	// 5. Optionally collect PVs and PVCs
	//
	// Example label selectors to look for:
	// - app=csi-wekafsplugin
	// - app.kubernetes.io/name=weka-csi-driver
	// - component=csi-driver
	//
	// Common namespaces:
	// - csi-wekafsplugin
	// - kube-system

	warnings = append(warnings, "CSI collection is not yet implemented (placeholder)")

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
