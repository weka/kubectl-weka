package cmd

import (
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/airgapped"
)

var airgappedHelpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show air-gapped deployment guide",
	Long: `Display a detailed guide for deploying WEKA in air-gapped (offline) environments.

This command provides step-by-step instructions for:
1. Downloading images on an internet-connected host
2. Transferring images to the air-gapped environment
3. Uploading images to a local registry
4. Updating Helm charts with local registry URLs
5. Deploying WEKA components`,
	RunE: runAirgappedHelp,
}

func init() {
	airgappedCmd.AddCommand(airgappedHelpCmd)
	airgappedHelpCmd.SilenceUsage = true
}

func runAirgappedHelp(cmd *cobra.Command, args []string) error {
	return airgapped.HelpAirGapped()
}
