package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	minFreeMem = resource.MustParse("4Gi")
	minFreeHP  = resource.MustParse("3Gi")

	preflightFailFast    bool
	preflightSummaryOnly bool
)

var preflightNodesCmd = &cobra.Command{
	Use:   "nodes [NODE...]",
	Short: "Preflight node checks (OS, hugepages, free resources, host readiness)",
	Args:  cobra.ArbitraryArgs,
	RunE:  runPreflightNodes,
}

func init() {
	preflightCmd.AddCommand(preflightNodesCmd)
	preflightNodesCmd.Flags().StringVar(&preflightNodeSelector, "node-selector", "", "Label selector to filter nodes")
	preflightNodesCmd.Flags().BoolVar(&preflightFailFast, "fail-fast", false, "Stop on first failed node")
	preflightNodesCmd.Flags().BoolVar(&preflightSummaryOnly, "summary-only", false, "Only print summary (no per-node details)")
	preflightNodesCmd.SilenceUsage = true
	// preflightNodesCmd.SilenceErrors = true // only if you print errors yourself
}

type checkStatus string

const (
	statusPass checkStatus = "PASS"
	statusWarn checkStatus = "WARN"
	statusFail checkStatus = "FAIL"
)

type nodeCheck struct {
	title  string
	status checkStatus
	detail string // printed in [..]
}

func runPreflightNodes(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	nodes, err := resolveNodes(ctx, clientset, args, preflightNodeSelector)
	if err != nil {
		return err
	}

	fmt.Println("Performing preflight verification for Kubernetes nodes to host WEKA")
	printCheckResult("Checking total number of eligible nodes...", true, fmt.Sprintf("%d", len(nodes)))

	// Run host checks via per-node pods (PCI, /etc/os-release, filesystem, processes, systemd units)
	hostFacts, scanErrs := scanHostChecksByPod(ctx, clientset, nodes)
	if len(scanErrs) > 0 && !preflightSummaryOnly {
		fmt.Printf("Note: host checks encountered %d issues; affected nodes may be WARN/FAIL depending on the check\n\n", len(scanErrs))
	}

	if !preflightSummaryOnly {
		fmt.Println("Validating node eligibility:")
	}

	checked := 0
	passCnt := 0
	warnCnt := 0
	failCnt := 0

	var warnedNodes []string
	var failedNodes []string

	for i := range nodes {
		n := nodes[i]
		checked++

		hf := hostFacts[n.Name] // zero value if missing (e.g. scan failed hard)
		checks, err := buildNodeChecks(ctx, clientset, &n, hf)
		if err != nil {
			checks = append(checks, nodeCheck{
				title:  "node_checks",
				status: statusFail,
				detail: fmt.Sprintf("error: %v", err),
			})
		}

		nodeStatus := aggregateNodeStatus(checks)
		switch nodeStatus {
		case statusPass:
			passCnt++
		case statusWarn:
			warnCnt++
			warnedNodes = append(warnedNodes, n.Name)
		default:
			failCnt++
			failedNodes = append(failedNodes, n.Name)
		}

		if !preflightSummaryOnly {
			printNodeHeader(n.Name, nodeStatus)
			for _, c := range checks {
				printNodeSubcheck(c)
			}
			fmt.Println()
		}

		if preflightFailFast && nodeStatus == statusFail {
			break
		}
	}

	// Summary
	fmt.Println("Summary:")
	fmt.Printf("  Eligible nodes:      %d\n", len(nodes))
	fmt.Printf("  Nodes checked:       %d\n", checked)
	fmt.Printf("  Nodes passed:        %s\n", green(fmt.Sprintf("%d", passCnt)))
	fmt.Printf("  Nodes warned:        %s\n", yellow(fmt.Sprintf("%d", warnCnt)))
	fmt.Printf("  Nodes failed:        %s\n", red(fmt.Sprintf("%d", failCnt)))

	if warnCnt > 0 {
		fmt.Printf("  Warned nodes:        %s\n", strings.Join(warnedNodes, ", "))
	}
	if failCnt > 0 {
		fmt.Printf("  Failed nodes:        %s\n", strings.Join(failedNodes, ", "))
	}

	// WARN does not fail preflight, FAIL does.
	if failCnt > 0 {
		return fmt.Errorf("preflight nodes failed")
	}
	return nil
}

