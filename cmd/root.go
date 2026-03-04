package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
	// Initialize controller-runtime logger before any client operations
	// This prevents the "eventuallyFulfillRoot" panic when background goroutines try to log
	// Use a quiet logger that doesn't spam output
	opts := zap.Options{
		Development: false, // Production mode - less verbose
		Level:       nil,   // Only log errors and above (no info/debug spam)
	}

	// Create and set the logger
	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
	log.SetLogger(logger)

	// Also configure klog to be quiet (used by some k8s libraries)
	klog.SetLogger(logger)

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
	InitializeHostCheckRegistry()

	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(preflightCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(planCmd)

}
