package clustercheck

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"k8s.io/api/core/v1"
	"strings"
)

// CPUManagerPolicyModule validates actual CPU manager policy on nodes
type CPUManagerPolicyModule struct{}

func init() {
	GlobalClusterCheckRegistry.Register(&CPUManagerPolicyModule{})
}

func (m *CPUManagerPolicyModule) Name() string {
	return "cpu_manager_policy"
}

func (m *CPUManagerPolicyModule) FriendlyName() string {
	return "CPU Manager Policy (Runtime)"
}

func (m *CPUManagerPolicyModule) Description() string {
	return "Validates actual CPU manager policy running on nodes is 'static'"
}

func (m *CPUManagerPolicyModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: All ready nodes running 'static' policy"
}

func (m *CPUManagerPolicyModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CPUManagerPolicyModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CPUManagerPolicyModule) SuggestedResolutionTemplate() string {
	return "Restart kubelet on affected nodes after setting cpuManagerPolicy to 'static'. May require node drain and restart."
}

func (m *CPUManagerPolicyModule) Validate(ctx context.Context, clients *kubernetes.K8sClients) (interface{}, error) {
	return nil, fmt.Errorf("CPU manager policy module requires nodes parameter - use ValidateWithParams")
}

func (m *CPUManagerPolicyModule) ValidateWithParams(ctx context.Context, clients *kubernetes.K8sClients, params map[string]interface{}) (interface{}, error) {
	// Extract ready nodes from params - skip NotReady nodes to avoid timeouts
	var nodes []v1.Node
	if readyNodesParam, ok := params["readyNodes"].([]v1.Node); ok {
		nodes = readyNodesParam
	} else if nodesParam, ok := params["nodes"].([]v1.Node); ok {
		// Fallback to all nodes if readyNodes not provided
		nodes = nodesParam
	} else {
		return nil, fmt.Errorf("nodes parameter is required for CPU manager policy check")
	}

	ok, detail := checkCPUManagerPolicyStatic(ctx, clients.Clientset, nodes)

	status := "success"
	issue := ""

	// Extract affected nodes from detail string
	var affectedNodes []string
	if !ok {
		status = "error"
		issue = detail

		// Parse the detail string to extract node names
		// Detail format: "X nodes not static: node1=none, node2=none, ..."
		if strings.Contains(detail, "not static:") {
			parts := strings.Split(detail, "not static:")
			if len(parts) > 1 {
				nodesList := strings.Split(parts[1], ",")
				for _, nodeInfo := range nodesList {
					nodeInfo = strings.TrimSpace(nodeInfo)
					if idx := strings.Index(nodeInfo, "="); idx > 0 {
						nodeName := nodeInfo[:idx]
						affectedNodes = append(affectedNodes, nodeName)
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"Status":        status,
		"Detail":        detail,
		"Issue":         issue,
		"OK":            ok,
		"AffectedNodes": affectedNodes,
		"AffectedCount": len(affectedNodes),
	}, nil
}
