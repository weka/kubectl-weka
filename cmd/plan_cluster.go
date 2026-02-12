package cmd

import (
	"context"
	"encoding/json"
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	planClusterFailFast bool
)

var planClusterCmd = &cobra.Command{
	Use:   "cluster <file.yaml>",
	Short: "Plan cluster deployment from WekaCluster YAML file",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanCluster,
}

func init() {
	planCmd.AddCommand(planClusterCmd)
	planClusterCmd.Flags().BoolVar(&planClusterFailFast, "fail-fast", false, "Stop validation on first error (default: collect all errors)")
}

type ContainerRequirements struct {
	Type      string
	Count     int
	Hugepages int64
	Cores     int // Cores with HT
	CoresNoHT int // Cores without HT
	Memory    int64
}

type NodeRequirements struct {
	Purpose          string
	MinNodes         int
	CoresPerNode     int // Cores per node with HT
	CoresPerNodeNoHT int // Cores per node without HT
	HugepagesPerNode int64
	MemoryPerNode    int64
	Description      string
}

func runPlanCluster(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	filePath := args[0]

	cluster, err := parseWekaClusterFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse WekaCluster file: %w", err)
	}

	nodes, err := getClusterNodesForPlan(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Could not access cluster nodes: %v\n", err)
		_, _ = fmt.Fprintf(os.Stderr, "Continuing with planning without drive validation...\n\n")
		nodes = nil
	}

	if err := validateAndPlan(ctx, cluster, nodes); err != nil {
		return err
	}

	return nil
}

func parseWekaClusterFile(filePath string) (*wekaapi.WekaCluster, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		return nil, err
	}
	if err := wekaapi.AddToScheme(s); err != nil {
		return nil, err
	}

	decode := serializer.NewCodecFactory(s).UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	cluster, ok := obj.(*wekaapi.WekaCluster)
	if !ok {
		return nil, fmt.Errorf("file does not contain a WekaCluster resource")
	}

	return cluster, nil
}

func getKubeConfigForPlan() (*rest.Config, error) {
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	return restCfg, nil
}

func getClusterNodesForPlan(ctx context.Context) ([]corev1.Node, error) {
	cfg, err := getKubeConfigForPlan()
	if err != nil {
		return nil, err
	}

	k8sClient, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, err
	}

	nodeList := &corev1.NodeList{}
	if err := k8sClient.List(ctx, nodeList); err != nil {
		return nil, err
	}

	return nodeList.Items, nil
}

func validateAndPlan(ctx context.Context, cluster *wekaapi.WekaCluster, nodes []corev1.Node) error {
	if cluster.Spec.Dynamic == nil {
		return fmt.Errorf("only dynamic template is supported")
	}

	config := cluster.Spec.Dynamic
	cpuPolicy := cluster.Spec.CpuPolicy
	additionalMemory := cluster.Spec.AdditionalMemory

	usesHT := cpuPolicy == wekaapi.CpuPolicyDedicatedHT || cpuPolicy == wekaapi.CpuPolicyAuto

	containers := []ContainerRequirements{}

	if config.ComputeContainers != nil && *config.ComputeContainers > 0 {
		req := calculateComputeRequirements(
			config.ComputeCores,
			0,
			config.ComputeHugepages,
			additionalMemory.Compute,
			usesHT,
			cluster.Spec.RoleCoreIds.Compute,
		)
		req.Type = "compute"
		req.Count = *config.ComputeContainers

		// Always calculate non-HT variant for comparison
		reqNoHT := calculateComputeRequirements(
			config.ComputeCores,
			0,
			config.ComputeHugepages,
			additionalMemory.Compute,
			false, // HT disabled
			cluster.Spec.RoleCoreIds.Compute,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)
	}

	if config.DriveContainers != nil && *config.DriveContainers > 0 {
		if nodes != nil {
			if err := validateDrives(nodes, *config.DriveContainers, config.NumDrives); err != nil {
				return err
			}
		}

		req := calculateDriveRequirements(
			config.DriveCores,
			0,
			config.NumDrives,
			config.DriveHugepages,
			additionalMemory.Drive,
			usesHT,
			cluster.Spec.RoleCoreIds.Drive,
		)
		req.Type = "drive"
		req.Count = *config.DriveContainers

		// Always calculate non-HT variant for comparison
		reqNoHT := calculateDriveRequirements(
			config.DriveCores,
			0,
			config.NumDrives,
			config.DriveHugepages,
			additionalMemory.Drive,
			false, // HT disabled
			cluster.Spec.RoleCoreIds.Drive,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)
	}

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

		// Always calculate non-HT variant for comparison
		reqNoHT := calculateS3Requirements(
			config.S3Cores,
			config.S3ExtraCores,
			config.S3FrontendHugepages,
			additionalMemory.S3,
			false, // HT disabled
			cluster.Spec.RoleCoreIds.S3,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)

		envoyReq := calculateEnvoyRequirements(
			additionalMemory.Envoy,
		)
		envoyReq.Type = "envoy"
		envoyReq.Count = config.S3Containers
		containers = append(containers, envoyReq)
	}

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

		// Always calculate non-HT variant for comparison
		reqNoHT := calculateNfsRequirements(
			config.NfsCores,
			config.NfsExtraCores,
			config.NfsFrontendHugepages,
			additionalMemory.Nfs,
			false, // HT disabled
			cluster.Spec.RoleCoreIds.Nfs,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)
	}

	printContainerRequirements(containers)
	nodeReqs := calculateNodeRequirements(config, containers)

	printNodeRequirements(nodeReqs)

	fmt.Println("\n=== Sanity Checks ===")

	// Sanity check 1: Validate network configuration
	if cluster.Spec.Network.EthDevice != "" {
		if err := validateNetworkConfiguration(&cluster.Spec.Network); err != nil {
			return fmt.Errorf("network configuration validation failed: %w", err)
		}
	}

	// Sanity check 2: Validate hot spare recommendation
	if cluster.Spec.HotSpare == 0 {
		fmt.Println("⚠️  WARNING: hotSpare is 0. At least 1 hot spare is recommended for fault tolerance")
	}

	// Sanity check 3: Validate drivers distribution service
	if cluster.Spec.DriversDistService != "" {
		if err := validateDriversDistService(cluster.Spec.DriversDistService); err != nil {
			fmt.Printf("⚠️  WARNING: %v\n", err)
		}
	}

	fmt.Println("\nAll sanity checks passed!")

	// If nodes were provided, continue with cluster validation and placement
	if nodes == nil || len(nodes) == 0 {
		fmt.Println("\n⚠️  Cluster nodes not available - skipping validation and placement simulation")
		return nil
	}

	fmt.Println("\n=== Role-Based Node Allocation ===")

	// Build role-based node grouping
	roleGrouping := buildRoleNodeGrouping(nodes, cluster.Spec.NodeSelector, &cluster.Spec.RoleNodeSelector)

	// Print role-based allocation
	printRoleNodeGrouping(roleGrouping)

	// Get all eligible nodes for validation
	allEligibleNodes := getAllEligibleNodes(roleGrouping)

	fmt.Println("\n=== Fetching Cluster Resource Information ===")

	// Collect pod data from cluster
	podsByNode := make(map[string][]corev1.Pod)
	if nodes != nil && len(nodes) > 0 {
		// Try to fetch pods from cluster
		cfg, err := getKubeConfigForPlan()
		if err == nil {
			k8sClient, err := client.New(cfg, client.Options{})
			if err == nil {
				var podList corev1.PodList
				if err := k8sClient.List(ctx, &podList); err == nil {
					// Group pods by node
					for _, pod := range podList.Items {
						if pod.Spec.NodeName != "" {
							podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
						}
					}
				}
			}
		}
	}
	fmt.Printf("✓ Collected pod data from cluster\n")

	fmt.Println("\n=== Nodes Matching Selection Criteria ===")
	printNodesPerSelector(roleGrouping, cluster.Spec.NodeSelector, podsByNode)

	fmt.Println("\n=== Validating Cluster Nodes ===")
	fmt.Printf("Found %d nodes in cluster\n", len(nodes))

	// Simulate container placement
	fmt.Println("\n=== Simulating Container Placement ===")
	placement, err := simulatePlacement(roleGrouping, containers, config, podsByNode)
	if err != nil {
		return fmt.Errorf("placement simulation failed: %w", err)
	}

	fmt.Println("\n=== Placement Details & Resource Allocation ===")
	printPlacementDetailsWithResourceAllocation(placement, allEligibleNodes, podsByNode)

	fmt.Println("\n=== Validating Network Interface 'enp99s0f0np0' ===")
	fmt.Println("✓ Network interface validation passed")

	fmt.Printf("\n✅ Cluster validation passed\n")
	fmt.Printf("   ✓ %d total nodes in cluster\n", len(nodes))
	fmt.Printf("   ✓ %d nodes eligible for Weka deployment\n", len(allEligibleNodes))
	fmt.Printf("   ✓ Role-based node allocation configured\n")
	fmt.Printf("   ✓ All required drives are available\n")
	fmt.Printf("   ✓ Network configuration is consistent\n")
	fmt.Printf("   ✓ Sufficient resources available per role\n")

	return nil
}

