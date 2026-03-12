package cmd

import ds "github.com/weka/kubectl-weka/pkg/device-support"

// CNIDetection contains detected CNI configuration from the node
type CNIDetection struct {
	PodCIDR  string `json:"pod_cidr,omitempty"` // Detected Pod CIDR (e.g., "10.244.0.0/24")
	Source   string `json:"source,omitempty"`   // Source of detection (kubelet_config, kubelet_args, flannel_data, config_files)
	CNIType  string `json:"cni_type,omitempty"` // CNI implementation (flannel, calico, weave, unknown)
	Detected bool   `json:"detected"`           // Whether CNI was successfully detected
}

// NetworkInterfaceMetrics contains network traffic and error statistics
type NetworkInterfaceMetrics struct {
	BytesIn      int64 `json:"bytes_in"`      // Total bytes received
	BytesOut     int64 `json:"bytes_out"`     // Total bytes sent
	PacketsIn    int64 `json:"packets_in"`    // Total packets received
	PacketsOut   int64 `json:"packets_out"`   // Total packets sent
	ErrorsIn     int64 `json:"errors_in"`     // Inbound errors
	ErrorsOut    int64 `json:"errors_out"`    // Outbound errors
	DroppedIn    int64 `json:"dropped_in"`    // Inbound dropped packets
	DroppedOut   int64 `json:"dropped_out"`   // Outbound dropped packets
	CollisionsIn int64 `json:"collisions_in"` // Collision errors
	OverrunsIn   int64 `json:"overruns_in"`   // Buffer overruns on input
	OverrunsOut  int64 `json:"overruns_out"`  // Buffer overruns on output
	CRCErrors    int64 `json:"crc_errors"`    // CRC errors
}

// RouteEntry represents a single route entry
type RouteEntry struct {
	Destination string `json:"destination"`        // Destination CIDR (e.g., "10.0.0.0/24" or "default")
	Gateway     string `json:"gateway,omitempty"`  // Gateway IP or "on-link"
	Device      string `json:"device,omitempty"`   // Interface name
	Metric      int    `json:"metric,omitempty"`   // Route metric/cost
	Source      string `json:"source,omitempty"`   // Source IP for policy-based routing
	Table       string `json:"table,omitempty"`    // Routing table name (main, local, custom)
	Protocol    string `json:"protocol,omitempty"` // Protocol (kernel, boot, static, etc.)
}

// RoutingRule represents a policy-based routing rule
type RoutingRule struct {
	Priority  int    `json:"priority"`            // Rule priority (lower = higher priority)
	Condition string `json:"condition,omitempty"` // Condition (from IP, to IP, etc.)
	Table     string `json:"table,omitempty"`     // Target routing table
	Action    string `json:"action,omitempty"`    // Action (lookup, blackhole, etc.)
}

// RoutingTableInfo contains all routes and rules for a routing table
type RoutingTableInfo struct {
	TableName string       `json:"table_name"`         // Table name (main, local, custom)
	TableID   int          `json:"table_id,omitempty"` // Table ID number
	Routes    []RouteEntry `json:"routes"`             // All routes in this table
}

// SubnetInterface represents a network interface on a subnet
type SubnetInterface struct {
	Name string `json:"name"` // Interface name (eth0, ib0, etc.)
	IP   string `json:"ip"`   // IP address on this subnet (x.x.x.x)
}

// Subnet represents a network subnet with its interfaces
type Subnet struct {
	NetworkAddress string            `json:"network_address"` // Network address (x.x.x.x)
	Netmask        string            `json:"netmask"`         // Netmask (x.x.x.x)
	CIDR           string            `json:"cidr"`            // CIDR notation (x.x.x.x/y)
	Interfaces     []SubnetInterface `json:"interfaces"`      // Interfaces on this subnet
	InterfaceCount int               `json:"interface_count"` // Number of interfaces
	IsCNISubnet    bool              `json:"is_cni_subnet"`   // True if Kubernetes CNI subnet
}

// NetworkNamespaceRouting contains routing info for a network namespace
type NetworkNamespaceRouting struct {
	Namespace     string             `json:"namespace"`      // Namespace name (empty = default)
	RoutingTables []RoutingTableInfo `json:"routing_tables"` // All routing tables
	RoutingRules  []RoutingRule      `json:"routing_rules"`  // All policy-based routing rules
	RuleCount     int                `json:"rule_count"`     // Total number of rules
	Subnets       []Subnet           `json:"subnets"`        // All subnets on the system
	SubnetCount   int                `json:"subnet_count"`   // Total number of subnets
	TableCount    int                `json:"table_count"`    // Total number of tables
}

