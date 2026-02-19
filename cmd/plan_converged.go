package cmd

import (
	"context"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"os"
	"sort"
	"strings"
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

}

// ConvergedNodeState tracks resource usage on a node through multiple allocation phases
type ConvergedNodeState struct {
	NodeName string
	Node     *corev1.Node

	// Original state (before any simulation)
	OriginalUsedCPU    resource.Quantity
	OriginalUsedMemory resource.Quantity
	OriginalUsedHP     resource.Quantity

	// After cluster allocation
	ClusterContainers []PlacedContainer
	ClusterUsedCores  int
	ClusterUsedMemory int64 // MiB
	ClusterUsedHP     int64 // MiB
	ClusterUsedDrives int   // Number of drives allocated to cluster containers

	// After client allocation
	ClientContainers []PlacedContainer
	ClientUsedCores  int
	ClientUsedMemory int64 // MiB
	ClientUsedHP     int64 // MiB

	// Total allocatable
	AllocatableCPU    resource.Quantity
	AllocatableMemory resource.Quantity
	AllocatableHP     resource.Quantity
}

func runPlanConverged(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	clusterFile := args[0]
	clientFile := args[1]

	// Parse YAML files
	fmt.Println("=== Parsing Deployment Files ===")
	cluster, err := ParseWekaResourceFile[*wekaapi.WekaCluster](clusterFile)
	if err != nil {
		return fmt.Errorf("failed to parse cluster file: %w", err)
	}
	fmt.Printf("✅ Cluster: %s/%s\n", cluster.Namespace, cluster.Name)

	client, err := ParseWekaResourceFile[*wekaapi.WekaClient](clientFile)
	if err != nil {
		return fmt.Errorf("failed to parse client file: %w", err)
	}
	fmt.Printf("✅ Client: %s/%s\n", client.Namespace, client.Name)

	// Validate YAML-only compatibility (before connecting to Kubernetes)
	fmt.Println("\n=== Validating Client-Cluster Compatibility ===")
	if err := validateClientClusterMatch(cluster, client); err != nil {
		fmt.Printf("client-cluster compatibility validation failed: %v", err)
		return err
	}
	fmt.Printf("✅ Client targetCluster matches WekaCluster\n")

	// Validate image version compatibility
	if err := validateImageVersionCompatibility(cluster, client); err != nil {
		fmt.Println("❌ image version validation failed")
		return err
	}

	// Get cluster nodes
	fmt.Println("\n=== Connecting to Kubernetes Cluster ===")
	nodes, err := GetClusterNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster nodes: %w", err)
	}
	fmt.Printf("✅ Connected. Found %d nodes\n", len(nodes))

	// Validate and plan converged deployment
	if err := validateAndPlanConverged(ctx, cluster, client, nodes); err != nil {
		return err
	}

	return nil
}

