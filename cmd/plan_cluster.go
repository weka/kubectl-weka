package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"sort"
	"strings"
)

var (
	planClusterFailFast bool
)

// RoleNodeGroup represents nodes targeting a specific role with their associated selector
type RoleNodeGroup struct {
	Role     string
	Selector map[string]string
	Nodes    []corev1.Node
}

// RoleNodeGrouping represents all role-based node groupings
type RoleNodeGrouping struct {
	Global []corev1.Node            // Nodes matching global nodeSelector
	ByRole map[string]RoleNodeGroup // Nodes per role (compute, drive, s3, nfs)
}

// ScheduledPodAllocation represents the simulated scheduling of pods to nodes
type ScheduledPodAllocation struct {
	Role            string        // Role name (compute, drive, s3, nfs)
	TotalContainers int           // Total containers of this role to schedule
	AllocatedNodes  []corev1.Node // Nodes where containers will be allocated
	PodsPerNode     int           // Average pods per node
	Description     string        // Human-readable description of allocation
}

// ContainerPlacement represents a single container placement on a node
type ContainerPlacement struct {
	Role      string // compute, drive, s3, nfs, envoy
	Index     int    // Which container number (0-7, etc)
	NodeName  string // Which node it's placed on
	Cores     int    // Cores required
	Memory    int64  // Memory required in MiB
	Hugepages int64  // Hugepages required in MiB
}

// NodeAllocationGroup represents nodes in a group with their placements and free resources
type NodeAllocationGroup struct {
	Role             string                          // Which role this group is for
	Selector         map[string]string               // Node selector for this group
	Nodes            map[string]*NodeAllocationState // All nodes in this group keyed by name
	PlacedContainers []ContainerPlacement            // Containers successfully placed
	FailureReason    string                          // If allocation failed, why
}

// NodeAllocationState tracks the resource state of a node during allocation
type NodeAllocationState struct {
	Node               *corev1.Node
	AllocatedCores     int
	AllocatedMemory    int64
	AllocatedHugepages int64
	PlacedRoles        map[string]int    // Count of containers by role on this node
	HostCheck          *HostChecksResult // Host check data if available
}

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

	// SANITY CHECK: Validate cluster definition before doing anything else
	if err := sanityCheckClusterDefinition(cluster); err != nil {
		fmt.Printf("\n❌ Cluster Definition Validation Failed:\n%v\n", err)
		return err
	}

	nodes, err := GetClusterNodes(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Could not access cluster nodes: %v\n", err)
		_, _ = fmt.Fprintf(os.Stderr, "Continuing with planning without cluster validation...\n\n")
		nodes = nil
	}

	if err := validateAndPlan(ctx, KubeClients, cluster, nodes); err != nil {
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

// sanityCheckClusterDefinition performs basic validation of the cluster definition
// before any planning calculations or cluster connections are made
func sanityCheckClusterDefinition(cluster *wekaapi.WekaCluster) error {
	fmt.Println("=== Cluster Definition Sanity Check ===")

	if cluster.Spec.Dynamic == nil {
		return fmt.Errorf("❌ dynamic template is required")
	}

	// Check UDP mode warning
	if cluster.Spec.Network.UdpMode {
		fmt.Printf("\n⚠️  WARNING: UDP mode is enabled for cluster. This is not recommended for fast-performance production environments\n")
	}

	// Check hot spare configuration
	if cluster.Spec.HotSpare == 0 {
		fmt.Printf("\n⚠️  WARNING: Hot spare is set to 0. At least 1 hot spare is recommended for production clusters to handle drive failures\n")
	}

	// Check driver distribution service
	if cluster.Spec.DriversDistService != "" {
		if err := sanityCheckDriverDistService(cluster.Spec.DriversDistService); err != nil {
			return err
		}
	}

	// Check ethDevice name validity
	if err := sanityCheckNetworkConfig(cluster.Spec.Network); err != nil {
		return err
	}

	fmt.Println("✅ Cluster definition validation passed")
	return nil
}

// sanityCheckNetworkConfig validates network configuration parameters
func sanityCheckNetworkConfig(network wekaapi.Network) error {
	ethDevice := network.EthDevice
	if ethDevice == "" && len(network.EthDevices) == 0 {
		return nil // No network config specified, that's OK
	}

	if ethDevice == "" && len(network.EthDevices) > 0 {
		ethDevice = network.EthDevices[0]
	}

	if ethDevice == "" {
		return nil
	}

	// Check if ethDevice name is obviously invalid (too short)
	if len(ethDevice) <= 2 {
		return fmt.Errorf("❌ invalid ethDevice '%s': name is too short. Common interface names are: eth0, eth1, bond0, bond1, etc.", ethDevice)
	}

	// Check for valid characters in interface name (alphanumeric, hyphens, underscores, and dots for VLAN)
	for _, ch := range ethDevice {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.') {
			return fmt.Errorf("❌ invalid ethDevice '%s': contains invalid character '%c'. Use alphanumeric characters, hyphens, underscores, or dots (for VLAN interfaces like bond0.12)", ethDevice, ch)
		}
	}

	fmt.Printf("✓ ethDevice '%s' name format is valid\n", ethDevice)
	return nil
}

func sanityCheckDriverDistService(url string) error {
	if url == "" {
		return nil
	}

	// Check if URL ends with cluster.local:<port>
	// Example: https://weka-driver-dist.weka-system.svc.cluster.local:14000
	if !strings.Contains(url, "cluster.local:") {
		// External URL, no validation needed
		fmt.Printf("✓ driversDistService points to external URL: %s\n", url)
		return nil
	}

	// Parse the service name and namespace from cluster.local URL
	// Format: https://<service>.<namespace>.svc.cluster.local:<port>
	serviceName, namespace, err := parseClusterLocalService(url)
	if err != nil {
		return fmt.Errorf("❌ failed to parse cluster.local service URL '%s': %v", url, err)
	}

	fmt.Printf("  Checking if service '%s' exists in namespace '%s'...\n", serviceName, namespace)

	// Get kubernetes client to check if service exists
	ctx := context.Background()
	cfg, err := GetKubeConfig()
	if err != nil {
		// Can't connect to cluster yet - skip service validation
		fmt.Printf("⚠️  WARNING: Cannot connect to cluster to verify service existence. Will validate later.\n")
		return nil
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("⚠️  WARNING: Cannot create kubernetes client to verify service. Will validate later.\n")
		return nil
	}

	// Check if service exists
	_, err = clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("❌ Driver distribution service '%s' not found in namespace '%s'.\n"+
				"   This usually means the DriverDistribution WekaPolicy is not installed.\n"+
				"   Please install the DriverDistribution WekaPolicy first:\n"+
				"   kubectl apply -f <driver-distribution-policy.yaml>", serviceName, namespace)
		}
		return fmt.Errorf("❌ failed to check service '%s' in namespace '%s': %v", serviceName, namespace, err)
	}

	fmt.Printf("✓ driversDistService '%s.%s' exists\n", serviceName, namespace)
	return nil
}

