package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/plan"
)

var planConvergedCmd = &cobra.Command{
	Use:   "converged <cluster.yaml> <client.yaml>",
	Short: "Plan converged deployment (cluster + client on same nodes)",
	Args:  cobra.ExactArgs(2),
	RunE:  runPlanConverged,
}

func init() {
	planCmd.AddCommand(planConvergedCmd)
	planConvergedCmd.SilenceUsage = true
	planConvergedCmd.ValidArgsFunction = completionListAllYamlFilesInDirectory

}

func runPlanConverged(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	if len(args) != 2 {
		return fmt.Errorf("please specify two WekaCluster and WekaClient manifest files as arguments")
	}
	clusterFile := args[0]
	clientFile := args[1]

	// Parse YAML files
	wekaConfigContext, err := plan.ParseAndValidateConfigs(ctx, KubeClients, clusterFile, clientFile)
	if err != nil {
		return err
	}

	// Validate and plan converged deployment
	if err := plan.ValidateAndPlanConverged(ctx, KubeClients, wekaConfigContext.Cluster, wekaConfigContext.Client); err != nil {
		return err
	}

	return nil
}
