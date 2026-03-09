package cmd

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
	"time"
)

// ============================================================================
// File I/O Utilities
// ============================================================================

// writeToFile writes content to a file in the bundle directory
func writeToFile(bundlePath, relativePath, content string) error {
	fullPath := filepath.Join(bundlePath, relativePath)

	// Create parent directories if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

// ============================================================================
// Pod Log Collection
// ============================================================================

// collectPodLogs collects logs from a pod's containers
// If containerName is empty: collects logs from all containers in the pod
// If containerName is specified: collects logs only from that specific container
// Returns a map of container name to logs content
// The opts parameter can include Previous flag to collect logs from previous container instance
// Returns (logs, isPreviousLogsUnavailable, error)
// - isPreviousLogsUnavailable is true when opts.Previous=true but container hasn't restarted
func collectPodLogs(ctx context.Context, clients *K8sClients, namespace, podName, containerName string, opts *corev1.PodLogOptions) (map[string]string, bool, error) {
	if opts == nil {
		opts = &corev1.PodLogOptions{}
	}

	// Get the pod to find containers
	pod, err := clients.Clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	logs := make(map[string]string)
	var containersToCollect []corev1.Container
	isPreviousUnavailable := false

	// Determine which containers to collect
	if containerName != "" {
		// Collect only the specified container
		found := false
		for _, c := range pod.Spec.Containers {
			if c.Name == containerName {
				containersToCollect = append(containersToCollect, c)
				found = true
				break
			}
		}
		if !found {
			return nil, false, fmt.Errorf("container %q not found in pod %s/%s", containerName, namespace, podName)
		}
	} else {
		// Collect all containers
		containersToCollect = pod.Spec.Containers
	}

	// Collect logs from each selected container
	for _, container := range containersToCollect {
		// Check if we're requesting previous logs
		if opts.Previous {
			// Find the container status to check restart count
			hasRestarted := false
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Name == container.Name {
					hasRestarted = cs.RestartCount > 0
					break
				}
			}

			// If container hasn't restarted, previous logs don't exist - skip without error
			if !hasRestarted {
				isPreviousUnavailable = true
				continue
			}
		}

		containerOpts := *opts // Copy the options
		containerOpts.Container = container.Name

		req := clients.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &containerOpts)
		logData, err := req.DoRaw(ctx)
		if err != nil {
			// Check if this is a "previous terminated container not found" error
			errMsg := err.Error()
			if opts.Previous && (strings.Contains(errMsg, "previous terminated container") ||
				strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "unknown reason")) {
				// Previous logs don't exist (container hasn't actually terminated/restarted)
				isPreviousUnavailable = true
				continue
			}
			return nil, false, fmt.Errorf("failed to get logs from pod %s/%s container %s (previous=%v): %w", namespace, podName, container.Name, opts.Previous, err)
		}
		logs[container.Name] = string(logData)
	}

	return logs, isPreviousUnavailable, nil
}

// ============================================================================
// Pod Description Collection
// ============================================================================

// collectPodDescription collects 'kubectl describe pod' output
func collectPodDescription(ctx context.Context, clients *K8sClients, namespace, podName string) (string, error) {
	pod, err := clients.Clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	// Convert to YAML for a readable format
	yamlData, err := yaml.Marshal(pod)
	if err != nil {
		return "", fmt.Errorf("failed to marshal pod to YAML: %w", err)
	}
	return string(yamlData), nil
}

// ============================================================================
// Object Collection and Serialization
// ============================================================================

// collectObjectAsYAMLWithSensitiveData collects any Kubernetes object and optionally redacts sensitive data
func collectObjectAsYAMLWithSensitiveData(obj runtime.Object, includeSensitive bool) (string, error) {
	yamlData, err := yaml.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal object to YAML: %w", err)
	}

	yamlString := string(yamlData)

	// If sensitive data should not be included, redact common sensitive fields
	if !includeSensitive {
		yamlString = redactSensitiveData(yamlString)
	}

	return yamlString, nil
}

// redactSensitiveData removes or masks sensitive information from YAML content
func redactSensitiveData(yamlContent string) string {
	lines := strings.Split(yamlContent, "\n")
	sensitivePatterns := []string{
		"password:",
		"passwd:",
		"secret:",
		"token:",
		"api-key:",
		"apikey:",
		"api_key:",
		"key:",
		"credential:",
		"credentials:",
		"bearer:",
		"authorization:",
		"auth:",
		"username:",
		"org:",
		"organization:",
		"join-token:",
	}

	for i, line := range lines {
		// Check if current line contains sensitive fields
		for _, pattern := range sensitivePatterns {
			if containsCI(line, pattern) {
				// Redact the value part after the colon
				if idx := strings.Index(line, ":"); idx != -1 {
					indent := line[:idx]
					lines[i] = indent + ": [REDACTED]"
					break
				}
			}
		}
	}

	return strings.Join(lines, "\n")
}

// ============================================================================
// String Utilities
// ============================================================================

// containsCI checks if a string contains a substring (case-insensitive)
func containsCI(s, substr string) bool {
	s_lower := strings.ToLower(s)
	substr_lower := strings.ToLower(substr)
	return strings.Contains(s_lower, substr_lower)
}

// ============================================================================
// File and Bundle Naming
// ============================================================================

// generateSafeFileName generates a safe filename from object metadata
func generateSafeFileName(kind, namespace, name, suffix string) string {
	// Ensure suffix has a dot prefix if not empty
	if suffix != "" && !strings.HasPrefix(suffix, ".") {
		suffix = "." + suffix
	}

	if namespace == "" {
		return fmt.Sprintf("%s_%s%s", kind, name, suffix)
	}
	return fmt.Sprintf("%s_%s_%s%s", kind, namespace, name, suffix)
}

// ============================================================================
// Timestamp and Bundle Naming
// ============================================================================

// generateTimestamp generates a timestamp for bundle naming
func generateTimestamp() string {
	return time.Now().UTC().Format("20060102-150405Z")
}

// generateBundleName generates the bundle archive name
func generateBundleName(caseID string) string {
	timestamp := generateTimestamp()
	if caseID != "" {
		return fmt.Sprintf("weka-support-bundle-%s-%s", caseID, timestamp)
	}
	return fmt.Sprintf("weka-support-bundle-%s", timestamp)
}