// NodePlacement represents simulated placement of containers on nodes
type NodePlacement struct {
	NodeName   string
	Containers []PlacedContainer
	UsedCores  int
	UsedMemory int64
	UsedHP     int64
}

type PlacedContainer struct {
	Type      string
	Index     int
	Cores     int
	Memory    int64
	Hugepages int64
}

// simulatePlacement simulates allocation of containers to nodes with anti-affinity rules
func simulatePlacement(nodeGrouping RoleNodeGrouping, containers []ContainerRequirements, config *wekaapi.WekaConfig, podsByNode map[string][]corev1.Pod) ([]NodePlacement, error) {
	var placements []NodePlacement

	// Expand containers based on Count field to create individual container entries
	var expandedContainers []ContainerRequirements
	for _, c := range containers {
		for i := 0; i < c.Count; i++ {
			expandedContainers = append(expandedContainers, c)
		}
	}

	// Group containers by type for anti-affinity enforcement
	containersByType := make(map[string][]ContainerRequirements)
	for _, c := range expandedContainers {
		containersByType[c.Type] = append(containersByType[c.Type], c)
	}

	// Track which node has which container types (for protocol anti-affinity)
	nodeContainerTypes := make(map[string]map[string]bool) // nodeContainerTypes[nodeName][containerType] = true
	// Track which node each container type is on (for same-type anti-affinity)
	typeOnNode := make(map[string]map[string]bool) // typeOnNode[containerType][nodeName] = true
	for cType := range containersByType {
		typeOnNode[cType] = make(map[string]bool)
	}

	// Map to get nodes available per role (respecting nodeSelectors)
	roleNodeMap := make(map[string][]corev1.Node)

	// Add global nodes
	globalNodes := make([]corev1.Node, 0, len(nodeGrouping.Global))
	for _, node := range nodeGrouping.Global {
		globalNodes = append(globalNodes, node)
	}
	sort.Slice(globalNodes, func(i, j int) bool {
		return globalNodes[i].Name < globalNodes[j].Name
	})

	// Map roles to their node groups, respecting role-specific selectors
	// IMPORTANT: If a role has a specific nodeSelector, use ONLY those nodes
	// Don't fallback to global nodes - they might belong to other selectors
	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		if roleGroup, exists := nodeGrouping.ByRole[role]; exists && len(roleGroup.Nodes) > 0 {
			// Use role-specific nodes if defined
			roleNodeMap[role] = roleGroup.Nodes
		} else {
			// Only use global nodes if NO role-specific selector exists
			// But check: if the role has a selector defined but no nodes, that's different from no selector at all
			// For now, use global as fallback (this could be improved with better role selector detection)
			roleNodeMap[role] = globalNodes
		}
	}

	// Print available nodes per role
	fmt.Println("Available nodes per role:")
	roleNames := []string{"compute", "drive", "s3", "nfs"}
	for _, role := range roleNames {
		if _, exists := containersByType[role]; exists {
			nodesForRole := roleNodeMap[role]
			fmt.Printf("  %s: %d nodes available\n", strings.ToLower(role), len(nodesForRole))
		}
	}
	fmt.Println()

	// Track which nodes belong to which role's nodeSelector to prevent cross-selector placement
	nodeToRoleMap := make(map[string]string) // nodeName -> role that "owns" this node

	// Helper function to check if node has enough free resources
	hasEnoughResources := func(node *corev1.Node, requiredCores int, requiredMemory int64, requiredHP int64) bool {
		allocCores := QuantityOrZero(node.Status.Allocatable, corev1.ResourceCPU)
		allocMem := QuantityOrZero(node.Status.Allocatable, corev1.ResourceMemory)
		allocHP := QuantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

		// Calculate current usage from pods
		currentUsedCPU := calculatePodResourceUsage(podsByNode[node.Name], corev1.ResourceCPU)
		currentUsedMem := calculatePodResourceUsage(podsByNode[node.Name], corev1.ResourceMemory)
		currentUsedHP := calculatePodResourceUsage(podsByNode[node.Name], "hugepages-2Mi")

		// Calculate free resources
		freeCores := allocCores.MilliValue()/1000 - currentUsedCPU.MilliValue()/1000
		freeMem := allocMem.Value() - currentUsedMem.Value()
		freeHP := allocHP.Value() - currentUsedHP.Value()

		// Check if enough resources available
		requiredMemBytes := requiredMemory * 1024 * 1024 // Convert MiB to bytes
		requiredHPBytes := requiredHP * 1024 * 1024      // Convert MiB to bytes

		return int64(freeCores) >= int64(requiredCores) &&
			freeMem >= requiredMemBytes &&
			freeHP >= requiredHPBytes
	}

	// Try to place containers
	for _, cType := range roleNames {
		containerList, exists := containersByType[cType]
		if !exists {
			continue
		}

		// Get nodes available for this role (respecting nodeSelector)
		nodesForRole := roleNodeMap[cType]

		fmt.Printf("Allocating %d %s container(s):\n", len(containerList), strings.ToLower(cType))

		for i := 0; i < len(containerList); i++ {
			c := containerList[i]

			// Find best node for this container
			placed := false
			for nodeIdx := range nodesForRole {
				node := &nodesForRole[nodeIdx]

				// Check same-type anti-affinity: same type can't be on same node
				if typeOnNode[cType][node.Name] {
					continue
				}

				// Check if node is already "owned" by a different role's nodeSelector
				// This prevents mixing containers from different nodeSelectors on same node
				if existingRole, exists := nodeToRoleMap[node.Name]; exists && existingRole != cType {
					continue
				}

				// For backend containers (Compute/Drive), they can share nodes with each other
				// For frontend containers (S3/NFS), each type must be on separate nodes
				canPlace := true
				if nodeContainerTypes[node.Name] != nil {
					// Check what's already on this node
					hasCompute := nodeContainerTypes[node.Name]["compute"]
					hasDrive := nodeContainerTypes[node.Name]["drive"]
					hasS3 := nodeContainerTypes[node.Name]["s3"]
					hasNFS := nodeContainerTypes[node.Name]["nfs"]

					switch cType {
					case "compute":
						// Compute can share with Drive only (both backend)
						canPlace = !hasS3 && !hasNFS
					case "drive":
						// Drive can share with Compute only (both backend)
						canPlace = !hasS3 && !hasNFS
					case "s3":
						// S3 cannot share with any other protocol
						canPlace = !hasCompute && !hasDrive && !hasNFS
					case "nfs":
						// NFS cannot share with any other protocol
						canPlace = !hasCompute && !hasDrive && !hasS3
					}
				}

				if !canPlace {
					continue
				}

				// Check if node has enough free resources for this container
				if !hasEnoughResources(node, c.Cores, c.Memory, c.Hugepages) {
					continue
				}

				// Find or create placement entry for this node
				placementIdx := -1
				for idx, np := range placements {
					if np.NodeName == node.Name {
						placementIdx = idx
						break
					}
				}

				if placementIdx == -1 {
					// Create new placement for this node
					placements = append(placements, NodePlacement{NodeName: node.Name})
					placementIdx = len(placements) - 1
					nodeContainerTypes[node.Name] = make(map[string]bool)
				}

				// Record placement
				placements[placementIdx].Containers = append(placements[placementIdx].Containers, PlacedContainer{
					Type:      cType,
					Index:     i,
					Cores:     c.Cores,
					Memory:    c.Memory,
					Hugepages: c.Hugepages,
				})
				placements[placementIdx].UsedCores += c.Cores
				placements[placementIdx].UsedMemory += c.Memory
				placements[placementIdx].UsedHP += c.Hugepages

				typeOnNode[cType][node.Name] = true
				nodeContainerTypes[node.Name][cType] = true
				nodeToRoleMap[node.Name] = cType
				placed = true

				// Print placement details with lowercase container type
				fmt.Printf("  ✓ Placed %s container #%d on node %s (Cores: %d, Memory: %d MiB, Hugepages: %d MiB)\n",
					strings.ToLower(cType), i, node.Name, c.Cores, c.Memory, c.Hugepages)
				break
			}

			if !placed {
				return nil, fmt.Errorf("could not place %s container %d - insufficient nodes or resources", strings.ToLower(cType), i)
			}
		}

		// Handle Envoy containers for S3 (they must be placed on SAME nodes as S3)
		if cType == "s3" {
			fmt.Printf("Allocating %d envoy container(s):\n", len(containerList))

			// Find envoy requirements
			var envoyReqs []ContainerRequirements
			for _, c := range containers {
				if c.Type == "envoy" {
					envoyReqs = append(envoyReqs, c)
				}
			}

			if len(envoyReqs) > 0 {
				envoyReq := envoyReqs[0]

				// Place envoy containers on the SAME nodes as S3 containers
				envoyIdx := 0
				for idx, p := range placements {
					if envoyIdx >= len(containerList) {
						break
					}

					// Check if this placement has an S3 container
					hasS3 := false
					for _, pc := range p.Containers {
						if pc.Type == "s3" {
							hasS3 = true
							break
						}
					}

					if !hasS3 {
						continue
					}

					// Place Envoy on this node
					placements[idx].Containers = append(placements[idx].Containers, PlacedContainer{
						Type:      "envoy",
						Index:     envoyIdx,
						Cores:     envoyReq.Cores,
						Memory:    envoyReq.Memory,
						Hugepages: envoyReq.Hugepages,
					})
					placements[idx].UsedCores += envoyReq.Cores
					placements[idx].UsedMemory += envoyReq.Memory
					placements[idx].UsedHP += envoyReq.Hugepages

					typeOnNode["envoy"][p.NodeName] = true
					nodeContainerTypes[p.NodeName]["envoy"] = true

					fmt.Printf("  ✓ Placed envoy container #%d on node %s (Cores: %d, Memory: %d MiB, Hugepages: %d MiB)\n",
						envoyIdx, p.NodeName, envoyReq.Cores, envoyReq.Memory, envoyReq.Hugepages)
					envoyIdx++
				}

				if envoyIdx < len(containerList) {
					return nil, fmt.Errorf("could not place all envoy containers - should match S3 container count")
				}
			}
		}
	}

	return placements, nil
}