// parseClusterLocalService extracts service name and namespace from cluster.local URL
// Example: https://weka-driver-dist.weka-system.svc.cluster.local:14000
// Returns: serviceName="weka-driver-dist", namespace="weka-system"
func parseClusterLocalService(url string) (string, string, error) {
	// Remove protocol prefix
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Split by ':' to remove port
	parts := strings.Split(url, ":")
	if len(parts) == 0 {
		return "", "", fmt.Errorf("invalid URL format")
	}
	hostPart := parts[0]

	// Expected format: <service>.<namespace>.svc.cluster.local
	segments := strings.Split(hostPart, ".")
	if len(segments) < 5 {
		return "", "", fmt.Errorf("invalid cluster.local format, expected <service>.<namespace>.svc.cluster.local")
	}

	serviceName := segments[0]
	namespace := segments[1]

	if serviceName == "" || namespace == "" {
		return "", "", fmt.Errorf("service name or namespace is empty")
	}

	return serviceName, namespace, nil
}

func validateAndPlan(ctx context.Context, clients *K8sClients, cluster *wekaapi.WekaCluster, nodes []corev1.Node) error {
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
		req.Type = "Compute"
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
		req.Type = "Drive"
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
		req.Type = "S3"
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
		envoyReq.Type = "Envoy (S3)"
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
		req.Type = "NFS"
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

	// Validate cluster has sufficient resources
	if nodes != nil {
		if err := validateClusterResources(ctx, clients, cluster, nodes, nodeReqs, containers); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "\n❌ Cluster Validation Failed:\n%v\n", err)
		}
	}

	return nil
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
	req.CoresNoHT = 1
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
		if c.Type == "Compute" {
			computeCount = c.Count
			computeReq = c
		} else if c.Type == "Drive" {
			driveCount = c.Count
			driveReq = c
		}
	}

	if computeCount > 0 || driveCount > 0 {
		// With anti-affinity, compute and drive containers need separate nodes
		// So we need to sum them, not take the max
		backendNodes := computeCount + driveCount

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
		case "S3":
			s3Count = c.Count
			s3Req = c
		case "NFS":
			nfsCount = c.Count
			nfsReq = c
		case "Envoy (S3)":
			envoyReq = c
		}
	}

	if s3Count > 0 || nfsCount > 0 {
		// With anti-affinity, S3 and NFS containers need separate nodes
		// So we need to sum them, not take the max
		frontendNodes := s3Count + nfsCount

		totalCores := 0
		totalCoresNoHT := 0
		totalHugepages := int64(0)
		totalMemory := int64(0)

		if s3Count > 0 {
			totalCores += s3Req.Cores + envoyReq.Cores
			totalCoresNoHT += s3Req.CoresNoHT + envoyReq.CoresNoHT
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

func validateClusterResources(ctx context.Context, clients *K8sClients, cluster *wekaapi.WekaCluster, nodes []corev1.Node, nodeReqs []NodeRequirements, containers []ContainerRequirements) error {
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in cluster")
	}

	config := cluster.Spec.Dynamic
	network := &cluster.Spec.Network
	nodeSelector := cluster.Spec.NodeSelector
	roleNodeSelector := &cluster.Spec.RoleNodeSelector

	// Check UDP mode warning
	if network.UdpMode {
		fmt.Printf("\n⚠️  WARNING: UDP mode is enabled for cluster. This is not recommended for fast-performance production environments\n\n")
	}

	// Build role-based node grouping (union of all nodes that match any selector)
	roleGrouping := buildRoleNodeGrouping(nodes, nodeSelector, roleNodeSelector)

	// Check if we have any eligible nodes across all roles
	totalEligibleNodes := len(roleGrouping.Global)
	for _, roleGroup := range roleGrouping.ByRole {
		if len(roleGroup.Nodes) > totalEligibleNodes {
			totalEligibleNodes = len(roleGroup.Nodes)
		}
	}

	if totalEligibleNodes == 0 {
		return fmt.Errorf("no nodes match any nodeSelector criteria (global: %v, roles: %+v)", nodeSelector, roleNodeSelector)
	}

	// Display role-based node allocation
	printRoleNodeGrouping(roleGrouping)

	// Get union of all nodes that match any role selector for resource inspection
	allEligibleNodes := make(map[string]corev1.Node)
	for _, node := range roleGrouping.Global {
		allEligibleNodes[node.Name] = node
	}
	for _, roleGroup := range roleGrouping.ByRole {
		for _, node := range roleGroup.Nodes {
			allEligibleNodes[node.Name] = node
		}
	}

	// Convert map back to slice
	var allEligibleNodesList []corev1.Node
	for _, node := range allEligibleNodes {
		allEligibleNodesList = append(allEligibleNodesList, node)
	}
	sort.Slice(allEligibleNodesList, func(i, j int) bool {
		return allEligibleNodesList[i].Name < allEligibleNodesList[j].Name
	})

	// STEP 1: Get current pod data to calculate resource usage
	fmt.Println("\n=== Fetching Cluster Resource Information ===")
	var podsByNode map[string][]corev1.Pod
	clientset := clients.Clientset
	if clientset != nil {
		podList, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("⚠️  Warning: Could not get pod data: %v\n", err)
			podsByNode = make(map[string][]corev1.Pod)
		} else {
			podsByNode = make(map[string][]corev1.Pod)
			for _, pod := range podList.Items {
				podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
			}
			fmt.Printf("✓ Collected pod data from cluster\n")
		}
	} else {
		podsByNode = make(map[string][]corev1.Pod)
	}

	// STEP 2: Present table of all nodes matching selection per nodeSelector
	fmt.Println("\n=== Nodes Matching Selection Criteria ===")
	printNodesPerSelector(roleGrouping, allEligibleNodesList, podsByNode)

	// STEP 3: Check for overlapping nodeSelectors and warn
	checkOverlappingSelectors(roleGrouping)

	// STEP 4: Perform pseudo-placement (only if --show-only-schedulable flag is set)
	var allocationGroups map[string]*NodeAllocationGroup
	var allocationErr error

	fmt.Println("\n=== Simulating Container Placement ===")
	if len(roleGrouping.ByRole) > 0 {

		// Create empty hostChecksMap for initial allocation (we don't have host check data yet)
		emptyHostChecks := make(map[string]HostChecksResult)
		allocationGroups, allocationErr = simulateActualScheduling(roleGrouping, containers, emptyHostChecks, allEligibleNodesList, podsByNode)
		if allocationErr != nil {
			fmt.Printf("❌ Scheduling failed: %v\n", allocationErr)
		} else {
			// STEP 5 & 6: Combined placement details and resource visualization
			printPlacementDetailsWithResourceBars(allocationGroups, allEligibleNodesList, podsByNode)
		}
	}

	// Validate drives on nodes that will have drive containers
	if config.DriveContainers != nil && *config.DriveContainers > 0 && config.NumDrives > 0 {
		driveNodes := roleGrouping.ByRole["drive"].Nodes
		if len(driveNodes) == 0 {
			driveNodes = roleGrouping.Global // Fallback to global nodes
		}
		if err := validateDrivesOnNodes(driveNodes, *config.DriveContainers, config.NumDrives); err != nil {
			return err
		}
	}

	// Validate network configuration on all eligible nodes
	if err := validateNetworkConfiguration(ctx, clients, allEligibleNodesList, network); err != nil {
		return err
	}

	// Validate resource availability per role
	if err := validateResourceAvailabilityPerRole(roleGrouping, nodeReqs); err != nil {
		return err
	}

	fmt.Printf("\n✅ Cluster validation passed\n")
	fmt.Printf("   ✓ %d total nodes in cluster\n", len(nodes))
	fmt.Printf("   ✓ %d nodes eligible for Weka deployment\n", len(allEligibleNodesList))
	fmt.Printf("   ✓ Role-based node allocation configured\n")
	fmt.Printf("   ✓ All required drives are available\n")
	fmt.Printf("   ✓ Network configuration is consistent\n")
	fmt.Printf("   ✓ Sufficient resources available per role\n")

	return nil
}