// NetworkInterface represents a generic network interface (Ethernet, InfiniBand, or Bond)
type NetworkInterface struct {
	Name             string                   `json:"name"`                      // e.g., "eth0", "ib0", "bond0"
	Type             string                   `json:"type"`                      // "ethernet", "infiniband", or "bond"
	IP               string                   `json:"ip,omitempty"`              // CIDR notation (e.g., 10.0.0.1/24)
	MTU              int                      `json:"mtu,omitempty"`             // Maximum Transmission Unit
	MAC              string                   `json:"mac,omitempty"`             // MAC address
	BondMaster       string                   `json:"bond_master,omitempty"`     // Bond interface this is enslaved to (for slaves)
	IsBondSlave      bool                     `json:"is_bond_slave"`             // Whether this is a bond slave
	BondMode         string                   `json:"bond_mode,omitempty"`       // Bond mode (e.g. "802.3ad") - only for bonds
	BondSlaves       []string                 `json:"bond_slaves,omitempty"`     // Slave interfaces (only for bonds)
	MaxSpeed         string                   `json:"max_speed,omitempty"`       // Maximum speed (e.g., "100Gbps")
	EffectiveSpeed   string                   `json:"effective_speed,omitempty"` // Current speed (e.g., "40Gbps")
	PCIAddress       string                   `json:"pci_address"`               // PCI address (e.g., "0000:3d:00.0")
	NUMANode         int                      `json:"numa_node"`                 // NUMA node (-1 if unknown)
	Model            string                   `json:"model,omitempty"`           // NIC model (e.g., "CX-7")
	VendorModel      string                   `json:"vendor_model,omitempty"`    // Vendor:Model in format "1234:5678" (e.g., "15b3:1021" for Mellanox CX-7)
	Metrics          *NetworkInterfaceMetrics `json:"metrics,omitempty"`         // Network statistics
	Status           string                   `json:"status,omitempty"`          // Interface status (up/down)
	IsDefaultRoute   bool                     `json:"is_default_route"`          // True if used as default route (0.0.0.0/0)
	AssociatedRoutes []RouteEntry             `json:"associated_routes"`         // Routes using this interface
	RouteCount       int                      `json:"route_count"`               // Number of routes using this interface
}

// IsBond returns true if this interface is a bond
func (ni *NetworkInterface) IsBond() bool {
	return ni.Type == "bond"
}

// IsInfiniBand returns true if this interface is an InfiniBand interface
func (ni *NetworkInterface) IsInfiniBand() bool {
	return ni.Type == "infiniband"
}

// IsEthernet returns true if this interface is an Ethernet interface
func (ni *NetworkInterface) IsEthernet() bool {
	return ni.Type == "ethernet"
}

// NetworkInterface Methods - Device Detection and Capability Checking

// IsSupportedByWekaDpdk checks if the NIC can be used with Weka DPDK.
// Optionally validates against a Weka version if forWekaVersion is specified.
// Note: May require dedicated NIC per Weka process depending on device capabilities
func (ni *NetworkInterface) IsSupportedByWekaDpdk(forWekaVersion ...string) bool {
	if ni == nil {
		return false
	}

	var wekaVersion string
	if len(forWekaVersion) > 0 {
		wekaVersion = forWekaVersion[0]
	}

	if ni.VendorModel != "" {
		caps := ds.GetNICCapabilities(ni.VendorModel)
		if !caps.SupportedByWekaDpdk {
			return false
		}

		// Check version constraints if a Weka version was provided
		if wekaVersion != "" {
			devInfo := ds.GetNICInfo(ni.VendorModel)
			if devInfo != nil && !ds.IsVersionInRange(wekaVersion, devInfo.MinSupportedWekaVersion, devInfo.MaxSupportedWekaVersion) {
				return false
			}
		}

		return true
	}
	// Fallback: device not in registry, no support
	return false
}

// IsSupportedByWekaDpdkSingleNic checks if multiple Weka processes can share this NIC.
// Optionally validates against a Weka version if forWekaVersion is specified.
// These are typically high-performance devices like Mellanox that support this mode
func (ni *NetworkInterface) IsSupportedByWekaDpdkSingleNic(forWekaVersion ...string) bool {
	if ni == nil {
		return false
	}

	var wekaVersion string
	if len(forWekaVersion) > 0 {
		wekaVersion = forWekaVersion[0]
	}

	if ni.VendorModel != "" {
		caps := ds.GetNICCapabilities(ni.VendorModel)
		if !caps.SupportedByWekaDpdkSingleNic {
			return false
		}

		// Check version constraints if a Weka version was provided
		if wekaVersion != "" {
			devInfo := ds.GetNICInfo(ni.VendorModel)
			if devInfo != nil && !ds.IsVersionInRange(wekaVersion, devInfo.MinSupportedWekaVersion, devInfo.MaxSupportedWekaVersion) {
				return false
			}
		}

		return true
	}
	// Fallback: device not in registry, no support
	return false
}

