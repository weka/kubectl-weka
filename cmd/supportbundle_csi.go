package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
  - CSI driver deployment information`,
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

// CSIResourcesCollector collects CSI driver components and diagnostics
type CSIResourcesCollector struct{}

func (c *CSIResourcesCollector) Name() string {
	return "CSI Driver Components"
}

func (c *CSIResourcesCollector) Start(ctx context.Context) {
	logger := getLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect",
		"items", "CSI drivers list, CSI instances (pods), unhealthy instances (wide view)")
}

func (c *CSIResourcesCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := getLogger(ctx)
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files_created", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files_created", len(result.FilesCreated))
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

func (c *CSIResourcesCollector) Collect(ctx context.Context) CollectorResult {
	logger := getLogger(ctx)
	bundlePath := getBundlePath(ctx)
	var filesCreated []string
	var errors []string

	// Create CSI directory in the bundle
	csiDir := filepath.Join(bundlePath, "csi")
	if err := os.MkdirAll(csiDir, 0755); err != nil {
		logger.Error("Failed to create CSI directory", "error", err)
		return CollectorResult{
			Status:       StatusFailure,
			FilesCreated: filesCreated,
			Error:        err,
		}
	}

	// Collect CSI drivers information
	logger.Debug("Collecting CSI drivers")
	driverOutput, err := generateCSIDriversOutput(ctx, KubeClients, false, false, false, "")
	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to get CSI drivers: %v", err))
		logger.Warn("Failed to collect CSI drivers", "error", err)
	} else {
		driversFile := filepath.Join(csiDir, "csi-drivers.txt")
		if err := os.WriteFile(driversFile, []byte(driverOutput), 0644); err != nil {
			errors = append(errors, fmt.Sprintf("failed to write CSI drivers file: %v", err))
			logger.Warn("Failed to write CSI drivers file", "error", err)
		} else {
			filesCreated = append(filesCreated, driversFile)
			logger.Debug("Collected CSI drivers", "file", driversFile)
		}
	}

	// Collect CSI instances (pods) information
	logger.Debug("Collecting CSI instances")
	printer := NewSupportBundlePrinter()
	instancesOutput, err := generateCSIInstancesOutput(ctx, KubeClients, "", "", "", false, printer)
	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to get CSI instances: %v", err))
		logger.Warn("Failed to collect CSI instances", "error", err)
	} else {
		instancesFile := filepath.Join(csiDir, "csi-instances.txt")
		if err := os.WriteFile(instancesFile, []byte(instancesOutput), 0644); err != nil {
			errors = append(errors, fmt.Sprintf("failed to write CSI instances file: %v", err))
			logger.Warn("Failed to write CSI instances file", "error", err)
		} else {
			filesCreated = append(filesCreated, instancesFile)
			logger.Debug("Collected CSI instances", "file", instancesFile)
		}
	}

	// Collect unhealthy CSI instances in wide view
	logger.Debug("Collecting unhealthy CSI instances")
	unhealthyOutput, err := generateCSIInstancesOutput(ctx, KubeClients, "", "", "", true, printer)
	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to get unhealthy CSI instances: %v", err))
		logger.Warn("Failed to collect unhealthy CSI instances", "error", err)
	} else {
		unhealthyFile := filepath.Join(csiDir, "csi-instances-unhealthy.txt")
		if err := os.WriteFile(unhealthyFile, []byte(unhealthyOutput), 0644); err != nil {
			errors = append(errors, fmt.Sprintf("failed to write unhealthy CSI instances file: %v", err))
			logger.Warn("Failed to write unhealthy CSI instances file", "error", err)
		} else {
			filesCreated = append(filesCreated, unhealthyFile)
			logger.Debug("Collected unhealthy CSI instances", "file", unhealthyFile)
		}
	}

	// Determine overall status
	status := StatusSuccess
	if len(errors) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     errors,
	}
}
