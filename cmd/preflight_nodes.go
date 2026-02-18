package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

// AggregatedResult represents the combined results of hostcheck validation for a node
type AggregatedResult struct {
	NodeName      string
	Status        string // "success", "partial", "failure", or "skipped"
	ModuleResults map[string]*HostCheckResult
}

type checkStatus string

const (
	statusPass    checkStatus = "✅ OK"
	statusWarn    checkStatus = "⚠️ WARNING"
	statusFail    checkStatus = "❌ FAILED"
	statusSkipped checkStatus = "⏭️ SKIPPED (Node not ready)"
)

type nodeCheck struct {
	title  string
	status checkStatus
	detail string // printed in [..]
}

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
	var podsByNode map[string][]corev1.Pod
	{
		var allPods corev1.PodList
		if err := crClient.List(ctx, &allPods); err != nil {
			fmt.Printf("Warning: failed to list pods: %v\n", err)
			podsByNode = make(map[string][]corev1.Pod)
		} else {
			// Build a map of nodeName -> pods on that node
			podsByNode = make(map[string][]corev1.Pod)
			for i := range allPods.Items {
				pod := &allPods.Items[i]
				if pod.Spec.NodeName == "" {
					continue
				}
				podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], *pod)
			}
			fmt.Printf("Fetched %d pods across %d nodes\n", len(allPods.Items), len(podsByNode))
		}
	}

	// SECOND: Run host checks using the generic RunHostChecks function
	opts := HostCheckOptions{
		Verbose:             true,
		CleanupInBackground: true, // Do not wait for cleanup in preflight
		Timeout:             3 * time.Minute,
	}

	fmt.Println("Performing host checks...")
	hostFacts, err := RunHostChecks(ctx, nodes, opts)
	if err != nil {
		return fmt.Errorf("failed to run hostchecks: %w", err)
	}

	// Build a map for quick node lookup
	nodeMap := make(map[string]*corev1.Node, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].Name] = &nodes[i]
	}

	// Create module instances to get their templates
	osModule := &OSModule{}
	xfsModule := &XFSModule{}
	wekaClientModule := &WekaClientModule{}
	networkModule := &NetworkModule{}

	fmt.Println("Processing node results...")
	receivedCount := 0

	// Map to store aggregated results per node
	nodeResults := make(map[string]*AggregatedResult, len(nodes))

	// Process all nodes with hostcheck results
	for nodeName, hf := range hostFacts {
		receivedCount++

		// Get node from map
		node, exists := nodeMap[nodeName]
		if !exists {
			continue
		}

		// Skip health check processing for non-ready nodes - they will be marked as skipped later
		if !isNodeReady(node) {
			// Store a minimal result indicating the node was skipped
			nodeResults[nodeName] = &AggregatedResult{
				NodeName:      nodeName,
				Status:        "skipped",
				ModuleResults: make(map[string]*HostCheckResult),
			}
			continue
		}

		moduleResults := make(map[string]*HostCheckResult)

		// OS check
		osImage := node.Status.NodeInfo.OSImage
		osOK := strings.Contains(strings.ToLower(osImage), "ubuntu")
		moduleResults["os"] = &HostCheckResult{
			ModuleName:      "os",
			Status:          statusToModuleStatus(passOrFail(osOK)),
			SuccessTemplate: osModule.SuccessTemplate(),
			WarningTemplate: osModule.WarningTemplate(),
			ErrorTemplate:   osModule.ErrorTemplate(),
			SuggestedFix:    osImage,
		}

		// Kernel check
		moduleResults["kernel"] = &HostCheckResult{
			ModuleName:   "kernel",
			Status:       "success",
			SuggestedFix: node.Status.NodeInfo.KernelVersion,
		}

		// Hugepages check
		hpSet := quantityOrZero(node.Status.Capacity, "hugepages-2Mi")
		hpAlloc := quantityOrZero(node.Status.Allocatable, "hugepages-2Mi")
		hpOK := !(hpSet.IsZero() && hpAlloc.IsZero())
		moduleResults["hugepages"] = &HostCheckResult{
			ModuleName:   "hugepages",
			Status:       statusToModuleStatus(passOrFail(hpOK)),
			SuggestedFix: fmt.Sprintf("set=%s allocatable=%s", hpSet.String(), hpAlloc.String()),
		}

		// Free memory and hugepages
		memFree, hpFree, warn := computeFreeFromPods(node, hpAlloc, podsByNode[node.Name])
		memOK := memFree.Cmp(minFreeMem) >= 0
		moduleResults["mem_free"] = &HostCheckResult{
			ModuleName:   "mem_free",
			Status:       statusToModuleStatus(passOrFail(memOK)),
			SuggestedFix: fmt.Sprintf("%s free (min %s)%s", memFree.String(), minFreeMem.String(), warnSuffix(warn)),
		}

		hpFreeOK := hpFree.Cmp(minFreeHP) >= 0
		moduleResults["hugepages_free"] = &HostCheckResult{
			ModuleName:   "hugepages_free",
			Status:       statusToModuleStatus(passOrFail(hpFreeOK)),
			SuggestedFix: fmt.Sprintf("%s free (min %s)%s", hpFree.String(), minFreeHP.String(), warnSuffix(warn)),
		}

		// Weka directory check
		minFailWeka := preflightWekaDirFailGB * 1000 * 1000 * 1000
		minWarnWeka := preflightWekaDirWarnGB * 1000 * 1000 * 1000
		wakaDirStatus := statusPass
		wakaDirDetail := ""

		if hf.WekaDirDetail == "directory does not exist" {
			wakaDirStatus = statusFail
			wakaDirDetail = "directory does not exist"
		} else if hf.WekaDirAvailBytes < minFailWeka {
			wakaDirStatus = statusFail
			availGB := float64(hf.WekaDirAvailBytes) / (1000 * 1000 * 1000)
			wakaDirDetail = fmt.Sprintf("%.1fGB available (min: %dGB)", availGB, preflightWekaDirFailGB)
		} else if hf.WekaDirAvailBytes < minWarnWeka {
			wakaDirStatus = statusWarn
			availGB := float64(hf.WekaDirAvailBytes) / (1000 * 1000 * 1000)
			wakaDirDetail = fmt.Sprintf("%.1fGB available (min: %dGB)", availGB, preflightWekaDirWarnGB)
		} else {
			wakaDirStatus = statusPass
			availGB := float64(hf.WekaDirAvailBytes) / (1000 * 1000 * 1000)
			wakaDirDetail = fmt.Sprintf("%s: %.1fGB available", nonEmpty(hf.WekaDirPath, "(unknown)"), availGB)
		}

		moduleResults["weka_dir"] = &HostCheckResult{
			ModuleName:      "weka_dir",
			Status:          statusToModuleStatus(wakaDirStatus),
			SuccessTemplate: "✅ OK:  {{.FriendlyName}}: {{.Detail}}",
			WarningTemplate: "⚠️ WARN: {{.FriendlyName}}: {{.Detail}}",
			ErrorTemplate:   "❌ ERROR: {{.FriendlyName}}: {{.Detail}}",
			SuggestedFix:    wakaDirDetail,
		}

		// XFS check
		moduleResults["xfs"] = &HostCheckResult{
			ModuleName:      "xfs",
			Status:          statusToModuleStatus(passOrFail(hf.XFSInstalled)),
			SuccessTemplate: xfsModule.SuccessTemplate(),
			WarningTemplate: xfsModule.WarningTemplate(),
			ErrorTemplate:   xfsModule.ErrorTemplate(),
			SuggestedFix:    nonEmpty(hf.XFSDetail, "no details"),
		}

		// Weka client check
		moduleResults["weka_client"] = &HostCheckResult{
			ModuleName:      "weka_client",
			Status:          statusToModuleStatus(passOrFail(hf.WekaClientClean)),
			SuccessTemplate: wekaClientModule.SuccessTemplate(),
			WarningTemplate: wekaClientModule.WarningTemplate(),
			ErrorTemplate:   wekaClientModule.ErrorTemplate(),
			SuggestedFix:    nonEmpty(hf.WekaClientDetail, "no details"),
		}

		// Mellanox NIC check
		var mellanoxDetail string
		var mellanoxStatus checkStatus
		if len(hf.MlxIfaces) == 0 {
			mellanoxStatus = statusWarn
			mellanoxDetail = "No Mellanox NIC detected; only UDP mode is supported"
		} else {
			mellanoxStatus = statusPass
			var lines []string
			lines = append(lines, "")
			for _, nic := range hf.MlxIfaces {
				spd := nic.Speed
				if strings.TrimSpace(spd) == "" {
					spd = "unknown"
				}
				if nic.Bond != "" {
					lines = append(lines, fmt.Sprintf("• %s [*%s] %s %s", nic.Name, nic.Bond, spd, nic.Model))
				} else {
					ip := nic.IP
					if strings.TrimSpace(ip) == "" {
						ip = "-"
					} else {
						lines = append(lines, fmt.Sprintf("• %s [%s] %s %s", nic.Name, ip, spd, nic.Model))
					}
				}
			}
			for _, b := range hf.MlxBonds {
				ip := b.IP
				if strings.TrimSpace(ip) == "" {
					ip = "-"
				}
				spd := b.Speed
				if strings.TrimSpace(spd) == "" {
					spd = "unknown"
				}
				lines = append(lines, fmt.Sprintf("• %s [%s] %s (%s)", b.Name, ip, spd, strings.Join(b.Slaves, ", ")))
			}
			mellanoxDetail = strings.Join(lines, "\n")
		}

		moduleResults["mellanox_nic"] = &HostCheckResult{
			ModuleName:      "mellanox_nic",
			Status:          statusToModuleStatus(mellanoxStatus),
			SuccessTemplate: networkModule.SuccessTemplate(),
			WarningTemplate: networkModule.WarningTemplate(),
			ErrorTemplate:   networkModule.ErrorTemplate(),
			SuggestedFix:    mellanoxDetail,
		}

		// Bond LACP check
		moduleResults["bond_lacp"] = &HostCheckResult{
			ModuleName:   "bond_lacp",
			Status:       statusToModuleStatus(passOrFail(hf.BondLACPOk)),
			SuggestedFix: nonEmpty(hf.BondLACPDetail, "no details"),
		}

		// Determine overall status
		overallStatus := "success"
		for _, mr := range moduleResults {
			if mr.Status == "error" {
				overallStatus = "failure"
				break
			}
			if mr.Status == "warning" && overallStatus == "success" {
				overallStatus = "partial"
			}
		}

		nodeResults[nodeName] = &AggregatedResult{
			NodeName:      nodeName,
			Status:        overallStatus,
			ModuleResults: moduleResults,
		}

		if receivedCount%10 == 0 || receivedCount == len(hostFacts) {
			fmt.Printf("Processed %d/%d results...\n", receivedCount, len(hostFacts))
		}
	}
	fmt.Printf("All %d results processed!\n", receivedCount)

	// Process all nodes while pod fetch happens in background
	checked := 0
	passCnt := 0
	warnCnt := 0
	failCnt := 0
	skippedCnt := 0

	var warnedNodes []string
	var failedNodes []string
	var skippedNodes []string
	kernels := make(map[string]struct{})
	oses := make(map[string]struct{})

	// Process all nodes
	for i := range nodes {
		n := &nodes[i]
		aggResult, ok := nodeResults[n.Name]
		if !ok {
			continue // node result wasn't received
		}

		checked++

		var nodeStatus checkStatus
		// Determine node status from aggregated result
		switch aggResult.Status {
		case "success":
			nodeStatus = statusPass
		case "partial":
			nodeStatus = statusWarn
		case "skipped":
			nodeStatus = statusSkipped
		default:
			nodeStatus = statusFail
		}

		// Only collect kernel/OS info for ready nodes
		if nodeStatus != statusSkipped {
			hf := hostFacts[n.Name]
			kernels[n.Status.NodeInfo.KernelVersion] = struct{}{}
			oses[hf.OSRelease] = struct{}{}
		}

		switch nodeStatus {
		case statusPass:
			passCnt++
		case statusWarn:
			warnCnt++
			warnedNodes = append(warnedNodes, n.Name)
		case statusSkipped:
			skippedCnt++
			skippedNodes = append(skippedNodes, n.Name)
		default:
			failCnt++
			failedNodes = append(failedNodes, n.Name)
		}

		// Skip printing completely PASS nodes if --failed-only is set
		if preflightFailedOnly && nodeStatus == statusPass {
			continue
		}

		printNodeHeader(n.Name, nodeStatus)

		// Print module results using Summary() format
		// Define the order of checks to display
		checkOrder := []string{"os", "kernel", "hugepages", "mem_free", "hugepages_free", "weka_dir", "xfs", "weka_client", "mellanox_nic", "bond_lacp"}

		for _, moduleName := range checkOrder {
			moduleResult, exists := aggResult.ModuleResults[moduleName]
			if !exists {
				continue
			}

			if !preflightSummaryOnly || moduleResult.Status != "success" {
				// Map module name to friendly name
				var friendlyName string
				switch moduleName {
				case "os":
					friendlyName = "Operating System"
				case "kernel":
					friendlyName = "Kernel"
				case "hugepages":
					friendlyName = "Hugepages"
				case "mem_free":
					friendlyName = "Free Memory"
				case "hugepages_free":
					friendlyName = "Free Hugepages"
				case "weka_dir":
					friendlyName = "Weka Directory"
				case "xfs":
					friendlyName = "XFS Tools"
				case "weka_client":
					friendlyName = "Weka Client"
				case "mellanox_nic":
					friendlyName = "Network Configuration"
				case "bond_lacp":
					friendlyName = "Bond LACP"
				default:
					friendlyName = moduleName
				}

				// Create context params for Summary()
				contextParams := map[string]interface{}{
					"FriendlyName": friendlyName,
					"Detail":       moduleResult.SuggestedFix,
					"NodeName":     n.Name,
				}

				// Use Summary() if templates are available, otherwise format manually
				var displayText string
				if moduleResult.SuccessTemplate != "" || moduleResult.WarningTemplate != "" || moduleResult.ErrorTemplate != "" {
					displayText = moduleResult.Summary(contextParams)
				} else {
					// Fallback for modules without templates
					switch moduleResult.Status {
					case "success":
						displayText = fmt.Sprintf("✅ OK:  %s: %s", friendlyName, moduleResult.SuggestedFix)
					case "warning":
						displayText = fmt.Sprintf("⚠️  WARN: %s: %s", friendlyName, moduleResult.SuggestedFix)
					default:
						displayText = fmt.Sprintf("❌ ERROR: %s: %s", friendlyName, moduleResult.SuggestedFix)
					}
				}

				// Handle multiline details (like mellanox_nic)
				if strings.Contains(displayText, "\n") {
					lines := strings.Split(displayText, "\n")
					fmt.Printf("     %s\n", lines[0])
					// Indent subsequent lines to align with friendly name + 2 spaces
					// "     ✅ OK:  " = 13 characters (5 spaces + emoji ~2 chars + " OK:  " 6 chars)
					// So indent is 13 + 2 = 15 spaces to align with "Network Configuration" + 2
					indent := "               "
					for i := 1; i < len(lines); i++ {
						fmt.Printf("%s%s\n", indent, lines[i])
					}
				} else {
					fmt.Printf("     %s\n", displayText)
				}
			}
		}
		fmt.Println()

		if preflightFailFast && nodeStatus == statusFail {
			break
		}
	}

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
		fmt.Printf("Warning: Multiple OSes detected: %s\n", strings.Join(mapKeysToList(kernels), ", "))
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