func validateAndPlanConverged(ctx context.Context, cluster *wekaapi.WekaCluster, client *wekaapi.WekaClient, nodes []corev1.Node) error {

	// Collect pod data
	fmt.Println("\n=== Fetching Current Resource Usage ===")
	podsByNode := make(map[string][]corev1.Pod)
	crClient := KubeClients.CRClient

	var podList corev1.PodList
	if err := crClient.List(ctx, &podList); err == nil {
		for _, pod := range podList.Items {
			if pod.Spec.NodeName != "" {
				podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
			}
		}
	}
	fmt.Printf("✅ Collected pod data from cluster\n")

	// Calculate cluster container requirements
	fmt.Println("\n=== Cluster Container Requirements ===")
	clusterContainers := buildClusterContainerList(cluster)
	printClusterContainerRequirements(clusterContainers)

	// Calculate and print node requirements for cluster
	clusterNodeReqs := calculateNodeRequirements(cluster.Spec.Dynamic, clusterContainers)
	printNodeRequirements(clusterNodeReqs)

	// Calculate client container requirements
	fmt.Println("\n=== Client Container Requirements ===")
	clientReqs := calculateClientContainerRequirements(client)
	clientNodes := FilterNodesBySelector(nodes, client.Spec.NodeSelector)
	printClientContainerRequirements(clientReqs, len(clientNodes))

	// Get node groupings for cluster
	fmt.Println("\n=== Analyzing Node Selectors ===")

	// Track NotReady nodes for warnings
	readyNodes := FilterReadyNodes(nodes)
	totalNotReadyNodes := CountNotReadyNodes(nodes)
	hasNotReadyNodes := totalNotReadyNodes > 0

	// Build role grouping with ready nodes only
	roleGrouping := buildRoleNodeGrouping(readyNodes, cluster.Spec.NodeSelector, &cluster.Spec.RoleNodeSelector)
	printNodeSelectorSummary(roleGrouping, cluster.Spec.NodeSelector)

	// Show client node selector info
	allClientNodes := FilterNodesBySelector(nodes, client.Spec.NodeSelector)
	clientNodes = FilterNodesBySelector(readyNodes, client.Spec.NodeSelector)
	clientNotReadyCount := len(allClientNodes) - len(clientNodes)
	fmt.Printf("Client NodeSelector (%s): %d nodes\n", formatSelector(client.Spec.NodeSelector), len(clientNodes))

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
	var hostChecksMap HostChecksMap
	if cluster.Spec.Dynamic != nil && cluster.Spec.Dynamic.DriveContainers != nil &&
		*cluster.Spec.Dynamic.DriveContainers > 0 && cluster.Spec.Dynamic.NumDrives > 0 {
		fmt.Println("\n=== Detailed Drive Validation ===")
		fmt.Println("Scanning nodes for NVMe drives...")

		var err error
		hostChecksMap, err = GlobalHostCheckRegistry.GetHostChecksForNodes(ctx, allEligibleNodes)
		if err != nil {
			fmt.Printf("⚠️ WARNING: Could not scan drives on all nodes: %v\n", err)
			fmt.Println("   Falling back to basic drive validation...")
			hostChecksMap = nil
		} else if hostChecksMap != nil {
			// Use detailed validation with hostcheck data
			if err := validateDrivesDetailed(hostChecksMap, allEligibleNodes,
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

	// Phase 4: Validate no conflicts between client/s3/nfs
	fmt.Println("\n=== Validating Container Compatibility ===")
	if err := validateContainerCompatibility(convergedStates); err != nil {
		return fmt.Errorf("container compatibility validation failed: %w", err)
	}
	fmt.Printf("✅ No conflicting container types on same nodes\n")

	// Print converged placement details with drive information
	fmt.Println("\n=== Converged Deployment Plan ===")
	printConvergedPlacementDetails(convergedStates, hostChecksMap)

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

func initializeConvergedStates(nodes []corev1.Node, podsByNode map[string][]corev1.Pod, clusterPlacements []NodePlacement) map[string]*ConvergedNodeState {
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
			AllocatableCPU:    QuantityOrZero(node.Status.Allocatable, corev1.ResourceCPU),
			AllocatableMemory: QuantityOrZero(node.Status.Allocatable, corev1.ResourceMemory),
			AllocatableHP:     QuantityOrZero(node.Status.Allocatable, "hugepages-2Mi"),
		}

		// Calculate current pod usage
		state.OriginalUsedCPU = calculatePodResourceUsage(podsByNode[nodeName], corev1.ResourceCPU)
		state.OriginalUsedMemory = calculatePodResourceUsage(podsByNode[nodeName], corev1.ResourceMemory)
		state.OriginalUsedHP = calculatePodResourceUsage(podsByNode[nodeName], "hugepages-2Mi")

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

func simulateClientOnConverged(states map[string]*ConvergedNodeState, clientNodes []corev1.Node, clientReqs ClientContainerRequirements) error {
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

func printConvergedPlacementDetails(states map[string]*ConvergedNodeState, hostChecksMap HostChecksMap) {
	// Get sorted list of nodes that have any containers
	var activeNodes []string
	for nodeName, state := range states {
		if len(state.ClusterContainers) > 0 || len(state.ClientContainers) > 0 {
			activeNodes = append(activeNodes, nodeName)
		}
	}
	sortNodeNamesNumerically(activeNodes)

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
			coloredType := colorizeContainerType(pc.Type)
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
			coloredType := colorizeContainerType(pc.Type)
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
			currentDrivesUsed := 0 // Drives used by existing pods (TODO: could track this if needed)
			wekaDrivesPercent := 0.0
			if totalDrives > 0 {
				wekaDrivesPercent = float64(state.ClusterUsedDrives) * 100.0 / float64(totalDrives)
			}
			// Create bar showing drive allocation (no "currently used" for drives)
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

func getActiveNodes(states map[string]*ConvergedNodeState) []string {
	var active []string
	for nodeName, state := range states {
		if len(state.ClusterContainers) > 0 || len(state.ClientContainers) > 0 {
			active = append(active, nodeName)
		}
	}
	sort.Strings(active)
	return active
}

// validateContainerCompatibility checks that client, s3, and nfs containers don't coexist on the same node
// This is a requirement for current WEKA software versions
func validateContainerCompatibility(states map[string]*ConvergedNodeState) error {
	var errors []string

	for nodeName, state := range states {
		// Get container types on this node
		hasClient := false
		hasS3 := false
		hasNFS := false

		// Check cluster containers
		for _, container := range state.ClusterContainers {
			switch container.Type {
			case "s3":
				hasS3 = true
			case "nfs":
				hasNFS = true
			}
		}

		// Check client containers
		for _, container := range state.ClientContainers {
			if container.Type == "client" {
				hasClient = true
			}
		}

		// Validate incompatible combinations
		conflicts := []string{}
		if hasClient && hasS3 {
			conflicts = append(conflicts, "client and s3")
		}
		if hasClient && hasNFS {
			conflicts = append(conflicts, "client and nfs")
		}
		if hasS3 && hasNFS {
			conflicts = append(conflicts, "s3 and nfs")
		}

		if len(conflicts) > 0 {
			errors = append(errors, fmt.Sprintf("  ❌ Node %s: incompatible container types (%s) cannot coexist",
				nodeName, strings.Join(conflicts, ", ")))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("container compatibility violations found:\n%s\n\nClient, S3, and NFS containers cannot run on the same node in current WEKA software versions.\nPlease adjust nodeSelectors to prevent overlap.",
			strings.Join(errors, "\n"))
	}

	return nil
}

func printNodeSelectorSummary(grouping RoleNodeGrouping, globalSelector map[string]string) {
	fmt.Printf("Global NodeSelector: %s (%d nodes)\n", formatSelector(globalSelector), len(grouping.Global))

	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		if roleGroup, exists := grouping.ByRole[role]; exists && len(roleGroup.Nodes) > 0 {
			fmt.Printf("%s role: %s (%d nodes)\n",
				capitalizeFirst(role),
				formatSelector(roleGroup.Selector),
				len(roleGroup.Nodes))
		}
	}
}

func printClusterContainerRequirements(containers []ContainerRequirements) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"TYPE", "COUNT", "CORES (HT ON)", "CORES (HT OFF)", "HUGEPAGES", "MEMORY", "DRIVES"})

	for _, c := range containers {
		if c.Count == 0 {
			continue
		}
		drivesStr := "-"
		if c.Drives > 0 {
			drivesStr = fmt.Sprintf("%d", c.Drives)
		}
		t.AppendRow(table.Row{
			capitalizeFirst(c.Type),
			c.Count,
			c.Cores,
			c.CoresNoHT,
			fmt.Sprintf("%d MiB", c.Hugepages),
			fmt.Sprintf("%d MiB", c.Memory),
			drivesStr,
		})
	}

	t.SetStyle(table.StyleLight)
	t.Render()
}

// buildClusterContainerList builds the list of containers from a WekaCluster spec
func buildClusterContainerList(cluster *wekaapi.WekaCluster) []ContainerRequirements {
	var containers []ContainerRequirements

	if cluster.Spec.Dynamic == nil {
		return containers
	}

	config := cluster.Spec.Dynamic
	usesHT := cluster.Spec.CpuPolicy == wekaapi.CpuPolicyDedicatedHT || cluster.Spec.CpuPolicy == wekaapi.CpuPolicyAuto
	additionalMemory := cluster.Spec.AdditionalMemory

	// Compute containers
	if config.ComputeContainers != nil && *config.ComputeContainers > 0 {
		req := calculateComputeRequirements(
			config.ComputeCores,
			0, // ComputeExtraCores - not in WekaConfig
			config.ComputeHugepages,
			additionalMemory.Compute,
			usesHT,
			cluster.Spec.RoleCoreIds.Compute,
		)
		req.Type = "compute"
		req.Count = *config.ComputeContainers

		reqNoHT := calculateComputeRequirements(
			config.ComputeCores,
			0,
			config.ComputeHugepages,
			additionalMemory.Compute,
			false,
			cluster.Spec.RoleCoreIds.Compute,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)
	}

	// Drive containers
	if config.DriveContainers != nil && *config.DriveContainers > 0 {
		req := calculateDriveRequirements(
			config.DriveCores,
			0, // DriveExtraCores - not in WekaConfig
			config.NumDrives,
			config.DriveHugepages,
			additionalMemory.Drive,
			usesHT,
			cluster.Spec.RoleCoreIds.Drive,
		)
		req.Type = "drive"
		req.Count = *config.DriveContainers
		req.Drives = config.NumDrives // Set drive requirements

		reqNoHT := calculateDriveRequirements(
			config.DriveCores,
			0,
			config.NumDrives,
			config.DriveHugepages,
			additionalMemory.Drive,
			false,
			cluster.Spec.RoleCoreIds.Drive,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)
	}

	// S3 containers
	if config.S3Containers > 0 {
		req := calculateS3Requirements(
			config.S3Cores,
			config.S3ExtraCores,
			config.S3FrontendHugepages,
			additionalMemory.S3,
			usesHT,
			cluster.Spec.RoleCoreIds.S3,
		)
		req.Type = "s3"
		req.Count = config.S3Containers

		reqNoHT := calculateS3Requirements(
			config.S3Cores,
			config.S3ExtraCores,
			config.S3FrontendHugepages,
			additionalMemory.S3,
			false,
			cluster.Spec.RoleCoreIds.S3,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)

		// Envoy containers (paired with S3)
		envoyReq := calculateEnvoyRequirements(additionalMemory.Envoy)
		envoyReq.Type = "envoy"
		envoyReq.Count = config.S3Containers
		containers = append(containers, envoyReq)
	}

	// NFS containers
	if config.NfsContainers > 0 {
		req := calculateNfsRequirements(
			config.NfsCores,
			config.NfsExtraCores,
			config.NfsFrontendHugepages,
			additionalMemory.Nfs,
			usesHT,
			cluster.Spec.RoleCoreIds.Nfs,
		)
		req.Type = "nfs"
		req.Count = config.NfsContainers

		reqNoHT := calculateNfsRequirements(
			config.NfsCores,
			config.NfsExtraCores,
			config.NfsFrontendHugepages,
			additionalMemory.Nfs,
			false,
			cluster.Spec.RoleCoreIds.Nfs,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)
	}

	return containers
}