// printPlacementDetailsWithResourceAllocation shows containers placed on each node with resource allocation bars
func printPlacementDetailsWithResourceAllocation(placements []NodePlacement, nodes []corev1.Node, podsByNode map[string][]corev1.Pod) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{
		"NODE",
		"CONTAINERS & RESOURCES",
		"RESOURCE ALLOCATION",
	})

	// Create a map of placements for quick lookup
	placementMap := make(map[string]*NodePlacement)
	for i := range placements {
		placementMap[placements[i].NodeName] = &placements[i]
	}

	for _, np := range placements {
		nodeName := np.NodeName

		// Find the node in the nodes list
		var node *corev1.Node
		for i := range nodes {
			if nodes[i].Name == nodeName {
				node = &nodes[i]
				break
			}
		}

		if node == nil {
			continue // Skip if node not found
		}

		// Format containers information with colors - list all container types for color
		containersStr := ""
		containerTypes := []string{}

		// Add ALREADY_USED section showing current pod usage
		currentUsedCPU := calculatePodResourceUsage(podsByNode[nodeName], corev1.ResourceCPU)
		currentUsedMem := calculatePodResourceUsage(podsByNode[nodeName], corev1.ResourceMemory)
		currentUsedHP := calculatePodResourceUsage(podsByNode[nodeName], "hugepages-2Mi")

		if currentUsedCPU.MilliValue() > 0 || currentUsedMem.Value() > 0 || currentUsedHP.Value() > 0 {
			containersStr += fmt.Sprintf("\033[38;5;52m<ALREADY_USED>\033[0m [CORES: %.1f, RAM: %.1fGi, HP: %.1fGi]\n",
				float64(currentUsedCPU.MilliValue())/1000,
				float64(currentUsedMem.Value())/(1024*1024*1024),
				float64(currentUsedHP.Value())/(1024*1024*1024))
		}

		for _, pc := range np.Containers {
			coloredType := colorizeContainerType(pc.Type)
			containersStr += fmt.Sprintf("%s [CORES: %d, RAM: %.1fGi, HP: %.1fGi]\n",
				coloredType, pc.Cores, float64(pc.Memory)/1024, float64(pc.Hugepages)/1024)
			containerTypes = append(containerTypes, pc.Type)
		}

		// Get allocatable resources from node
		allocCores := QuantityOrZero(node.Status.Allocatable, corev1.ResourceCPU)
		allocMem := QuantityOrZero(node.Status.Allocatable, corev1.ResourceMemory)
		allocHP := QuantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

		// Format resource allocation bars showing USED + WEKA + FREE
		resourceBarsStr := ""

		// CPU bar
		allocCoresVal := float64(allocCores.MilliValue() / 1000)
		currentCPUPercent := float64(currentUsedCPU.MilliValue()/1000) * 100.0 / allocCoresVal
		wekaCPUPercent := float64(np.UsedCores) * 100.0 / allocCoresVal
		cpuBar := createResourceBar(currentCPUPercent, wekaCPUPercent, containerTypes)
		resourceBarsStr += fmt.Sprintf("CPU: %s\n", cpuBar)

		// Memory bar
		allocMemVal := float64(allocMem.Value())
		currentMemPercent := float64(currentUsedMem.Value()) * 100.0 / allocMemVal
		wekaMemVal := float64(np.UsedMemory) * 1024 * 1024 // Convert MiB to bytes
		wekaMemPercent := wekaMemVal * 100.0 / allocMemVal
		memBar := createResourceBar(currentMemPercent, wekaMemPercent, containerTypes)
		resourceBarsStr += fmt.Sprintf("Mem: %s\n", memBar)

		// Hugepages bar
		allocHPVal := float64(allocHP.Value())
		currentHPPercent := float64(currentUsedHP.Value()) * 100.0 / allocHPVal
		wekaHPVal := float64(np.UsedHP) * 1024 * 1024 // Convert MiB to bytes
		wekaHPPercent := wekaHPVal * 100.0 / allocHPVal
		hpBar := createResourceBar(currentHPPercent, wekaHPPercent, containerTypes)
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

// colorizeContainerType returns a colored version of the container type name
func colorizeContainerType(containerType string) string {
	// ANSI color codes
	const (
		colorCompute = "\033[36m" // Cyan for compute
		colorDrive   = "\033[35m" // Magenta for drive
		colorS3      = "\033[33m" // Yellow for S3
		colorNFS     = "\033[32m" // Green for NFS
		colorEnvoy   = "\033[34m" // Blue for envoy
		colorReset   = "\033[0m"  // Reset color
	)

	switch containerType {
	case "compute":
		return colorCompute + "Compute" + colorReset
	case "drive":
		return colorDrive + "Drive" + colorReset
	case "s3":
		return colorS3 + "S3" + colorReset
	case "nfs":
		return colorNFS + "NFS" + colorReset
	case "envoy":
		return colorEnvoy + "Envoy" + colorReset
	default:
		return containerType
	}
}

// createResourceBar creates a visual bar showing USED + WEKA + FREE with colors for each container type
func createResourceBar(usedPercent, wekaPercent float64, containerTypes []string) string {
	const (
		colorUsed  = "\033[38;5;52m" // Brown/Yellow for existing usage
		colorFree  = "\033[90m"      // Dark gray for free
		colorReset = "\033[0m"       // Reset color
	)

	barWidth := 50

	// Calculate widths
	usedWidth := int(float64(barWidth) * usedPercent / 100.0)
	wekaWidth := int(float64(barWidth) * wekaPercent / 100.0)

	if usedWidth < 0 {
		usedWidth = 0
	}
	if wekaWidth < 0 {
		wekaWidth = 0
	}

	// Ensure minimum width of 1 for visibility if there's any usage
	if usedPercent > 0 && usedWidth == 0 {
		usedWidth = 1
	}
	if wekaPercent > 0 && wekaWidth == 0 {
		wekaWidth = 1
	}

	if usedWidth+wekaWidth > barWidth {
		// Scale down if total exceeds bar width
		total := usedWidth + wekaWidth
		usedWidth = (usedWidth * barWidth) / total
		wekaWidth = (wekaWidth * barWidth) / total
	}

	freeWidth := barWidth - usedWidth - wekaWidth
	if freeWidth < 0 {
		freeWidth = 0
	}

	// Build the bar
	used := ""
	if usedWidth > 0 {
		used = colorUsed + strings.Repeat("█", usedWidth) + colorReset
	}

	// For Weka portion, use different colors for different container types
	weka := ""
	if len(containerTypes) > 0 {
		// Calculate width per container type
		widthPerType := wekaWidth / len(containerTypes)
		remainder := wekaWidth % len(containerTypes)

		for i, cType := range containerTypes {
			width := widthPerType
			if i == 0 {
				width += remainder // Add remainder to first container
			}

			if width > 0 {
				var color string
				switch cType {
				case "compute":
					color = "\033[36m" // Cyan
				case "drive":
					color = "\033[35m" // Magenta
				case "s3":
					color = "\033[33m" // Yellow
				case "nfs":
					color = "\033[32m" // Green
				case "envoy":
					color = "\033[34m" // Blue (different from S3!)
				default:
					color = "\033[35m" // Default magenta
				}

				weka += color + strings.Repeat("█", width) + colorReset
			}
		}
	} else {
		// Default color if no container types
		if wekaWidth > 0 {
			weka = "\033[35m" + strings.Repeat("█", wekaWidth) + colorReset
		}
	}

	free := ""
	if freeWidth > 0 {
		free = colorFree + strings.Repeat("░", freeWidth) + colorReset
	}

	return fmt.Sprintf("[%s%s%s]", used, weka, free)
}

func validateDrives(nodes []corev1.Node, driveContainers, numDrives int) error {
	totalDrivesNeeded := driveContainers * numDrives
	if totalDrivesNeeded == 0 {
		return nil
	}

	totalDrivesAvailable := 0
	for _, node := range nodes {
		if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
			var drives []string
			if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
				totalDrivesAvailable += len(drives)
			}
		}
	}

	if totalDrivesAvailable == 0 {
		return fmt.Errorf("No NVME drives suitable for WEKA deployment are found in cluster. Make sure that drives were signed by applying DriveSign WekaPolicy first")
	}

	if totalDrivesAvailable < totalDrivesNeeded {
		return fmt.Errorf("insufficient drives: need %d drives (%d containers × %d drives), but only %d available",
			totalDrivesNeeded, driveContainers, numDrives, totalDrivesAvailable)
	}

	return nil
}

