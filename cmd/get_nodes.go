package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/completion"
	"github.com/weka/kubectl-weka/pkg/getters"
	"github.com/weka/kubectl-weka/pkg/printer"
)

var (
	getNodesSelector string
)

var getNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Get node information in Weka format",
	RunE:  runGetNodes,
}

func init() {
	getCmd.AddCommand(getNodesCmd)
	getNodesCmd.Flags().BoolVar(&flagNoHeaders, "no-headers", false, "Don't print headers")
	getNodesCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")
	getNodesCmd.Flags().StringVar(&getNodesSelector, "node-selector", "", "Label selector to filter nodes (e.g., role=storage)")
	getNodesCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		flags := completion.SuggestAllUnusedFlagsWithUsageForCompletion(cmd, args, toComplete)
		return flags, cobra.ShellCompDirectiveNoFileComp
	}
	getNodesCmd.RegisterFlagCompletionFunc("output", completionGetNodesOutput)
	getNodesCmd.RegisterFlagCompletionFunc("node-selector", completionListNodeSelectors)

	getNodesCmd.SilenceUsage = true
}

// runGetNodes executes the get nodes command
func runGetNodes(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	p, _ := printer.GetPrinterFromFlags(flagOutput, !flagNoHeaders, nil, false, 0, printer.TableStyleMinimal)
	output, err := getters.GenerateNodesOutput(ctx, KubeClients, p, getNodesSelector)
	if err != nil {
		return err
	}

	fmt.Println(output)
	return nil
}
