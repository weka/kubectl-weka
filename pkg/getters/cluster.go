package getters

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/utils"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strings"
)

func GetWekaClusters(ctx context.Context, c client.Client, ns string, allNS bool, name string) ([]v1alpha1.WekaCluster, error) {
	if name != "" {
		var wc v1alpha1.WekaCluster
		err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &wc)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get WekaCluster %q in namespace %q: %w", name, ns, err)
		}
		return []v1alpha1.WekaCluster{wc}, nil
	}

	var lst v1alpha1.WekaClusterList
	opts := []client.ListOption{}
	if !allNS {
		opts = append(opts, client.InNamespace(ns))
	}
	if err := c.List(ctx, &lst, opts...); err != nil {
		return nil, fmt.Errorf("failed to list WekaCluster CRs: %w", err)
	}
	return lst.Items, nil
}

// GetClusterInstancesOutput generates the cluster instances table as a string
func GetClusterInstancesOutput(
	ctx context.Context,
	clients *kubernetes.K8sClients,
	namespace string,
	allNamespaces bool,
	targetCluster string,
	printerObj printer.ResourcePrinter,
) (string, error) {
	crClient := clients.CRClient

	clusters, err := GetWekaClusters(ctx, crClient, namespace, allNamespaces, targetCluster)
	if err != nil {
		return "", err
	}

	// Preload WekaContainers + Pods per namespace once
	needNS := map[string]struct{}{}
	for i := range clusters {
		needNS[clusters[i].GetNamespace()] = struct{}{}
	}

	nsToContainers := map[string][]v1alpha1.WekaContainer{}
	nsToPods := map[string]map[string]*v1.Pod{}

	for ns := range needNS {
		var contList v1alpha1.WekaContainerList
		if err := crClient.List(ctx, &contList, client.InNamespace(ns)); err != nil {
			return "", fmt.Errorf("failed to list WekaContainer CRs in namespace %q: %w", ns, err)
		}
		nsToContainers[ns] = contList.Items

		var podList v1.PodList
		err := crClient.List(ctx, &podList, client.InNamespace(ns))
		if err != nil {
			return "", fmt.Errorf("failed to list pods in namespace %q: %w", ns, err)
		}
		m := make(map[string]*v1.Pod, len(podList.Items))
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
	columns := []printer.TableColumn{
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "WEKACLUSTER", VisibleInWide: false},
		{Name: "NODE", VisibleInWide: false},
		{Name: "WEKACONTAINER", VisibleInWide: false},
		{Name: "WC_STATUS", VisibleInWide: false},
		{Name: "POD_STATUS", VisibleInWide: false},
		{Name: "MGMT_IP", VisibleInWide: false},
		{Name: "CONTAINER_ID", VisibleInWide: false},
		{Name: "CPU_UTIL", VisibleInWide: true},
		{Name: "AGE", VisibleInWide: true, FormatFuncs: printer.TableFormatFunctions{utils.HumanAge}},
	}

	// Build rows
	var rows []printer.TableRow
	for i := range clusters {
		cluster := &clusters[i]
		clusterName := cluster.GetName()
		ns := cluster.GetNamespace()

		containers := nsToContainers[ns]
		podsByName := nsToPods[ns]

		matching := kubernetes.FilterOwnerContainers(containers, cluster)

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
			wcStatus := GetWekaContainerStatus(&wc)

			ips := wc.Status.GetManagementIps()
			mgmtIP := utils.FirstOrNone(ips)

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

			row := printer.TableRow{Values: map[string]interface{}{}}
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
	_ = printerObj.Print(columns, rows, &sb)
	return sb.String(), nil
}