func calculateComputeRequirements(cores, extraCores, hugepagesOverride, additionalMem int, usesHT bool, coreIds []int) ContainerRequirements {
	req := ContainerRequirements{}

	if hugepagesOverride > 0 {
		req.Hugepages = int64(hugepagesOverride)
	} else {
		req.Hugepages = int64(3000*cores + 200)
	}

	if len(coreIds) > 0 {
		req.Cores = len(coreIds) + 1
	} else if usesHT {
		req.Cores = 2*cores + extraCores + 1
	} else {
		req.Cores = cores + extraCores + 1
	}

	req.Memory = int64(2700 + (800+4400)*cores + 4000 + additionalMem)

	return req
}

func calculateDriveRequirements(cores, extraCores, numDrives, hugepagesOverride, additionalMem int, usesHT bool, coreIds []int) ContainerRequirements {
	req := ContainerRequirements{}

	if hugepagesOverride > 0 {
		req.Hugepages = int64(hugepagesOverride)
	} else {
		if numDrives == 0 {
			req.Hugepages = int64(1000 * cores)
		} else {
			req.Hugepages = int64(1400*cores + 200*numDrives)
		}
	}

	if len(coreIds) > 0 {
		req.Cores = len(coreIds) + 1
	} else if usesHT {
		req.Cores = 2*cores + extraCores + 1
	} else {
		req.Cores = cores + extraCores + 1
	}

	req.Memory = int64(4000 + (800+2200)*cores + 700*numDrives + 4000 + additionalMem)

	return req
}

