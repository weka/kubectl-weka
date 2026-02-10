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

// ============================================================================
// Resource Utilities
// ============================================================================

// FormatAsGi formats a Quantity as Gi with 1 decimal place
func FormatAsGi(q *resource.Quantity) string {
	// Get value in bytes
	bytes := q.Value()
	// Convert to Gi (1 Gi = 1024^3 bytes)
	gi := float64(bytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.1fGi", gi)
}

// QuantityOrZero returns the quantity value or zero if not found
func QuantityOrZero(resourceList v1.ResourceList, resourceName v1.ResourceName) resource.Quantity {
	val, ok := resourceList[resourceName]
	if !ok {
		return resource.Quantity{}
	}
	return val
}

// ============================================================================
// Pod & Resource Calculation
// ============================================================================

// CalculateNodeUsage calculates current resource usage on a node from pods
func CalculateNodeUsage(nodeName string, podsByNode map[string][]v1.Pod) (cpuMillicores int64, memoryBytes int64, hugepagesBytes int64) {
	podsOnNode := podsByNode[nodeName]
	for _, pod := range podsOnNode {
		// Skip non-running pods
		if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			continue
		}

		// Sum container requests
		for _, container := range pod.Spec.Containers {
			cpuReq := QuantityOrZero(container.Resources.Requests, v1.ResourceCPU)
			memReq := QuantityOrZero(container.Resources.Requests, v1.ResourceMemory)
			hpReq := QuantityOrZero(container.Resources.Requests, "hugepages-2Mi")

			cpuMillicores += cpuReq.MilliValue()
			memoryBytes += memReq.Value()
			hugepagesBytes += hpReq.Value()
		}
	}
	return
}

// ============================================================================
// File Parsing
// ============================================================================

// ParseWekaResourceFile parses a Weka YAML file and returns the specified resource type
// This is a generic function that works with any Weka resource type.
// Example usage:
//
//	cluster, err := ParseWekaResourceFile[*wekaapi.WekaCluster](filePath)
//	policy, err := ParseWekaResourceFile[*wekaapi.WekaPolicy](filePath)
//	client, err := ParseWekaResourceFile[*wekaapi.WekaClient](filePath)
//	container, err := ParseWekaResourceFile[*wekaapi.WekaContainer](filePath)
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

// ============================================================================
// Helper Functions
// ============================================================================

// RepeatChar repeats a character n times
func RepeatChar(ch rune, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += string(ch)
	}
	return result
}

// SubtractQuantity returns total - used
// This is a utility function for resource calculations, used by plan_clients
// nolint:unused
func SubtractQuantity(total, used resource.Quantity) resource.Quantity {
	result := total.DeepCopy()
	result.Sub(used)
	if result.Sign() < 0 {
		return resource.MustParse("0")
	}
	return result
}

// GetPercentage calculates what percentage 'used' is of 'total'
// This is a utility function for resource calculations, used by plan_clients
// nolint:unused
func GetPercentage(used, total resource.Quantity) int {
	if total.IsZero() {
		return 0
	}
	pct := int((used.Value() * 100) / total.Value())
	if pct > 100 {
		pct = 100
	}
	return pct
}

// ============================================================================
// Sorting Helpers
// ============================================================================

// SortNodeNames returns a sorted list of node names
func SortNodeNames(nodes []v1.Node) []string {
	var names []string
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	sort.Strings(names)
	return names
}

// SortStringKeys returns sorted keys from a string map
func SortStringKeys(m map[string]interface{}) []string {
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ============================================================================
// Label & Selector Utilities
// ============================================================================

// MergeSelectorMaps merges selectors, with role-specific taking precedence
func MergeSelectorMaps(global map[string]string, roleSpecific map[string]string) map[string]string {
	// If role-specific selector exists, use it exclusively
	if len(roleSpecific) > 0 {
		merged := make(map[string]string)
		for k, v := range roleSpecific {
			merged[k] = v
		}
		return merged
	}

	// Otherwise fall back to global
	merged := make(map[string]string)
	for k, v := range global {
		merged[k] = v
	}
	return merged
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
