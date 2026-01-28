package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ---- CLI ----

var (
	verifyNodeSelector string

	// thresholds (can be flags later; default to what you asked)
	minFreeMem = resource.MustParse("4Gi")
	minFreeHP  = resource.MustParse("3Gi")
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Run verification/analytics checks",
}

var verifyNodeCmd = &cobra.Command{
	Use:   "node [NODE...]",
	Short: "Verify node(s) for Weka readiness",
	Args:  cobra.ArbitraryArgs,
	RunE:  runVerifyNode,
}

func init() {
	verifyCmd.AddCommand(verifyNodeCmd)

	verifyNodeCmd.Flags().StringVar(&verifyNodeSelector, "node-selector", "", "Label selector to filter nodes (e.g. 'node-role.kubernetes.io/worker=')")
	// You can optionally expose thresholds:
	// verifyNodeCmd.Flags().String("min-free-mem", "4Gi", "Minimum free memory (allocatable minus requested)")
	// verifyNodeCmd.Flags().String("min-free-hugepages", "3Gi", "Minimum free hugepages-2Mi (allocatable minus requested)")
}

// ---- Implementation ----

type nodeVerifyResult struct {
	Name          string
	IP            string
	OSImage       string
	Kernel        string
	CPUManager    string // "static", "none", "unknown", etc.
	HugepagesSet  resource.Quantity
	HugepagesFree resource.Quantity
	MemFree       resource.Quantity
	Pass          bool
	Failures      []string
	Warnings      []string
}

func runVerifyNode(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
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

	// Resolve eligible nodes
	nodes, err := resolveNodes(ctx, clientset, args, verifyNodeSelector)
	if err != nil {
		return err
	}

	// Output
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)

	if !flagNoHeaders {
		if flagWide {
			fmt.Fprintln(w, "NAME\tIP\tOS\tKERNEL\tCPU_MGR\tHP_SET\tHP_FREE\tMEM_FREE\tRESULT\tDETAILS")
		} else {
			fmt.Fprintln(w, "NAME\tRESULT\tDETAILS")
		}
	}

	now := time.Now()
	_ = now // kept in case you later add age etc.

	for i := range nodes {
		res, err := verifyOneNode(ctx, clientset, &nodes[i])
		if err != nil {
			// hard error for that node only; still print it
			res = nodeVerifyResult{
				Name:     nodes[i].Name,
				Pass:     false,
				Failures: []string{fmt.Sprintf("error: %v", err)},
			}
		}
		printVerifyResult(w, res, flagWide)
	}

	w.Flush()
	return nil
}