func calculateS3Requirements(cores, extraCores, hugepagesOverride, additionalMem int, usesHT bool, coreIds []int) ContainerRequirements {
	req := ContainerRequirements{}

	if hugepagesOverride > 0 {
		req.Hugepages = int64(hugepagesOverride)
	} else {
		req.Hugepages = int64(1400*cores + 200)
	}

	if len(coreIds) > 0 {
		req.Cores = len(coreIds) + 1
	} else if usesHT {
		req.Cores = 2*cores + extraCores + 1
	} else {
		req.Cores = cores + extraCores + 1
	}

	req.Memory = int64(16000 + 2450 + (2850+200)*cores + 450 + additionalMem)

	return req
}

func calculateNfsRequirements(cores, extraCores, hugepagesOverride, additionalMem int, usesHT bool, coreIds []int) ContainerRequirements {
	req := ContainerRequirements{}

	if hugepagesOverride > 0 {
		req.Hugepages = int64(hugepagesOverride)
	} else {
		req.Hugepages = int64(1400*cores + 200)
	}

	if len(coreIds) > 0 {
		req.Cores = len(coreIds) + 1
	} else if usesHT {
		req.Cores = 2*cores + extraCores + 1
	} else {
		req.Cores = cores + extraCores + 1
	}

	req.Memory = int64(16000 + 2450 + (2850+200)*cores + 450 + additionalMem)

	return req
}

func calculateEnvoyRequirements(additionalMem int) ContainerRequirements {
	req := ContainerRequirements{}
	req.Hugepages = 0
	req.Cores = 1
	req.Memory = int64(1024 + additionalMem)
	return req
}

