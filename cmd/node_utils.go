package cmd

import (
	"context"
	"fmt"
	"k8s.io/api/core/v1"
	"strings"
)

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

// IsNodeReady checks if a node is in Ready state
func IsNodeReady(node v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}

// FilterReadyNodes returns only nodes that are in Ready state
func FilterReadyNodes(nodes []v1.Node) []v1.Node {
	var ready []v1.Node
	for _, node := range nodes {
		if IsNodeReady(node) {
			ready = append(ready, node)
		}
	}
	return ready
}

// FilterNodesByNames returns nodes matching the given names
func FilterNodesByNames(nodes []v1.Node, names []string) []v1.Node {
	if len(names) == 0 {
		return nodes
	}

	// Build a set of requested names for O(1) lookup
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}

	var filtered []v1.Node
	for _, node := range nodes {
		if _, exists := nameSet[node.Name]; exists {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

// matchesSelector checks if a node matches all labels in the selector
func matchesSelector(node v1.Node, selector map[string]string) bool {
	if selector == nil || len(selector) == 0 {
		return true
	}
	for key, value := range selector {
		if labelValue, ok := node.Labels[key]; !ok || labelValue != value {
			return false
		}
	}
	return true
}

// printNodeList prints node names in a multi-column tabbed format
func printNodeList(indent string, nodeNames []string) {
	if len(nodeNames) == 0 {
		return
	}

	// Calculate column width based on longest node name
	maxLen := 0
	for _, name := range nodeNames {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}
	colWidth := maxLen + 2 // Add 2 for spacing

	// Determine number of columns based on terminal width (assume 120 chars)
	terminalWidth := 120
	availableWidth := terminalWidth - len(indent) - 5 // Account for indent and padding
	numCols := availableWidth / colWidth
	if numCols < 1 {
		numCols = 1
	}

	// Print nodes in columns
	for i, name := range nodeNames {
		fmt.Print(indent + name)

		// Add spacing to align columns
		if (i+1)%numCols == 0 || i == len(nodeNames)-1 {
			fmt.Println()
		} else {
			// Pad to column width
			padding := colWidth - len(name)
			fmt.Print(strings.Repeat(" ", padding))
		}
	}
}