func resolveNodes(ctx context.Context, clientset *kubernetes.Clientset, names []string, selector string) ([]corev1.Node, error) {
	// If explicit node names provided: Get each
	if len(names) > 0 {
		out := make([]corev1.Node, 0, len(names))
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			obj, err := clientset.CoreV1().Nodes().Get(ctx, n, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			out = append(out, *obj)
		}
		return out, nil
	}

	// Else list all or selector-filtered
	opts := metav1.ListOptions{}
	if selector != "" {
		opts.LabelSelector = selector
	}

	list, err := clientset.CoreV1().Nodes().List(ctx, opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func verifyOneNode(ctx context.Context, clientset *kubernetes.Clientset, n *corev1.Node) (nodeVerifyResult, error) {
	res := nodeVerifyResult{
		Name:    n.Name,
		IP:      firstInternalIP(n),
		OSImage: n.Status.NodeInfo.OSImage,
		Kernel:  n.Status.NodeInfo.KernelVersion,
		Pass:    true,
	}

	// (1) OS is Ubuntu
	if !strings.Contains(strings.ToLower(n.Status.NodeInfo.OSImage), "ubuntu") {
		res.Failures = append(res.Failures, "OS!=Ubuntu")
		res.Pass = false
	}

	// (2) hugepages set (2Mi)
	hpSet := quantityOrZero(n.Status.Capacity, "hugepages-2Mi")
	hpAlloc := quantityOrZero(n.Status.Allocatable, "hugepages-2Mi")
	if hpSet.IsZero() && hpAlloc.IsZero() {
		res.Failures = append(res.Failures, "hugepages-2Mi not set")
		res.Pass = false
	}
	res.HugepagesSet = hpSet

	// (3) cpuManagerPolicy static from kubelet configz (best-effort)
	cpuPol, cpuPolErr := getCPUManagerPolicyViaConfigz(ctx, clientset, n.Name)
	if cpuPolErr != nil {
		res.Warnings = append(res.Warnings, "cpuManagerPolicy unknown (configz unavailable)")
		res.CPUManager = "unknown"
		// You asked to enforce it; treat unknown as fail (change to warning if you prefer)
		res.Failures = append(res.Failures, "cpuManagerPolicy!=static (unknown)")
		res.Pass = false
	} else {
		res.CPUManager = cpuPol
		if strings.ToLower(cpuPol) != "static" {
			res.Failures = append(res.Failures, "cpuManagerPolicy!=static")
			res.Pass = false
		}
	}

	// (4) free RAM + hugepages free: allocatable - requested(pods)
	memFree, hpFree, calcWarn, err := computeFreeFromRequests(ctx, clientset, n.Name, hpAlloc)
	if err != nil {
		return res, err
	}
	if calcWarn != "" {
		res.Warnings = append(res.Warnings, calcWarn)
	}
	res.MemFree = memFree
	res.HugepagesFree = hpFree

	if memFree.Cmp(minFreeMem) < 0 {
		res.Failures = append(res.Failures, fmt.Sprintf("mem_free<%s", minFreeMem.String()))
		res.Pass = false
	}
	if hpFree.Cmp(minFreeHP) < 0 {
		res.Failures = append(res.Failures, fmt.Sprintf("hp_free<%s", minFreeHP.String()))
		res.Pass = false
	}

	return res, nil
}

func printVerifyResult(w *tabwriter.Writer, r nodeVerifyResult, wide bool) {
	result := "PASS"
	if !r.Pass {
		result = "FAIL"
	}

	details := strings.Join(r.Failures, ",")
	if details == "" {
		details = "-"
	}

	// Optionally append warnings (without failing)
	if len(r.Warnings) > 0 {
		details = details + " (warn: " + strings.Join(r.Warnings, ",") + ")"
	}

	if wide {
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Name,
			r.IP,
			r.OSImage,
			r.Kernel,
			r.CPUManager,
			r.HugepagesSet.String(),
			r.HugepagesFree.String(),
			r.MemFree.String(),
			result,
			details,
		)
	} else {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Name, result, details)
	}
}

func firstInternalIP(n *corev1.Node) string {
	for _, a := range n.Status.Addresses {
		if a.Type == corev1.NodeInternalIP {
			return a.Address
		}
	}
	if len(n.Status.Addresses) > 0 {
		return n.Status.Addresses[0].Address
	}
	return ""
}

func quantityOrZero(rl corev1.ResourceList, key corev1.ResourceName) resource.Quantity {
	if q, ok := rl[key]; ok {
		return q.DeepCopy()
	}
	return resource.MustParse("0")
}

// ---- CPU Manager Policy via configz ----

// kubelet configz is served via apiserver proxy:
//
//	GET /api/v1/nodes/<node>/proxy/configz
//
// Typical shape:
//
//	{"kubeletconfig": {"cpuManagerPolicy":"static", ...}, ...}
func getCPUManagerPolicyViaConfigz(ctx context.Context, clientset *kubernetes.Clientset, nodeName string) (string, error) {
	raw, err := clientset.CoreV1().
		RESTClient().
		Get().
		AbsPath("/api/v1/nodes", nodeName, "proxy", "configz").
		Do(ctx).
		Raw()
	if err != nil {
		return "", err
	}

	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return "", err
	}

	// Try top["kubeletconfig"]["cpuManagerPolicy"]
	kc, ok := top["kubeletconfig"].(map[string]any)
	if ok {
		if v, ok := kc["cpuManagerPolicy"].(string); ok && v != "" {
			return v, nil
		}
	}

	// Some kubelets expose slightly different nesting; be tolerant:
	// search for "cpuManagerPolicy" anywhere one level deep
	for _, v := range top {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if pol, ok := m["cpuManagerPolicy"].(string); ok && pol != "" {
			return pol, nil
		}
	}

	return "", fmt.Errorf("cpuManagerPolicy not found in configz")
}

