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
)

var (
	planClientFailFast bool
)

var planClientCmd = &cobra.Command{
	Use:   "client <file.yaml>",
	Short: "Plan client deployment from WekaClient YAML file",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanClient,
}

func init() {
	planCmd.AddCommand(planClientCmd)
	planClientCmd.Flags().BoolVar(&planClientFailFast, "fail-fast", false, "Stop validation on first error (default: collect all errors)")
}

type ClientContainerRequirements struct {
	Hugepages int64
	Cores     int // Cores with HT
	CoresNoHT int // Cores without HT
	Memory    int64
	CPUMilli  int64
}

type ClientNodeAllocation struct {
	NodeName          string
	ContainerCount    int
	TotalHugepages    int64
	TotalMemory       int64
	TotalCores        int
	TotalCPUMilli     int64
	AvailableMemory   int64
	AvailableHugepage int64
	CanFit            bool
	Issues            []string
}

func runPlanClient(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	filePath := args[0]

	wekaClient, err := ParseWekaResourceFile[*wekaapi.WekaClient](filePath)
	if err != nil {
		return fmt.Errorf("failed to parse WekaClient file: %w", err)
	}

	fmt.Printf("=== Planning WekaClient Deployment: %s ===\n\n", wekaClient.Name)

	// Get cluster nodes
	nodes, err := GetClusterNodes(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Could not access cluster nodes: %v\n", err)
		_, _ = fmt.Fprintf(os.Stderr, "Continuing with planning without node validation...\n\n")
		nodes = nil
	}

	// Validate client and plan allocation
	if err := validateAndPlanClient(ctx, wekaClient, nodes); err != nil {
		return err
	}

	return nil
}

func validateAndPlanClient(ctx context.Context, client *wekaapi.WekaClient, nodes []corev1.Node) error {
	// Print client specification
	fmt.Println("=== Client Specification ===")
	printClientSpec(client)

	// Validate client configuration
	fmt.Println("\n=== Validating Client Configuration ===")
	if err := validateClientConfig(client); err != nil {
		return err
	}
	fmt.Println("✓ Configuration valid")

	// Calculate container requirements
	fmt.Println("\n=== Container Resource Requirements ===")
	containerReqs := calculateClientContainerRequirements(client)
	printClientContainerRequirements(containerReqs)

	// If nodes are not available, skip allocation simulation
	if nodes == nil || len(nodes) == 0 {
		fmt.Println("\n⚠️  Cluster nodes not available - skipping allocation simulation")
		return nil
	}

	fmt.Println("\n=== Fetching Cluster Resource Information ===")

	// Collect pod data from cluster
	podsByNode := make(map[string][]corev1.Pod)
	crClient := KubeClients.CRClient

	var podList corev1.PodList
	if err := crClient.List(ctx, &podList); err == nil {
		// Group pods by node
		for _, pod := range podList.Items {
			if pod.Spec.NodeName != "" {
				podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
			}
		}
	}
	fmt.Printf("✓ Collected pod data from cluster\n")

	// Find matching nodes
	fmt.Println("\n=== Finding Matching Nodes ===")
	matchingNodes := FilterNodesBySelector(nodes, client.Spec.NodeSelector)
	fmt.Printf("Found %d nodes matching selector: %s\n", len(matchingNodes), SelectorToString(client.Spec.NodeSelector))

	if len(matchingNodes) == 0 {
		fmt.Println("⚠️  WARNING: No nodes match the nodeSelector")
		return nil
	}

	// Print matching nodes table
	printClientNodesTable(matchingNodes, podsByNode)

	// Simulate allocation
	fmt.Println("\n=== Simulating Container Placement ===")
	allocations := simulateClientAllocation(matchingNodes, podsByNode, containerReqs)

	// Convert to NodePlacement format for reusing existing visualization
	placements := convertClientAllocationsToNodePlacements(allocations)

	// Print placement details with resource allocation (reuses cluster function)
	fmt.Println("\n=== Placement Details & Resource Allocation ===")
	printPlacementDetailsWithResourceAllocation(placements, matchingNodes, podsByNode)

	// Check if allocation is feasible
	fmt.Println("\n=== Allocation Summary ===")
	printClientAllocationFeasibility(allocations)

	fmt.Println("\n✓ Plan complete!")
	return nil
}

