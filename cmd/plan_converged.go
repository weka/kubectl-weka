package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	fmt.Printf("✓ Cluster: %s/%s\n", cluster.Namespace, cluster.Name)

	client, err := ParseWekaResourceFile[*wekaapi.WekaClient](clientFile)
	if err != nil {
		return fmt.Errorf("failed to parse client file: %w", err)
	}
	fmt.Printf("✓ Client: %s/%s\n", client.Namespace, client.Name)

	// Validate YAML-only compatibility (before connecting to Kubernetes)
	fmt.Println("\n=== Validating Client-Cluster Compatibility ===")
	if err := validateClientClusterMatch(cluster, client); err != nil {
		fmt.Printf("client-cluster compatibility validation failed: %v", err)
		return err
	}
	fmt.Printf("✓ Client targetCluster matches WekaCluster\n")

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
	fmt.Printf("✓ Connected. Found %d nodes\n", len(nodes))

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
	fmt.Printf("✓ Collected pod data from cluster\n")

	// Calculate cluster container requirements
	fmt.Println("\n=== Cluster Container Requirements ===")
	clusterContainers := buildClusterContainerList(cluster)
	printClusterContainerRequirements(clusterContainers)

	// Calculate client container requirements
	fmt.Println("\n=== Client Container Requirements ===")
	clientReqs := calculateClientContainerRequirements(client)
	printClientContainerRequirements(clientReqs)

	// Get node groupings for cluster
	fmt.Println("\n=== Analyzing Node Selectors ===")
	roleGrouping := buildRoleNodeGrouping(nodes, cluster.Spec.NodeSelector, &cluster.Spec.RoleNodeSelector)
	printNodeSelectorSummary(roleGrouping, cluster.Spec.NodeSelector)

	// Filter client nodes
	clientNodes := FilterNodesBySelector(nodes, client.Spec.NodeSelector)
	fmt.Printf("Client NodeSelector (%s): %d nodes\n", formatSelector(client.Spec.NodeSelector), len(clientNodes))

	// Phase 1: Simulate cluster placement
	fmt.Println("\n=== Simulating Container Placement ===")
	// Note: simulatePlacement expects a WekaConfig pointer, which we don't have access to in plan_converged
	// We pass nil and it won't be used since we already have the container list
	clusterPlacements, err := simulatePlacement(roleGrouping, clusterContainers, nil, podsByNode)
	if err != nil {
		return fmt.Errorf("cluster placement failed: %w", err)
	}
	fmt.Printf("✓ Successfully placed all cluster containers\n")

	// Phase 2: Initialize converged state with cluster allocations
	convergedStates := initializeConvergedStates(nodes, podsByNode, clusterPlacements)

	// Phase 3: Simulate client placement on top of cluster
	fmt.Println("\n=== Simulating Client Placement (on top of cluster) ===")
	if err := simulateClientOnConverged(convergedStates, clientNodes, clientReqs); err != nil {
		return fmt.Errorf("client placement failed: %w", err)
	}
	fmt.Printf("✓ Successfully placed all client containers\n")

	// Phase 4: Validate no conflicts between client/s3/nfs
	fmt.Println("\n=== Validating Container Compatibility ===")
	if err := validateContainerCompatibility(convergedStates); err != nil {
		return fmt.Errorf("container compatibility validation failed: %w", err)
	}
	fmt.Printf("✓ No conflicting container types on same nodes\n")

	// Print converged placement details
	fmt.Println("\n=== Converged Deployment Plan ===")
	printConvergedPlacementDetails(convergedStates)

	// Print summary
	fmt.Println("\n=== Deployment Summary ===")
	printConvergedSummary(convergedStates)

	fmt.Println("\n✅ Converged deployment plan complete!")
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

