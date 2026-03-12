package cmd

import (
	"fmt"
	ds "github.com/weka/kubectl-weka/pkg/device-support"
)

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

// NetworkInterface represents a generic network interface (Ethernet, InfiniBand, Bond, or VLAN)
type NetworkInterface struct {
	Name             string                   `json:"name"`                      // e.g., "eth0", "ib0", "bond0", "eth0.100"
	Type             string                   `json:"type"`                      // "ethernet", "infiniband", "bond", or "vlan"
	IP               string                   `json:"ip,omitempty"`              // CIDR notation (e.g., 10.0.0.1/24)
	MTU              int                      `json:"mtu,omitempty"`             // Maximum Transmission Unit
	MAC              string                   `json:"mac,omitempty"`             // MAC address
	BondMaster       string                   `json:"bond_master,omitempty"`     // Bond interface this is enslaved to (for slaves)
	IsBondSlave      bool                     `json:"is_bond_slave"`             // Whether this is a bond slave
	BondMode         string                   `json:"bond_mode,omitempty"`       // Bond mode (e.g. "802.3ad") - only for bonds
	BondSlaves       []string                 `json:"bond_slaves,omitempty"`     // Slave interfaces (only for bonds)
	VLANParent       string                   `json:"vlan_parent,omitempty"`     // Parent interface for VLAN (e.g., "eth0" for "eth0.100")
	VLANID           int                      `json:"vlan_id,omitempty"`         // VLAN ID (only for VLAN interfaces)
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

	// Internal reference to parent NetworkInterfaces list (not JSON serialized)
	// Populated when HostChecksResult is created, allows navigation within interface hierarchy
	parent *NetworkInterfaces `json:"-"`
}

// IsBond returns true if this interface is a bond
func (ni *NetworkInterface) IsBond() bool {
	return ni != nil && ni.Type == "bond"
}

func (ni *NetworkInterface) IsLACP() bool {
	return ni != nil && ni.IsBond() && ni.BondMode == "lacp"
}

// IsVlan returns true if this interface is a VLAN interface
func (ni *NetworkInterface) IsVlan() bool {
	return ni != nil && ni.Type == "vlan"
}

// GetParent returns the parent interface for a VLAN or bond slave
// For VLANs, returns the parent interface (e.g., eth0 for eth0.100)
// For regular interfaces, returns nil
func (ni *NetworkInterface) GetParent() *NetworkInterface {
	if ni == nil || ni.parent == nil {
		return nil
	}

	// For VLAN interfaces, get parent by VLANParent name
	if ni.IsVlan() && ni.VLANParent != "" {
		for i, iface := range *ni.parent {
			if iface.Name == ni.VLANParent {
				return &(*ni.parent)[i]
			}
		}
		return nil
	}

	return nil
}

// HasVlans returns true if this interface has VLAN interfaces configured on it
// Only meaningful for non-VLAN interfaces
func (ni *NetworkInterface) HasVlans() bool {
	if ni == nil || ni.IsVlan() || ni.parent == nil {
		return false
	}

	// Check if any VLAN has this interface as parent
	for _, iface := range *ni.parent {
		if iface.IsVlan() && iface.VLANParent == ni.Name {
			return true
		}
	}
	return false
}

// GetVlans returns all VLAN interfaces that are configured on this interface
// Returns empty slice for non-VLAN interfaces
func (ni *NetworkInterface) GetVlans() []*NetworkInterface {
	if ni == nil || ni.IsVlan() || ni.parent == nil {
		return []*NetworkInterface{}
	}

	var vlans []*NetworkInterface
	for i, iface := range *ni.parent {
		if iface.IsVlan() && iface.VLANParent == ni.Name {
			vlans = append(vlans, &(*ni.parent)[i])
		}
	}
	return vlans
}

// IsInfiniBand returns true if this interface is an InfiniBand interface
func (ni *NetworkInterface) IsInfiniBand() bool {
	return ni != nil && ni.Type == "infiniband"
}

// IsEthernet returns true if this interface is an Ethernet interface
func (ni *NetworkInterface) IsEthernet() bool {
	return ni != nil && ni.Type == "ethernet"
}

// GetSlaves returns all NetworkInterfaces that are slaves of this bond
// For non-bond interfaces, returns an empty slice
func (ni *NetworkInterface) GetSlaves() []*NetworkInterface {
	if ni == nil || !ni.IsBond() || ni.parent == nil {
		return []*NetworkInterface{}
	}

	var slaves []*NetworkInterface
	for _, slaveName := range ni.BondSlaves {
		for i := range *ni.parent {
			if (*ni.parent)[i].Name == slaveName {
				slaves = append(slaves, &(*ni.parent)[i])
				break
			}
		}
	}
	return slaves
}

