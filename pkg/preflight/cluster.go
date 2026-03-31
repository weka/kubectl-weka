package preflight

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/clustercheck"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/utils"
	"strings"
)

// GeneratePreflightK8sClusterOutput performs K8s cluster preflight checks and streams output
func GeneratePreflightK8sClusterOutput(
	ctx context.Context,
	clients *kubernetes.K8sClients,
	nodeArgs []string,
	nodeSelector string,
	output *PreflightOutput,
) *PreflightK8sResult {
	result := &PreflightK8sResult{}

	output.Println("Performing preflight verification for Kubernetes cluster")
	output.Println("")

	// Resolve nodes once using cached client (used for node-scoped cluster checks: cpu policy, CNI health)
	output.Printf("🔍 Connecting to cluster and discovering nodes... ")
	nodes, err := kubernetes.ResolveNodes(ctx, clients, nodeArgs, nodeSelector)
	if err != nil {
		output.Println(utils.Red("FAILED"))
		result.Error = err
		return result
	}
	output.Println(utils.Green(fmt.Sprintf("found %d nodes", len(nodes))))

	// Filter ready nodes and count
	readyNodes := kubernetes.FilterReadyNodes(nodes)
	readyCount := len(readyNodes)
	notReadyCount := len(nodes) - readyCount

	if notReadyCount > 0 {
		output.Printf("   %s %d ready, %s %d not ready (checks will skip NotReady nodes to avoid timeouts)\n",
			utils.Green("✓"), readyCount, utils.Yellow("⚠"), notReadyCount)
	} else {
		output.Printf("   %s All %d nodes are ready\n", utils.Green("✓"), readyCount)
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
	results, err := clustercheck.GlobalClusterCheckRegistry.ValidateAll(ctx, clients, validationParams)
	if err != nil {
		result.Error = fmt.Errorf("failed to validate cluster: %w", err)
		return result
	}

	// Format and print results
	formattedResults := clustercheck.GlobalClusterCheckRegistry.FormatCheckResults(results)
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
		output.Println("\n" + utils.Yellow("=== Warnings Summary ==="))
		for _, r := range results {
			if r.Status == "warning" {
				module, _ := clustercheck.GlobalClusterCheckRegistry.Get(r.ModuleName)
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

				output.Printf("\n⚠️  %s\n", utils.Yellow(module.FriendlyName()))
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
		output.Println("\n" + utils.Red("=== Errors & Suggested Fixes ==="))
		for _, r := range results {
			if r.Status == "error" {
				module, _ := clustercheck.GlobalClusterCheckRegistry.Get(r.ModuleName)
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
				output.Printf("\n❌ %s\n", utils.Red(module.FriendlyName()))
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
