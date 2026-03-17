package cmd

import (
	"context"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/jedib0t/go-pretty/v6/table"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// styleTableMinimal configures a table with minimal styling (kubectl-like output)
func styleTableMinimal(w table.Writer) {
	w.SetStyle(table.StyleDefault)
	w.Style().Options.DrawBorder = false
	w.Style().Options.SeparateRows = false
	w.Style().Options.SeparateColumns = false
	w.Style().Options.SeparateFooter = false
	w.Style().Options.SeparateHeader = false
}

// indentText indents a block of text by the specified number of spaces
func indentText(text string, spaces int, subsequentSpace ...int) string {
	if spaces <= 0 || text == "" {
		return text
	}

	indent := strings.Repeat(" ", spaces)
	subIndent := indent
	lines := strings.Split(text, "\n")

	if subsequentSpace != nil {
		subIndent = strings.Repeat(" ", subsequentSpace[0]) + subIndent
	}
	var result []string
	for i, line := range lines {
		if line == "" {
			result = append(result, "")
		} else if i > 0 {
			result = append(result, subIndent+line)
		} else {
			result = append(result, indent+line)
		}
	}

	return strings.Join(result, "\n")
}

// -----------------------------
func humanAge(t interface{}) string {
	var d time.Duration
	if t == nil {
		return "-"
	}
	switch v := t.(type) {
	case time.Time:
		d = time.Since(v)
	case metav1.Time:
		d = time.Since(v.Time)
	case time.Duration:
		d = v
	case string:
		return v
	default:
		return "-"
	}
	if d < 0 {
		d = -d
	}
	// kubectl-ish compact
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d / (24 * time.Hour))
	if days < 365 {
		return fmt.Sprintf("%dd", days)
	}
	years := days / 365
	return fmt.Sprintf("%dy", years)
}

// WekaVersion represents a parsed WEKA version
type WekaVersion struct {
	Major int
	Minor int
	Patch int
	Build int
	Raw   string
}

func (v WekaVersion) String() string {
	if v.Build > 0 {
		return fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Build)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// parseWekaVersion extracts version from WEKA container image
// Supports formats like:
//   - quay.io/weka.io/weka-in-container:4.4.10.200
//   - weka/weka:4.2.5
//   - registry.example.com/weka:4.3.0.100
//   - quay.io/weka.io/weka:5.1.0.461-qa-alpha
func parseWekaVersion(image string) (*WekaVersion, error) {
	// Extract version from image tag (everything after the last ':')
	// Format: <registry>/<image>:<version>
	colonIndex := strings.LastIndex(image, ":")
	if colonIndex == -1 {
		return nil, fmt.Errorf("image does not contain version tag: %s", image)
	}

	versionStr := image[colonIndex+1:]

	// Remove any suffix after a dash (e.g., "-qa-alpha", "-rc1", "-dev")
	// This allows us to parse "5.1.0.461-qa-alpha" as "5.1.0.461"
	if dashIndex := strings.Index(versionStr, "-"); dashIndex != -1 {
		versionStr = versionStr[:dashIndex]
	}

	// Parse version components (e.g., "4.4.10.200" or "4.2.5")
	versionParts := strings.Split(versionStr, ".")
	if len(versionParts) < 3 {
		return nil, fmt.Errorf("invalid version format: %s (expected at least major.minor.patch)", versionStr)
	}

	version := &WekaVersion{Raw: versionStr}

	// Parse major version
	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version '%s': %w", versionParts[0], err)
	}
	version.Major = major

	// Parse minor version
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version '%s': %w", versionParts[1], err)
	}
	version.Minor = minor

	// Parse patch version
	patch, err := strconv.Atoi(versionParts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version '%s': %w", versionParts[2], err)
	}
	version.Patch = patch

	// Parse build version (optional)
	if len(versionParts) >= 4 {
		build, err := strconv.Atoi(versionParts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid build version '%s': %w", versionParts[3], err)
		}
		version.Build = build
	}

	return version, nil
}

