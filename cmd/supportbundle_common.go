package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	"log/slog"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

// CollectionMode defines which collectors to run
type CollectionMode string

const (
	ModeOperator CollectionMode = "operator"
	ModeCluster  CollectionMode = "cluster"
	ModeClient   CollectionMode = "client"
	ModeCSI      CollectionMode = "csi"
	ModeK8s      CollectionMode = "k8s"
	ModeAll      CollectionMode = "all"
)

// Global shared variables for support bundle commands
var (
	supportBundleCaseID           string
	supportBundleOutput           string
	supportBundleNamespace        string
	supportBundleAllNS            bool
	supportBundleNodeSel          string // For k8s mode
	supportBundleIncludeSensitive bool   // Include sensitive data (secrets, configs, etc.)
	supportBundleDebug            bool   // Enable debug output

	// Global logger for the current bundle collection
	bundleLogger *slog.Logger
)

// newMultiSinkLogger creates a logger that outputs to both console (stderr) and a file
// The console handler logs at the configured level (info or debug based on supportBundleDebug)
// The file handler always logs at debug level to capture all information
// Format: [HH:MM:SS] LEVEL message
func newMultiSinkLogger(logFilePath string) (*slog.Logger, *os.File, error) {
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Determine console log level based on debug flag
	consoleLevel := slog.LevelInfo
	if supportBundleDebug {
		consoleLevel = slog.LevelDebug
	}

	// Create handlers with custom formatter
	consoleHandler := &simpleHandler{
		writer: os.Stderr,
		level:  consoleLevel,
	}

	fileHandler := &simpleHandler{
		writer: logFile,
		level:  slog.LevelDebug, // Always capture debug in file
	}

	// Create a multi-handler by wrapping both handlers
	multiHandler := &multiHandler{
		console: consoleHandler,
		file:    fileHandler,
	}

	return slog.New(multiHandler), logFile, nil
}

type simpleHandler struct {
	writer io.Writer
	level  slog.Level
}

func (sh *simpleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= sh.level
}

func (sh *simpleHandler) Handle(ctx context.Context, r slog.Record) error {
	if !sh.Enabled(ctx, r.Level) {
		return nil
	}

	// ANSI color codes
	reset, dim := "\033[0m", "\033[90m"
	red, yellow, blue := "\033[31m", "\033[33m", "\033[36m"

	var levelColor, levelStr string
	switch r.Level {
	case slog.LevelDebug:
		levelColor, levelStr = dim, "DEBUG"
	case slog.LevelInfo:
		levelColor, levelStr = blue, "INFO"
	case slog.LevelWarn:
		levelColor, levelStr = yellow, "WARN"
	case slog.LevelError:
		levelColor, levelStr = red, "ERROR"
	default:
		levelColor, levelStr = reset, r.Level.String()
	}

	timestamp := r.Time.Format("15:04:05")
	line := fmt.Sprintf("%s[%s]%s %s%s%s %s", dim, timestamp, reset, levelColor, levelStr, reset, r.Message)

	if r.NumAttrs() > 0 {
		r.Attrs(func(a slog.Attr) bool {
			line += fmt.Sprintf(" %s=%v", a.Key, a.Value)
			return true
		})
	}

	line += "\n"
	_, err := io.WriteString(sh.writer, line)
	return err
}

func (sh *simpleHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return sh
}

func (sh *simpleHandler) WithGroup(_ string) slog.Handler {
	return sh
}

// multiHandler implements slog.Handler to write to multiple sinks
type multiHandler struct {
	console slog.Handler
	file    slog.Handler
}

func (mh *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return mh.console.Enabled(ctx, level) || mh.file.Enabled(ctx, level)
}

