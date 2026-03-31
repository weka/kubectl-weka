package clustercheck

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"k8s.io/apimachinery/pkg/util/version"
	"strings"
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

func (m *K8sVersionModule) Validate(ctx context.Context, clients *kubernetes.K8sClients) (interface{}, error) {
	sv, err := clients.Clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	ok, detail, err := checkK8sVersion124Plus(ctx, clients)
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

func (m *K8sVersionModule) ValidateWithParams(ctx context.Context, clients *kubernetes.K8sClients, params map[string]interface{}) (interface{}, error) {
	return m.Validate(ctx, clients)
}

func checkK8sVersion124Plus(ctx context.Context, clients *kubernetes.K8sClients) (bool, string, error) {
	sv, err := clients.Clientset.Discovery().ServerVersion()
	if err != nil {
		return false, "", err
	}

	// sv.GitVersion is like "v1.29.1"
	got, err := version.ParseGeneric(sv.GitVersion)
	if err != nil {
		// fallback: try without leading v
		got, err = version.ParseGeneric(strings.TrimPrefix(sv.GitVersion, "v"))
		if err != nil {
			return false, "", fmt.Errorf("cannot parse server version %q", sv.GitVersion)
		}
	}

	min := version.MustParseGeneric("1.24.0")
	if got.LessThan(min) {
		return false, fmt.Sprintf("k8s is %s, should be >= 1.24.0", sv.GitVersion), nil
	}
	return true, "", nil
}