// formatSelector converts a label selector map to a string representation
func formatSelector(selector map[string]string) string {
	if len(selector) == 0 {
		return "(none)"
	}
	var parts []string
	for key, value := range selector {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// randomString generates a random string of specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// writeFile writes content to a file
func writeFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

// readFile reads content from a file
func readFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// getPodLogs retrieves logs from a pod container
func getPodLogs(ctx context.Context, namespace, podName, containerName string) (string, error) {
	clientset := KubeClients.Clientset

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Container: containerName,
	})

	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func boolPtr(b bool) *bool { return &b }

func joinLimited(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(", ...(+%d)", len(items)-max)
}

func shortErr(err error) string {
	// Keep it readable inside brackets
	s := err.Error()
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	//if len(s) > 60 {
	//	return s[:57] + "..."
	//}
	return s
}

// Simple ANSI colors. If you want, we can auto-disable color when not a TTY / NO_COLOR.
func green(s string) string  { return "\033[32m" + s + "\033[0m" }
func red(s string) string    { return "\033[31m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func cyan(s string) string   { return "\033[36m" + s + "\033[0m" }

func hasAnyLabelValue(labels map[string]string, keys []string, values []string) bool {
	for _, k := range keys {
		if v, ok := labels[k]; ok {
			for _, want := range values {
				if v == want {
					return true
				}
			}
		}
	}
	return false
}

// colorizeContainerType returns a colored version of the container type name
func colorizeContainerType(containerType string) string {

	switch containerType {
	case "compute":
		return colorCompute + "Compute" + colorReset
	case "drive":
		return colorDrive + "Drive" + colorReset
	case "s3":
		return colorS3 + "S3" + colorReset
	case "nfs":
		return colorNFS + "NFS" + colorReset
	case "envoy":
		return colorEnvoy + "Envoy" + colorReset
	case "client":
		return colorClient + "Client" + colorReset // Reuse cyan color for client
	default:
		return containerType
	}
}

// tryParseInt tries to parse a string as an integer
// Returns the integer value and whether parsing was successful
func tryParseInt(s string) (int, bool) {
	num, err := strconv.Atoi(s)
	return num, err == nil
}

// parseSelector converts a label selector string to a map for crclient.MatchingLabels
func parseSelector(selector string) map[string]string {
	result := make(map[string]string)
	if selector == "" {
		return result
	}

	pairs := strings.Split(selector, ",")
	for _, pair := range pairs {
		kv := strings.Split(strings.TrimSpace(pair), "=")
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}

func mapKeysToList(m map[string]struct{}) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func firstOrNone(xs []string) string {
	if len(xs) == 0 {
		return "<none>"
	}
	if strings.TrimSpace(xs[0]) == "" {
		return "<none>"
	}
	return xs[0]
}

// getNameOrNone returns the name or "<none>" if empty
func getNameOrNone(name string) string {
	if name == "" {
		return "<none>"
	}
	return name
}

func selectorMapToSelector(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	ls := labels.Set(m)
	return labels.SelectorFromSet(ls).String()
}

// PodHealthCheckOptions contains optional parameters for pod health validation
type PodHealthCheckOptions struct {
	RestartThreshold     int32         // Restart count threshold (default: 2)
	LastRestartThreshold time.Duration // Time threshold for last restart (default: 5 minutes)
}

// DefaultPodHealthCheckOptions returns the default pod health check options
func DefaultPodHealthCheckOptions() PodHealthCheckOptions {
	return PodHealthCheckOptions{
		RestartThreshold:     2,
		LastRestartThreshold: 5 * time.Minute,
	}
}

// IsPodUnhealthy checks if a pod is unhealthy based on restart count and last restart time.
// A pod is considered unhealthy if:
// - It has more restarts than the threshold, AND
// - The last restart was within the threshold time duration
//
// The function examines ALL containers (regular and init) to:
// - Sum up total restart count across all containers
// - Find the most recent termination time across all containers
//
// Parameters:
//
//	pod: The pod to check
//	opts: Optional parameters (uses defaults if nil)
//
// Returns true if the pod meets the unhealthy criteria
func IsPodUnhealthy(pod *v1.Pod, opts *PodHealthCheckOptions) bool {
	// Use defaults if options not provided
	if opts == nil {
		opts = &PodHealthCheckOptions{
			RestartThreshold:     2,
			LastRestartThreshold: 5 * time.Minute,
		}
	}

	// Calculate total restart count across all containers
	var totalRestarts int32

	// Sum restarts from regular containers
	if pod.Status.ContainerStatuses != nil {
		for i := range pod.Status.ContainerStatuses {
			totalRestarts += pod.Status.ContainerStatuses[i].RestartCount
		}
	}

	// Sum restarts from init containers
	if pod.Status.InitContainerStatuses != nil {
		for i := range pod.Status.InitContainerStatuses {
			totalRestarts += pod.Status.InitContainerStatuses[i].RestartCount
		}
	}

	// If restart count doesn't exceed threshold, pod is healthy
	if totalRestarts <= opts.RestartThreshold {
		return false
	}

	// Find the most recent termination time across all containers
	var lastRestartTime *metav1.Time

	// Check regular containers for most recent termination
	if pod.Status.ContainerStatuses != nil {
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			if cs.LastTerminationState.Terminated != nil {
				finishedTime := &cs.LastTerminationState.Terminated.FinishedAt
				// Track the most recent termination time
				if lastRestartTime == nil || finishedTime.After(lastRestartTime.Time) {
					lastRestartTime = finishedTime
				}
			}
		}
	}

	// Check init containers for most recent termination
	if pod.Status.InitContainerStatuses != nil {
		for i := range pod.Status.InitContainerStatuses {
			cs := &pod.Status.InitContainerStatuses[i]
			if cs.LastTerminationState.Terminated != nil {
				finishedTime := &cs.LastTerminationState.Terminated.FinishedAt
				// Track the most recent termination time
				if lastRestartTime == nil || finishedTime.After(lastRestartTime.Time) {
					lastRestartTime = finishedTime
				}
			}
		}
	}

	// Pod is unhealthy if:
	// 1. Restart count exceeds threshold (already checked above), AND
	// 2. Last restart time is set (has actually restarted), AND
	// 3. Last restart was within the threshold duration
	if lastRestartTime != nil {
		thresholdTime := metav1.Now().Add(-opts.LastRestartThreshold)
		if lastRestartTime.After(thresholdTime) {
			return true
		}
	}

	return false
}

// IsPodUnhealthyWithValues checks if a pod is unhealthy based on pre-computed restart count and last restart time.
// This is a wrapper around IsPodUnhealthy for cases where restart metrics have already been aggregated.
//
// It constructs a minimal pod object from the provided values and delegates to IsPodUnhealthy.
//
// Parameters:
//
//	restartCount: Total number of restarts across all containers
//	lastRestartTime: Timestamp of the most recent container termination (nil if never restarted)
//	opts: Optional parameters (uses defaults if nil)
//
// Returns true if the pod meets the unhealthy criteria
func IsPodUnhealthyWithValues(restartCount int32, lastRestartTime *metav1.Time, opts *PodHealthCheckOptions) bool {
	// Construct a minimal pod object with the provided values
	pod := &v1.Pod{
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					RestartCount: restartCount,
				},
			},
		},
	}

	// Set last termination state only if lastRestartTime is not nil
	if lastRestartTime != nil {
		pod.Status.ContainerStatuses[0].LastTerminationState = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				FinishedAt: *lastRestartTime,
			},
		}
	}

	// Delegate to the main function
	return IsPodUnhealthy(pod, opts)
}