func (mh *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	if mh.console.Enabled(ctx, r.Level) {
		if err := mh.console.Handle(ctx, r); err != nil {
			return err
		}
	}
	if mh.file.Enabled(ctx, r.Level) {
		if err := mh.file.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (mh *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &multiHandler{
		console: mh.console.WithAttrs(attrs),
		file:    mh.file.WithAttrs(attrs),
	}
}

func (mh *multiHandler) WithGroup(name string) slog.Handler {
	return &multiHandler{
		console: mh.console.WithGroup(name),
		file:    mh.file.WithGroup(name),
	}
}

// collectorsByMode returns the list of collectors for a given mode
func collectorsByMode(mode CollectionMode, resourceName string) []Collector {
	// Nodes descriptions collector is always included with optional node selector
	nodesCollector := &NodesDescriptionCollector{NodeSelector: supportBundleNodeSel}

	switch mode {
	case ModeOperator:
		return []Collector{
			nodesCollector,
			&OperatorLogsCollector{},
			&OperatorNodeAgentLogsCollector{},
			&OperatorResourcesCollector{},
		}

	case ModeCluster:
		return []Collector{
			nodesCollector,
			&ClusterResourcesCollector{ClusterName: resourceName},
		}

	case ModeClient:
		return []Collector{
			nodesCollector,
			&ClientResourcesCollector{ClientName: resourceName},
		}

	case ModeCSI:
		return []Collector{
			nodesCollector,
			&CSIResourcesCollector{},
			&CSIPodLogsCollector{},
			&CSISecretsCollector{},
			&StorageClassCollector{},
			&PersistentVolumeClaimCollector{},
			&PersistentVolumeCollector{},
			&CSIDriverCollector{},
		}

	case ModeK8s:
		return []Collector{
			nodesCollector,
			&K8sPreflightCollector{NodeSelector: supportBundleNodeSel},
		}

	case ModeAll:
		return []Collector{
			nodesCollector,
			&OperatorLogsCollector{},
			&OperatorNodeAgentLogsCollector{},
			&OperatorResourcesCollector{},
			&ClusterResourcesCollector{ClusterName: ""},
			&ClientResourcesCollector{ClientName: ""},
			&K8sPreflightCollector{NodeSelector: supportBundleNodeSel},
			&CSIResourcesCollector{},
			&CSIPodLogsCollector{},
			&CSISecretsCollector{},
			&StorageClassCollector{},
			&PersistentVolumeClaimCollector{},
			&PersistentVolumeCollector{},
			&CSIDriverCollector{},
		}

	default:
		return []Collector{}
	}
}

// runSupportBundleByMode is the unified handler for all support-bundle subcommands
func runSupportBundleByMode(mode CollectionMode, resourceName, namespace string, allNamespaces bool) error {
	// Warn if collecting sensitive data
	if supportBundleIncludeSensitive {
		printSensitiveDataWarning()
	}

	// For certain modes, override namespace settings

	if mode == ModeK8s {
		// K8s mode doesn't use namespace filtering
		namespace = ""
		allNamespaces = false
	}

	collectors := collectorsByMode(mode, resourceName)
	return runSupportBundleCommand(
		supportBundleCaseID,
		supportBundleOutput,
		namespace,
		allNamespaces,
		collectors,
	)
}

// printSensitiveDataWarning prints a warning about sensitive data collection
func printSensitiveDataWarning() {
	fmt.Println()
	fmt.Println("================================================================================")
	fmt.Println("⚠️  WARNING: SENSITIVE DATA COLLECTION ENABLED")
	fmt.Println("================================================================================")
	fmt.Println()
	fmt.Println("This support bundle will include SENSITIVE INFORMATION such as:")
	fmt.Println("  • Kubernetes Secrets (including credentials, API keys, etc.)")
	fmt.Println("  • Configuration data (potentially containing sensitive values)")
	fmt.Println("  • Environment variables (may contain sensitive information)")
	fmt.Println()
	fmt.Println("⚠️  SECURITY WARNING:")
	fmt.Println("  • This bundle will contain UNENCRYPTED sensitive data")
	fmt.Println("  • Do NOT share this bundle publicly or insecurely")
	fmt.Println("  • Only share with TRUSTED support personnel via SECURE channels")
	fmt.Println("  • Consider ENCRYPTING this archive before transmission")
	fmt.Println("  • ROTATE any exposed secrets/credentials after sharing")
	fmt.Println()
	fmt.Println("================================================================================")
	fmt.Println()
}

// createTempDir creates a temporary directory with the given prefix
func createTempDir(prefix string) (string, error) {
	tempDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	if supportBundleDebug {
		bundleLogger.Debug("Created temporary directory", "path", tempDir)
	}
	return tempDir, nil
}

// cleanupTempDir removes a temporary directory and all its contents
func cleanupTempDir(dir string) {
	_ = os.RemoveAll(dir)
}

// createTarGz creates a tar.gz archive from a source directory
func createTarGz(sourceDir, targetPath string) error {
	if supportBundleDebug {
		bundleLogger.Debug("Creating archive", "path", targetPath)
	}

	// Create output file
	outFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Walk through source directory
	fileCount := 0
	err = filepath.Walk(sourceDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, filePath)
		if err != nil {
			return err
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Write file content if it's a regular file
		if info.Mode().IsRegular() {
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
			fileCount++
		}

		return nil
	})

	if err == nil && supportBundleDebug {
		fmt.Printf("✓ Archived %d file(s)\n", fileCount)
	}

	return err
}

// runSupportBundleCommand is a common function to run collectors and create archive
func runSupportBundleCommand(caseID, output, namespace string, allNamespaces bool, collectors []Collector) error {
	bgCtx := context.Background()

	// Generate bundle name first
	bundleName := generateBundleName(caseID)

	// Create a temporary parent directory first just for the log file
	parentTempDir, err := os.MkdirTemp("", "weka-bundle-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer cleanupTempDir(parentTempDir)

	// Create logger in parent temp dir so it can capture all output
	logFilePath := filepath.Join(parentTempDir, "collection.log")
	logger, logFile, logErr := newMultiSinkLogger(logFilePath)
	if logErr != nil {
		logger = slog.Default()
		logger.Warn("Failed to create log file", "error", logErr)
	}
	bundleLogger = logger
	defer func() {
		if logFile != nil {
			_ = logFile.Close()
		}
	}()

	logger.Info("Support Bundle Collection Started")
	logger.Info("Bundle Name", "name", bundleName)
	if caseID != "" {
		logger.Info("Case ID", "id", caseID)
	}
	if supportBundleDebug {
		logger.Debug("Debug mode enabled")
	}

	// Now create the main bundle temp directory with logger active
	tempDir, err := createTempDir(bundleName)
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer cleanupTempDir(tempDir)

	// Copy collection.log to the main bundle directory
	collectionLogSrc := logFilePath
	collectionLogDst := filepath.Join(tempDir, "collection.log")

	logger.Info("Collecting support bundle data")

	// Determine namespace if not set
	if namespace == "" && !allNamespaces {
		namespace, err = GetKubeNamespace()
		if err != nil {
			return err
		}
	}

	// Create context with all necessary values
	ctx := bgCtx
	ctx = withClients(ctx, KubeClients)
	ctx = withBundlePath(ctx, tempDir)
	ctx = withNamespace(ctx, namespace)
	ctx = withAllNamespaces(ctx, allNamespaces)
	ctx = withCollectSensitiveData(ctx, supportBundleIncludeSensitive)
	ctx = withLogger(ctx, bundleLogger)

	// Run all collectors
	totalSuccess := 0
	totalPartial := 0
	totalFailed := 0
	for _, collector := range collectors {
		// Start reporting
		collector.Start(ctx)

		// Run collection
		result := collector.Collect(ctx)

		// Finish reporting
		collector.Finish(ctx, result)

		// Track statistics by status
		switch result.Status {
		case StatusSuccess:
			totalSuccess++
		case StatusPartial:
			totalPartial++
		case StatusFailure:
			totalFailed++
		}
	}

	summaryMsg := fmt.Sprintf("Collection complete: %d succeeded, %d partial, %d failed", totalSuccess, totalPartial, totalFailed)
	logger.Info(summaryMsg)

	// Copy collection.log to bundle directory before archiving
	logger.Info("Creating archive...")
	if logFile != nil {
		logFile.Close() // Close first so file is flushed
	}

	// Copy the log file to the bundle directory
	logContent, err := os.ReadFile(collectionLogSrc)
	if err == nil {
		_ = os.WriteFile(collectionLogDst, logContent, 0644)
	}

	// Create archive
	archivePath := filepath.Join(output, bundleName+".tar.gz")
	if err := createTarGz(tempDir, archivePath); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	successMsg := fmt.Sprintf("Support bundle created: %s", archivePath)
	logger.Info(successMsg)
	return nil
}

// PodLogCollectionResult holds the result of collecting logs from a single pod
type PodLogCollectionResult struct {
	PodName  string
	LogFiles []string // relative paths created
	Warnings []string
	Error    error
}

// CollectPodLogsParallel collects logs from multiple pods in parallel
// It collects current and previous logs for all containers
// logsDir: subdirectory path within bundle (e.g., "operator/logs" or "cluster/container-logs")
// Pods on NotReady nodes are filtered out to avoid timeouts on inaccessible containers
func CollectPodLogsParallel(ctx context.Context, clients *K8sClients, namespace string, pods []string, logsDir string, maxConcurrency int) ([]string, []string) {
	if len(pods) == 0 {
		return []string{}, []string{}
	}

	logger := getLogger(ctx)
	bundlePath := getBundlePath(ctx)

	// First, get all nodes using cached controller-runtime client and build a map of ready nodes
	var nodeList corev1.NodeList
	err := clients.CRClient.List(ctx, &nodeList)
	if err != nil {
		logger.Warn("Failed to list nodes, proceeding without node readiness check", "error", err)
		// Continue anyway - filter will be skipped
	}

	readyNodes := make(map[string]bool)
	if len(nodeList.Items) > 0 {
		for _, node := range nodeList.Items {
			readyNodes[node.Name] = IsNodeReady(node)
		}
		logger.Debug("Node readiness check", "totalNodes", len(nodeList.Items), "readyNodes", len(readyNodes))
	}

	// Filter pods: only include pods on ready nodes
	var validPods []string
	var skippedPods []string
	for _, podName := range pods {
		// Try to get pod to check its node using cached client
		var pod corev1.Pod
		err := clients.CRClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: podName}, &pod)
		if err != nil {
			// Pod not found - it may have been deleted, skip it
			skippedPods = append(skippedPods, podName)
			logger.Debug("Pod not found, skipping", "pod", podName)
			continue
		}

		// Check if node is ready
		if len(readyNodes) > 0 {
			if isReady, exists := readyNodes[pod.Spec.NodeName]; exists && !isReady {
				skippedPods = append(skippedPods, podName)
				logger.Debug("Pod on NotReady node, skipping", "pod", podName, "node", pod.Spec.NodeName)
				continue
			}
		}

		validPods = append(validPods, podName)
	}

	if len(skippedPods) > 0 {
		logger.Info("Skipped pods on NotReady nodes", "count", len(skippedPods), "total", len(pods))
	}

	// Return early if all pods were filtered out
	if len(validPods) == 0 {
		logger.Warn("No valid pods to collect logs from", "skipped", len(skippedPods))
		return []string{}, []string{}
	}

	// Limit concurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 5 // default
	}
	semaphore := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	resultsChan := make(chan PodLogCollectionResult, len(validPods))

	// Collect from each pod in parallel
	for _, podName := range validPods {
		wg.Add(1)
		go func(pod string) {
			defer wg.Done()
			semaphore <- struct{}{}        // acquire
			defer func() { <-semaphore }() // release

			result := collectSinglePodLogs(ctx, clients, namespace, pod, logsDir, bundlePath, logger)
			resultsChan <- result
		}(podName)
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
			warnings = append(warnings, fmt.Sprintf("failed to collect logs from pod %s: %v", result.PodName, result.Error))
			logger.Debug("⚠️  Failed to collect logs", "pod", result.PodName, "error", result.Error)
		} else {
			filesCreated = append(filesCreated, result.LogFiles...)
			logger.Debug("✓ Collected logs", "pod", result.PodName, "files", len(result.LogFiles))
		}
		warnings = append(warnings, result.Warnings...)
	}

	return filesCreated, warnings
}

