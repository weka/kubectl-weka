package utils

import (
	"strings"
)

// BoolPtr returns a pointer to a bool value
func BoolPtr(b bool) *bool { return &b }

// Int64Ptr returns a pointer to an int64 value
func Int64Ptr(i int64) *int64 { return &i }

// ShortErr returns a shortened error string
func ShortErr(err error) string {
	// Keep it readable inside brackets
	s := err.Error()
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	return s
}
