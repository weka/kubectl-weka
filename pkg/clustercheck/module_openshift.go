package clustercheck

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"k8s.io/api/core/v1"
	"strings"
)

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

func (m *OpenShiftModule) Validate(ctx context.Context, clients *kubernetes.K8sClients) (interface{}, error) {
	ok, detail, err := checkNotOpenShiftOrROSA(ctx, clients)
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

func (m *OpenShiftModule) ValidateWithParams(ctx context.Context, clients *kubernetes.K8sClients, params map[string]interface{}) (interface{}, error) {
	return m.Validate(ctx, clients)
}

func checkNotOpenShiftOrROSA(ctx context.Context, clients *kubernetes.K8sClients) (bool, string, error) {
	// Detect OpenShift by API groups exposed by the apiserver.
	// If any are present, it's OpenShift (including ROSA / managed OpenShift).
	grps, err := clients.Clientset.Discovery().ServerGroups()
	if err != nil {
		return false, "", err
	}

	isOpenShift := false
	var found []string

	for _, g := range grps.Groups {
		switch g.Name {
		case "route.openshift.io", "config.openshift.io", "security.openshift.io", "operator.openshift.io", "machine.openshift.io":
			isOpenShift = true
			found = append(found, g.Name)
		}
	}

	if !isOpenShift {
		return true, "", nil
	}

	// Best-effort ROSA hints:
	// - "openshift-rosa" or "openshift-addon-operator" namespaces often exist on ROSA,
	//   but not guaranteed. We'll keep the message helpful either way.
	rosaHint := detectROSAHint(ctx, clients)

	if rosaHint != "" {
		return false, fmt.Sprintf("OpenShift detected (%s); ROSA hint: %s", strings.Join(found, ","), rosaHint), nil
	}
	return false, fmt.Sprintf("OpenShift detected (%s)", strings.Join(found, ",")), nil
}

func detectROSAHint(ctx context.Context, clients *kubernetes.K8sClients) string {
	// This is heuristic: safe and optional.
	var nsList v1.NamespaceList
	err := clients.CRClient.List(ctx, &nsList)
	if err != nil {
		return ""
	}
	for _, ns := range nsList.Items {
		if ns.Name == "openshift-rosa" || strings.Contains(ns.Name, "rosa") {
			return "namespace " + ns.Name
		}
		if ns.Name == "openshift-addon-operator" {
			return "namespace openshift-addon-operator"
		}
	}
	return ""
}
