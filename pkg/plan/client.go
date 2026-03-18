package plan

import (
	"context"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/weka/kubectl-weka/pkg/hostcheck"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/preflight"
	"github.com/weka/kubectl-weka/pkg/wekaconfig"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
	"os"
	"strings"
)

func ValidateAndPlanClient(ctx context.Context, clients *kubernetes.K8sClients, client *v1alpha1.WekaClient) error {
	fmt.Printf("=== Planning WekaClient Deployment: %s ===\n\n", client.Name)

	// Print client specification
	fmt.Println("=== Client Specification ===")
	printClientSpec(client)

	// Validate client configuration using modular validation system
	fmt.Println("\n=== Validating Client Configuration ===")
	validationCtx := &wekaconfig.WekaConfigContext{
		Client: client,
	}

	results, err := wekaconfig.GlobalWekaConfigValidationRegistry.ValidateAll(ctx, clients, validationCtx)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Print validation results
	wekaconfig.GlobalWekaConfigValidationRegistry.PrintValidationResults(results)

	// Check for errors or warnings
	hasErrors := false
	hasWarnings := false
	for _, result := range results {
		if result.Status == "error" {
			hasErrors = true
		} else if result.Status == "warning" {
			hasWarnings = true
		}
	}

	if hasErrors {
		fmt.Println("\n❌ Configuration validation failed")
		return fmt.Errorf("client configuration has errors")
	}

	if !hasWarnings {
		fmt.Println("\n✅ Configuration valid")
	}

	// Get cluster nodes
	nodes, err := kubernetes.GetClusterNodes(ctx, clients.CRClient)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Could not access cluster nodes: %v\n", err)
		_, _ = fmt.Fprintf(os.Stderr, "Continuing with planning without node validation...\n\n")
		nodes = nil
	}

	// Calculate container requirements
	fmt.Println("\n=== Container Resource Requirements ===")
	containerReqs := calculateClientContainerRequirements(client)
	nodeCount := getClientInstanceCount(client, nodes)
	printClientContainerRequirements(containerReqs, nodeCount)

	// Calculate and print node requirements
	clientContainers := []ContainerRequirements{
		{
			Type:      "client",
			Count:     getClientInstanceCount(client, nodes),
			Hugepages: containerReqs.Hugepages,
			Cores:     containerReqs.Cores,
			CoresNoHT: containerReqs.CoresNoHT,
			Memory:    containerReqs.Memory / (1024 * 1024), // Convert bytes to MiB
		},
	}
	clientNodeReqs := CalculateNodeRequirements(nil, clientContainers)
	PrintNodeRequirements(clientNodeReqs)

	// If nodes are not available, skip allocation simulation
	if nodes == nil || len(nodes) == 0 {
		fmt.Println("\n⚠️ Cluster nodes not available - skipping allocation simulation")
		return nil
	}

	fmt.Println("\n=== Fetching Cluster Resource Information ===")

	// Collect pod data from cluster
	podsByNode := preflight.GetPodsMapByNode(ctx, clients.CRClient, nil)
	fmt.Printf("✅ Collected pod data from cluster\n")

	// Find matching nodes
	fmt.Println("\n=== Finding Matching Nodes ===")
	allMatchingNodes := kubernetes.FilterNodesBySelector(nodes, client.Spec.NodeSelector)
	matchingNodes := kubernetes.FilterReadyNodes(allMatchingNodes)
	notReadyMatchingCount := len(allMatchingNodes) - len(matchingNodes)

	fmt.Printf("Found %d nodes matching selector: %s\n", len(matchingNodes), SelectorToString(client.Spec.NodeSelector))

	// Warn about NotReady nodes
	if notReadyMatchingCount > 0 {
		fmt.Printf("\n⚠️ WARNING: Additional %d node(s) match the selector but are in NotReady state.\n", notReadyMatchingCount)
		fmt.Println("   These nodes will not be checked for compliancy.")
	}

	if len(matchingNodes) == 0 {
		if notReadyMatchingCount > 0 {
			fmt.Println("⚠️ WARNING: No ready nodes match the nodeSelector (all matching nodes are NotReady)")
		} else {
			fmt.Println("⚠️ WARNING: No nodes match the nodeSelector")
		}
		return nil
	}

	// Print matching nodes table
	printClientNodesTable(matchingNodes, podsByNode)

	// Validate network interfaces
	fmt.Println("\n=== Validating Network Interfaces ===")
	if networkValidationErrors := validateNetworkInterfacesForClient(clients, client, matchingNodes); networkValidationErrors {
		return fmt.Errorf("network interface validation failed")
	}

	// Simulate allocation
	fmt.Println("\n=== Simulating Container Placement ===")
	allocations := simulateClientAllocation(matchingNodes, podsByNode, containerReqs)

	// Convert to NodePlacement format for reusing existing visualization
	placements := convertClientAllocationsToNodePlacements(allocations)

	// Print placement details with resource allocation (reuses cluster function)
	fmt.Println("\n=== Placement Details & Resource Allocation ===")
	// Note: Clients don't allocate drives, so pass nil for hostChecksMap
	printPlacementDetailsWithResourceAllocation(placements, matchingNodes, podsByNode, nil)

	// Check if allocation is feasible
	fmt.Println("\n=== Allocation Summary ===")
	printClientAllocationFeasibility(allocations)

	// Final summary with NotReady node warning if applicable
	if notReadyMatchingCount > 0 {
		fmt.Println("\n⚠️ WARNING: Plan completed with warnings")
		fmt.Printf("   ⚠️ %d node(s) were not ready during planning and were skipped\n", notReadyMatchingCount)
		fmt.Println("   Please notice that required validations were not performed on these nodes.")
		fmt.Println("   Recommended to remediate the nodes and rerun plan.")
	} else {
		fmt.Println("\n✅ Plan complete!")
	}

	return nil
}

