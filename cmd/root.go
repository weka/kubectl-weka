package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagNamespace     string
	flagAllNamespaces bool
	flagNoHeaders     bool
	flagWide          bool
)

var rootCmd = &cobra.Command{
	Use:   "kubectl-weka",
	Short: "kubectl plugin for Weka operator",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(preflightCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(planCmd)
}
