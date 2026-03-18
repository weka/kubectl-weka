package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/plan"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

var planClusterCmd = &cobra.Command{
	Use:   "cluster <file.yaml>",
	Short: "Plan cluster deployment from WekaCluster YAML file",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanCluster,
}

func init() {
	planCmd.AddCommand(planClusterCmd)
	planClusterCmd.Flags().BoolVar(&flagFailFast, "fail-fast", false, "Stop validation on first error (default: collect all errors)")
	planClusterCmd.SilenceUsage = true
}

func runPlanCluster(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	filePath := args[0]

	cluster, err := plan.ParseWekaResourceFile[*wekaapi.WekaCluster](filePath)
	if err != nil {
		return fmt.Errorf("failed to parse WekaCluster file: %w", err)
	}

	if err := plan.ValidateAndPlanCluster(ctx, KubeClients, cluster); err != nil {
		return err
	}

	return nil
}
