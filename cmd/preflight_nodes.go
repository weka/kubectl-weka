package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	minFreeMem = resource.MustParse("4Gi")
	minFreeHP  = resource.MustParse("3Gi")

	preflightFailFast      bool
	preflightSummaryOnly   bool
	preflightFailedOnly    bool
	preflightWekaDirFailGB int64
	preflightWekaDirWarnGB int64
)

var preflightNodesCmd = &cobra.Command{
	Use:   "nodes [NODE...]",
	Short: "Preflight node checks (OS, hugepages, free resources, host readiness)",
	Args:  cobra.ArbitraryArgs,
	RunE:  runPreflightNodes,
}

func init() {
	preflightCmd.AddCommand(preflightNodesCmd)
	preflightNodesCmd.Flags().StringVar(&preflightNodeSelector, "node-selector", "", "Label selector to filter nodes, e.g. if only part of nodes are targeted for WEKA")
	preflightNodesCmd.Flags().BoolVar(&preflightFailFast, "fail-fast", false, "Stop on first failed node")
	preflightNodesCmd.Flags().BoolVar(&preflightSummaryOnly, "summary-only", false, "Only print summary (no per-node details)")
	preflightNodesCmd.Flags().BoolVar(&preflightFailedOnly, "failed-only", false, "Only show failed nodes")
	preflightNodesCmd.Flags().Int64Var(&preflightWekaDirFailGB, "weka-dir-min-fail", 100, "Minimum GB for weka directory (FAIL if below, default 100)")
	preflightNodesCmd.Flags().Int64Var(&preflightWekaDirWarnGB, "weka-dir-min-warn", 300, "Minimum GB for weka directory (WARN if below, default 300)")
	preflightNodesCmd.SilenceUsage = true

}

// checkStatus represents the overall status of node checks
type checkStatus string

const (
	statusPass    checkStatus = "✅ OK"
	statusWarn    checkStatus = "⚠️ WARNING"
	statusFail    checkStatus = "❌ FAILED"
	statusSkipped checkStatus = "⏭️ SKIPPED (Node not ready)"
)

