package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

var flagOutput string
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

func runGetPolicies(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()

	crClient := KubeClients.CRClient

	// List policies according to scope/flags
	var list wekaapi.WekaPolicyList
	var listOpts []crclient.ListOption

	ns, _, err := GetNamespaceFromFlags(flagAllNamespaces, flagNamespace)
	if err != nil {
		return err
	}
	if !flagAllNamespaces {
		listOpts = append(listOpts, crclient.InNamespace(ns))
	}

	if err := crClient.List(ctx, &list, listOpts...); err != nil {
		return err
	}

	// Build columns
	var columns []TableColumn
	columns = []TableColumn{
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "NAME", VisibleInWide: false},
		{Name: "AGE", VisibleInWide: false, TableFormatFunctions: []func(interface{}) string{humanAge}},
		{Name: "TYPE", VisibleInWide: false},
		{Name: "STATUS", VisibleInWide: false},
		{Name: "PROGRESS", VisibleInWide: true},
	}

	// Build rows
	var rows []TableRow
	for i := range list.Items {
		p := &list.Items[i]
		row := TableRow{Values: map[string]interface{}{}}
		row.Values["NAMESPACE"] = p.GetNamespace()
		row.Values["NAME"] = p.GetName()
		row.Values["AGE"] = p.GetCreationTimestamp().Time
		row.Values["TYPE"] = strings.TrimSpace(p.Spec.Type)
		row.Values["STATUS"] = strings.TrimSpace(p.Status.Status)
		row.Values["PROGRESS"] = strings.TrimSpace(p.Status.Progress)
		rows = append(rows, row)
	}

	// Select printer
	printer, _ := GetPrinterFromFlags(flagOutput, !flagNoHeaders, nil, false, 0, TableStyleMinimal)

	err = printer.Print(columns, rows, cmd.OutOrStdout())
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout()) // Add newline after table
	return nil
}
