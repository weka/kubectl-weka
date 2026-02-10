package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

var getPoliciesCmd = &cobra.Command{
	Use:   "policies",
	Short: "List WekaPolicy custom resources",
	Args:  cobra.NoArgs,
	RunE:  runGetPolicies,
}

func init() {
	getCmd.AddCommand(getPoliciesCmd)
	getPoliciesCmd.Flags().BoolVarP(&flagAllNamespaces, "all-namespaces", "A", false, "If present, list WekaPolicy resources across all namespaces")
	getPoliciesCmd.Flags().StringVarP(&flagNamespace, "namespace", "n", "", "Namespace. Defaults to current kubeconfig namespace")
	getPoliciesCmd.Flags().BoolVar(&flagNoHeaders, "no-headers", false, "Don't print headers")
	getPoliciesCmd.Flags().BoolVar(&flagWide, "wide", false, "Wide output (adds PROGRESS column)")
	getPoliciesCmd.SilenceUsage = true
}

func runGetPolicies(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	crClient := KubeClients.CRClient

	includeNamespaceColumn := false

	// List policies according to scope/flags
	var list wekaapi.WekaPolicyList
	listOpts := []crclient.ListOption{}

	var err error
	if flagAllNamespaces {
		includeNamespaceColumn = true
		if err = crClient.List(ctx, &list); err != nil {
			return err
		}
	} else {
		ns := flagNamespace
		if ns == "" {
			ns, err = GetKubeNamespace()
			if err != nil {
				return err
			}
		}
		listOpts = append(listOpts, crclient.InNamespace(ns))
		if err := crClient.List(ctx, &list, listOpts...); err != nil {
			return err
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	if len(list.Items) == 0 {
		if includeNamespaceColumn {
			fmt.Printf("No resources found.\n")
		} else if flagNamespace != "" {
			fmt.Printf("No resources found in namespace %q.\n", flagNamespace)
		} else {
			fmt.Printf("No resources found in default namespace.\n")
		}
		return nil
	}

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