func buildNodeChecks(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	n *corev1.Node,
	hf HostChecksResult,
) ([]nodeCheck, error) {
	var out []nodeCheck

	// 1) OS must be Ubuntu (from kube nodeInfo)
	osImage := n.Status.NodeInfo.OSImage
	osOK := strings.Contains(strings.ToLower(osImage), "ubuntu")
	out = append(out, nodeCheck{
		title:  "os",
		status: passOrFail(osOK),
		detail: chooseDetail(osOK, osImage, fmt.Sprintf("OS is %q, expected Ubuntu", osImage)),
	})
	// print out kernel version
	out = append(out, nodeCheck{
		title:  "kernel",
		status: statusPass,
		detail: n.Status.NodeInfo.KernelVersion,
	})

	// 2) hugepages configured (from node status)
	hpSet := quantityOrZero(n.Status.Capacity, "hugepages-2Mi")
	hpAlloc := quantityOrZero(n.Status.Allocatable, "hugepages-2Mi")
	hpOK := !(hpSet.IsZero() && hpAlloc.IsZero())

	out = append(out, nodeCheck{
		title:  "hugepages",
		status: passOrFail(hpOK),
		detail: chooseDetail(
			hpOK,
			fmt.Sprintf("set=%s allocatable=%s", hpSet.String(), hpAlloc.String()),
			"No hugepages configured on node",
		),
	})

	// 3/4) free resources (allocatable - sum(pod requests))
	memFree, hpFree, warn, err := computeFreeFromRequests(ctx, clientset, n.Name, hpAlloc)
	if err != nil {
		return out, err
	}

	memOK := memFree.Cmp(minFreeMem) >= 0
	out = append(out, nodeCheck{
		title:  "mem_free",
		status: passOrFail(memOK),
		detail: fmt.Sprintf("%s free (min %s)%s", memFree.String(), minFreeMem.String(), warnSuffix(warn)),
	})

	hpFreeOK := hpFree.Cmp(minFreeHP) >= 0
	out = append(out, nodeCheck{
		title:  "hugepages_free",
		status: passOrFail(hpFreeOK),
		detail: fmt.Sprintf("%s free (min %s)%s", hpFree.String(), minFreeHP.String(), warnSuffix(warn)),
	})

	// --- Host-based checks (via per-node host-check pod) ---

	// 5) Weka directory exists and has >= 300GB available
	// For RHCOS: /root/k8s-weka ; otherwise: /opt/k8s-weka (handled by pod script)
	out = append(out, nodeCheck{
		title:  "weka_dir",
		status: passOrFail(hf.WekaDirOK),
		detail: fmt.Sprintf("%s: %s", nonEmpty(hf.WekaDirPath, "(unknown)"), nonEmpty(hf.WekaDirDetail, "no details")),
	})

	// 6) XFS installed (mkfs.xfs exists on host)
	out = append(out, nodeCheck{
		title:  "xfs",
		status: passOrFail(hf.XFSInstalled),
		detail: nonEmpty(hf.XFSDetail, "no details"),
	})

	// 7) Node is not running WEKA client
	// - no wekanode processes
	// - weka-agent service does not exist
	out = append(out, nodeCheck{
		title:  "weka_client",
		status: passOrFail(hf.WekaClientClean),
		detail: nonEmpty(hf.WekaClientDetail, "no details"),
	})

	out = append(out, formatMellanoxBlock(hf))
	out = append(out, formatBondLACPCheck(hf))

	return out, nil
}

