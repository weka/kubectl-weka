package cmd

import "github.com/spf13/cobra"

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan, simulate and validate deployment of WEKA resources (without applying)",
}
