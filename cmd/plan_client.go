package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/plan"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

var planClientCmd = &cobra.Command{
	Use:   "client <file.yaml>",
	Short: "Plan client deployment from WekaClient YAML file",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanClient,
}

func init() {
	planCmd.AddCommand(planClientCmd)
	planClientCmd.Flags().BoolVar(&flagFailFast, "fail-fast", false, "Stop validation on first error (default: collect all errors)")

	planClientCmd.ValidArgs = []cobra.Completion{"yml", "yaml"}
	planClientCmd.CompletionOptions.SetDefaultShellCompDirective(cobra.ShellCompDirectiveFilterFileExt | cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveFilterDirs)

	planClientCmd.SilenceUsage = true
}

func runPlanClient(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	filePath := args[0]

	wekaClient, err := plan.ParseWekaResourceFile[*wekaapi.WekaClient](filePath)
	if err != nil {
		return fmt.Errorf("failed to parse WekaClient file: %w", err)
	}

	// Validate client and plan allocation
	if err := plan.ValidateAndPlanClient(ctx, KubeClients, wekaClient); err != nil {
		return err
	}
	return nil
}
