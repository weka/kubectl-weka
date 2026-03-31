package utils

import (
	"fmt"
	"github.com/weka/kubectl-weka/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"os"
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

// JoinAndTruncate joins items with a limit and adds ellipsis for overflow
func JoinAndTruncate(items []string, max int) string {
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

// TryParseInt tries to parse a string as an integer
// Returns the integer value and whether parsing was successful
func TryParseInt(s string) (int, bool) {
	num, err := strconv.Atoi(s)
	return num, err == nil
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
