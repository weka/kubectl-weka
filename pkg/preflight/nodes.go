package preflight

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/hostcheck"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/types"
	"github.com/weka/kubectl-weka/pkg/utils"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

var minFreeMem = resource.MustParse("4Gi")

var minFreeHP = resource.MustParse("3Gi")

// GeneratePreflightNodesOutput performs preflight checks and streams output
func GeneratePreflightNodesOutput(
	ctx context.Context,
	clients *kubernetes.K8sClients,
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

	crClient := clients.CRClient
	nodes, err := kubernetes.ResolveNodes(ctx, clients, nodeArgs, nodeSelector)
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
	nodeModuleResults, err := hostcheck.GlobalHostCheckRegistry.ValidateAll(ctx, clients, "preflight_nodes", nodes, validationParams)
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
		if !kubernetes.IsNodeReady(*node) {
			skippedCnt++
			skippedNodes = append(skippedNodes, node.Name)
		}
	}

	// Then process the validation results from ready nodes
	// First, get hostchecks to collect OS/kernel info
	hostChecksMap, _ := hostcheck.GlobalHostCheckRegistry.GetHostChecksForNodes(ctx, clients, nodes)

	// Collect warnings, errors, and suggested fixes for summary
	type SuggestedFix struct {
		NodeName string
		Issues   []string
	}
	type NodeWarning struct {
		NodeName string
		Module   hostcheck.ModuleName
		Message  string
	}
	type NodeError struct {
		NodeName string
		Module   hostcheck.ModuleName
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
		var nodeStatus types.CheckStatus
		if hasError {
			overallStatus = "failure"
			nodeStatus = types.StatusFail
		} else if hasWarning {
			overallStatus = "partial"
			nodeStatus = types.StatusWarn
		} else {
			overallStatus = "success"
			nodeStatus = types.StatusPass
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
	output.Printf("  Nodes skipped:       %s\n", utils.Cyan(fmt.Sprintf("%d", skippedCnt)))
	output.Printf("  Nodes checked:       %d\n", checked)
	output.Printf("  Nodes passed:        %s\n", utils.Green(fmt.Sprintf("%d", passCnt)))
	output.Printf("  Nodes warned:        %s\n", utils.Yellow(fmt.Sprintf("%d", warnCnt)))
	output.Printf("  Nodes failed:        %s\n", utils.Red(fmt.Sprintf("%d", failCnt)))

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
		output.Printf("Warning: Multiple OSes detected: %s\n", strings.Join(utils.MapKeysToList(oses), ", "))
	}
	if len(kernels) > 1 {
		output.Printf("Warning: Multiple kernels detected: %s\n", strings.Join(utils.MapKeysToList(kernels), ", "))
	}

	// Print centralized warnings summary
	if len(allWarnings) > 0 {
		output.Println("\n" + utils.Yellow("=== Warnings Summary ==="))
		// Group warnings by type
		warningsByMessage := make(map[string][]string)
		for _, w := range allWarnings {
			warningsByMessage[w.Message] = append(warningsByMessage[w.Message], w.NodeName)
		}

		for msg, nodes := range warningsByMessage {
			output.Printf("\n⚠️  %s\n", utils.Yellow(msg))
			output.Printf("   Affected nodes (%d): %s\n", len(nodes), strings.Join(nodes, ", "))
		}
	}

	// Print centralized errors summary
	if len(allErrors) > 0 {
		output.Println("\n" + utils.Red("=== Errors Summary ==="))

		// Group errors by module (not by exact message)
		type ErrorGroup struct {
			Module       hostcheck.ModuleName
			Nodes        []string
			Messages     []string
			SuggestedFix string
		}
		errorGroups := make(map[hostcheck.ModuleName]*ErrorGroup)

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
				output.Printf("\n❌ %s: %s\n", utils.Red(string(group.Module)), firstMsg)
			} else {
				// Errors differ - show it's a common issue with varying details
				output.Printf("\n❌ %s: %s (values vary by node)\n", utils.Red(string(group.Module)), firstMsg)
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
func printNodeValidationToOutput(output *PreflightOutput, nodeName, category string, moduleResults map[hostcheck.ModuleName]*hostcheck.HostCheckModuleResult) {
	// Use the registry's FormatNodeValidationResults to get formatted output as string
	formattedOutput, _ := hostcheck.GlobalHostCheckRegistry.FormatNodeValidationResults(nodeName, category, moduleResults)
	// Write to our output instead of stdout
	output.Println(formattedOutput)
}
