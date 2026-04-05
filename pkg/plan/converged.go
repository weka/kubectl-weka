package plan

import (
	"context"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/weka/kubectl-weka/pkg/hostcheck"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/preflight"
	"github.com/weka/kubectl-weka/pkg/wekaconfig"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
)

func ValidateAndPlanConverged(ctx context.Context, clients *kubernetes.K8sClients, cluster *v1alpha1.WekaCluster, client *v1alpha1.WekaClient) error {
	// Get cluster nodes
	fmt.Println("\n=== Connecting to Kubernetes Cluster ===")
	nodes, err := kubernetes.GetClusterNodes(ctx, clients.CRClient)
	if err != nil {
		return fmt.Errorf("failed to get cluster nodes: %w", err)
	}
	fmt.Printf("✅ Connected. Found %d nodes\n", len(nodes))

	// Collect pod data
	fmt.Println("\n=== Fetching Current Resource Usage ===")
	podsByNode := preflight.GetPodsMapByNode(ctx, clients.CRClient, nil)

	fmt.Printf("✅ Collected pod data from cluster\n")

	// Collect existing WekaContainer data for drive allocation tracking
	var existingContainers []v1alpha1.WekaContainer
	var containerList v1alpha1.WekaContainerList
	if err := clients.CRClient.List(ctx, &containerList); err == nil {
		existingContainers = containerList.Items
		fmt.Printf("✅ Collected existing WekaContainer data (%d containers)\n", len(existingContainers))
	} else {
		fmt.Printf("⚠️ Could not retrieve existing WekaContainers: %v\n", err)
		existingContainers = []v1alpha1.WekaContainer{}
	}

	// Calculate cluster container requirements
	fmt.Println("\n=== Cluster Container Requirements ===")
	clusterContainers := buildClusterContainerList(cluster)
	printClusterContainerRequirements(clusterContainers)

	// Calculate and print node requirements for cluster
	clusterNodeReqs := CalculateNodeRequirements(cluster.Spec.Dynamic, clusterContainers)
	PrintNodeRequirements(clusterNodeReqs)

	// Calculate client container requirements
	fmt.Println("\n=== Client Container Requirements ===")
	clientReqs := calculateClientContainerRequirements(client)
	clientNodes := kubernetes.FilterNodesBySelector(nodes, client.Spec.NodeSelector)
	printClientContainerRequirements(clientReqs, len(clientNodes))

	// Get node groupings for cluster
	fmt.Println("\n=== Analyzing Node Selectors ===")

	// Track NotReady nodes for warnings
	readyNodes := kubernetes.FilterReadyNodes(nodes)
	totalNotReadyNodes := CountNotReadyNodes(nodes)
	hasNotReadyNodes := totalNotReadyNodes > 0

	// Build role grouping with ready nodes only
	roleGrouping := buildRoleNodeGrouping(readyNodes, cluster.Spec.NodeSelector, &cluster.Spec.RoleNodeSelector)
	printNodeSelectorSummary(roleGrouping, cluster.Spec.NodeSelector)

	// Show client node selector info
	allClientNodes := kubernetes.FilterNodesBySelector(nodes, client.Spec.NodeSelector)
	clientNodes = kubernetes.FilterNodesBySelector(readyNodes, client.Spec.NodeSelector)
	clientNotReadyCount := len(allClientNodes) - len(clientNodes)
	fmt.Printf("Client NodeSelector (%s): %d nodes\n", SelectorToString(client.Spec.NodeSelector), len(clientNodes))

	// Check how many NotReady nodes matched the cluster selectors
	allClusterMatching := getAllEligibleNodes(buildRoleNodeGrouping(nodes, cluster.Spec.NodeSelector, &cluster.Spec.RoleNodeSelector))
	allEligibleNodes := getAllEligibleNodes(roleGrouping)
	clusterNotReadyMatchingCount := len(allClusterMatching) - len(allEligibleNodes)

	// Warn about NotReady nodes
	totalNotReadyMatching := clusterNotReadyMatchingCount
	if clientNotReadyCount > clusterNotReadyMatchingCount {
		// Some NotReady nodes matched client selector but not cluster selector
		totalNotReadyMatching = clientNotReadyCount
	}

	if totalNotReadyMatching > 0 {
		fmt.Printf("\n⚠️ WARNING: Additional %d node(s) match the selectors but are in NotReady state.\n", totalNotReadyMatching)
		if clusterNotReadyMatchingCount > 0 {
			fmt.Printf("   - %d node(s) match cluster selectors\n", clusterNotReadyMatchingCount)
		}
		if clientNotReadyCount > 0 {
			fmt.Printf("   - %d node(s) match client selector\n", clientNotReadyCount)
		}
		fmt.Println("   These nodes will not be checked for compliancy.")
	}

	// Get all eligible nodes for drive validation (already filtered to ready)

	// Get hostchecks for all eligible nodes (cached execution)
	// This runs hostchecks on-demand and caches results for subsequent use
	var hostChecksMap hostcheck.HostChecksMap
	if cluster.Spec.Dynamic != nil && cluster.Spec.Dynamic.DriveContainers != nil &&
		*cluster.Spec.Dynamic.DriveContainers > 0 && cluster.Spec.Dynamic.NumDrives > 0 {
		fmt.Println("\n=== Detailed Drive Validation ===")
		fmt.Println("Scanning nodes for NVMe drives...")

		var err error
		hostChecksMap, err = hostcheck.GlobalHostCheckRegistry.GetHostChecksForNodes(ctx, clients, allEligibleNodes)
		if err != nil {
			fmt.Printf("⚠️ WARNING: Could not scan drives on all nodes: %v\n", err)
			fmt.Println("   Falling back to basic drive validation...")
			hostChecksMap = nil
		} else if hostChecksMap != nil {
			// Use detailed validation with hostcheck data
			// Get nodes that match the drive role selector for accurate validation
			// This ensures validation counts only drives on nodes that will be used for placement
			var driveRoleNodes []v1.Node
			if roleGrouping.ByRole != nil {
				if driveRole, ok := roleGrouping.ByRole["drive"]; ok {
					driveRoleNodes = append(driveRoleNodes, driveRole.Nodes...)
				}
			}
			// Add global nodes (they go to all roles)
			for _, node := range roleGrouping.Global {
				driveRoleNodes = append(driveRoleNodes, node)
			}

			// Validate with only nodes that will be used for drive placement
			// This ensures validation matches what placement will actually try
			// Pass allEligibleNodes as well to check if drives exist elsewhere

			if err := validateDrivesDetailed(hostChecksMap, driveRoleNodes, allEligibleNodes,
				*cluster.Spec.Dynamic.DriveContainers,
				cluster.Spec.Dynamic.NumDrives); err != nil {
				return err
			}
		}
	}

	// Phase 1: Simulate cluster placement with drive awareness
	clusterPlacements, err := simulatePlacement(roleGrouping, clusterContainers, podsByNode, hostChecksMap)
	if err != nil {
		return fmt.Errorf("cluster placement failed: %w", err)
	}
	fmt.Printf("✅ Successfully placed all cluster containers\n")

	// Phase 2: Initialize converged state with cluster allocations
	convergedStates := initializeConvergedStates(nodes, podsByNode, clusterPlacements)

	// Phase 3: Simulate client placement on top of cluster
	fmt.Println("\n=== Simulating Client Placement (on top of cluster) ===")
	if err := simulateClientOnConverged(convergedStates, clientNodes, clientReqs); err != nil {
		return fmt.Errorf("client placement failed: %w", err)
	}
	fmt.Printf("✅ Successfully placed all client containers\n")

	// Phase 3.5: Validate network interfaces for both cluster and client
	fmt.Println("\n=== Validating Network Interfaces ===")
	if networkValidationErrors := validateNetworkInterfacesForConverged(cluster, client, allEligibleNodes, clientNodes, hostChecksMap); networkValidationErrors {
		return fmt.Errorf("network interface validation failed")
	}

	// Phase 4: Validate no conflicts between client/s3/nfs
	fmt.Println("\n=== Validating Container Compatibility ===")
	if err := validateContainerCompatibility(convergedStates); err != nil {
		return fmt.Errorf("container compatibility validation failed: %w", err)
	}
	fmt.Printf("✅ No conflicting container types on same nodes\n")

	// Print converged placement details with drive information
	fmt.Println("\n=== Converged Deployment Plan ===")
	printConvergedPlacementDetails(convergedStates, hostChecksMap, existingContainers)

	// Print summary
	fmt.Println("\n=== Deployment Summary ===")
	printConvergedSummary(convergedStates)

	// Final summary with NotReady node warning if applicable
	if hasNotReadyNodes && totalNotReadyMatching > 0 {
		fmt.Println("\n⚠️ WARNING: Plan completed with warnings")
		fmt.Printf("   ⚠️ %d node(s) were not ready during planning and were skipped\n", totalNotReadyNodes)
		if clusterNotReadyMatchingCount > 0 {
			fmt.Printf("   ⚠️ %d of the skipped nodes matched cluster selectors\n", clusterNotReadyMatchingCount)
		}
		if clientNotReadyCount > 0 {
			fmt.Printf("   ⚠️ %d of the skipped nodes matched client selector\n", clientNotReadyCount)
		}
		fmt.Println("   Please notice that required validations were not performed on these nodes.")
		fmt.Println("   Recommended to remediate the nodes and rerun plan.")
	} else {
		fmt.Println("\n✅ Converged deployment plan complete!")
	}

	return nil
}