// ---- Free resources analytics: allocatable - sum(requests) ----

func computeFreeFromRequests(ctx context.Context, clientset *kubernetes.Clientset, nodeName string, hpAlloc resource.Quantity) (memFree resource.Quantity, hpFree resource.Quantity, warn string, err error) {
	// List all pods scheduled to this node across namespaces
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return resource.Quantity{}, resource.Quantity{}, "", err
	}

	var memReq resource.Quantity
	memReq = resource.MustParse("0")
	var hpReq resource.Quantity
	hpReq = resource.MustParse("0")

	for i := range pods.Items {
		p := &pods.Items[i]

		// Skip terminal pods
		if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}

		pMem, pHP := podRequests(p)
		memReq.Add(pMem)
		hpReq.Add(pHP)
	}

	// allocatable memory from NodeStatus is not passed here; fetch it from node (cheap get)
	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return resource.Quantity{}, resource.Quantity{}, "", err
	}

	memAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceMemory)

	// free = alloc - requested (never below 0)
	memFree = memAlloc.DeepCopy()
	memFree.Sub(memReq)
	if memFree.Sign() < 0 {
		warn = "mem requested > allocatable (check requests?)"
		memFree = resource.MustParse("0")
	}

	hpFree = hpAlloc.DeepCopy()
	hpFree.Sub(hpReq)
	if hpFree.Sign() < 0 {
		warn = strings.TrimSpace(warn + " hp requested > allocatable")
		hpFree = resource.MustParse("0")
	}

	return memFree, hpFree, warn, nil
}

// podRequests returns total requested (memory, hugepages-2Mi) for the pod, using kube scheduling semantics:
// - sum regular containers
// - init containers: take max
// - add overhead if present
func podRequests(p *corev1.Pod) (mem resource.Quantity, hp resource.Quantity) {
	mem = resource.MustParse("0")
	hp = resource.MustParse("0")

	sumMem := resource.MustParse("0")
	sumHP := resource.MustParse("0")

	for i := range p.Spec.Containers {
		c := &p.Spec.Containers[i]
		sumMem.Add(quantityOrZero(c.Resources.Requests, corev1.ResourceMemory))
		sumHP.Add(quantityOrZero(c.Resources.Requests, "hugepages-2Mi"))
	}

	maxInitMem := resource.MustParse("0")
	maxInitHP := resource.MustParse("0")
	for i := range p.Spec.InitContainers {
		c := &p.Spec.InitContainers[i]
		m := quantityOrZero(c.Resources.Requests, corev1.ResourceMemory)
		h := quantityOrZero(c.Resources.Requests, "hugepages-2Mi")
		if m.Cmp(maxInitMem) > 0 {
			maxInitMem = m
		}
		if h.Cmp(maxInitHP) > 0 {
			maxInitHP = h
		}
	}

	// total requested is max(sum(containers), max(init)) per resource
	mem = sumMem.DeepCopy()
	if maxInitMem.Cmp(mem) > 0 {
		mem = maxInitMem.DeepCopy()
	}
	hp = sumHP.DeepCopy()
	if maxInitHP.Cmp(hp) > 0 {
		hp = maxInitHP.DeepCopy()
	}

	// overhead
	if p.Spec.Overhead != nil {
		if ov, ok := (p.Spec.Overhead)[corev1.ResourceMemory]; ok {
			mem.Add(ov)
		}
		if ov, ok := (p.Spec.Overhead)["hugepages-2Mi"]; ok {
			hp.Add(ov)
		}
	}

	return mem, hp
}
