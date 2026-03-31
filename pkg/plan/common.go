package plan

import (
	"fmt"
	"github.com/weka/kubectl-weka/pkg/types"
	"github.com/weka/kubectl-weka/pkg/utils"
	"os"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

// GetFileContents reads file contents
func GetFileContents(filePath string) ([]byte, error) {
	return GetFileContentsFromPath(filePath)
}

// GetFileContentsFromPath is a wrapper for reading file contents
func GetFileContentsFromPath(filePath string) ([]byte, error) {
	// This will be implemented using the actual file reading
	// For now it's a stub that uses the OS package
	content, err := GetFileBytes(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", filePath, err)
	}
	return content, nil
}

// GetFileBytes reads bytes from a file
func GetFileBytes(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

// SelectorToString converts selector map to a readable string
func SelectorToString(selector map[string]string) string {
	if len(selector) == 0 {
		return "none"
	}
	var pairs []string
	for k, v := range selector {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

// CalculatePodResourceUsage sums up resource requests for all pods on a node
func CalculatePodResourceUsage(pods []v1.Pod, resourceName v1.ResourceName) resource.Quantity {
	total := resource.NewQuantity(0, resource.BinarySI)

	for _, pod := range pods {
		// Check regular containers
		for _, container := range pod.Spec.Containers {
			if container.Resources.Requests != nil {
				if val, ok := container.Resources.Requests[resourceName]; ok {
					total.Add(val)
				}
			}
		}

		// Check init containers
		for _, container := range pod.Spec.InitContainers {
			if container.Resources.Requests != nil {
				if val, ok := container.Resources.Requests[resourceName]; ok {
					total.Add(val)
				}
			}
		}
	}

	return *total
}

// createResourceBar creates a visual bar showing USED + WEKA + FREE with colors for each container type
func createResourceBar(usedPercent, wekaPercent float64, containerTypes []string) string {

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
		used = types.ColorUsed + strings.Repeat("█", usedWidth) + types.ColorReset
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
					color = types.ColorCompute
				case "drive":
					color = types.ColorDrive
				case "s3":
					color = types.ColorS3
				case "nfs":
					color = types.ColorNFS
				case "envoy":
					color = types.ColorEnvoy
				case "client":
					color = types.ColorClient
				default:
					color = types.ColorDefault
				}

				weka += color + strings.Repeat("█", width) + types.ColorReset
			}
		}
	} else {
		// Default color if no container types
		if wekaWidth > 0 {
			weka = "\033[35m" + strings.Repeat("█", wekaWidth) + types.ColorReset
		}
	}

	free := ""
	if freeWidth > 0 {
		free = types.ColorFree + strings.Repeat("░", freeWidth) + types.ColorReset
	}

	return fmt.Sprintf("[%s%s%s]", used, weka, free)
}

// NodePlacement represents simulated placement of containers on nodes
type NodePlacement struct {
	NodeName   string
	Containers []PlacedContainer
	UsedCores  int
	UsedMemory int64
	UsedHP     int64
	UsedDrives int // Number of drives allocated to containers on this node
}

type PlacedContainer struct {
	Type      string
	Index     int
	Cores     int
	Memory    int64
	Hugepages int64
	Drives    int // Number of drives needed (only for drive containers)
}

type ContainerRequirements struct {
	Type      string
	Count     int
	Hugepages int64
	Cores     int // Cores with HT
	CoresNoHT int // Cores without HT
	Memory    int64
	Drives    int // Number of drives required (only for drive containers)
}

// ============================================================================
// Node Requirements Calculation & Display
// ============================================================================

// CalculateNodeRequirements calculates minimum node requirements per purpose (Backend, Frontend)
// Adds 10% spare capacity to all resources
func CalculateNodeRequirements(_ *wekaapi.WekaConfig, containers []ContainerRequirements) []NodeRequirements {
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
		backendNodes := utils.MaxInt(computeCount, driveCount)

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

		// Add 10% spare
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
		frontendNodes := utils.MaxInt(s3Count, nfsCount)

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

		// Add 10% spare
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

	// Handle client containers
	clientCount := 0
	var clientReq ContainerRequirements

	for _, c := range containers {
		if c.Type == "client" {
			clientCount = c.Count
			clientReq = c
		}
	}

	if clientCount > 0 {
		// Clients deploy one instance per node
		totalCores := clientReq.Cores
		totalCoresNoHT := clientReq.CoresNoHT
		totalHugepages := clientReq.Hugepages
		totalMemory := clientReq.Memory

		// Add 10% spare
		totalCores = int(float64(totalCores) * 1.1)
		totalCoresNoHT = int(float64(totalCoresNoHT) * 1.1)
		totalHugepages = int64(float64(totalHugepages) * 1.1)
		totalMemory = int64(float64(totalMemory) * 1.1)

		nodeReqs = append(nodeReqs, NodeRequirements{
			Purpose:          "Client",
			MinNodes:         clientCount,
			CoresPerNode:     totalCores,
			CoresPerNodeNoHT: totalCoresNoHT,
			HugepagesPerNode: totalHugepages,
			MemoryPerNode:    totalMemory,
			Description:      fmt.Sprintf("To accommodate %d client container(s)", clientCount),
		})
	}

	return nodeReqs
}

// PrintNodeRequirements displays node requirements in a formatted table
func PrintNodeRequirements(nodeReqs []NodeRequirements) {
	if len(nodeReqs) > 0 {
		fmt.Printf("Recommendation\t-\t-\t-\t-\t-\tAt least 1 more node of required capacity for fault tolerance\n")
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

	// Sort by node count descending
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
	fmt.Println("\n=== Node Requirements (with 10% spare) ===")
	t.Render()

	if len(nodeReqs) > 0 && t.Length() > 1 { // this is not a client-only deployment
		fmt.Printf("\n💡 Recommendation: At least 1 more node of the required capacity is recommended to provide fault tolerance.\n")
	}
}
