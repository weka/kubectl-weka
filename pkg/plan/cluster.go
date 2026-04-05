package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/weka/kubectl-weka/pkg/hostcheck"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/preflight"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/utils"
	"github.com/weka/kubectl-weka/pkg/wekaconfig"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func ValidateAndPlanCluster(ctx context.Context, clients *kubernetes.K8sClients, cluster *v1alpha1.WekaCluster) error {
	if cluster.Spec.Dynamic == nil {
		return fmt.Errorf("only dynamic template is supported")
	}

	config := cluster.Spec.Dynamic
	cpuPolicy := cluster.Spec.CpuPolicy
	additionalMemory := cluster.Spec.AdditionalMemory

	usesHT := cpuPolicy == v1alpha1.CpuPolicyDedicatedHT || cpuPolicy == v1alpha1.CpuPolicyAuto

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

	nodes, err := kubernetes.GetClusterNodes(ctx, clients.CRClient)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Could not access cluster nodes: %v\n", err)
		_, _ = fmt.Fprintf(os.Stderr, "Continuing with planning without drive validation...\n\n")
		nodes = nil
	}

	if config.DriveContainers != nil && *config.DriveContainers > 0 {
		// Drive validation is deferred to detailed validation phase (line 278)
		// which has actual hostcheck data. Basic validation without hostcheck is unreliable.

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
		req.Drives = config.NumDrives // Set drive requirements

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
	nodeReqs := CalculateNodeRequirements(config, containers)

	PrintNodeRequirements(nodeReqs)

	// Validate cluster configuration using modular validation system
	fmt.Println("\n=== Validating Cluster Configuration ===")
	validationCtx := &wekaconfig.WekaConfigContext{
		Cluster: cluster,
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
		return fmt.Errorf("cluster configuration has errors")
	}

	if !hasWarnings {
		fmt.Println("\n✅ Cluster definition validation passed")
	} else {
		fmt.Println("\n⚠️  Cluster definition validation passed with warnings")
	}

	// If nodes were provided, continue with cluster validation and placement
	if nodes == nil || len(nodes) == 0 {
		fmt.Println("\n⚠️ Cluster nodes not available - skipping validation and placement simulation")
		return nil
	}

	fmt.Println("\n=== Validating Cluster Nodes ===")
	fmt.Printf("Found %d nodes in cluster\n", len(nodes))

	fmt.Println("\n=== Role-Based Node Allocation ===")

	// Track NotReady nodes for warnings
	totalNotReadyNodes := CountNotReadyNodes(nodes)
	hasNotReadyNodes := totalNotReadyNodes > 0

	// Filter to only ready nodes for planning
	readyNodes := kubernetes.FilterReadyNodes(nodes)

	// Build role-based node grouping (only with ready nodes)
	roleGrouping := buildRoleNodeGrouping(readyNodes, cluster.Spec.NodeSelector, &cluster.Spec.RoleNodeSelector)

	// Print role-based allocation
	printRoleNodeGrouping(cluster, roleGrouping)

	// Get all eligible nodes for validation
	allEligibleNodes := getAllEligibleNodes(roleGrouping)

	// Check how many NotReady nodes matched the selectors
	allMatchingNodes := getAllEligibleNodes(buildRoleNodeGrouping(nodes, cluster.Spec.NodeSelector, &cluster.Spec.RoleNodeSelector))
	notReadyMatchingCount := len(allMatchingNodes) - len(allEligibleNodes)

	// Warn about NotReady nodes if any matched the selectors
	if notReadyMatchingCount > 0 {
		fmt.Printf("\n⚠️ WARNING: Additional %d node(s) match the selectors but are in NotReady state.\n", notReadyMatchingCount)
		fmt.Println("   These nodes will not be checked for compliancy.")
	}

	fmt.Println("\n=== Fetching Cluster Resource Information ===")

	// Collect pod data from cluster
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

	fmt.Println("\n=== Nodes Matching Selection Criteria ===")
	printNodesPerSelector(roleGrouping, cluster.Spec.NodeSelector, podsByNode)

	// Perform detailed drive validation if drive containers are configured
	// Also collect hostcheck data for placement simulation
	var hostChecksMap hostcheck.HostChecksMap
	if config.DriveContainers != nil && *config.DriveContainers > 0 && config.NumDrives > 0 {
		fmt.Println("\n=== Detailed Drive Validation ===")
		fmt.Println("Scanning nodes for NVMe drives...")

		// Get hostchecks using registry (cached execution with defaults)
		var err error
		hostChecksMap, err = hostcheck.GlobalHostCheckRegistry.GetHostChecksForNodes(ctx, clients, allEligibleNodes)
		if err != nil {
			fmt.Printf("⚠️ WARNING: Could not scan drives on all nodes: %v\n", err)
			fmt.Println("   Falling back to basic drive validation...")
			hostChecksMap = nil
		} else if hostChecksMap != nil {
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
			if err := validateDrivesDetailed(hostChecksMap, driveRoleNodes, allEligibleNodes, *config.DriveContainers, config.NumDrives); err != nil {
				return err
			}
		}
	}

	// Validate network interfaces
	fmt.Println("\n=== Validating Network Interfaces ===")
	networkValidationErrors := validateNetworkInterfacesForCluster(ctx, cluster, allEligibleNodes, hostChecksMap)
	if networkValidationErrors {
		return fmt.Errorf("network interface validation failed")
	}

	// Simulate container placement
	fmt.Println("\n=== Simulating Container Placement ===")
	placement, err := simulatePlacement(roleGrouping, containers, podsByNode, hostChecksMap)
	if err != nil {
		return fmt.Errorf("placement simulation failed: %w", err)
	}

	fmt.Println("\n=== Placement Details & Resource Allocation ===")
	printPlacementDetailsWithResourceAllocation(placement, allEligibleNodes, podsByNode, hostChecksMap, existingContainers)

	// Final summary
	if hasNotReadyNodes {
		fmt.Printf("\n⚠️ WARNING: Plan completed with warnings\n")
		fmt.Printf("   ✅ %d total nodes in cluster\n", len(nodes))
		fmt.Printf("   ✅ %d nodes eligible for Weka deployment\n", len(allEligibleNodes))
		fmt.Printf("   ⚠️ %d node(s) were not ready during planning and were skipped\n", totalNotReadyNodes)
		if notReadyMatchingCount > 0 {
			fmt.Printf("   ⚠️ %d of the skipped nodes matched the node selectors\n", notReadyMatchingCount)
		}
		fmt.Printf("   ✅ Role-based node allocation configured\n")
		fmt.Printf("   ✅ All required drives are available\n")
		fmt.Printf("   ✅ Network configuration is consistent\n")
		fmt.Printf("   ✅ Sufficient resources available per role\n")
		fmt.Println("\n⚠️ Please notice that required validations were not performed on the NotReady nodes.")
		fmt.Println("   Recommended to remediate the nodes and rerun plan.")
	} else {
		fmt.Printf("\n✅ Cluster validation passed\n")
		fmt.Printf("   ✅ %d total nodes in cluster\n", len(nodes))
		fmt.Printf("   ✅ %d nodes eligible for Weka deployment\n", len(allEligibleNodes))
		fmt.Printf("   ✅ Role-based node allocation configured\n")
		fmt.Printf("   ✅ All required drives are available\n")
		fmt.Printf("   ✅ Network configuration is consistent\n")
		fmt.Printf("   ✅ Sufficient resources available per role\n")
	}

	return nil
}

