package utils

import "github.com/weka/kubectl-weka/pkg/types"

// Simple ANSI colors. If you want, we can auto-disable color when not a TTY / NO_COLOR.
func Green(s string) string  { return "\033[32m" + s + "\033[0m" }
func Red(s string) string    { return "\033[31m" + s + "\033[0m" }
func Yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func Cyan(s string) string   { return "\033[36m" + s + "\033[0m" }

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