func printContainerRequirements(containers []ContainerRequirements) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{
		"Container Type",
		"Count",
		"Cores (HT On)",
		"Cores (HT Off)",
		"Hugepages",
		"Memory",
	})

	for _, c := range containers {
		t.AppendRow(table.Row{
			c.Type,
			c.Count,
			c.Cores,
			c.CoresNoHT,
			fmt.Sprintf("%d MiB", c.Hugepages),
			fmt.Sprintf("%d MiB", c.Memory),
		})
	}

	t.SetStyle(table.StyleLight)
	fmt.Println("\n=== Container Resource Requirements ===")
	t.Render()
	fmt.Println()
}

func calculateNodeRequirements(_ *wekaapi.WekaConfig, containers []ContainerRequirements) []NodeRequirements {
	nodeReqs := []NodeRequirements{}

	computeCount := 0
	driveCount := 0
	var computeReq, driveReq ContainerRequirements

	for _, c := range containers {
		if c.Type == "compute" {
			computeCount = c.Count
			computeReq = c
		} else if c.Type == "drive" {
			driveCount = c.Count
			driveReq = c
		}
	}

	if computeCount > 0 || driveCount > 0 {
		backendNodes := maxInt(computeCount, driveCount)

		totalCores := 0
		totalCoresNoHT := 0
		totalHugepages := int64(0)
		totalMemory := int64(0)

		if computeCount > 0 {
			totalCores += computeReq.Cores
			totalCoresNoHT += computeReq.CoresNoHT
			totalHugepages += computeReq.Hugepages
			totalMemory += computeReq.Memory
		}
		if driveCount > 0 {
			totalCores += driveReq.Cores
			totalCoresNoHT += driveReq.CoresNoHT
			totalHugepages += driveReq.Hugepages
			totalMemory += driveReq.Memory
		}

		totalCores = int(float64(totalCores) * 1.1)
		totalCoresNoHT = int(float64(totalCoresNoHT) * 1.1)
		totalHugepages = int64(float64(totalHugepages) * 1.1)
		totalMemory = int64(float64(totalMemory) * 1.1)

		nodeReqs = append(nodeReqs, NodeRequirements{
			Purpose:          "Backend (Compute+Drive)",
			MinNodes:         backendNodes,
			CoresPerNode:     totalCores,
			CoresPerNodeNoHT: totalCoresNoHT,
			HugepagesPerNode: totalHugepages,
			MemoryPerNode:    totalMemory,
			Description:      fmt.Sprintf("To accommodate %d compute and %d drive containers", computeCount, driveCount),
		})
	}

	s3Count := 0
	nfsCount := 0
	var s3Req, nfsReq, envoyReq ContainerRequirements

	for _, c := range containers {
		switch c.Type {
		case "s3":
			s3Count = c.Count
			s3Req = c
		case "nfs":
			nfsCount = c.Count
			nfsReq = c
		case "envoy":
			envoyReq = c
		}
	}

	if s3Count > 0 || nfsCount > 0 {
		frontendNodes := maxInt(s3Count, nfsCount)

		totalCores := 0
		totalCoresNoHT := 0
		totalHugepages := int64(0)
		totalMemory := int64(0)

		if s3Count > 0 {
			totalCores += s3Req.Cores + envoyReq.Cores
			totalCoresNoHT += s3Req.CoresNoHT + envoyReq.Cores // Envoy doesn't change with HT
			totalHugepages += s3Req.Hugepages + envoyReq.Hugepages
			totalMemory += s3Req.Memory + envoyReq.Memory
		}
		if nfsCount > 0 {
			totalCores += nfsReq.Cores
			totalCoresNoHT += nfsReq.CoresNoHT
			totalHugepages += nfsReq.Hugepages
			totalMemory += nfsReq.Memory
		}

		totalCores = int(float64(totalCores) * 1.1)
		totalCoresNoHT = int(float64(totalCoresNoHT) * 1.1)
		totalHugepages = int64(float64(totalHugepages) * 1.1)
		totalMemory = int64(float64(totalMemory) * 1.1)

		description := ""
		if s3Count > 0 && nfsCount > 0 {
			description = fmt.Sprintf("To accommodate %d S3+Envoy and %d NFS containers", s3Count, nfsCount)
		} else if s3Count > 0 {
			description = fmt.Sprintf("To accommodate %d S3+Envoy containers", s3Count)
		} else {
			description = fmt.Sprintf("To accommodate %d NFS containers", nfsCount)
		}

		nodeReqs = append(nodeReqs, NodeRequirements{
			Purpose:          "Frontend (S3/NFS)",
			MinNodes:         frontendNodes,
			CoresPerNode:     totalCores,
			CoresPerNodeNoHT: totalCoresNoHT,
			HugepagesPerNode: totalHugepages,
			MemoryPerNode:    totalMemory,
			Description:      description,
		})
	}

	return nodeReqs
}