func validateDrivesOnNodes(nodes []corev1.Node, driveContainers, numDrives int) error {
	totalDrivesNeeded := driveContainers * numDrives
	totalDrivesAvailable := 0
	nodesWithDrives := 0

	for _, node := range nodes {
		if drivesAnnotation, ok := node.Annotations["weka.io/weka-drives"]; ok {
			var drives []string
			if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
				totalDrivesAvailable += len(drives)
				if len(drives) > 0 {
					nodesWithDrives++
				}
			}
		}
	}

	if nodesWithDrives == 0 {
		return fmt.Errorf("drives were not discovered on hosts. Please install weka operator and drive signing policy, then rerun the command")
	}

	if totalDrivesAvailable < totalDrivesNeeded {
		return fmt.Errorf("insufficient drives: need %d drives (%d containers × %d drives), but only %d available across %d nodes with drives",
			totalDrivesNeeded, driveContainers, numDrives, totalDrivesAvailable, nodesWithDrives)
	}

	return nil
}

func validateNetworkConfiguration(ctx context.Context, clients *K8sClients, nodes []corev1.Node, network *wekaapi.Network) error {
	if network == nil {
		return nil
	}

	ethDevice := network.EthDevice
	if ethDevice == "" && len(network.EthDevices) == 0 {
		return nil // No network validation required
	}

	if ethDevice == "" && len(network.EthDevices) > 0 {
		ethDevice = network.EthDevices[0]
	}

	if ethDevice == "" {
		return nil
	}

	// Use the shared network validation function with empty host check data (will be populated later if needed)
	emptyHostChecks := make(map[string]HostChecksResult)
	return validateNetworkInterfaceOnNodes(ctx, clients, nodes, ethDevice, planClusterFailFast, emptyHostChecks)
}

