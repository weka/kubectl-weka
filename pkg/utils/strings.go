package utils

import (
	"fmt"
	"math/rand"
	"strings"
)

// SplitAndTrim splits a string by a separator and trims whitespace from each part
func SplitAndTrim(s string, sep string) []string {
	all := strings.Split(s, sep)
	var ret []string
	for _, part := range all {
		ret = append(ret, strings.TrimSpace(part))
	}
	return ret
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

// JoinWithTruncation joins items with a limit and adds ellipsis for overflow
func JoinWithTruncation(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(", ...(+%d)", len(items)-max)
}

// TruncateString truncates a string to maxLength characters and adds ellipsis if needed
func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

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
