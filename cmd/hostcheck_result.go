package cmd

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

// NetworkInterface represents a generic network interface (Ethernet or InfiniBand)
type NetworkInterface struct {
	Name           string                   `json:"name"`                      // e.g., "eth0", "ib0"
	Type           string                   `json:"type"`                      // "ethernet" or "infiniband"
	IP             string                   `json:"ip,omitempty"`              // CIDR notation (e.g., 10.0.0.1/24)
	MTU            int                      `json:"mtu,omitempty"`             // Maximum Transmission Unit
	MAC            string                   `json:"mac,omitempty"`             // MAC address
	BondMaster     string                   `json:"bond_master,omitempty"`     // Bond interface this is enslaved to
	BondSlave      bool                     `json:"bond_slave"`                // Whether this is a bond slave
	MaxSpeed       string                   `json:"max_speed,omitempty"`       // Maximum speed (e.g., "100Gbps")
	EffectiveSpeed string                   `json:"effective_speed,omitempty"` // Current speed (e.g., "40Gbps")
	PCIAddress     string                   `json:"pci_address,omitempty"`     // PCI address (e.g., "0000:3d:00.0")
	Model          string                   `json:"model,omitempty"`           // NIC model (e.g., "CX-7")
	Metrics        *NetworkInterfaceMetrics `json:"metrics,omitempty"`         // Network statistics
	Status         string                   `json:"status,omitempty"`          // Interface status (up/down)
}

// MellanoxIface contains Mellanox-specific network interface information
type MellanoxIface struct {
	Name  string `json:"name"`
	Bond  string `json:"bond,omitempty"` // bond name if enslaved
	IP    string `json:"ip,omitempty"`   // CIDR (e.g. 192.168.1.2/24) when not enslaved
	Model string `json:"model"`          // e.g. "CX-7" or "unknown (15b3:1023 on 0000:3d:00.0)"
	Speed string `json:"speed,omitempty"`
}

type BondInfo struct {
	Name   string   `json:"name"`
	IP     string   `json:"ip,omitempty"` // CIDR
	Slaves []string `json:"slaves"`
	Mode   string   `json:"mode,omitempty"` // e.g. "802.3ad"
	Speed  string   `json:"speed,omitempty"`
}

type HostChecksResult struct {
	// OS detection via /etc/os-release on host
	IsRHCOS   bool   `json:"is_rhcos"`
	OSRelease string `json:"os_release"`

	// Kernel version detection via /proc/version
	KernelVersion string `json:"kernel_version"`

	// Weka directory exists + has >=300GB available
	WekaDirOK         bool   `json:"weka_dir_ok"`
	WekaDirPath       string `json:"weka_dir_path"`
	WekaDirDetail     string `json:"weka_dir_detail"`
	WekaDirAvailBytes int64  `json:"weka_dir_avail_bytes"`

	// XFS tools
	XFSInstalled bool   `json:"xfs_installed"`
	XFSDetail    string `json:"xfs_detail"`

	// Weka client presence
	WekaClientClean  bool   `json:"weka_client_clean"`
	WekaClientDetail string `json:"weka_client_detail"`

	// Generic Network Interfaces (Ethernet + InfiniBand)
	NetworkInterfaces      []NetworkInterface `json:"network_interfaces"`       // All network interfaces
	NetworkInterfaceCount  int                `json:"network_interface_count"`  // Total count
	NetworkInterfaceDetail string             `json:"network_interface_detail"` // Summary details

	// NIC detection
	Mellanox       bool   `json:"mellanox"`
	MellanoxDetail string `json:"mellanox_detail"`

	// Mellanox-specific interface inventory + bonds (kept for backward compatibility)
	MlxIfaces []MellanoxIface `json:"mlx_ifaces"`
	MlxBonds  []BondInfo      `json:"mlx_bonds"`

	BondLACPOk     bool   `json:"bond_lacp_ok"`
	BondLACPDetail string `json:"bond_lacp_detail"`

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
	NVMeDrives      []NVMeDriveInfo `json:"nvme_drives"`
	NVMeDriveCount  int             `json:"nvme_drive_count"`
	NVMeDriveDetail string          `json:"nvme_drive_detail"`
}

// NVMeDriveInfo contains information about a single NVMe drive
type NVMeDriveInfo struct {
	DeviceName   string `json:"device_name"` // e.g., "nvme0n1"
	DevicePath   string `json:"device_path"` // e.g., "/dev/nvme0n1"
	SerialNumber string `json:"serial"`      // Drive serial number
	Model        string `json:"model"`       // Drive model
	Size         int64  `json:"size"`        // Size in bytes
	Mounted      bool   `json:"mounted"`     // Is the drive currently mounted?
	MountPoint   string `json:"mount_point"` // Mount point if mounted
	PCIAddress   string `json:"pci_address"` // PCI address (e.g., "0000:01:00.0")
}
