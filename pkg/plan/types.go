package plan

import (
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// RoleNodeGrouping represents nodes grouped by role and global selection
type RoleNodeGrouping struct {
	Global map[string]v1.Node
	ByRole map[string]struct {
		Selector map[string]string
		Nodes    []v1.Node
	}
}

// ConvergedNodeState tracks resource usage on a node through multiple allocation phases
type ConvergedNodeState struct {
	NodeName string
	Node     *v1.Node

	// Original state (before any simulation)
	OriginalUsedCPU    resource.Quantity
	OriginalUsedMemory resource.Quantity
	OriginalUsedHP     resource.Quantity

	// After cluster allocation
	ClusterContainers []PlacedContainer
	ClusterUsedCores  int
	ClusterUsedMemory int64 // MiB
	ClusterUsedHP     int64 // MiB
	ClusterUsedDrives int   // Number of drives allocated to cluster containers

	// After client allocation
	ClientContainers []PlacedContainer
	ClientUsedCores  int
	ClientUsedMemory int64 // MiB
	ClientUsedHP     int64 // MiB

	// Total allocatable
	AllocatableCPU    resource.Quantity
	AllocatableMemory resource.Quantity
	AllocatableHP     resource.Quantity
}

// NetworkValidationError represents a network validation error with severity
type NetworkValidationError struct {
	NodeName string
	NICName  string
	Severity string // "ERROR", "WARN"
	Message  string
}

func (e NetworkValidationError) String() string {
	return fmt.Sprintf("[%s] Node %s NIC %s: %s", e.Severity, e.NodeName, e.NICName, e.Message)
}

// NetworkValidationResult holds the results of network validation
type NetworkValidationResult struct {
	Errors   []NetworkValidationError
	Warnings []NetworkValidationError
	Valid    bool
}

// NetworkInterfaceStats tracks the status of a network interface across nodes
type NetworkInterfaceStats struct {
	InterfaceName string
	Configured    int
	Missing       int
	Misconfigured int
}

// NetworkValidationStats holds aggregated statistics for network validation
type NetworkValidationStats struct {
	InterfaceStats map[string]*NetworkInterfaceStats
}

// NewNetworkValidationStats creates a new stats collector
func NewNetworkValidationStats() *NetworkValidationStats {
	return &NetworkValidationStats{
		InterfaceStats: make(map[string]*NetworkInterfaceStats),
	}
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

// NodeRequirements represents the minimum node requirements for a specific purpose
type NodeRequirements struct {
	Purpose          string
	MinNodes         int
	CoresPerNode     int // Cores per node with HT
	CoresPerNodeNoHT int // Cores per node without HT
	HugepagesPerNode int64
	MemoryPerNode    int64
	Description      string
}