func runPreflightNodes(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Setup signal handling for graceful shutdown (cleanup pods on Ctrl-C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle signals in background
	go func() {
		sig := <-sigChan
		fmt.Printf("\n\nReceived signal %v, cleaning up pods...\n", sig)
		cancel() // Cancel context to stop operations
		// ...existing code...
	}()

	crClient := KubeClients.CRClient
	nodes, err := resolveNodes(ctx, crClient, args, preflightNodeSelector)
	if err != nil {
		return err
	}

	fmt.Println("Performing preflight verification for Kubernetes nodes to host WEKA")
	printCheckResult("Checking total number of eligible nodes...", true, fmt.Sprintf("%d", len(nodes)))

	// FIRST: Fetch pod statistics BEFORE creating host-check pods
	// (so host-check pods don't pollute the pod resource statistics)
	fmt.Println("Fetching pod resource usage...")
	podsByNode := GetPodsMapByNode(ctx, crClient)

	// SECOND: Run validation using the registry
	fmt.Println("Performing validation...")

	// Set up validation parameters
	validationParams := map[string]interface{}{
		"minFreeMem":       minFreeMem,
		"minFreeHP":        minFreeHP,
		"wekaDirMinFailGB": preflightWekaDirFailGB,
		"wekaDirMinWarnGB": preflightWekaDirWarnGB,
		"podsByNode":       podsByNode,
	}

	// Run validation for all nodes - handles caching and execution
	nodeModuleResults, err := GlobalHostCheckRegistry.ValidateAll(ctx, "preflight_nodes", nodes, validationParams)
	if err != nil {
		return fmt.Errorf("failed to validate nodes: %w", err)
	}

	// Track results
	passCnt := 0
	warnCnt := 0
	failCnt := 0
	skippedCnt := 0

	var warnedNodes []string
	var failedNodes []string
	var skippedNodes []string
	kernels := make(map[string]struct{})
	oses := make(map[string]struct{})

	// First, mark NotReady nodes as skipped and collect info
	for i := range nodes {
		node := &nodes[i]
		if !isNodeReady(node) {
			skippedCnt++
			skippedNodes = append(skippedNodes, node.Name)
		}
	}

	// Then process the validation results from ready nodes
	// First, get hostchecks to collect OS/kernel info
	hostChecksMap, _ := GlobalHostCheckRegistry.GetHostChecksForNodes(ctx, nodes)

	// Collect warnings, errors, and suggested fixes for summary
	type SuggestedFix struct {
		NodeName string
		Issues   []string
	}
	type NodeWarning struct {
		NodeName string
		Module   string
		Message  string
	}
	type NodeError struct {
		NodeName string
		Module   string
		Message  string
		Fix      string
	}

	var suggestedFixes []SuggestedFix
	var allWarnings []NodeWarning
	var allErrors []NodeError

	for nodeName, moduleResults := range nodeModuleResults {
		// Determine overall status
		hasWarning := false
		hasError := false
		var issuesForNode []string

		for moduleName, mr := range moduleResults {
			if mr.Status == "error" {
				hasError = true
				errorMsg := ""
				if mr.Error != "" {
					errorMsg = mr.Error
					issuesForNode = append(issuesForNode, fmt.Sprintf("%s: %s", moduleName, mr.Error))
				} else if dataMap, ok := mr.Data.(map[string]interface{}); ok {
					if issue, ok := dataMap["Issue"].(string); ok && issue != "" {
						errorMsg = issue
						issuesForNode = append(issuesForNode, fmt.Sprintf("%s: %s", moduleName, issue))
					}
				}

				// Get suggested fix from module using template interpolation
				suggestedFix := ""
				if mr.SuggestedResolutionTemplate != "" {
					// Build context params for interpolation
					fixParams := map[string]interface{}{
						"NodeName": nodeName,
					}
					// Add all data from the module result
					if dataMap, ok := mr.Data.(map[string]interface{}); ok {
						for k, v := range dataMap {
							fixParams[k] = v
						}
					}
					suggestedFix = mr.FormatSuggestedFix(fixParams)
				}

				allErrors = append(allErrors, NodeError{
					NodeName: nodeName,
					Module:   moduleName,
					Message:  errorMsg,
					Fix:      suggestedFix,
				})
			} else if mr.Status == "warning" {
				hasWarning = true

				// Extract warning message from module data
				warningMsg := ""
				if dataMap, ok := mr.Data.(map[string]interface{}); ok {
					if warning, ok := dataMap["Warning"].(string); ok && warning != "" {
						warningMsg = warning
					} else if issue, ok := dataMap["Issue"].(string); ok && issue != "" {
						warningMsg = issue
					} else if detail, ok := dataMap["Detail"].(string); ok && detail != "" {
						warningMsg = detail
					}
				}

				if warningMsg != "" {
					allWarnings = append(allWarnings, NodeWarning{
						NodeName: nodeName,
						Module:   moduleName,
						Message:  warningMsg,
					})
				}
			}
		}

		var overallStatus string
		var nodeStatus checkStatus
		if hasError {
			overallStatus = "failure"
			nodeStatus = statusFail
		} else if hasWarning {
			overallStatus = "partial"
			nodeStatus = statusWarn
		} else {
			overallStatus = "success"
			nodeStatus = statusPass
		}

		// Track suggested fixes for failed nodes
		if hasError && len(issuesForNode) > 0 {
			suggestedFixes = append(suggestedFixes, SuggestedFix{
				NodeName: nodeName,
				Issues:   issuesForNode,
			})
		}

		// Determine what to print based on flags and status
		shouldPrintHeader := false
		shouldPrintDetails := false

		if preflightSummaryOnly {
			// In summary-only mode, print only failed nodes (header and details)
			shouldPrintHeader = hasError
			shouldPrintDetails = hasError
		} else if preflightFailedOnly {
			// In failed-only mode, print failed nodes with details, skip passed nodes
			if hasError {
				shouldPrintHeader = true
				shouldPrintDetails = true
			}
		} else {
			// Default: print all nodes with all their validation results
			shouldPrintHeader = true
			shouldPrintDetails = true
		}

		if shouldPrintHeader {
			// Print node header with status
			fmt.Printf("  %s: %s\n", nodeName, nodeStatus)
		}

		if shouldPrintDetails {
			// Print all validation results for this node
			GlobalHostCheckRegistry.PrintNodeValidationResults(nodeName, "preflight_nodes", moduleResults)
			fmt.Println()
		}

		// Update counters
		switch overallStatus {
		case "success":
			passCnt++
		case "partial":
			warnCnt++
			warnedNodes = append(warnedNodes, nodeName)
		case "failure":
			failCnt++
			failedNodes = append(failedNodes, nodeName)
		}

		// Collect OS/kernel info
		if hf, exists := hostChecksMap[nodeName]; exists {
			kernels[hf.KernelVersion] = struct{}{}
			oses[hf.OSRelease] = struct{}{}
		}

		if preflightFailFast && overallStatus == "failure" {
			break
		}
	}

	// Count checked nodes (not skipped)
	checked := passCnt + warnCnt + failCnt

	// Summary
	fmt.Println("Summary:")
	fmt.Printf("  Eligible nodes:      %d\n", len(nodes))
	fmt.Printf("  Nodes skipped:       %s\n", cyan(fmt.Sprintf("%d", skippedCnt)))
	fmt.Printf("  Nodes checked:       %d\n", checked)
	fmt.Printf("  Nodes passed:        %s\n", green(fmt.Sprintf("%d", passCnt)))
	fmt.Printf("  Nodes warned:        %s\n", yellow(fmt.Sprintf("%d", warnCnt)))
	fmt.Printf("  Nodes failed:        %s\n", red(fmt.Sprintf("%d", failCnt)))

	if skippedCnt > 0 {
		fmt.Printf("  Skipped nodes:       %s\n", strings.Join(skippedNodes, ", "))
	}
	if warnCnt > 0 {
		fmt.Printf("  Warned nodes:        %s\n", strings.Join(warnedNodes, ", "))
	}
	if failCnt > 0 {
		fmt.Printf("  Failed nodes:        %s\n", strings.Join(failedNodes, ", "))
	}
	fmt.Printf("  Unique OSes:         %d\n", len(oses))
	fmt.Printf("  Unique Kernels:      %d\n", len(kernels))

	if len(oses) > 1 {
		fmt.Printf("Warning: Multiple OSes detected: %s\n", strings.Join(mapKeysToList(oses), ", "))
	}
	if len(kernels) > 1 {
		fmt.Printf("Warning: Multiple kernels detected: %s\n", strings.Join(mapKeysToList(kernels), ", "))
	}

	// Print centralized warnings summary
	if len(allWarnings) > 0 {
		fmt.Println("\n" + yellow("=== Warnings Summary ==="))
		// Group warnings by type
		warningsByMessage := make(map[string][]string)
		for _, w := range allWarnings {
			warningsByMessage[w.Message] = append(warningsByMessage[w.Message], w.NodeName)
		}

		for msg, nodes := range warningsByMessage {
			fmt.Printf("\n⚠️  %s\n", yellow(msg))
			fmt.Printf("   Affected nodes (%d): %s\n", len(nodes), strings.Join(nodes, ", "))
		}
	}

	// Print centralized errors summary
	if len(allErrors) > 0 {
		fmt.Println("\n" + red("=== Errors Summary ==="))
		// Group errors by type
		errorsByMessage := make(map[string][]string)
		errorFixes := make(map[string]string)
		for _, e := range allErrors {
			key := fmt.Sprintf("%s: %s", e.Module, e.Message)
			errorsByMessage[key] = append(errorsByMessage[key], e.NodeName)
			if e.Fix != "" {
				errorFixes[key] = e.Fix
			}
		}

		for msg, nodes := range errorsByMessage {
			fmt.Printf("\n❌ %s\n", red(msg))
			fmt.Printf("   Affected nodes (%d): %s\n", len(nodes), strings.Join(nodes, ", "))
			if fix, hasFix := errorFixes[msg]; hasFix {
				fmt.Printf("   💡 Suggested fix: %s\n", fix)
			}
		}
	}

	// WARN does not fail preflight, FAIL does.
	if failCnt > 0 {
		return fmt.Errorf("preflight nodes failed")
	}

	return nil
}

func mapKeysToList(m map[string]struct{}) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
