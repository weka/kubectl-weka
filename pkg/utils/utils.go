package utils

import (
	"fmt"
	"github.com/weka/kubectl-weka/pkg/types"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// IndentText indents a block of text by the specified number of spaces
func IndentText(text string, spaces int, subsequentSpace ...int) string {
	if spaces <= 0 || text == "" {
		return text
	}

	indent := strings.Repeat(" ", spaces)
	subIndent := indent
	lines := strings.Split(text, "\n")

	if subsequentSpace != nil {
		subIndent = strings.Repeat(" ", subsequentSpace[0]) + subIndent
	}
	var result []string
	for i, line := range lines {
		if line == "" {
			result = append(result, "")
		} else if i > 0 {
			result = append(result, subIndent+line)
		} else {
			result = append(result, indent+line)
		}
	}

	return strings.Join(result, "\n")
}

// HumanAge converts a time value to a human-readable age string
func HumanAge(t interface{}) string {
	var d time.Duration
	if t == nil {
		return "-"
	}
	switch v := t.(type) {
	case time.Time:
		d = time.Since(v)
	case metav1.Time:
		d = time.Since(v.Time)
	case time.Duration:
		d = v
	case string:
		return v
	default:
		return "-"
	}
	if d < 0 {
		d = -d
	}
	// kubectl-ish compact
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d / (24 * time.Hour))
	if days < 365 {
		return fmt.Sprintf("%dd", days)
	}
	years := days / 365
	return fmt.Sprintf("%dy", years)
}

// WekaVersion represents a parsed WEKA version
type WekaVersion struct {
	Major int
	Minor int
	Patch int
	Build int
	Raw   string
}

func (v WekaVersion) String() string {
	if v.Build > 0 {
		return fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Build)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// ParseWekaVersion extracts version from WEKA container image
// Supports formats like:
//   - quay.io/weka.io/weka-in-container:4.4.10.200
//   - weka/weka:4.2.5
//   - registry.example.com/weka:4.3.0.100
//   - quay.io/weka.io/weka:5.1.0.461-qa-alpha
func ParseWekaVersion(image string) (*WekaVersion, error) {
	// Extract version from image tag (everything after the last ':')
	// Format: <registry>/<image>:<version>
	colonIndex := strings.LastIndex(image, ":")
	if colonIndex == -1 {
		return nil, fmt.Errorf("image does not contain version tag: %s", image)
	}

	versionStr := image[colonIndex+1:]

	// Remove any suffix after a dash (e.g., "-qa-alpha", "-rc1", "-dev")
	// This allows us to parse "5.1.0.461-qa-alpha" as "5.1.0.461"
	if dashIndex := strings.Index(versionStr, "-"); dashIndex != -1 {
		versionStr = versionStr[:dashIndex]
	}

	// Parse version components (e.g., "4.4.10.200" or "4.2.5")
	versionParts := strings.Split(versionStr, ".")
	if len(versionParts) < 3 {
		return nil, fmt.Errorf("invalid version format: %s (expected at least major.minor.patch)", versionStr)
	}

	version := &WekaVersion{Raw: versionStr}

	// Parse major version
	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version '%s': %w", versionParts[0], err)
	}
	version.Major = major

	// Parse minor version
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version '%s': %w", versionParts[1], err)
	}
	version.Minor = minor

	// Parse patch version
	patch, err := strconv.Atoi(versionParts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version '%s': %w", versionParts[2], err)
	}
	version.Patch = patch

	// Parse build version (optional)
	if len(versionParts) >= 4 {
		build, err := strconv.Atoi(versionParts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid build version '%s': %w", versionParts[3], err)
		}
		version.Build = build
	}

	return version, nil
}

// FormatSelector converts a label selector map to a string representation
func FormatSelector(selector map[string]string) string {
	if len(selector) == 0 {
		return "(none)"
	}
	var parts []string
	for key, value := range selector {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// CapitalizeFirst capitalizes the first letter of a string
func CapitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// RandomString generates a random string of specified length
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// WriteFile writes content to a file
func WriteFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

// ReadFile reads content from a file
func ReadFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// BoolPtr returns a pointer to a bool value
func BoolPtr(b bool) *bool { return &b }

// JoinLimited joins items with a limit and adds ellipsis for overflow
func JoinLimited(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(", ...(+%d)", len(items)-max)
}

// ShortErr returns a shortened error string
func ShortErr(err error) string {
	// Keep it readable inside brackets
	s := err.Error()
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	return s
}

// Simple ANSI colors. If you want, we can auto-disable color when not a TTY / NO_COLOR.
func Green(s string) string  { return "\033[32m" + s + "\033[0m" }
func Red(s string) string    { return "\033[31m" + s + "\033[0m" }
func Yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func Cyan(s string) string   { return "\033[36m" + s + "\033[0m" }

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

// TryParseInt tries to parse a string as an integer
// Returns the integer value and whether parsing was successful
func TryParseInt(s string) (int, bool) {
	num, err := strconv.Atoi(s)
	return num, err == nil
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

// MapKeysToList converts a map with string keys to a list of keys
func MapKeysToList(m map[string]struct{}) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// FirstOrNone returns the first string in a slice or "<none>" if empty
func FirstOrNone(xs []string) string {
	if len(xs) == 0 {
		return "<none>"
	}
	if strings.TrimSpace(xs[0]) == "" {
		return "<none>"
	}
	return xs[0]
}

// GetNameOrNone returns the name or "<none>" if empty
func GetNameOrNone(name string) string {
	if name == "" {
		return "<none>"
	}
	return name
}

// SelectorMapToSelector converts a label map to a selector string
func SelectorMapToSelector(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	ls := labels.Set(m)
	return labels.SelectorFromSet(ls).String()
}

// TruncateString truncates a string to maxLength characters and adds ellipsis if needed
func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

// BoolToOkError converts a bool to "OK" or "ERROR"
func BoolToOkError(v bool) string {
	if v {
		return "OK"
	}
	return "ERROR"
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

// FormatMbpsToHuman converts Mbps to human-readable format
func FormatMbpsToHuman(r interface{}) string {
	if num, ok := r.(int); ok {
		if num <= 0 {
			return "Unknown/No link"
		}
		if num >= 1000 {
			return strconv.Itoa(num/1000) + " Gbps"
		}
		return strconv.Itoa(num) + " Mbps"
	}
	return fmt.Sprintf("%v", r)
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
		aNum, aIsNum := TryParseInt(aPart)
		bNum, bIsNum := TryParseInt(bPart)

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

func SanitizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ColorizeContainerType returns a colored version of the container type name
func ColorizeContainerType(containerType string) string {

	switch containerType {
	case "compute":
		return types.ColorCompute + "Compute" + types.ColorReset
	case "drive":
		return types.ColorDrive + "Drive" + types.ColorReset
	case "s3":
		return types.ColorS3 + "S3" + types.ColorReset
	case "nfs":
		return types.ColorNFS + "NFS" + types.ColorReset
	case "envoy":
		return types.ColorEnvoy + "Envoy" + types.ColorReset
	case "client":
		return types.ColorClient + "Client" + types.ColorReset // Reuse cyan color for client
	default:
		return containerType
	}
}
