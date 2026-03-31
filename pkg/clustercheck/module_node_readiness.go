package clustercheck

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"k8s.io/api/core/v1"
)

// NotReadyNodesModule validates if there are NotReady nodes in the cluster
type NotReadyNodesModule struct{}

func init() {
	GlobalClusterCheckRegistry.Register(&NotReadyNodesModule{})
}

func (m *NotReadyNodesModule) Name() string {
	return "notready_nodes"
}

func (m *NotReadyNodesModule) FriendlyName() string {
	return "NotReady Nodes Check"
}

func (m *NotReadyNodesModule) Description() string {
	return "Checks for NotReady nodes in the cluster that may not be validated"
}

func (m *NotReadyNodesModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: All nodes are Ready"
}

func (m *NotReadyNodesModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.NotReadyCount}} node(s) are NotReady"
}

func (m *NotReadyNodesModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NotReadyNodesModule) SuggestedResolutionTemplate() string {
	return "Preflight node validation cannot check NotReady nodes. Fix the following nodes and rerun validation to ensure they meet WEKA requirements: {{.NotReadyNodesList}}"
}

func (m *NotReadyNodesModule) Validate(ctx context.Context, clients *kubernetes.K8sClients) (interface{}, error) {
	return nil, fmt.Errorf("NotReady nodes module requires nodes parameter - use ValidateWithParams")
}

func (m *NotReadyNodesModule) ValidateWithParams(ctx context.Context, clients *kubernetes.K8sClients, params map[string]interface{}) (interface{}, error) {
	// Extract nodes from params
	var nodes []v1.Node
	if nodesParam, ok := params["nodes"].([]v1.Node); ok {
		nodes = nodesParam
	} else {
		return nil, fmt.Errorf("nodes parameter is required for NotReady nodes check")
	}

	// Count NotReady nodes and collect their names
	notReadyNodes := []string{}
	for _, node := range nodes {
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				isReady = condition.Status == v1.ConditionTrue
				break
			}
		}
		if !isReady {
			notReadyNodes = append(notReadyNodes, node.Name)
		}
	}

	notReadyCount := len(notReadyNodes)
	status := "success"
	detail := fmt.Sprintf("All %d node(s) are Ready", len(nodes))
	issue := ""
	notReadyNodesList := ""

	if notReadyCount > 0 {
		status = "warning"
		detail = fmt.Sprintf("%d out of %d node(s) are NotReady", notReadyCount, len(nodes))
		issue = "Preflight node validation might not detect issues on NotReady nodes"
		// Join node names for template
		notReadyNodesList = fmt.Sprintf("%v", notReadyNodes)
		if notReadyCount <= 5 {
			// Show all nodes if 5 or fewer
			notReadyNodesList = fmt.Sprintf("%v", notReadyNodes)
		} else {
			// Show first 5 + count if more
			notReadyNodesList = fmt.Sprintf("%v and %d more", notReadyNodes[:5], notReadyCount-5)
		}
	}

	return map[string]interface{}{
		"Status":            status,
		"Detail":            detail,
		"Issue":             issue,
		"NotReadyCount":     notReadyCount,
		"NotReadyNodes":     notReadyNodes,
		"NotReadyNodesList": notReadyNodesList,
		"TotalNodes":        len(nodes),
	}, nil
}