// PodRestartMetrics holds restart-related metrics for a pod
type PodRestartMetrics struct {
	RestartCount    int32        // Total number of restarts across all containers
	LastRestartTime *metav1.Time // Timestamp of the most recent container termination
}

// GetPodRestartMetrics calculates restart metrics for a pod by examining all containers.
// It sums restart counts from all regular and init containers, and finds the most recent
// termination time across all containers.
//
// Parameters:
//
//	pod: The pod to analyze
//
// Returns PodRestartMetrics with aggregated restart information
func GetPodRestartMetrics(pod *v1.Pod) PodRestartMetrics {
	metrics := PodRestartMetrics{}

	// Sum restarts from regular containers
	if pod.Status.ContainerStatuses != nil {
		for i := range pod.Status.ContainerStatuses {
			metrics.RestartCount += pod.Status.ContainerStatuses[i].RestartCount
		}
	}

	// Sum restarts from init containers
	if pod.Status.InitContainerStatuses != nil {
		for i := range pod.Status.InitContainerStatuses {
			metrics.RestartCount += pod.Status.InitContainerStatuses[i].RestartCount
		}
	}

	// Find the most recent termination time across all containers
	// Check regular containers for most recent termination
	if pod.Status.ContainerStatuses != nil {
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			if cs.LastTerminationState.Terminated != nil {
				finishedTime := &cs.LastTerminationState.Terminated.FinishedAt
				// Track the most recent termination time
				if metrics.LastRestartTime == nil || finishedTime.After(metrics.LastRestartTime.Time) {
					metrics.LastRestartTime = finishedTime
				}
			}
		}
	}

	// Check init containers for most recent termination
	if pod.Status.InitContainerStatuses != nil {
		for i := range pod.Status.InitContainerStatuses {
			cs := &pod.Status.InitContainerStatuses[i]
			if cs.LastTerminationState.Terminated != nil {
				finishedTime := &cs.LastTerminationState.Terminated.FinishedAt
				// Track the most recent termination time
				if metrics.LastRestartTime == nil || finishedTime.After(metrics.LastRestartTime.Time) {
					metrics.LastRestartTime = finishedTime
				}
			}
		}
	}

	return metrics
}