func validateResourceAvailabilityPerRole(grouping RoleNodeGrouping, nodeReqs []NodeRequirements) error {
	if len(nodeReqs) == 0 {
		return nil
	}

	var issues []string

	// Validate backend (compute+drive) requirements
	backendNodes := grouping.ByRole["compute"].Nodes
	if len(backendNodes) == 0 {
		backendNodes = grouping.ByRole["drive"].Nodes
	}
	if len(backendNodes) == 0 && len(grouping.Global) > 0 {
		backendNodes = grouping.Global // Fallback to global
	}

	for _, req := range nodeReqs {
		if req.Purpose == "Backend (Compute+Drive)" {
			if req.MinNodes > len(backendNodes) {
				issues = append(issues, fmt.Sprintf(
					"Backend (%s): requires %d nodes but only %d match backend selectors (need %d cores/node, %d MiB hugepages/node, %d MiB memory/node)",
					req.Description, req.MinNodes, len(backendNodes),
					req.CoresPerNode, req.HugepagesPerNode, req.MemoryPerNode,
				))
			}
		}
	}

	// Validate frontend (s3/nfs) requirements
	frontendNodes := grouping.ByRole["s3"].Nodes
	if len(frontendNodes) == 0 {
		frontendNodes = grouping.ByRole["nfs"].Nodes
	}
	if len(frontendNodes) == 0 && len(grouping.Global) > 0 {
		frontendNodes = grouping.Global // Fallback to global
	}

	for _, req := range nodeReqs {
		if req.Purpose == "Frontend (S3/NFS)" {
			if req.MinNodes > len(frontendNodes) {
				issues = append(issues, fmt.Sprintf(
					"Frontend (%s): requires %d nodes but only %d match frontend selectors (need %d cores/node, %d MiB hugepages/node, %d MiB memory/node)",
					req.Description, req.MinNodes, len(frontendNodes),
					req.CoresPerNode, req.HugepagesPerNode, req.MemoryPerNode,
				))
			}
		}
	}

	if len(issues) > 0 {
		errMsg := "insufficient cluster nodes for role requirements:\n"
		for _, issue := range issues {
			errMsg += "  - " + issue + "\n"
		}
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// buildRoleNodeGrouping creates role-based node groups based on global and role-specific selectors
func buildRoleNodeGrouping(allNodes []corev1.Node, globalSelector map[string]string, roleSelectors *wekaapi.RoleNodeSelector) RoleNodeGrouping {
	grouping := RoleNodeGrouping{
		Global: []corev1.Node{},
		ByRole: make(map[string]RoleNodeGroup),
	}

	// First, filter nodes matching global selector
	globalNodes := FilterNodesBySelector(allNodes, globalSelector)
	grouping.Global = globalNodes

	// If no roleNodeSelector provided, use global nodes for all roles
	if roleSelectors == nil {
		for _, role := range []string{"compute", "drive", "s3", "nfs"} {
			grouping.ByRole[role] = RoleNodeGroup{
				Role:     role,
				Selector: globalSelector,
				Nodes:    globalNodes,
			}
		}
		return grouping
	}

	// Define role mapping
	roles := map[string]*map[string]string{
		"compute": roleSelectors.Compute,
		"drive":   roleSelectors.Drive,
		"s3":      roleSelectors.S3,
		"nfs":     roleSelectors.Nfs,
	}

	// For each role, determine target nodes
	for roleName, roleSelector := range roles {
		if roleSelector == nil {
			// No role-specific selector, use global nodes
			grouping.ByRole[roleName] = RoleNodeGroup{
				Role:     roleName,
				Selector: globalSelector,
				Nodes:    globalNodes,
			}
		} else {
			// Combine global and role-specific selectors
			combinedSelector := MergeSelectorMaps(globalSelector, *roleSelector)
			roleNodes := FilterNodesBySelector(allNodes, combinedSelector)
			grouping.ByRole[roleName] = RoleNodeGroup{
				Role:     roleName,
				Selector: combinedSelector,
				Nodes:    roleNodes,
			}
		}
	}

	return grouping
}

// mergeSelectors uses role-specific selector if provided, otherwise falls back to global
// Role-specific selector completely replaces global selector (not merged)

// printRoleNodeGrouping prints information about role-based node groupings
func printRoleNodeGrouping(grouping RoleNodeGrouping) {
	fmt.Println("\n=== Role-Based Node Allocation ===")

	if len(grouping.Global) > 0 {
		fmt.Printf("Global NodeSelector matches: %d nodes\n\n", len(grouping.Global))
	}

	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		if rng, exists := grouping.ByRole[role]; exists {
			fmt.Printf("%s role:\n", strings.ToUpper(role[:1])+role[1:])
			fmt.Printf("  Selector: %s\n", SelectorToString(rng.Selector))
			fmt.Printf("  Target nodes: %d\n", len(rng.Nodes))
			if len(rng.Nodes) > 0 {
				nodeNames := make([]string, len(rng.Nodes))
				for i, n := range rng.Nodes {
					nodeNames[i] = n.Name
				}
				fmt.Printf("  Node list: %v\n", nodeNames)
			} else {
				fmt.Printf("  ⚠️  WARNING: No nodes match selector!\n")
			}
			fmt.Println()
		}
	}
}

// simulateActualScheduling simulates actual Kubernetes scheduler behavior by iteratively
// placing containers one by one, respecting anti-affinity rules and tracking resource availability
func simulateActualScheduling(
	roleGrouping RoleNodeGrouping,
	containers []ContainerRequirements,
	hostChecksMap map[string]HostChecksResult,
	allNodes []corev1.Node,
	podsByNode map[string][]corev1.Pod,
) (map[string]*NodeAllocationGroup, error) {

	// Create a map of nodes for quick lookup
	nodeMap := make(map[string]*corev1.Node)
	for i := range allNodes {
		nodeMap[allNodes[i].Name] = &allNodes[i]
	}

	// Initialize allocation groups for each role
	allocationGroups := make(map[string]*NodeAllocationGroup)
	containerCountByRole := make(map[string]int)

	for _, c := range containers {
		if c.Type == "Compute" {
			containerCountByRole["compute"] = c.Count
		} else if c.Type == "Drive" {
			containerCountByRole["drive"] = c.Count
		} else if c.Type == "S3" {
			containerCountByRole["s3"] = c.Count
		} else if c.Type == "NFS" {
			containerCountByRole["nfs"] = c.Count
		} else if c.Type == "Envoy (S3)" {
			containerCountByRole["envoy"] = c.Count
		}
	}

	// Initialize allocation groups with all available nodes
	for _, role := range []string{"compute", "drive", "s3", "nfs", "envoy"} {
		if containerCountByRole[role] == 0 {
			continue // Skip roles not needed
		}

		rng, exists := roleGrouping.ByRole[role]
		if !exists {
			// For envoy, use s3 nodes
			rng = roleGrouping.ByRole["s3"]
		}

		// Initialize node allocation states for this role group
		nodeStates := make(map[string]*NodeAllocationState)
		for _, node := range rng.Nodes {
			if k8sNode, ok := nodeMap[node.Name]; ok {
				hostCheck := hostChecksMap[node.Name]

				// Calculate current pod resource usage on this node
				var allocatedCores int
				var allocatedMemory int64
				var allocatedHugepages int64

				podsOnNode := podsByNode[node.Name]
				for _, pod := range podsOnNode {
					// Skip non-running pods
					if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
						continue
					}

					// Sum container requests
					for _, container := range pod.Spec.Containers {
						cpuReq := quantityOrZero(container.Resources.Requests, corev1.ResourceCPU)
						memReq := quantityOrZero(container.Resources.Requests, corev1.ResourceMemory)
						hpReq := quantityOrZero(container.Resources.Requests, "hugepages-2Mi")

						allocatedCores += int(cpuReq.MilliValue() / 1000)   // Convert millicores to cores
						allocatedMemory += memReq.Value() / (1024 * 1024)   // Convert bytes to MiB
						allocatedHugepages += hpReq.Value() / (1024 * 1024) // Convert bytes to MiB
					}
				}

				nodeStates[node.Name] = &NodeAllocationState{
					Node:               k8sNode,
					AllocatedCores:     allocatedCores,
					AllocatedMemory:    allocatedMemory,
					AllocatedHugepages: allocatedHugepages,
					PlacedRoles:        make(map[string]int),
					HostCheck:          &hostCheck,
				}
			}
		}

		allocationGroups[role] = &NodeAllocationGroup{
			Role:             role,
			Selector:         rng.Selector,
			Nodes:            nodeStates,
			PlacedContainers: []ContainerPlacement{},
		}
	}

	// Show available nodes per role
	fmt.Println("Available nodes per role:")
	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		group, exists := allocationGroups[role]
		if exists && len(group.Nodes) > 0 {
			fmt.Printf("  %s: %d nodes available\n", role, len(group.Nodes))
		}
	}
	fmt.Println()

	for _, containerReq := range containers {
		role := strings.ToLower(containerReq.Type)
		if role == "envoy (s3)" {
			role = "envoy"
		}

		count := containerReq.Count
		if count == 0 {
			continue
		}

		group, exists := allocationGroups[role]
		if !exists {
			continue
		}

		fmt.Printf("Allocating %d %s container(s):\n", count, role)

		// Allocate 'count' containers of this role
		for containerIdx := 0; containerIdx < count; containerIdx++ {
			// Find best available node for this container
			bestNode, err := findBestNodeForContainer(
				group.Nodes,
				role,
				containerReq,
			)

			if err != nil {
				group.FailureReason = fmt.Sprintf(
					"Cannot allocate %s container %d/%d: %v",
					role, containerIdx+1, count, err,
				)
				return allocationGroups, fmt.Errorf(
					"allocation failed for %s container %d: %v",
					role, containerIdx+1, err,
				)
			}

			// Place container on best node
			nodeStates := group.Nodes[bestNode.Name]
			nodeStates.AllocatedCores += int(containerReq.Cores)
			nodeStates.AllocatedMemory += containerReq.Memory
			nodeStates.AllocatedHugepages += containerReq.Hugepages
			nodeStates.PlacedRoles[role]++

			placement := ContainerPlacement{
				Role:      role,
				Index:     containerIdx,
				NodeName:  bestNode.Name,
				Cores:     int(containerReq.Cores),
				Memory:    containerReq.Memory,
				Hugepages: containerReq.Hugepages,
			}
			group.PlacedContainers = append(group.PlacedContainers, placement)

			// Print placement info
			fmt.Printf("  ✓ Placed %s container #%d on node %s (Cores: %d, Memory: %d MiB, Hugepages: %d MiB)\n",
				role, containerIdx, bestNode.Name, placement.Cores, placement.Memory, placement.Hugepages)
		}
	}

	return allocationGroups, nil
}

