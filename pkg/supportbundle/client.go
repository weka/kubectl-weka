package supportbundle

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"path/filepath"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ClientResourcesCollector collects WekaClient resources and their dependencies
type ClientResourcesCollector struct {
	ClientName string
}

func (c *ClientResourcesCollector) Name() string {
	if c.ClientName != "" {
		return fmt.Sprintf("Client Resources (%s)", c.ClientName)
	}
	return "Client Resources"
}

func (c *ClientResourcesCollector) Start(ctx context.Context) {
	logger := logging.GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	if c.ClientName != "" {
		logger.Info("Searching for client", "name", c.ClientName)
	} else {
		logger.Info("Searching for all clients")
	}
	logger.Info("Will collect", "items", "WekaClient, pods")
}

func (c *ClientResourcesCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := logging.GetLogger(ctx)
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files", len(result.FilesCreated), "warnings", len(result.Warnings))
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

func (c *ClientResourcesCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := logging.GetLogger(ctx)

	// List WekaClient resources
	var clients v1alpha1.WekaClientList
	listOpts := []crclient.ListOption{}

	ns := getNamespace(ctx)
	allNs := getAllNamespaces(ctx)
	if !allNs && ns != "" {
		listOpts = append(listOpts, crclient.InNamespace(ns))
	}

	kubeClients := getClients(ctx)
	if err := kubeClients.CRClient.List(ctx, &clients, listOpts...); err != nil {
		return CollectorResult{Status: StatusFailure, Error: fmt.Errorf("failed to list WekaClient resources: %w", err), Warnings: warnings}
	}

	// Filter by client name if specified
	var filteredClients []v1alpha1.WekaClient
	if c.ClientName != "" {
		for _, client := range clients.Items {
			if client.Name == c.ClientName {
				filteredClients = append(filteredClients, client)
			}
		}
		if len(filteredClients) == 0 {
			return CollectorResult{Status: StatusFailure, Error: fmt.Errorf("client %q not found", c.ClientName), Warnings: warnings}
		}
	} else {
		filteredClients = clients.Items
	}

	if len(filteredClients) == 0 {
		logger.Debug("⚠️  No clients found")
		return CollectorResult{Status: StatusSuccess, FilesCreated: filesCreated, Warnings: warnings}
	}

	// Collect each client
	bundlePath := getBundlePath(ctx)
	for _, client := range filteredClients {
		logger.Debug("✓ Processing client", "namespace", client.Namespace, "name", client.Name)
		// Dump WekaClient resource
		yaml, err := collectObjectAsYAMLWithSensitiveData(&client, getCollectSensitiveData(ctx))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to marshal client %s/%s: %v", client.Namespace, client.Name, err))
			logger.Debug("⚠️  Failed to marshal client", "namespace", client.Namespace, "name", client.Name, "error", err)
			continue
		}

		filePath := filepath.Join("clients", client.Name, GenerateSafeFileName("WekaClient", client.Namespace, client.Name, "yaml"))
		if err := WriteToFile(bundlePath, filePath, yaml); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write client file for %s/%s: %v", client.Namespace, client.Name, err))
			logger.Debug("⚠️  Failed to write client file", "namespace", client.Namespace, "name", client.Name, "error", err)
			continue
		}
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected client resource", "namespace", client.Namespace, "name", client.Name)

		// Collect client instances information
		instancesFile, instancesWarning := c.collectClientInstances(ctx, &client)
		if instancesFile != "" {
			filesCreated = append(filesCreated, instancesFile)
		}
		if instancesWarning != "" {
			warnings = append(warnings, instancesWarning)
		}

		// Collect WekaContainers for this client
		containerFiles, containerWarnings, err := c.collectClientContainers(ctx, &client)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to collect containers for client %s/%s: %v", client.Namespace, client.Name, err))
			logger.Debug("⚠️  Failed to collect containers", "namespace", client.Namespace, "name", client.Name, "error", err)
		} else {
			filesCreated = append(filesCreated, containerFiles...)
			warnings = append(warnings, containerWarnings...)
			if len(containerFiles) > 0 {
				logger.Debug("✓ Collected pod files", "client", client.Name, "count", len(containerFiles))
			}
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

// collectClientInstances collects client instances information for a specific client
func (c *ClientResourcesCollector) collectClientInstances(ctx context.Context, client *v1alpha1.WekaClient) (string, string) {
	logger := logging.GetLogger(ctx)
	bundlePath := getBundlePath(ctx)
	clients := getClients(ctx)

	logger.Debug("Collecting client instances", "client", client.Name, "namespace", client.Namespace)

	p := printer.NewSupportBundlePrinter()

	output, err := getters.GenerateClientInstancesOutput(
		ctx,
		clients,
		client.Namespace,
		false, // allNamespaces = false (we want this specific client only)
		client.Name,
		p,
	)

	if err != nil {
		warning := fmt.Sprintf("failed to collect client instances for %s/%s: %v", client.Namespace, client.Name, err)
		logger.Debug("⚠️  Failed to collect client instances", "client", client.Name, "namespace", client.Namespace, "error", err)
		return "", warning
	}

	// Write output to file
	fileName := fmt.Sprintf("client-instances-%s.txt", client.Name)
	filePath := filepath.Join("clients", client.Name, fileName)
	if err := WriteToFile(bundlePath, filePath, output); err != nil {
		warning := fmt.Sprintf("failed to write client instances output for %s/%s: %v", client.Namespace, client.Name, err)
		logger.Debug("⚠️  Failed to write client instances file", "client", client.Name, "error", err)
		return "", warning
	}

	logger.Debug("✓ Collected client instances", "client", client.Name, "file", filePath)
	return filePath, ""
}

// collectClientContainers collects WekaContainers and their pods for a client
func (c *ClientResourcesCollector) collectClientContainers(ctx context.Context, client *v1alpha1.WekaClient) ([]string, []string, error) {
	var filesCreated []string
	var warnings []string

	logger := logging.GetLogger(ctx)
	clients := getClients(ctx)
	bundlePath := getBundlePath(ctx)
	collectSensitive := getCollectSensitiveData(ctx)

	// List WekaContainers for this client using cached CRClient
	var containers v1alpha1.WekaContainerList
	listOpts := []crclient.ListOption{
		crclient.InNamespace(client.Namespace),
	}

	if err := clients.CRClient.List(ctx, &containers, listOpts...); err != nil {
		return nil, nil, fmt.Errorf("failed to list WekaContainer CRs: %w", err)
	}

	// Filter containers owned by this client
	var matchingContainers []v1alpha1.WekaContainer
	for _, container := range containers.Items {
		for _, ownerRef := range container.GetOwnerReferences() {
			if ownerRef.Kind == "WekaClient" && ownerRef.Name == client.Name {
				matchingContainers = append(matchingContainers, container)
				break
			}
		}
	}

	logger.Debug("Found WekaContainers for client", "client", client.Name, "count", len(matchingContainers))

	// Collect each WekaContainer
	var podNames []string
	for _, container := range matchingContainers {
		// Dump WekaContainer resource
		containerYAML, err := collectObjectAsYAMLWithSensitiveData(&container, collectSensitive)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to marshal WekaContainer %s/%s: %v", container.Namespace, container.Name, err))
			logger.Debug("⚠️  Failed to marshal container", "namespace", container.Namespace, "name", container.Name, "error", err)
			continue
		}

		containerPath := filepath.Join("clients", client.Name, "containers", GenerateSafeFileName("WekaContainer", container.Namespace, container.Name, "yaml"))
		if err := WriteToFile(bundlePath, containerPath, containerYAML); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write WekaContainer file for %s/%s: %v", container.Namespace, container.Name, err))
			logger.Debug("⚠️  Failed to write container file", "namespace", container.Namespace, "name", container.Name, "error", err)
			continue
		}
		filesCreated = append(filesCreated, containerPath)
		logger.Debug("✓ Collected WekaContainer resource", "namespace", container.Namespace, "name", container.Name)

		// Pod has the same name as WekaContainer
		podNames = append(podNames, container.Name)
	}

	// Collect pod logs in parallel (both current and previous)
	if len(podNames) > 0 {
		logsDir := filepath.Join("clients", client.Name, "logs")
		logger.Debug("Collecting pod logs in parallel", "client", client.Name, "pods", len(podNames))

		logFiles, logWarnings := CollectPodLogsParallel(ctx, clients, client.Namespace, podNames, logsDir, 10)
		filesCreated = append(filesCreated, logFiles...)
		warnings = append(warnings, logWarnings...)

		logger.Debug("✓ Collected pod logs", "client", client.Name, "files", len(logFiles))

		// Collect pod descriptions in parallel
		descDir := filepath.Join("clients", client.Name, "pods")
		logger.Debug("Collecting pod descriptions in parallel", "client", client.Name, "pods", len(podNames))

		descFiles, descWarnings := CollectPodDescriptionsParallel(ctx, clients, client.Namespace, podNames, descDir, 10)
		filesCreated = append(filesCreated, descFiles...)
		warnings = append(warnings, descWarnings...)

		logger.Debug("✓ Collected pod descriptions", "client", client.Name, "files", len(descFiles))
	}

	return filesCreated, warnings, nil
}
