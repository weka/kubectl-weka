package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
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
	getClusterInstancesNoHeaders     bool
	getClusterInstancesWide          bool
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
	getClusterInstancesCmd.Flags().BoolVar(&getClusterInstancesNoHeaders, "no-headers", false, "Don't print headers")
	getClusterInstancesCmd.Flags().BoolVar(&getClusterInstancesWide, "wide", false, "Wide output (adds AGE and CPU_UTIL)")

	getClusterInstancesCmd.SilenceUsage = true
}

func runGetClusterInstances(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	currentNS, err := GetKubeNamespace()
	if err != nil {
		return err
	}
	if getClusterInstancesNamespace != "" {
		currentNS = getClusterInstancesNamespace
	}

	var targetCluster string
	if len(args) == 1 {
		targetCluster = args[0]
		if getClusterInstancesAllNamespaces {
			return fmt.Errorf("cannot use -A/--all-namespaces when specifying a WekaCluster name; use -n to choose namespace")
		}
	}

	// Generate the output
	output, err := generateClusterInstancesOutput(
		ctx,
		KubeClients,
		currentNS,
		getClusterInstancesAllNamespaces,
		targetCluster,
		getClusterInstancesNoHeaders,
		getClusterInstancesWide,
	)
	if err != nil {
		return err
	}

	// Print the output
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
	noHeaders bool,
	wide bool,
) (string, error) {
	includeNamespaceColumn := allNamespaces

	k8s := clients.Clientset
	crClient := clients.CRClient

	clusters, err := getWekaClustersTyped(ctx, crClient, namespace, allNamespaces, targetCluster)
	if err != nil {
		return "", err
	}
	if len(clusters) == 0 {
		if targetCluster != "" {
			return fmt.Sprintf("WekaCluster %q not found.\n", targetCluster), nil
		} else if allNamespaces {
			return "No WekaCluster resources found.\n", nil
		} else {
			return fmt.Sprintf("No WekaCluster resources found in namespace %q.\n", namespace), nil
		}
	}

	var output strings.Builder
	t := table.NewWriter()
	styleTableMinimal(t)
	t.SetOutputMirror(&output)

	if !noHeaders {
		if includeNamespaceColumn {
			if wide {
				t.AppendHeader(table.Row{"NAMESPACE", "WEKACLUSTER", "NODE", "WEKACONTAINER", "WC_STATUS", "POD", "MGMT_IP", "CONTAINER_ID", "AGE", "CPU_UTIL"})
			} else {
				t.AppendHeader(table.Row{"NAMESPACE", "WEKACLUSTER", "NODE", "WEKACONTAINER", "WC_STATUS", "POD", "MGMT_IP", "CONTAINER_ID"})
			}
		} else {
			if wide {
				t.AppendHeader(table.Row{"WEKACLUSTER", "NODE", "WEKACONTAINER", "WC_STATUS", "POD", "MGMT_IP", "CONTAINER_ID", "AGE", "CPU_UTIL"})
			} else {
				t.AppendHeader(table.Row{"WEKACLUSTER", "NODE", "WEKACONTAINER", "WC_STATUS", "POD", "MGMT_IP", "CONTAINER_ID"})
			}
		}
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

	now := time.Now()

	// Sort stable by ns/name
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].GetNamespace() != clusters[j].GetNamespace() {
			return clusters[i].GetNamespace() < clusters[j].GetNamespace()
		}
		return clusters[i].GetName() < clusters[j].GetName()
	})

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

			if wide {
				age := humanAge(now.Sub(wc.GetCreationTimestamp().Time))
				cpuUtil := ""
				if wc.Status.Stats != nil {
					cpuUtil = string(wc.Status.Stats.CpuUsage)
				} else {
					cpuUtil = "<none>"
				}
				if includeNamespaceColumn {
					t.AppendRow(table.Row{ns, clusterName, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID, age, cpuUtil})
				} else {
					t.AppendRow(table.Row{clusterName, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID, age, cpuUtil})
				}
			} else {
				if includeNamespaceColumn {
					t.AppendRow(table.Row{ns, clusterName, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID})
				} else {
					t.AppendRow(table.Row{clusterName, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID})
				}
			}
		}
	}

	t.Render()
	return output.String() + "\n", nil
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