// findBestNodeForContainer finds the best available node for placing a container
// respecting anti-affinity rules and resource constraints
func findBestNodeForContainer(
	nodeStates map[string]*NodeAllocationState,
	role string,
	containerReq ContainerRequirements,
) (*corev1.Node, error) {

	var bestNode *NodeAllocationState
	bestAvailableResources := int64(-1)

	for _, nodeState := range nodeStates {
		if nodeState == nil {
			continue
		}

		k8sNode := nodeState.Node

		// Check anti-affinity: no same-role containers on same node
		if nodeState.PlacedRoles[role] > 0 {
			continue // This node already has a container of this role
		}

		// Get allocatable resources
		cpuAlloc := quantityOrZero(k8sNode.Status.Allocatable, corev1.ResourceCPU)
		memAlloc := quantityOrZero(k8sNode.Status.Allocatable, corev1.ResourceMemory)
		hpAlloc := quantityOrZero(k8sNode.Status.Allocatable, "hugepages-2Mi")

		// Calculate already allocated (from actual pods + our allocations)
		cpuAllocated := resource.NewMilliQuantity(int64(nodeState.AllocatedCores*1000), resource.DecimalSI)
		memAllocated := resource.NewQuantity(nodeState.AllocatedMemory*1024*1024, resource.BinarySI)
		hpAllocated := resource.NewQuantity(nodeState.AllocatedHugepages*1024*1024, resource.BinarySI)

		// Calculate free resources
		cpuFree := cpuAlloc.DeepCopy()
		cpuFree.Sub(*cpuAllocated)

		memFree := memAlloc.DeepCopy()
		memFree.Sub(*memAllocated)

		hpFree := hpAlloc.DeepCopy()
		hpFree.Sub(*hpAllocated)

		// Check if container fits
		cpuNeeded := resource.NewMilliQuantity(int64(containerReq.Cores*1000), resource.DecimalSI)
		memNeeded := resource.NewQuantity(containerReq.Memory*1024*1024, resource.BinarySI)
		hpNeeded := resource.NewQuantity(containerReq.Hugepages*1024*1024, resource.BinarySI)

		if cpuFree.Cmp(*cpuNeeded) < 0 {
			continue // Not enough CPU
		}
		if memFree.Cmp(*memNeeded) < 0 {
			continue // Not enough memory
		}
		if hpFree.Cmp(*hpNeeded) < 0 {
			continue // Not enough hugepages
		}

		// For drive containers, check available drives from annotation
		if role == "drive" {
			drivesNeeded := 1
			drivesAvailable := 0

			// Check node annotation for available drives
			if drivesAnnotation, ok := k8sNode.Annotations["weka.io/weka-drives"]; ok {
				var drives []string
				if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
					drivesAvailable = len(drives)
				}
			}

			// Subtract already allocated drives on this node
			drivesUsed := nodeState.PlacedRoles["drive"]
			drivesFree := drivesAvailable - drivesUsed

			if drivesFree < drivesNeeded {
				continue // Not enough drives available
			}
		}

		// This node is suitable; prefer the one with most free resources
		availableRes := memFree.Value() // Use memory as primary metric for "best"
		if availableRes > bestAvailableResources {
			bestAvailableResources = availableRes
			bestNode = nodeState
		}
	}

	if bestNode == nil {
		// Provide detailed error message
		var errMsg strings.Builder
		errMsg.WriteString(fmt.Sprintf("no suitable nodes available for %s container %d\n", role, 1))
		errMsg.WriteString(fmt.Sprintf("  Analyzed %d nodes:\n", len(nodeStates)))

		antiAffinityCount := 0
		insufficientCoresCount := 0
		insufficientMemCount := 0
		insufficientHPCount := 0
		insufficientDrivesCount := 0

		for _, nodeState := range nodeStates {
			if nodeState == nil {
				continue
			}

			k8sNode := nodeState.Node

			// Check anti-affinity
			if nodeState.PlacedRoles[role] > 0 {
				antiAffinityCount++
				continue
			}

			// Check resources
			cpuAlloc := quantityOrZero(k8sNode.Status.Allocatable, corev1.ResourceCPU)
			memAlloc := quantityOrZero(k8sNode.Status.Allocatable, corev1.ResourceMemory)
			hpAlloc := quantityOrZero(k8sNode.Status.Allocatable, "hugepages-2Mi")

			cpuAllocated := resource.NewMilliQuantity(int64(nodeState.AllocatedCores*1000), resource.DecimalSI)
			memAllocated := resource.NewQuantity(nodeState.AllocatedMemory*1024*1024, resource.BinarySI)
			hpAllocated := resource.NewQuantity(nodeState.AllocatedHugepages*1024*1024, resource.BinarySI)

			cpuFree := cpuAlloc.DeepCopy()
			cpuFree.Sub(*cpuAllocated)
			memFree := memAlloc.DeepCopy()
			memFree.Sub(*memAllocated)
			hpFree := hpAlloc.DeepCopy()
			hpFree.Sub(*hpAllocated)

			cpuNeeded := resource.NewMilliQuantity(int64(containerReq.Cores*1000), resource.DecimalSI)
			memNeeded := resource.NewQuantity(containerReq.Memory*1024*1024, resource.BinarySI)
			hpNeeded := resource.NewQuantity(containerReq.Hugepages*1024*1024, resource.BinarySI)

			if cpuFree.Cmp(*cpuNeeded) < 0 {
				insufficientCoresCount++
				continue
			}
			if memFree.Cmp(*memNeeded) < 0 {
				insufficientMemCount++
				continue
			}
			if hpFree.Cmp(*hpNeeded) < 0 {
				insufficientHPCount++
				continue
			}

			// Check drives for drive containers via annotation
			if role == "drive" {
				drivesNeeded := 1
				drivesAvailable := 0

				// Check node annotation for available drives
				if drivesAnnotation, ok := k8sNode.Annotations["weka.io/weka-drives"]; ok {
					var drives []string
					if err := json.Unmarshal([]byte(drivesAnnotation), &drives); err == nil {
						drivesAvailable = len(drives)
					}
				}

				drivesFree := drivesAvailable - nodeState.PlacedRoles["drive"]

				if drivesFree < drivesNeeded {
					insufficientDrivesCount++
					continue
				}
			}
		}

		errMsg.WriteString(fmt.Sprintf("  Failure breakdown:\n"))
		if antiAffinityCount > 0 {
			errMsg.WriteString(fmt.Sprintf("    - %d nodes: already have %s containers (anti-affinity violation)\n", antiAffinityCount, role))
		}
		if insufficientCoresCount > 0 {
			errMsg.WriteString(fmt.Sprintf("    - %d nodes: insufficient CPU cores (need %d, available varies)\n", insufficientCoresCount, containerReq.Cores))
		}
		if insufficientMemCount > 0 {
			errMsg.WriteString(fmt.Sprintf("    - %d nodes: insufficient memory (need %d MiB, available varies)\n", insufficientMemCount, containerReq.Memory))
		}
		if insufficientHPCount > 0 {
			errMsg.WriteString(fmt.Sprintf("    - %d nodes: insufficient hugepages (need %d MiB, available varies)\n", insufficientHPCount, containerReq.Hugepages))
		}
		if insufficientDrivesCount > 0 {
			errMsg.WriteString(fmt.Sprintf("    - %d nodes: insufficient drives (need 1, available varies)\n", insufficientDrivesCount))
		}

		totalAccounted := antiAffinityCount + insufficientCoresCount + insufficientMemCount + insufficientHPCount + insufficientDrivesCount
		if totalAccounted < len(nodeStates) {
			errMsg.WriteString(fmt.Sprintf("    - %d nodes: other constraints\n", len(nodeStates)-totalAccounted))
		}

		return nil, fmt.Errorf("%s", errMsg.String())
	}

	return bestNode.Node, nil
}

