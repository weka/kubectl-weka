package cmd

import (
	"context"
	"fmt"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"sort"
	"strings"
)

// Common types used by both cluster and client planning
// Note: NodeRequirements is defined in plan_cluster.go and should be moved here in the future

// ============================================================================
// Node Operations
// ============================================================================

// GetClusterNodes retrieves all nodes from the Kubernetes cluster using the provided client
func GetClusterNodes(ctx context.Context) ([]v1.Node, error) {
	client := KubeClients.CRClient
	var nodeList v1.NodeList
	err := client.List(ctx, &nodeList)
	if err != nil {
		return nil, err
	}

	return nodeList.Items, nil
}

// ============================================================================
// Node Selector & Grouping
// ============================================================================

// FilterNodesBySelector returns nodes matching the given label selector
func FilterNodesBySelector(nodes []v1.Node, selector map[string]string) []v1.Node {
	if selector == nil || len(selector) == 0 {
		return nodes
	}

	var eligible []v1.Node
	for _, node := range nodes {
		if MatchesSelector(node, selector) {
			eligible = append(eligible, node)
		}
	}
	return eligible
}

// MatchesSelector checks if a node matches the given label selector
func MatchesSelector(node v1.Node, selector map[string]string) bool {
	for key, value := range selector {
		if labelValue, ok := node.Labels[key]; !ok || labelValue != value {
			return false
		}
	}
	return true
}

// QuantityOrZero returns the quantity value or zero if not found
func QuantityOrZero(resourceList v1.ResourceList, resourceName v1.ResourceName) resource.Quantity {
	val, ok := resourceList[resourceName]
	if !ok {
		return resource.Quantity{}
	}
	return val
}

func ParseWekaResourceFile[T runtime.Object](filePath string) (T, error) {
	var result T

	data, err := GetFileContents(filePath)
	if err != nil {
		return result, err
	}

	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		return result, err
	}
	if err := wekaapi.AddToScheme(s); err != nil {
		return result, err
	}

	decode := serializer.NewCodecFactory(s).UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return result, err
	}

	// Type assert to the expected type
	result, ok := obj.(T)
	if !ok {
		return result, fmt.Errorf("file does not contain expected resource type")
	}

	return result, nil
}

// GetFileContents reads file contents
func GetFileContents(filePath string) ([]byte, error) {
	return GetFileContentsFromPath(filePath)
}

// GetFileContentsFromPath is a wrapper for reading file contents
func GetFileContentsFromPath(filePath string) ([]byte, error) {
	// This will be implemented using the actual file reading
	// For now it's a stub that uses the OS package
	content, err := GetFileBytes(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", filePath, err)
	}
	return content, nil
}

