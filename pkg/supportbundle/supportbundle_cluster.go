package supportbundle

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/printer"
	"path/filepath"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterResourcesCollector collects WekaCluster and WekaContainer resources
type ClusterResourcesCollector struct {
	ClusterName string
}

func (c *ClusterResourcesCollector) Name() string {
	if c.ClusterName != "" {
		return fmt.Sprintf("Cluster Resources (%s)", c.ClusterName)
	}
	return "Cluster Resources"
}

func (c *ClusterResourcesCollector) Start(ctx context.Context) {
	logger := logging.GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	if c.ClusterName != "" {
		logger.Info("Searching for cluster", "name", c.ClusterName)
	} else {
		logger.Info("Searching for all clusters")
	}
	logger.Info("Will collect", "items", "WekaCluster, WekaContainer, pods")
}

func (c *ClusterResourcesCollector) Finish(ctx context.Context, result CollectorResult) {
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

func (c *ClusterResourcesCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := logging.GetLogger(ctx)

	// List WekaCluster resources
	var clusters wekaapi.WekaClusterList
	listOpts := []crclient.ListOption{}

	ns := getNamespace(ctx)
	allNs := getAllNamespaces(ctx)
	if !allNs && ns != "" {
		listOpts = append(listOpts, crclient.InNamespace(ns))
	}

	clients := getClients(ctx)
	if err := clients.CRClient.List(ctx, &clusters, listOpts...); err != nil {
		return CollectorResult{Status: StatusFailure, Error: fmt.Errorf("failed to list WekaCluster resources: %w", err), Warnings: warnings}
	}

	// Filter by cluster name if specified
	var filteredClusters []wekaapi.WekaCluster
	if c.ClusterName != "" {
		for _, cluster := range clusters.Items {
			if cluster.Name == c.ClusterName {
				filteredClusters = append(filteredClusters, cluster)
			}
		}
		if len(filteredClusters) == 0 {
			return CollectorResult{Status: StatusFailure, Error: fmt.Errorf("cluster %q not found", c.ClusterName), Warnings: warnings}
		}
	} else {
		filteredClusters = clusters.Items
	}

	if len(filteredClusters) == 0 {
		logger.Debug("⚠️  No clusters found")
		return CollectorResult{Status: StatusSuccess, FilesCreated: filesCreated, Warnings: warnings}
	}

	// Collect each cluster and its containers
	bundlePath := getBundlePath(ctx)
	collectSensitive := getCollectSensitiveData(ctx)

	for _, cluster := range filteredClusters {
		logger.Debug("✓ Processing cluster", "namespace", cluster.Namespace, "name", cluster.Name)
		// Dump WekaCluster resource
		yaml, err := collectObjectAsYAMLWithSensitiveData(&cluster, collectSensitive)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to marshal cluster %s/%s: %v", cluster.Namespace, cluster.Name, err))
			logger.Debug("⚠️  Failed to marshal cluster", "namespace", cluster.Namespace, "name", cluster.Name, "error", err)
			continue
		}

		filePath := filepath.Join("clusters", cluster.Name, GenerateSafeFileName("WekaCluster", cluster.Namespace, cluster.Name, "yaml"))
		if err := WriteToFile(bundlePath, filePath, yaml); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write cluster file for %s/%s: %v", cluster.Namespace, cluster.Name, err))
			logger.Debug("⚠️  Failed to write cluster file", "namespace", cluster.Namespace, "name", cluster.Name, "error", err)
			continue
		}
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected cluster resource", "namespace", cluster.Namespace, "name", cluster.Name)

		// Collect cluster instances information
		instancesFile, instancesWarning := c.collectClusterInstances(ctx, &cluster)
		if instancesFile != "" {
			filesCreated = append(filesCreated, instancesFile)
		}
		if instancesWarning != "" {
			warnings = append(warnings, instancesWarning)
		}

		// Collect WekaContainers for this cluster
		containerFiles, containerWarnings, err := c.collectClusterContainers(ctx, &cluster)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to collect containers for cluster %s/%s: %v", cluster.Namespace, cluster.Name, err))
			logger.Debug("⚠️  Failed to collect containers", "namespace", cluster.Namespace, "name", cluster.Name, "error", err)
		} else {
			filesCreated = append(filesCreated, containerFiles...)
			warnings = append(warnings, containerWarnings...)
			if len(containerFiles) > 0 {
				logger.Debug("✓ Collected pod files", "cluster", cluster.Name, "count", len(containerFiles))
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

func (c *ClusterResourcesCollector) collectClusterContainers(ctx context.Context, cluster *wekaapi.WekaCluster) ([]string, []string, error) {
	var filesCreated []string
	var warnings []string

	logger := logging.GetLogger(ctx)
	clients := getClients(ctx)
	bundlePath := getBundlePath(ctx)
	collectSensitive := getCollectSensitiveData(ctx)

	// List WekaContainers for this cluster using cached CRClient
	var containers wekaapi.WekaContainerList
	listOpts := []crclient.ListOption{
		crclient.InNamespace(cluster.Namespace),
		crclient.MatchingLabels{"weka.io/cluster-id": string(cluster.UID)},
	}

	if err := clients.CRClient.List(ctx, &containers, listOpts...); err != nil {
		return nil, nil, fmt.Errorf("failed to list WekaContainer resources: %w", err)
	}

	logger.Debug("Found WekaContainers for cluster", "cluster", cluster.Name, "count", len(containers.Items))

	// Collect WekaContainer resources and extract pod names
	var podNames []string
	for _, container := range containers.Items {
		// Dump WekaContainer resource
		yaml, err := collectObjectAsYAMLWithSensitiveData(&container, collectSensitive)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to marshal container %s/%s: %v", container.Namespace, container.Name, err))
			logger.Debug("⚠️  Failed to marshal container", "namespace", container.Namespace, "name", container.Name, "error", err)
			continue
		}

		filePath := filepath.Join("clusters", cluster.Name, "containers", GenerateSafeFileName("WekaContainer", container.Namespace, container.Name, "yaml"))
		if err := WriteToFile(bundlePath, filePath, yaml); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write container file for %s/%s: %v", container.Namespace, container.Name, err))
			logger.Debug("⚠️  Failed to write container file", "namespace", container.Namespace, "name", container.Name, "error", err)
			continue
		}
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected container resource", "namespace", container.Namespace, "name", container.Name)

		// Pod name matches container name
		if container.Name != "" {
			podNames = append(podNames, container.Name)
		}
	}

	// Collect pod logs in parallel (both current and previous)
	if len(podNames) > 0 {
		logsDir := filepath.Join("clusters", cluster.Name, "logs")
		logger.Debug("Collecting pod logs in parallel", "cluster", cluster.Name, "pods", len(podNames))

		logFiles, logWarnings := CollectPodLogsParallel(ctx, clients, cluster.Namespace, podNames, logsDir, 10)
		filesCreated = append(filesCreated, logFiles...)
		warnings = append(warnings, logWarnings...)

		logger.Debug("✓ Collected pod logs", "cluster", cluster.Name, "files", len(logFiles))

		// Collect pod descriptions in parallel
		descDir := filepath.Join("clusters", cluster.Name, "pods")
		logger.Debug("Collecting pod descriptions in parallel", "cluster", cluster.Name, "pods", len(podNames))

		descFiles, descWarnings := CollectPodDescriptionsParallel(ctx, clients, cluster.Namespace, podNames, descDir, 10)
		filesCreated = append(filesCreated, descFiles...)
		warnings = append(warnings, descWarnings...)

		logger.Debug("✓ Collected pod descriptions", "cluster", cluster.Name, "files", len(descFiles))
	}

	return filesCreated, warnings, nil
}

