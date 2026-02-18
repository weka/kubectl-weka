package cmd

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

	// NIC detection
	Mellanox       bool   `json:"mellanox"`
	MellanoxDetail string `json:"mellanox_detail"`

	// Mellanox interface inventory + bonds
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
}