// GetMaster returns the bond interface that has this interface as a slave
// Returns nil if this interface is not a slave or if the master bond is not found
func (ni *NetworkInterface) GetMaster() *NetworkInterface {
	if ni == nil || ni.BondMaster == "" || ni.parent == nil {
		return nil
	}

	for i, iface := range *ni.parent {
		if iface.Name == ni.BondMaster && iface.IsBond() {
			return &(*ni.parent)[i]
		}
	}
	return nil
}

// IsLacpOnSameCard returns true if this is a bond and all slaves are ports on the same NIC card
// For non-bonds or bonds with no slaves, returns false
// Slaves are considered on the same card if they share the same PCI address prefix (e.g., 0000:01:00)
func (ni *NetworkInterface) IsLacpOnSameCard() bool {
	if ni == nil || !ni.IsBond() {
		return false
	}

	slaves := ni.GetSlaves()
	if len(slaves) == 0 {
		return false
	}

	// Extract PCI address prefix (everything before the last dot)
	// e.g., "0000:01:00.0" -> "0000:01:00"
	getPciPrefix := func(pciAddr string) string {
		idx := -1
		for i := 0; i < len(pciAddr); i++ {
			if pciAddr[i] == '.' {
				idx = i
			}
		}
		if idx >= 0 {
			return pciAddr[:idx]
		}
		return pciAddr
	}

	// Get the prefix from the first slave
	firstPrefix := ""
	if len(slaves[0].PCIAddress) > 0 {
		firstPrefix = getPciPrefix(slaves[0].PCIAddress)
	}

	if firstPrefix == "" {
		return false // No valid PCI address to compare
	}

	// Check that all slaves have the same prefix
	for _, slave := range slaves {
		if len(slave.PCIAddress) == 0 {
			return false // Slave has no PCI address
		}
		slavePrefix := getPciPrefix(slave.PCIAddress)
		if slavePrefix != firstPrefix {
			return false // Different NIC card
		}
	}

	return true
}

// NetworkInterface Methods - Device Detection and Capability Checking

// IsSupportedByWekaDpdk checks if the NIC can be used with Weka DPDK.
// Returns (true, nil) if supported, or (false, error) with detailed error message.
// For bonds, returns true only if ALL slaves support DPDK.
// For VLANs, delegates to the parent interface.
// Optionally validates against a Weka version if forWekaVersion is specified.
// Note: May require dedicated NIC per Weka process depending on device capabilities
func (ni *NetworkInterface) IsSupportedByWekaDpdk(forWekaVersion ...string) (bool, error) {
	if ni == nil {
		return false, fmt.Errorf("interface is nil")
	}

	// For VLAN interfaces, check the parent interface
	if ni.IsVlan() {
		parent := ni.GetParent()
		if parent == nil {
			return false, fmt.Errorf("VLAN %s has no parent interface", ni.Name)
		}
		supported, err := parent.IsSupportedByWekaDpdk(forWekaVersion...)
		if !supported {
			return false, fmt.Errorf("VLAN %s (on %s): %w", ni.Name, parent.Name, err)
		}
		return true, nil
	}

	// For bonds, all slaves must support the feature
	if ni.IsBond() {
		slaves := ni.GetSlaves()
		if len(slaves) == 0 {
			return false, fmt.Errorf("bond has no slaves")
		}

		// Check LACP requirement
		if !ni.IsLACP() {
			return false, fmt.Errorf("bond is not using LACP (802.3ad) mode")
		}

		// Check that all slaves support DPDK
		for i, slave := range slaves {
			supported, err := slave.IsSupportedByWekaDpdk(forWekaVersion...)
			if !supported {
				return false, fmt.Errorf("slave %d (%s) does not support DPDK: %w", i, slave.Name, err)
			}
		}

		// Check LACP same card requirement if applicable
		if ni.IsLacpOnSameCard() {
			supported, err := ni.IsSupportedByWekaForLacpSameCard(forWekaVersion...)
			if !supported {
				return false, fmt.Errorf("bond is on same card but does not support same-card LACP: %w", err)
			}
		}

		return true, nil
	}

	var wekaVersion string
	if len(forWekaVersion) > 0 {
		wekaVersion = forWekaVersion[0]
	}

	if ni.VendorModel == "" {
		return false, fmt.Errorf("unknown network device (no VendorModel)")
	}

	caps := ds.GetNICCapabilities(ni.VendorModel)
	if caps == nil {
		return false, fmt.Errorf("device %s not found in NIC registry", ni.VendorModel)
	}

	if !caps.SupportedByWekaDpdk {
		// Try to get device model for better error message
		devInfo := ds.GetNICInfo(ni.VendorModel)
		deviceModel := ni.VendorModel
		if devInfo != nil && devInfo.Model != "" {
			deviceModel = devInfo.Model
		}
		return false, fmt.Errorf("device %s (%s) does not support Weka DPDK", ni.Name, deviceModel)
	}

	// Check version constraints if a Weka version was provided
	if wekaVersion != "" {
		devInfo := ds.GetNICInfo(ni.VendorModel)
		if devInfo != nil {
			deviceModel := ni.VendorModel
			if devInfo.Model != "" {
				deviceModel = devInfo.Model
			}

			if devInfo.MinSupportedWekaVersion != "" && !ds.IsVersionInRange(wekaVersion, devInfo.MinSupportedWekaVersion, devInfo.MaxSupportedWekaVersion) {
				if devInfo.MinSupportedWekaVersion != "" && wekaVersion < devInfo.MinSupportedWekaVersion {
					return false, fmt.Errorf("device %s (%s) requires Weka version %s or later (current: %s)", ni.Name, deviceModel, devInfo.MinSupportedWekaVersion, wekaVersion)
				}
				if devInfo.MaxSupportedWekaVersion != "" && wekaVersion > devInfo.MaxSupportedWekaVersion {
					return false, fmt.Errorf("device %s (%s) is no longer supported after Weka version %s (current: %s)", ni.Name, deviceModel, devInfo.MaxSupportedWekaVersion, wekaVersion)
				}
			}
		}
	}

	return true, nil
}