// collectClusterInstances collects cluster instances information for a specific cluster
func (c *ClusterResourcesCollector) collectClusterInstances(ctx context.Context, cluster *wekaapi.WekaCluster) (string, string) {
	logger := logging.GetLogger(ctx)
	bundlePath := getBundlePath(ctx)
	clients := getClients(ctx)

	logger.Debug("Collecting cluster instances", "cluster", cluster.Name, "namespace", cluster.Namespace)

	// Generate cluster instances output directly using the function
	p := printer.NewSupportBundlePrinter()
	output, err := getters.GetClusterInstancesOutput(
		ctx,
		clients,
		cluster.Namespace,
		false, // allNamespaces = false (we want this specific cluster only)
		cluster.Name,
		p,
	)
	if err != nil {
		warning := fmt.Sprintf("failed to collect cluster instances for %s/%s: %v", cluster.Namespace, cluster.Name, err)
		logger.Debug("⚠️  Failed to collect cluster instances", "cluster", cluster.Name, "namespace", cluster.Namespace, "error", err)
		return "", warning
	}

	// Write output to file
	fileName := fmt.Sprintf("cluster-instances-%s.txt", cluster.Name)
	filePath := filepath.Join("clusters", cluster.Name, fileName)
	if err := WriteToFile(bundlePath, filePath, output); err != nil {
		warning := fmt.Sprintf("failed to write cluster instances output for %s/%s: %v", cluster.Namespace, cluster.Name, err)
		logger.Debug("⚠️  Failed to write cluster instances file", "cluster", cluster.Name, "error", err)
		return "", warning
	}

	logger.Debug("✓ Collected cluster instances", "cluster", cluster.Name, "file", filePath)
	return filePath, ""
}
