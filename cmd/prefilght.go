package cmd

import "github.com/spf13/cobra"

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Run preflight checks before WEKA deployment",
}
