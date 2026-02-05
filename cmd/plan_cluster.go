package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

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

	// SANITY CHECK: Validate cluster definition before doing anything else
	if err := sanityCheckClusterDefinition(cluster); err != nil {
		fmt.Printf("\n❌ Cluster Definition Validation Failed:\n%v\n", err)
		return err
	}

	nodes, clientset, err := getClusterNodesForPlan(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Could not access cluster nodes: %v\n", err)
		_, _ = fmt.Fprintf(os.Stderr, "Continuing with planning without cluster validation...\n\n")
		nodes = nil
		clientset = nil
	}

	if err := validateAndPlan(ctx, clientset, cluster, nodes); err != nil {
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

func getClusterNodesForPlan(ctx context.Context) ([]corev1.Node, *kubernetes.Clientset, error) {
	cfg, err := getKubeConfigForPlan()
	if err != nil {
		return nil, nil, err
	}

	// Get the standard client
	k8sClient, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, nil, err
	}

	// Also get the kubernetes clientset for host check pods
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	nodeList := &corev1.NodeList{}
	if err := k8sClient.List(ctx, nodeList); err != nil {
		return nil, nil, err
	}

	return nodeList.Items, clientset, nil
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

	fmt.Println("✅ Cluster definition validation passed\n")
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
	cfg, err := getKubeConfigForPlan()
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

func validateAndPlan(ctx context.Context, clientset *kubernetes.Clientset, cluster *wekaapi.WekaCluster, nodes []corev1.Node) error {
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
		if err := validateClusterResources(ctx, clientset, cluster, nodes, nodeReqs); err != nil {
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
		frontendNodes := maxInt(s3Count, nfsCount)

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

// printNodeResources displays available resources on each node
// Collects data from host checks but shows Kubernetes resource allocation
// Also shows potential Weka resource allocation based on nodeReqs
func printNodeResources(ctx context.Context, clientset *kubernetes.Clientset, nodes []corev1.Node, nodeReqs []NodeRequirements) (map[string]HostChecksResult, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	fmt.Println("\n=== Node Resources ===")

	// Run host checks to get resource info
	resultChan, cleanupWg := scanHostChecksByPod(ctx, clientset, nodes)
	defer cleanupWg.Wait()

	hostChecksMap := make(map[string]HostChecksResult)
	nodeResourcesTable := table.NewWriter()
	nodeResourcesTable.SetOutputMirror(os.Stdout)
	nodeResourcesTable.AppendHeader(table.Row{
		"Node",
		"HT Enabled",
		"Allocatable CPU",
		"Allocatable Memory",
		"Allocatable Hugepages",
		"Free CPU",
		"Free Memory",
		"Free Hugepages",
		"CPU Model",
	})

	// Create a map of nodes by name for quick lookup
	nodeMap := make(map[string]*corev1.Node)
	for i := range nodes {
		nodeMap[nodes[i].Name] = &nodes[i]
	}

	// Get all pods in cluster to calculate free resources
	podList, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	// Create a map of pods per node
	podsByNode := make(map[string][]corev1.Pod)
	for _, pod := range podList.Items {
		podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
	}

	// Collect results
	for result := range resultChan {
		hostChecksMap[result.nodeName] = result.result

		if result.err != nil {
			nodeResourcesTable.AppendRow(table.Row{
				result.nodeName,
				"ERROR",
				"-",
				"-",
				"-",
				"-",
				"-",
				"-",
				fmt.Sprintf("Error: %v", result.err),
			})
			continue
		}

		hc := result.result

		htStatus := "OFF"
		if hc.HTEnabled {
			htStatus = "ON"
		}

		// Get all resources from Kubernetes node status
		if node, ok := nodeMap[result.nodeName]; ok {
			// Allocatable resources
			cpuAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceCPU)
			memAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceMemory)
			hpAlloc := quantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

			// Calculate free resources (allocatable - pod requests)
			podsOnNode := podsByNode[result.nodeName]
			cpuReq := resource.MustParse("0")
			memReq := resource.MustParse("0")
			hpReq := resource.MustParse("0")

			for _, p := range podsOnNode {
				// Skip non-running pods
				if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
					continue
				}

				// Sum container requests
				for _, c := range p.Spec.Containers {
					cpuReq.Add(quantityOrZero(c.Resources.Requests, corev1.ResourceCPU))
					memReq.Add(quantityOrZero(c.Resources.Requests, corev1.ResourceMemory))
					hpReq.Add(quantityOrZero(c.Resources.Requests, "hugepages-2Mi"))
				}

				// Take max of init containers
				for _, c := range p.Spec.InitContainers {
					cpu := quantityOrZero(c.Resources.Requests, corev1.ResourceCPU)
					mem := quantityOrZero(c.Resources.Requests, corev1.ResourceMemory)
					hp := quantityOrZero(c.Resources.Requests, "hugepages-2Mi")
					if cpu.Cmp(cpuReq) > 0 {
						cpuReq = cpu
					}
					if mem.Cmp(memReq) > 0 {
						memReq = mem
					}
					if hp.Cmp(hpReq) > 0 {
						hpReq = hp
					}
				}
			}

			// Calculate free
			cpuFree := cpuAlloc.DeepCopy()
			cpuFree.Sub(cpuReq)
			if cpuFree.Sign() < 0 {
				cpuFree = resource.MustParse("0")
			}

			memFree := memAlloc.DeepCopy()
			memFree.Sub(memReq)
			if memFree.Sign() < 0 {
				memFree = resource.MustParse("0")
			}

			hpFree := hpAlloc.DeepCopy()
			hpFree.Sub(hpReq)
			if hpFree.Sign() < 0 {
				hpFree = resource.MustParse("0")
			}

			nodeResourcesTable.AppendRow(table.Row{
				result.nodeName,
				htStatus,
				cpuAlloc.String(),
				memAlloc.String(),
				hpAlloc.String(),
				cpuFree.String(),
				memFree.String(),
				hpFree.String(),
				hc.CPUModel,
			})
		} else {
			nodeResourcesTable.AppendRow(table.Row{
				result.nodeName,
				htStatus,
				"-",
				"-",
				"-",
				"-",
				"-",
				"-",
				hc.CPUModel,
			})
		}
	}

	nodeResourcesTable.SetStyle(table.StyleLight)
	nodeResourcesTable.Render()

	// Print resource allocation visualization for each node
	fmt.Println("\n=== Resource Allocation Visualization ===")
	fmt.Println("Format: [Light Red=USED | Light Purple=COMPUTE+DRIVE | Light Cyan=S3+ENVOY | Light Yellow=NFS | Light Green=FREE]\n")

	// Build a map of Weka resource requirements per resource type
	backendReq := findNodeRequirementByPurpose(nodeReqs, "Backend (Compute+Drive)")
	frontendReq := findNodeRequirementByPurpose(nodeReqs, "Frontend (S3/NFS)")

	for nodeName, node := range nodeMap {
		if _, exists := hostChecksMap[nodeName]; !exists {
			continue // Skip nodes without host checks
		}

		hc := hostChecksMap[nodeName]

		// Determine which CPU cores calculation to use based on HT status
		backendCoresPerNode := backendReq.CoresPerNode
		frontendCoresPerNode := frontendReq.CoresPerNode
		if !hc.HTEnabled {
			backendCoresPerNode = backendReq.CoresPerNodeNoHT
			frontendCoresPerNode = frontendReq.CoresPerNodeNoHT
		}

		// Get allocatable resources
		cpuAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceCPU)
		memAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceMemory)
		hpAlloc := quantityOrZero(node.Status.Allocatable, "hugepages-2Mi")

		// Calculate requests
		podsOnNode := podsByNode[nodeName]
		cpuReq := resource.MustParse("0")
		memReq := resource.MustParse("0")
		hpReq := resource.MustParse("0")

		for _, p := range podsOnNode {
			if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
				continue
			}
			for _, c := range p.Spec.Containers {
				cpuReq.Add(quantityOrZero(c.Resources.Requests, corev1.ResourceCPU))
				memReq.Add(quantityOrZero(c.Resources.Requests, corev1.ResourceMemory))
				hpReq.Add(quantityOrZero(c.Resources.Requests, "hugepages-2Mi"))
			}
			for _, c := range p.Spec.InitContainers {
				cpu := quantityOrZero(c.Resources.Requests, corev1.ResourceCPU)
				mem := quantityOrZero(c.Resources.Requests, corev1.ResourceMemory)
				hp := quantityOrZero(c.Resources.Requests, "hugepages-2Mi")
				if cpu.Cmp(cpuReq) > 0 {
					cpuReq = cpu
				}
				if mem.Cmp(memReq) > 0 {
					memReq = mem
				}
				if hp.Cmp(hpReq) > 0 {
					hpReq = hp
				}
			}
		}

		// Print node name
		fmt.Printf("%s (HT: %s)\n", nodeName, map[bool]string{true: "ON", false: "OFF"}[hc.HTEnabled])

		// CPU visualization
		// CPU cores in Kubernetes are measured in millicores (1 core = 1000m)
		cpuUsedPct := getPercentage(cpuReq, cpuAlloc)
		cpuWekaBEPct := getPercentage(*resource.NewMilliQuantity(int64(backendCoresPerNode*1000), resource.DecimalSI), cpuAlloc)
		cpuWekaFEPct := getPercentage(*resource.NewMilliQuantity(int64(frontendCoresPerNode*1000), resource.DecimalSI), cpuAlloc)
		cpuFreePct := 100 - cpuUsedPct - cpuWekaBEPct - cpuWekaFEPct
		if cpuFreePct < 0 {
			cpuFreePct = 0
		}
		fmt.Printf("  CPU:       %s\n", drawBarWithWeka(cpuUsedPct, cpuWekaBEPct, cpuWekaFEPct, 0, cpuFreePct))
		cpuFree := subtractQuantity(cpuAlloc, cpuReq)
		fmt.Printf("             (%s allocatable, %s used, %s free for Weka)\n",
			(&cpuAlloc).String(), (&cpuReq).String(),
			(&cpuFree).String())

		// Memory visualization
		// Memory is stored in MiB, need to convert to bytes (1 MiB = 1024*1024 bytes)
		memUsedPct := getPercentage(memReq, memAlloc)
		memWekaBEBytes := backendReq.MemoryPerNode * 1024 * 1024
		memWekaFEBytes := frontendReq.MemoryPerNode * 1024 * 1024
		memWekaBEPct := getPercentage(*resource.NewQuantity(memWekaBEBytes, resource.BinarySI), memAlloc)
		memWekaFEPct := getPercentage(*resource.NewQuantity(memWekaFEBytes, resource.BinarySI), memAlloc)
		memFreePct := 100 - memUsedPct - memWekaBEPct - memWekaFEPct
		if memFreePct < 0 {
			memFreePct = 0
		}
		fmt.Printf("  Memory:    %s\n", drawBarWithWeka(memUsedPct, memWekaBEPct, memWekaFEPct, 0, memFreePct))
		memFree := subtractQuantity(memAlloc, memReq)
		fmt.Printf("             (%s allocatable, %s used, %s free for Weka)\n",
			(&memAlloc).String(), (&memReq).String(),
			(&memFree).String())

		// Hugepages visualization
		// Hugepages are stored in MiB, need to convert to bytes (1 MiB = 1024*1024 bytes)
		hpUsedPct := getPercentage(hpReq, hpAlloc)
		hpWekaBEBytes := backendReq.HugepagesPerNode * 1024 * 1024
		hpWekaFEBytes := frontendReq.HugepagesPerNode * 1024 * 1024
		hpWekaBEPct := getPercentage(*resource.NewQuantity(hpWekaBEBytes, resource.BinarySI), hpAlloc)
		hpWekaFEPct := getPercentage(*resource.NewQuantity(hpWekaFEBytes, resource.BinarySI), hpAlloc)
		hpFreePct := 100 - hpUsedPct - hpWekaBEPct - hpWekaFEPct
		if hpFreePct < 0 {
			hpFreePct = 0
		}
		fmt.Printf("  Hugepages: %s\n", drawBarWithWeka(hpUsedPct, hpWekaBEPct, hpWekaFEPct, 0, hpFreePct))
		hpFree := subtractQuantity(hpAlloc, hpReq)
		fmt.Printf("             (%s allocatable, %s used, %s free for Weka)\n",
			(&hpAlloc).String(), (&hpReq).String(),
			(&hpFree).String())

		fmt.Println()
	}

	return hostChecksMap, nil
}

