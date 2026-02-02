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
	// Standard-ish flags (kubectl won't pass its global flags to plugins, so we implement our own)
	rootCmd.PersistentFlags().StringVarP(&flagNamespace, "namespace", "n", "", "If present, the namespace scope for this CLI request")
	rootCmd.PersistentFlags().BoolVarP(&flagAllNamespaces, "all-namespaces", "A", false, "If present, list the requested object(s) across all namespaces")

	// kubectl-get style flags
	rootCmd.PersistentFlags().BoolVar(&flagNoHeaders, "no-headers", false, "When using the default output format, don't print headers")
	rootCmd.PersistentFlags().BoolVar(&flagWide, "wide", false, "When using the default output format, print more information")

	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(preflightCmd)
	rootCmd.AddCommand(logsCmd)

}
