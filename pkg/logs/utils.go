package logs

import (
	"bufio"
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetes2 "k8s.io/client-go/kubernetes"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strings"
	"sync"
	"time"
)

// applyContainerFilters applies role, name, and ID filters to containers
func applyContainerFilters(containers []v1alpha1.WekaContainer, opts WekaLogsOptions) []v1alpha1.WekaContainer {
	result := make([]v1alpha1.WekaContainer, 0)

	validRoles := map[string]bool{
		"compute": true,
		"s3":      true,
		"drive":   true,
		"envoy":   true,
		"nfs":     true,
		"client":  true,
	}

	for i := range containers {
		container := containers[i]

		// Filter by role if specified
		if opts.Role != "" {
			if !validRoles[opts.Role] {
				continue
			}

			// Check if container mode matches the role
			switch opts.Role {
			case "compute":
				if container.Spec.Mode != v1alpha1.WekaContainerModeCompute {
					continue
				}
			case "s3":
				if container.Spec.Mode != v1alpha1.WekaContainerModeS3 {
					continue
				}
			case "drive":
				if container.Spec.Mode != v1alpha1.WekaContainerModeDrive {
					continue
				}
			case "envoy":
				if container.Spec.Mode != v1alpha1.WekaContainerModeEnvoy {
					continue
				}
			case "nfs":
				if container.Spec.Mode != v1alpha1.WekaContainerModeNfs {
					continue
				}
			case "frontend", "client":
				if container.Spec.Mode != v1alpha1.WekaContainerModeClient {
					continue
				}
			}
		}

		// Filter by container name if specified
		if opts.ContainerName != "" {
			if container.Name != opts.ContainerName {
				continue
			}
		}

		// Filter by container ID if specified
		if opts.ContainerID >= 0 {
			if container.Status.ClusterContainerID != nil && (*container.Status.ClusterContainerID != opts.ContainerID) {
				continue
			}
		}

		result = append(result, container)
	}

	return result
}

// containerShouldHavePod returns true for containers that are supposed to have pod running
func containerShouldHavePod(container v1alpha1.WekaContainer) bool {
	switch container.Status.Status {
	case v1alpha1.PodNotRunning, v1alpha1.Init, v1alpha1.WaitForDrivers, v1alpha1.Stopped, v1alpha1.Paused, v1alpha1.Destroying, v1alpha1.Completed:
		return false
	}
	return true
}

// getPodsForContainers finds pods with the same name as the WekaContainers
func getPodsForContainers(ctx context.Context, clients *kubernetes.K8sClients, containers []v1alpha1.WekaContainer) (map[string]*v1.Pod, error) {
	podMap := make(map[string]*v1.Pod)
	crclient := clients.CRClient
	for _, container := range containers {
		if !containerShouldHavePod(container) {
			continue
		}
		var pod v1.Pod

		err := crclient.Get(ctx, types.NamespacedName{Namespace: container.Namespace, Name: container.Name}, &pod)
		if err != nil {
			continue
		}
		podMap[container.Name] = &pod
	}
	return podMap, nil
}

// filterPodsByNodeSelector filters pods based on node labels matching the nodeSelector
// nodeSelector should be comma-separated key=value pairs (e.g., "disk=ssd,region=us-west")
func filterPodsByNodeSelector(ctx context.Context, clients *kubernetes.K8sClients, namespace string, podMap map[string]*v1.Pod, nodeSelector string) map[string]*v1.Pod {
	if nodeSelector == "" {
		return podMap
	}

	// Parse nodeSelector into map of key=value pairs
	selectorMap := kubernetes.ParseSelector(nodeSelector)
	// Get all nodes to match labels
	node := &v1.Node{}
	var opts []client.ListOption
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	filteredPodMap := make(map[string]*v1.Pod)
	for podName, pod := range podMap {
		err := clients.CRClient.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, node)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to get node %q for pod %q: %v\n", pod.Spec.NodeName, pod.Name, err)
			continue
		}
		if kubernetes.MatchesSelector(*node, selectorMap) {
			filteredPodMap[podName] = pod
		}
	}
	return filteredPodMap
}

// streamLogsFromPods streams logs from multiple pods in parallel with synchronized timestamp-based output
func streamLogsFromPods(ctx context.Context, clientset kubernetes2.Interface, opts AggregatedLogOptions, podMap map[string]*v1.Pod) error {
	var wg sync.WaitGroup
	logsChan := make(chan LogLine, 1000)
	errsChan := make(chan error, len(podMap))

	// Create a semaphore to limit concurrent log streams
	var sem chan struct{}
	limit := opts.LimitConcurrent
	if limit > 0 {
		sem = make(chan struct{}, limit)
	}

	// Stream logs from all pods in parallel (with optional concurrency limit)
	for podName, pod := range podMap {
		wg.Add(1)
		go func(podName string, pod *v1.Pod) {
			defer wg.Done()

			// Acquire semaphore slot if limiting is enabled
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			if err := streamPodLogs(ctx, clientset, opts, logsChan, podName, pod); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to stream logs from pod %q: %v\n", podName, err)
				errsChan <- err
			}
		}(podName, pod)
	}

	// Close the logs channel when all goroutines are done
	go func() {
		wg.Wait()
		close(logsChan)
	}()

	// Process logs in real-time with synchronized output
	outputWithSynchronization(logsChan, opts)

	// Check for errors but don't fail completely
	select {
	case err := <-errsChan:
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Errors occurred while streaming logs: %v\n", err)
		}
	default:
	}

	return nil
}

