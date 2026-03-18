package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

var (
	getClusterInstancesAllNamespaces bool
	getClusterInstancesNamespace     string
)

var getClusterInstancesCmd = &cobra.Command{
	Use:   "cluster-instances [WEKACLUSTER]",
	Short: "Show WEKA cluster instances (WekaContainers) and their status, optionally filtered by WekaCluster",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGetClusterInstances,
}

func init() {
	getCmd.AddCommand(getClusterInstancesCmd)

	getClusterInstancesCmd.Flags().BoolVarP(&getClusterInstancesAllNamespaces, "all-namespaces", "A", false, "If present, list WekaCluster resources across all namespaces")
	getClusterInstancesCmd.Flags().StringVarP(&getClusterInstancesNamespace, "namespace", "n", "", "Namespace. Defaults to current kubeconfig namespace")
	getClusterInstancesCmd.Flags().BoolVar(&flagNoHeaders, "no-headers", false, "Don't print headers")
	getClusterInstancesCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")

	getClusterInstancesCmd.SilenceUsage = true
}

func runGetClusterInstances(_ *cobra.Command, args []string) error {
	ctx := context.Background()

	currentNS, _, err := GetNamespaceFromFlags(getClusterInstancesAllNamespaces, getClusterInstancesNamespace)
	var targetCluster string

	printer, _ := GetPrinterFromFlags(flagOutput, !flagNoHeaders, nil, false, 0, TableStyleMinimal)
	output, err := generateClusterInstancesOutput(
		ctx,
		KubeClients,
		currentNS,
		getClusterInstancesAllNamespaces,
		targetCluster,
		printer,
	)
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}

