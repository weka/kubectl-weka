package cmd

import (
	"context"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"time"
)

var (
	getNodesSelector string
)

var getNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Get node information in Weka format",
	RunE:  runGetNodes,
}

func init() {
	getCmd.AddCommand(getNodesCmd)
	getNodesCmd.Flags().BoolVar(&flagNoHeaders, "no-headers", false, "Don't print headers")
	getNodesCmd.Flags().BoolVar(&flagWide, "wide", false, "Wide output (adds allocatable and allocated resources)")
	getNodesCmd.Flags().StringVar(&getNodesSelector, "node-selector", "", "Label selector to filter nodes (e.g., role=storage)")
	getNodesCmd.SilenceUsage = true
}

// runGetNodes executes the get nodes command
func runGetNodes(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Generate the output
	output, err := generateNodesOutput(ctx, KubeClients, flagNoHeaders, flagWide, getNodesSelector)
	if err != nil {
		return err
	}

	// Print the output
	fmt.Println(output)
	return nil
}

// generateNodesOutput generates the nodes information table as a string
func generateNodesOutput(ctx context.Context, clients *K8sClients, noHeaders, wide bool, nodeSelector string) (string, error) {
	crClient := clients.CRClient

	// List nodes using the cached client, optionally filtered by label selector
	var nodeList corev1.NodeList
	opts := []crclient.ListOption{}
	if nodeSelector != "" {
		opts = append(opts, crclient.MatchingLabels(parseSelector(nodeSelector)))
	}
	if err := crClient.List(ctx, &nodeList, opts...); err != nil {
		return "", err
	}

	// List all pods to calculate actual hugepage allocations per node
	var podList corev1.PodList
	if err := crClient.List(ctx, &podList); err != nil {
		return "", err
	}

	// Build a map of node -> hugepage allocations
	hugepageAllocations := calculateHugepageAllocations(&podList)

	// Build maps of node -> cores and RAM allocations
	coreAllocations := calculateResourceAllocations(&podList, corev1.ResourceCPU)
	ramAllocations := calculateResourceAllocations(&podList, corev1.ResourceMemory)

	// Sort nodes by name (numeric aware: node1, node2, node11, etc.)
	sort.Slice(nodeList.Items, func(i, j int) bool {
		return compareNodeNames(nodeList.Items[i].Name, nodeList.Items[j].Name) < 0
	})

	w := table.NewWriter()
	w.SetStyle(table.StyleDefault)
	w.Style().Options.DrawBorder = false
	w.Style().Options.SeparateRows = false
	w.Style().Options.SeparateColumns = false
	w.Style().Options.SeparateFooter = false
	w.Style().Options.SeparateHeader = false

	if !noHeaders {
		if wide {
			w.AppendHeader(table.Row{
				"NAME", "IP", "OS", "ARCH", "KERNEL", "STATUS",
				"HP_ALLOCATABLE", "HP_ALLOCATED", "HP_FREE",
				"CORES_ALLOCATABLE", "CORES_ALLOCATED", "CORES_FREE",
				"RAM_ALLOCATABLE", "RAM_ALLOCATED", "RAM_FREE",
				"CLTROLE", "BKNDROLE",
			})
		} else {
			w.AppendHeader(table.Row{
				"NAME", "IP", "OS", "ARCH", "KERNEL", "STATUS",
				"HP_FREE", "CORES_FREE", "RAM_FREE",
				"CLTROLE", "BKNDROLE",
			})
		}
	}

	for i := range nodeList.Items {
		printNodeRow(w, &nodeList.Items[i], hugepageAllocations, coreAllocations, ramAllocations, wide)
	}

	output := w.Render()
	return output, nil
}

func printNodeRow(w table.Writer, n *corev1.Node, hugepageAllocations map[string]corev1.ResourceList, coreAllocations map[string]corev1.ResourceList, ramAllocations map[string]corev1.ResourceList, wide bool) {
	name := n.Name
	ip := firstInternalIP(n)
	osImage := n.Status.NodeInfo.OSImage
	arch := n.Status.NodeInfo.Architecture
	kernel := n.Status.NodeInfo.KernelVersion

	// Calculate node status (Ready/NotReady with uptime)
	status := "NotReady"
	for _, condition := range n.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				// Calculate uptime
				uptime := "unknown"
				if n.Status.NodeInfo.BootID != "" {
					uptime = humanAge(time.Since(condition.LastTransitionTime.Time))
				}
				status = fmt.Sprintf("Ready (%s)", uptime)
			}
			break
		}
	}

	// Hugepages
	hpCapacity := n.Status.Allocatable["hugepages-2Mi"]
	hpAllocated := hugepageAllocations[name]["hugepages-2Mi"]
	hpFree := hpCapacity.DeepCopy()
	hpFree.Sub(hpAllocated)

	// Cores
	coresCapacity := n.Status.Allocatable[corev1.ResourceCPU]
	coresAllocated := coreAllocations[name][corev1.ResourceCPU]
	coresFree := coresCapacity.DeepCopy()
	coresFree.Sub(coresAllocated)

	// RAM
	ramCapacity := n.Status.Allocatable[corev1.ResourceMemory]
	ramAllocated := ramAllocations[name][corev1.ResourceMemory]
	ramFree := ramCapacity.DeepCopy()
	ramFree.Sub(ramAllocated)

	cltRole := n.Labels["weka.io/supports-clients"]
	bkndRole := n.Labels["weka.io/supports-backends"]

	if cltRole == "" {
		cltRole = "<none>"
	}
	if bkndRole == "" {
		bkndRole = "<none>"
	}

	// Format values
	nameStr := name
	hpCapacityStr := formatQuantityToGB(hpCapacity)
	hpAllocatedStr := formatQuantityToGB(hpAllocated)
	hpFreeStr := formatQuantityToGB(hpFree)
	coreCapacityStr := formatQuantityToGB(coresCapacity)
	coreAllocatedStr := formatQuantityToGB(coresAllocated)
	coreFreeStr := formatQuantityToGB(coresFree)
	ramCapacityStr := formatQuantityToGB(ramCapacity)
	ramAllocatedStr := formatQuantityToGB(ramAllocated)
	ramFreeStr := formatQuantityToGB(ramFree)

	if wide {
		w.AppendRow(table.Row{
			nameStr, ip, osImage, arch, kernel, status,
			hpCapacityStr, hpAllocatedStr, hpFreeStr,
			coreCapacityStr, coreAllocatedStr, coreFreeStr,
			ramCapacityStr, ramAllocatedStr, ramFreeStr,
			cltRole, bkndRole,
		})
	} else {
		w.AppendRow(table.Row{
			nameStr, ip, osImage, arch, kernel, status,
			hpFreeStr, coreFreeStr, ramFreeStr,
			cltRole, bkndRole,
		})
	}
}

