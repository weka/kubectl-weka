package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of kubectl-weka",
	Long:  `Display the version, commit hash, and build date of kubectl-weka.`,
	// Build metadata is compiled in; reporting it must not require a cluster.
	Annotations: map[string]string{annotationSkipClusterAccess: "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Plugin version: %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.Commit)
		fmt.Printf("Build date: %s\n", version.Date)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.SilenceUsage = true
	versionCmd.CompletionOptions.SetDefaultShellCompDirective(cobra.ShellCompDirectiveNoFileComp)
}
