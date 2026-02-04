package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/util/version"
	"strings"

	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Shared selection flag (used by both preflight subcommands)
var preflightNodeSelector string

func resolveNodes(ctx context.Context, client crclient.Client, names []string, selector string) ([]corev1.Node, error) {
	if len(names) > 0 {
		out := make([]corev1.Node, 0, len(names))
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			var node corev1.Node
			if err := client.Get(ctx, crclient.ObjectKey{Name: n}, &node); err != nil {
				return nil, err
			}
			out = append(out, node)
		}
		return out, nil
	}

	var nodeList corev1.NodeList
	opts := []crclient.ListOption{}
	if selector != "" {
		opts = append(opts, crclient.MatchingLabels(parseSelector(selector)))
	}

	if err := client.List(ctx, &nodeList, opts...); err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

// parseSelector converts a label selector string to a map for crclient.MatchingLabels
func parseSelector(selector string) map[string]string {
	result := make(map[string]string)
	if selector == "" {
		return result
	}

	pairs := strings.Split(selector, ",")
	for _, pair := range pairs {
		kv := strings.Split(strings.TrimSpace(pair), "=")
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func firstInternalIP(n *corev1.Node) string {
	for _, a := range n.Status.Addresses {
		if a.Type == corev1.NodeInternalIP {
			return a.Address
		}
	}
	if len(n.Status.Addresses) > 0 {
		return n.Status.Addresses[0].Address
	}
	return ""
}

func quantityOrZero(rl corev1.ResourceList, key corev1.ResourceName) resource.Quantity {
	if q, ok := rl[key]; ok {
		return q.DeepCopy()
	}
	return resource.MustParse("0")
}

// kubelet configz via apiserver proxy:
// GET /api/v1/nodes/<node>/proxy/configz
func getCPUManagerPolicyViaConfigz(ctx context.Context, clientset *kubernetes.Clientset, nodeName string) (string, error) {
	raw, err := clientset.CoreV1().
		RESTClient().
		Get().
		AbsPath("/api/v1/nodes", nodeName, "proxy", "configz").
		Do(ctx).
		Raw()
	if err != nil {
		return "", err
	}

	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return "", err
	}

	if kc, ok := top["kubeletconfig"].(map[string]any); ok {
		if v, ok := kc["cpuManagerPolicy"].(string); ok && v != "" {
			return v, nil
		}
	}

	for _, v := range top {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if pol, ok := m["cpuManagerPolicy"].(string); ok && pol != "" {
			return pol, nil
		}
	}

	return "", fmt.Errorf("cpuManagerPolicy not found in configz")
}

func printCheckResult(title string, pass bool, details string) {
	// one-line output: "<title> PASS" or "<title> FAIL [..]"
	if pass {
		if details != "" {
			fmt.Printf("%s %s [%s]\n", title, green("PASS"), details)
		} else {
			fmt.Printf("%s %s\n", title, green("PASS"))
		}
		return
	}

	if details != "" {
		fmt.Printf("%s %s [%s]\n", title, red("FAIL"), details)
	} else {
		fmt.Printf("%s %s\n", title, red("FAIL"))
	}
}

func buildCPUDetails(notStatic []string, unknown []string) string {
	parts := make([]string, 0, 2)

	if len(notStatic) > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes not static: %s", len(notStatic), joinLimited(notStatic, 5)))
	}
	if len(unknown) > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes unknown: %s", len(unknown), joinLimited(unknown, 3)))
	}

	return strings.Join(parts, "; ")
}

func joinLimited(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(", ...(+%d)", len(items)-max)
}

func shortErr(err error) string {
	// Keep it readable inside brackets
	s := err.Error()
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	//if len(s) > 60 {
	//	return s[:57] + "..."
	//}
	return s
}

