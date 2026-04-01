package helm

import (
	"fmt"
	"os"
	"strings"
)

// CreateOverrideValuesFile writes a YAML values file with only the overridden values
// This file can be used with `helm install/upgrade -f values-override.yaml`
func CreateOverrideValuesFile(values map[string]interface{}, outputPath string) error {
	if len(values) == 0 {
		// Nothing to override
		return nil
	}

	// Convert to YAML
	yamlData, err := marshalToYAML(values)
	if err != nil {
		return fmt.Errorf("marshal values to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, yamlData, 0644); err != nil {
		return fmt.Errorf("write override values file: %w", err)
	}

	return nil
}

// GetNestedValue extracts a value from a nested map using dot notation
// Example: "csi.image" will navigate ch.Values["csi"]["image"]
func GetNestedValue(values map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := interface{}(values)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return ""
			}
		default:
			return ""
		}
	}

	// Try to convert final value to string
	if str, ok := current.(string); ok {
		return str
	}
	return ""
}

// SetNestedValue sets a value in a nested map using dot notation
// Creates intermediate maps as needed
// Example: "csi.image" with value "newimage:tag" creates updatedValues["csi"]["image"] = "newimage:tag"
func SetNestedValue(values map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := values

	// Navigate/create path to final key
	for i := 0; i < len(parts)-1; i++ {
		key := parts[i]
		if _, exists := current[key]; !exists {
			current[key] = make(map[string]interface{})
		}

		// Move to next level
		if nextMap, ok := current[key].(map[string]interface{}); ok {
			current = nextMap
		} else {
			// Path is blocked by non-map value, can't proceed
			return
		}
	}

	// Set final value
	if len(parts) > 0 {
		current[parts[len(parts)-1]] = value
	}
}
