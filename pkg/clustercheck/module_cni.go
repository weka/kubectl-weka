package clustercheck

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/utils"
	v3 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

// CNIModule validates CNI is configured
type CNIModule struct{}

func init() {
	GlobalClusterCheckRegistry.Register(&CNIModule{})
}

func (m *CNIModule) Name() string {
	return "cni_configured"
}

func (m *CNIModule) FriendlyName() string {
	return "CNI Configuration"
}

func (m *CNIModule) Description() string {
	return "Validates CNI is properly configured on nodes"
}

func (m *CNIModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.CNIName}}"
}

func (m *CNIModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CNIModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CNIModule) SuggestedResolutionTemplate() string {
	return "Ensure a CNI plugin (Calico, Flannel, Cilium, etc.) is installed and configured on the cluster."
}

func (m *CNIModule) Validate(ctx context.Context, clients *kubernetes.K8sClients) (interface{}, error) {
	return nil, fmt.Errorf("CNI module requires nodes parameter - use ValidateWithParams")
}

func (m *CNIModule) ValidateWithParams(ctx context.Context, clients *kubernetes.K8sClients, params map[string]interface{}) (interface{}, error) {
	// Extract ready nodes from params - skip NotReady nodes to avoid timeouts
	var nodes []v1.Node
	if readyNodesParam, ok := params["readyNodes"].([]v1.Node); ok {
		nodes = readyNodesParam
	} else if nodesParam, ok := params["nodes"].([]v1.Node); ok {
		// Fallback to all nodes if readyNodes not provided
		nodes = nodesParam
	} else {
		return nil, fmt.Errorf("nodes parameter is required for CNI check")
	}

	// Detect CNI
	hasKnownCNI, cniName, err := detectKnownCNIDaemonSet(ctx, clients)
	if err != nil {
		return nil, err
	}

	// Check for bad nodes (NetworkUnavailable=true)
	var badNodes []string
	for i := range nodes {
		n := &nodes[i]
		nu := getNodeConditionStatus(n, v1.NodeNetworkUnavailable)
		if nu == v1.ConditionTrue {
			badNodes = append(badNodes, n.Name)
		}
	}

	status := "success"
	issue := ""
	detail := ""

	if !hasKnownCNI {
		status = "error"
		issue = "No known CNI daemonset found in kube-system"
		detail = issue
	} else if len(badNodes) > 0 {
		status = "error"
		issue = fmt.Sprintf("%d nodes NetworkUnavailable=true", len(badNodes))
		detail = fmt.Sprintf("Detected CNI: %s, but %d nodes have NetworkUnavailable=true", cniName, len(badNodes))
	} else {
		detail = cniName
	}

	return map[string]interface{}{
		"Status":        status,
		"Detail":        detail,
		"Issue":         issue,
		"CNIName":       cniName,
		"HasCNI":        hasKnownCNI,
		"BadNodes":      badNodes,
		"BadNodesCount": len(badNodes),
		"AffectedNodes": badNodes,
		"AffectedCount": len(badNodes),
	}, nil
}

func getNodeConditionStatus(n *v1.Node, t v1.NodeConditionType) v1.ConditionStatus {
	for _, c := range n.Status.Conditions {
		if c.Type == t {
			return c.Status
		}
	}
	return ""
}