// NodeResourceVisualization represents the resource state for visualization
type NodeResourceVisualization struct {
	NodeName        string
	TotalCPU        int64 // millicores
	TotalMemory     int64 // bytes
	TotalHugepages  int64 // bytes
	AllocatedByRole map[string]struct {
		Cores     int
		Memory    int64
		Hugepages int64
	}
	UsedByOthers struct {
		Cores     int
		Memory    int64
		Hugepages int64
	}
}

// printNodesPerSelector displays all nodes that match each nodeSelector
func printNodesPerSelector(roleGrouping RoleNodeGrouping, allNodes []corev1.Node, podsByNode map[string][]corev1.Pod) {
	// Build node map for quick lookup
	nodeMap := make(map[string]*corev1.Node)
	for i := range allNodes {
		nodeMap[allNodes[i].Name] = &allNodes[i]
	}

	// Show global nodeSelector nodes
	if len(roleGrouping.Global) > 0 {
		fmt.Printf("\nGlobal NodeSelector: %s\n", SelectorToString(roleGrouping.ByRole["compute"].Selector))
		fmt.Printf("  Matching nodes: %d\n", len(roleGrouping.Global))

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Node", "CPU Alloc.", "CPU Used", "CPU Free", "Memory Alloc.", "Memory Used", "Memory Free", "HP Alloc.", "HP Used", "HP Free"})

		for _, node := range roleGrouping.Global {
			cpuAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceCPU)
			memAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceMemory)
			hpAlloc := quantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

			// Calculate used resources from pods
			cpuUsed, memUsed, hpUsed := CalculateNodeUsage(node.Name, podsByNode)

			cpuFree := cpuAlloc.DeepCopy()
			cpuFree.Sub(*resource.NewMilliQuantity(cpuUsed, resource.DecimalSI))
			memFree := memAlloc.DeepCopy()
			memFree.Sub(*resource.NewQuantity(memUsed, resource.BinarySI))
			hpFree := hpAlloc.DeepCopy()
			hpFree.Sub(*resource.NewQuantity(hpUsed, resource.BinarySI))

			t.AppendRow(table.Row{
				node.Name,
				fmt.Sprintf("%.1f", float64(cpuAlloc.MilliValue())/1000),
				fmt.Sprintf("%.1f", float64(cpuUsed)/1000),
				fmt.Sprintf("%.1f", float64(cpuFree.MilliValue())/1000),
				FormatAsGi(&memAlloc),
				FormatAsGi(resource.NewQuantity(memUsed, resource.BinarySI)),
				FormatAsGi(&memFree),
				FormatAsGi(&hpAlloc),
				FormatAsGi(resource.NewQuantity(hpUsed, resource.BinarySI)),
				FormatAsGi(&hpFree),
			})
		}

		t.SetStyle(table.StyleLight)
		t.Render()
	}

	// Show role-specific selectors
	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		rng, exists := roleGrouping.ByRole[role]
		if !exists || len(rng.Nodes) == 0 {
			continue
		}

		// Skip if it's the same as global
		if len(rng.Nodes) == len(roleGrouping.Global) {
			same := true
			for i := range rng.Nodes {
				if rng.Nodes[i].Name != roleGrouping.Global[i].Name {
					same = false
					break
				}
			}
			if same {
				continue
			}
		}

		fmt.Printf("\n%s NodeSelector: %s\n", strings.Title(role), SelectorToString(rng.Selector))
		fmt.Printf("  Matching nodes: %d\n", len(rng.Nodes))

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Node", "CPU Alloc.", "CPU Used", "CPU Free", "Memory Alloc.", "Memory Used", "Memory Free", "HP Alloc.", "HP Used", "HP Free"})

		for _, node := range rng.Nodes {
			cpuAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceCPU)
			memAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceMemory)
			hpAlloc := quantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

			// Calculate used resources from pods
			cpuUsed, memUsed, hpUsed := CalculateNodeUsage(node.Name, podsByNode)

			cpuFree := cpuAlloc.DeepCopy()
			cpuFree.Sub(*resource.NewMilliQuantity(cpuUsed, resource.DecimalSI))
			memFree := memAlloc.DeepCopy()
			memFree.Sub(*resource.NewQuantity(memUsed, resource.BinarySI))
			hpFree := hpAlloc.DeepCopy()
			hpFree.Sub(*resource.NewQuantity(hpUsed, resource.BinarySI))

			t.AppendRow(table.Row{
				node.Name,
				fmt.Sprintf("%.1f", float64(cpuAlloc.MilliValue())/1000),
				fmt.Sprintf("%.1f", float64(cpuUsed)/1000),
				fmt.Sprintf("%.1f", float64(cpuFree.MilliValue())/1000),
				FormatAsGi(&memAlloc),
				FormatAsGi(resource.NewQuantity(memUsed, resource.BinarySI)),
				FormatAsGi(&memFree),
				FormatAsGi(&hpAlloc),
				FormatAsGi(resource.NewQuantity(hpUsed, resource.BinarySI)),
				FormatAsGi(&hpFree),
			})
		}

		t.SetStyle(table.StyleLight)
		t.Render()
	}
}

