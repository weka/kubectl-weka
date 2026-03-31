package wekaconfig

// isValidEthDeviceName checks if the network interface name is reasonable
func isValidEthDeviceName(name string) bool {
	if name == "" {
		return false
	}

	// Allow alphanumeric, underscore, hyphen, and dot (for VLAN)
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '-' || char == '.') {
			return false
		}
	}

	return true
}
