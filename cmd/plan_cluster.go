package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var planClusterCmd = &cobra.Command{
	Use:   "cluster <file.yaml>",
	Short: "Plan cluster deployment from WekaCluster YAML file",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanCluster,
}

func init() {
	planCmd.AddCommand(planClusterCmd)
	planClusterCmd.Flags().BoolVar(&flagNoHeaders, "no-headers", false, "Don't print headers")
}

type ContainerRequirements struct {
	Type      string
	Count     int
	Hugepages int64
	Cores     int
	Memory    int64
}

type NodeRequirements struct {
	Purpose          string
	MinNodes         int
	CoresPerNode     int
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

	if err := validateAndPlan(cluster, nodes); err != nil {
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

func validateAndPlan(cluster *wekaapi.WekaCluster, nodes []corev1.Node) error {
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
		containers = append(containers, req)
	}

	printContainerRequirements(containers)
	nodeReqs := calculateNodeRequirements(config, containers)
	printNodeRequirements(nodeReqs)

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
	req.Memory = int64(1024 + additionalMem)
	return req
}

func printContainerRequirements(containers []ContainerRequirements) {
	if flagNoHeaders {
		for _, c := range containers {
			fmt.Printf("%s\t%d\t%d\t%d MiB\t%d MiB\n",
				c.Type, c.Count, c.Cores, c.Hugepages, c.Memory)
		}
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Container Type", "Count", "Cores/Container", "Hugepages/Container", "Memory/Container"})

	for _, c := range containers {
		t.AppendRow(table.Row{
			c.Type,
			c.Count,
			c.Cores,
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
		totalHugepages := int64(0)
		totalMemory := int64(0)

		if computeCount > 0 {
			totalCores += computeReq.Cores
			totalHugepages += computeReq.Hugepages
			totalMemory += computeReq.Memory
		}
		if driveCount > 0 {
			totalCores += driveReq.Cores
			totalHugepages += driveReq.Hugepages
			totalMemory += driveReq.Memory
		}

		totalCores = int(float64(totalCores) * 1.1)
		totalHugepages = int64(float64(totalHugepages) * 1.1)
		totalMemory = int64(float64(totalMemory) * 1.1)

		nodeReqs = append(nodeReqs, NodeRequirements{
			Purpose:          "Backend (Compute+Drive)",
			MinNodes:         backendNodes,
			CoresPerNode:     totalCores,
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
		totalHugepages := int64(0)
		totalMemory := int64(0)

		if s3Count > 0 {
			totalCores += s3Req.Cores + envoyReq.Cores
			totalHugepages += s3Req.Hugepages + envoyReq.Hugepages
			totalMemory += s3Req.Memory + envoyReq.Memory
		}
		if nfsCount > 0 {
			totalCores += nfsReq.Cores
			totalHugepages += nfsReq.Hugepages
			totalMemory += nfsReq.Memory
		}

		totalCores = int(float64(totalCores) * 1.1)
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
			fmt.Printf("%s\t%d\t%d\t%d MiB\t%d MiB\t%s\n",
				nr.Purpose, nr.MinNodes, nr.CoresPerNode, nr.HugepagesPerNode, nr.MemoryPerNode, nr.Description)
		}
		if len(nodeReqs) > 0 {
			fmt.Printf("Recommendation\t-\t-\t-\t-\tAt least 1 more node of required capacity for fault tolerance\n")
		}
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Purpose", "Min Nodes", "Cores/Node", "Hugepages/Node", "Memory/Node", "Description"})

	sort.Slice(nodeReqs, func(i, j int) bool {
		return nodeReqs[i].MinNodes > nodeReqs[j].MinNodes
	})

	for _, nr := range nodeReqs {
		t.AppendRow(table.Row{
			nr.Purpose,
			nr.MinNodes,
			nr.CoresPerNode,
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