// Helper functions for module status conversion
func passOrFail(ok bool) checkStatus {
	if ok {
		return statusPass
	}
	return statusFail
}

// Helper functions for status conversion
func statusToModuleStatus(st checkStatus) string {
	switch st {
	case statusPass:
		return "success"
	case statusWarn:
		return "warning"
	case statusFail:
		return "error"
	default:
		return "error"
	}
}

func warnSuffix(w string) string {
	w = strings.TrimSpace(w)
	if w == "" {
		return ""
	}
	return "; warn=" + w
}

func nonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func statusString(s checkStatus) string {
	switch s {
	case statusPass:
		return green("PASS")
	case statusWarn:
		return yellow("WARN")
	case statusSkipped:
		return cyan("SKIPPED")
	default:
		return red("FAIL")
	}
}

func printNodeHeader(name string, st checkStatus) {
	// For skipped nodes, print a clear reason
	fmt.Printf("  %s: %s\n", name, statusString(st))
}

func printNodeSubcheck(c nodeCheck) {
	statusStr := statusString(c.status)

	// Base prefix up to '['
	prefix := fmt.Sprintf("     %s: %s [", c.title, statusStr)

	// If single-line, keep old behavior
	if !strings.Contains(c.detail, "\n") {
		fmt.Printf("%s%s]\n", prefix, c.detail)
		return
	}

	// Multiline: first line after '[' then subsequent lines aligned
	parts := strings.Split(c.detail, "\n")
	fmt.Printf("%s\n", prefix)

	// indent to align under the first detail char (same length as prefix)
	indent := strings.Repeat(" ", len(prefix)-9)
	for i := 0; i < len(parts); i++ {
		fmt.Printf("%s%s\n", indent, parts[i])
	}
	// close bracket on last line
	fmt.Printf("%s]\n", strings.Repeat(" ", len(prefix)-10))
}

func isNodeReady(n *corev1.Node) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