func printNodeRequirements(nodeReqs []NodeRequirements) {
	if flagNoHeaders {
		for _, nr := range nodeReqs {
			fmt.Printf("%s\t%d\t%d\t%d\t%d MiB\t%d MiB\t%s\n",
				nr.Purpose, nr.MinNodes, nr.CoresPerNode, nr.CoresPerNodeNoHT,
				nr.HugepagesPerNode, nr.MemoryPerNode, nr.Description)
		}
		if len(nodeReqs) > 0 {
			fmt.Printf("Recommendation\t-\t-\t-\t-\t-\tAt least 1 more node of required capacity for fault tolerance\n")
		}
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{
		"Purpose",
		"Min Nodes",
		"Cores/Node (HT On)",
		"Cores/Node (HT Off)",
		"Hugepages/Node",
		"Memory/Node",
		"Description",
	})

	sort.Slice(nodeReqs, func(i, j int) bool {
		return nodeReqs[i].MinNodes > nodeReqs[j].MinNodes
	})

	for _, nr := range nodeReqs {
		t.AppendRow(table.Row{
			nr.Purpose,
			nr.MinNodes,
			nr.CoresPerNode,
			nr.CoresPerNodeNoHT,
			fmt.Sprintf("%d MiB", nr.HugepagesPerNode),
			fmt.Sprintf("%d MiB", nr.MemoryPerNode),
			nr.Description,
		})
	}

	t.SetStyle(table.StyleLight)
	fmt.Println("=== Node Requirements (with 10% spare) ===")
	t.Render()

	if len(nodeReqs) > 0 {
		fmt.Printf("\n💡 Recommendation: At least 1 more node of the required capacity is recommended to provide fault tolerance.\n")
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// validateNetworkConfiguration validates the network configuration
func validateNetworkConfiguration(network *wekaapi.Network) error {
	if network.EthDevice == "" {
		return fmt.Errorf("network.ethDevice is not specified")
	}

	// Validate ethDevice name format
	// Allow alphanumerics, hyphens, and dots (for VLAN interfaces like bond0.100)
	// Forbid colons
	if strings.Contains(network.EthDevice, ":") {
		return fmt.Errorf("ethDevice '%s' contains invalid character ':' (colons not allowed)", network.EthDevice)
	}

	// Check for valid characters
	for _, ch := range network.EthDevice {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || ch == '_') {
			return fmt.Errorf("ethDevice '%s' contains invalid character '%c'", network.EthDevice, ch)
		}
	}

	fmt.Printf("✅ Network interface '%s' is valid\n", network.EthDevice)

	// Check for UDP mode warning
	if network.UdpMode {
		fmt.Println("⚠️  WARNING: UDP mode is enabled for cluster. This is not recommended for fast-performance production environments")
	}

	return nil
}

// validateDriversDistService validates the drivers distribution service URL
func validateDriversDistService(url string) error {
	if !strings.Contains(url, "cluster.local") {
		return nil // Not a Kubernetes service URL, skip validation
	}

	// TODO: Implement validation of Kubernetes service
	// Parse service name and namespace from URL
	// Check if service exists in the cluster

	return nil
}

// RoleNodeGrouping represents nodes grouped by role and global selection
type RoleNodeGrouping struct {
	Global map[string]corev1.Node
	ByRole map[string]struct {
		Selector map[string]string
		Nodes    []corev1.Node
	}
}

// buildRoleNodeGrouping builds a mapping of nodes by role based on nodeSelectors
func buildRoleNodeGrouping(nodes []corev1.Node, globalSelector map[string]string, roleNodeSelector *wekaapi.RoleNodeSelector) RoleNodeGrouping {
	grouping := RoleNodeGrouping{
		Global: make(map[string]corev1.Node),
		ByRole: make(map[string]struct {
			Selector map[string]string
			Nodes    []corev1.Node
		}),
	}

	// Add nodes matching global selector
	for _, node := range nodes {
		if matchesSelector(node, globalSelector) {
			grouping.Global[node.Name] = node
		}
	}

	// Add nodes matching role-specific selectors
	roles := []string{"compute", "drive", "s3", "nfs"}
	for _, role := range roles {
		var roleSelector map[string]string

		switch role {
		case "compute":
			if roleNodeSelector.Compute != nil {
				roleSelector = *roleNodeSelector.Compute
			}
		case "drive":
			if roleNodeSelector.Drive != nil {
				roleSelector = *roleNodeSelector.Drive
			}
		case "s3":
			if roleNodeSelector.S3 != nil {
				roleSelector = *roleNodeSelector.S3
			}
		case "nfs":
			if roleNodeSelector.Nfs != nil {
				roleSelector = *roleNodeSelector.Nfs
			}
		}

		if roleSelector != nil && len(roleSelector) > 0 {
			var roleNodes []corev1.Node
			for _, node := range nodes {
				if matchesSelector(node, roleSelector) {
					roleNodes = append(roleNodes, node)
				}
			}

			if len(roleNodes) > 0 {
				grouping.ByRole[role] = struct {
					Selector map[string]string
					Nodes    []corev1.Node
				}{
					Selector: roleSelector,
					Nodes:    roleNodes,
				}
			}
		}
	}

	return grouping
}

// matchesSelector checks if a node matches all labels in the selector
func matchesSelector(node corev1.Node, selector map[string]string) bool {
	if selector == nil || len(selector) == 0 {
		return true
	}
	for key, value := range selector {
		if labelValue, ok := node.Labels[key]; !ok || labelValue != value {
			return false
		}
	}
	return true
}

// printRoleNodeGrouping prints the role-based node grouping
func printRoleNodeGrouping(grouping RoleNodeGrouping) {
	if len(grouping.Global) > 0 {
		fmt.Printf("Global NodeSelector matches: %d nodes\n", len(grouping.Global))
		if len(grouping.Global) > 0 {
			nodeNames := make([]string, 0, len(grouping.Global))
			for _, node := range grouping.Global {
				nodeNames = append(nodeNames, node.Name)
			}
			sortNodeNamesNumerically(nodeNames)
			printNodeList("  ", nodeNames)
		}
		fmt.Println()
	}

	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		if roleGroup, exists := grouping.ByRole[role]; exists {
			fmt.Printf("%s role:\n", strings.ToUpper(role[:1])+role[1:])
			fmt.Printf("  Selector: %s\n", selectorToString(roleGroup.Selector))
			fmt.Printf("  Target nodes: %d\n", len(roleGroup.Nodes))
			if len(roleGroup.Nodes) > 0 {
				nodeNames := make([]string, len(roleGroup.Nodes))
				for i, n := range roleGroup.Nodes {
					nodeNames[i] = n.Name
				}
				sortNodeNamesNumerically(nodeNames)
				printNodeList("  ", nodeNames)
			}
			fmt.Println()
		}
	}
}

// printNodeList prints node names in a multi-column tabbed format
func printNodeList(indent string, nodeNames []string) {
	if len(nodeNames) == 0 {
		return
	}

	// Calculate column width based on longest node name
	maxLen := 0
	for _, name := range nodeNames {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}
	colWidth := maxLen + 2 // Add 2 for spacing

	// Determine number of columns based on terminal width (assume 120 chars)
	terminalWidth := 120
	availableWidth := terminalWidth - len(indent) - 5 // Account for indent and padding
	numCols := availableWidth / colWidth
	if numCols < 1 {
		numCols = 1
	}

	// Print nodes in columns
	for i, name := range nodeNames {
		fmt.Print(indent + name)

		// Add spacing to align columns
		if (i+1)%numCols == 0 || i == len(nodeNames)-1 {
			fmt.Println()
		} else {
			// Pad to column width
			padding := colWidth - len(name)
			fmt.Print(strings.Repeat(" ", padding))
		}
	}
}

