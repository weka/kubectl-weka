package progress

import (
	"fmt"
	"strings"
)

// RenderProgress prints a progress bar with percentage and sizes
func RenderProgress(current, total int64, category, operation string) {
	if total <= 0 {
		return
	}

	// Cap the percentage at 100% to avoid showing >100%
	percentage := float64(current) / float64(total) * 100
	if percentage > 100 {
		percentage = 100
	}

	barLength := 30
	filledLength := int(float64(barLength) * percentage / 100)
	bar := strings.Repeat("=", filledLength) + strings.Repeat(" ", barLength-filledLength)

	currentStr := formatBytes(current)
	totalStr := formatBytes(total)

	// \r moves to start of line, \033[K clears line to end of cursor
	fmt.Printf("\r%-10s [%-30s] %6.2f%% (%s/%s) %s\033[K", category, bar, percentage, currentStr, totalStr, operation)
	if percentage >= 100 {
		fmt.Println() // New line when complete
	}
}

// formatBytes converts bytes to human-readable format (B, KB, MB, GB, etc.)
func formatBytes(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(bytes)
	unitIdx := 0

	for value >= 1024 && unitIdx < len(units)-1 {
		value /= 1024
		unitIdx++
	}

	if unitIdx == 0 {
		return fmt.Sprintf("%d %s", int64(value), units[unitIdx])
	}
	return fmt.Sprintf("%.1f %s", value, units[unitIdx])
}
