package supportbundle

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"os"
	"path/filepath"
	"sync"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

// CSIPodLogsCollector collects logs from all CSI pods
type CSIPodLogsCollector struct{}

func (c *CSIPodLogsCollector) Name() string {
	return "CSI Pod Logs"
}

func (c *CSIPodLogsCollector) Start(ctx context.Context) {
	logger := GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect",
		"items", "Current and previous logs from all CSI pods")
}

func (c *CSIPodLogsCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := GetLogger(ctx)
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files_created", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files_created", len(result.FilesCreated), "warnings", len(result.Warnings))
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

func (c *CSIPodLogsCollector) Collect(ctx context.Context) CollectorResult {
	logger := GetLogger(ctx)
	bundlePath := getBundlePath(ctx)
	var filesCreated []string
	var warnings []string

	// Create logs directory in the bundle
	logsDir := filepath.Join(bundlePath, "csi", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		logger.Error("Failed to create logs directory", "error", err)
		return CollectorResult{
			Status:       StatusFailure,
			FilesCreated: filesCreated,
			Error:        err,
		}
	}

	// Get all CSI pods
	logger.Debug("Fetching CSI pods")
	pods, err := getAllCSIPods(ctx)
	if err != nil {
		logger.Error("Failed to fetch CSI pods", "error", err)
		return CollectorResult{
			Status:       StatusFailure,
			FilesCreated: filesCreated,
			Error:        err,
		}
	}

	if len(pods) == 0 {
		logger.Info("No CSI pods found")
		return CollectorResult{
			Status:       StatusSuccess,
			FilesCreated: filesCreated,
			Warnings:     []string{"No CSI pods found"},
		}
	}

	logger.Debug("Found CSI pods", "count", len(pods))

	// Collect logs from all pods in parallel
	logsChan := make(chan podLogResult, len(pods))
	var wg sync.WaitGroup

	for _, pod := range pods {
		wg.Add(1)
		go func(p *corev1.Pod) {
			defer wg.Done()
			c.collectPodLogs(ctx, logsDir, p, logsChan)
		}(pod)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(logsChan)

	// Process results
	for result := range logsChan {
		if result.err != nil {
			warnings = append(warnings, result.err.Error())
			logger.Warn("Failed to collect logs", "pod", result.podName, "error", result.err)
		} else {
			filesCreated = append(filesCreated, result.filesCreated...)
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

	logger.Info("Collected logs", "files", len(filesCreated), "warnings", len(warnings))

	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}
}

type podLogResult struct {
	podName      string
	filesCreated []string
	err          error
}

func (c *CSIPodLogsCollector) collectPodLogs(ctx context.Context, baseDir string, pod *corev1.Pod, results chan<- podLogResult) {
	logger := GetLogger(ctx)
	clients := getClients(ctx)
	result := podLogResult{podName: pod.Name}

	// Determine driver name and role
	driverName := kubernetes.GetCSIDriverFromPod(pod)
	if driverName == "" {
		result.err = fmt.Errorf("could not determine CSI driver name for pod %s/%s", pod.Namespace, pod.Name)
		results <- result
		return
	}

	role := getters.DeterminePodRole(pod)
	if role == "" {
		result.err = fmt.Errorf("could not determine pod role for pod %s/%s", pod.Namespace, pod.Name)
		results <- result
		return
	}

	// Create directory structure: logs/drivername/role/podname
	podLogsDir := filepath.Join(baseDir, driverName, role, pod.Name)
	if err := os.MkdirAll(podLogsDir, 0755); err != nil {
		result.err = fmt.Errorf("failed to create pod logs directory %s: %w", podLogsDir, err)
		results <- result
		return
	}

	// Collect current logs using existing function
	currentLogs, _, err := collectPodLogs(ctx, clients, pod.Namespace, pod.Name, "", &corev1.PodLogOptions{})
	if err != nil {
		result.err = fmt.Errorf("failed to get current logs for pod %s/%s: %w", pod.Namespace, pod.Name, err)
		logger.Warn("Failed to collect current logs", "pod", pod.Name, "error", err)
		results <- result
		return
	}

	// Write current logs for each container
	for containerName, logData := range currentLogs {
		currentFile := filepath.Join(podLogsDir, fmt.Sprintf("%s.log", containerName))
		if err := os.WriteFile(currentFile, []byte(logData), 0644); err != nil {
			result.err = fmt.Errorf("failed to write current logs file %s: %w", currentFile, err)
			logger.Warn("Failed to write current logs", "file", currentFile, "error", err)
		} else {
			result.filesCreated = append(result.filesCreated, currentFile)
			logger.Debug("Collected current logs", "pod", pod.Name, "container", containerName, "file", currentFile)
		}
	}
	// Collect previous logs if pod has restarted
	metrics := kubernetes.GetPodRestartMetrics(pod)
	if metrics.RestartCount > 0 {
		previousLogs, _, err := collectPodLogs(ctx, clients, pod.Namespace, pod.Name, "", &corev1.PodLogOptions{Previous: true})
		if err != nil {
			// Don't fail on previous logs - they might not exist
			logger.Debug("Previous logs not available", "pod", pod.Name, "error", err)
		} else {
			// Write previous logs for each container
			for containerName, logData := range previousLogs {
				previousFile := filepath.Join(podLogsDir, fmt.Sprintf("%s.previous.log", containerName))
				if err := os.WriteFile(previousFile, []byte(logData), 0644); err != nil {
					logger.Warn("Failed to write previous logs", "file", previousFile, "error", err)
				} else {
					result.filesCreated = append(result.filesCreated, previousFile)
					logger.Debug("Collected previous logs", "pod", pod.Name, "container", containerName, "file", previousFile)
				}
			}
		}
	}

	results <- result
}

// getAllCSIPods returns all pods belonging to CSI drivers
func getAllCSIPods(ctx context.Context) ([]*corev1.Pod, error) {
	clients := getClients(ctx)
	crClient := clients.CRClient

	// List all pods across all namespaces
	var podList corev1.PodList
	if err := crClient.List(ctx, &podList); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Get all CSI drivers to build a map
	var csiDriverList storagev1.CSIDriverList
	if err := crClient.List(ctx, &csiDriverList); err != nil {
		return nil, fmt.Errorf("failed to list CSI drivers: %w", err)
	}

	// Build a map of Weka CSI drivers
	driverMap := make(map[string]bool)
	for _, driver := range csiDriverList.Items {
		if kubernetes.IsWekaCSI(driver.Name) {
			driverMap[driver.Name] = true
		}
	}

	if len(driverMap) == 0 {
		return nil, nil
	}

	// Filter pods that belong to CSI drivers
	var csiPods []*corev1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		if getters.IsPodBelongsToCSIDriver(pod, driverMap) {
			csiPods = append(csiPods, pod)
		}
	}

	return csiPods, nil
}