func printClientSpec(client *v1alpha1.WekaClient) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"PROPERTY", "VALUE"})

	t.AppendRow(table.Row{"Name", client.Name})
	t.AppendRow(table.Row{"Namespace", client.Namespace})
	t.AppendRow(table.Row{"Image", client.Spec.Image})
	t.AppendRow(table.Row{"Cores Number", client.Spec.CoresNumber})
	t.AppendRow(table.Row{"CPU Policy", client.Spec.CpuPolicy})

	if client.Spec.TargetCluster.Name != "" {
		targetNs := client.Spec.TargetCluster.Namespace
		if targetNs == "" {
			targetNs = client.Namespace // Default to client's namespace
		}
		t.AppendRow(table.Row{"Target Cluster", fmt.Sprintf("%s/%s", targetNs, client.Spec.TargetCluster.Name)})
	} else if len(client.Spec.JoinIps) > 0 {
		t.AppendRow(table.Row{"Join IPs", fmt.Sprintf("%v", client.Spec.JoinIps)})
	}

	t.AppendRow(table.Row{"Node Selector", SelectorToString(client.Spec.NodeSelector)})

	t.SetStyle(table.StyleLight)
	t.Render()
}

// getClientInstanceCount returns the number of client instances that will be created
// For clients, typically one instance per matching node
func getClientInstanceCount(client *v1alpha1.WekaClient, nodes []v1.Node) int {
	if nodes == nil {
		return 0 // Unknown if nodes not available
	}
	matchingNodes := kubernetes.FilterNodesBySelector(nodes, client.Spec.NodeSelector)
	return len(matchingNodes)
}

func calculateClientContainerRequirements(client *v1alpha1.WekaClient) ClientContainerRequirements {
	coresNum := client.Spec.CoresNumber
	cpuPolicy := client.Spec.CpuPolicy

	// Determine if HT is enabled
	usesHT := cpuPolicy == v1alpha1.CpuPolicyDedicatedHT || cpuPolicy == v1alpha1.CpuPolicyAuto

	// Calculate cores: 1 + coresNum, or 1 + (coresNum * 2) if HT enabled
	var cores int
	var coresNoHT int

	coresNoHT = 1 + coresNum
	if usesHT {
		cores = 1 + (coresNum * 2)
	} else {
		cores = coresNoHT
	}

	// Calculate hugepages: 750Mi per core
	hugepages := int64(coresNum) * 750 // in MiB

	// Calculate memory: 2GB per core
	memory := int64(coresNum) * 2 * 1024 * 1024 * 1024 // in bytes

	// Calculate CPU (millis): roughly 1000m per core
	cpuMilli := int64(cores) * 1000

	return ClientContainerRequirements{
		Hugepages: hugepages,
		Cores:     cores,
		CoresNoHT: coresNoHT,
		Memory:    memory,
		CPUMilli:  cpuMilli,
	}
}