// IsSupportedByWekaDpdkSingleNic checks if multiple Weka processes can share this NIC.
// Returns (true, nil) if supported, or (false, error) with detailed error message.
// For bonds, returns true only if ALL slaves support single NIC sharing.
// For VLANs, delegates to the parent interface.
// Optionally validates against a Weka version if forWekaVersion is specified.
// These are typically high-performance devices like Mellanox that support this mode
func (ni *NetworkInterface) IsSupportedByWekaDpdkSingleNic(forWekaVersion ...string) (bool, error) {
	if ni == nil {
		return false, fmt.Errorf("interface is nil")
	}

	// For VLAN interfaces, check the parent interface
	if ni.IsVlan() {
		parent := ni.GetParent()
		if parent == nil {
			return false, fmt.Errorf("VLAN %s has no parent interface", ni.Name)
		}
		supported, err := parent.IsSupportedByWekaDpdkSingleNic(forWekaVersion...)
		if !supported {
			return false, fmt.Errorf("VLAN %s (on %s): %w", ni.Name, parent.Name, err)
		}
		return true, nil
	}

	// For bonds, all slaves must support the feature
	if ni.IsBond() {
		slaves := ni.GetSlaves()
		if len(slaves) == 0 {
			return false, fmt.Errorf("bond has no slaves")
		}

		for i, slave := range slaves {
			supported, err := slave.IsSupportedByWekaDpdkSingleNic(forWekaVersion...)
			if !supported {
				return false, fmt.Errorf("slave %d (%s) does not support single NIC sharing: %w", i, slave.Name, err)
			}
		}
		return true, nil
	}

	var wekaVersion string
	if len(forWekaVersion) > 0 {
		wekaVersion = forWekaVersion[0]
	}

	if ni.VendorModel == "" {
		return false, fmt.Errorf("unknown network device (no VendorModel)")
	}

	caps := ds.GetNICCapabilities(ni.VendorModel)
	if caps == nil {
		return false, fmt.Errorf("device %s not found in NIC registry", ni.VendorModel)
	}

	if !caps.SupportedByWekaDpdkSingleNic {
		// Try to get device model for better error message
		devInfo := ds.GetNICInfo(ni.VendorModel)
		deviceModel := ni.VendorModel
		if devInfo != nil && devInfo.Model != "" {
			deviceModel = devInfo.Model
		}
		return false, fmt.Errorf("device %s (%s) does not support single NIC sharing (requires dedicated NIC per process)", ni.Name, deviceModel)
	}

	// Check version constraints if a Weka version was provided
	if wekaVersion != "" {
		devInfo := ds.GetNICInfo(ni.VendorModel)
		if devInfo != nil {
			deviceModel := ni.VendorModel
			if devInfo.Model != "" {
				deviceModel = devInfo.Model
			}

			if devInfo.MinSupportedWekaVersion != "" && !ds.IsVersionInRange(wekaVersion, devInfo.MinSupportedWekaVersion, devInfo.MaxSupportedWekaVersion) {
				if devInfo.MinSupportedWekaVersion != "" && wekaVersion < devInfo.MinSupportedWekaVersion {
					return false, fmt.Errorf("device %s (%s) requires Weka version %s or later (current: %s)", ni.Name, deviceModel, devInfo.MinSupportedWekaVersion, wekaVersion)
				}
				if devInfo.MaxSupportedWekaVersion != "" && wekaVersion > devInfo.MaxSupportedWekaVersion {
					return false, fmt.Errorf("device %s (%s) is no longer supported after Weka version %s (current: %s)", ni.Name, deviceModel, devInfo.MaxSupportedWekaVersion, wekaVersion)
				}
			}
		}
	}

	return true, nil
}

