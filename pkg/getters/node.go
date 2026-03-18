package getters

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/utils"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

// GenerateNodesOutput generates the nodes information table as a string
func GenerateNodesOutput(ctx context.Context, clients *kubernetes.K8sClients, printerObj printer.ResourcePrinter, nodeSelector string) (string, error) {
	crClient := clients.CRClient

	// List nodes using the cached client, optionally filtered by label selector
	var nodeList v1.NodeList
	var opts []client.ListOption
	if nodeSelector != "" {
		opts = append(opts, client.MatchingLabels(utils.ParseSelector(nodeSelector)))
	}
	if err := crClient.List(ctx, &nodeList, opts...); err != nil {
		return "", err
	}

	// List all pods to calculate actual hugepage allocations per node
	var podList v1.PodList
	if err := crClient.List(ctx, &podList); err != nil {
		return "", err
	}

	// Build a map of node -> hugepage allocations
	hugepageAllocations := calculateHugepageAllocations(&podList)

	// Build maps of node -> cores and RAM allocations
	coreAllocations := calculateResourceAllocations(&podList, v1.ResourceCPU)
	ramAllocations := calculateResourceAllocations(&podList, v1.ResourceMemory)

	// Define columns slice (single, with VisibleInWide)
	cols := []printer.TableColumn{
		{Name: "NAME", VisibleInWide: false},
		{Name: "IP", VisibleInWide: false},
		{Name: "OS", VisibleInWide: false},
		{Name: "ARCH", VisibleInWide: false},
		{Name: "KERNEL", VisibleInWide: false},
		{Name: "STATUS", VisibleInWide: false},
		{Name: "HP2MI_USABLE", VisibleInWide: true, FormatFuncs: printer.TableFormatFunctions{utils.FormatQuantityToGB}},
		{Name: "HP2MI_ALLOC", VisibleInWide: true, FormatFuncs: printer.TableFormatFunctions{utils.FormatQuantityToGB}},
		{Name: "HP2MI_FREE", VisibleInWide: false, FormatFuncs: printer.TableFormatFunctions{utils.FormatQuantityToGB}},
		{Name: "CORES_USABLE", VisibleInWide: true, FormatFuncs: printer.TableFormatFunctions{utils.FormatQuantityToGB}},
		{Name: "CORES_ALLOC", VisibleInWide: true, FormatFuncs: printer.TableFormatFunctions{utils.FormatQuantityToGB}},
		{Name: "CORES_FREE", VisibleInWide: false, FormatFuncs: printer.TableFormatFunctions{utils.FormatQuantityToGB}},
		{Name: "RAM_USABLE", VisibleInWide: true, FormatFuncs: printer.TableFormatFunctions{utils.FormatQuantityToGB}},
		{Name: "RAM_ALLOC", VisibleInWide: true, FormatFuncs: printer.TableFormatFunctions{utils.FormatQuantityToGB}},
		{Name: "RAM_FREE", VisibleInWide: false, FormatFuncs: printer.TableFormatFunctions{utils.FormatQuantityToGB}},
	}

	var rows []printer.TableRow
	for i := range nodeList.Items {
		n := &nodeList.Items[i]
		row := printer.TableRow{Values: map[string]interface{}{}}
		row.Values["NAME"] = n.Name
		row.Values["IP"] = kubernetes.FirstInternalIP(n)
		row.Values["OS"] = n.Status.NodeInfo.OSImage
		row.Values["ARCH"] = n.Status.NodeInfo.Architecture
		row.Values["KERNEL"] = n.Status.NodeInfo.KernelVersion
		row.Values["STATUS"] = func() string {
			status := "NotReady"
			for _, condition := range n.Status.Conditions {
				if condition.Type == v1.NodeReady {
					if condition.Status == v1.ConditionTrue {
						uptime := "unknown"
						if n.Status.NodeInfo.BootID != "" {
							uptime = utils.HumanAge(condition.LastTransitionTime.Time)
						}
						status = fmt.Sprintf("Ready (%s)", uptime)
					}
					break
				}
			}
			return status
		}()
		row.Values["HP2MI_USABLE"] = n.Status.Allocatable["hugepages-2Mi"]
		row.Values["HP2MI_ALLOC"] = hugepageAllocations[n.Name]["hugepages-2Mi"]
		row.Values["HP2MI_FREE"] = func() resource.Quantity {
			free := n.Status.Allocatable["hugepages-2Mi"].DeepCopy()
			free.Sub(hugepageAllocations[n.Name]["hugepages-2Mi"])
			return free
		}()
		row.Values["CORES_USABLE"] = n.Status.Allocatable[v1.ResourceCPU]
		row.Values["CORES_ALLOC"] = coreAllocations[n.Name][v1.ResourceCPU]
		row.Values["CORES_FREE"] = func() resource.Quantity {
			free := n.Status.Allocatable[v1.ResourceCPU].DeepCopy()
			free.Sub(coreAllocations[n.Name][v1.ResourceCPU])
			return free
		}()
		row.Values["RAM_USABLE"] = n.Status.Allocatable[v1.ResourceMemory]
		row.Values["RAM_ALLOC"] = ramAllocations[n.Name][v1.ResourceMemory]
		row.Values["RAM_FREE"] = func() resource.Quantity {
			free := n.Status.Allocatable[v1.ResourceMemory].DeepCopy()
			free.Sub(ramAllocations[n.Name][v1.ResourceMemory])
			return free
		}()
		rows = append(rows, row)
	}

	// Render output
	var buf strings.Builder
	if err := printerObj.Print(cols, rows, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// calculateHugepageAllocations sums up hugepage requests from all Pods per node
func calculateHugepageAllocations(podList *v1.PodList) map[string]v1.ResourceList {
	allocations := make(map[string]v1.ResourceList)

	for i := range podList.Items {
		pod := &podList.Items[i]
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue // Pod not yet scheduled
		}

		if allocations[nodeName] == nil {
			allocations[nodeName] = make(v1.ResourceList)
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
func calculateResourceAllocations(podList *v1.PodList, resourceName v1.ResourceName) map[string]v1.ResourceList {
	allocations := make(map[string]v1.ResourceList)

	for i := range podList.Items {
		pod := &podList.Items[i]
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue // Pod not yet scheduled
		}

		if allocations[nodeName] == nil {
			allocations[nodeName] = make(v1.ResourceList)
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