// collectSinglePodLog collects logs from a single pod and returns results
func collectSinglePodLogs(ctx context.Context, clients *K8sClients, namespace, podName, logsDir string, bundlePath string, logger *slog.Logger) PodLogCollectionResult {
	result := PodLogCollectionResult{
		PodName:  podName,
		LogFiles: []string{},
		Warnings: []string{},
	}

	// Collect current logs
	logs, _, err := collectPodLogs(ctx, clients, namespace, podName, "", nil)
	if err != nil {
		result.Error = err
		return result
	}

	for containerName, logContent := range logs {
		filePath := filepath.Join(logsDir, fmt.Sprintf("%s_%s.log", podName, containerName))
		if err := writeToFile(bundlePath, filePath, logContent); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write log file for %s/%s: %v", podName, containerName, err))
			logger.Debug("⚠️  Failed to write log file", "pod", podName, "container", containerName, "error", err)
			continue
		}
		result.LogFiles = append(result.LogFiles, filePath)
		logger.Debug("✓ Collected logs", "pod", podName, "container", containerName, "bytes", len(logContent))
	}

	// Collect previous logs if any containers have restarted
	previousLogs, previousUnavailable, err := collectPodLogs(ctx, clients, namespace, podName, "", &corev1.PodLogOptions{Previous: true})
	if err != nil {
		// This is a real error (not just "previous logs don't exist")
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to collect previous logs from pod %s: %v", podName, err))
		logger.Debug("⚠️  Failed to collect previous logs", "pod", podName, "error", err)
	} else if previousUnavailable {
		// Previous logs don't exist - this is normal for pods that haven't restarted
		logger.Debug("Previous logs unavailable (container hasn't restarted)", "pod", podName)
	} else {
		// Successfully collected previous logs
		for containerName, logContent := range previousLogs {
			if logContent != "" {
				prevFilePath := filepath.Join(logsDir, fmt.Sprintf("%s_%s.previous.log", podName, containerName))
				if err := writeToFile(bundlePath, prevFilePath, logContent); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write previous log file for %s/%s: %v", podName, containerName, err))
					logger.Debug("⚠️  Failed to write previous log file", "pod", podName, "container", containerName, "error", err)
					continue
				}
				result.LogFiles = append(result.LogFiles, prevFilePath)
				logger.Debug("✓ Collected previous logs", "pod", podName, "container", containerName, "bytes", len(logContent))
			}
		}
	}

	return result
}

