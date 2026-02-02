// cmd/get_cluster_instances.go
package cmd

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
	// assumes you already have getCmd under wekaCmd
	getCmd.AddCommand(getClusterInstancesCmd)

	getClusterInstancesCmd.Flags().BoolVarP(&getClusterInstancesAllNamespaces, "all-namespaces", "A", false, "If present, list WekaCluster resources across all namespaces")
	getClusterInstancesCmd.Flags().StringVarP(&getClusterInstancesNamespace, "namespace", "n", "", "Namespace. Defaults to current kubeconfig namespace")
	getClusterInstancesCmd.Flags().BoolVar(&getClusterInstancesNoHeaders, "no-headers", false, "Don't print headers")
	getClusterInstancesCmd.Flags().BoolVar(&getClusterInstancesWide, "wide", false, "Wide output (adds AGE and CPU_UTIL)")

	getClusterInstancesCmd.SilenceUsage = true
}

func runGetClusterInstances(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

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
	if getClusterInstancesNamespace != "" {
		currentNS = getClusterInstancesNamespace
	}

	k8s, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	dc, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	disc, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return err
	}

	// Discover GVRs
	wekaClusterGVR, err := discoverGVR(disc, "weka.weka.io", []string{"v1", "v1beta1", "v1alpha1"}, []string{"wekaclusters"})
	if err != nil {
		return fmt.Errorf("failed to discover WekaCluster GVR: %w", err)
	}

	wekaContainerGVR, err := discoverGVR(disc, "weka.weka.io", []string{"v1", "v1beta1", "v1alpha1"}, []string{"wekacontainers"})
	if err != nil {
		return fmt.Errorf("failed to discover WekaContainer GVR: %w", err)
	}

	var targetCluster string
	if len(args) == 1 {
		targetCluster = args[0]
		if getClusterInstancesAllNamespaces {
			return fmt.Errorf("cannot use -A/--all-namespaces when specifying a WekaCluster name; use -n to choose namespace")
		}
	}

	// Get or list WekaClusters
	clusters, err := getWekaClusters(ctx, dc, wekaClusterGVR, currentNS, getClusterInstancesAllNamespaces, targetCluster)
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

	// Output table
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()

	if !getClusterInstancesNoHeaders {
		if getClusterInstancesWide {
			fmt.Fprintln(w, "WEKACLUSTER\tNAMESPACE\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tMGMT_IP\tCONTAINER_ID\tAGE\tCPU_UTIL")
		} else {
			fmt.Fprintln(w, "WEKACLUSTER\tNAMESPACE\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tMGMT_IP\tCONTAINER_ID")
		}
	}

	// We list WekaContainers per namespace once and pods per namespace once (fast)
	// Then for each cluster, filter containers by:
	//   - label weka.weka.io/cluster == clusterName (preferred if present)
	//   - else name prefix clusterName + "-"
	nsToContainers := map[string][]unstructured.Unstructured{}
	nsToPods := map[string]map[string]*corev1.Pod{}

	// Gather namespaces we need
	needNS := map[string]struct{}{}
	for _, c := range clusters {
		needNS[c.GetNamespace()] = struct{}{}
	}

	// Preload data per namespace
	for ns := range needNS {
		lst, err := dc.Resource(wekaContainerGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list WekaContainer CRs in namespace %q: %w", ns, err)
		}
		nsToContainers[ns] = lst.Items

		// Pods: list once, index by name
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

	// Print per cluster, sorted by ns/name
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].GetNamespace() != clusters[j].GetNamespace() {
			return clusters[i].GetNamespace() < clusters[j].GetNamespace()
		}
		return clusters[i].GetName() < clusters[j].GetName()
	})

	for _, cluster := range clusters {
		clusterName := cluster.GetName()
		clusterUid := cluster.GetUID()
		ns := cluster.GetNamespace()

		containers := nsToContainers[ns]
		podsByName := nsToPods[ns]

		// Filter containers for this cluster
		matching := filterClusterContainers(containers, clusterUid)

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
			wcStatus := inferWekaContainerStatus(&wc)

			mgmtIP := firstOrNone(getStringSlice(wc.Object, "status", "managementIPs"))
			containerID := getString(wc.Object, "status", "containerID")

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
				cpuUtil := getString(wc.Object, "status", "stats", "cpuUtilization")
				if cpuUtil == "" {
					cpuUtil = "<none>"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					clusterName, ns, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID, age, cpuUtil)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					clusterName, ns, nodeName, wcName, wcStatus, podPhase, mgmtIP, containerID)
			}
		}

		// If no containers match, still show a single row? (kubectl usually prints nothing)
	}

	return nil
}

//
// ---- helpers ----
//

func getWekaClusters(ctx context.Context, dc dynamic.Interface, gvr schema.GroupVersionResource, ns string, allNS bool, name string) ([]unstructured.Unstructured, error) {
	if name != "" {
		u, err := dc.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get WekaCluster %q in namespace %q: %w", name, ns, err)
		}
		return []unstructured.Unstructured{*u}, nil
	}

	var lst *unstructured.UnstructuredList
	var err error
	if allNS {
		lst, err = dc.Resource(gvr).List(ctx, metav1.ListOptions{})
	} else {
		lst, err = dc.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list WekaCluster CRs: %w", err)
	}
	return lst.Items, nil
}

func filterClusterContainers(all []unstructured.Unstructured, clusterUid types.UID) []unstructured.Unstructured {
	var out []unstructured.Unstructured

	for i := range all {
		u := all[i]

		// Prefer label match if present
		lbls := u.GetLabels()
		if v := lbls["weka.io/cluster-id"]; v != "" {
			if v == string(clusterUid) {
				out = append(out, u)
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
