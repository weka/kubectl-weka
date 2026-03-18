package clustercheck

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// K8sVersionModule validates Kubernetes version is 1.24+
type K8sVersionModule struct{}

func init() {
	GlobalClusterCheckRegistry.Register(&K8sVersionModule{})
}

func (m *K8sVersionModule) Name() string {
	return "k8s_version"
}

func (m *K8sVersionModule) FriendlyName() string {
	return "Kubernetes Version"
}

func (m *K8sVersionModule) Description() string {
	return "Validates Kubernetes version is 1.24 or higher"
}

func (m *K8sVersionModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Version}}"
}

func (m *K8sVersionModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *K8sVersionModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *K8sVersionModule) SuggestedResolutionTemplate() string {
	return "Upgrade Kubernetes cluster to version 1.24 or higher. Current version: {{.Version}}"
}

func (m *K8sVersionModule) Validate(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client) (interface{}, error) {
	sv, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	ok, detail, err := CheckK8sVersion124Plus(ctx, clientset)
	if err != nil {
		return nil, err
	}

	status := "success"
	issue := ""
	version := sv.GitVersion

	if !ok {
		status = "error"
		issue = detail
	}

	return map[string]interface{}{
		"Status":  status,
		"Detail":  detail,
		"Issue":   issue,
		"Version": version,
		"OK":      ok,
	}, nil
}

func (m *K8sVersionModule) ValidateWithParams(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client, params map[string]interface{}) (interface{}, error) {
	return m.Validate(ctx, clientset, crClient)
}

// OpenShiftModule validates cluster is not ROSA or managed OpenShift
type OpenShiftModule struct{}

func init() {
	GlobalClusterCheckRegistry.Register(&OpenShiftModule{})
}

func (m *OpenShiftModule) Name() string {
	return "openshift_check"
}

func (m *OpenShiftModule) FriendlyName() string {
	return "OpenShift/ROSA Check"
}

func (m *OpenShiftModule) Description() string {
	return "Validates cluster is not ROSA or managed OpenShift"
}

func (m *OpenShiftModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: Cluster is not ROSA/managed OpenShift"
}

func (m *OpenShiftModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *OpenShiftModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *OpenShiftModule) SuggestedResolutionTemplate() string {
	return "WEKA does not support ROSA or managed OpenShift clusters. Use a standard Kubernetes cluster or self-managed OpenShift."
}

func (m *OpenShiftModule) Validate(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client) (interface{}, error) {
	ok, detail, err := CheckNotOpenShiftOrROSA(ctx, clientset)
	if err != nil {
		return nil, err
	}

	status := "success"
	issue := ""
	if !ok {
		status = "error"
		issue = detail
	}

	return map[string]interface{}{
		"Status": status,
		"Detail": detail,
		"Issue":  issue,
		"OK":     ok,
	}, nil
}

func (m *OpenShiftModule) ValidateWithParams(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client, params map[string]interface{}) (interface{}, error) {
	return m.Validate(ctx, clientset, crClient)
}

// HelmPermissionsModule validates permissions for Helm install
type HelmPermissionsModule struct{}

func init() {
	GlobalClusterCheckRegistry.Register(&HelmPermissionsModule{})
}

func (m *HelmPermissionsModule) Name() string {
	return "helm_permissions"
}

func (m *HelmPermissionsModule) FriendlyName() string {
	return "Helm Permissions"
}

func (m *HelmPermissionsModule) Description() string {
	return "Validates sufficient permissions for Helm install (cluster-scope)"
}

func (m *HelmPermissionsModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: Sufficient permissions for cluster-scoped Helm install"
}

func (m *HelmPermissionsModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *HelmPermissionsModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *HelmPermissionsModule) SuggestedResolutionTemplate() string {
	return "Ensure the current user/service account has cluster-admin permissions or the required RBAC permissions for WEKA deployment."
}

func (m *HelmPermissionsModule) Validate(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client) (interface{}, error) {
	ok, detail, err := CheckHelmClusterPermissions(ctx, clientset)
	if err != nil {
		return nil, err
	}

	status := "success"
	issue := ""
	if !ok {
		status = "error"
		issue = detail
	}

	return map[string]interface{}{
		"Status": status,
		"Detail": detail,
		"Issue":  issue,
		"OK":     ok,
	}, nil
}

func (m *HelmPermissionsModule) ValidateWithParams(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client, params map[string]interface{}) (interface{}, error) {
	return m.Validate(ctx, clientset, crClient)
}

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

func (m *CNIModule) Validate(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client) (interface{}, error) {
	return nil, fmt.Errorf("CNI module requires nodes parameter - use ValidateWithParams")
}