// PodDescriptionCollectionResult holds the result of collecting description from a single pod
type PodDescriptionCollectionResult struct {
	PodName  string
	FilePath string // relative path created
	Warning  string
	Error    error
}

// CollectPodDescriptionsParallel collects descriptions from multiple pods in parallel
// descDir: subdirectory path within bundle (e.g., "clusters/cluster-name/pods" or "operator/pods")
func CollectPodDescriptionsParallel(ctx context.Context, clients *K8sClients, namespace string, pods []string, descDir string, maxConcurrency int) ([]string, []string) {
	if len(pods) == 0 {
		return []string{}, []string{}
	}

	logger := getLogger(ctx)
	bundlePath := getBundlePath(ctx)

	// Limit concurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 10 // default
	}
	semaphore := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	resultsChan := make(chan PodDescriptionCollectionResult, len(pods))

	// Collect from each pod in parallel
	for _, podName := range pods {
		wg.Add(1)
		go func(pod string) {
			defer wg.Done()
			semaphore <- struct{}{}        // acquire
			defer func() { <-semaphore }() // release

			result := collectSinglePodDescription(ctx, clients, namespace, pod, descDir, bundlePath, logger)
			resultsChan <- result
		}(podName)
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
			warnings = append(warnings, fmt.Sprintf("failed to collect description from pod %s: %v", result.PodName, result.Error))
			logger.Debug("⚠️  Failed to collect pod description", "pod", result.PodName, "error", result.Error)
		} else {
			filesCreated = append(filesCreated, result.FilePath)
			logger.Debug("✓ Collected pod description", "pod", result.PodName)
		}
		if result.Warning != "" {
			warnings = append(warnings, result.Warning)
		}
	}

	return filesCreated, warnings
}

// collectSinglePodDescription collects description from a single pod and returns result
func collectSinglePodDescription(ctx context.Context, clients *K8sClients, namespace, podName, descDir string, bundlePath string, logger *slog.Logger) PodDescriptionCollectionResult {
	result := PodDescriptionCollectionResult{
		PodName: podName,
	}

	// Collect pod description
	desc, err := collectPodDescription(ctx, clients, namespace, podName)
	if err != nil {
		result.Error = err
		return result
	}

	// Write description to file
	filePath := filepath.Join(descDir, fmt.Sprintf("%s_describe.yaml", podName))
	if err := writeToFile(bundlePath, filePath, desc); err != nil {
		result.Error = fmt.Errorf("failed to write pod description for %s: %w", podName, err)
		logger.Debug("⚠️  Failed to write pod description file", "pod", podName, "error", err)
		return result
	}

	result.FilePath = filePath
	return result
}
