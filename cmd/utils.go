package cmd

import (
	"context"
	"fmt"
	"io"
	"k8s.io/api/core/v1"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// -----------------------------
func humanAge(d time.Duration) string {
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
