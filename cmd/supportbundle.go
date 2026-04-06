package cmd

import (
	"github.com/spf13/cobra"
)

var (
	flagCaseID           string
	flagIncludeSensitive bool
	flagDebug            bool
)

var supportBundleCmd = &cobra.Command{
	Use:   "support-bundle",
	Short: "Collect diagnostic information for support purposes",
	Long: `Collect logs, object descriptions, and other diagnostic information
for troubleshooting Weka operator and cluster issues. The collected data
is packaged into a compressed archive that can be shared with support.`,
}

func init() {
	supportBundleCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Enable debug output")
	supportBundleCmd.PersistentFlags().StringVar(&flagCaseID, "case-id", "", "Associate the support bundle with a specific case ID")
	supportBundleCmd.Flags().BoolVar(&flagIncludeSensitive, "include-sensitive-data", false, "Include sensitive data such as Secrets and credentials (⚠️  INSECURE - use with caution)")
}
