package cmd

import (
	"context"
	"fmt"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
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

// validateImageVersionCompatibility checks that client and cluster images are compatible
func validateImageVersionCompatibility(cluster *v1alpha1.WekaCluster, client *v1alpha1.WekaClient) error {
	clusterImage := cluster.Spec.Image
	clientImage := client.Spec.Image

	// If images are identical, no validation needed
	if clusterImage == clientImage {
		fmt.Printf("✅ Client and cluster images match: %s\n", clusterImage)
		return nil
	}

	// Parse versions from images
	clusterVersion, err := parseWekaVersion(clusterImage)
	if err != nil {
		// If we can't parse, just warn about different images
		fmt.Printf("⚠️  WARNING: Different images detected (cluster: %s, client: %s)\n", clusterImage, clientImage)
		fmt.Printf("    Unable to parse versions for compatibility check\n")
		return nil
	}

	clientVersion, err := parseWekaVersion(clientImage)
	if err != nil {
		// If we can't parse, just warn about different images
		fmt.Printf("⚠️  WARNING: Different images detected (cluster: %s, client: %s)\n", clusterImage, clientImage)
		fmt.Printf("    Unable to parse versions for compatibility check\n")
		return nil
	}

	// Compare versions
	if clusterVersion.Major != clientVersion.Major {
		return fmt.Errorf(
			"incompatible WEKA versions:\n"+
				"  Cluster image: %s (version %s)\n"+
				"  Client image:  %s (version %s)\n\n"+
				"Major version mismatch detected (%d vs %d).\n"+
				"Client and cluster must use the same major version.",
			clusterImage, clusterVersion.String(),
			clientImage, clientVersion.String(),
			clusterVersion.Major, clientVersion.Major)
	}

	if clusterVersion.Minor != clientVersion.Minor {
		return fmt.Errorf(
			"incompatible WEKA versions:\n"+
				"  Cluster image: %s (version %s)\n"+
				"  Client image:  %s (version %s)\n\n"+
				"Minor version mismatch detected (%d.%d vs %d.%d).\n"+
				"Client and cluster must use the same minor version.",
			clusterImage, clusterVersion.String(),
			clientImage, clientVersion.String(),
			clusterVersion.Major, clusterVersion.Minor,
			clientVersion.Major, clientVersion.Minor)
	}

	// Same major.minor but different patch or build
	// Client version must be equal to or older than cluster version
	if clientVersion.Patch < clusterVersion.Patch ||
		(clientVersion.Patch == clusterVersion.Patch && clientVersion.Build < clusterVersion.Build) {
		// Client is older - this may work but warn
		fmt.Printf("⚠️  WARNING: Client version is older than cluster version\n")
		fmt.Printf("    Cluster: %s (version %s)\n", clusterImage, clusterVersion.String())
		fmt.Printf("    Client:  %s (version %s)\n", clientImage, clientVersion.String())
		fmt.Printf("    This may work but is not recommended. Consider upgrading client to match cluster version.\n")
	} else if clientVersion.Patch > clusterVersion.Patch ||
		(clientVersion.Patch == clusterVersion.Patch && clientVersion.Build > clusterVersion.Build) {
		// Client is newer - not allowed
		return fmt.Errorf(
			"incompatible WEKA versions:\n"+
				"  Cluster image: %s (version %s)\n"+
				"  Client image:  %s (version %s)\n\n"+
				"Client version is newer than cluster version.\n"+
				"Client version must be equal to or older than the cluster version.\n"+
				"Please downgrade client to %s or upgrade cluster to match client version.",
			clusterImage, clusterVersion.String(),
			clientImage, clientVersion.String(),
			clusterVersion.String())
	} else {
		// Exact match
		fmt.Printf("✅ Client and cluster versions compatible: %s\n", clusterVersion.String())
	}

	return nil
}

// validateClientClusterMatch ensures the WekaClient's targetCluster matches the WekaCluster
func validateClientClusterMatch(cluster *v1alpha1.WekaCluster, client *v1alpha1.WekaClient) error {
	// Check if client has targetCluster specified
	if client.Spec.TargetCluster.Name == "" {
		// If targetCluster is not set, client might use joinIps instead
		// This is valid, so we skip the check
		return nil
	}

	// Validate namespace match
	targetNamespace := client.Spec.TargetCluster.Namespace
	if targetNamespace == "" {
		// If namespace is not specified in targetCluster, it defaults to the client's namespace
		targetNamespace = client.Namespace
	}

	if targetNamespace != cluster.Namespace {
		return fmt.Errorf(
			"client targetCluster namespace mismatch:\n"+
				"  Client '%s/%s' targets cluster namespace: %s\n"+
				"  But WekaCluster is in namespace: %s\n\n"+
				"The client's targetCluster.namespace must match the WekaCluster namespace.",
			client.Namespace, client.Name,
			targetNamespace,
			cluster.Namespace)
	}

	// Validate name match
	if client.Spec.TargetCluster.Name != cluster.Name {
		return fmt.Errorf(
			"client targetCluster name mismatch:\n"+
				"  Client '%s/%s' targets cluster: %s\n"+
				"  But WekaCluster name is: %s\n\n"+
				"The client's targetCluster.name must match the WekaCluster name.",
			client.Namespace, client.Name,
			client.Spec.TargetCluster.Name,
			cluster.Name)
	}

	return nil
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
