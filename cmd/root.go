package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagNamespace     string
	flagAllNamespaces bool
	flagNoHeaders     bool
	flagWide          bool
	KubeClients       *K8sClients
)

var rootCmd = &cobra.Command{
	Use:   "kubectl-weka",
	Short: "kubectl plugin for Weka operator",
}

func Execute() {
	ctx := context.Background()
	var err error
	KubeClients, err = NewK8sClients(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize Kubernetes client: %v\n", err)
		os.Exit(1)
	}
	defer KubeClients.Stop()

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