// IsSupportedByWekaForLacpSameCard checks if LACP bonding is supported using 2 ports on the same NIC.
// Returns (true, nil) if supported, or (false, error) with detailed error message.
// For bonds, returns true only if ALL slaves support LACP same card.
// For VLANs, delegates to the parent interface.
// Optionally validates against a Weka version if forWekaVersion is specified.
// Currently only certain high-performance devices support this
func (ni *NetworkInterface) IsSupportedByWekaForLacpSameCard(forWekaVersion ...string) (bool, error) {
	if ni == nil {
		return false, fmt.Errorf("interface is nil")
	}

	// For VLAN interfaces, check the parent interface
	if ni.IsVlan() {
		parent := ni.GetParent()
		if parent == nil {
			return false, fmt.Errorf("VLAN %s has no parent interface", ni.Name)
		}
		supported, err := parent.IsSupportedByWekaForLacpSameCard(forWekaVersion...)
		if !supported {
			return false, fmt.Errorf("VLAN %s (on %s): %w", ni.Name, parent.Name, err)
		}
		return true, nil
	}

	// For bonds, all slaves must support the feature AND be on same card
	if ni.IsBond() {
		slaves := ni.GetSlaves()
		if len(slaves) == 0 {
			return false, fmt.Errorf("bond has no slaves")
		}

		// Check if all slaves are on the same NIC card
		if !ni.IsLacpOnSameCard() {
			return false, fmt.Errorf("bond slaves are located on different NIC cards (not supported for same-card LACP)")
		}

		for i, slave := range slaves {
			supported, err := slave.IsSupportedByWekaForLacpSameCard(forWekaVersion...)
			if !supported {
				return false, fmt.Errorf("slave %d (%s) does not support LACP same card: %w", i, slave.Name, err)
			}
		}
		return true, nil
	}

	var wekaVersion string
	if len(forWekaVersion) > 0 {
		wekaVersion = forWekaVersion[0]
	}

	if ni.VendorModel == "" {
		return false, fmt.Errorf("unknown network device (no VendorModel)")
	}

	caps := ds.GetNICCapabilities(ni.VendorModel)
	if caps == nil {
		return false, fmt.Errorf("device %s not found in NIC registry", ni.VendorModel)
	}

	if !caps.SupportedByWekaForLacpSameCard {
		// Try to get device model for better error message
		devInfo := ds.GetNICInfo(ni.VendorModel)
		deviceModel := ni.VendorModel
		if devInfo != nil && devInfo.Model != "" {
			deviceModel = devInfo.Model
		}
		return false, fmt.Errorf("device %s (%s) does not support LACP on same card", ni.Name, deviceModel)
	}

	// Check version constraints if a Weka version was provided
	if wekaVersion != "" {
		devInfo := ds.GetNICInfo(ni.VendorModel)
		if devInfo != nil {
			deviceModel := ni.VendorModel
			if devInfo.Model != "" {
				deviceModel = devInfo.Model
			}

			if devInfo.MinSupportedWekaVersion != "" && !ds.IsVersionInRange(wekaVersion, devInfo.MinSupportedWekaVersion, devInfo.MaxSupportedWekaVersion) {
				if devInfo.MinSupportedWekaVersion != "" && wekaVersion < devInfo.MinSupportedWekaVersion {
					return false, fmt.Errorf("device %s (%s) requires Weka version %s or later (current: %s)", ni.Name, deviceModel, devInfo.MinSupportedWekaVersion, wekaVersion)
				}
				if devInfo.MaxSupportedWekaVersion != "" && wekaVersion > devInfo.MaxSupportedWekaVersion {
					return false, fmt.Errorf("device %s (%s) is no longer supported after Weka version %s (current: %s)", ni.Name, deviceModel, devInfo.MaxSupportedWekaVersion, wekaVersion)
				}
			}
		}
	}

	return true, nil
}

