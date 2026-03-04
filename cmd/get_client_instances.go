package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

var (
	getClientInstancesAllNamespaces bool
	getClientInstancesNamespace     string
	getClientInstancesNoHeaders     bool
	getClientInstancesWide          bool
	getClientInstancesWatch         bool
)

var getClientInstancesCmd = &cobra.Command{
	Use:   "client-instances [WEKACLIENT]",
	Short: "Display WEKA client instances and status (derived from WekaClient configuration)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGetClientNodes,
}

func init() {
	getCmd.AddCommand(getClientInstancesCmd)

	getClientInstancesCmd.Flags().BoolVarP(&getClientInstancesAllNamespaces, "all-namespaces", "A", false, "If present, list WekaClient resources across all namespaces")
	getClientInstancesCmd.Flags().StringVarP(&getClientInstancesNamespace, "namespace", "n", "", "Namespace. Defaults to current kubeconfig namespace")
	getClientInstancesCmd.Flags().BoolVar(&getClientInstancesNoHeaders, "no-headers", false, "Don't print headers")
	getClientInstancesCmd.Flags().BoolVar(&getClientInstancesWide, "wide", false, "Wide output (adds selector and all mgmt IPs)")
	getClientInstancesCmd.Flags().BoolVar(&getClientInstancesWatch, "watch", false, "Watch for changes and recalculate information")

	getClientInstancesCmd.SilenceUsage = true
}

func runGetClientNodes(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	currentNS, err := GetKubeNamespace()
	if err != nil {
		return err
	}
	if getClientInstancesNamespace != "" {
		currentNS = getClientInstancesNamespace
	}
	crClient := KubeClients.CRClient
	k8s := KubeClients.Clientset

	includeNamespaceColumn := getClientInstancesAllNamespaces
	var targetName string
	if len(args) == 1 {
		targetName = args[0]
	}

	// ----- List/Get WekaClients (typed) -----
	var wekaClients []wekaapi.WekaClient
	if targetName != "" {
		if getClientInstancesAllNamespaces {
			return fmt.Errorf("cannot use -A/--all-namespaces when specifying a WekaClient name; use -n to choose namespace")
		}
		var wc wekaapi.WekaClient
		err := crClient.Get(ctx, types.NamespacedName{Namespace: currentNS, Name: targetName}, &wc)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to get WekaClient %q in namespace %q: %w", targetName, currentNS, err)
		}
		wekaClients = []wekaapi.WekaClient{wc}
	} else {
		var lst wekaapi.WekaClientList
		opts := []crclient.ListOption{}
		if !getClientInstancesAllNamespaces {
			opts = append(opts, crclient.InNamespace(currentNS))
		}
		err := crClient.List(ctx, &lst, opts...)
		if err != nil {
			return fmt.Errorf("failed to list WekaClient CRs: %w", err)
		}
		wekaClients = lst.Items
	}

	if len(wekaClients) == 0 {
		if targetName != "" {
			fmt.Printf("WekaClient %q not found.\n", targetName)
		} else if getClientInstancesAllNamespaces {
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

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()

	if !getClientInstancesNoHeaders {
		if includeNamespaceColumn {
			if getClientInstancesWide {
				fmt.Fprintln(w, "NAMESPACE\tWEKACLIENT\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tJOINED\tCONTAINER_ID\tMGMT_IPS\tACTIVE_MOUNTS\tCPU_UTIL\tNODE_SELECTOR")
			} else {
				fmt.Fprintln(w, "NAMESPACE\tWEKACLIENT\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tJOINED\tCONTAINER_ID\tMGMT_IP\tACTIVE_MOUNTS\tCPU_UTIL")
			}
		} else {
			if getClientInstancesWide {
				fmt.Fprintln(w, "WEKACLIENT\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tJOINED\tCONTAINER_ID\tMGMT_IPS\tACTIVE_MOUNTS\tCPU_UTIL\tNODE_SELECTOR")
			} else {
				fmt.Fprintln(w, "WEKACLIENT\tNODE\tWEKACONTAINER\tWC_STATUS\tPOD\tJOINED\tCONTAINER_ID\tMGMT_IP\tACTIVE_MOUNTS\tCPU_UTIL")
			}
		}
	}

	// For each WekaClient, compute eligible nodes + join with WekaContainers and Pods
	for _, wcClient := range wekaClients {
		clientNS := wcClient.GetNamespace()
		clientName := wcClient.GetName()

		selectorMap := wcClient.Spec.NodeSelector
		selectorStr := selectorMapToSelector(selectorMap)

		nodes, err := k8s.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selectorStr})
		if err != nil {
			// show a single error row for this client
			if getClientInstancesWide {
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
				wContName    = "<none>"
				wContStatus  = "<missing>"
				podPhase     = "<missing>"
				joined       = "<none>"
				containerID  = "<none>"
				mgmtIPShort  = "<none>"
				mgmtIPsAll   = "<none>"
				activeMounts = "<none>"
				cpuUtil      = "<none>"
			)

			expectedWCName := fmt.Sprintf("%s-%s", clientName, n.Name)
			wContName = expectedWCName

			var wCont wekaapi.WekaContainer
			err := crClient.Get(ctx, types.NamespacedName{Namespace: clientNS, Name: expectedWCName}, &wCont)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					wContStatus = "<error>"
					podPhase = "<error>"
				}
			} else {
				wContStatus = inferWekaContainerStatusTyped(&wCont)
				joined = findConditionStatusTyped(wCont.Status.Conditions, "JoinedCluster")
				if wCont.Status.ClusterContainerID != nil {
					containerID = fmt.Sprintf("%d", *wCont.Status.ClusterContainerID)
				}

				ips := wCont.Status.GetManagementIps()
				if len(ips) > 0 {
					mgmtIPsAll = strings.Join(ips, ",")
					mgmtIPShort = ips[0]
				}

				activeMounts = string(wCont.Status.GetPrinterColumns().ActiveMounts)
				if wCont.Status.Stats != nil {
					cpuUtil = string(wCont.Status.Stats.CpuUsage)
				}

				// Pod has same name as the WekaContainer CR
				p, err := k8s.CoreV1().Pods(clientNS).Get(ctx, wContName, metav1.GetOptions{})
				if err == nil {
					podPhase = string(p.Status.Phase)
				} else {
					podPhase = "<not-found>"
				}
			}
			if includeNamespaceColumn {
				if getClientInstancesWide {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						clientNS, clientName, n.Name, wContName, wContStatus, podPhase, joined, containerID, mgmtIPsAll, activeMounts, cpuUtil, selectorStr)
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						clientNS, clientName, n.Name, wContName, wContStatus, podPhase, joined, containerID, mgmtIPShort, activeMounts, cpuUtil)
				}
			} else {
				if getClientInstancesWide {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						clientName, n.Name, wContName, wContStatus, podPhase, joined, containerID, mgmtIPsAll, activeMounts, cpuUtil, selectorStr)
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						clientName, n.Name, wContName, wContStatus, podPhase, joined, containerID, mgmtIPShort, activeMounts, cpuUtil)
				}
			}
		}
	}

	return nil
}