// GetFileBytes reads bytes from a file
func GetFileBytes(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

// SelectorToString converts selector map to a readable string
func SelectorToString(selector map[string]string) string {
	if len(selector) == 0 {
		return "none"
	}
	var pairs []string
	for k, v := range selector {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

// calculatePodResourceUsage sums up resource requests for all pods on a node
func calculatePodResourceUsage(pods []v1.Pod, resourceName v1.ResourceName) resource.Quantity {
	total := resource.NewQuantity(0, resource.BinarySI)

	for _, pod := range pods {
		// Check regular containers
		for _, container := range pod.Spec.Containers {
			if container.Resources.Requests != nil {
				if val, ok := container.Resources.Requests[resourceName]; ok {
					total.Add(val)
				}
			}
		}

		// Check init containers
		for _, container := range pod.Spec.InitContainers {
			if container.Resources.Requests != nil {
				if val, ok := container.Resources.Requests[resourceName]; ok {
					total.Add(val)
				}
			}
		}
	}

	return *total
}

// createResourceBar creates a visual bar showing USED + WEKA + FREE with colors for each container type
func createResourceBar(usedPercent, wekaPercent float64, containerTypes []string) string {

	barWidth := 50

	// Calculate widths
	usedWidth := int(float64(barWidth) * usedPercent / 100.0)
	wekaWidth := int(float64(barWidth) * wekaPercent / 100.0)

	if usedWidth < 0 {
		usedWidth = 0
	}
	if wekaWidth < 0 {
		wekaWidth = 0
	}

	// Ensure minimum width of 1 for visibility if there's any usage
	if usedPercent > 0 && usedWidth == 0 {
		usedWidth = 1
	}
	if wekaPercent > 0 && wekaWidth == 0 {
		wekaWidth = 1
	}

	if usedWidth+wekaWidth > barWidth {
		// Scale down if total exceeds bar width
		total := usedWidth + wekaWidth
		usedWidth = (usedWidth * barWidth) / total
		wekaWidth = (wekaWidth * barWidth) / total
	}

	freeWidth := barWidth - usedWidth - wekaWidth
	if freeWidth < 0 {
		freeWidth = 0
	}

	// Build the bar
	used := ""
	if usedWidth > 0 {
		used = colorUsed + strings.Repeat("█", usedWidth) + colorReset
	}

	// For Weka portion, use different colors for different container types
	weka := ""
	if len(containerTypes) > 0 {
		// Calculate width per container type
		widthPerType := wekaWidth / len(containerTypes)
		remainder := wekaWidth % len(containerTypes)

		for i, cType := range containerTypes {
			width := widthPerType
			if i == 0 {
				width += remainder // Add remainder to first container
			}

			if width > 0 {
				var color string
				switch cType {
				case "compute":
					color = colorCompute
				case "drive":
					color = colorDrive
				case "s3":
					color = colorS3
				case "nfs":
					color = colorNFS
				case "envoy":
					color = colorEnvoy
				case "client":
					color = colorClient
				default:
					color = colorDefault
				}

				weka += color + strings.Repeat("█", width) + colorReset
			}
		}
	} else {
		// Default color if no container types
		if wekaWidth > 0 {
			weka = "\033[35m" + strings.Repeat("█", wekaWidth) + colorReset
		}
	}

	free := ""
	if freeWidth > 0 {
		free = colorFree + strings.Repeat("░", freeWidth) + colorReset
	}

	return fmt.Sprintf("[%s%s%s]", used, weka, free)
}

// sortNodeNamesNumerically sorts node names using natural/numerical ordering
// e.g., h1, h2, h10, h11 instead of h1, h10, h11, h2
func sortNodeNamesNumerically(names []string) {
	sort.Slice(names, func(i, j int) bool {
		return compareNodeNames(names[i], names[j]) < 0
	})
}

// compareNodeNames compares two node names numerically
// Returns -1 if a < b, 0 if a == b, 1 if a > b
func compareNodeNames(a, b string) int {
	// Split each name into alternating text and number parts
	aParts := splitNodeName(a)
	bParts := splitNodeName(b)

	// Compare each part
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aPart := aParts[i]
		bPart := bParts[i]

		// Try to parse as numbers
		aNum, aIsNum := tryParseInt(aPart)
		bNum, bIsNum := tryParseInt(bPart)

		if aIsNum && bIsNum {
			// Both are numbers, compare numerically
			if aNum < bNum {
				return -1
			} else if aNum > bNum {
				return 1
			}
		} else if aIsNum != bIsNum {
			// One is number, one is text - numbers come after text
			if aIsNum {
				return 1
			}
			return -1
		} else {
			// Both are text, compare alphabetically
			if aPart < bPart {
				return -1
			} else if aPart > bPart {
				return 1
			}
		}
	}

	// One is prefix of the other
	if len(aParts) < len(bParts) {
		return -1
	} else if len(aParts) > len(bParts) {
		return 1
	}
	return 0
}

// splitNodeName splits a node name into alternating text and number parts
// e.g., "h5-15-a" -> ["h", "5", "-", "15", "-", "a"]
func splitNodeName(name string) []string {
	var parts []string
	var current strings.Builder
	isDigit := false

	for _, r := range name {
		if (r >= '0' && r <= '9') != isDigit {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			isDigit = !isDigit
		}
		current.WriteRune(r)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
