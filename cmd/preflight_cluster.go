package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

var preflightK8sClusterCmd = &cobra.Command{
	Use:   "cluster [NODE...]",
	Short: "Preflight cluster checks (platform, permissions, kubelet configuration)",
	Args:  cobra.ArbitraryArgs,
	RunE:  runPreflightK8sCluster,
}

func init() {
	preflightCmd.AddCommand(preflightK8sClusterCmd)
	preflightK8sClusterCmd.Flags().StringVar(&preflightNodeSelector, "node-selector", "", "Label selector to filter nodes for node-scoped cluster checks")
}

func runPreflightK8sCluster(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Create output that writes to stdout in real-time
	output := NewPreflightOutput(os.Stdout)
	defer output.Close()

	// Run the preflight checks
	result := generatePreflightK8sClusterOutput(ctx, args, preflightNodeSelector, output)

	if result.Error != nil {
		return result.Error
	}

	if !result.Success {
		return fmt.Errorf("preflight cluster failed")
	}

	return nil
}

// generatePreflightK8sClusterOutput performs K8s cluster preflight checks and streams output
func generatePreflightK8sClusterOutput(
	ctx context.Context,
	nodeArgs []string,
	nodeSelector string,
	output *PreflightOutput,
) *PreflightK8sResult {
	result := &PreflightK8sResult{}

	output.Println("Performing preflight verification for Kubernetes cluster")
	output.Println("")

	clientset := KubeClients.Clientset
	crClient := KubeClients.CRClient

	// Resolve nodes once using cached client (used for node-scoped cluster checks: cpu policy, CNI health)
	output.Printf("🔍 Connecting to cluster and discovering nodes... ")
	nodes, err := resolveNodes(ctx, nodeArgs, nodeSelector)
	if err != nil {
		output.Println(red("FAILED"))
		result.Error = err
		return result
	}
	output.Println(green(fmt.Sprintf("found %d nodes", len(nodes))))

	// Filter ready nodes and count
	readyNodes := FilterReadyNodes(nodes)
	readyCount := len(readyNodes)
	notReadyCount := len(nodes) - readyCount

	if notReadyCount > 0 {
		output.Printf("   %s %d ready, %s %d not ready (checks will skip NotReady nodes to avoid timeouts)\n",
			green("✓"), readyCount, yellow("⚠"), notReadyCount)
	} else {
		output.Printf("   %s All %d nodes are ready\n", green("✓"), readyCount)
	}
	output.Println("")

	// Set up validation parameters - include only ready nodes for node-specific checks
	validationParams := map[string]interface{}{
		"nodes":      nodes,      // All nodes for NotReady check
		"readyNodes": readyNodes, // Only ready nodes for other checks
	}

	output.Println("⚙️  Running cluster validation checks (this may take a minute)...")
	output.Println("")

	// Run all cluster checks using the registry
	results, err := GlobalClusterCheckRegistry.ValidateAll(ctx, clientset, crClient, validationParams)
	if err != nil {
		result.Error = fmt.Errorf("failed to validate cluster: %w", err)
		return result
	}

	// Format and print results
	formattedResults := GlobalClusterCheckRegistry.FormatCheckResults(results)
	output.Println(formattedResults)

	// Check if any validations failed or warned
	anyFail := false
	anyWarn := false

	for _, r := range results {
		if r.Status == "error" {
			anyFail = true
		} else if r.Status == "warning" {
			anyWarn = true
		}
	}

	// Print warnings summary if any
	if anyWarn {
		output.Println("\n" + yellow("=== Warnings Summary ==="))
		for _, r := range results {
			if r.Status == "warning" {
				module, _ := GlobalClusterCheckRegistry.Get(r.ModuleName)
				if module == nil {
					continue
				}

				// Build context params
				contextParams := map[string]interface{}{
					"FriendlyName": module.FriendlyName(),
				}
				if dataMap, ok := r.Data.(map[string]interface{}); ok {
					for k, v := range dataMap {
						contextParams[k] = v
					}
				}

				output.Printf("\n⚠️  %s\n", yellow(module.FriendlyName()))
				if issue, ok := contextParams["Issue"].(string); ok && issue != "" {
					output.Printf("   %s\n", issue)
				}

				// Show affected nodes if available
				if affectedNodes, ok := contextParams["AffectedNodes"].([]string); ok && len(affectedNodes) > 0 {
					affectedCount := len(affectedNodes)
					nodesList := strings.Join(affectedNodes, ", ")
					if affectedCount > 10 {
						nodesList = strings.Join(affectedNodes[:10], ", ") + fmt.Sprintf(" and %d more", affectedCount-10)
					}
					output.Printf("   Affected nodes (%d): %s\n", affectedCount, nodesList)
				} else if notReadyNodes, ok := contextParams["NotReadyNodes"].([]string); ok && len(notReadyNodes) > 0 {
					// Handle NotReadyNodesModule specifically (uses NotReadyNodes field)
					affectedCount := len(notReadyNodes)
					nodesList := strings.Join(notReadyNodes, ", ")
					if affectedCount > 10 {
						nodesList = strings.Join(notReadyNodes[:10], ", ") + fmt.Sprintf(" and %d more", affectedCount-10)
					}
					output.Printf("   Affected nodes (%d): %s\n", affectedCount, nodesList)
				}
			}
		}
	}

	if anyFail {
		// Print suggested fixes for failed checks
		output.Println("\n" + red("=== Errors & Suggested Fixes ==="))
		for _, r := range results {
			if r.Status == "error" {
				module, _ := GlobalClusterCheckRegistry.Get(r.ModuleName)
				if module == nil {
					continue
				}

				// Build context params for fix interpolation
				fixParams := map[string]interface{}{}
				if dataMap, ok := r.Data.(map[string]interface{}); ok {
					for k, v := range dataMap {
						fixParams[k] = v
					}
				}

				suggestedFix := r.FormatSuggestedFix(fixParams)
				output.Printf("\n❌ %s\n", red(module.FriendlyName()))
				if issue, ok := fixParams["Issue"].(string); ok && issue != "" {
					output.Printf("   Issue: %s\n", issue)
				}

				// Show affected nodes if available
				if affectedNodes, ok := fixParams["AffectedNodes"].([]string); ok && len(affectedNodes) > 0 {
					affectedCount := len(affectedNodes)
					nodesList := strings.Join(affectedNodes, ", ")
					if affectedCount > 10 {
						nodesList = strings.Join(affectedNodes[:10], ", ") + fmt.Sprintf(" and %d more", affectedCount-10)
					}
					output.Printf("   Affected nodes (%d): %s\n", affectedCount, nodesList)
				}

				if suggestedFix != "" {
					output.Printf("   💡 Suggested fix: %s\n", suggestedFix)
				}
			}
		}
	}

	result.Success = !anyFail
	result.Output = output.GetFullOutput()
	return result
}