func printClientContainerRequirements(req ClientContainerRequirements, nodeCount int) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{
		"Container Type",
		"Count",
		"Cores (HT On)",
		"Cores (HT Off)",
		"Hugepages",
		"Memory",
		"Drives",
	})

	memMiB := req.Memory / (1024 * 1024)
	t.AppendRow(table.Row{
		"Client",
		nodeCount,
		req.Cores,
		req.CoresNoHT,
		fmt.Sprintf("%d MiB", req.Hugepages),
		fmt.Sprintf("%d MiB", memMiB),
		"-",
	})

	t.SetStyle(table.StyleLight)
	t.Render()
	fmt.Println()
}

func printClientNodesTable(nodes []v1.Node, podsByNode map[string][]v1.Pod) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{
		"NODE",
		"CPU ALLOC.",
		"CPU USED",
		"CPU FREE",
		"MEMORY ALLOC.",
		"MEMORY USED",
		"MEMORY FREE",
		"HP ALLOC.",
		"HP USED",
		"HP FREE",
	})

	for _, node := range nodes {
		allocCPU := kubernetes.QuantityOrZero(node.Status.Allocatable, v1.ResourceCPU)
		allocMem := kubernetes.QuantityOrZero(node.Status.Allocatable, v1.ResourceMemory)
		allocHP := kubernetes.QuantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

		// Calculate used resources from pods on this node
		usedCPU := CalculatePodResourceUsage(podsByNode[node.Name], v1.ResourceCPU)
		usedMem := CalculatePodResourceUsage(podsByNode[node.Name], v1.ResourceMemory)
		usedHP := CalculatePodResourceUsage(podsByNode[node.Name], "hugepages-2Mi")

		// Calculate free resources
		freeCPU := allocCPU.MilliValue() - usedCPU.MilliValue()
		freeMem := allocMem.Value() - usedMem.Value()
		freeHP := allocHP.Value() - usedHP.Value()

		t.AppendRow(table.Row{
			node.Name,
			fmt.Sprintf("%.1f", float64(allocCPU.MilliValue())/1000),
			fmt.Sprintf("%.1f", float64(usedCPU.MilliValue())/1000),
			fmt.Sprintf("%.1f", float64(freeCPU)/1000),
			fmt.Sprintf("%.1f Gi", float64(allocMem.Value())/(1024*1024*1024)),
			fmt.Sprintf("%.1f Gi", float64(usedMem.Value())/(1024*1024*1024)),
			fmt.Sprintf("%.1f Gi", float64(freeMem)/(1024*1024*1024)),
			fmt.Sprintf("%.1f Gi", float64(allocHP.Value())/(1024*1024*1024)),
			fmt.Sprintf("%.1f Gi", float64(usedHP.Value())/(1024*1024*1024)),
			fmt.Sprintf("%.1f Gi", float64(freeHP)/(1024*1024*1024)),
		})
	}

	t.SetStyle(table.StyleLight)
	t.Render()
}

func simulateClientAllocation(nodes []v1.Node, podsByNode map[string][]v1.Pod, containerReqs ClientContainerRequirements) []ClientNodeAllocation {
	var allocations []ClientNodeAllocation

	for _, node := range nodes {
		alloc := ClientNodeAllocation{
			NodeName:       node.Name,
			ContainerCount: 1, // 1 client container per node
		}

		// Calculate available resources
		memQuantity := node.Status.Allocatable[v1.ResourceMemory]
		hpQuantity := node.Status.Allocatable["hugepages-2Mi"]

		// Subtract pod resources
		usedMem := CalculatePodResourceUsage(podsByNode[node.Name], v1.ResourceMemory)
		usedHP := CalculatePodResourceUsage(podsByNode[node.Name], "hugepages-2Mi")

		alloc.AvailableMemory = memQuantity.Value() - usedMem.Value()
		alloc.AvailableHugepage = hpQuantity.Value() - usedHP.Value()

		// Calculate total requirements for 1 client container
		alloc.TotalHugepages = containerReqs.Hugepages * 1024 * 1024 // Convert MiB to bytes
		alloc.TotalMemory = containerReqs.Memory
		alloc.TotalCores = containerReqs.Cores
		alloc.TotalCPUMilli = containerReqs.CPUMilli

		// Check if allocation fits
		alloc.CanFit = alloc.TotalHugepages <= alloc.AvailableHugepage && alloc.TotalMemory <= alloc.AvailableMemory

		if !alloc.CanFit {
			if alloc.TotalHugepages > alloc.AvailableHugepage {
				alloc.Issues = append(alloc.Issues, fmt.Sprintf("Insufficient hugepages: need %dMi, have %dMi", containerReqs.Hugepages, alloc.AvailableHugepage/1024/1024))
			}
			if alloc.TotalMemory > alloc.AvailableMemory {
				alloc.Issues = append(alloc.Issues, fmt.Sprintf("Insufficient memory: need %.2fGi, have %.2fGi", float64(alloc.TotalMemory)/float64(1024*1024*1024), float64(alloc.AvailableMemory)/float64(1024*1024*1024)))
			}
		}

		allocations = append(allocations, alloc)
	}

	return allocations
}

