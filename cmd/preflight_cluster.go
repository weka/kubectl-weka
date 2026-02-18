package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var preflightK8sClusterCmd = &cobra.Command{
	Use:   "cluster [NODE...]",
	Short: "Preflight cluster checks (platform, permissions, kubelet configuration)",
	Args:  cobra.ArbitraryArgs,
	RunE:  runPreflightK8sCluster,
}

func init() {
	preflightCmd.AddCommand(preflightK8sClusterCmd)
	preflightK8sClusterCmd.Flags().StringVar(&preflightNodeSelector, "node-selector", "", "Label selector to filter nodes for node-scoped cluster checks")
}

func runPreflightK8sCluster(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	anyFail := false

	clientset := KubeClients.Clientset
	crClient := KubeClients.CRClient

	// Resolve nodes once using cached client (used for node-scoped cluster checks: cpu policy, CNI health)
	nodes, err := resolveNodes(ctx, crClient, args, preflightNodeSelector)
	if err != nil {
		return err
	}

	// ---- 1) Kubernetes version 1.24+ ----
	{
		title := "Validating Kubernetes version is 1.24+..."
		ok, detail, err := checkK8sVersion124Plus(ctx, clientset)
		if err != nil {
			printCheckResult(title, false, fmt.Sprintf("error: %v", err))
			anyFail = true
		} else if ok {
			printCheckResult(title, true, "")
		} else {
			printCheckResult(title, false, detail)
			anyFail = true
		}
	}

	// ---- 2) Kubernetes not being ROSA or managed OpenShift ----
	{
		title := "Validating cluster is not ROSA / managed OpenShift..."
		ok, detail, err := checkNotOpenShiftOrROSA(ctx, clientset)
		if err != nil {
			printCheckResult(title, false, fmt.Sprintf("error: %v", err))
			anyFail = true
		} else if ok {
			printCheckResult(title, true, "")
		} else {
			printCheckResult(title, false, detail)
			anyFail = true
		}
	}

	// ---- 3) Permissions sufficient for Helm install (cluster-scoped, not single-namespace) ----
	{
		title := "Validating permissions for Helm install (cluster-scope)..."
		ok, detail, err := checkHelmClusterPermissions(ctx, clientset)
		if err != nil {
			printCheckResult(title, false, fmt.Sprintf("error: %v", err))
			anyFail = true
		} else if ok {
			printCheckResult(title, true, "")
		} else {
			printCheckResult(title, false, detail)
			anyFail = true
		}
	}

	// ---- 4) CNI configured ----
	{
		title := "Validating CNI is configured..."
		ok, detail, err := checkCNIConfigured(ctx, clientset, nodes)
		if err != nil {
			printCheckResult(title, false, fmt.Sprintf("error: %v", err))
			anyFail = true
		} else if ok {
			printCheckResult(title, true, "")
		} else {
			printCheckResult(title, false, detail)
			anyFail = true
		}
	}

	// ---- Existing: cpuManagerPolicy must be static ----
	{
		title := "Validating cpu policy set to static..."
		ok, detail := checkCPUManagerPolicyStatic(ctx, clientset, nodes)
		if ok {
			printCheckResult(title, true, detail)
		} else {
			printCheckResult(title, false, detail)
			anyFail = true
		}
	}

	if anyFail {
		return fmt.Errorf("preflight cluster failed")
	}

	// Give background Kubernetes client goroutines time to shut down gracefully
	time.Sleep(100 * time.Millisecond)

	return nil
}
