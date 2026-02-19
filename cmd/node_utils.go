package cmd

import (
	"context"
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