// checkOverlappingSelectors detects and warns about overlapping nodeSelectors
func checkOverlappingSelectors(roleGrouping RoleNodeGrouping) {
	// Build map of node -> unique selectors that select it
	// Key: node name, Value: map of selector string -> list of roles using that selector
	nodeToSelectors := make(map[string]map[string][]string)

	// Build a global selector map for quick comparison
	globalNodes := make(map[string]bool)
	for _, node := range roleGrouping.Global {
		globalNodes[node.Name] = true
	}

	for role, rng := range roleGrouping.ByRole {
		// Convert selector to string for comparison
		selectorStr := fmt.Sprintf("%v", rng.Selector)

		for _, node := range rng.Nodes {
			if nodeToSelectors[node.Name] == nil {
				nodeToSelectors[node.Name] = make(map[string][]string)
			}
			nodeToSelectors[node.Name][selectorStr] = append(nodeToSelectors[node.Name][selectorStr], role)
		}
	}

	// Check for nodes matching multiple different selectors
	type SelectorInfo struct {
		SelectorType string   // "global" or role name
		Roles        []string // Roles using this selector
	}

	overlaps := make(map[string][]SelectorInfo)

	for nodeName, selectors := range nodeToSelectors {
		if len(selectors) > 1 {
			// This node matches multiple different selectors
			var selectorInfos []SelectorInfo

			for selectorStr, roles := range selectors {
				// Check if this is the global selector
				isGlobal := false
				if globalNodes[nodeName] {
					// Check if any role using this selector matches global
					for _, role := range roles {
						if _, exists := roleGrouping.ByRole[role]; exists {
							globalSelectorStr := fmt.Sprintf("%v", roleGrouping.ByRole["compute"].Selector)
							if selectorStr == globalSelectorStr {
								isGlobal = true
								break
							}
						}
					}
				}

				if isGlobal {
					selectorInfos = append(selectorInfos, SelectorInfo{
						SelectorType: "global",
						Roles:        roles,
					})
				} else {
					// Role-specific selector - use first role name as identifier
					selectorInfos = append(selectorInfos, SelectorInfo{
						SelectorType: roles[0], // Use first role as the selector identifier
						Roles:        roles,
					})
				}
			}

			if len(selectorInfos) > 1 {
				overlaps[nodeName] = selectorInfos
			}
		}
	}

	if len(overlaps) > 0 {
		fmt.Printf("\n⚠️  WARNING: Overlapping NodeSelectors Detected\n")
		fmt.Printf("The following nodes match multiple nodeSelectors:\n")

		for nodeName, selectorInfos := range overlaps {
			var selectorTypes []string
			for _, info := range selectorInfos {
				selectorTypes = append(selectorTypes, info.SelectorType)
			}
			fmt.Printf("  • %s: [multiple nodeSelectors match: %s]\n", nodeName, strings.Join(selectorTypes, ", "))
		}

		fmt.Printf("\nThis means containers of different roles may be placed on the same node.\n")
		fmt.Printf("Resources allocated to one role will reduce capacity for other roles on shared nodes.\n\n")
	}
}

