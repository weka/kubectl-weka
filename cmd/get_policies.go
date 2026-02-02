package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

const wekaPoliciesCRDName = "wekapolicies.weka.weka.io"

var getPoliciesCmd = &cobra.Command{
	Use:   "policies",
	Short: "List WekaPolicy custom resources",
	Args:  cobra.NoArgs,
	RunE:  runGetPolicies,
}

func init() {
	getCmd.AddCommand(getPoliciesCmd)

	// Automatically hide/enforce namespace flags based on CRD scope (Cluster vs Namespaced)
	AttachScopeAutoEnforce(getPoliciesCmd, wekaPoliciesCRDName)
}

func runGetPolicies(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return err
	}

	// Detect CRD details (scope, group, plural, version)
	crd, err := GetCRD(ctx, restCfg, wekaPoliciesCRDName)
	if err != nil {
		return err
	}

	group := crd.Spec.Group
	resource := crd.Spec.Names.Plural
	version, err := PickCRDVersion(crd)
	if err != nil {
		return err
	}

	scope := crd.Spec.Scope
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}

	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	var list *unstructured.UnstructuredList
	includeNamespaceColumn := false

	switch scope {
	case apiextv1.ClusterScoped:
		// always cluster scope
		list, err = dyn.Resource(gvr).List(ctx, metav1.ListOptions{})

	case apiextv1.NamespaceScoped:
		// allow -A / -n for namespaced resources
		if flagAllNamespaces {
			includeNamespaceColumn = true
			list, err = dyn.Resource(gvr).List(ctx, metav1.ListOptions{})
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
		}

	default:
		return fmt.Errorf("unsupported CRD scope %q for %s", scope, wekaPoliciesCRDName)
	}

	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)

	if !flagNoHeaders {
		if includeNamespaceColumn {
			if flagWide {
				fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE\tSPEC")
			} else {
				fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE")
			}
		} else {
			if flagWide {
				fmt.Fprintln(w, "NAME\tAGE\tSPEC")
			} else {
				fmt.Fprintln(w, "NAME\tAGE")
			}
		}
	}

	now := time.Now()
	for i := range list.Items {
		item := &list.Items[i]
		ns := item.GetNamespace()
		name := item.GetName()
		age := humanAge(now.Sub(item.GetCreationTimestamp().Time))

		if !flagWide {
			if includeNamespaceColumn {
				fmt.Fprintf(w, "%s\t%s\t%s\n", ns, name, age)
			} else {
				fmt.Fprintf(w, "%s\t%s\n", name, age)
			}
			continue
		}

		spec := nestedCompactJSON(item.Object, "spec")
		if spec == "" {
			spec = "<none>"
		}
		spec = trimTo(spec, 160)

		if includeNamespaceColumn {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ns, name, age, spec)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, age, spec)
		}
	}

	w.Flush()
	return nil
}

func nestedCompactJSON(obj map[string]any, fields ...string) string {
	v, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !found || v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	return s
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