func formatMellanoxBlock(hf HostChecksResult) nodeCheck {
	if len(hf.MlxIfaces) == 0 {
		return nodeCheck{
			title:  "mellanox_nic",
			status: statusWarn,
			detail: "No Mellanox NIC detected; only UDP mode is supported",
		}
	}

	var lines []string

	for _, nic := range hf.MlxIfaces {
		spd := nic.Speed
		if strings.TrimSpace(spd) == "" {
			spd = "unknown"
		}

		if nic.Bond != "" {
			lines = append(lines, fmt.Sprintf("%s [*%s] %s %s", nic.Name, nic.Bond, spd, nic.Model))
		} else {
			ip := nic.IP
			if strings.TrimSpace(ip) == "" {
				ip = "-"
			} else {
				lines = append(lines, fmt.Sprintf("%s [%s] %s %s", nic.Name, ip, spd, nic.Model))
			}
		}
	}

	for _, b := range hf.MlxBonds {
		ip := b.IP
		if strings.TrimSpace(ip) == "" {
			ip = "-"
		}
		spd := b.Speed
		if strings.TrimSpace(spd) == "" {
			spd = "unknown"
		}
		lines = append(lines, fmt.Sprintf("%s [%s] %s (%s)", b.Name, ip, spd, strings.Join(b.Slaves, ", ")))
	}

	return nodeCheck{
		title:  "mellanox_nic",
		status: statusPass,
		detail: strings.Join(lines, "\n"),
	}
}

func formatBondLACPCheck(hf HostChecksResult) nodeCheck {
	return nodeCheck{
		title:  "bond_lacp",
		status: passOrFail(hf.BondLACPOk),
		detail: nonEmpty(hf.BondLACPDetail, "no details"),
	}
}

func aggregateNodeStatus(checks []nodeCheck) checkStatus {
	hasWarn := false
	for _, c := range checks {
		if c.status == statusFail {
			return statusFail
		}
		if c.status == statusWarn {
			hasWarn = true
		}
	}
	if hasWarn {
		return statusWarn
	}
	return statusPass
}

func passOrFail(ok bool) checkStatus {
	if ok {
		return statusPass
	}
	return statusFail
}

func chooseDetail(ok bool, okDetail, failDetail string) string {
	if ok {
		return okDetail
	}
	return failDetail
}

func warnSuffix(w string) string {
	w = strings.TrimSpace(w)
	if w == "" {
		return ""
	}
	return "; warn=" + w
}

func nonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func printNodeHeader(name string, st checkStatus) {
	switch st {
	case statusPass:
		fmt.Printf("  %s: %s\n", name, green("PASS"))
	case statusWarn:
		fmt.Printf("  %s: %s\n", name, yellow("WARN"))
	default:
		fmt.Printf("  %s: %s\n", name, red("FAIL"))
	}
}

func printNodeSubcheck(c nodeCheck) {
	statusStr := ""
	switch c.status {
	case statusPass:
		statusStr = green("PASS")
	case statusWarn:
		statusStr = yellow("WARN")
	default:
		statusStr = red("FAIL")
	}

	// Base prefix up to '['
	prefix := fmt.Sprintf("     %s: %s [", c.title, statusStr)

	// If single-line, keep old behavior
	if !strings.Contains(c.detail, "\n") {
		fmt.Printf("%s%s]\n", prefix, c.detail)
		return
	}

	// Multiline: first line after '[' then subsequent lines aligned
	parts := strings.Split(c.detail, "\n")
	fmt.Printf("%s\n", prefix)

	// indent to align under the first detail char (same length as prefix)
	indent := strings.Repeat(" ", len(prefix)-9)
	for i := 0; i < len(parts); i++ {
		fmt.Printf("%s%s\n", indent, parts[i])
	}
	// close bracket on last line
	fmt.Printf("%s]\n", strings.Repeat(" ", len(prefix)-11))
}