// outputWithSynchronization reads logs from multiple streams and outputs them in timestamp order
// Uses a time window buffer to maintain order while allowing real-time output
func outputWithSynchronization(logsChan <-chan LogLine, opts AggregatedLogOptions) {
	// Buffer to hold pending lines from each pod
	buffer := make(map[string][]*LogLine) // podName -> list of pending lines
	var processingMutex sync.Mutex

	// Helper function to format log output with optional container prefix
	formatLogOutput := func(line *LogLine) string {
		if opts.AddContainerPrefix {
			return fmt.Sprintf("[%s] %s", line.PodName, line.RawLine)
		}
		return line.RawLine
	}

	// Helper function to output lines that are safe based on time window
	outputSafeLines := func() {
		processingMutex.Lock()
		defer processingMutex.Unlock()

		for {
			// Find the earliest timestamp among all buffered lines
			var earliestLine *LogLine
			var earliestPod string
			var latestTimestamp time.Time

			// First pass: find earliest and latest timestamps
			for pod, lines := range buffer {
				if len(lines) > 0 {
					if earliestLine == nil || lines[0].Timestamp.Before(earliestLine.Timestamp) {
						earliestLine = lines[0]
						earliestPod = pod
					}
					// Check last line in buffer for latest timestamp
					if lines[len(lines)-1].Timestamp.After(latestTimestamp) {
						latestTimestamp = lines[len(lines)-1].Timestamp
					}
				}
			}

			if earliestLine == nil {
				break // No lines to output
			}

			// Time window: output if the earliest line is sufficiently old compared to latest buffered line
			// Allow 2 second window for stragglers to arrive
			const timeWindowSeconds = 2
			timeDiff := latestTimestamp.Sub(earliestLine.Timestamp).Seconds()

			// Output if:
			// 1. This is the first/only line, OR
			// 2. Time window has passed, OR
			// 3. All buffers are small (less than 5 lines per pod), OR
			// 4. We have no other pods with data
			totalBufferedLines := 0
			podsWithData := 0
			for _, lines := range buffer {
				totalBufferedLines += len(lines)
				if len(lines) > 0 {
					podsWithData++
				}
			}

			shouldOutput := timeDiff >= timeWindowSeconds ||
				totalBufferedLines < len(buffer)*5 ||
				podsWithData <= 1

			if !shouldOutput {
				break
			}

			// Output the earliest line
			fmt.Println(formatLogOutput(earliestLine))

			// Remove from buffer
			buffer[earliestPod] = buffer[earliestPod][1:]
			if len(buffer[earliestPod]) == 0 {
				delete(buffer, earliestPod)
			}
		}
	}

	// Main loop: collect and output lines as they arrive
	for line := range logsChan {
		processingMutex.Lock()
		if buffer[line.PodName] == nil {
			buffer[line.PodName] = make([]*LogLine, 0, 1000)
		}
		buffer[line.PodName] = append(buffer[line.PodName], &line)
		processingMutex.Unlock()

		// Try to output safe lines
		outputSafeLines()
	}

	// All streams closed, output remaining lines in timestamp order
	processingMutex.Lock()
	defer processingMutex.Unlock()

	var allLines []*LogLine
	for _, lines := range buffer {
		allLines = append(allLines, lines...)
	}
	sort.Slice(allLines, func(i, j int) bool {
		if allLines[i].Timestamp.Equal(allLines[j].Timestamp) {
			return allLines[i].PodName < allLines[j].PodName
		}
		return allLines[i].Timestamp.Before(allLines[j].Timestamp)
	})
	for _, line := range allLines {
		fmt.Println(formatLogOutput(line))
	}
}

// streamPodLogs streams logs from a single pod
func streamPodLogs(ctx context.Context, clientset kubernetes2.Interface, opts AggregatedLogOptions, logsChan chan<- LogLine, podName string, pod *v1.Pod) error {
	// Get first container (usually there's only one)
	if len(pod.Spec.Containers) == 0 {
		return fmt.Errorf("pod %q has no containers", podName)
	}

	containerName := pod.Spec.Containers[0].Name
	// Build log options
	logOpts := &v1.PodLogOptions{
		Container: containerName,
		Follow:    opts.Follow,
		Previous:  opts.Previous,
		TailLines: &opts.Tail,
	}

	// Only set TailLines if the user explicitly set --tail.
	if opts.TailFlagSet && opts.Tail >= 0 {
		logOpts.TailLines = &opts.Tail
	}

	// --since: use SinceSeconds for relative duration
	if opts.Since > 0 {
		sec := int64(opts.Since.Seconds())
		if sec <= 0 {
			sec = 1
		}
		logOpts.SinceSeconds = &sec
	}

	// Get log stream
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to stream logs from pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	defer func() {
		_ = stream.Close()
	}()

	// Read logs line by line
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()
		timestamp, timeStr := parseLogLineTimestamp(line)

		logsChan <- LogLine{
			Timestamp: timestamp,
			PodName:   podName,
			RawLine:   line,
			TimeStr:   timeStr,
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading logs from pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}

	return nil
}

// parseLogLineTimestamp tries to extract timestamp from log line
// Returns the timestamp and the original time string for display
func parseLogLineTimestamp(line string) (time.Time, string) {
	// Try to parse ISO 8601 timestamp at the beginning of the line
	// Format: 2024-01-15T10:30:45.123456Z or similar
	parts := strings.Fields(line)
	if len(parts) > 0 {
		timeStr := parts[0]

		// Try parsing common timestamp formats
		formats := []string{
			"2026-02-18 13:20:16,263", // log4j style
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05.999999Z07:00",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
		}

		for _, format := range formats {
			if t, err := time.Parse(format, timeStr); err == nil {
				return t, timeStr
			}
		}
	}

	// If no timestamp found, use current time as fallback
	now := time.Now()
	return now, now.Format(time.RFC3339Nano)
}