// generateClusterInstancesOutput generates the cluster instances table as a string
func generateClusterInstancesOutput(
	ctx context.Context,
	clients *K8sClients,
	namespace string,
	allNamespaces bool,
	targetCluster string,
	printer ResourcePrinter,
) (string, error) {
	k8s := clients.Clientset
	crClient := clients.CRClient

	clusters, err := getWekaClustersTyped(ctx, crClient, namespace, allNamespaces, targetCluster)
	if err != nil {
		return "", err
	}

	// Preload WekaContainers + Pods per namespace once
	needNS := map[string]struct{}{}
	for i := range clusters {
		needNS[clusters[i].GetNamespace()] = struct{}{}
	}

	nsToContainers := map[string][]wekaapi.WekaContainer{}
	nsToPods := map[string]map[string]*corev1.Pod{}

	for ns := range needNS {
		var contList wekaapi.WekaContainerList
		if err := crClient.List(ctx, &contList, crclient.InNamespace(ns)); err != nil {
			return "", fmt.Errorf("failed to list WekaContainer CRs in namespace %q: %w", ns, err)
		}
		nsToContainers[ns] = contList.Items

		podList, err := k8s.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to list pods in namespace %q: %w", ns, err)
		}
		m := make(map[string]*corev1.Pod, len(podList.Items))
		for i := range podList.Items {
			p := &podList.Items[i]
			m[p.Name] = p
		}
		nsToPods[ns] = m
	}

	// Sort clusters stable by ns/name
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].GetNamespace() != clusters[j].GetNamespace() {
			return clusters[i].GetNamespace() < clusters[j].GetNamespace()
		}
		return clusters[i].GetName() < clusters[j].GetName()
	})

	// Define columns - include NAMESPACE only if showing all namespaces
	var columns []TableColumn
	columns = []TableColumn{
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "WEKACLUSTER", VisibleInWide: false},
		{Name: "NODE", VisibleInWide: false},
		{Name: "WEKACONTAINER", VisibleInWide: false},
		{Name: "WC_STATUS", VisibleInWide: false},
		{Name: "POD_STATUS", VisibleInWide: false},
		{Name: "MGMT_IP", VisibleInWide: false},
		{Name: "CONTAINER_ID", VisibleInWide: false},
		{Name: "CPU_UTIL", VisibleInWide: true},
		{Name: "AGE", VisibleInWide: true, formatFuncs: TableFormatFunctions{humanAge}},
	}

	// Build rows
	var rows []TableRow
	for i := range clusters {
		cluster := &clusters[i]
		clusterName := cluster.GetName()
		ns := cluster.GetNamespace()

		containers := nsToContainers[ns]
		podsByName := nsToPods[ns]

		matching := filterClusterContainersTyped(containers, cluster)

		// Sort stable by node, then name
		sort.Slice(matching, func(i, j int) bool {
			ai, aj := matching[i], matching[j]
			pi := podsByName[ai.GetName()]
			pj := podsByName[aj.GetName()]
			ni, nj := "", ""
			if pi != nil {
				ni = pi.Spec.NodeName
			}
			if pj != nil {
				nj = pj.Spec.NodeName
			}
			if ni != nj {
				return ni < nj
			}
			return ai.GetName() < aj.GetName()
		})

		for _, wc := range matching {
			wcName := wc.GetName()
			wcStatus := inferWekaContainerStatusTyped(&wc)

			ips := wc.Status.GetManagementIps()
			mgmtIP := firstOrNone(ips)

			containerID := "<none>"
			if wc.Status.ClusterContainerID != nil {
				containerID = fmt.Sprintf("%d", *wc.Status.ClusterContainerID)
			}

			podPhase := "<not-found>"
			nodeName := "<unknown>"
			if p := podsByName[wcName]; p != nil {
				podPhase = string(p.Status.Phase)
				if p.Spec.NodeName != "" {
					nodeName = p.Spec.NodeName
				}
			}

			age := wc.GetCreationTimestamp().Time
			cpuUtil := "<none>"
			if wc.Status.Stats != nil {
				cpuUtil = string(wc.Status.Stats.CpuUsage)
			}

			row := TableRow{Values: map[string]interface{}{}}
			row.Values["NAMESPACE"] = ns
			row.Values["WEKACLUSTER"] = clusterName
			row.Values["NODE"] = nodeName
			row.Values["WEKACONTAINER"] = wcName
			row.Values["WC_STATUS"] = wcStatus
			row.Values["POD_STATUS"] = podPhase
			row.Values["MGMT_IP"] = mgmtIP
			row.Values["CONTAINER_ID"] = containerID
			row.Values["AGE"] = age
			row.Values["CPU_UTIL"] = cpuUtil
			rows = append(rows, row)
		}
	}

	// Render output
	var sb strings.Builder
	_ = printer.Print(columns, rows, &sb)
	return sb.String(), nil
}

// ---- helpers ----

func getWekaClustersTyped(ctx context.Context, c crclient.Client, ns string, allNS bool, name string) ([]wekaapi.WekaCluster, error) {
	if name != "" {
		var wc wekaapi.WekaCluster
		err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &wc)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get WekaCluster %q in namespace %q: %w", name, ns, err)
		}
		return []wekaapi.WekaCluster{wc}, nil
	}

	var lst wekaapi.WekaClusterList
	opts := []crclient.ListOption{}
	if !allNS {
		opts = append(opts, crclient.InNamespace(ns))
	}
	if err := c.List(ctx, &lst, opts...); err != nil {
		return nil, fmt.Errorf("failed to list WekaCluster CRs: %w", err)
	}
	return lst.Items, nil
}

func filterClusterContainersTyped(all []wekaapi.WekaContainer, cluster *wekaapi.WekaCluster) []wekaapi.WekaContainer {
	if cluster == nil {
		return nil
	}

	clusterUID := string(cluster.GetUID())

	out := make([]wekaapi.WekaContainer, 0, len(all))
	for i := range all {
		wc := all[i]

		for _, o := range wc.GetOwnerReferences() {
			if o.Kind == "WekaCluster" && string(o.UID) == clusterUID {
				out = append(out, wc)
				break
			}
		}
	}

	return out
}
