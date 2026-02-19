package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
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

	fmt.Println("Performing preflight verification for Kubernetes cluster")
	fmt.Println()

	clientset := KubeClients.Clientset
	crClient := KubeClients.CRClient

	// Resolve nodes once using cached client (used for node-scoped cluster checks: cpu policy, CNI health)
	fmt.Print("🔍 Connecting to cluster and discovering nodes... ")
	nodes, err := resolveNodes(ctx, args, preflightNodeSelector)
	if err != nil {
		fmt.Println(red("FAILED"))
		return err
	}
	fmt.Println(green(fmt.Sprintf("found %d nodes", len(nodes))))

	// Filter ready nodes and count
	readyNodes := FilterReadyNodes(nodes)
	readyCount := len(readyNodes)
	notReadyCount := len(nodes) - readyCount

	if notReadyCount > 0 {
		fmt.Printf("   %s %d ready, %s %d not ready (checks will skip NotReady nodes to avoid timeouts)\n",
			green("✓"), readyCount, yellow("⚠"), notReadyCount)
	} else {
		fmt.Printf("   %s All %d nodes are ready\n", green("✓"), readyCount)
	}
	fmt.Println()

	// Set up validation parameters - include only ready nodes for node-specific checks
	validationParams := map[string]interface{}{
		"nodes":      nodes,      // All nodes for NotReady check
		"readyNodes": readyNodes, // Only ready nodes for other checks
	}

	fmt.Println("⚙️  Running cluster validation checks (this may take a minute)...")
	fmt.Println()

	// Run all cluster checks using the registry
	results, err := GlobalClusterCheckRegistry.ValidateAll(ctx, clientset, crClient, validationParams)
	if err != nil {
		return fmt.Errorf("failed to validate cluster: %w", err)
	}

	// Print results
	GlobalClusterCheckRegistry.PrintCheckResults(results)

	// Check if any validations failed or warned
	anyFail := false
	anyWarn := false
	var warnings []string
	var errors []string

	for _, result := range results {
		if result.Status == "error" {
			anyFail = true
			module, _ := GlobalClusterCheckRegistry.Get(result.ModuleName)
			if module != nil {
				errors = append(errors, module.FriendlyName())
			}
		} else if result.Status == "warning" {
			anyWarn = true
			module, _ := GlobalClusterCheckRegistry.Get(result.ModuleName)
			if module != nil {
				warnings = append(warnings, module.FriendlyName())
			}
		}
	}

	// Print warnings summary if any
	if anyWarn {
		fmt.Println("\n" + yellow("=== Warnings Summary ==="))
		for _, result := range results {
			if result.Status == "warning" {
				module, _ := GlobalClusterCheckRegistry.Get(result.ModuleName)
				if module == nil {
					continue
				}

				// Build context params
				contextParams := map[string]interface{}{
					"FriendlyName": module.FriendlyName(),
				}
				if dataMap, ok := result.Data.(map[string]interface{}); ok {
					for k, v := range dataMap {
						contextParams[k] = v
					}
				}

				fmt.Printf("\n⚠️  %s\n", yellow(module.FriendlyName()))
				if issue, ok := contextParams["Issue"].(string); ok && issue != "" {
					fmt.Printf("   %s\n", issue)
				}

				// Show affected nodes if available
				if affectedNodes, ok := contextParams["AffectedNodes"].([]string); ok && len(affectedNodes) > 0 {
					affectedCount := len(affectedNodes)
					nodesList := strings.Join(affectedNodes, ", ")
					if affectedCount > 10 {
						nodesList = strings.Join(affectedNodes[:10], ", ") + fmt.Sprintf(" and %d more", affectedCount-10)
					}
					fmt.Printf("   Affected nodes (%d): %s\n", affectedCount, nodesList)
				} else if notReadyNodes, ok := contextParams["NotReadyNodes"].([]string); ok && len(notReadyNodes) > 0 {
					// Handle NotReadyNodesModule specifically (uses NotReadyNodes field)
					affectedCount := len(notReadyNodes)
					nodesList := strings.Join(notReadyNodes, ", ")
					if affectedCount > 10 {
						nodesList = strings.Join(notReadyNodes[:10], ", ") + fmt.Sprintf(" and %d more", affectedCount-10)
					}
					fmt.Printf("   Affected nodes (%d): %s\n", affectedCount, nodesList)
				}
			}
		}
	}

	if anyFail {
		// Print suggested fixes for failed checks
		fmt.Println("\n" + red("=== Errors & Suggested Fixes ==="))
		for _, result := range results {
			if result.Status == "error" {
				module, _ := GlobalClusterCheckRegistry.Get(result.ModuleName)
				if module == nil {
					continue
				}

				// Build context params for fix interpolation
				fixParams := map[string]interface{}{}
				if dataMap, ok := result.Data.(map[string]interface{}); ok {
					for k, v := range dataMap {
						fixParams[k] = v
					}
				}

				suggestedFix := result.FormatSuggestedFix(fixParams)
				fmt.Printf("\n❌ %s\n", red(module.FriendlyName()))
				if issue, ok := fixParams["Issue"].(string); ok && issue != "" {
					fmt.Printf("   Issue: %s\n", issue)
				}

				// Show affected nodes if available
				if affectedNodes, ok := fixParams["AffectedNodes"].([]string); ok && len(affectedNodes) > 0 {
					affectedCount := len(affectedNodes)
					nodesList := strings.Join(affectedNodes, ", ")
					if affectedCount > 10 {
						nodesList = strings.Join(affectedNodes[:10], ", ") + fmt.Sprintf(" and %d more", affectedCount-10)
					}
					fmt.Printf("   Affected nodes (%d): %s\n", affectedCount, nodesList)
				}

				if suggestedFix != "" {
					fmt.Printf("   💡 Suggested fix: %s\n", suggestedFix)
				}
			}
		}

		return fmt.Errorf("preflight cluster failed")
	}

	return nil
}
