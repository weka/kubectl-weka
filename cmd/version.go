package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information - set by main package via SetVersion()
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// SetVersion sets the version information from main package
func SetVersion(version, commit, date string) {
	Version = version
	Commit = commit
	Date = date
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of kubectl-weka",
	Long:  `Display the version, commit hash, and build date of kubectl-weka.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Plugin version: %s\n", Version)
		fmt.Printf("Commit: %s\n", Commit)
		fmt.Printf("Build date: %s\n", Date)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.SilenceUsage = true
}
