package device_support

import (
	"strings"
)

// GetNVMeInfo retrieves NVMe device information from the registry
func GetNVMeInfo(vendorModel string) *NVMeInfo {
	if vendorModel == "" {
		return nil
	}
	return NVMeRegistry[strings.ToLower(vendorModel)]
}

// NVMeRegistry maintains all known NVMe drive devices
var NVMeRegistry = map[string]*NVMeInfo{
	// Intel NVMe drives
	"8086:0953": {
		VendorID:                "8086",
		DeviceID:                "0953",
		Vendor:                  "Intel",
		ShortModel:              "DC P3320",
		Model:                   "Intel DC P3320 Series NVMe SSD",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:0a54": {
		VendorID:                "8086",
		DeviceID:                "0a54",
		Vendor:                  "Intel",
		ShortModel:              "DC P3700",
		Model:                   "Intel DC P3700 Series NVMe SSD",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:0a55": {
		VendorID:                "8086",
		DeviceID:                "0a55",
		Vendor:                  "Intel",
		ShortModel:              "DC P3500",
		Model:                   "Intel DC P3500 Series NVMe SSD",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:f1a5": {
		VendorID:                "8086",
		DeviceID:                "f1a5",
		Vendor:                  "Intel",
		ShortModel:              "Optane SSD DC P4800X",
		Model:                   "Intel Optane SSD DC P4800X",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"8086:2700": {
		VendorID:                "8086",
		DeviceID:                "2700",
		Vendor:                  "Intel",
		ShortModel:              "E5-1600/E5-2600 NVMe Controller",
		Model:                   "Intel E5-1600/E5-2600 NVMe Controller",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Samsung NVMe drives
	"144d:a808": {
		VendorID:                "144d",
		DeviceID:                "a808",
		Vendor:                  "Samsung",
		ShortModel:              "SM961/PM961",
		Model:                   "Samsung SM961/PM961 NVMe SSD",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"144d:a801": {
		VendorID:                "144d",
		DeviceID:                "a801",
		Vendor:                  "Samsung",
		ShortModel:              "PM951",
		Model:                   "Samsung PM951 NVMe SSD",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},

	// Mellanox/Nvidia NVMe (BlueField integrated storage)
	"15b3:0211": {
		VendorID:                "15b3",
		DeviceID:                "0211",
		Vendor:                  "Nvidia Mellanox",
		ShortModel:              "BlueField-2 eMMC",
		Model:                   "Nvidia Mellanox BlueField-2 Integrated Storage",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
	"15b3:0212": {
		VendorID:                "15b3",
		DeviceID:                "0212",
		Vendor:                  "Nvidia Mellanox",
		ShortModel:              "BlueField-3 eMMC",
		Model:                   "Nvidia Mellanox BlueField-3 Integrated Storage",
		MinSupportedWekaVersion: "",
		MaxSupportedWekaVersion: "",
	},
}
