package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
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
	getNodesCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")
	getNodesCmd.Flags().StringVar(&getNodesSelector, "node-selector", "", "Label selector to filter nodes (e.g., role=storage)")
	getNodesCmd.SilenceUsage = true
}

// runGetNodes executes the get nodes command
func runGetNodes(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	printer, _ := GetPrinterFromFlags(flagOutput, !flagNoHeaders, nil, false, 0, TableStyleMinimal)
	output, err := generateNodesOutput(ctx, KubeClients, printer, getNodesSelector)
	if err != nil {
		return err
	}

	fmt.Println(output)
	return nil
}

// generateNodesOutput generates the nodes information table as a string
func generateNodesOutput(ctx context.Context, clients *K8sClients, printer ResourcePrinter, nodeSelector string) (string, error) {
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

	// Define columns slice (single, with VisibleInWide)
	cols := []TableColumn{
		{Name: "NAME", VisibleInWide: false},
		{Name: "IP", VisibleInWide: false},
		{Name: "OS", VisibleInWide: false},
		{Name: "ARCH", VisibleInWide: false},
		{Name: "KERNEL", VisibleInWide: false},
		{Name: "STATUS", VisibleInWide: false},
		{Name: "HP2MI_ALLOCATABLE", VisibleInWide: true, formatFuncs: TableFormatFunctions{formatQuantityToGB}},
		{Name: "HP2MI_ALLOCATED", VisibleInWide: true, formatFuncs: TableFormatFunctions{formatQuantityToGB}},
		{Name: "HP2MI_FREE", VisibleInWide: false, formatFuncs: TableFormatFunctions{formatQuantityToGB}},
		{Name: "CORES_ALLOCATABLE", VisibleInWide: true, formatFuncs: TableFormatFunctions{formatQuantityToGB}},
		{Name: "CORES_ALLOCATED", VisibleInWide: true, formatFuncs: TableFormatFunctions{formatQuantityToGB}},
		{Name: "CORES_FREE", VisibleInWide: false, formatFuncs: TableFormatFunctions{formatQuantityToGB}},
		{Name: "RAM_ALLOCATABLE", VisibleInWide: true, formatFuncs: TableFormatFunctions{formatQuantityToGB}},
		{Name: "RAM_ALLOCATED", VisibleInWide: true, formatFuncs: TableFormatFunctions{formatQuantityToGB}},
		{Name: "RAM_FREE", VisibleInWide: false, formatFuncs: TableFormatFunctions{formatQuantityToGB}},
	}

	var rows []TableRow
	for i := range nodeList.Items {
		n := &nodeList.Items[i]
		row := TableRow{Values: map[string]interface{}{}}
		row.Values["NAME"] = n.Name
		row.Values["IP"] = firstInternalIP(n)
		row.Values["OS"] = n.Status.NodeInfo.OSImage
		row.Values["ARCH"] = n.Status.NodeInfo.Architecture
		row.Values["KERNEL"] = n.Status.NodeInfo.KernelVersion
		row.Values["STATUS"] = func() string {
			status := "NotReady"
			for _, condition := range n.Status.Conditions {
				if condition.Type == corev1.NodeReady {
					if condition.Status == corev1.ConditionTrue {
						uptime := "unknown"
						if n.Status.NodeInfo.BootID != "" {
							uptime = humanAge(condition.LastTransitionTime.Time)
						}
						status = fmt.Sprintf("Ready (%s)", uptime)
					}
					break
				}
			}
			return status
		}()
		row.Values["HP2MI_ALLOCATABLE"] = n.Status.Allocatable["hugepages-2Mi"]
		row.Values["HP2MI_ALLOCATED"] = hugepageAllocations[n.Name]["hugepages-2Mi"]
		row.Values["HP2MI_FREE"] = func() resource.Quantity {
			free := n.Status.Allocatable["hugepages-2Mi"].DeepCopy()
			free.Sub(hugepageAllocations[n.Name]["hugepages-2Mi"])
			return free
		}()
		row.Values["CORES_ALLOCATABLE"] = n.Status.Allocatable[corev1.ResourceCPU]
		row.Values["CORES_ALLOCATED"] = coreAllocations[n.Name][corev1.ResourceCPU]
		row.Values["CORES_FREE"] = func() resource.Quantity {
			free := n.Status.Allocatable[corev1.ResourceCPU].DeepCopy()
			free.Sub(coreAllocations[n.Name][corev1.ResourceCPU])
			return free
		}()
		row.Values["RAM_ALLOCATABLE"] = n.Status.Allocatable[corev1.ResourceMemory]
		row.Values["RAM_ALLOCATED"] = ramAllocations[n.Name][corev1.ResourceMemory]
		row.Values["RAM_FREE"] = func() resource.Quantity {
			free := n.Status.Allocatable[corev1.ResourceMemory].DeepCopy()
			free.Sub(ramAllocations[n.Name][corev1.ResourceMemory])
			return free
		}()
		cltRole := n.Labels["weka.io/supports-clients"]
		bkndRole := n.Labels["weka.io/supports-backends"]
		if cltRole == "" {
			cltRole = "<none>"
		}
		if bkndRole == "" {
			bkndRole = "<none>"
		}
		rows = append(rows, row)
	}

	// Render output
	var buf strings.Builder
	if err := printer.Print(cols, rows, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// formatQuantityToGB converts a resource quantity to human-readable format in the largest appropriate unit
// e.g., 2000Mi -> "2GB", 2500Mi -> "2.4GB", 512Mi -> "512MB", 512Ki -> "512KB"
func formatQuantityToGB(val interface{}) string {
	qty, ok := val.(resource.Quantity)
	if !ok {
		// Try pointer
		if ptr, ok := val.(*resource.Quantity); ok && ptr != nil {
			qty = *ptr
		} else {
			return "-"
		}
	}

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