// simulatePlacement simulates allocation of containers to nodes with anti-affinity rules and drive constraints
func simulatePlacement(nodeGrouping RoleNodeGrouping, containers []ContainerRequirements, podsByNode map[string][]v1.Pod, hostChecksMap hostcheck.HostChecksMap) ([]NodePlacement, error) {
	var placements []NodePlacement

	// DEBUG: Show which nodes have hostcheck data
	if len(containers) > 0 && containers[0].Drives > 0 {
		fmt.Println("DEBUG: Hostcheck data available for nodes:")
		if hostChecksMap == nil || len(hostChecksMap) == 0 {
			fmt.Println("  (no hostcheck data available)")
		} else {
			for nodeName := range hostChecksMap {
				fmt.Printf("  - %s\n", nodeName)
			}
		}
	}

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
	// Track drives used per node
	nodeDrivesUsed := make(map[string]int) // nodeName -> number of drives allocated

	// Map to get nodes available per role (respecting nodeSelectors)
	roleNodeMap := make(map[string][]v1.Node)

	// Add global nodes
	globalNodes := make([]v1.Node, 0, len(nodeGrouping.Global))
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

	// Helper function to get available free drives on a node
	// Must match the validation logic: physical + annotated (signed) drives
	getFreeDrivesCount := func(node *v1.Node) int {
		if hostChecksMap != nil {
			if hc, ok := hostChecksMap[node.Name]; ok {
				// Build annotated drives set (from node annotation) - these are "signed" drives
				annotatedMap := make(map[string]bool)
				if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
					var drives []string
					if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
						for _, serial := range drives {
							annotatedMap[serial] = true
						}
					}
				}

				// Count free drives: physical + signed (don't check mounted status)
				// This matches the validation logic which counts physical+annotated as free
				freeCount := 0
				for _, drive := range hc.NVMeDrives {
					if drive.SerialNumber != "" && annotatedMap[drive.SerialNumber] {
						freeCount++
					}
				}

				// Subtract drives already allocated during this simulation
				if alreadyUsed, ok := nodeDrivesUsed[node.Name]; ok {
					freeCount -= alreadyUsed
				}

				return freeCount
			} else {
				// Node not in hostChecksMap - will use fallback
				// This can happen if hostcheck failed for this node
			}
		}

		// Fallback 1: Use allocatable quantity
		if drivesQuantity, ok := node.Status.Allocatable["weka.io/drives"]; ok {
			freeCount := int(drivesQuantity.Value())
			if alreadyUsed, ok := nodeDrivesUsed[node.Name]; ok {
				freeCount -= alreadyUsed
			}
			return freeCount
		}

		// Fallback 2: Count annotated drives
		if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
			var drives []string
			if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
				freeCount := len(drives)
				if alreadyUsed, ok := nodeDrivesUsed[node.Name]; ok {
					freeCount -= alreadyUsed
				}
				return freeCount
			}
		}

		return 0
	}

	// Helper function to check if node has enough free resources
	hasEnoughResources := func(node *v1.Node, requiredCores int, requiredMemory int64, requiredHP int64, requiredDrives int) bool {
		allocCores := kubernetes.QuantityOrZero(node.Status.Allocatable, v1.ResourceCPU)
		allocMem := kubernetes.QuantityOrZero(node.Status.Allocatable, v1.ResourceMemory)
		allocHP := kubernetes.QuantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

		// Calculate current usage from pods
		currentUsedCPU := CalculatePodResourceUsage(podsByNode[node.Name], v1.ResourceCPU)
		currentUsedMem := CalculatePodResourceUsage(podsByNode[node.Name], v1.ResourceMemory)
		currentUsedHP := CalculatePodResourceUsage(podsByNode[node.Name], "hugepages-2Mi")

		// Calculate free resources
		freeCores := allocCores.MilliValue()/1000 - currentUsedCPU.MilliValue()/1000
		freeMem := allocMem.Value() - currentUsedMem.Value()
		freeHP := allocHP.Value() - currentUsedHP.Value()

		// Check if enough resources available
		requiredMemBytes := requiredMemory * 1024 * 1024 // Convert MiB to bytes
		requiredHPBytes := requiredHP * 1024 * 1024      // Convert MiB to bytes

		hasEnoughCPUMemHP := int64(freeCores) >= int64(requiredCores) &&
			freeMem >= requiredMemBytes &&
			freeHP >= requiredHPBytes

		// If drives are required, check drive availability
		if requiredDrives > 0 {
			freeDrives := getFreeDrivesCount(node)
			return hasEnoughCPUMemHP && freeDrives >= requiredDrives
		}

		return hasEnoughCPUMemHP
	}

	// Try to place containers
	for _, cType := range roleNames {
		containerList, exists := containersByType[cType]
		if !exists {
			continue
		}

		// Get nodes available for this role (respecting nodeSelector)
		nodesForRole := roleNodeMap[cType]

		// For drive containers, pre-filter nodes to only those with available drives
		// This avoids wasting time checking hundreds of nodes without drives
		if cType == "drive" {
			// Check if this container type requires drives
			var needsDrives int
			if len(containerList) > 0 {
				needsDrives = containerList[0].Drives
			}

			if needsDrives > 0 {
				var nodesWithDrives []v1.Node
				for _, node := range nodesForRole {
					freeDrives := getFreeDrivesCount(&node)
					if freeDrives > 0 {
						nodesWithDrives = append(nodesWithDrives, node)
					}
				}
				if len(nodesWithDrives) > 0 {
					nodesForRole = nodesWithDrives
				}
				// If no nodes with drives, let placement error handling report it
			}
		}

		fmt.Printf("Allocating %d %s container(s):\n", len(containerList), strings.ToLower(cType))

		for i := 0; i < len(containerList); i++ {
			c := containerList[i]

			// Get drive requirements from container (will be 0 for non-drive containers)
			requiredDrives := c.Drives

			// Find best node for this container
			placed := false
			for nodeIdx := range nodesForRole {
				node := &nodesForRole[nodeIdx]

				// Check same-type anti-affinity: same type can't be on same node
				if typeOnNode[cType][node.Name] {
					continue
				}

				// Protocol coexistence rules:
				// - Storage (compute/drive): can coexist with anything
				// - Access (s3/nfs) and client: cannot coexist with each other
				canPlace := true
				if nodeContainerTypes[node.Name] != nil {
					// Check what's already on this node
					hasS3 := nodeContainerTypes[node.Name]["s3"]
					hasNFS := nodeContainerTypes[node.Name]["nfs"]
					hasClient := nodeContainerTypes[node.Name]["client"]

					switch cType {
					case "compute", "drive":
						// Storage protocols (compute/drive) can share with everything
						canPlace = true
					case "s3":
						// S3 cannot share with NFS or Client
						canPlace = !hasNFS && !hasClient
					case "nfs":
						// NFS cannot share with S3 or Client
						canPlace = !hasS3 && !hasClient
					case "client":
						// Client cannot share with S3 or NFS
						canPlace = !hasS3 && !hasNFS
					}
				}

				if !canPlace {
					continue
				}

				// Check if node has enough free resources for this container (including drives)
				if !hasEnoughResources(node, c.Cores, c.Memory, c.Hugepages, requiredDrives) {
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
					Drives:    requiredDrives,
				})
				placements[placementIdx].UsedCores += c.Cores
				placements[placementIdx].UsedMemory += c.Memory
				placements[placementIdx].UsedHP += c.Hugepages
				placements[placementIdx].UsedDrives += requiredDrives

				// Track drive allocation
				nodeDrivesUsed[node.Name] += requiredDrives

				typeOnNode[cType][node.Name] = true
				nodeContainerTypes[node.Name][cType] = true
				placed = true

				// Print placement details with lowercase container type
				if requiredDrives > 0 {
					fmt.Printf("  ✅ Placed %s container #%d on node %s (Cores: %d, Memory: %d MiB, Hugepages: %d MiB, Drives: %d)\n",
						strings.ToLower(cType), i, node.Name, c.Cores, c.Memory, c.Hugepages, requiredDrives)
				} else {
					fmt.Printf("  ✅ Placed %s container #%d on node %s (Cores: %d, Memory: %d MiB, Hugepages: %d MiB)\n",
						strings.ToLower(cType), i, node.Name, c.Cores, c.Memory, c.Hugepages)
				}
				break
			}

			if !placed {
				// Provide detailed error message about what's missing
				errorMsg := fmt.Sprintf("could not place %s container %d", strings.ToLower(cType), i)

				if requiredDrives > 0 {
					// Check which nodes have drives and why they couldn't accommodate this container
					var nodeDetails []string
					for _, node := range nodesForRole {
						freeDrives := getFreeDrivesCount(&node)

						// Check why this node was rejected
						if typeOnNode[cType][node.Name] {
							nodeDetails = append(nodeDetails, fmt.Sprintf("%s (already has %s container)", node.Name, cType))
						} else if freeDrives < requiredDrives {
							nodeDetails = append(nodeDetails, fmt.Sprintf("%s (only %d of %d drives available)", node.Name, freeDrives, requiredDrives))
						} else {
							// Node has drives but failed for protocol compatibility or resource reasons
							nodeDetails = append(nodeDetails, fmt.Sprintf("%s (protocol conflict or insufficient CPU/Memory/HP)", node.Name))
						}
					}

					if len(nodeDetails) > 0 {
						errorMsg += fmt.Sprintf(" - all nodes rejected:\n    %s", strings.Join(nodeDetails, "\n    "))
					} else {
						errorMsg += " - no nodes available for role"
					}
				} else {
					errorMsg += " - insufficient nodes or resources"
				}

				return nil, errors.New(errorMsg)
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

					fmt.Printf("  ✅ Placed envoy container #%d on node %s (Cores: %d, Memory: %d MiB, Hugepages: %d MiB)\n",
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
func printPlacementDetailsWithResourceAllocation(placements []NodePlacement, nodes []v1.Node, podsByNode map[string][]v1.Pod, hostChecksMap hostcheck.HostChecksMap, existingContainers []v1alpha1.WekaContainer) {

	// Build table rows for placement details
	placementCols := []printer.TableColumn{
		{Name: "NODE", VisibleInWide: false},
		{Name: "CONTAINERS & RESOURCES", VisibleInWide: false},
		{Name: "RESOURCE ALLOCATION", VisibleInWide: false},
	}

	placementRows := []printer.TableRow{}

	// Create a map of placements for quick lookup
	placementMap := make(map[string]*NodePlacement)
	for i := range placements {
		placementMap[placements[i].NodeName] = &placements[i]
	}

	for _, np := range placements {
		nodeName := np.NodeName

		// Find the node in the nodes list
		var node *v1.Node
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
		containerTypes := []string{}

		// Add ALREADY_USED section showing current pod usage (always show)
		currentUsedCPU := CalculatePodResourceUsage(podsByNode[nodeName], v1.ResourceCPU)
		currentUsedMem := CalculatePodResourceUsage(podsByNode[nodeName], v1.ResourceMemory)
		currentUsedHP := CalculatePodResourceUsage(podsByNode[nodeName], "hugepages-2Mi")

		// Build container subtable using printer.TablePrinter
		containerCols := []printer.TableColumn{
			{Name: "COMPONENT", VisibleInWide: false},
			{Name: "CORES", VisibleInWide: false},
			{Name: "RAM", VisibleInWide: false},
			{Name: "HP", VisibleInWide: false},
			{Name: "DRIVES", VisibleInWide: false},
		}

		containerRows := []printer.TableRow{}

		// Add ALREADY_USED row
		// Recalculate drives used to ensure accuracy
		drivesUsedByOthers := CalculateAllocatedDrives(existingContainers, nodeName)
		drivesStr := "-"
		if drivesUsedByOthers > 0 {
			drivesStr = fmt.Sprintf("%d", drivesUsedByOthers)
		}

		containerRows = append(containerRows, printer.TableRow{
			Values: map[string]interface{}{
				"COMPONENT": createContainerLegend("<ALREADY_USED>"),
				"CORES":     fmt.Sprintf("%.1f", float64(currentUsedCPU.MilliValue())/1000),
				"RAM":       fmt.Sprintf("%.1fGi", float64(currentUsedMem.Value())/(1024*1024*1024)),
				"HP":        fmt.Sprintf("%.1fGi", float64(currentUsedHP.Value())/(1024*1024*1024)),
				"DRIVES":    drivesStr,
			},
		})

		// Add each container row with unique legend of color and pattern
		for _, pc := range np.Containers {
			drivesStr := "-"
			if pc.Drives > 0 {
				drivesStr = fmt.Sprintf("%d", pc.Drives)
			}
			containerRows = append(containerRows, printer.TableRow{
				Values: map[string]interface{}{
					"COMPONENT": createContainerLegend(pc.Type),
					"CORES":     fmt.Sprintf("%d", pc.Cores),
					"RAM":       fmt.Sprintf("%.1fGi", float64(pc.Memory)/1024),
					"HP":        fmt.Sprintf("%.1fGi", float64(pc.Hugepages)/1024),
					"DRIVES":    drivesStr,
				},
			})
			// Only add to containerTypes if this type uses resources on this node
			if pc.Cores > 0 || pc.Memory > 0 || pc.Hugepages > 0 || pc.Drives > 0 {
				containerTypes = append(containerTypes, pc.Type)
			}
		}

		// Render container table to string
		containerPrinter := &printer.TablePrinter{}
		containerPrinter.SetOptions(printer.PrinterOptions{
			ShowHeader: true,
			TableStyle: printer.TableStyleMinimal,
		})

		sb := &strings.Builder{}
		_ = containerPrinter.Print(containerCols, containerRows, sb)
		containersStr := strings.TrimSpace(sb.String())

		// Get allocatable resources from node
		allocCores := kubernetes.QuantityOrZero(node.Status.Allocatable, v1.ResourceCPU)
		allocMem := kubernetes.QuantityOrZero(node.Status.Allocatable, v1.ResourceMemory)
		allocHP := kubernetes.QuantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

		// Format resource allocation bars showing USED + WEKA + FREE
		resourceBarsStr := ""

		// CPU bar
		allocCoresVal := float64(allocCores.MilliValue() / 1000)
		currentCPUPercent := float64(currentUsedCPU.MilliValue()/1000) * 100.0 / allocCoresVal
		wekaCPUPercent := float64(np.UsedCores) * 100.0 / allocCoresVal
		cpuBar := createResourceBar(currentCPUPercent, wekaCPUPercent, containerTypes)
		resourceBarsStr += fmt.Sprintf("CPU:    %s\n", cpuBar)

		// Memory bar
		allocMemVal := float64(allocMem.Value())
		currentMemPercent := float64(currentUsedMem.Value()) * 100.0 / allocMemVal
		wekaMemVal := float64(np.UsedMemory) * 1024 * 1024 // Convert MiB to bytes
		wekaMemPercent := wekaMemVal * 100.0 / allocMemVal
		memBar := createResourceBar(currentMemPercent, wekaMemPercent, containerTypes)
		resourceBarsStr += fmt.Sprintf("Mem:    %s\n", memBar)

		// Hugepages bar
		allocHPVal := float64(allocHP.Value())
		currentHPPercent := float64(currentUsedHP.Value()) * 100.0 / allocHPVal
		wekaHPVal := float64(np.UsedHP) * 1024 * 1024 // Convert MiB to bytes
		wekaHPPercent := wekaHPVal * 100.0 / allocHPVal
		hpBar := createResourceBar(currentHPPercent, wekaHPPercent, containerTypes)
		resourceBarsStr += fmt.Sprintf("HP:     %s\n", hpBar)

		// Drives bar (only show if node has drives)
		if np.UsedDrives > 0 || hasNodeDrives(node, hostChecksMap) {
			totalDrives := getNodeTotalDrives(node, hostChecksMap)

			// Calculate percentages for the bar
			currentDrivesPercent := 0.0
			wekaDrivesPercent := 0.0
			// For drives bar, only include container types that actually use drives
			var driveContainerTypes []string
			for _, pc := range np.Containers {
				if pc.Drives > 0 {
					driveContainerTypes = append(driveContainerTypes, pc.Type)
				}
			}

			if totalDrives > 0 {
				// Drives already used by other WEKA clusters - recalculate to be sure
				drivesUsedByOthers := CalculateAllocatedDrives(existingContainers, nodeName)
				// Drives used by THIS deployment
				currentDrivesPercent = float64(drivesUsedByOthers) * 100.0 / float64(totalDrives)
				wekaDrivesPercent = float64(np.UsedDrives) * 100.0 / float64(totalDrives)
			}

			// Create bar showing drive allocation (used from other clusters + used by this deployment)
			// Use only drive container types, not all types
			drivesBar := createResourceBar(currentDrivesPercent, wekaDrivesPercent, driveContainerTypes)
			resourceBarsStr += fmt.Sprintf("Drives: %s", drivesBar)
		}

		placementRows = append(placementRows, printer.TableRow{
			Values: map[string]interface{}{
				"NODE":                   nodeName,
				"CONTAINERS & RESOURCES": containersStr,
				"RESOURCE ALLOCATION":    resourceBarsStr,
			},
		})
	}

	// Render placement table using printer.TablePrinter
	placementPrinter := &printer.TablePrinter{}
	placementPrinter.SetOptions(printer.PrinterOptions{
		ShowHeader:        true,
		TableStyle:        printer.TableStyleRoundedBox,
		PrintRowSeparator: true,
	})
	_ = placementPrinter.Print(placementCols, placementRows, os.Stdout)
}

// hasNodeDrives checks if a node has any drives available
func hasNodeDrives(node *v1.Node, hostChecksMap hostcheck.HostChecksMap) bool {
	// Check hostcheck data
	if hostChecksMap != nil {
		if hc, ok := hostChecksMap[node.Name]; ok {
			return hc.NVMeDriveCount > 0
		}
	}

	// Check allocatable
	if drivesQuantity, ok := node.Status.Allocatable["weka.io/drives"]; ok {
		return drivesQuantity.Value() > 0
	}

	// Check annotation
	if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
		var drives []string
		if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
			return len(drives) > 0
		}
	}

	return false
}

// getNodeTotalDrives returns the total number of drives on a node
func getNodeTotalDrives(node *v1.Node, hostChecksMap hostcheck.HostChecksMap) int {
	// Check hostcheck data first
	if hostChecksMap != nil {
		if hc, ok := hostChecksMap[node.Name]; ok {
			// Count signed drives
			count := 0
			annotatedMap := make(map[string]bool)
			if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
				var drives []string
				if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
					for _, serial := range drives {
						annotatedMap[serial] = true
					}
				}
			}

			for _, drive := range hc.NVMeDrives {
				if drive.SerialNumber != "" && annotatedMap[drive.SerialNumber] {
					count++
				}
			}

			if count > 0 {
				return count
			}
		}
	}

	// Fallback to allocatable
	if drivesQuantity, ok := node.Status.Allocatable["weka.io/drives"]; ok {
		return int(drivesQuantity.Value())
	}

	// Last resort: annotation
	if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
		var drives []string
		if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
			return len(drives)
		}
	}

	return 0
}

