package cmd

import "github.com/spf13/cobra"

var supportBundleCmd = &cobra.Command{
	Use:   "support-bundle",
	Short: "Collect diagnostic information for support purposes",
	Long: `Collect logs, object descriptions, and other diagnostic information
for troubleshooting Weka operator and cluster issues. The collected data
is packaged into a compressed archive that can be shared with support.`,
}

func init() {
	supportBundleCmd.PersistentFlags().BoolVar(&supportBundleDebug, "debug", false, "Enable debug output")
}
