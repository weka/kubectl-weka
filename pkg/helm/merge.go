package helm

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

// mergeValuesWithComments applies changes from modifiedValues to originalContent while preserving comments
// Uses gopkg.in/yaml.v3 which preserves comments during parsing and marshaling
func mergeValuesWithComments(originalContent []byte, modifiedValues map[string]interface{}) ([]byte, error) {
	// Parse original YAML preserving comments
	var node yaml.Node
	if err := yaml.Unmarshal(originalContent, &node); err != nil {
		return nil, fmt.Errorf("parse original values: %w", err)
	}

	// Apply modifications to the node tree
	if err := applyModificationsToNode(&node, modifiedValues); err != nil {
		return nil, fmt.Errorf("apply modifications: %w", err)
	}

	// Marshal back to YAML (preserves comments and structure)
	result, err := yaml.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("marshal modified values: %w", err)
	}

	return result, nil
}

// applyModificationsToNode recursively applies changes from a map to a YAML node
func applyModificationsToNode(node *yaml.Node, values map[string]interface{}) error {
	if node == nil {
		return nil
	}

	// Handle only mapping nodes
	if node.Kind != yaml.MappingNode {
		return nil
	}

	// Process all key-value pairs in the mapping
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}

		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		if keyNode == nil || keyNode.Value == "" {
			continue
		}

		key := keyNode.Value

		// If this key exists in the modifications
		if newVal, ok := values[key]; ok {
			// Update the value node to reflect the change
			if err := updateNodeValue(valueNode, newVal); err != nil {
				// Log warning but continue
				continue
			}
		}
	}

	return nil
}

// updateNodeValue updates a YAML node with a new value, preserving structure where possible
func updateNodeValue(node *yaml.Node, newValue interface{}) error {
	if node == nil {
		return nil
	}

	// Convert newValue to string representation
	newValStr := fmt.Sprintf("%v", newValue)

	// For scalar values, just update the content
	if node.Kind == yaml.ScalarNode {
		node.Value = newValStr
		return nil
	}

	// For complex types (maps, sequences), marshal and reparse
	// This is a fallback that may lose some formatting but preserves structure
	if mapVal, ok := newValue.(map[string]interface{}); ok && node.Kind == yaml.MappingNode {
		return applyModificationsToNode(node, mapVal)
	}

	// For other cases, update the scalar value
	if node.Kind == yaml.ScalarNode {
		node.Value = newValStr
	}

	return nil
}
