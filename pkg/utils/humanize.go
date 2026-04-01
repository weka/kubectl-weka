package utils

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
	"time"
)

// HumanAge converts a time value to a human-readable age string
func HumanAge(t interface{}) string {
	var d time.Duration
	if t == nil {
		return "-"
	}
	switch v := t.(type) {
	case time.Time:
		d = time.Since(v)
	case v1.Time:
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

func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// HumanMbps converts Mbps to human-readable format
func HumanMbps(r interface{}) string {
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

// CapitalizeFirst capitalizes the first letter of a string
func CapitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