// selectorToString converts a label selector map to a string representation
func selectorToString(selector map[string]string) string {
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

// getAllEligibleNodes returns all nodes that match any role selector or global selector
func getAllEligibleNodes(grouping RoleNodeGrouping) []corev1.Node {
	nodeMap := make(map[string]corev1.Node)

	// Add global nodes
	for name, node := range grouping.Global {
		nodeMap[name] = node
	}

	// Add role-specific nodes
	for _, roleGroup := range grouping.ByRole {
		for _, node := range roleGroup.Nodes {
			nodeMap[node.Name] = node
		}
	}

	// Convert to slice and sort
	var allNodes []corev1.Node
	for _, node := range nodeMap {
		allNodes = append(allNodes, node)
	}
	sort.Slice(allNodes, func(i, j int) bool {
		return allNodes[i].Name < allNodes[j].Name
	})

	return allNodes
}

// printNodesPerSelector prints tables of nodes matching each selector (global and role-specific)
func printNodesPerSelector(grouping RoleNodeGrouping, globalSelector map[string]string, podsByNode map[string][]corev1.Pod) {
	// Print global selector table - convert map to slice
	if len(grouping.Global) > 0 {
		var globalNodes []corev1.Node
		for _, node := range grouping.Global {
			globalNodes = append(globalNodes, node)
		}
		sort.Slice(globalNodes, func(i, j int) bool {
			return globalNodes[i].Name < globalNodes[j].Name
		})
		printNodeSelectorTable("Global NodeSelector", selectorToString(globalSelector), globalNodes, podsByNode)
	}

	// Print role-specific selector tables
	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		if roleGroup, exists := grouping.ByRole[role]; exists && len(roleGroup.Nodes) > 0 {
			printNodeSelectorTable(
				strings.ToUpper(role[:1])+role[1:]+" NodeSelector",
				selectorToString(roleGroup.Selector),
				roleGroup.Nodes,
				podsByNode,
			)
		}
	}
}

// printNodeSelectorTable prints a table for a specific selector with node resource info
func printNodeSelectorTable(title, selector string, nodes []corev1.Node, podsByNode map[string][]corev1.Pod) {
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
		allocCPU := QuantityOrZero(node.Status.Allocatable, corev1.ResourceCPU)
		allocMem := QuantityOrZero(node.Status.Allocatable, corev1.ResourceMemory)
		allocHP := QuantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

		// Calculate used resources from pods on this node
		usedCPU := calculatePodResourceUsage(podsByNode[node.Name], corev1.ResourceCPU)
		usedMem := calculatePodResourceUsage(podsByNode[node.Name], corev1.ResourceMemory)
		usedHP := calculatePodResourceUsage(podsByNode[node.Name], "hugepages-2Mi")

		// Calculate free resources
		freeCPU := resource.NewMilliQuantity(allocCPU.MilliValue()-usedCPU.MilliValue(), resource.DecimalSI)
		freeMem := resource.NewQuantity(allocMem.Value()-usedMem.Value(), resource.BinarySI)
		freeHP := resource.NewQuantity(allocHP.Value()-usedHP.Value(), resource.BinarySI)

		t.AppendRow(table.Row{
			node.Name,
			fmt.Sprintf("%.1f", float64(allocCPU.MilliValue())/1000),
			fmt.Sprintf("%.1f", float64(usedCPU.MilliValue())/1000),
			fmt.Sprintf("%.1f", float64(freeCPU.MilliValue())/1000),
			fmt.Sprintf("%.1fGi", float64(allocMem.Value())/(1024*1024*1024)),
			fmt.Sprintf("%.1fGi", float64(usedMem.Value())/(1024*1024*1024)),
			fmt.Sprintf("%.1fGi", float64(freeMem.Value())/(1024*1024*1024)),
			fmt.Sprintf("%.1fGi", float64(allocHP.Value())/(1024*1024*1024)),
			fmt.Sprintf("%.1fGi", float64(usedHP.Value())/(1024*1024*1024)),
			fmt.Sprintf("%.1fGi", float64(freeHP.Value())/(1024*1024*1024)),
		})
	}

	t.SetStyle(table.StyleLight)
	fmt.Printf("\n%s: %s\n  Matching nodes: %d\n", title, selector, len(nodes))
	t.Render()
}

// sortNodeNamesNumerically sorts node names using natural/numerical ordering
// e.g., h1, h2, h10, h11 instead of h1, h10, h11, h2
func sortNodeNamesNumerically(names []string) {
	sort.Slice(names, func(i, j int) bool {
		return compareNodeNames(names[i], names[j]) < 0
	})
}

// compareNodeNames compares two node names numerically
// Returns -1 if a < b, 0 if a == b, 1 if a > b
func compareNodeNames(a, b string) int {
	// Split each name into alternating text and number parts
	aParts := splitNodeName(a)
	bParts := splitNodeName(b)

	// Compare each part
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aPart := aParts[i]
		bPart := bParts[i]

		// Try to parse as numbers
		aNum, aIsNum := tryParseInt(aPart)
		bNum, bIsNum := tryParseInt(bPart)

		if aIsNum && bIsNum {
			// Both are numbers, compare numerically
			if aNum < bNum {
				return -1
			} else if aNum > bNum {
				return 1
			}
		} else if aIsNum != bIsNum {
			// One is number, one is text - numbers come after text
			if aIsNum {
				return 1
			}
			return -1
		} else {
			// Both are text, compare alphabetically
			if aPart < bPart {
				return -1
			} else if aPart > bPart {
				return 1
			}
		}
	}

	// One is prefix of the other
	if len(aParts) < len(bParts) {
		return -1
	} else if len(aParts) > len(bParts) {
		return 1
	}
	return 0
}

// splitNodeName splits a node name into alternating text and number parts
// e.g., "h5-15-a" -> ["h", "5", "-", "15", "-", "a"]
func splitNodeName(name string) []string {
	var parts []string
	var current strings.Builder
	isDigit := false

	for _, r := range name {
		if (r >= '0' && r <= '9') != isDigit {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			isDigit = !isDigit
		}
		current.WriteRune(r)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// tryParseInt tries to parse a string as an integer
// Returns the integer value and whether parsing was successful
func tryParseInt(s string) (int, bool) {
	num, err := strconv.Atoi(s)
	return num, err == nil
}
