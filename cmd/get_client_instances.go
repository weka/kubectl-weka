package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

var (
	getClientNodesAllNamespaces bool
	getClientNodesNamespace     string
	getClientNodesNoHeaders     bool
	getClientNodesWide          bool
)

var getClientNodesCmd = &cobra.Command{
	Use:   "client-instances [WEKACLIENT]",
	Short: "Display WEKA client instances and status (derived from WekaClient configuration)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGetClientNodes,
}

func init() {
	// assumes you already have: wekaCmd -> getCmd ("kubectl weka get")
	getCmd.AddCommand(getClientNodesCmd)

	getClientNodesCmd.Flags().BoolVarP(&getClientNodesAllNamespaces, "all-namespaces", "A", false, "If present, list WekaClient resources across all namespaces")
	getClientNodesCmd.Flags().StringVarP(&getClientNodesNamespace, "namespace", "n", "", "Namespace. Defaults to current kubeconfig namespace")
	getClientNodesCmd.Flags().BoolVar(&getClientNodesNoHeaders, "no-headers", false, "Don't print headers")
	getClientNodesCmd.Flags().BoolVar(&getClientNodesWide, "wide", false, "Wide output (adds selector and all mgmt IPs)")

	getClientNodesCmd.SilenceUsage = true
}

func runGetClientNodes(cmd *cobra.Command, args []string) error {
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
	if getClientNodesNamespace != "" {
		currentNS = getClientNodesNamespace
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

	wekaClientGVR, err := discoverGVR(disc,
		"weka.weka.io",
		[]string{"v1", "v1beta1", "v1alpha1"},
		[]string{"wekaclients"},
	)
	if err != nil {
		return fmt.Errorf("failed to discover WekaClient GVR: %w", err)
	}

	wekaContainerGVR, err := discoverGVR(disc,
		"weka.weka.io",
		[]string{"v1", "v1beta1", "v1alpha1"},
		[]string{"wekacontainers"},
	)
	if err != nil {
		return fmt.Errorf("failed to discover WekaContainer GVR: %w", err)
	}

	var targetName string
	if len(args) == 1 {
		targetName = args[0]
	}

	// ----- List/Get WekaClients -----
	var wekaClients []unstructured.Unstructured
	if targetName != "" {
		if getClientNodesAllNamespaces {
			return fmt.Errorf("cannot use -A/--all-namespaces when specifying a WekaClient name; use -n to choose namespace")
		}
		u, err := dc.Resource(wekaClientGVR).Namespace(currentNS).Get(ctx, targetName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get WekaClient %q in namespace %q: %w", targetName, currentNS, err)
		}
		wekaClients = []unstructured.Unstructured{*u}
	} else {
		var lst *unstructured.UnstructuredList
		var err error
		if getClientNodesAllNamespaces {
			lst, err = dc.Resource(wekaClientGVR).List(ctx, metav1.ListOptions{})
		} else {
			lst, err = dc.Resource(wekaClientGVR).Namespace(currentNS).List(ctx, metav1.ListOptions{})
		}
		if err != nil {
			return fmt.Errorf("failed to list WekaClient CRs: %w", err)
		}
		wekaClients = lst.Items
	}

	if len(wekaClients) == 0 {
		if targetName != "" {
			fmt.Printf("WekaClient %q not found.\n", targetName)
		} else if getClientNodesAllNamespaces {
			fmt.Println("No WekaClient resources found.")
		} else {
			fmt.Printf("No WekaClient resources found in namespace %q.\n", currentNS)
		}
		return nil
	}

	// Sort stable by ns/name
	sort.Slice(wekaClients, func(i, j int) bool {
		ai, aj := wekaClients[i], wekaClients[j]
		if ai.GetNamespace() != aj.GetNamespace() {
			return ai.GetNamespace() < aj.GetNamespace()
		}
		return ai.GetName() < aj.GetName()
	})

	// Output table
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()

	if !getClientNodesNoHeaders {
		if getClientNodesWide {
			fmt.Fprintln(w, "WEKACLIENT\tNODE\tNAMESPACE\tWEKACONTAINER\tWC_STATUS\tPOD\tJOINED\tCONTAINER_ID\tMGMT_IPS\tACTIVE_MOUNTS\tCPU_UTIL\tNODE_SELECTOR")
		} else {
			fmt.Fprintln(w, "WEKACLIENT\tNODE\tNAMESPACE\tWEKACONTAINER\tWC_STATUS\tPOD\tJOINED\tCONTAINER_ID\tMGMT_IP\tACTIVE_MOUNTS\tCPU_UTIL")
		}
	}

	// For each WekaClient, compute eligible nodes + join with WekaContainers and Pods
	for _, client := range wekaClients {
		clientNS := client.GetNamespace()
		clientName := client.GetName()

		selectorMap := getStringMap(client.Object, "spec", "nodeSelector")
		selectorStr := selectorMapToSelector(selectorMap)

		nodes, err := k8s.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selectorStr})
		if err != nil {
			// show a single error row for this client
			if getClientNodesWide {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					clientName, "<nodes?>", clientNS, "<none>", "FAIL", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", selectorStr)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					clientName, "<nodes?>", clientNS, "<none>", "FAIL", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a")
			}
			continue
		}

		for _, n := range nodes.Items {
			var (
				wcName       = "<none>"
				wcStatus     = "<missing>"
				podPhase     = "<missing>"
				joined       = "<none>"
				containerID  = "<none>"
				mgmtIPShort  = "<none>"
				mgmtIPsAll   = "<none>"
				activeMounts = "<none>"
				cpuUtil      = "<none>"
			)

			expectedWCName := fmt.Sprintf("%s-%s", clientName, n.Name)

			u, err := dc.Resource(wekaContainerGVR).Namespace(clientNS).Get(ctx, expectedWCName, metav1.GetOptions{})
			if err != nil {
				// Eligible node but WekaContainer not found (yet)
				wcName = expectedWCName
				wcStatus = "<missing>"
				podPhase = "<missing>"
			} else {
				wcName = u.GetName()
				wcStatus = inferWekaContainerStatus(u)
				joined = findConditionStatus(u, "JoinedCluster")
				containerID = getString(u.Object, "status", "containerID")

				ips := getStringSlice(u.Object, "status", "managementIPs")
				if len(ips) > 0 {
					mgmtIPsAll = strings.Join(ips, ",")
					mgmtIPShort = ips[0]
				}

				activeMounts = getString(u.Object, "status", "printer", "activeMounts")
				cpuUtil = getString(u.Object, "status", "stats", "cpuUtilization")

				// Pod has same name as the WekaContainer CR
				p, err := k8s.CoreV1().Pods(clientNS).Get(ctx, wcName, metav1.GetOptions{})
				if err == nil {
					podPhase = string(p.Status.Phase)
				} else {
					podPhase = "<not-found>"
				}
			}

			if getClientNodesWide {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					clientName, n.Name, clientNS, wcName, wcStatus, podPhase, joined, containerID, mgmtIPsAll, activeMounts, cpuUtil, selectorStr)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					clientName, n.Name, clientNS, wcName, wcStatus, podPhase, joined, containerID, mgmtIPShort, activeMounts, cpuUtil)
			}
		}
	}

	return nil
}
