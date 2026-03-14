package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
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
	}()

	// Create output that writes to stdout in real-time
	output := NewPreflightOutput(os.Stdout)
	defer output.Close()

	// Run the preflight checks
	result := generatePreflightNodesOutput(
		ctx,
		args,
		preflightNodeSelector,
		preflightFailFast,
		preflightSummaryOnly,
		preflightFailedOnly,
		preflightWekaDirFailGB,
		preflightWekaDirWarnGB,
		output,
	)

	if result.Error != nil {
		return result.Error
	}

	if !result.Success {
		return fmt.Errorf("preflight nodes failed")
	}

	return nil
}

// generatePreflightNodesOutput performs preflight checks and streams output
func generatePreflightNodesOutput(
	ctx context.Context,
	nodeArgs []string,
	nodeSelector string,
	failFast bool,
	summaryOnly bool,
	failedOnly bool,
	wekaDirFailGB int64,
	wekaDirWarnGB int64,
	output *PreflightOutput,
) *PreflightNodesResult {
	result := &PreflightNodesResult{}

	crClient := KubeClients.CRClient
	nodes, err := resolveNodes(ctx, nodeArgs, nodeSelector)
	if err != nil {
		result.Error = err
		return result
	}

	output.Println("Performing preflight verification for Kubernetes nodes to host WEKA")
	printCheckResultToOutput(output, "Checking total number of eligible nodes...", true, fmt.Sprintf("%d", len(nodes)))

	// FIRST: Fetch pod statistics BEFORE creating host-check pods
	// (so host-check pods don't pollute the pod resource statistics)
	output.Println("Fetching pod resource usage...")
	podsByNode := GetPodsMapByNode(ctx, crClient, output)

	// SECOND: Run validation using the registry
	output.Println("Performing validation...")

	// Set up validation parameters
	validationParams := map[string]interface{}{
		"minFreeMem":       minFreeMem,
		"minFreeHP":        minFreeHP,
		"wekaDirMinFailGB": wekaDirFailGB,
		"wekaDirMinWarnGB": wekaDirWarnGB,
		"podsByNode":       podsByNode,
	}

	// Run validation for all nodes - handles caching and execution
	nodeModuleResults, err := GlobalHostCheckRegistry.ValidateAll(ctx, "preflight_nodes", nodes, validationParams)
	if err != nil {
		result.Error = fmt.Errorf("failed to validate nodes: %w", err)
		return result
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
		if !IsNodeReady(*node) {
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
			dataMap := mr.Data.Map()
			if mr.Status == "error" {
				hasError = true
				errorMsg := ""
				if mr.Error != "" {
					errorMsg = mr.Error
					issuesForNode = append(issuesForNode, fmt.Sprintf("%s: %s", moduleName, mr.Error))
				} else if mr.Data.Error() != nil {
					errorMsg = mr.Data.Error().Error()
					issuesForNode = append(issuesForNode, fmt.Sprintf("%s: %s", moduleName, mr.Data.Error()))
				}

				// Get suggested fix from module using template interpolation
				suggestedFix := ""
				if mr.SuggestedResolutionTemplate != "" {
					// Build context params for interpolation
					fixParams := map[string]interface{}{
						"NodeName": nodeName,
					}
					// Add all data from the module result
					for k, v := range dataMap {
						fixParams[k] = v
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
				if warning, ok := dataMap["Warning"].(string); ok && warning != "" {
					warningMsg = warning
				} else if issue, ok := dataMap["Issue"].(string); ok && issue != "" {
					warningMsg = issue
				} else if detail, ok := dataMap["Detail"].(string); ok && detail != "" {
					warningMsg = detail
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

		if summaryOnly {
			// In summary-only mode, print only failed nodes (header and details)
			shouldPrintHeader = hasError
			shouldPrintDetails = hasError
		} else if failedOnly {
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
			output.Printf("  %s: %s\n", nodeName, nodeStatus)
		}

		if shouldPrintDetails {
			// Print all validation results for this node
			printNodeValidationToOutput(output, nodeName, "preflight_nodes", moduleResults)

			output.Println("")
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

		if failFast && overallStatus == "failure" {
			break
		}
	}

	// Count checked nodes (not skipped)
	checked := passCnt + warnCnt + failCnt

	// Summary
	output.Println("Summary:")
	output.Printf("  Eligible nodes:      %d\n", len(nodes))
	output.Printf("  Nodes skipped:       %s\n", cyan(fmt.Sprintf("%d", skippedCnt)))
	output.Printf("  Nodes checked:       %d\n", checked)
	output.Printf("  Nodes passed:        %s\n", green(fmt.Sprintf("%d", passCnt)))
	output.Printf("  Nodes warned:        %s\n", yellow(fmt.Sprintf("%d", warnCnt)))
	output.Printf("  Nodes failed:        %s\n", red(fmt.Sprintf("%d", failCnt)))

	if skippedCnt > 0 {
		output.Printf("  Skipped nodes:       %s\n", strings.Join(skippedNodes, ", "))
	}
	if warnCnt > 0 {
		output.Printf("  Warned nodes:        %s\n", strings.Join(warnedNodes, ", "))
	}
	if failCnt > 0 {
		output.Printf("  Failed nodes:        %s\n", strings.Join(failedNodes, ", "))
	}
	output.Printf("  Unique OSes:         %d\n", len(oses))
	output.Printf("  Unique Kernels:      %d\n", len(kernels))

	if len(oses) > 1 {
		output.Printf("Warning: Multiple OSes detected: %s\n", strings.Join(mapKeysToList(oses), ", "))
	}
	if len(kernels) > 1 {
		output.Printf("Warning: Multiple kernels detected: %s\n", strings.Join(mapKeysToList(kernels), ", "))
	}

	// Print centralized warnings summary
	if len(allWarnings) > 0 {
		output.Println("\n" + yellow("=== Warnings Summary ==="))
		// Group warnings by type
		warningsByMessage := make(map[string][]string)
		for _, w := range allWarnings {
			warningsByMessage[w.Message] = append(warningsByMessage[w.Message], w.NodeName)
		}

		for msg, nodes := range warningsByMessage {
			output.Printf("\n⚠️  %s\n", yellow(msg))
			output.Printf("   Affected nodes (%d): %s\n", len(nodes), strings.Join(nodes, ", "))
		}
	}

	// Print centralized errors summary
	if len(allErrors) > 0 {
		output.Println("\n" + red("=== Errors Summary ==="))

		// Group errors by module (not by exact message)
		type ErrorGroup struct {
			Module       string
			Nodes        []string
			Messages     []string
			SuggestedFix string
		}
		errorGroups := make(map[string]*ErrorGroup)

		for _, e := range allErrors {
			// Use module name as the grouping key
			key := e.Module

			if group, exists := errorGroups[key]; exists {
				group.Nodes = append(group.Nodes, e.NodeName)
				group.Messages = append(group.Messages, e.Message)
			} else {
				errorGroups[key] = &ErrorGroup{
					Module:       e.Module,
					Nodes:        []string{e.NodeName},
					Messages:     []string{e.Message},
					SuggestedFix: e.Fix,
				}
			}
		}

		for _, group := range errorGroups {
			// Check if all messages are identical
			allSame := true
			firstMsg := group.Messages[0]
			for _, msg := range group.Messages {
				if msg != firstMsg {
					allSame = false
					break
				}
			}

			// Display the error with appropriate context
			if allSame {
				// All errors are identical - show the exact error
				output.Printf("\n❌ %s: %s\n", red(group.Module), firstMsg)
			} else {
				// Errors differ - show it's a common issue with varying details
				output.Printf("\n❌ %s: %s (values vary by node)\n", red(group.Module), firstMsg)
			}

			output.Printf("   Affected nodes (%d): %s\n", len(group.Nodes), strings.Join(group.Nodes, ", "))
			if group.SuggestedFix != "" {
				output.Printf("   💡 Suggested fix: %s\n", group.SuggestedFix)
			}
		}
	}

	// Set result values
	result.PassedCount = passCnt
	result.WarningCount = warnCnt
	result.FailedCount = failCnt
	result.SkippedCount = skippedCnt
	result.Success = failCnt == 0
	result.Output = output.GetFullOutput()

	return result
}

// printCheckResultToOutput is a helper to print check results to PreflightOutput
func printCheckResultToOutput(output *PreflightOutput, msg string, ok bool, detail string) {
	status := "✅"
	if !ok {
		status = "❌"
	}
	if detail != "" {
		output.Printf("%s %s (%s)\n", status, msg, detail)
	} else {
		output.Printf("%s %s\n", status, msg)
	}
}

// printNodeValidationToOutput prints validation results to PreflightOutput
func printNodeValidationToOutput(output *PreflightOutput, nodeName, category string, moduleResults map[string]*HostCheckModuleResult) {
	// Use the registry's FormatNodeValidationResults to get formatted output as string
	formattedOutput, _ := GlobalHostCheckRegistry.FormatNodeValidationResults(nodeName, category, moduleResults)
	// Write to our output instead of stdout
	output.Println(formattedOutput)
}