// printPlacementDetailsWithResourceBars displays containers and resource allocation in a single combined table
func printPlacementDetailsWithResourceBars(allocationGroups map[string]*NodeAllocationGroup, nodeList []corev1.Node, podsByNode map[string][]corev1.Pod) {
	fmt.Println("\n=== Placement Details & Resource Allocation ===")

	// Build map of node -> containers
	nodeToContainers := make(map[string][]ContainerPlacement)
	for _, group := range allocationGroups {
		for _, placement := range group.PlacedContainers {
			nodeToContainers[placement.NodeName] = append(nodeToContainers[placement.NodeName], placement)
		}
	}

	// Create a map of nodes by name
	nodeMap := make(map[string]*corev1.Node)
	for i := range nodeList {
		nodeMap[nodeList[i].Name] = &nodeList[i]
	}

	// Sort node names - build list of nodes from the container map
	var nodesToSort []corev1.Node
	for nodeName := range nodeToContainers {
		if node, exists := nodeMap[nodeName]; exists {
			nodesToSort = append(nodesToSort, *node)
		}
	}
	sortedNodeNames := SortNodeNames(nodesToSort)

	// Create table
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Node", "Containers & Resources", "Resource Allocation"})

	// ANSI color codes for roles
	const (
		colorCompute = "\033[38;2;100;150;255m" // Blue
		colorDrive   = "\033[38;2;255;150;100m" // Orange
		colorS3      = "\033[38;2;150;255;100m" // Light green
		colorEnvoy   = "\033[38;2;255;255;150m" // Light yellow
		colorNFS     = "\033[38;2;255;150;200m" // Pink
		colorReset   = "\033[0m"
	)

	roleColors := map[string]string{
		"compute": colorCompute,
		"drive":   colorDrive,
		"s3":      colorS3,
		"envoy":   colorEnvoy,
		"nfs":     colorNFS,
	}

	barWidth := 50

	for i, nodeName := range sortedNodeNames {
		placements := nodeToContainers[nodeName]
		k8sNode := nodeMap[nodeName]

		// Build container list with colors and resource info
		var containerLines []string
		for _, p := range placements {
			color, exists := roleColors[p.Role]
			if !exists {
				color = colorReset
			}

			line := fmt.Sprintf("%s%s [CORES: %d, RAM: %.1fGi, HP: %.1fGi]%s",
				color, strings.ToUpper(p.Role[:1])+p.Role[1:],
				p.Cores,
				float64(p.Memory)/(1024),
				float64(p.Hugepages)/(1024),
				colorReset,
			)
			containerLines = append(containerLines, line)
		}

		// Get node resources and calculate bars
		var barLines []string
		if k8sNode != nil {
			// Get allocatable resources
			cpuAlloc := quantityOrZero(k8sNode.Status.Allocatable, corev1.ResourceCPU)
			memAlloc := quantityOrZero(k8sNode.Status.Allocatable, corev1.ResourceMemory)
			hpAlloc := quantityOrZero(k8sNode.Status.Allocatable, "hugepages-2Mi")

			// Calculate actual pod resource usage
			podsOnNode := podsByNode[nodeName]
			cpuUsed := resource.MustParse("0")
			memUsed := resource.MustParse("0")
			hpUsed := resource.MustParse("0")

			for _, p := range podsOnNode {
				if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
					continue
				}
				for _, c := range p.Spec.Containers {
					cpuUsed.Add(quantityOrZero(c.Resources.Requests, corev1.ResourceCPU))
					memUsed.Add(quantityOrZero(c.Resources.Requests, corev1.ResourceMemory))
					hpUsed.Add(quantityOrZero(c.Resources.Requests, "hugepages-2Mi"))
				}
			}

			// Build role allocation maps
			roleAllocCPU := make(map[string]int64)
			roleAllocMem := make(map[string]int64)
			roleAllocHP := make(map[string]int64)

			for _, placement := range placements {
				roleAllocCPU[placement.Role] += int64(placement.Cores) * 1000
				roleAllocMem[placement.Role] += placement.Memory * 1024 * 1024
				roleAllocHP[placement.Role] += placement.Hugepages * 1024 * 1024
			}

			// Generate bars (without percentages in legend)
			cpuBar := generateBarWithoutLegend(cpuAlloc.MilliValue(), cpuUsed.MilliValue(), roleAllocCPU, barWidth)
			memBar := generateBarWithoutLegend(memAlloc.Value(), memUsed.Value(), roleAllocMem, barWidth)
			hpBar := generateBarWithoutLegend(hpAlloc.Value(), hpUsed.Value(), roleAllocHP, barWidth)

			barLines = []string{
				"CPU: " + cpuBar,
				"Mem: " + memBar,
				"HP:  " + hpBar,
			}
		}

		// Pad to ensure same number of lines
		maxLines := len(containerLines)
		if len(barLines) > maxLines {
			maxLines = len(barLines)
		}

		for len(containerLines) < maxLines {
			containerLines = append(containerLines, "")
		}
		for len(barLines) < maxLines {
			barLines = append(barLines, "")
		}
		if i > 0 {
			t.AppendSeparator() // Add row separators for better readability
		}
		t.AppendRow(table.Row{
			nodeName,
			strings.Join(containerLines, "\n"),
			strings.Join(barLines, "\n"),
		})
	}

	t.SetStyle(table.StyleLight)
	t.Render()
	fmt.Println()
}

// generateBarWithoutLegend creates a resource bar without the percentage legend
func generateBarWithoutLegend(total int64, used int64, roleAlloc map[string]int64, width int) string {
	const (
		colorReset   = "\033[0m"
		colorUsed    = "\033[38;2;200;100;100m" // Soft red
		colorCompute = "\033[38;2;100;150;255m" // Blue
		colorDrive   = "\033[38;2;255;150;100m" // Orange
		colorS3      = "\033[38;2;150;255;100m" // Light green
		colorEnvoy   = "\033[38;2;255;255;150m" // Light yellow
		colorNFS     = "\033[38;2;255;150;200m" // Pink
		colorFree    = "\033[38;2;150;150;150m" // Gray
	)

	if total == 0 {
		return "[empty]"
	}

	computeAlloc := roleAlloc["compute"]
	driveAlloc := roleAlloc["drive"]
	s3Alloc := roleAlloc["s3"]
	envoyAlloc := roleAlloc["envoy"]
	nfsAlloc := roleAlloc["nfs"]

	// Calculate percentages
	usedPct := int((used * 100) / total)
	computePct := int((computeAlloc * 100) / total)
	drivePct := int((driveAlloc * 100) / total)
	s3Pct := int((s3Alloc * 100) / total)
	envoyPct := int((envoyAlloc * 100) / total)
	nfsPct := int((nfsAlloc * 100) / total)

	// Calculate bar characters
	usedChars := (usedPct * width) / 100
	computeChars := (computePct * width) / 100
	driveChars := (drivePct * width) / 100
	s3Chars := (s3Pct * width) / 100
	envoyChars := (envoyPct * width) / 100
	nfsChars := (nfsPct * width) / 100
	freeChars := width - usedChars - computeChars - driveChars - s3Chars - envoyChars - nfsChars

	// Ensure minimum of 1 char if percentage > 0
	if usedPct > 0 && usedChars == 0 {
		usedChars = 1
	}
	if computePct > 0 && computeChars == 0 {
		computeChars = 1
	}
	if drivePct > 0 && driveChars == 0 {
		driveChars = 1
	}
	if s3Pct > 0 && s3Chars == 0 {
		s3Chars = 1
	}
	if envoyPct > 0 && envoyChars == 0 {
		envoyChars = 1
	}
	if nfsPct > 0 && nfsChars == 0 {
		nfsChars = 1
	}
	if freeChars < 0 {
		freeChars = 0
	}

	// Build bar
	bar := "["

	if usedChars > 0 {
		bar += colorUsed + RepeatChar('█', usedChars) + colorReset
	}
	if computeChars > 0 {
		bar += colorCompute + RepeatChar('█', computeChars) + colorReset
	}
	if driveChars > 0 {
		bar += colorDrive + RepeatChar('█', driveChars) + colorReset
	}
	if s3Chars > 0 {
		bar += colorS3 + RepeatChar('█', s3Chars) + colorReset
	}
	if envoyChars > 0 {
		bar += colorEnvoy + RepeatChar('█', envoyChars) + colorReset
	}
	if nfsChars > 0 {
		bar += colorNFS + RepeatChar('█', nfsChars) + colorReset
	}
	if freeChars > 0 {
		bar += colorFree + RepeatChar('░', freeChars) + colorReset
	}

	bar += "]"
	return bar
}
