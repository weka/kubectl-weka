package clustercheck

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/utils"
	"k8s.io/api/authorization/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes2 "k8s.io/client-go/kubernetes"
)

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

func (m *HelmPermissionsModule) Validate(ctx context.Context, clients *kubernetes.K8sClients) (interface{}, error) {
	ok, detail, err := checkHelmClusterPermissions(ctx, clients.Clientset)
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

func (m *HelmPermissionsModule) ValidateWithParams(ctx context.Context, clients *kubernetes.K8sClients, params map[string]interface{}) (interface{}, error) {
	return m.Validate(ctx, clients)
}

func checkHelmClusterPermissions(ctx context.Context, clientset *kubernetes2.Clientset) (bool, string, error) {
	type check struct {
		verb, group, resource string
	}

	// Minimal set that implies you are NOT confined to a single namespace,
	// and can install typical operator charts.
	checks := []check{
		{"create", "apiextensions.k8s.io", "customresourcedefinitions"},
		{"create", "rbac.authorization.k8s.io", "clusterroles"},
		{"create", "rbac.authorization.k8s.io", "clusterrolebindings"},
		{"create", "", "namespaces"},
		// Nice-to-have for many operators; not strictly "helm" but common:
		{"create", "admissionregistration.k8s.io", "validatingwebhookconfigurations"},
		{"create", "admissionregistration.k8s.io", "mutatingwebhookconfigurations"},
	}

	var missing []string
	for _, c := range checks {
		allowed, err := ssarAllowed(ctx, clientset, c.verb, c.group, c.resource, "", "")
		if err != nil {
			return false, "", err
		}
		if !allowed {
			if c.group == "" {
				missing = append(missing, fmt.Sprintf("%s %s", c.verb, c.resource))
			} else {
				missing = append(missing, fmt.Sprintf("%s %s.%s", c.verb, c.resource, c.group))
			}
		}
	}

	if len(missing) == 0 {
		return true, "", nil
	}

	return false, fmt.Sprintf("missing permissions: %s", utils.JoinLimited(missing, 5)), nil
}

func ssarAllowed(ctx context.Context, clientset *kubernetes2.Clientset, verb, group, resource, namespace, name string) (bool, error) {
	ssar := &v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Verb:      verb,
				Group:     group,
				Resource:  resource,
				Namespace: namespace,
				Name:      name,
			},
		},
	}

	out, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, ssar, v2.CreateOptions{})
	if err != nil {
		return false, err
	}
	return out.Status.Allowed, nil
}
