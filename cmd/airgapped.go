package cmd

import "github.com/spf13/cobra"

var airgappedCmd = &cobra.Command{
	Use:   "air-gapped",
	Short: "Tools for air-gapped (offline) WEKA deployments",
	Long: `Air-gapped deployment tools for environments without internet access.

This command group helps you prepare and deploy WEKA in air-gapped environments by:
- Downloading Docker images for WEKA components
- Uploading images to custom registries
- Updating Helm charts with custom image URLs
- Providing step-by-step deployment guidance`,
}