// extractFriendlyName is a helper function just for sake of representation
// gives a better name to module so it can be later embedded in node status checks
func extractFriendlyName(moduleName ModuleName) string {
	friendlyName := string(moduleName)
	switch moduleName {
	case "os":
		friendlyName = "Operating System"
	case "kernel":
		friendlyName = "Kernel"
	case "cpu_memory":
		friendlyName = "CPU & Memory"
	case "weka_dir":
		friendlyName = "Weka Directory"
	case "xfs":
		friendlyName = "XFS Tools"
	case "weka_client":
		friendlyName = "Weka Client"
	case "network_interfaces":
		friendlyName = "Network Interfaces"
	case "nvme_drives":
		friendlyName = "NVMe Drives"
	}
	return friendlyName
}

// truncateString truncates a string to maxLength characters and adds ellipsis if needed
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

// GetNamespaceFromFlags centralizes logic for namespace selection based on flags.
// Returns: namespace string, allNamespaces bool, error
func GetNamespaceFromFlags(allNamespaces bool, namespace string) (string, bool, error) {
	if allNamespaces {
		return "", true, nil
	}
	if namespace != "" {
		return namespace, false, nil
	}
	ns, err := GetKubeNamespace()
	if err != nil {
		return "", false, err
	}
	return ns, false, nil
}

func boolToOkError(v bool) string {
	if v {
		return "OK"
	}
	return "ERROR"
}

// formatQuantityToGB converts a resource quantity to human-readable format in the largest appropriate unit
// e.g., 2000Mi -> "2GB", 2500Mi -> "2.4GB", 512Mi -> "512MB", 512Ki -> "512KB"
func formatQuantityToGB(val interface{}) string {
	qty, ok := val.(resource.Quantity)
	if !ok {
		// Try pointer
		if ptr, ok := val.(*resource.Quantity); ok && ptr != nil {
			qty = *ptr
		} else {
			return "-"
		}
	}

	// Get the value in bytes (canonical form)
	bytes := qty.Value()
	if bytes < 0 {
		bytes = -bytes
	}

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	// Format with appropriate precision, using the largest unit that keeps value >= 1
	switch {
	case bytes >= TB:
		value := float64(bytes) / float64(TB)
		if value >= 10 {
			return fmt.Sprintf("%.0fTB", value)
		}
		return fmt.Sprintf("%.1fTB", value)
	case bytes >= GB:
		value := float64(bytes) / float64(GB)
		if value >= 10 {
			return fmt.Sprintf("%.0fGB", value)
		}
		return fmt.Sprintf("%.1fGB", value)
	case bytes >= MB:
		value := float64(bytes) / float64(MB)
		if value >= 10 {
			return fmt.Sprintf("%.0fMB", value)
		}
		return fmt.Sprintf("%.1fMB", value)
	case bytes >= KB:
		value := float64(bytes) / float64(KB)
		if value >= 10 {
			return fmt.Sprintf("%.0fKB", value)
		}
		return fmt.Sprintf("%.1fKB", value)
	default:
		return fmt.Sprintf("%d", bytes)
	}
}
