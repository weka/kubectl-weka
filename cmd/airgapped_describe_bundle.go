package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/airgapped"
)

var (
	describeBundleVerbose bool
)

var airgappedDescribeBundleCmd = &cobra.Command{
	Use:   "describe-bundle <bundle-file>",
	Short: "Display manifest contents of an air-gapped bundle",
	Long: `Describe the contents of an air-gapped bundle file.

This command:
1. Verifies the bundle's integrity using SHA256 checksum
2. Extracts and displays the manifest.json
3. Shows all components, Docker images, and Helm charts
4. Lists original Docker image URLs and architectures
5. Displays Helm chart information and sources

The bundle file is a tar.gz archive created by the 'download' command.
A companion .sha256 file should exist in the same directory for integrity verification.

Examples:
  kubectl weka air-gapped describe-bundle weka-5.3.0-offline-bundle.tar.gz
  kubectl weka air-gapped describe-bundle ./my-bundle.tar.gz`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bundleFile := args[0]
		if bundleFile == "" {
			return fmt.Errorf("no bundle file specified")
		}
		cmd.SilenceUsage = true

		return airgapped.DescribeBundle(bundleFile, describeBundleVerbose)
	},
}

func init() {
	airgappedDescribeBundleCmd.Flags().BoolVarP(&describeBundleVerbose, "verbose", "v", false, "Show verbose output with detailed image and chart information")
	airgappedDescribeBundleCmd.ValidArgsFunction = completionListAllTarGzFilesInDirectory
	airgappedCmd.AddCommand(airgappedDescribeBundleCmd)
}