// Simple ANSI colors. If you want, we can auto-disable color when not a TTY / NO_COLOR.
func green(s string) string  { return "\033[32m" + s + "\033[0m" }
func red(s string) string    { return "\033[31m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func cyan(s string) string   { return "\033[36m" + s + "\033[0m" }

func checkK8sVersion124Plus(ctx context.Context, clientset *kubernetes.Clientset) (bool, string, error) {
	sv, err := clientset.Discovery().ServerVersion()
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

func checkNotOpenShiftOrROSA(ctx context.Context, clientset *kubernetes.Clientset) (bool, string, error) {
	// Detect OpenShift by API groups exposed by the apiserver.
	// If any are present, it's OpenShift (including ROSA / managed OpenShift).
	grps, err := clientset.Discovery().ServerGroups()
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
	rosaHint := detectROSAHint(ctx, clientset)

	if rosaHint != "" {
		return false, fmt.Sprintf("OpenShift detected (%s); ROSA hint: %s", strings.Join(found, ","), rosaHint), nil
	}
	return false, fmt.Sprintf("OpenShift detected (%s)", strings.Join(found, ",")), nil
}

func detectROSAHint(ctx context.Context, clientset *kubernetes.Clientset) string {
	// This is heuristic: safe and optional.
	nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
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

func checkHelmClusterPermissions(ctx context.Context, clientset *kubernetes.Clientset) (bool, string, error) {
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

	return false, fmt.Sprintf("missing permissions: %s", joinLimited(missing, 5)), nil
}

func ssarAllowed(ctx context.Context, clientset *kubernetes.Clientset, verb, group, resource, namespace, name string) (bool, error) {
	ssar := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:      verb,
				Group:     group,
				Resource:  resource,
				Namespace: namespace,
				Name:      name,
			},
		},
	}

	out, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, ssar, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	return out.Status.Allowed, nil
}

func checkCNIConfigured(ctx context.Context, clientset *kubernetes.Clientset, nodes []corev1.Node) (bool, string, error) {
	// A) Node condition check
	var badNodes []string
	for i := range nodes {
		n := &nodes[i]
		nu := getNodeConditionStatus(n, corev1.NodeNetworkUnavailable)
		// If condition missing, treat as unknown (not immediate fail), but note later.
		if nu == corev1.ConditionTrue {
			badNodes = append(badNodes, n.Name)
		}
	}

	// B) Known CNI daemonset presence (kube-system)
	hasKnownCNI, cniName, err := detectKnownCNIDaemonSet(ctx, clientset)
	if err != nil {
		return false, "", err
	}

	if len(badNodes) == 0 && hasKnownCNI {
		return true, "", nil
	}

	parts := make([]string, 0, 2)
	if !hasKnownCNI {
		parts = append(parts, "no known CNI daemonset found in kube-system")
	} else {
		parts = append(parts, fmt.Sprintf("detected CNI: %s", cniName))
	}
	if len(badNodes) > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes NetworkUnavailable=true: %s", len(badNodes), joinLimited(badNodes, 5)))
	}

	// If we detected a known CNI but a few nodes are bad, still FAIL (this is preflight).
	return false, strings.Join(parts, "; "), nil
}

func getNodeConditionStatus(n *corev1.Node, t corev1.NodeConditionType) corev1.ConditionStatus {
	for _, c := range n.Status.Conditions {
		if c.Type == t {
			return c.Status
		}
	}
	return ""
}

func detectKnownCNIDaemonSet(ctx context.Context, clientset *kubernetes.Clientset) (bool, string, error) {
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
		dsList, err := clientset.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
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
			if hasAnyLabelValue(labels, []string{"k8s-app", "app"}, []string{"flannel", "kube-flannel"}) {
				return true, fmt.Sprintf("%s/%s", ns, name), nil
			}
			if hasAnyLabelValue(labels, []string{"k8s-app", "app"}, []string{"calico-node", "cilium", "weave-net", "antrea"}) {
				return true, fmt.Sprintf("%s/%s", ns, name), nil
			}
		}
	}

	// 3) Fallback: look for CNI pods (covers cases where CNI isn't a DS, or DS is elsewhere)
	// Try kube-system + kube-flannel; if flannel exists, it’s usually visible here.
	for _, ns := range namespaces {
		pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				continue
			}
			return false, "", err
		}
		for _, p := range pods.Items {
			name := p.Name
			for _, sub := range knownSubstrings {
				if strings.Contains(name, sub) {
					return true, fmt.Sprintf("%s/%s", ns, name), nil
				}
			}
			if hasAnyLabelValue(p.Labels, []string{"k8s-app", "app"}, []string{"flannel", "kube-flannel"}) {
				return true, fmt.Sprintf("%s/%s", ns, name), nil
			}
		}
	}

	return false, "", nil
}