// validateDrivesDetailed performs detailed drive validation using hostcheck data
// This function analyzes physical drives vs annotated drives vs allocated drives
// This is the ONLY reliable validation since it checks what the kernel actually sees (physical drives)
func validateDrivesDetailed(hostChecksMap hostcheck.HostChecksMap, nodes []v1.Node, allNodes []v1.Node, driveContainers, numDrives int) error {
	totalDrivesNeeded := driveContainers * numDrives
	if totalDrivesNeeded == 0 {
		return nil
	}

	type NodeDriveStatus struct {
		NodeName          string
		PhysicalDrives    []string // Serial numbers from /dev scan
		AnnotatedDrives   []string // Serial numbers from annotation
		AllocatableDrives int      // From resources.allocatable
		FreeDrives        []string // Drives that are physical + annotated but not mounted
		UnsignedDrives    []string // Drives that are physical but not annotated
		MissingDrives     []string // Drives that are annotated but not physical (used)
		IsReady           bool     // Whether node is in Ready state
	}

	var nodeStatuses []NodeDriveStatus
	totalFreeDrives := 0
	var warnings []string

	for _, node := range nodes {
		status := NodeDriveStatus{
			NodeName: node.Name,
			IsReady:  kubernetes.IsNodeReady(node),
		}

		// Skip validation for not-ready nodes - no reliable hostcheck data
		if !status.IsReady {
			warnings = append(warnings, fmt.Sprintf(
				"   Node %s: Not Ready - excluding from drive validation (no hostcheck data available)",
				node.Name))
			nodeStatuses = append(nodeStatuses, status)
			continue
		}

		// Get physical drives from hostcheck
		if hc, ok := hostChecksMap[node.Name]; ok {
			for _, drive := range hc.NVMeDrives {
				if drive.SerialNumber != "" {
					status.PhysicalDrives = append(status.PhysicalDrives, drive.SerialNumber)
					// Check if drive is mounted
					if !drive.Mounted {
						// Drive is physical and not mounted - check if it's annotated
						// We'll do this check below
					}
				}
			}
		}

		// Get annotated drives (signed drives)
		if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
			var drives []string
			if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
				status.AnnotatedDrives = drives
			}
		}

		// Get allocatable drives count
		if drivesQuantity, ok := node.Status.Allocatable["weka.io/drives"]; ok {
			status.AllocatableDrives = int(drivesQuantity.Value())
		}

		// Build maps for quick lookup
		physicalMap := make(map[string]bool)
		for _, serial := range status.PhysicalDrives {
			physicalMap[serial] = true
		}

		annotatedMap := make(map[string]bool)
		for _, serial := range status.AnnotatedDrives {
			annotatedMap[serial] = true
		}

		// Identify drive categories
		for _, serial := range status.PhysicalDrives {
			if annotatedMap[serial] {
				// Physical + annotated = free drive
				status.FreeDrives = append(status.FreeDrives, serial)
			} else {
				// Physical but not annotated = unsigned drive
				status.UnsignedDrives = append(status.UnsignedDrives, serial)
			}
		}

		// Drives that are annotated but not physical = already used/allocated
		for _, serial := range status.AnnotatedDrives {
			if !physicalMap[serial] {
				status.MissingDrives = append(status.MissingDrives, serial)
			}
		}

		// Count free drives
		totalFreeDrives += len(status.FreeDrives)

		// Generate warnings for this node
		if len(status.UnsignedDrives) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"   Node %s: %d unsigned NVMe drive(s) found. Run DriveSign WekaPolicy to make them available for WEKA",
				node.Name, len(status.UnsignedDrives)))
		}

		nodeStatuses = append(nodeStatuses, status)
	}

	// Check if we have enough free drives
	if totalFreeDrives == 0 {
		notReadyCount := 0
		for _, status := range nodeStatuses {
			if !status.IsReady {
				notReadyCount++
			}
		}

		msg := "❌ No free NVMe drives found on nodes matching the drive role selector.\n"
		if notReadyCount > 0 {
			msg += fmt.Sprintf("   (%d node(s) were Not Ready and excluded from validation)\n", notReadyCount)
		}
		msg += "   Drives must be:\n" +
			"   1. Physically present (detected in /dev)\n" +
			"   2. Signed (by configuring WekaPolicy of type \"sign-drives\")\n" +
			"   3. Not mounted or in use by existing WEKA clusters\n\n"

		// Check if drives exist on other nodes (not in the drive role selector)
		drivesOnOtherNodes := 0
		if allNodes != nil && len(allNodes) > len(nodes) {
			// Build set of nodes in drive role for comparison
			driveNodeSet := make(map[string]bool)
			for _, n := range nodes {
				driveNodeSet[n.Name] = true
			}

			// Check all nodes for drives not in drive role selector
			for _, node := range allNodes {
				if !driveNodeSet[node.Name] && kubernetes.IsNodeReady(node) {
					if hc, ok := hostChecksMap[node.Name]; ok {
						// Count annotated drives on this node
						if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
							var drives []string
							if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
								// Build annotated map to count physical+annotated
								annotatedMap := make(map[string]bool)
								for _, serial := range drives {
									annotatedMap[serial] = true
								}

								// Count physical+annotated drives
								for _, drive := range hc.NVMeDrives {
									if drive.SerialNumber != "" && annotatedMap[drive.SerialNumber] {
										drivesOnOtherNodes++
									}
								}
							}
						}
					}
				}
			}
		}

		if drivesOnOtherNodes > 0 {
			msg += fmt.Sprintf("💡 Note: %d drive(s) are available on other nodes in the cluster,\n", drivesOnOtherNodes)
			msg += "   but these nodes do not match your drive role node selector.\n"
			msg += "   To use those drives, update your drive role node selector.\n\n"
			msg += "   Apply DriveSign WekaPolicy to sign any unsigned drives."
		} else {
			msg += "   Apply DriveSign WekaPolicy to sign drives."
		}

		return errors.New(msg)
	}

	if totalFreeDrives < totalDrivesNeeded {
		msg := fmt.Sprintf("❌ Insufficient free drives: need %d drives (%d containers × %d drives/container), but only %d available on nodes matching the drive role selector\n\n",
			totalDrivesNeeded, driveContainers, numDrives, totalFreeDrives)

		// Add per-node breakdown
		msg += "Drive availability by node (matching drive role selector):\n"
		for _, status := range nodeStatuses {
			if status.IsReady {
				if len(status.FreeDrives) > 0 || len(status.UnsignedDrives) > 0 {
					msg += fmt.Sprintf("  %s: %d free, %d unsigned, %d in use\n",
						status.NodeName, len(status.FreeDrives), len(status.UnsignedDrives), len(status.MissingDrives))
				}
			} else {
				msg += fmt.Sprintf("  %s: Not Ready (excluded from validation)\n", status.NodeName)
			}
		}

		// Check if drives exist on other nodes (not in the drive role selector)
		drivesOnOtherNodes := 0
		if allNodes != nil && len(allNodes) > len(nodes) {
			// Build set of nodes in drive role for comparison
			driveNodeSet := make(map[string]bool)
			for _, n := range nodes {
				driveNodeSet[n.Name] = true
			}

			// Check all nodes for drives not in drive role selector
			for _, node := range allNodes {
				if !driveNodeSet[node.Name] && kubernetes.IsNodeReady(node) {
					if hc, ok := hostChecksMap[node.Name]; ok {
						// Count annotated drives on this node
						if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
							var drives []string
							if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
								// Build annotated map to count physical+annotated
								annotatedMap := make(map[string]bool)
								for _, serial := range drives {
									annotatedMap[serial] = true
								}

								// Count physical+annotated drives
								for _, drive := range hc.NVMeDrives {
									if drive.SerialNumber != "" && annotatedMap[drive.SerialNumber] {
										drivesOnOtherNodes++
									}
								}
							}
						}
					}
				}
			}
		}

		if drivesOnOtherNodes > 0 {
			msg += fmt.Sprintf("\n💡 Note: %d drive(s) are available on other nodes in the cluster,\n", drivesOnOtherNodes)
			msg += "   but these nodes do not match your drive role node selector.\n"
			msg += "   To use those drives, update your drive role node selector."
		}

		return errors.New(msg)
	}

	// Print warnings if any (not-ready nodes or unsigned drives)
	if len(warnings) > 0 {
		fmt.Println("\n⚠️ Drive Validation Warnings:")
		for _, warning := range warnings {
			fmt.Println(warning)
		}
		fmt.Println()
	}

	// Print per-node drive availability for successful validation
	fmt.Println("\nDrive availability by node:")
	for _, status := range nodeStatuses {
		if status.IsReady {
			if len(status.FreeDrives) > 0 || len(status.UnsignedDrives) > 0 {
				fmt.Printf("  %s: %d free, %d unsigned, %d in use\n",
					status.NodeName, len(status.FreeDrives), len(status.UnsignedDrives), len(status.MissingDrives))
			} else {
				fmt.Printf("  %s: 0 free drives\n", status.NodeName)
			}
		} else {
			fmt.Printf("  %s: Not Ready (excluded from validation)\n", status.NodeName)
		}
	}

	// Success message
	fmt.Printf("\n✅ Drive validation passed: %d free drives available (need %d)\n", totalFreeDrives, totalDrivesNeeded)

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
		"Drives",
	})

	for _, c := range containers {
		drivesStr := "-"
		if c.Drives > 0 {
			drivesStr = fmt.Sprintf("%d", c.Drives)
		}
		t.AppendRow(table.Row{
			utils.CapitalizeFirst(c.Type),
			c.Count,
			c.Cores,
			c.CoresNoHT,
			fmt.Sprintf("%d MiB", c.Hugepages),
			fmt.Sprintf("%d MiB", c.Memory),
			drivesStr,
		})
	}

	t.SetStyle(table.StyleLight)
	fmt.Println("\n=== Container Resource Requirements ===")
	t.Render()
	fmt.Println()
}