// formatQuantityToGB converts a resource quantity to human-readable format in the largest appropriate unit
// e.g., 2000Mi -> "2GB", 2500Mi -> "2.4GB", 512Mi -> "512MB", 512Ki -> "512KB"
func formatQuantityToGB(qty resource.Quantity) string {

	// Get the value in bytes (canonical form)
	bytes := qty.Value()
	if bytes < 0 {
		bytes = -bytes
	}

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	// Format with appropriate precision, using the largest unit that keeps value >= 1
	switch {
	case bytes >= TB:
		value := float64(bytes) / float64(TB)
		if value >= 10 {
			return fmt.Sprintf("%.0fTB", value)
		}
		return fmt.Sprintf("%.1fTB", value)
	case bytes >= GB:
		value := float64(bytes) / float64(GB)
		if value >= 10 {
			return fmt.Sprintf("%.0fGB", value)
		}
		return fmt.Sprintf("%.1fGB", value)
	case bytes >= MB:
		value := float64(bytes) / float64(MB)
		if value >= 10 {
			return fmt.Sprintf("%.0fMB", value)
		}
		return fmt.Sprintf("%.1fMB", value)
	case bytes >= KB:
		value := float64(bytes) / float64(KB)
		if value >= 10 {
			return fmt.Sprintf("%.0fKB", value)
		}
		return fmt.Sprintf("%.1fKB", value)
	default:
		return fmt.Sprintf("%d", bytes)
	}
}

// calculateHugepageAllocations sums up hugepage requests from all Pods per node
func calculateHugepageAllocations(podList *corev1.PodList) map[string]corev1.ResourceList {
	allocations := make(map[string]corev1.ResourceList)

	for i := range podList.Items {
		pod := &podList.Items[i]
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue // Pod not yet scheduled
		}

		if allocations[nodeName] == nil {
			allocations[nodeName] = make(corev1.ResourceList)
		}

		// Sum hugepage requests from all containers
		for _, container := range pod.Spec.Containers {
			for resName, resQty := range container.Resources.Requests {
				if resName == "hugepages-2Mi" {
					currentQty := allocations[nodeName][resName]
					currentQty.Add(resQty)
					allocations[nodeName][resName] = currentQty
				}
			}
		}

		// Also check init containers
		for _, container := range pod.Spec.InitContainers {
			for resName, resQty := range container.Resources.Requests {
				if resName == "hugepages-2Mi" {
					currentQty := allocations[nodeName][resName]
					currentQty.Add(resQty)
					allocations[nodeName][resName] = currentQty
				}
			}
		}
	}

	return allocations
}

// calculateResourceAllocations sums up resource requests (CPU or Memory) from all Pods per node
func calculateResourceAllocations(podList *corev1.PodList, resourceName corev1.ResourceName) map[string]corev1.ResourceList {
	allocations := make(map[string]corev1.ResourceList)

	for i := range podList.Items {
		pod := &podList.Items[i]
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue // Pod not yet scheduled
		}

		if allocations[nodeName] == nil {
			allocations[nodeName] = make(corev1.ResourceList)
		}

		// Sum resource requests from all containers
		for _, container := range pod.Spec.Containers {
			if resQty, exists := container.Resources.Requests[resourceName]; exists {
				currentQty := allocations[nodeName][resourceName]
				currentQty.Add(resQty)
				allocations[nodeName][resourceName] = currentQty
			}
		}

		// Also check init containers
		for _, container := range pod.Spec.InitContainers {
			if resQty, exists := container.Resources.Requests[resourceName]; exists {
				currentQty := allocations[nodeName][resourceName]
				currentQty.Add(resQty)
				allocations[nodeName][resourceName] = currentQty
			}
		}
	}

	return allocations
}