func printClientSpec(client *wekaapi.WekaClient) {
	fmt.Printf("Name:               %s\n", client.Name)
	fmt.Printf("Namespace:          %s\n", client.Namespace)
	fmt.Printf("Image:              %s\n", client.Spec.Image)
	fmt.Printf("Cores Number:       %d\n", client.Spec.CoresNumber)
	fmt.Printf("CPU Policy:         %s\n", client.Spec.CpuPolicy)

	if client.Spec.TargetCluster.Name != "" {
		fmt.Printf("Target Cluster:     %s/%s\n", client.Spec.TargetCluster.Namespace, client.Spec.TargetCluster.Name)
	} else if len(client.Spec.JoinIps) > 0 {
		fmt.Printf("Join IPs:           %v\n", client.Spec.JoinIps)
	}

	fmt.Printf("Node Selector:      %v\n", client.Spec.NodeSelector)
}

func validateClientConfig(client *wekaapi.WekaClient) error {
	// Validate that either targetCluster or joinIps is set
	if client.Spec.TargetCluster.Name == "" && len(client.Spec.JoinIps) == 0 {
		return fmt.Errorf("either targetCluster or joinIps must be specified")
	}

	// Validate coresNum is set
	if client.Spec.CoresNumber <= 0 {
		return fmt.Errorf("coresNum must be greater than 0")
	}

	// Validate image is set
	if client.Spec.Image == "" {
		return fmt.Errorf("image must be specified")
	}

	return nil
}

func calculateClientContainerRequirements(client *wekaapi.WekaClient) ClientContainerRequirements {
	coresNum := client.Spec.CoresNumber
	cpuPolicy := client.Spec.CpuPolicy

	// Determine if HT is enabled
	usesHT := cpuPolicy == wekaapi.CpuPolicyDedicatedHT || cpuPolicy == wekaapi.CpuPolicyAuto

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

func printClientContainerRequirements(req ClientContainerRequirements) {
	fmt.Printf("Per Container Requirements:\n")
	fmt.Printf("  CPU Cores (with HT):    %d\n", req.Cores)
	fmt.Printf("  CPU Cores (no HT):      %d\n", req.CoresNoHT)
	fmt.Printf("  Hugepages:              %dMi\n", req.Hugepages)
	fmt.Printf("  Memory:                 %.2fGi\n", float64(req.Memory)/float64(1024*1024*1024))
	fmt.Printf("  CPU (milli):            %dm\n", req.CPUMilli)
}

func printClientNodesTable(nodes []corev1.Node, podsByNode map[string][]corev1.Pod) {
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
	t.Render()
}

func simulateClientAllocation(nodes []corev1.Node, podsByNode map[string][]corev1.Pod, containerReqs ClientContainerRequirements) []ClientNodeAllocation {
	var allocations []ClientNodeAllocation

	for _, node := range nodes {
		alloc := ClientNodeAllocation{
			NodeName:       node.Name,
			ContainerCount: 1, // 1 client container per node
		}

		// Calculate available resources
		memQuantity := node.Status.Allocatable[corev1.ResourceMemory]
		hpQuantity := node.Status.Allocatable["hugepages-2Mi"]

		// Subtract pod resources
		usedMem := calculatePodResourceUsage(podsByNode[node.Name], corev1.ResourceMemory)
		usedHP := calculatePodResourceUsage(podsByNode[node.Name], "hugepages-2Mi")

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

	for _, alloc := range allocations {
		if alloc.CanFit {
			successCount++
		} else {
			failureCount++
		}
	}

	fmt.Printf("✓ %d nodes can accommodate the client container\n", successCount)
	if failureCount > 0 {
		fmt.Printf("✗ %d nodes CANNOT accommodate the client container\n", failureCount)
	}

	if failureCount == 0 {
		fmt.Println("\n✅ All nodes can accommodate client containers")
	} else {
		fmt.Printf("\n⚠️  %d nodes have insufficient resources\n", failureCount)
	}
}
