package kubernetes

import (
	"fmt"
	"github.com/weka/kubectl-weka/pkg/utils"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

// QuantityOrZero returns the quantity value or zero if not found
func QuantityOrZero(resourceList v1.ResourceList, resourceName v1.ResourceName) resource.Quantity {
	val, ok := resourceList[resourceName]
	if !ok {
		return resource.Quantity{}
	}
	return val
}

// GetNamespaceFromFlags centralizes logic for namespace selection based on flags.
// Returns: namespace string, allNamespaces bool, error
func GetNamespaceFromFlags(allNamespaces bool, namespace string) (string, bool, error) {
	if allNamespaces {
		return "", true, nil
	}
	if namespace != "" {
		return namespace, false, nil
	}
	ns, err := GetKubeNamespace()
	if err != nil {
		return "", false, err
	}
	return ns, false, nil
}

// FilterOwnerContainers gets a list of WekaContainers and returns only those that have an owner reference matching the given owner(s) (WekaCluster or WekaClient)
func FilterOwnerContainers(all []v1alpha1.WekaContainer, owners ...client.Object) []v1alpha1.WekaContainer {
	var out []v1alpha1.WekaContainer

	if owners == nil || len(owners) == 0 {
		// do not filter if no owner provided
		return all
	}
	for _, wc := range all { // iterate over weka containers
		for _, owner := range owners {
			found := false
			kind := owner.GetObjectKind().GroupVersionKind().Kind
			for _, o := range wc.GetOwnerReferences() {
				if o.Kind != kind {
					continue
				}
				if o.UID == owner.GetUID() {
					out = append(out, wc)
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	return out
}

// HasAnyLabelValue checks if any of the given label keys have any of the given values
func HasAnyLabelValue(labels map[string]string, keys []string, values []string) bool {
	for _, k := range keys {
		if v, ok := labels[k]; ok {
			for _, want := range values {
				if v == want {
					return true
				}
			}
		}
	}
	return false
}

// ParseSelector converts a label selector string to a map for crclient.MatchingLabels
func ParseSelector(selector string) map[string]string {
	result := make(map[string]string)
	if selector == "" {
		return result
	}

	pairs := strings.Split(selector, ",")
	for _, pair := range pairs {
		kv := strings.Split(strings.TrimSpace(pair), "=")
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}

// SelectorMapToSelector converts a label map to a selector string
func SelectorMapToSelector(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	ls := labels.Set(m)
	return labels.SelectorFromSet(ls).String()
}

// FormatQuantityToGB converts a resource quantity to human-readable format in the largest appropriate unit
// e.g., 2000Mi -> "2GB", 2500Mi -> "2.4GB", 512Mi -> "512MB", 512Ki -> "512KB"
func FormatQuantityToGB(val interface{}) string {
	qty, ok := val.(resource.Quantity)
	if !ok {
		// Try pointer
		if ptr, ok := val.(*resource.Quantity); ok && ptr != nil {
			qty = *ptr
		} else {
			return "-"
		}
	}

	// Get the value in bytes (canonical form)
	bytes := qty.Value()
	if bytes < 0 {
		bytes = -bytes
	}

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	// Format with appropriate precision, using the largest unit that keeps value >= 1
	switch {
	case bytes >= TB:
		value := float64(bytes) / float64(TB)
		if value >= 10 {
			return fmt.Sprintf("%.0fTB", value)
		}
		return fmt.Sprintf("%.1fTB", value)
	case bytes >= GB:
		value := float64(bytes) / float64(GB)
		if value >= 10 {
			return fmt.Sprintf("%.0fGB", value)
		}
		return fmt.Sprintf("%.1fGB", value)
	case bytes >= MB:
		value := float64(bytes) / float64(MB)
		if value >= 10 {
			return fmt.Sprintf("%.0fMB", value)
		}
		return fmt.Sprintf("%.1fMB", value)
	case bytes >= KB:
		value := float64(bytes) / float64(KB)
		if value >= 10 {
			return fmt.Sprintf("%.0fKB", value)
		}
		return fmt.Sprintf("%.1fKB", value)
	default:
		return fmt.Sprintf("%d", bytes)
	}
}

// CompareNodeNames compares two node names numerically
// Returns -1 if a < b, 0 if a == b, 1 if a > b
func CompareNodeNames(a, b string) int {
	// Split each name into alternating text and number parts
	aParts := SplitNameToParts(a)
	bParts := SplitNameToParts(b)

	// Compare each part
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aPart := aParts[i]
		bPart := bParts[i]

		// Try to parse as numbers
		aNum, aIsNum := utils.TryParseInt(aPart)
		bNum, bIsNum := utils.TryParseInt(bPart)

		if aIsNum && bIsNum {
			// Both are numbers, compare numerically
			if aNum < bNum {
				return -1
			} else if aNum > bNum {
				return 1
			}
		} else if aIsNum != bIsNum {
			// One is number, one is text - numbers come after text
			if aIsNum {
				return 1
			}
			return -1
		} else {
			// Both are text, compare alphabetically
			if aPart < bPart {
				return -1
			} else if aPart > bPart {
				return 1
			}
		}
	}

	// One is prefix of the other
	if len(aParts) < len(bParts) {
		return -1
	} else if len(aParts) > len(bParts) {
		return 1
	}
	return 0
}

func SanitizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}

// SplitNameToParts splits a string (e.g. node name) into alternating text and number parts
// e.g., "h5-15-a" -> ["h", "5", "-", "15", "-", "a"]
func SplitNameToParts(name string) []string {
	var parts []string
	var current strings.Builder
	isDigit := false

	for _, r := range name {
		if (r >= '0' && r <= '9') != isDigit {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			isDigit = !isDigit
		}
		current.WriteRune(r)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
