package device_support

import (
	"fmt"
	"strings"
)

// GetNICInfo retrieves NIC device information from the registry
func GetNICInfo(vendorModel string) *NICInfo {
	if vendorModel == "" {
		return nil
	}
	return NICRegistry[strings.ToLower(vendorModel)]
}

// GetNICCapabilities retrieves capabilities for a NIC device
func GetNICCapabilities(vendorModel string) *NICCapabilities {
	if vendorModel == "" {
		return nil
	}
	caps := NICCapabilityMap[strings.ToLower(vendorModel)]
	if caps != nil {
		return caps
	}
	// Return default capabilities (no support)
	return &NICCapabilities{
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              false,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	}
}

// NICRegistry maintains all known NIC devices
var NICRegistry = map[string]*NICInfo{
	// Mellanox InfiniHost III Ex devices
	"15b3:6278": {
		VendorID:                "15b3",
		DeviceID:                "6278",
		Vendor:                  "Mellanox",
		ShortModel:              "InfiniHost III Ex",
		Model:                   "MT25208 InfiniHost III Ex HCA",
		Speed:                   "SDR/DDR",
		NumPorts:                1,
		InterfaceMode:           "InfiniBand",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"15b3:6282": {
		VendorID:                "15b3",
		DeviceID:                "6282",
		Vendor:                  "Mellanox",
		ShortModel:              "InfiniHost III Ex",
		Model:                   "MT25208 InfiniHost III Ex",
		Speed:                   "SDR/DDR",
		NumPorts:                1,
		InterfaceMode:           "InfiniBand",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-1 (First VPI - Converged)
	"15b3:6340": {
		VendorID:                "15b3",
		DeviceID:                "6340",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-1",
		Model:                   "MT25408 ConnectX VPI",
		Speed:                   "10GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-2 EN (Ethernet only)
	"15b3:6368": {
		VendorID:                "15b3",
		DeviceID:                "6368",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-2 EN",
		Model:                   "MT25448 ConnectX EN",
		Speed:                   "10GbE",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-2 VPI (Converged)
	"15b3:673c": {
		VendorID:                "15b3",
		DeviceID:                "673c",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-2 VPI",
		Model:                   "MT26428 ConnectX-2 VPI",
		Speed:                   "10GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-3 (FDR10 with 40GbE)
	"15b3:1003": {
		VendorID:                "15b3",
		DeviceID:                "1003",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-3",
		Model:                   "MT27500 ConnectX-3",
		Speed:                   "40GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-3 Pro (FDR with 56GbE)
	"15b3:1007": {
		VendorID:                "15b3",
		DeviceID:                "1007",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-3 Pro",
		Model:                   "MT27520 ConnectX-3 Pro",
		Speed:                   "56GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-4 (EDR with 100GbE)
	"15b3:1013": {
		VendorID:                "15b3",
		DeviceID:                "1013",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-4",
		Model:                   "MT27700 ConnectX-4",
		Speed:                   "100GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-4 Lx (Ethernet only, 25/50GbE)
	"15b3:1015": {
		VendorID:                "15b3",
		DeviceID:                "1015",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-4 Lx",
		Model:                   "MT27710 ConnectX-4 Lx",
		Speed:                   "50GbE",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-5 (HDR100 with 100GbE)
	"15b3:1016": {
		VendorID:                "15b3",
		DeviceID:                "1016",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-5",
		Model:                   "MT27800 ConnectX-5",
		Speed:                   "100GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-5 Ex (HDR100 with 100GbE)
	"15b3:1019": {
		VendorID:                "15b3",
		DeviceID:                "1019",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-5 Ex",
		Model:                   "MT28800 ConnectX-5 Ex",
		Speed:                   "100GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-6 (HDR200 with 200GbE)
	"15b3:101b": {
		VendorID:                "15b3",
		DeviceID:                "101b",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-6",
		Model:                   "MT28908 ConnectX-6",
		Speed:                   "200GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-6 Dx (HDR200 with 200GbE)
	"15b3:101d": {
		VendorID:                "15b3",
		DeviceID:                "101d",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-6 Dx",
		Model:                   "MT2892 ConnectX-6 Dx",
		Speed:                   "200GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-6 Lx (Ethernet only, 25/50GbE)
	"15b3:101e": {
		VendorID:                "15b3",
		DeviceID:                "101e",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-6 Lx",
		Model:                   "MT2894 ConnectX-6 Lx",
		Speed:                   "50GbE",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-7 (NDR400 with 400GbE)
	"15b3:1021": {
		VendorID:                "15b3",
		DeviceID:                "1021",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-7",
		Model:                   "MT2910 ConnectX-7",
		Speed:                   "400GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox ConnectX-7 Lx (Ethernet only, 200GbE)
	"15b3:1025": {
		VendorID:                "15b3",
		DeviceID:                "1025",
		Vendor:                  "Mellanox",
		ShortModel:              "ConnectX-7 Lx",
		Model:                   "MT2912 ConnectX-7 Lx",
		Speed:                   "200GbE",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox BlueField-1 (EDR IB with 100GbE)
	"15b3:a2d2": {
		VendorID:                "15b3",
		DeviceID:                "a2d2",
		Vendor:                  "Mellanox",
		ShortModel:              "BlueField-1",
		Model:                   "MT41682 BlueField SmartNIC",
		Speed:                   "100GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox BlueField-2 (HDR200 IB with 200GbE)
	"15b3:a2d6": {
		VendorID:                "15b3",
		DeviceID:                "a2d6",
		Vendor:                  "Mellanox",
		ShortModel:              "BlueField-2",
		Model:                   "MT42822 BlueField-2 DPU",
		Speed:                   "200GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox BlueField-3 (NDR400 IB with 400GbE)
	"15b3:a2dc": {
		VendorID:                "15b3",
		DeviceID:                "a2dc",
		Vendor:                  "Mellanox",
		ShortModel:              "BlueField-3",
		Model:                   "MT43244 BlueField-3 DPU",
		Speed:                   "400GbE",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Intel Network adapters (DPDK capable)
	"8086:0d58": {
		VendorID:                "8086",
		DeviceID:                "0d58",
		Vendor:                  "Intel",
		ShortModel:              "82599ES",
		Model:                   "82599ES 10-Gigabit SFI/SFP+ Network Connection",
		Speed:                   "10Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:1521": {
		VendorID:                "8086",
		DeviceID:                "1521",
		Vendor:                  "Intel",
		ShortModel:              "I350",
		Model:                   "I350 Gigabit Network Connection",
		Speed:                   "1Gb/s",
		NumPorts:                4,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:1572": {
		VendorID:                "8086",
		DeviceID:                "1572",
		Vendor:                  "Intel",
		ShortModel:              "X540",
		Model:                   "Ethernet Controller X540",
		Speed:                   "10Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:1583": {
		VendorID:                "8086",
		DeviceID:                "1583",
		Vendor:                  "Intel",
		ShortModel:              "X550",
		Model:                   "Ethernet Controller X550",
		Speed:                   "10Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:1589": {
		VendorID:                "8086",
		DeviceID:                "1589",
		Vendor:                  "Intel",
		ShortModel:              "X550-VF",
		Model:                   "Ethernet Controller X550 [Virtual Function]",
		Speed:                   "10Gb/s",
		NumPorts:                1,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:0aad": {
		VendorID:                "8086",
		DeviceID:                "0aad",
		Vendor:                  "Intel",
		ShortModel:              "XL710",
		Model:                   "Ethernet Controller X710 for 10GbE SFP+",
		Speed:                   "10Gb/s",
		NumPorts:                4,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:1593": {
		VendorID:                "8086",
		DeviceID:                "1593",
		Vendor:                  "Intel",
		ShortModel:              "E810",
		Model:                   "Ethernet Controller E810-XXVDA4",
		Speed:                   "100Gb/s",
		NumPorts:                4,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Broadcom NetXtreme adapters (some DPDK capable)
	"14e4:164c": {
		VendorID:                "14e4",
		DeviceID:                "164c",
		Vendor:                  "Broadcom",
		ShortModel:              "NetXtreme II BCM57810",
		Model:                   "NetXtreme II BCM57810 10 Gigabit Ethernet",
		Speed:                   "10Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"14e4:168a": {
		VendorID:                "14e4",
		DeviceID:                "168a",
		Vendor:                  "Broadcom",
		ShortModel:              "NetXtreme II BCM57830",
		Model:                   "NetXtreme II BCM57840 NetXtremeE [10GigE]",
		Speed:                   "10Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Hypervisor/Virtualization NICs
	"1af4:1000": {
		VendorID:                "1af4",
		DeviceID:                "1000",
		Vendor:                  "Red Hat",
		ShortModel:              "virtio-net",
		Model:                   "Virtio network device",
		Speed:                   "10Gb/s",
		NumPorts:                1,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"15ad:0720": {
		VendorID:                "15ad",
		DeviceID:                "0720",
		Vendor:                  "VMware",
		ShortModel:              "vmxnet3",
		Model:                   "VMXNET3 virtual adapter",
		Speed:                   "10Gb/s",
		NumPorts:                1,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"1d0f:ec20": {
		VendorID:                "1d0f",
		DeviceID:                "ec20",
		Vendor:                  "Amazon",
		ShortModel:              "ENA",
		Model:                   "Elastic Network Adapter (ENA)",
		Speed:                   "25Gb/s",
		NumPorts:                1,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"18ca:00e5": {
		VendorID:                "18ca",
		DeviceID:                "00e5",
		Vendor:                  "Google",
		ShortModel:              "gVNIC",
		Model:                   "Google Virtual NIC",
		Speed:                   "50Gb/s",
		NumPorts:                1,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// ========================================================================
	// TEST DEVICES - Comprehensive capability coverage for unit tests
	// ========================================================================
	// These devices are used for testing all combinations of capabilities

	// Test Device 1: UDP=T, DPDK=T (SingleNic), SameCard=T, DiffCard=T
	"ffff:0001": {
		VendorID:                "ffff",
		DeviceID:                "0001",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-FullSupport",
		Model:                   "Test Device: Full Support (UDP, DPDK SingleNic, LACP)",
		Speed:                   "100Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 2: UDP=T, DPDK=T (SingleNic), SameCard=T, DiffCard=F
	"ffff:0002": {
		VendorID:                "ffff",
		DeviceID:                "0002",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-SingleNicSameCard",
		Model:                   "Test Device: SingleNic+SameCard (no DiffCard)",
		Speed:                   "50Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 3: UDP=T, DPDK=T (SingleNic), SameCard=F
	"ffff:0003": {
		VendorID:                "ffff",
		DeviceID:                "0003",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-SingleNicNoLACP",
		Model:                   "Test Device: SingleNic no LACP",
		Speed:                   "25Gb/s",
		NumPorts:                1,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 4: UDP=T, DPDK=T (PerProcess), SameCard=T, DiffCard=T
	"ffff:0004": {
		VendorID:                "ffff",
		DeviceID:                "0004",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-PerProcessLACP",
		Model:                   "Test Device: PerProcess DPDK with LACP",
		Speed:                   "100Gb/s",
		NumPorts:                2,
		InterfaceMode:           "InfiniBand",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 5: UDP=T, DPDK=T (PerProcess), SameCard=T, DiffCard=F
	"ffff:0005": {
		VendorID:                "ffff",
		DeviceID:                "0005",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-PerProcessSameCard",
		Model:                   "Test Device: PerProcess SameCard (no DiffCard)",
		Speed:                   "50Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Converged",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 6: UDP=T, DPDK=T (PerProcess), SameCard=F
	"ffff:0006": {
		VendorID:                "ffff",
		DeviceID:                "0006",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-PerProcessNoLACP",
		Model:                   "Test Device: PerProcess no LACP",
		Speed:                   "25Gb/s",
		NumPorts:                1,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 7: UDP=T, DPDK=F, SameCard=T, DiffCard=T
	"ffff:0007": {
		VendorID:                "ffff",
		DeviceID:                "0007",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-UDPOnlyLACP",
		Model:                   "Test Device: UDP only with LACP",
		Speed:                   "10Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 8: UDP=T, DPDK=F, SameCard=T, DiffCard=F
	"ffff:0008": {
		VendorID:                "ffff",
		DeviceID:                "0008",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-UDPSameCard",
		Model:                   "Test Device: UDP SameCard",
		Speed:                   "10Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 9: UDP=T, DPDK=F, SameCard=F
	"ffff:0009": {
		VendorID:                "ffff",
		DeviceID:                "0009",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-UDPOnly",
		Model:                   "Test Device: UDP only no LACP",
		Speed:                   "10Gb/s",
		NumPorts:                1,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 10: UDP=T, DPDK=F, SameCard=T, DiffCard=T - Already defined above

	// Test Device 11: UDP=F, DPDK=F, SameCard=T, DiffCard=T
	"ffff:000a": {
		VendorID:                "ffff",
		DeviceID:                "000a",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-NoSupport",
		Model:                   "Test Device: No Weka Support (LACP only)",
		Speed:                   "1Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 12: UDP=F, DPDK=F, SameCard=T, DiffCard=F
	"ffff:000b": {
		VendorID:                "ffff",
		DeviceID:                "000b",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-NoSupportSameCard",
		Model:                   "Test Device: No Support SameCard",
		Speed:                   "1Gb/s",
		NumPorts:                2,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Test Device 13: UDP=F, DPDK=F, SameCard=F
	"ffff:000c": {
		VendorID:                "ffff",
		DeviceID:                "000c",
		Vendor:                  "TestVendor",
		ShortModel:              "TestNIC-Unsupported",
		Model:                   "Test Device: Completely Unsupported",
		Speed:                   "1Gb/s",
		NumPorts:                1,
		InterfaceMode:           "Ethernet",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
}

// NICCapabilityMap maps vendor:device IDs to their capabilities
var NICCapabilityMap = map[string]*NICCapabilities{
	// Mellanox devices - full support (DPDK, single NIC sharing, LACP same card)
	"15b3:1021": { // ConnectX-7
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15b3:1023": { // ConnectX-8
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15b3:101d": { // ConnectX-6 Dx
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15b3:1019": { // ConnectX-6
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15b3:101f": { // ConnectX-6 Lite
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15b3:1017": { // ConnectX-5
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15b3:1015": { // ConnectX-4
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15b3:1013": { // ConnectX-3
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15b3:0211": { // BlueField-2
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15b3:0212": { // BlueField-3
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Intel devices - DPDK capable but need dedicated NIC per process
	"8086:0d58": { // 82599ES
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              false,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"8086:1521": { // I350
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              false,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"8086:1572": { // X540
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"8086:1583": { // X550
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"8086:1589": { // X550-VF
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"8086:0aad": { // XL710
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"8086:1593": { // E810
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Broadcom devices - some DPDK capable (needs verification per model)
	"14e4:164c": { // NetXtreme II BCM57810
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              false,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"14e4:168a": { // NetXtreme II BCM57830
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Hypervisor/Virtualization NICs - some DPDK capable
	"1af4:1000": { // Virtio-net
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"15ad:0720": { // VMware vmxnet3
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"1d0f:ec20": { // Amazon ENA
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
	"0x18ca:00e5": { // Google gVNIC
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},

	// ========================================================================
	// TEST DEVICES - Capability Matrix (18 devices covering all combinations)
	// ========================================================================

	// Device 1: UDP=T, DPDK=T (SingleNic), SameCard=T, DiffCard=T
	"ffff:0001": {
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: true,
	},

	// Device 2: UDP=T, DPDK=T (SingleNic), SameCard=T, DiffCard=F
	"ffff:0002": {
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Device 3: UDP=T, DPDK=T (SingleNic), SameCard=F
	"ffff:0003": {
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    true,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Device 4: UDP=T, DPDK=T (PerProcess), SameCard=T, DiffCard=T
	"ffff:0004": {
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: true,
	},

	// Device 5: UDP=T, DPDK=T (PerProcess), SameCard=T, DiffCard=F
	"ffff:0005": {
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Device 6: UDP=T, DPDK=T (PerProcess), SameCard=F
	"ffff:0006": {
		SupportedByWekaDpdk:             true,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Device 7: UDP=T, DPDK=F, SameCard=T, DiffCard=T
	"ffff:0007": {
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: true,
	},

	// Device 8: UDP=T, DPDK=F, SameCard=T, DiffCard=F
	"ffff:0008": {
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Device 9: UDP=T, DPDK=F, SameCard=F
	"ffff:0009": {
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              true,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Device 10: UDP=F, DPDK=F, SameCard=T, DiffCard=T
	"ffff:000a": {
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              false,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: true,
	},

	// Device 11: UDP=F, DPDK=F, SameCard=T, DiffCard=F
	"ffff:000b": {
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              false,
		SupportedByWekaForLacpSameCard:  true,
		SupportedByWekaForLacpDiffCards: false,
	},

	// Device 12: UDP=F, DPDK=F, SameCard=F
	"ffff:000c": {
		SupportedByWekaDpdk:             false,
		SupportedByWekaDpdkSingleNic:    false,
		SupportedByWekaUdp:              false,
		SupportedByWekaForLacpSameCard:  false,
		SupportedByWekaForLacpDiffCards: false,
	},
}

// SpeedMbpsToDisplayString converts a speed value in Mbps to a human-readable display string
// For Ethernet interfaces, returns format like "100Gbps", "400Gbps"
// For InfiniBand interfaces, returns format like "400GB/s 2xNDR"
// interfaceType should be "ethernet" or "infiniband"
func SpeedMbpsToDisplayString(speedMbps int, interfaceType string) string {
	if speedMbps <= 0 {
		return "unknown"
	}

	if strings.ToLower(interfaceType) == "infiniband" {
		// InfiniBand uses GB/s (bytes/sec) which is 1/8 of Gbps
		// Also add the generation suffix
		gbps := speedMbps
		gbs := gbps / 8

		// Map speeds to InfiniBand generations
		var suffix string
		switch gbps {
		case 2000: // SDR
			suffix = "2xSDR"
		case 4000: // DDR
			suffix = "2xDDR"
		case 10000: // QDR (approx)
			suffix = "2xQDR"
		case 20000: // FDR (approx)
			suffix = "2xFDR"
		case 40000: // EDR (approx)
			suffix = "2xEDR"
		case 100000: // HDR (approx)
			suffix = "2xHDR"
		case 200000: // NDR (approx)
			suffix = "2xNDR"
		case 400000: // XDR (approx)
			suffix = "2xXDR"
		default:
			// Generic format if we don't know the generation
			return strings.TrimSpace(fmt.Sprintf("%dGB/s", gbs))
		}

		return fmt.Sprintf("%dGB/s %s", gbs, suffix)
	}

	// Default: Ethernet format (Gbps, not GB/s)
	if speedMbps >= 1000 {
		gbps := speedMbps / 1000
		return fmt.Sprintf("%dGbps", gbps)
	}

	// For speeds less than 1Gbps
	return fmt.Sprintf("%dMbps", speedMbps)
}

// RateMBsToDisplayString converts an InfiniBand rate in MB/s to a human-readable display string
// Format includes the generation suffix: "100GB/s 2xHDR", "400GB/s 2xNDR", etc.
// Only used for InfiniBand interfaces
func RateMBsToDisplayString(rateMBs int, interfaceType string) string {
	if rateMBs <= 0 {
		return "unknown"
	}

	if strings.ToLower(interfaceType) != "infiniband" {
		// Not an InfiniBand interface
		return "unknown"
	}

	// Convert MB/s to Gbps to determine generation
	// MB/s * 8 bits/byte / 1000 = Gbps
	gbps := rateMBs * 8 / 1000
	if gbps*1000 != rateMBs*8 {
		// Not a clean conversion, use raw format
		return fmt.Sprintf("%dGB/s", rateMBs/1000)
	}

	// Map speeds to InfiniBand generations based on the effective Gbps
	var suffix string
	switch gbps {
	case 2, 4, 8: // SDR/DDR (single or dual port approximations)
		suffix = "2xSDR"
	case 10, 20: // QDR/FDR approximations
		suffix = "2xQDR"
	case 40: // EDR
		suffix = "2xEDR"
	case 100: // HDR
		suffix = "2xHDR"
	case 200: // NDR
		suffix = "2xNDR"
	case 400: // XDR
		suffix = "2xXDR"
	default:
		// For 800 (25Gb/s per port), 1600, etc., use the rate directly
		if gbps > 400 {
			suffix = "2xXDR+" // Beyond XDR
		} else {
			// For other values, calculate based on Gbps
			suffix = fmt.Sprintf("2x%dGbps", gbps)
		}
	}

	gbs := rateMBs / 1000
	return fmt.Sprintf("%dGB/s %s", gbs, suffix)
}

// ParseSpeedStringToMbps converts a speed string to Mbps integer value
// Supports formats: "100Gbps", "100GB/s", "100Mb/s", "100Gbit/s", etc.
// Returns 0 if the string cannot be parsed
func ParseSpeedStringToMbps(speedStr string) int {
	if speedStr == "" || strings.ToLower(speedStr) == "unknown" {
		return 0
	}

	// Normalize the string
	speedStr = strings.TrimSpace(speedStr)

	// Try different patterns
	var value int
	var unit string

	// Parse "400Gbps" or "400 Gbps"
	if _, err := fmt.Sscanf(speedStr, "%dGbps", &value); err == nil {
		return value * 1000
	}
	if _, err := fmt.Sscanf(speedStr, "%d Gbps", &value); err == nil {
		return value * 1000
	}

	// Parse "400GB/s" or "400 GB/s"
	if _, err := fmt.Sscanf(speedStr, "%dGB/s", &value); err == nil {
		return value * 8 * 1000 // GB/s to Mbps: * 8 (bytes to bits) * 1000 (GB to Mb)
	}
	if _, err := fmt.Sscanf(speedStr, "%d GB/s", &value); err == nil {
		return value * 8 * 1000
	}

	// Parse "400Gbit/s" or "400 Gbit/s"
	if _, err := fmt.Sscanf(speedStr, "%dGbit/s", &value); err == nil {
		return value * 1000
	}
	if _, err := fmt.Sscanf(speedStr, "%d Gbit/s", &value); err == nil {
		return value * 1000
	}

	// Parse "400Mb/s" or "400 Mb/s"
	if _, err := fmt.Sscanf(speedStr, "%dMb/s", &value); err == nil {
		return value
	}
	if _, err := fmt.Sscanf(speedStr, "%d Mb/s", &value); err == nil {
		return value
	}

	// Parse "400Mbps" or "400 Mbps"
	if _, err := fmt.Sscanf(speedStr, "%dMbps", &value); err == nil {
		return value
	}
	if _, err := fmt.Sscanf(speedStr, "%d Mbps", &value); err == nil {
		return value
	}

	// Parse "400Gbit" or "400 Gbit"
	if _, err := fmt.Sscanf(speedStr, "%dGbit", &value); err == nil {
		return value * 1000
	}
	if _, err := fmt.Sscanf(speedStr, "%d Gbit", &value); err == nil {
		return value * 1000
	}

	// Parse "400Gb/sec" or "400 Gb/sec" (InfiniBand format)
	if _, err := fmt.Sscanf(speedStr, "%dGb/sec", &value); err == nil {
		return value * 1000
	}
	if _, err := fmt.Sscanf(speedStr, "%d Gb/sec", &value); err == nil {
		return value * 1000
	}

	// Parse just a number followed by unit
	parts := strings.Fields(speedStr)
	if len(parts) >= 1 {
		if n, err := fmt.Sscanf(parts[0], "%d", &value); err == nil && n == 1 {
			if len(parts) >= 2 {
				unit = strings.ToLower(parts[1])
			}
			if strings.HasPrefix(unit, "g") {
				return value * 1000
			} else if strings.HasPrefix(unit, "m") {
				return value
			}
		}
	}

	return 0
}
