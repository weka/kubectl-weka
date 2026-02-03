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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
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

	// Detect CRD details (scope)
	crd, err := GetCRD(ctx, restCfg, wekaPoliciesCRDName)
	if err != nil {
		return err
	}

	// Create typed CR client
	crClient, err := newWekaCRClient(ctx, restCfg)
	if err != nil {
		return err
	}

	scope := crd.Spec.Scope
	includeNamespaceColumn := false

	// List policies according to scope/flags
	var list wekaapi.WekaPolicyList
	listOpts := []crclient.ListOption{}

	switch scope {
	case apiextv1.ClusterScoped:
		// cluster-wide list, namespace flags are hidden/enforced by AttachScopeAutoEnforce
		if err := crClient.List(ctx, &list); err != nil {
			return err
		}

	case apiextv1.NamespaceScoped:
		if flagAllNamespaces {
			includeNamespaceColumn = true
			if err := crClient.List(ctx, &list); err != nil {
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
			listOpts = append(listOpts, crclient.InNamespace(ns))
			if err := crClient.List(ctx, &list, listOpts...); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported CRD scope %q for %s", scope, wekaPoliciesCRDName)
	}

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)

	if !flagNoHeaders {
		if includeNamespaceColumn {
			if flagWide {
				fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE\tTYPE\tSTATUS\tPROGRESS")
			} else {
				fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE\tTYPE\tSTATUS")
			}
		} else {
			if flagWide {
				fmt.Fprintln(w, "NAME\tAGE\tTYPE\tSTATUS\tPROGRESS")
			} else {
				fmt.Fprintln(w, "NAME\tAGE\tTYPE\tSTATUS")
			}
		}
	}

	now := time.Now()
	for i := range list.Items {
		p := &list.Items[i]
		ns := p.GetNamespace()
		name := p.GetName()
		age := humanAge(now.Sub(p.GetCreationTimestamp().Time))

		pType := strings.TrimSpace(p.Spec.Type)
		if pType == "" {
			pType = "<none>"
		}

		pStatus := strings.TrimSpace(p.Status.Status)
		if pStatus == "" {
			pStatus = "<none>"
		}

		pProgress := strings.TrimSpace(p.Status.Progress)
		if pProgress == "" {
			pProgress = "<none>"
		}

		// NOTE: for typed resources we no longer print the raw spec blob by default,
		// because it can be large and not stable across versions.
		// If you want it back, we can add --wide-json or similar.

		if !flagWide {
			if includeNamespaceColumn {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ns, name, age, pType, pStatus)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, age, pType, pStatus)
			}
			continue
		}

		if includeNamespaceColumn {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", ns, name, age, pType, pStatus, pProgress)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, age, pType, pStatus, pProgress)
		}
	}

	w.Flush()
	return nil
}

// compactJSON is used for optional debug-friendly columns.
func compactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	return s
}

// ignoreNotFound makes it easier to treat "not found" as empty result.
func ignoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