func printConvergedPlacementDetails(states map[string]*ConvergedNodeState) {
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
			containersStr += fmt.Sprintf("%s [CORES: %d, RAM: %.1fGi, HP: %.1fGi]\n",
				coloredType, pc.Cores, float64(pc.Memory)/1024, float64(pc.Hugepages)/1024)
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
		resourceBarsStr += fmt.Sprintf("CPU: %s\n", cpuBar)

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
		resourceBarsStr += fmt.Sprintf("Mem: %s\n", memBar)

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
		resourceBarsStr += fmt.Sprintf("HP:  %s", hpBar)

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

	fmt.Printf("✓ Cluster containers: %d across %d nodes\n", totalClusterContainers, nodesWithCluster)
	fmt.Printf("✓ Client containers: %d across %d nodes\n", totalClientContainers, nodesWithClient)
	fmt.Printf("✓ Converged nodes (both cluster + client): %d\n", nodesWithBoth)
	fmt.Printf("✓ Total nodes used: %d\n", len(getActiveNodes(states)))
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

// validateClientClusterMatch ensures the WekaClient's targetCluster matches the WekaCluster
func validateClientClusterMatch(cluster *wekaapi.WekaCluster, client *wekaapi.WekaClient) error {
	// Check if client has targetCluster specified
	if client.Spec.TargetCluster.Name == "" {
		// If targetCluster is not set, client might use joinIps instead
		// This is valid, so we skip the check
		return nil
	}

	// Validate namespace match
	targetNamespace := client.Spec.TargetCluster.Namespace
	if targetNamespace == "" {
		// If namespace is not specified in targetCluster, it defaults to the client's namespace
		targetNamespace = client.Namespace
	}

	if targetNamespace != cluster.Namespace {
		return fmt.Errorf(
			"client targetCluster namespace mismatch:\n"+
				"  Client '%s/%s' targets cluster namespace: %s\n"+
				"  But WekaCluster is in namespace: %s\n\n"+
				"The client's targetCluster.namespace must match the WekaCluster namespace.",
			client.Namespace, client.Name,
			targetNamespace,
			cluster.Namespace)
	}

	// Validate name match
	if client.Spec.TargetCluster.Name != cluster.Name {
		return fmt.Errorf(
			"client targetCluster name mismatch:\n"+
				"  Client '%s/%s' targets cluster: %s\n"+
				"  But WekaCluster name is: %s\n\n"+
				"The client's targetCluster.name must match the WekaCluster name.",
			client.Namespace, client.Name,
			client.Spec.TargetCluster.Name,
			cluster.Name)
	}

	return nil
}

// validateImageVersionCompatibility checks that client and cluster images are compatible
func validateImageVersionCompatibility(cluster *wekaapi.WekaCluster, client *wekaapi.WekaClient) error {
	clusterImage := cluster.Spec.Image
	clientImage := client.Spec.Image

	// If images are identical, no validation needed
	if clusterImage == clientImage {
		fmt.Printf("✓ Client and cluster images match: %s\n", clusterImage)
		return nil
	}

	// Parse versions from images
	clusterVersion, err := parseWekaVersion(clusterImage)
	if err != nil {
		// If we can't parse, just warn about different images
		fmt.Printf("⚠️  WARNING: Different images detected (cluster: %s, client: %s)\n", clusterImage, clientImage)
		fmt.Printf("    Unable to parse versions for compatibility check\n")
		return nil
	}

	clientVersion, err := parseWekaVersion(clientImage)
	if err != nil {
		// If we can't parse, just warn about different images
		fmt.Printf("⚠️  WARNING: Different images detected (cluster: %s, client: %s)\n", clusterImage, clientImage)
		fmt.Printf("    Unable to parse versions for compatibility check\n")
		return nil
	}

	// Compare versions
	if clusterVersion.Major != clientVersion.Major {
		return fmt.Errorf(
			"incompatible WEKA versions:\n"+
				"  Cluster image: %s (version %s)\n"+
				"  Client image:  %s (version %s)\n\n"+
				"Major version mismatch detected (%d vs %d).\n"+
				"Client and cluster must use the same major version.",
			clusterImage, clusterVersion.String(),
			clientImage, clientVersion.String(),
			clusterVersion.Major, clientVersion.Major)
	}

	if clusterVersion.Minor != clientVersion.Minor {
		return fmt.Errorf(
			"incompatible WEKA versions:\n"+
				"  Cluster image: %s (version %s)\n"+
				"  Client image:  %s (version %s)\n\n"+
				"Minor version mismatch detected (%d.%d vs %d.%d).\n"+
				"Client and cluster must use the same minor version.",
			clusterImage, clusterVersion.String(),
			clientImage, clientVersion.String(),
			clusterVersion.Major, clusterVersion.Minor,
			clientVersion.Major, clientVersion.Minor)
	}

	// Same major.minor but different patch or build
	// Client version must be equal to or older than cluster version
	if clientVersion.Patch < clusterVersion.Patch ||
		(clientVersion.Patch == clusterVersion.Patch && clientVersion.Build < clusterVersion.Build) {
		// Client is older - this may work but warn
		fmt.Printf("⚠️  WARNING: Client version is older than cluster version\n")
		fmt.Printf("    Cluster: %s (version %s)\n", clusterImage, clusterVersion.String())
		fmt.Printf("    Client:  %s (version %s)\n", clientImage, clientVersion.String())
		fmt.Printf("    This may work but is not recommended. Consider upgrading client to match cluster version.\n")
	} else if clientVersion.Patch > clusterVersion.Patch ||
		(clientVersion.Patch == clusterVersion.Patch && clientVersion.Build > clusterVersion.Build) {
		// Client is newer - not allowed
		return fmt.Errorf(
			"incompatible WEKA versions:\n"+
				"  Cluster image: %s (version %s)\n"+
				"  Client image:  %s (version %s)\n\n"+
				"Client version is newer than cluster version.\n"+
				"Client version must be equal to or older than the cluster version.\n"+
				"Please downgrade client to %s or upgrade cluster to match client version.",
			clusterImage, clusterVersion.String(),
			clientImage, clientVersion.String(),
			clusterVersion.String())
	} else {
		// Exact match
		fmt.Printf("✓ Client and cluster versions compatible: %s\n", clusterVersion.String())
	}

	return nil
}

// WekaVersion represents a parsed WEKA version
type WekaVersion struct {
	Major int
	Minor int
	Patch int
	Build int
	Raw   string
}

func (v WekaVersion) String() string {
	if v.Build > 0 {
		return fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Build)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// parseWekaVersion extracts version from WEKA container image
// Supports formats like:
//   - quay.io/weka.io/weka-in-container:4.4.10.200
//   - weka/weka:4.2.5
//   - registry.example.com/weka:4.3.0.100
//   - quay.io/weka.io/weka:5.1.0.461-qa-alpha
func parseWekaVersion(image string) (*WekaVersion, error) {
	// Extract version from image tag (everything after the last ':')
	// Format: <registry>/<image>:<version>
	colonIndex := strings.LastIndex(image, ":")
	if colonIndex == -1 {
		return nil, fmt.Errorf("image does not contain version tag: %s", image)
	}

	versionStr := image[colonIndex+1:]

	// Remove any suffix after a dash (e.g., "-qa-alpha", "-rc1", "-dev")
	// This allows us to parse "5.1.0.461-qa-alpha" as "5.1.0.461"
	if dashIndex := strings.Index(versionStr, "-"); dashIndex != -1 {
		versionStr = versionStr[:dashIndex]
	}

	// Parse version components (e.g., "4.4.10.200" or "4.2.5")
	versionParts := strings.Split(versionStr, ".")
	if len(versionParts) < 3 {
		return nil, fmt.Errorf("invalid version format: %s (expected at least major.minor.patch)", versionStr)
	}

	version := &WekaVersion{Raw: versionStr}

	// Parse major version
	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version '%s': %w", versionParts[0], err)
	}
	version.Major = major

	// Parse minor version
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version '%s': %w", versionParts[1], err)
	}
	version.Minor = minor

	// Parse patch version
	patch, err := strconv.Atoi(versionParts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version '%s': %w", versionParts[2], err)
	}
	version.Patch = patch

	// Parse build version (optional)
	if len(versionParts) >= 4 {
		build, err := strconv.Atoi(versionParts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid build version '%s': %w", versionParts[3], err)
		}
		version.Build = build
	}

	return version, nil
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
	t.AppendHeader(table.Row{"TYPE", "COUNT", "CORES (HT ON)", "CORES (HT OFF)", "HUGEPAGES", "MEMORY"})

	for _, c := range containers {
		if c.Count == 0 {
			continue
		}
		t.AppendRow(table.Row{
			capitalizeFirst(c.Type),
			c.Count,
			c.Cores,
			c.CoresNoHT,
			fmt.Sprintf("%d MiB", c.Hugepages),
			fmt.Sprintf("%d MiB", c.Memory),
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

// formatSelector converts a label selector map to a string representation
func formatSelector(selector map[string]string) string {
	if len(selector) == 0 {
		return "(none)"
	}
	var parts []string
	for key, value := range selector {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