// IsSupportedByWekaForLacpSameCard checks if LACP bonding is supported using 2 ports on the same NIC.
// Optionally validates against a Weka version if forWekaVersion is specified.
// Currently only certain high-performance devices support this
func (ni *NetworkInterface) IsSupportedByWekaForLacpSameCard(forWekaVersion ...string) bool {
	if ni == nil {
		return false
	}

	var wekaVersion string
	if len(forWekaVersion) > 0 {
		wekaVersion = forWekaVersion[0]
	}

	if ni.VendorModel != "" {
		caps := ds.GetNICCapabilities(ni.VendorModel)
		if !caps.SupportedByWekaForLacpSameCard {
			return false
		}

		// Check version constraints if a Weka version was provided
		if wekaVersion != "" {
			devInfo := ds.GetNICInfo(ni.VendorModel)
			if devInfo != nil && !ds.IsVersionInRange(wekaVersion, devInfo.MinSupportedWekaVersion, devInfo.MaxSupportedWekaVersion) {
				return false
			}
		}

		return true
	}
	// Fallback: device not in registry, no support
	return false
}

// IsSupportedByWekaForLacpDiffCards checks if LACP bonding is supported across different NIC cards.
// Optionally validates against a Weka version if forWekaVersion is specified.
// Currently not supported for any devices
func (ni *NetworkInterface) IsSupportedByWekaForLacpDiffCards(forWekaVersion ...string) bool {
	if ni == nil {
		return false
	}

	var wekaVersion string
	if len(forWekaVersion) > 0 {
		wekaVersion = forWekaVersion[0]
	}

	if ni.VendorModel != "" {
		caps := ds.GetNICCapabilities(ni.VendorModel)
		if !caps.SupportedByWekaForLacpDiffCards {
			return false
		}

		// Check version constraints if a Weka version was provided
		if wekaVersion != "" {
			devInfo := ds.GetNICInfo(ni.VendorModel)
			if devInfo != nil && !ds.IsVersionInRange(wekaVersion, devInfo.MinSupportedWekaVersion, devInfo.MaxSupportedWekaVersion) {
				return false
			}
		}

		return true
	}
	return false
}

// GetDeviceInfo returns detailed information about the NIC
func (ni *NetworkInterface) GetDeviceInfo() *ds.NICInfo {
	if ni == nil || ni.VendorModel == "" {
		return nil
	}
	return ds.GetNICInfo(ni.VendorModel)
}

// NetworkInterfaces is a slice of NetworkInterface with helper methods
type NetworkInterfaces []NetworkInterface

// GetBonds returns all bond interfaces
func (ni NetworkInterfaces) GetBonds() []NetworkInterface {
	var bonds []NetworkInterface
	for _, iface := range ni {
		if iface.Type == "bond" {
			bonds = append(bonds, iface)
		}
	}
	return bonds
}

// GetEthernets returns all ethernet interfaces
func (ni NetworkInterfaces) GetEthernets() []NetworkInterface {
	var ethernets []NetworkInterface
	for _, iface := range ni {
		if iface.Type == "ethernet" {
			ethernets = append(ethernets, iface)
		}
	}
	return ethernets
}

// GetInfiniBands returns all infiniband interfaces
func (ni NetworkInterfaces) GetInfiniBands() []NetworkInterface {
	var ib []NetworkInterface
	for _, iface := range ni {
		if iface.Type == "infiniband" {
			ib = append(ib, iface)
		}
	}
	return ib
}

// GetVirtualInterfaces returns all virtual interfaces (bonds)
func (ni NetworkInterfaces) GetVirtualInterfaces() []NetworkInterface {
	return ni.GetBonds()
}

// NvmeDrive contains information about a single NVMe drive discovered during hostcheck
type NvmeDrive struct {
	DeviceName   string `json:"device_name"`            // e.g., "nvme0n1"
	DevicePath   string `json:"device_path"`            // e.g., "/dev/nvme0n1"
	SerialNumber string `json:"serial"`                 // Drive serial number
	Model        string `json:"model"`                  // Drive model
	Size         int64  `json:"size"`                   // Size in bytes
	Mounted      bool   `json:"mounted"`                // Is the drive currently mounted?
	MountPoint   string `json:"mount_point"`            // Mount point if mounted
	PCIAddress   string `json:"pci_address"`            // PCI address (e.g., "0000:01:00.0")
	NUMANode     int    `json:"numa_node"`              // NUMA node (-1 if unknown)
	VendorModel  string `json:"vendor_model,omitempty"` // Vendor:Model in format "1234:5678"
}