func (m *CNIModule) ValidateWithParams(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client, params map[string]interface{}) (interface{}, error) {
	// Extract ready nodes from params - skip NotReady nodes to avoid timeouts
	var nodes []corev1.Node
	if readyNodesParam, ok := params["readyNodes"].([]corev1.Node); ok {
		nodes = readyNodesParam
	} else if nodesParam, ok := params["nodes"].([]corev1.Node); ok {
		// Fallback to all nodes if readyNodes not provided
		nodes = nodesParam
	} else {
		return nil, fmt.Errorf("nodes parameter is required for CNI check")
	}

	// Detect CNI
	hasKnownCNI, cniName, err := DetectKnownCNIDaemonSet(ctx, clientset)
	if err != nil {
		return nil, err
	}

	// Check for bad nodes (NetworkUnavailable=true)
	var badNodes []string
	for i := range nodes {
		n := &nodes[i]
		nu := getNodeConditionStatus(n, corev1.NodeNetworkUnavailable)
		if nu == corev1.ConditionTrue {
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

// CPUManagerConfigModule validates kubelet CPU manager configuration
type CPUManagerConfigModule struct{}

func init() {
	GlobalClusterCheckRegistry.Register(&CPUManagerConfigModule{})
}

func (m *CPUManagerConfigModule) Name() string {
	return "cpu_manager_config"
}

func (m *CPUManagerConfigModule) FriendlyName() string {
	return "CPU Manager Configuration"
}

func (m *CPUManagerConfigModule) Description() string {
	return "Validates kubelet CPU manager configuration is set to 'static'"
}

func (m *CPUManagerConfigModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: CPU manager policy configured as 'static' ({{.ConfiguredCount}}/{{.TotalNodes}} nodes)"
}

func (m *CPUManagerConfigModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CPUManagerConfigModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CPUManagerConfigModule) SuggestedResolutionTemplate() string {
	return "Set cpuManagerPolicy to 'static' in kubelet configuration on affected nodes. See: https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/"
}

func (m *CPUManagerConfigModule) Validate(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client) (interface{}, error) {
	return nil, fmt.Errorf("CPU manager config module requires nodes parameter - use ValidateWithParams")
}

func (m *CPUManagerConfigModule) ValidateWithParams(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client, params map[string]interface{}) (interface{}, error) {
	// Extract ready nodes from params - skip NotReady nodes to avoid timeouts
	var nodes []corev1.Node
	if readyNodesParam, ok := params["readyNodes"].([]corev1.Node); ok {
		nodes = readyNodesParam
	} else if nodesParam, ok := params["nodes"].([]corev1.Node); ok {
		// Fallback to all nodes if readyNodes not provided
		nodes = nodesParam
	} else {
		return nil, fmt.Errorf("nodes parameter is required for CPU manager config check")
	}

	var notStaticNodes []string
	var unknownNodes []string
	var configuredNodes []string

	for i := range nodes {
		n := &nodes[i]

		pol, err := getCPUManagerPolicyViaConfigz(ctx, clientset, n.Name)
		if err != nil {
			unknownNodes = append(unknownNodes, n.Name)
			continue
		}

		if strings.ToLower(strings.TrimSpace(pol)) != "static" {
			notStaticNodes = append(notStaticNodes, n.Name)
		} else {
			configuredNodes = append(configuredNodes, n.Name)
		}
	}

	status := "success"
	issue := ""
	detail := ""
	affectedNodes := []string{}

	if len(notStaticNodes) > 0 || len(unknownNodes) > 0 {
		status = "error"
		parts := []string{}
		if len(notStaticNodes) > 0 {
			parts = append(parts, fmt.Sprintf("%d nodes not configured as 'static'", len(notStaticNodes)))
			affectedNodes = append(affectedNodes, notStaticNodes...)
		}
		if len(unknownNodes) > 0 {
			parts = append(parts, fmt.Sprintf("%d nodes could not be checked", len(unknownNodes)))
			affectedNodes = append(affectedNodes, unknownNodes...)
		}
		issue = strings.Join(parts, ", ")
		detail = issue
	} else {
		detail = fmt.Sprintf("All %d ready nodes configured as 'static'", len(configuredNodes))
	}

	return map[string]interface{}{
		"Status":          status,
		"Detail":          detail,
		"Issue":           issue,
		"ConfiguredCount": len(configuredNodes),
		"NotStaticNodes":  notStaticNodes,
		"UnknownNodes":    unknownNodes,
		"TotalNodes":      len(nodes),
		"AffectedNodes":   affectedNodes,
		"AffectedCount":   len(affectedNodes),
	}, nil
}

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

func (m *CPUManagerPolicyModule) Validate(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client) (interface{}, error) {
	return nil, fmt.Errorf("CPU manager policy module requires nodes parameter - use ValidateWithParams")
}

func (m *CPUManagerPolicyModule) ValidateWithParams(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client, params map[string]interface{}) (interface{}, error) {
	// Extract ready nodes from params - skip NotReady nodes to avoid timeouts
	var nodes []corev1.Node
	if readyNodesParam, ok := params["readyNodes"].([]corev1.Node); ok {
		nodes = readyNodesParam
	} else if nodesParam, ok := params["nodes"].([]corev1.Node); ok {
		// Fallback to all nodes if readyNodes not provided
		nodes = nodesParam
	} else {
		return nil, fmt.Errorf("nodes parameter is required for CPU manager policy check")
	}

	ok, detail := CheckCPUManagerPolicyStatic(ctx, clientset, nodes)

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

func (m *NotReadyNodesModule) Validate(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client) (interface{}, error) {
	return nil, fmt.Errorf("NotReady nodes module requires nodes parameter - use ValidateWithParams")
}

func (m *NotReadyNodesModule) ValidateWithParams(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client, params map[string]interface{}) (interface{}, error) {
	// Extract nodes from params
	var nodes []corev1.Node
	if nodesParam, ok := params["nodes"].([]corev1.Node); ok {
		nodes = nodesParam
	} else {
		return nil, fmt.Errorf("nodes parameter is required for NotReady nodes check")
	}

	// Count NotReady nodes and collect their names
	notReadyNodes := []string{}
	for _, node := range nodes {
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				isReady = condition.Status == corev1.ConditionTrue
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