// IsSupportedByWekaForLacpDiffCards checks if LACP bonding is supported across different NIC cards.
// Returns (true, nil) if supported, or (false, error) with detailed error message.
// For bonds, returns true only if ALL slaves support LACP different cards.
// For VLANs, delegates to the parent interface.
// Optionally validates against a Weka version if forWekaVersion is specified.
// Currently not supported for any devices
func (ni *NetworkInterface) IsSupportedByWekaForLacpDiffCards(forWekaVersion ...string) (bool, error) {
	if ni == nil {
		return false, fmt.Errorf("interface is nil")
	}

	// For VLAN interfaces, check the parent interface
	if ni.IsVlan() {
		parent := ni.GetParent()
		if parent == nil {
			return false, fmt.Errorf("VLAN %s has no parent interface", ni.Name)
		}
		supported, err := parent.IsSupportedByWekaForLacpDiffCards(forWekaVersion...)
		if !supported {
			return false, fmt.Errorf("VLAN %s (on %s): %w", ni.Name, parent.Name, err)
		}
		return true, nil
	}

	// For bonds, all slaves must support the feature
	if ni.IsBond() {
		slaves := ni.GetSlaves()
		if len(slaves) == 0 {
			return false, fmt.Errorf("bond has no slaves")
		}

		for i, slave := range slaves {
			supported, err := slave.IsSupportedByWekaForLacpDiffCards(forWekaVersion...)
			if !supported {
				return false, fmt.Errorf("slave %d (%s) does not support LACP different cards: %w", i, slave.Name, err)
			}
		}
		return true, nil
	}

	var wekaVersion string
	if len(forWekaVersion) > 0 {
		wekaVersion = forWekaVersion[0]
	}

	if ni.VendorModel == "" {
		return false, fmt.Errorf("unknown network device (no VendorModel)")
	}

	caps := ds.GetNICCapabilities(ni.VendorModel)
	if caps == nil {
		return false, fmt.Errorf("device %s not found in NIC registry", ni.VendorModel)
	}

	if !caps.SupportedByWekaForLacpDiffCards {
		// Try to get device model for better error message
		devInfo := ds.GetNICInfo(ni.VendorModel)
		deviceModel := ni.VendorModel
		if devInfo != nil && devInfo.Model != "" {
			deviceModel = devInfo.Model
		}
		return false, fmt.Errorf("device %s (%s) does not support LACP across different cards", ni.Name, deviceModel)
	}

	// Check version constraints if a Weka version was provided
	if wekaVersion != "" {
		devInfo := ds.GetNICInfo(ni.VendorModel)
		if devInfo != nil {
			deviceModel := ni.VendorModel
			if devInfo.Model != "" {
				deviceModel = devInfo.Model
			}

			if devInfo.MinSupportedWekaVersion != "" && !ds.IsVersionInRange(wekaVersion, devInfo.MinSupportedWekaVersion, devInfo.MaxSupportedWekaVersion) {
				if devInfo.MinSupportedWekaVersion != "" && wekaVersion < devInfo.MinSupportedWekaVersion {
					return false, fmt.Errorf("device %s (%s) requires Weka version %s or later (current: %s)", ni.Name, deviceModel, devInfo.MinSupportedWekaVersion, wekaVersion)
				}
				if devInfo.MaxSupportedWekaVersion != "" && wekaVersion > devInfo.MaxSupportedWekaVersion {
					return false, fmt.Errorf("device %s (%s) is no longer supported after Weka version %s (current: %s)", ni.Name, deviceModel, devInfo.MaxSupportedWekaVersion, wekaVersion)
				}
			}
		}
	}

	return true, nil
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

// InitializeParents sets the parent pointer for all interfaces in the slice
// This must be called after the NetworkInterfaces slice is populated
// Allows GetSlaves() and GetMaster() methods to work properly
func (ni *NetworkInterfaces) InitializeParents() {
	if ni == nil {
		return
	}
	for i := range *ni {
		(*ni)[i].parent = ni
	}
}

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

// InitializeBondHierarchy initializes the parent pointers for all network interfaces
// This must be called after unmarshalling JSON to enable GetSlaves() and GetMaster() methods
// Typically called right after json.Unmarshal()
func (hc *HostChecksResult) InitializeBondHierarchy() {
	if hc != nil {
		hc.NetworkInterfaces.InitializeParents()
	}
}

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