func ParseAndValidateConfigs(ctx context.Context, clients *kubernetes.K8sClients, clusterFile string, clientFile string) (*wekaconfig.WekaConfigContext, error) {
	fmt.Println("=== Parsing Deployment Files ===")
	ret := &wekaconfig.WekaConfigContext{}
	cluster, err := ParseWekaResourceFile[*v1alpha1.WekaCluster](clusterFile)
	if err != nil {
		return ret, fmt.Errorf("failed to parse cluster file: %w", err)
	}
	ret.Cluster = cluster
	fmt.Printf("✅ Cluster: %s/%s\n", cluster.Namespace, cluster.Name)

	client, err := ParseWekaResourceFile[*v1alpha1.WekaClient](clientFile)
	if err != nil {
		return ret, fmt.Errorf("failed to parse client file: %w", err)
	}
	ret.Client = client
	fmt.Printf("✅ Client: %s/%s\n", client.Namespace, client.Name)

	// Validate YAML-only compatibility (before connecting to Kubernetes)
	fmt.Println("\n=== Validating Client-Cluster Compatibility ===")

	// Run all applicable validations
	results, err := wekaconfig.GlobalWekaConfigValidationRegistry.ValidateAll(ctx, clients, ret)
	if err != nil {
		return ret, fmt.Errorf("validation failed: %w", err)
	}

	// Print validation results
	wekaconfig.GlobalWekaConfigValidationRegistry.PrintValidationResults(results)

	// Check for errors
	hasErrors := false
	for _, result := range results {
		if result.Status == "error" {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return ret, fmt.Errorf("client-cluster compatibility validation failed")
	}

	fmt.Println("✅ Client-cluster compatibility validation passed")
	return ret, nil
}

func initializeConvergedStates(nodes []v1.Node, podsByNode map[string][]v1.Pod, clusterPlacements []NodePlacement) map[string]*ConvergedNodeState {
	states := make(map[string]*ConvergedNodeState)

	// Create map of cluster placements for quick lookup
	clusterPlacementMap := make(map[string]*NodePlacement)
	for i := range clusterPlacements {
		clusterPlacementMap[clusterPlacements[i].NodeName] = &clusterPlacements[i]
	}

	// Initialize all nodes
	for i := range nodes {
		node := &nodes[i]
		nodeName := node.Name

		state := &ConvergedNodeState{
			NodeName:          nodeName,
			Node:              node,
			AllocatableCPU:    kubernetes.QuantityOrZero(node.Status.Allocatable, v1.ResourceCPU),
			AllocatableMemory: kubernetes.QuantityOrZero(node.Status.Allocatable, v1.ResourceMemory),
			AllocatableHP:     kubernetes.QuantityOrZero(node.Status.Allocatable, "hugepages-2Mi"),
		}

		// Calculate current pod usage
		state.OriginalUsedCPU = CalculatePodResourceUsage(podsByNode[nodeName], v1.ResourceCPU)
		state.OriginalUsedMemory = CalculatePodResourceUsage(podsByNode[nodeName], v1.ResourceMemory)
		state.OriginalUsedHP = CalculatePodResourceUsage(podsByNode[nodeName], "hugepages-2Mi")

		// Add cluster containers if this node has them
		if placement, exists := clusterPlacementMap[nodeName]; exists {
			state.ClusterContainers = placement.Containers
			state.ClusterUsedCores = placement.UsedCores
			state.ClusterUsedMemory = placement.UsedMemory
			state.ClusterUsedHP = placement.UsedHP
			state.ClusterUsedDrives = placement.UsedDrives
		}

		states[nodeName] = state
	}

	return states
}

func simulateClientOnConverged(states map[string]*ConvergedNodeState, clientNodes []v1.Node, clientReqs ClientContainerRequirements) error {
	// Client containers go on every matching node (no anti-affinity)
	for _, node := range clientNodes {
		state, exists := states[node.Name]
		if !exists {
			continue
		}

		// Calculate current free resources (after cluster allocation)
		currentFreeCPU := state.AllocatableCPU.MilliValue()/1000 -
			state.OriginalUsedCPU.MilliValue()/1000 -
			int64(state.ClusterUsedCores)

		currentFreeMemory := state.AllocatableMemory.Value() -
			state.OriginalUsedMemory.Value() -
			(state.ClusterUsedMemory * 1024 * 1024)

		currentFreeHP := state.AllocatableHP.Value() -
			state.OriginalUsedHP.Value() -
			(state.ClusterUsedHP * 1024 * 1024)

		// Check if client can fit
		requiredMemoryBytes := clientReqs.Memory
		requiredHPBytes := clientReqs.Hugepages * 1024 * 1024

		if int64(clientReqs.Cores) > currentFreeCPU {
			return fmt.Errorf("node %s: insufficient CPU for client (need %d cores, have %d free)",
				node.Name, clientReqs.Cores, currentFreeCPU)
		}
		if requiredMemoryBytes > currentFreeMemory {
			return fmt.Errorf("node %s: insufficient memory for client (need %d bytes, have %d free)",
				node.Name, requiredMemoryBytes, currentFreeMemory)
		}
		if requiredHPBytes > currentFreeHP {
			return fmt.Errorf("node %s: insufficient hugepages for client (need %d bytes, have %d free)",
				node.Name, requiredHPBytes, currentFreeHP)
		}

		// Place client container
		state.ClientContainers = []PlacedContainer{
			{
				Type:      "client",
				Index:     0,
				Cores:     clientReqs.Cores,
				Memory:    clientReqs.Memory / (1024 * 1024), // Convert to MiB
				Hugepages: clientReqs.Hugepages,              // Already in MiB
			},
		}
		state.ClientUsedCores = clientReqs.Cores
		state.ClientUsedMemory = clientReqs.Memory / (1024 * 1024)
		state.ClientUsedHP = clientReqs.Hugepages
	}

	return nil
}

func printConvergedPlacementDetails(states map[string]*ConvergedNodeState, hostChecksMap hostcheck.HostChecksMap, existingContainers []v1alpha1.WekaContainer) {
	// Get sorted list of nodes that have any containers
	var activeNodes []string
	for nodeName, state := range states {
		if len(state.ClusterContainers) > 0 || len(state.ClientContainers) > 0 {
			activeNodes = append(activeNodes, nodeName)
		}
	}
	kubernetes.SortNodeNamesNumerically(activeNodes)

	if len(activeNodes) == 0 {
		fmt.Println("No containers allocated")
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{
		"NODE",
		"CONTAINERS & RESOURCES",
		"RESOURCE ALLOCATION",
	})

	for _, nodeName := range activeNodes {
		state := states[nodeName]

		// Format containers information
		containersStr := ""

		// Show original pod usage
		if state.OriginalUsedCPU.MilliValue() > 0 || state.OriginalUsedMemory.Value() > 0 || state.OriginalUsedHP.Value() > 0 {
			containersStr += fmt.Sprintf("\033[38;5;52m<ALREADY_USED>\033[0m [CORES: %.1f, RAM: %.1fGi, HP: %.1fGi]\n",
				float64(state.OriginalUsedCPU.MilliValue())/1000,
				float64(state.OriginalUsedMemory.Value())/(1024*1024*1024),
				float64(state.OriginalUsedHP.Value())/(1024*1024*1024))
		}

		// Show cluster containers
		for _, pc := range state.ClusterContainers {
			coloredType := createContainerLegend(pc.Type)
			if pc.Drives > 0 {
				containersStr += fmt.Sprintf("%s [CORES: %d, RAM: %.1fGi, HP: %.1fGi, DRIVES: %d]\n",
					coloredType, pc.Cores, float64(pc.Memory)/1024, float64(pc.Hugepages)/1024, pc.Drives)
			} else {
				containersStr += fmt.Sprintf("%s [CORES: %d, RAM: %.1fGi, HP: %.1fGi]\n",
					coloredType, pc.Cores, float64(pc.Memory)/1024, float64(pc.Hugepages)/1024)
			}
		}

		// Show client containers
		for _, pc := range state.ClientContainers {
			coloredType := createContainerLegend(pc.Type)
			containersStr += fmt.Sprintf("%s [CORES: %d, RAM: %.1fGi, HP: %.1fGi]",
				coloredType, pc.Cores, float64(pc.Memory)/1024, float64(pc.Hugepages)/1024)
		}

		// Create resource bars
		resourceBarsStr := ""

		// Collect all container types for coloring
		var containerTypes []string
		for _, pc := range state.ClusterContainers {
			containerTypes = append(containerTypes, pc.Type)
		}
		for _, pc := range state.ClientContainers {
			containerTypes = append(containerTypes, pc.Type)
		}

		// CPU bar
		allocCoresVal := float64(state.AllocatableCPU.MilliValue() / 1000)
		currentCPUPercent := 0.0
		if allocCoresVal > 0 {
			currentCPUPercent = float64(state.OriginalUsedCPU.MilliValue()/1000) * 100.0 / allocCoresVal
		}
		wekaClusterCPUPercent := 0.0
		if allocCoresVal > 0 {
			wekaClusterCPUPercent = float64(state.ClusterUsedCores) * 100.0 / allocCoresVal
		}
		wekaClientCPUPercent := 0.0
		if allocCoresVal > 0 {
			wekaClientCPUPercent = float64(state.ClientUsedCores) * 100.0 / allocCoresVal
		}
		totalWekaPercent := wekaClusterCPUPercent + wekaClientCPUPercent
		cpuBar := createResourceBar(currentCPUPercent, totalWekaPercent, containerTypes)
		resourceBarsStr += fmt.Sprintf("CPU:    %s\n", cpuBar)

		// Memory bar
		allocMemVal := float64(state.AllocatableMemory.Value())
		currentMemPercent := 0.0
		if allocMemVal > 0 {
			currentMemPercent = float64(state.OriginalUsedMemory.Value()) * 100.0 / allocMemVal
		}
		wekaClusterMemVal := float64(state.ClusterUsedMemory) * 1024 * 1024
		wekaClusterMemPercent := 0.0
		if allocMemVal > 0 {
			wekaClusterMemPercent = wekaClusterMemVal * 100.0 / allocMemVal
		}
		wekaClientMemVal := float64(state.ClientUsedMemory) * 1024 * 1024
		wekaClientMemPercent := 0.0
		if allocMemVal > 0 {
			wekaClientMemPercent = wekaClientMemVal * 100.0 / allocMemVal
		}
		totalWekaMemPercent := wekaClusterMemPercent + wekaClientMemPercent
		memBar := createResourceBar(currentMemPercent, totalWekaMemPercent, containerTypes)
		resourceBarsStr += fmt.Sprintf("Mem:    %s\n", memBar)

		// Hugepages bar
		allocHPVal := float64(state.AllocatableHP.Value())
		currentHPPercent := 0.0
		if allocHPVal > 0 {
			currentHPPercent = float64(state.OriginalUsedHP.Value()) * 100.0 / allocHPVal
		}
		wekaClusterHPVal := float64(state.ClusterUsedHP) * 1024 * 1024
		wekaClusterHPPercent := 0.0
		if allocHPVal > 0 {
			wekaClusterHPPercent = wekaClusterHPVal * 100.0 / allocHPVal
		}
		wekaClientHPVal := float64(state.ClientUsedHP) * 1024 * 1024
		wekaClientHPPercent := 0.0
		if allocHPVal > 0 {
			wekaClientHPPercent = wekaClientHPVal * 100.0 / allocHPVal
		}
		totalWekaHPPercent := wekaClusterHPPercent + wekaClientHPPercent
		hpBar := createResourceBar(currentHPPercent, totalWekaHPPercent, containerTypes)
		resourceBarsStr += fmt.Sprintf("HP:     %s\n", hpBar)

		// Drives bar (only show if node has drives)
		if state.ClusterUsedDrives > 0 || hasNodeDrives(state.Node, hostChecksMap) {
			totalDrives := getNodeTotalDrives(state.Node, hostChecksMap)
			currentDrivesUsed := CalculateAllocatedDrives(existingContainers, nodeName)
			wekaDrivesPercent := 0.0
			if totalDrives > 0 {
				wekaDrivesPercent = float64(state.ClusterUsedDrives) * 100.0 / float64(totalDrives)
			}
			// Create bar showing drive allocation (with currently allocated drives from existing containers)
			drivesBar := createResourceBar(float64(currentDrivesUsed), wekaDrivesPercent, containerTypes)
			resourceBarsStr += fmt.Sprintf("Drives: %s", drivesBar)
		}

		t.AppendRow(table.Row{
			nodeName,
			containersStr,
			resourceBarsStr,
		})
		t.AppendSeparator()
	}

	t.SetStyle(table.StyleLight)
	t.Render()
}

func printConvergedSummary(states map[string]*ConvergedNodeState) {
	// Count nodes and containers
	nodesWithCluster := 0
	nodesWithClient := 0
	nodesWithBoth := 0

	totalClusterContainers := 0
	totalClientContainers := 0

	for _, state := range states {
		hasCluster := len(state.ClusterContainers) > 0
		hasClient := len(state.ClientContainers) > 0

		if hasCluster {
			nodesWithCluster++
			totalClusterContainers += len(state.ClusterContainers)
		}
		if hasClient {
			nodesWithClient++
			totalClientContainers += len(state.ClientContainers)
		}
		if hasCluster && hasClient {
			nodesWithBoth++
		}
	}

	fmt.Printf("✅ Cluster containers: %d across %d nodes\n", totalClusterContainers, nodesWithCluster)
	fmt.Printf("✅ Client containers: %d across %d nodes\n", totalClientContainers, nodesWithClient)
	fmt.Printf("✅ Converged nodes (both cluster + client): %d\n", nodesWithBoth)
	fmt.Printf("✅ Total nodes used: %d\n", len(getActiveNodes(states)))
}

// validateNetworkInterfacesForConverged validates network interfaces for converged cluster and client
// Validates both cluster global/role NICs and client NICs on their respective node sets
func validateNetworkInterfacesForConverged(cluster *v1alpha1.WekaCluster, client *v1alpha1.WekaClient, clusterNodes []v1.Node, clientNodes []v1.Node, hostChecksMap hostcheck.HostChecksMap) bool {
	hasErrors := false

	// Print what's being tested
	printNetworkValidationTestsDescription()

	clusterStats := NewNetworkValidationStats()
	clientStats := NewNetworkValidationStats()

	// Validate cluster network interfaces
	if len(clusterNodes) > 0 {
		fmt.Println("  Validating cluster network interfaces...")

		// Build hostchecks map for cluster nodes
		hostChecksForValidation := make(map[string]*hostcheck.HostChecksResult)
		for _, node := range clusterNodes {
			if hostChecksMap != nil && hostChecksMap[node.Name] != nil {
				hostChecksForValidation[node.Name] = hostChecksMap[node.Name]
			}
		}

		// Validate cluster global config
		globalNodes := kubernetes.FilterNodesBySelector(clusterNodes, cluster.Spec.NodeSelector)
		if len(globalNodes) > 0 {
			result, stats := ValidateNetworkInterfacesWithStats(&cluster.Spec, hostChecksForValidation, false, globalNodes)
			if !result.Valid {
				hasErrors = true
				fmt.Println("    ❌ Global network interface validation failed:")
				for _, err := range result.Errors {
					fmt.Printf("      %s\n", err.String())
				}
			} else {
				fmt.Println("    ✅ Global network interfaces validated")
			}
			for _, warn := range result.Warnings {
				fmt.Printf("    ⚠️  %s\n", warn.String())
			}
			// Merge stats - sum up the counts
			if stats != nil {
				for ifName, stat := range stats.InterfaceStats {
					if existing, ok := clusterStats.InterfaceStats[ifName]; ok {
						existing.Configured += stat.Configured
						existing.Missing += stat.Missing
						existing.Misconfigured += stat.Misconfigured
					} else {
						clusterStats.InterfaceStats[ifName] = stat
					}
				}
			}
		}

		// Validate per-role configurations
		roles := map[string]*map[string]string{
			"compute": cluster.Spec.RoleNodeSelector.Compute,
			"drive":   cluster.Spec.RoleNodeSelector.Drive,
			"s3":      cluster.Spec.RoleNodeSelector.S3,
			"nfs":     cluster.Spec.RoleNodeSelector.Nfs,
		}

		for role, selector := range roles {
			if selector == nil || len(*selector) == 0 {
				continue
			}

			roleNodes := kubernetes.FilterNodesBySelector(clusterNodes, *selector)
			if len(roleNodes) == 0 {
				continue
			}

			result, stats := ValidateNetworkInterfacesWithStats(&cluster.Spec, hostChecksForValidation, false, roleNodes)
			if !result.Valid {
				hasErrors = true
				fmt.Printf("    ❌ Network interface validation failed for role '%s'\n", role)
				for _, err := range result.Errors {
					fmt.Printf("      %s\n", err.String())
				}
			} else {
				fmt.Printf("    ✅ Network interfaces validated for role '%s'\n", role)
			}
			for _, warn := range result.Warnings {
				fmt.Printf("    ⚠️  %s\n", warn.String())
			}
			// Merge stats - sum up the counts
			if stats != nil {
				for ifName, stat := range stats.InterfaceStats {
					if existing, ok := clusterStats.InterfaceStats[ifName]; ok {
						existing.Configured += stat.Configured
						existing.Missing += stat.Missing
						existing.Misconfigured += stat.Misconfigured
					} else {
						clusterStats.InterfaceStats[ifName] = stat
					}
				}
			}
		}
	}

	// Validate client network interfaces
	if len(clientNodes) > 0 {
		fmt.Println("  Validating client network interfaces...")

		// Build hostchecks map for client nodes
		hostChecksForValidation := make(map[string]*hostcheck.HostChecksResult)
		for _, node := range clientNodes {
			if hostChecksMap != nil && hostChecksMap[node.Name] != nil {
				hostChecksForValidation[node.Name] = hostChecksMap[node.Name]
			}
		}

		result, stats := ValidateNetworkInterfacesWithStats(&client.Spec, hostChecksForValidation, true, clientNodes)
		if !result.Valid {
			hasErrors = true
			fmt.Println("    ❌ Client network interface validation failed:")
			for _, err := range result.Errors {
				fmt.Printf("      %s\n", err.String())
			}
		} else {
			fmt.Println("    ✅ Client network interfaces validated")
		}
		for _, warn := range result.Warnings {
			fmt.Printf("    ⚠️  %s\n", warn.String())
		}
		// Merge stats
		if stats != nil {
			for ifName, stat := range stats.InterfaceStats {
				clientStats.InterfaceStats[ifName] = stat
			}
		}
	}

	// Print summary tables
	if len(clusterStats.InterfaceStats) > 0 {
		fmt.Println("\n  Cluster Network Interfaces Summary:")
		fmt.Println(printNetworkInterfaceSummaryTable(clusterStats))
	}

	if len(clientStats.InterfaceStats) > 0 {
		fmt.Println("\n  Client Network Interfaces Summary:")
		fmt.Println(printNetworkInterfaceSummaryTable(clientStats))
	}

	if !hasErrors {
		fmt.Println("  ✅ All network interfaces validated successfully")
	}

	return hasErrors
}