// buildRoleNodeGrouping builds a mapping of nodes by role based on nodeSelectors
func buildRoleNodeGrouping(nodes []v1.Node, globalSelector map[string]string, roleNodeSelector *v1alpha1.RoleNodeSelector) RoleNodeGrouping {
	grouping := RoleNodeGrouping{
		Global: make(map[string]v1.Node),
		ByRole: make(map[string]struct {
			Selector map[string]string
			Nodes    []v1.Node
		}),
	}

	// Add nodes matching global selector
	for _, node := range nodes {
		if kubernetes.MatchesSelector(node, globalSelector) {
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
			var roleNodes []v1.Node
			for _, node := range nodes {
				if kubernetes.MatchesSelector(node, roleSelector) {
					roleNodes = append(roleNodes, node)
				}
			}

			if len(roleNodes) > 0 {
				grouping.ByRole[role] = struct {
					Selector map[string]string
					Nodes    []v1.Node
				}{
					Selector: roleSelector,
					Nodes:    roleNodes,
				}
			}
		}
	}

	return grouping
}

// printRoleNodeGrouping prints the role-based node grouping
func printRoleNodeGrouping(cluster *v1alpha1.WekaCluster, grouping RoleNodeGrouping) {
	if len(grouping.Global) > 0 {
		fmt.Printf("Global NodeSelector matches: %d nodes\n", len(grouping.Global))
		fmt.Printf("  Selector: %s\n", SelectorToString(cluster.Spec.NodeSelector))
		if len(grouping.Global) > 0 {
			nodeNames := make([]string, 0, len(grouping.Global))
			for _, node := range grouping.Global {
				nodeNames = append(nodeNames, node.Name)
			}
			kubernetes.SortNodeNamesNumerically(nodeNames)
			kubernetes.PrintNodeList("  ", nodeNames)
		}
		fmt.Println()
	}

	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		if roleGroup, exists := grouping.ByRole[role]; exists {
			fmt.Printf("%s role NodeSelector matches: %d nodes\n", utils.CapitalizeFirst(role), len(roleGroup.Nodes))
			fmt.Printf("  Selector: %s\n", SelectorToString(roleGroup.Selector))
			if len(roleGroup.Nodes) > 0 {
				nodeNames := make([]string, len(roleGroup.Nodes))
				for i, n := range roleGroup.Nodes {
					nodeNames[i] = n.Name
				}
				kubernetes.SortNodeNamesNumerically(nodeNames)
				kubernetes.PrintNodeList("  ", nodeNames)
			}
			fmt.Println()
		}
	}
}