// GetDeviceInfo returns detailed device information from the registry for this drive
// Uses the VendorModel to look up information in the NvmeRegistry
func (nd *NvmeDrive) GetDeviceInfo() *ds.NVMeInfo {
	if nd == nil || nd.VendorModel == "" {
		return nil
	}
	return ds.GetNVMeInfo(nd.VendorModel)
}

type HostChecksResult struct {
	// OS detection via /etc/os-release on host
	IsRHCOS   bool   `json:"is_rhcos"`
	OSRelease string `json:"os_release"`

	// Kernel version detection via /proc/version
	KernelVersion string `json:"kernel_version"`

	// Weka directory existence and available space
	WekaDirExists     bool   `json:"weka_dir_exists"`
	WekaDirPath       string `json:"weka_dir_path"`
	WekaDirAvailBytes int64  `json:"weka_dir_avail_bytes"`

	// XFS tools availability
	XFSFound bool `json:"xfs_found"`

	// Weka agent service presence
	WekaAgentServiceExists bool `json:"weka_agent_service_exists"`

	// Generic Network Interfaces (Ethernet + InfiniBand)
	NetworkInterfaces     NetworkInterfaces `json:"network_interfaces"`      // All network interfaces
	NetworkInterfaceCount int               `json:"network_interface_count"` // Total count

	// Routing Configuration (for source-based routing and multi-path verification)
	NetworkNamespaceRouting *NetworkNamespaceRouting `json:"network_namespace_routing"` // Routing info for default namespace

	// CNI Detection (detected from node configuration)
	CNIDetection *CNIDetection `json:"cni_detection,omitempty"` // Detected CNI Pod CIDR configuration

	// CPU and Memory info
	HTEnabled       bool   `json:"ht_enabled"`
	PhysicalCores   int    `json:"physical_cores"`
	LogicalCores    int    `json:"logical_cores"`
	MemoryBytes     int64  `json:"memory_bytes"`
	FreeMemoryBytes int64  `json:"free_memory_bytes"`
	HugepagesFree   int64  `json:"hugepages_free_bytes"`
	CPUModel        string `json:"cpu_model"`
	CPUFamily       string `json:"cpu_family"`  // e.g., "Intel", "AMD", "ARM"
	CPUArch         string `json:"cpu_arch"`    // e.g., "x86_64", "aarch64"
	CPUSockets      int    `json:"cpu_sockets"` // Number of CPU sockets

	// NVMe drive detection
	NVMeDrives     []NvmeDrive `json:"nvme_drives"`
	NVMeDriveCount int         `json:"nvme_drive_count"`
}

// Validation helper methods for HostChecksResult

// IsWekaDirExists returns true if the Weka directory exists
func (hc *HostChecksResult) IsWekaDirExists() bool {
	return hc.WekaDirExists
}

// IsWekaDirAtLeast returns true if the Weka directory has at least n bytes available
func (hc *HostChecksResult) IsWekaDirAtLeast(n int64) bool {
	return hc.WekaDirExists && hc.WekaDirAvailBytes >= n
}

// HasXFS returns true if XFS tools are available
func (hc *HostChecksResult) HasXFS() bool {
	return hc.XFSFound
}

// IsWekaAgentClean returns true if Weka agent service is not present
func (hc *HostChecksResult) IsWekaAgentClean() bool {
	return !hc.WekaAgentServiceExists
}

// HasCustomRoutingRules returns true if custom routing rules are configured
func (hc *HostChecksResult) HasCustomRoutingRules() bool {
	if hc.NetworkNamespaceRouting == nil {
		return false
	}
	// Count custom rules (exclude default: priority 0, 32766, 32767)
	for _, rule := range hc.NetworkNamespaceRouting.RoutingRules {
		if rule.Priority != 0 && rule.Priority != 32766 && rule.Priority != 32767 {
			return true
		}
	}
	return false
}

// HasNetworkInterfaces returns true if any network interfaces were detected
func (hc *HostChecksResult) HasNetworkInterfaces() bool {
	return hc.NetworkInterfaceCount > 0
}

// HasNVMeDrives returns true if any NVMe drives were detected
func (hc *HostChecksResult) HasNVMeDrives() bool {
	return hc.NVMeDriveCount > 0
}
