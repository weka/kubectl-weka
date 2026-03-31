package supportbundle

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/hostcheck"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/printer"
	"log/slog"
	"path/filepath"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// NodesDescriptionCollector collects descriptions of all Kubernetes nodes
type NodesDescriptionCollector struct {
	NodeSelector string // Optional label selector to filter nodes
}

func (c *NodesDescriptionCollector) Name() string {
	return "Nodes Descriptions"
}

func (c *NodesDescriptionCollector) Start(ctx context.Context) {
	logger := GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "node descriptions for all nodes")
}

func (c *NodesDescriptionCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := GetLogger(ctx)
	clients := getClients(ctx)
	bundlePath := getBundlePath(ctx)

	logger.Debug("Collecting node descriptions")

	// Get all nodes, optionally filtered by selector
	var nodeList corev1.NodeList
	opts := []client.ListOption{}
	if c.NodeSelector != "" {
		opts = append(opts, client.MatchingLabels(kubernetes.ParseSelector(c.NodeSelector)))
	}
	if err := clients.CRClient.List(ctx, &nodeList, opts...); err != nil {
		return CollectorResult{
			Status: StatusFailure,
			Error:  fmt.Errorf("failed to list nodes: %w", err),
		}
	}

	if len(nodeList.Items) == 0 {
		logger.Debug("No nodes found")
		return CollectorResult{Status: StatusSuccess, FilesCreated: filesCreated}
	}

	logger.Debug("Found nodes", "count", len(nodeList.Items))

	// Collect nodes table output
	nodesTableFile, nodesTableWarning := c.collectNodesTable(ctx)
	if nodesTableFile != "" {
		filesCreated = append(filesCreated, nodesTableFile)
	}
	if nodesTableWarning != "" {
		warnings = append(warnings, nodesTableWarning)
	}

	// Collect descriptions in parallel
	descFiles, descWarnings := collectNodeDescriptionsParallel(ctx, &nodeList.Items, bundlePath, logger)
	filesCreated = append(filesCreated, descFiles...)
	warnings = append(warnings, descWarnings...)

	// Collect host checks and dump as JSON
	hostCheckFiles, hostCheckWarnings := c.collectHostChecks(ctx, &nodeList.Items, bundlePath, logger)
	filesCreated = append(filesCreated, hostCheckFiles...)
	warnings = append(warnings, hostCheckWarnings...)

	// Determine status
	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}
}

// collectNodesTable collects the output of get nodes command
func (c *NodesDescriptionCollector) collectNodesTable(ctx context.Context) (string, string) {
	logger := GetLogger(ctx)
	bundlePath := getBundlePath(ctx)
	clients := getClients(ctx)

	logger.Debug("Collecting nodes table", "nodeSelector", c.NodeSelector)

	p := printer.NewSupportBundlePrinter()

	// Generate nodes table output with node selector
	output, err := getters.GenerateNodesOutput(ctx, clients, p, c.NodeSelector)
	if err != nil {
		warning := fmt.Sprintf("failed to collect nodes table: %v", err)
		logger.Debug("⚠️  Failed to collect nodes table", "error", err)
		return "", warning
	}

	// Write output to file
	filePath := filepath.Join("nodes", "nodes-table.txt")
	if err := WriteToFile(bundlePath, filePath, output); err != nil {
		warning := fmt.Sprintf("failed to write nodes table file: %v", err)
		logger.Debug("⚠️  Failed to write nodes table file", "error", err)
		return "", warning
	}

	logger.Debug("✓ Collected nodes table", "file", filePath)
	return filePath, ""
}

