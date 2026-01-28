package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var getPoliciesCmd = &cobra.Command{
	Use:   "policies",
	Short: "List WekaPolicy custom resources",
	RunE:  runGetPolicies,
}

func init() {
	getCmd.AddCommand(getPoliciesCmd)

	// Auto-detect CRD scope and hide/enforce namespace flags accordingly.
	AttachScopeAutoEnforce(getPoliciesCmd, "wekapolicies.weka.weka.io")
}

func runGetPolicies(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load kubeconfig (kubectl-style)
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// Don't force namespace here; we will apply namespace logic after we detect scope.
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return err
	}

	// 1) Read CRD to auto-detect:
	//    - scope (Cluster/Namespaced)
	//    - group
	//    - resource plural
	//    - served/storage versions
	const crdName = "wekapolicies.weka.weka.io"

	ext, err := apiextclient.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	crd, err := ext.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	scope := crd.Spec.Scope // apiextv1.ClusterScoped or apiextv1.NamespaceScoped
	group := crd.Spec.Group
	resource := crd.Spec.Names.Plural // e.g. "wekapolicies"

	// Pick a version: prefer storage version, otherwise first served
	var version string
	for _, v := range crd.Spec.Versions {
		if v.Served && v.Storage {
			version = v.Name
			break
		}
	}
	if version == "" {
		for _, v := range crd.Spec.Versions {
			if v.Served {
				version = v.Name
				break
			}
		}
	}
	if version == "" {
		return fmt.Errorf("CRD %s has no served versions", crdName)
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// 2) Dynamic client for listing CR instances
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	// 3) List objects with correct scoping
	var list *unstructured.UnstructuredList
	includeNamespaceColumn := false

	switch scope {
	case apiextv1.ClusterScoped:
		// Cluster-scoped: never namespace, never all-namespaces, and -n/-A should be rejected by PreRunE.
		list, err = dyn.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

	case apiextv1.NamespaceScoped:
		// Namespaced: support -A and -n (or kubeconfig default namespace)
		if flagAllNamespaces {
			includeNamespaceColumn = true
			list, err = dyn.Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
		} else {
			ns := flagNamespace
			if ns == "" {
				ns, _, err = kubeCfg.Namespace()
				if err != nil {
					return err
				}
				if ns == "" {
					ns = "default"
				}
			}

			list, err = dyn.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("unsupported CRD scope %q for %s", scope, crdName)
	}

	// 4) Output
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)

	if !flagNoHeaders {
		if includeNamespaceColumn {
			if flagWide {
				fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE\tMODE\tSELECTOR\tSPEC")
			} else {
				fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE")
			}
		} else {
			if flagWide {
				fmt.Fprintln(w, "NAME\tAGE\tMODE\tSELECTOR\tSPEC")
			} else {
				fmt.Fprintln(w, "NAME\tAGE")
			}
		}
	}

	now := time.Now()
	for i := range list.Items {
		// You can keep your existing printPolicyRow(), but pass includeNamespaceColumn
		// instead of "allNS". Rename param if you want for clarity.
		printPolicyRow(w, &list.Items[i], now, includeNamespaceColumn, flagWide)
	}

	w.Flush()
	return nil
}

func printPolicyRow(w *tabwriter.Writer, u *unstructured.Unstructured, now time.Time, allNS bool, wide bool) {
	ns := u.GetNamespace()
	name := u.GetName()
	age := humanAge(now.Sub(u.GetCreationTimestamp().Time))

	if !wide {
		if allNS {
			fmt.Fprintf(w, "%s\t%s\t%s\n", ns, name, age)
		} else {
			fmt.Fprintf(w, "%s\t%s\n", name, age)
		}
		return
	}

	// "Wide" fields are best-effort and safe if missing.
	mode := nestedString(u.Object, "spec", "mode")
	if mode == "" {
		mode = "<none>"
	}

	// selector could be map; we'll render compact
	selector := nestedCompactJSON(u.Object, "spec", "selector")
	if selector == "" {
		selector = "<none>"
	}

	// spec summary: compact JSON of spec (may still be long; trimmed)
	spec := nestedCompactJSON(u.Object, "spec")
	if spec == "" {
		spec = "<none>"
	}
	spec = trimTo(spec, 120)

	if allNS {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", ns, name, age, mode, selector, spec)
	} else {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, age, mode, selector, spec)
	}
}

func nestedString(obj map[string]any, fields ...string) string {
	v, found, _ := unstructured.NestedString(obj, fields...)
	if !found {
		return ""
	}
	return v
}

func nestedCompactJSON(obj map[string]any, fields ...string) string {
	var v any
	var found bool
	var err error

	if len(fields) == 0 {
		v = obj
		found = true
	} else {
		v, found, err = unstructured.NestedFieldNoCopy(obj, fields...)
		if err != nil || !found || v == nil {
			return ""
		}
	}

	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	// Compact a bit for readability
	s := string(b)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	return s
}

func humanAge(d time.Duration) string {
	// simple kubectl-like age (seconds/minutes/hours/days)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func trimTo(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func gvrListString(gvrs []schema.GroupVersionResource) string {
	parts := make([]string, 0, len(gvrs))
	for _, g := range gvrs {
		parts = append(parts, fmt.Sprintf("%s/%s/%s", g.Group, g.Version, g.Resource))
	}
	return strings.Join(parts, ", ")
}