func detectK3s(ctx context.Context, clients *kubernetes.K8sClients) (bool, string, error) {
	// K3s detection strategies:
	// 1. Check for k3s nodes via labels (node.kubernetes.io/instance-type=k3s or provider contains k3s)
	// 2. Check for k3s-server service in kube-system
	// 3. Check for k3s components (coredns with k3s labels, etc.)

	// Strategy 1: Check nodes for k3s labels or provider ID
	var nodeList v1.NodeList
	err := clients.CRClient.List(ctx, &nodeList)
	if err != nil {
		return false, "", err
	}
	nodes := &nodeList

	for _, node := range nodes.Items {
		// Check labels
		if val, ok := node.Labels["node.kubernetes.io/instance-type"]; ok {
			if strings.Contains(strings.ToLower(val), "k3s") {
				return true, fmt.Sprintf("detected via node %s label", node.Name), nil
			}
		}

		// Check providerID (e.g., "k3s://hostname")
		if strings.HasPrefix(strings.ToLower(node.Spec.ProviderID), "k3s://") {
			return true, fmt.Sprintf("detected via node %s providerID", node.Name), nil
		}

		// Check node info (some k3s setups put k3s in the container runtime version or OS image)
		if strings.Contains(strings.ToLower(node.Status.NodeInfo.ContainerRuntimeVersion), "k3s") {
			return true, fmt.Sprintf("detected via node %s runtime version", node.Name), nil
		}
		if strings.Contains(strings.ToLower(node.Status.NodeInfo.OSImage), "k3s") {
			return true, fmt.Sprintf("detected via node %s OS image", node.Name), nil
		}
	}

	// Strategy 2: Check for k3s services in kube-system
	var svcList v1.ServiceList
	err = clients.CRClient.List(ctx, &svcList, client.InNamespace("kube-system"))
	if err == nil {
		for _, svc := range svcList.Items {
			if svc.Name == "k3s" || svc.Name == "k3s-server" || strings.HasPrefix(svc.Name, "k3s-") {
				return true, fmt.Sprintf("detected via service kube-system/%s", svc.Name), nil
			}
		}
	}

	// Strategy 3: Check for k3s-specific deployments or pods
	deploys, err := clients.Clientset.AppsV1().Deployments("kube-system").List(ctx, v2.ListOptions{})
	if err == nil {
		for _, deploy := range deploys.Items {
			// K3s typically runs coredns with specific labels
			if deploy.Name == "coredns" {
				if val, ok := deploy.Labels["k3s-app"]; ok && val == "kube-dns" {
					return true, "detected via coredns k3s-app label", nil
				}
			}
			if strings.Contains(strings.ToLower(deploy.Name), "k3s") {
				return true, fmt.Sprintf("detected via deployment kube-system/%s", deploy.Name), nil
			}
		}
	}

	return false, "", nil
}

func detectKnownCNIDaemonSet(ctx context.Context, clients *kubernetes.K8sClients) (bool, string, error) {
	// Check for K3s built-in CNI (Flannel is integrated into k3s-agent, not a separate daemonset)
	isK3s, k3sHint, err := detectK3s(ctx, clients)
	if err != nil {
		return false, "", err
	}
	if isK3s {
		return true, fmt.Sprintf("k3s built-in CNI (flannel) %s", k3sHint), nil
	}

	// Check typical namespaces where CNI runs
	namespaces := []string{"kube-system", "kube-flannel"}

	// Known identifiers (substring match) across CNIs
	knownSubstrings := []string{
		"calico-node",
		"cilium",
		"weave-net",
		"flannel",
		"antrea-agent",
		"canal",
		"aws-node",
		"azure-vnet",
	}

	// 1) DaemonSet name substring check in likely namespaces
	for _, ns := range namespaces {
		var dsList v3.DaemonSetList
		err := clients.CRClient.List(ctx, &dsList, client.InNamespace(ns))
		if err != nil {
			// If namespace doesn't exist, ignore; other errors bubble up
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				continue
			}
			return false, "", err
		}

		for _, ds := range dsList.Items {
			name := ds.Name
			for _, sub := range knownSubstrings {
				if strings.Contains(name, sub) {
					return true, fmt.Sprintf("%s/%s", ns, name), nil
				}
			}

			// 2) Label-based detection (helps for flannel and others)
			labels := ds.Labels
			if utils.HasAnyLabelValue(labels, []string{"k8s-app", "app"}, []string{"flannel", "kube-flannel"}) {
				return true, fmt.Sprintf("%s/%s", ns, name), nil
			}
			if utils.HasAnyLabelValue(labels, []string{"k8s-app", "app"}, []string{"calico-node", "cilium", "weave-net", "antrea"}) {
				return true, fmt.Sprintf("%s/%s", ns, name), nil
			}
		}
	}

	// 3) Fallback: look for CNI pods (covers cases where CNI isn't a DS, or DS is elsewhere)
	// Try kube-system + kube-flannel; if flannel exists, it's usually visible here.
	for _, ns := range namespaces {
		var podList v1.PodList
		err := clients.CRClient.List(ctx, &podList, client.InNamespace(ns))
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				continue
			}
			return false, "", err
		}
		for _, p := range podList.Items {
			name := p.Name
			for _, sub := range knownSubstrings {
				if strings.Contains(name, sub) {
					return true, fmt.Sprintf("%s/%s", ns, name), nil
				}
			}
			if utils.HasAnyLabelValue(p.Labels, []string{"k8s-app", "app"}, []string{"flannel", "kube-flannel"}) {
				return true, fmt.Sprintf("%s/%s", ns, name), nil
			}
		}
	}

	return false, "", nil
}