// getAllEligibleNodes returns all nodes that match any role selector or global selector
func getAllEligibleNodes(grouping RoleNodeGrouping) []v1.Node {
	nodeMap := make(map[string]v1.Node)

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
	var allNodes []v1.Node
	for _, node := range nodeMap {
		allNodes = append(allNodes, node)
	}
	sort.Slice(allNodes, func(i, j int) bool {
		return allNodes[i].Name < allNodes[j].Name
	})

	return allNodes
}

// printNodesPerSelector prints tables of nodes matching each selector (global and role-specific)
func printNodesPerSelector(grouping RoleNodeGrouping, globalSelector map[string]string, podsByNode map[string][]v1.Pod) {
	// Print global selector table - convert map to slice
	if len(grouping.Global) > 0 {
		var globalNodes []v1.Node
		for _, node := range grouping.Global {
			globalNodes = append(globalNodes, node)
		}
		sort.Slice(globalNodes, func(i, j int) bool {
			return globalNodes[i].Name < globalNodes[j].Name
		})
		printNodeSelectorTable("Global NodeSelector", SelectorToString(globalSelector), globalNodes, podsByNode)
	}

	// Print role-specific selector tables
	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		if roleGroup, exists := grouping.ByRole[role]; exists && len(roleGroup.Nodes) > 0 {
			printNodeSelectorTable(
				utils.CapitalizeFirst(role)+" NodeSelector",
				SelectorToString(roleGroup.Selector),
				roleGroup.Nodes,
				podsByNode,
			)
		}
	}
}

