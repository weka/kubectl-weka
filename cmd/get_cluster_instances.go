package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
	includeNamespaceColumn := false
	if getClusterInstancesAllNamespaces {
		includeNamespaceColumn = true
	}

	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return err
	}

	currentNS, _, err := kubeCfg.Namespace()
	if err != nil {
		return err
	}
	if currentNS == "" {
		currentNS = "default"
	}
	if getClusterInstancesNamespace != "" {
		currentNS = getClusterInstancesNamespace
	}

	k8s, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	cachedClient, err := newWekaCRClient(ctx, restCfg)
	if err != nil {
		return err
	}
	defer cachedClient.Stop()

	crClient := cachedClient.Client

	var targetCluster string
	if len(args) == 1 {
		targetCluster = args[0]
		if getClusterInstancesAllNamespaces {
			return fmt.Errorf("cannot use -A/--all-namespaces when specifying a WekaCluster name; use -n to choose namespace")
		}
	}
	clusters, err := getWekaClustersTyped(ctx, crClient, currentNS, getClusterInstancesAllNamespaces, targetCluster)
	if err != nil {
		return err
	}
	if len(clusters) == 0 {
		if targetCluster != "" {
			fmt.Printf("WekaCluster %q not found.\n", targetCluster)
		} else if getClusterInstancesAllNamespaces {
			fmt.Println("No WekaCluster resources found.")
		} else {
			fmt.Printf("No WekaCluster resources found in namespace %q.\n", currentNS)
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()

	if !getClusterInstancesNoHeaders {
		if includeNamespaceColumn {
			if getClusterInstancesWide {
				fmt.Fprintln(w, "WEKACLUSTER\tNAMESPACE\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tMGMT_IP\tCONTAINER_ID\tAGE\tCPU_UTIL")
			} else {
				fmt.Fprintln(w, "WEKACLUSTER\tNAMESPACE\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tMGMT_IP\tCONTAINER_ID")
			}
		} else {
			if getClusterInstancesWide {
				fmt.Fprintln(w, "WEKACLUSTER\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tMGMT_IP\tCONTAINER_ID\tAGE\tCPU_UTIL")
			} else {
				fmt.Fprintln(w, "WEKACLUSTER\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tMGMT_IP\tCONTAINER_ID")
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
			return fmt.Errorf("failed to list WekaContainer CRs in namespace %q: %w", ns, err)
		}
		nsToContainers[ns] = contList.Items

		podList, err := k8s.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list pods in namespace %q: %w", ns, err)
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

			if getClusterInstancesWide {
				age := humanAge(now.Sub(wc.GetCreationTimestamp().Time))
				cpuUtil := ""
				if wc.Status.Stats != nil {
					cpuUtil = string(wc.Status.Stats.CpuUsage)
				} else {
					cpuUtil = "<none>"
				}
				if includeNamespaceColumn {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						clusterName, ns, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID, age, cpuUtil)
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						clusterName, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID, age, cpuUtil)
				}
			} else {
				if includeNamespaceColumn {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						clusterName, ns, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID)
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						clusterName, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID)
				}
			}
		}
	}

	return nil
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

func firstOrNone(xs []string) string {
	if len(xs) == 0 {
		return "<none>"
	}
	if strings.TrimSpace(xs[0]) == "" {
		return "<none>"
	}
	return xs[0]
}
