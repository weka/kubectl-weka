package supportbundle

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/printer"
	"os"
	"path/filepath"
)

// CSIResourcesCollector collects CSI driver components and diagnostics
type CSIResourcesCollector struct{}

func (c *CSIResourcesCollector) Name() string {
	return "CSI Driver Components"
}

func (c *CSIResourcesCollector) Start(ctx context.Context) {
	logger := GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect",
		"items", "CSI drivers list, CSI instances (pods), unhealthy instances (wide view)")
}

func (c *CSIResourcesCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := GetLogger(ctx)
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
	logger := GetLogger(ctx)
	clients := getClients(ctx)
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
	p := printer.NewSupportBundlePrinter()

	// Collect CSI drivers information
	logger.Debug("Collecting CSI drivers")
	driverOutput, err := getters.GenerateCSIDriversOutput(ctx, clients, false, false, "", p)
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
	instancesOutput, err := getters.GenerateCSIInstancesOutput(ctx, clients, "", "", "", false, p)
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
	unhealthyOutput, err := getters.GenerateCSIInstancesOutput(ctx, clients, "", "", "", true, p)
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
	// Collect CSI secrets (respecting include-sensitive-data flag)
	includeSensitive := getCollectSensitiveData(ctx)

	logger.Debug("Collecting CSI secrets", "include_sensitive", includeSensitive)
	secretsOutput, err := getters.GenerateCSISecretsOutput(ctx, clients, p)
	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to get CSI secrets: %v", err))
		logger.Warn("Failed to collect CSI secrets", "error", err)
	} else {
		secretsFile := filepath.Join(csiDir, "csi-secrets.txt")
		if err := os.WriteFile(secretsFile, []byte(secretsOutput), 0644); err != nil {
			errors = append(errors, fmt.Sprintf("failed to write CSI secrets file: %v", err))
			logger.Warn("Failed to write CSI secrets file", "error", err)
		} else {
			filesCreated = append(filesCreated, secretsFile)
			if includeSensitive {
				logger.Warn("Collected CSI secrets (unredacted)", "file", secretsFile)
			} else {
				logger.Debug("Collected CSI secrets (redacted)", "file", secretsFile)
			}
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
