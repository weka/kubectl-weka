package cmd

import "github.com/spf13/cobra"

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan Weka cluster deployment",
}

func init() {
	rootCmd.AddCommand(planCmd)
}