// getPercentage calculates what percentage 'used' is of 'total'
func getPercentage(used, total resource.Quantity) int {
	if total.IsZero() {
		return 0
	}
	pct := int((used.Value() * 100) / total.Value())
	if pct > 100 {
		pct = 100
	}
	return pct
}

// subtractQuantity returns total - used
func subtractQuantity(total, used resource.Quantity) resource.Quantity {
	result := total.DeepCopy()
	result.Sub(used)
	if result.Sign() < 0 {
		return resource.MustParse("0")
	}
	return result
}

// drawBarWithWeka creates a visual bar showing [USED% | WEKA_BE% | WEKA_FE% | WEKA_NFS% | FREE%]
// Uses █ for used, ▓ for backend, ▒ for S3+envoy, ░ for free
func drawBarWithWeka(usedPct, bekaPct, fePct, nfsPct, freePct int) string {
	// ANSI color codes - using mild/pastel versions for better eye comfort
	const (
		colorReset        = "\033[0m"
		colorUsed         = "\033[38;5;217m" // Light red (mild)
		colorWekaCompute  = "\033[38;5;183m" // Light purple (mild) - for Compute+Drive
		colorWekaFrontend = "\033[38;5;152m" // Light cyan (mild) - for S3+Envoy
		colorWekaNFS      = "\033[38;5;230m" // Light yellow (mild) - for NFS
		colorFree         = "\033[38;5;157m" // Light green (mild)
	)

	totalChars := 40
	usedChars := (usedPct * totalChars) / 100
	beChars := (bekaPct * totalChars) / 100
	feChars := (fePct * totalChars) / 100
	nfsChars := (nfsPct * totalChars) / 100
	freeChars := totalChars - usedChars - beChars - feChars - nfsChars

	// Ensure at least 1 char if percentage > 0
	if usedPct > 0 && usedChars == 0 {
		usedChars = 1
	}
	if bekaPct > 0 && beChars == 0 {
		beChars = 1
	}
	if fePct > 0 && feChars == 0 {
		feChars = 1
	}
	if nfsPct > 0 && nfsChars == 0 {
		nfsChars = 1
	}
	if freePct > 0 && freeChars == 0 {
		freeChars = 1
	}

	bar := "["

	// Used (light red █)
	if usedChars > 0 {
		bar += colorUsed
		for i := 0; i < usedChars; i++ {
			bar += "█"
		}
		bar += colorReset
	}

	// Backend/Compute/Drive (light purple █)
	if beChars > 0 {
		bar += colorWekaCompute
		for i := 0; i < beChars; i++ {
			bar += "█"
		}
		bar += colorReset
	}

	// Frontend S3+Envoy (light cyan █)
	if feChars > 0 {
		bar += colorWekaFrontend
		for i := 0; i < feChars; i++ {
			bar += "█"
		}
		bar += colorReset
	}

	// NFS (light yellow █)
	if nfsChars > 0 {
		bar += colorWekaNFS
		for i := 0; i < nfsChars; i++ {
			bar += "█"
		}
		bar += colorReset
	}

	// Remaining free (light green ░)
	if freeChars > 0 {
		bar += colorFree
		for i := 0; i < freeChars; i++ {
			bar += "░"
		}
		bar += colorReset
	}

	bar += "]"

	// Color the percentages too
	return fmt.Sprintf("%s %s%d%%%s|%s%d%%%s|%s%d%%%s|%s%d%%%s|%s%d%%%s",
		bar,
		colorUsed, usedPct, colorReset,
		colorWekaCompute, bekaPct, colorReset,
		colorWekaFrontend, fePct, colorReset,
		colorWekaNFS, nfsPct, colorReset,
		colorFree, freePct, colorReset)
}