// printNodeSelectorTable prints a table for a specific selector with node resource info
func printNodeSelectorTable(title, selector string, nodes []v1.Node, podsByNode map[string][]v1.Pod) {
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

		// Calculate free resources (direct arithmetic, no need for Quantity objects)
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
	fmt.Printf("\n%s: %s\n  Matching nodes: %d\n", title, selector, len(nodes))
	t.Render()
}

// validateNetworkInterfacesForCluster validates network interfaces for the cluster
// It validates NICs on global nodeSelector, and per-role NICs
func validateNetworkInterfacesForCluster(_ context.Context, cluster *v1alpha1.WekaCluster, allEligibleNodes []v1.Node, hostChecksMap hostcheck.HostChecksMap) bool {
	hasErrors := false

	// Print what's being tested
	printNetworkValidationTestsDescription()

	// Accumulate stats across all validations
	globalStats := NewNetworkValidationStats()

	// Get nodes matching global nodeSelector
	globalNodes := kubernetes.FilterNodesBySelector(allEligibleNodes, cluster.Spec.NodeSelector)
	if len(globalNodes) == 0 {
		fmt.Println("  ℹ️  No nodes match global nodeSelector - skipping global network validation")
	} else {
		// Build hostchecks map for nodes
		hostChecksForValidation := make(map[string]*hostcheck.HostChecksResult)
		for _, node := range globalNodes {
			if hostChecksMap != nil && hostChecksMap[node.Name] != nil {
				hostChecksForValidation[node.Name] = hostChecksMap[node.Name]
			}
		}

		// Validate global network config with stats
		result, stats := ValidateNetworkInterfacesWithStats(&cluster.Spec, hostChecksForValidation, false, globalNodes)
		if !result.Valid {
			hasErrors = true
			fmt.Println("  ❌ Global network interface validation failed:")
			for _, err := range result.Errors {
				fmt.Printf("    %s\n", err.String())
			}
		} else {
			fmt.Println("  ✅ Global network interfaces validated")
		}
		for _, warn := range result.Warnings {
			fmt.Printf("  ⚠️  %s\n", warn.String())
		}
		// Merge stats - sum up the counts
		if stats != nil {
			for ifName, stat := range stats.InterfaceStats {
				if existing, ok := globalStats.InterfaceStats[ifName]; ok {
					existing.Configured += stat.Configured
					existing.Missing += stat.Missing
					existing.Misconfigured += stat.Misconfigured
				} else {
					globalStats.InterfaceStats[ifName] = stat
				}
			}
		}
	}

	// Validate per-role network configurations
	// Check each role's selector
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

		roleNodes := kubernetes.FilterNodesBySelector(allEligibleNodes, *selector)
		if len(roleNodes) == 0 {
			fmt.Printf("  ℹ️  No nodes match nodeSelector for role '%s' - skipping role network validation\n", role)
			continue
		}

		// Build hostchecks map for role nodes
		hostChecksForValidation := make(map[string]*hostcheck.HostChecksResult)
		for _, node := range roleNodes {
			if hostChecksMap != nil && hostChecksMap[node.Name] != nil {
				hostChecksForValidation[node.Name] = hostChecksMap[node.Name]
			}
		}

		// Validate role network config with stats
		result, stats := ValidateNetworkInterfacesWithStats(&cluster.Spec, hostChecksForValidation, false, roleNodes)
		if !result.Valid {
			hasErrors = true
			fmt.Printf("  ❌ Network interface validation failed for role '%s':\n", role)
			for _, err := range result.Errors {
				fmt.Printf("    %s\n", err.String())
			}
		} else {
			fmt.Printf("  ✅ Network interfaces validated for role '%s'\n", role)
		}
		for _, warn := range result.Warnings {
			fmt.Printf("  ⚠️  %s\n", warn.String())
		}
		// Merge stats - sum up the counts
		if stats != nil {
			for ifName, stat := range stats.InterfaceStats {
				if existing, ok := globalStats.InterfaceStats[ifName]; ok {
					existing.Configured += stat.Configured
					existing.Missing += stat.Missing
					existing.Misconfigured += stat.Misconfigured
				} else {
					globalStats.InterfaceStats[ifName] = stat
				}
			}
		}
	}

	// Print summary table if we have any stats
	if len(globalStats.InterfaceStats) > 0 {
		fmt.Println(printNetworkInterfaceSummaryTable(globalStats))
	}

	return hasErrors
}
