package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	flagNamespace     string
	flagAllNamespaces bool
	flagNoHeaders     bool
	flagOutput        string
	flagNodeSelector  string
	flagFailFast      bool
	flagRole          string

	KubeClients *kubernetes.K8sClients
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
	if commandNeedsCluster(os.Args[1:]) {
		var err error
		KubeClients, err = kubernetes.NewK8sClients(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to initialize Kubernetes client: %v\n", err)
			os.Exit(1)
		}
		defer KubeClients.Stop()
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

}

// annotationSkipClusterAccess marks a command (and everything under it) as usable
// without cluster access. Set it on commands that never touch the Kubernetes API.
const annotationSkipClusterAccess = "weka.io/skip-cluster-access"

// commandNeedsCluster reports whether the command being invoked requires a
// Kubernetes client. Air-gapped bundling runs on an internet-connected workstation
// and registry-side hosts that may have no cluster at all, and printing a version
// or a help text never needs one, so those must not be blocked by a missing
// kubeconfig. Anything unrecognized is treated as needing a client, keeping the
// stricter behavior for real cluster commands.
func commandNeedsCluster(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return false
		}
		// shell completion runs on every TAB and must never fail on kubeconfig.
		// Completion callbacks do not use the global client: those that need one
		// build it themselves and fall back to no completions if it fails.
		if arg == cobra.ShellCompRequestCmd || arg == cobra.ShellCompNoDescRequestCmd {
			return false
		}
	}

	cmd, _, err := rootCmd.Find(args)
	if err != nil || cmd == nil {
		return true
	}
	for c := cmd; c != nil; c = c.Parent() {
		if c.Annotations[annotationSkipClusterAccess] == "true" {
			return false
		}
	}
	return true
}

func init() {
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(preflightCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(supportBundleCmd)
	rootCmd.AddCommand(airgappedCmd)
}