// collectHostChecks runs host checks on all nodes and dumps results as JSON
// Uses the HostCheckModuleRegistry with caching to avoid redundant host check execution
func (c *NodesDescriptionCollector) collectHostChecks(ctx context.Context, nodes *[]corev1.Node, bundlePath string, logger *slog.Logger) ([]string, []string) {
	if len(*nodes) == 0 {
		return []string{}, []string{}
	}
	clients := getClients(ctx)
	logger.Debug("Running host checks on nodes", "count", len(*nodes))

	// Use GlobalHostCheckRegistry to get cached results when available
	// This avoids re-running hostchecks if they were already executed elsewhere
	hostChecksMap, err := hostcheck.GlobalHostCheckRegistry.GetHostChecksForNodes(ctx, clients, *nodes)
	if err != nil {
		warning := fmt.Sprintf("failed to run host checks: %v", err)
		logger.Debug("⚠️  Failed to run host checks", "error", err)
		return []string{}, []string{warning}
	}

	// Dump host checks as pretty-printed JSON files
	var filesCreated []string
	var warnings []string

	for nodeName, hostCheckResult := range hostChecksMap {
		if hostCheckResult == nil {
			warning := fmt.Sprintf("no host check result for node %s", nodeName)
			logger.Debug("⚠️  No host check result", "node", nodeName)
			warnings = append(warnings, warning)
			continue
		}

		// Marshal to pretty-printed JSON
		jsonData, err := json.MarshalIndent(hostCheckResult, "", "  ")
		if err != nil {
			warning := fmt.Sprintf("failed to marshal host check result for node %s: %v", nodeName, err)
			logger.Debug("⚠️  Failed to marshal host check", "node", nodeName, "error", err)
			warnings = append(warnings, warning)
			continue
		}

		// Write to file in nodes/hostchecks directory
		filePath := filepath.Join("node-hostchecks", fmt.Sprintf("%s_hostcheck.json", nodeName))
		if err := WriteToFile(bundlePath, filePath, string(jsonData)); err != nil {
			warning := fmt.Sprintf("failed to write host check file for node %s: %v", nodeName, err)
			logger.Debug("⚠️  Failed to write host check file", "node", nodeName, "error", err)
			warnings = append(warnings, warning)
			continue
		}

		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected host check", "node", nodeName, "file", filePath)
	}

	return filesCreated, warnings
}

func (c *NodesDescriptionCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := GetLogger(ctx)

	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files", len(result.FilesCreated))
		if len(result.Warnings) > 0 {
			logger.Info("Non-fatal warnings found", "count", len(result.Warnings))
			for _, warning := range result.Warnings {
				logger.Info("Warning", "message", warning)
			}
		}
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

// collectNodeDescriptionsParallel collects descriptions from all nodes in parallel
func collectNodeDescriptionsParallel(ctx context.Context, nodes *[]corev1.Node, bundlePath string, logger *slog.Logger) ([]string, []string) {
	if len(*nodes) == 0 {
		return []string{}, []string{}
	}

	maxConcurrency := 10
	semaphore := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	resultsChan := make(chan nodeDescriptionResult, len(*nodes))

	// Collect from each node in parallel
	for _, node := range *nodes {
		wg.Add(1)
		go func(n corev1.Node) {
			defer wg.Done()
			semaphore <- struct{}{}        // acquire
			defer func() { <-semaphore }() // release

			result := collectSingleNodeDescription(n, bundlePath, logger)
			resultsChan <- result
		}(node)
	}

	// Wait for all goroutines and close results channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Aggregate results
	var filesCreated []string
	var warnings []string

	for result := range resultsChan {
		if result.Error != nil {
			warnings = append(warnings, fmt.Sprintf("failed to collect description for node %s: %v", result.NodeName, result.Error))
			logger.Debug("⚠️  Failed to collect node description", "node", result.NodeName, "error", result.Error)
		} else {
			filesCreated = append(filesCreated, result.FilePath)
			logger.Debug("✓ Collected node description", "node", result.NodeName, "file", result.FilePath)
		}
	}

	return filesCreated, warnings
}

type nodeDescriptionResult struct {
	NodeName string
	FilePath string
	Error    error
}

// collectSingleNodeDescription collects the description for a single node
func collectSingleNodeDescription(node corev1.Node, bundlePath string, logger *slog.Logger) nodeDescriptionResult {
	result := nodeDescriptionResult{
		NodeName: node.Name,
	}

	// Convert node to YAML for description
	yamlData, err := yaml.Marshal(&node)
	if err != nil {
		result.Error = fmt.Errorf("failed to marshal node to YAML: %w", err)
		return result
	}

	// Write to file
	filePath := filepath.Join("nodes", fmt.Sprintf("%s_describe.yaml", node.Name))
	if err := WriteToFile(bundlePath, filePath, string(yamlData)); err != nil {
		result.Error = fmt.Errorf("failed to write node description file: %w", err)
		return result
	}

	result.FilePath = filePath
	return result
}