// convertClientAllocationsToNodePlacements converts ClientNodeAllocation to NodePlacement format
// so we can reuse the existing printPlacementDetailsWithResourceAllocation function
func convertClientAllocationsToNodePlacements(allocations []ClientNodeAllocation) []NodePlacement {
	var placements []NodePlacement

	for _, alloc := range allocations {
		placement := NodePlacement{
			NodeName: alloc.NodeName,
			Containers: []PlacedContainer{
				{
					Type:      "client",
					Index:     0,
					Cores:     alloc.TotalCores,
					Memory:    alloc.TotalMemory / (1024 * 1024),    // Convert bytes to MiB
					Hugepages: alloc.TotalHugepages / (1024 * 1024), // Convert bytes to MiB
				},
			},
			UsedCores:  alloc.TotalCores,
			UsedMemory: alloc.TotalMemory / (1024 * 1024),    // Convert bytes to MiB
			UsedHP:     alloc.TotalHugepages / (1024 * 1024), // Convert bytes to MiB
		}
		placements = append(placements, placement)
	}

	return placements
}

func printClientAllocationFeasibility(allocations []ClientNodeAllocation) {
	successCount := 0
	failureCount := 0
	var failedNodes []string

	for _, alloc := range allocations {
		if alloc.CanFit {
			successCount++
		} else {
			failureCount++
			failedNodes = append(failedNodes, alloc.NodeName)
		}
	}

	totalNodes := len(allocations)
	fmt.Printf("Total nodes analyzed: %d\n", totalNodes)
	fmt.Printf("✅ Nodes with sufficient resources: %d (%.1f%%)\n",
		successCount, float64(successCount)*100/float64(totalNodes))

	if failureCount > 0 {
		fmt.Printf("✗ Nodes with insufficient resources: %d (%.1f%%)\n",
			failureCount, float64(failureCount)*100/float64(totalNodes))

		if failureCount <= 5 {
			// Show failed nodes if not too many
			fmt.Printf("   Failed nodes: %s\n", strings.Join(failedNodes, ", "))
		}
	}

	if failureCount == 0 {
		fmt.Println("\n✅ All nodes can accommodate client containers")
	} else if successCount > 0 {
		fmt.Printf("\n⚠️ %d/%d nodes have insufficient resources\n", failureCount, totalNodes)
	} else {
		fmt.Println("\n❌ No nodes can accommodate client containers - please review resource requirements")
	}
}

// validateNetworkInterfacesForClient validates network interfaces for the client
func validateNetworkInterfacesForClient(clients *kubernetes.K8sClients, client *v1alpha1.WekaClient, matchingNodes []v1.Node) bool {
	// For clients, we typically don't have hostcheck data readily available
	// Network validation requires hostcheck data to verify NICs exist and have proper configuration
	// For now, provide guidance to run preflight checks

	// Print what's being tested
	printNetworkValidationTestsDescription()

	if len(matchingNodes) == 0 {
		fmt.Println("  ℹ️  No matching nodes - skipping network validation")
		return false
	}

	hostChecksForValidation, err := hostcheck.GlobalHostCheckRegistry.GetHostChecksForNodes(context.Background(), clients, matchingNodes)
	if err != nil {
		fmt.Println("  ❌ Could not retrieve host check data for network validation:", err)
		return false
	}

	// Validate network config with stats
	result, stats := ValidateNetworkInterfacesWithStats(&client.Spec, hostChecksForValidation, true, matchingNodes)

	if !result.Valid {
		fmt.Println("  ❌ Network interface validation failed:")
		for _, err := range result.Errors {
			fmt.Printf("    %s\n", err.String())
		}
		return true
	}

	fmt.Println("  ✅ Network interfaces validated")
	for _, warn := range result.Warnings {
		fmt.Printf("  ⚠️  %s\n", warn.String())
	}

	// Print summary table if we have any stats
	if stats != nil && len(stats.InterfaceStats) > 0 {
		fmt.Println(printNetworkInterfaceSummaryTable(stats))
	}

	return false
}