func hasAnyLabelValue(labels map[string]string, keys []string, values []string) bool {
	for _, k := range keys {
		if v, ok := labels[k]; ok {
			for _, want := range values {
				if v == want {
					return true
				}
			}
		}
	}
	return false
}

func checkCPUManagerPolicyStatic(ctx context.Context, clientset *kubernetes.Clientset, nodes []corev1.Node) (bool, string) {
	var notStatic []string
	var unknown []string

	for i := range nodes {
		n := &nodes[i]

		pol, err := getCPUManagerPolicyViaConfigz(ctx, clientset, n.Name)
		if err != nil {
			unknown = append(unknown, fmt.Sprintf("%s=%s", n.Name, shortErr(err)))
			continue
		}

		if strings.ToLower(strings.TrimSpace(pol)) != "static" {
			notStatic = append(notStatic, fmt.Sprintf("%s=%s", n.Name, pol))
		}
	}

	if len(notStatic) == 0 && len(unknown) == 0 {
		return true, ""
	}

	parts := make([]string, 0, 2)
	if len(notStatic) > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes not static: %s", len(notStatic), joinLimited(notStatic, 5)))
	}
	if len(unknown) > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes unknown: %s", len(unknown), joinLimited(unknown, 3)))
	}
	return false, strings.Join(parts, "; ")
}

func nodeHasMellanoxNIC(n *corev1.Node) (bool, string) {
	// Best-effort detection via Node labels (common with NFD / Mellanox operator / SR-IOV).
	// Mellanox PCI vendor ID is 15b3.
	for k, v := range n.Labels {
		kl := strings.ToLower(k)
		vl := strings.ToLower(v)

		// Node Feature Discovery style:
		// feature.node.kubernetes.io/pci-15b3.present=true
		if strings.Contains(kl, "pci-15b3") && (vl == "true" || vl == "1" || vl == "present") {
			return true, k + "=" + v
		}

		// Some environments label explicit vendor/model
		if strings.Contains(kl, "mellanox") || strings.Contains(vl, "mellanox") {
			return true, k + "=" + v
		}

		// NVIDIA acquired Mellanox; sometimes labels say nvidia + mlx
		if (strings.Contains(kl, "nvidia") || strings.Contains(vl, "nvidia")) &&
			(strings.Contains(kl, "mlx") || strings.Contains(vl, "mlx") || strings.Contains(vl, "connectx") || strings.Contains(vl, "cx")) {
			return true, k + "=" + v
		}

		// SR-IOV / RDMA hints (less strict; still useful)
		if strings.Contains(kl, "rdma") && (vl == "true" || vl == "enabled") {
			return true, k + "=" + v
		}
	}

	return false, ""
}

func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType { return &t }

func sanitizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}

func readPodLogs(ctx context.Context, clientset *kubernetes.Clientset, ns, pod, container string) (string, error) {
	req := clientset.CoreV1().Pods(ns).GetLogs(pod, &corev1.PodLogOptions{Container: container})
	b, err := req.Do(ctx).Raw()
	if err != nil {
		return "", err
	}
	return string(b), nil
}