// findNodeRequirementByPurpose finds a NodeRequirement by its purpose string
func findNodeRequirementByPurpose(reqs []NodeRequirements, purpose string) NodeRequirements {
	for _, req := range reqs {
		if req.Purpose == purpose {
			return req
		}
	}
	return NodeRequirements{} // Return empty if not found
}

func validateClusterResources(ctx context.Context, clientset *kubernetes.Clientset, cluster *wekaapi.WekaCluster, nodes []corev1.Node, nodeReqs []NodeRequirements) error {
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in cluster")
	}

	config := cluster.Spec.Dynamic
	network := &cluster.Spec.Network
	nodeSelector := cluster.Spec.NodeSelector

	// Check UDP mode warning
	if network.UdpMode {
		fmt.Printf("\n⚠️  WARNING: UDP mode is enabled for cluster. This is not recommended for fast-performance production environments\n\n")
	}

	// Filter nodes that match nodeSelector
	eligibleNodes := filterNodesBySelector(nodes, nodeSelector)
	if len(eligibleNodes) == 0 {
		return fmt.Errorf("no nodes match the nodeSelector criteria: %v", nodeSelector)
	}

	// Display node resources and collect host checks data
	hostChecksMap, err := printNodeResources(ctx, clientset, eligibleNodes, nodeReqs)
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not collect full node resource data: %v\n", err)
	}

	// Save host checks summary to JSON file
	if len(hostChecksMap) > 0 {
		summaryPath := "hostchecks-summary.json"
		if err := saveHostChecksSummary(hostChecksMap, summaryPath); err != nil {
			fmt.Printf("⚠️  Warning: Could not save host checks summary: %v\n", err)
		}
	}

	// Validate drives
	if config.DriveContainers != nil && *config.DriveContainers > 0 && config.NumDrives > 0 {
		if err := validateDrivesOnNodes(eligibleNodes, *config.DriveContainers, config.NumDrives); err != nil {
			return err
		}
	}

	// Validate network configuration
	if err := validateNetworkConfiguration(ctx, clientset, eligibleNodes, network); err != nil {
		return err
	}

	// Validate resource availability
	if err := validateResourceAvailability(eligibleNodes, nodeReqs); err != nil {
		return err
	}

	fmt.Printf("\n✅ Cluster validation passed\n")
	fmt.Printf("   ✓ %d nodes match nodeSelector\n", len(eligibleNodes))
	fmt.Printf("   ✓ All required drives are available\n")
	fmt.Printf("   ✓ Network configuration is consistent\n")
	fmt.Printf("   ✓ Sufficient resources available\n")

	return nil
}

