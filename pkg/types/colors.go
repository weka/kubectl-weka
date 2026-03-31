package types

// ANSI color codes
const (
	ColorCompute = "\033[36m" // Cyan for compute
	ColorDrive   = "\033[35m" // Magenta for drive
	ColorS3      = "\033[33m" // Yellow for S3
	ColorNFS     = "\033[32m" // Green for NFS
	ColorEnvoy   = "\033[34m" // Blue for envoy
	ColorClient  = "\033[31m" // Orange for client
	ColorReset   = "\033[0m"  // Reset color
	ColorDefault = "\033[35m" // Default is magenta too
	ColorUsed    = "\033[38;5;52m"
	ColorFree    = "\033[90m" // Dark gray for free
)
