package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/utils"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	getPoliciesCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")
	getPoliciesCmd.SilenceUsage = true
}

func runGetPolicies(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	ns, _, err := kubernetes.GetNamespaceFromFlags(flagAllNamespaces, flagNamespace)
	if err != nil {
		return err
	}

	p, _ := printer.GetPrinterFromFlags(flagOutput, !flagNoHeaders, nil, false, 0, printer.TableStyleMinimal)

	var targetName string
	if len(args) == 1 {
		targetName = args[0]
		if flagAllNamespaces {
			return fmt.Errorf("cannot use -A/--all-namespaces when specifying a WekaPolicy name; use -n to choose namespace")
		}
	}

	output, err := GenerateGetPoliciesOutput(ctx, KubeClients, ns, flagAllNamespaces, targetName, p)
	if err != nil {
		return err
	}

	fmt.Println(output)
	return nil
}

// GenerateGetPoliciesOutput generates the policies output table as a string
func GenerateGetPoliciesOutput(ctx context.Context, clients *kubernetes.K8sClients, namespace string, allNamespaces bool, targetPolicy string, printerObj printer.ResourcePrinter) (string, error) {
	crClient := clients.CRClient

	// List policies according to scope/flags
	list, err := GetWekaPolicies(ctx, crClient, namespace, allNamespaces, targetPolicy)
	if err != nil {
		return "", err
	}

	// Build columns
	columns := []printer.TableColumn{
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "NAME", VisibleInWide: false},
		{Name: "AGE", VisibleInWide: false, FormatFuncs: printer.TableFormatFunctions{utils.HumanAge}},
		{Name: "TYPE", VisibleInWide: false},
		{Name: "STATUS", VisibleInWide: false},
		{Name: "PROGRESS", VisibleInWide: true},
	}

	// Build rows
	var rows []printer.TableRow
	for i := range list {
		p := list[i]
		row := printer.TableRow{Values: map[string]interface{}{}}
		row.Values["NAMESPACE"] = p.GetNamespace()
		row.Values["NAME"] = p.GetName()
		row.Values["AGE"] = p.GetCreationTimestamp().Time
		row.Values["TYPE"] = strings.TrimSpace(p.Spec.Type)
		row.Values["STATUS"] = strings.TrimSpace(p.Status.Status)
		row.Values["PROGRESS"] = strings.TrimSpace(p.Status.Progress)
		rows = append(rows, row)
	}

	// Select printer
	var buf strings.Builder
	if err := printerObj.Print(columns, rows, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func GetWekaPolicies(ctx context.Context, c client.Client, ns string, allNS bool, name string) ([]wekaapi.WekaPolicy, error) {
	if name != "" {
		var wp wekaapi.WekaPolicy
		err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &wp)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get WekaCluster %q in namespace %q: %w", name, ns, err)
		}
		return []wekaapi.WekaPolicy{wp}, nil
	}

	var lst wekaapi.WekaPolicyList
	opts := []client.ListOption{}
	if !allNS {
		opts = append(opts, client.InNamespace(ns))
	}
	if err := c.List(ctx, &lst, opts...); err != nil {
		return nil, fmt.Errorf("failed to list WekaCluster CRs: %w", err)
	}
	return lst.Items, nil
}