func filterNodesBySelector(nodes []corev1.Node, selector map[string]string) []corev1.Node {
	if selector == nil || len(selector) == 0 {
		return nodes
	}

	var eligible []corev1.Node
	for _, node := range nodes {
		if matchesSelector(node, selector) {
			eligible = append(eligible, node)
		}
	}
	return eligible
}

func matchesSelector(node corev1.Node, selector map[string]string) bool {
	for key, value := range selector {
		if labelValue, ok := node.Labels[key]; !ok || labelValue != value {
			return false
		}
	}
	return true
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

func validateNetworkConfiguration(ctx context.Context, clientset *kubernetes.Clientset, nodes []corev1.Node, network *wekaapi.Network) error {
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

	// Use the shared network validation function that uses host check pods
	return validateNetworkInterfaceOnNodes(ctx, clientset, nodes, ethDevice, planClusterFailFast)
}

func validateResourceAvailability(nodes []corev1.Node, nodeReqs []NodeRequirements) error {
	if len(nodeReqs) == 0 {
		return nil
	}

	var issues []string

	for _, req := range nodeReqs {
		if req.MinNodes > len(nodes) {
			issues = append(issues, fmt.Sprintf(
				"- %s: requires %d nodes but only %d available (need %d cores/node, %d MiB hugepages/node, %d MiB memory/node)",
				req.Purpose, req.MinNodes, len(nodes),
				req.CoresPerNode, req.HugepagesPerNode, req.MemoryPerNode,
			))
		}
	}

	if len(issues) > 0 {
		errMsg := "insufficient cluster nodes for requirements:\n"
		for _, issue := range issues {
			errMsg += issue + "\n"
		}
		return fmt.Errorf(errMsg)
	}

	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// saveHostChecksSummary writes the collected host checks results to a JSON file
// The file contains a dictionary where keys are node names and values are HostChecksResult
func saveHostChecksSummary(results map[string]HostChecksResult, outputPath string) error {
	// Convert map to JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal host checks results: %v", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write host checks summary: %v", err)
	}

	fmt.Printf("✓ Host checks summary saved to: %s\n", outputPath)
	return nil
}
