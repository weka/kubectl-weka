package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
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

	var targetName string
	if len(args) == 1 {
		targetName = args[0]
		if getClientInstancesAllNamespaces {
			return fmt.Errorf("cannot use -A/--all-namespaces when specifying a WekaClient name; use -n to choose namespace")
		}
	}

	// Generate the output
	output, err := generateClientInstancesOutput(
		ctx,
		KubeClients,
		currentNS,
		getClientInstancesAllNamespaces,
		targetName,
		getClientInstancesNoHeaders,
		getClientInstancesWide,
	)
	if err != nil {
		return err
	}

	// Print the output
	fmt.Print(output)
	return nil
}

// generateClientInstancesOutput generates the client instances table as a string
func generateClientInstancesOutput(
	ctx context.Context,
	clients *K8sClients,
	namespace string,
	allNamespaces bool,
	targetName string,
	noHeaders bool,
	wide bool,
) (string, error) {
	includeNamespaceColumn := allNamespaces

	crClient := clients.CRClient
	k8s := clients.Clientset

	// ----- List/Get WekaClients (typed) -----
	var wekaClients []wekaapi.WekaClient
	if targetName != "" {
		var wc wekaapi.WekaClient
		err := crClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: targetName}, &wc)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Sprintf("WekaClient %q not found.\n", targetName), nil
			}
			return "", fmt.Errorf("failed to get WekaClient %q in namespace %q: %w", targetName, namespace, err)
		}
		wekaClients = []wekaapi.WekaClient{wc}
	} else {
		var lst wekaapi.WekaClientList
		opts := []crclient.ListOption{}
		if !allNamespaces {
			opts = append(opts, crclient.InNamespace(namespace))
		}
		err := crClient.List(ctx, &lst, opts...)
		if err != nil {
			return "", fmt.Errorf("failed to list WekaClient CRs: %w", err)
		}
		wekaClients = lst.Items
	}

	if len(wekaClients) == 0 {
		if targetName != "" {
			return fmt.Sprintf("WekaClient %q not found.\n", targetName), nil
		} else if allNamespaces {
			return "No WekaClient resources found.\n", nil
		} else {
			return fmt.Sprintf("No WekaClient resources found in namespace %q.\n", namespace), nil
		}
	}

	// Sort stable by ns/name
	sort.Slice(wekaClients, func(i, j int) bool {
		ai, aj := wekaClients[i], wekaClients[j]
		if ai.GetNamespace() != aj.GetNamespace() {
			return ai.GetNamespace() < aj.GetNamespace()
		}
		return ai.GetName() < aj.GetName()
	})

	var output strings.Builder
	t := table.NewWriter()
	styleTableMinimal(t)
	t.SetOutputMirror(&output)

	if !noHeaders {
		if includeNamespaceColumn {
			if wide {
				t.AppendHeader(table.Row{"NAMESPACE", "WEKACLIENT", "NODE", "WEKACONTAINER", "WC_STATUS", "POD", "JOINED", "CONTAINER_ID", "MGMT_IPS", "ACTIVE_MOUNTS", "CPU_UTIL", "NODE_SELECTOR"})
			} else {
				t.AppendHeader(table.Row{"NAMESPACE", "WEKACLIENT", "NODE", "WEKACONTAINER", "WC_STATUS", "POD", "JOINED", "CONTAINER_ID", "MGMT_IP", "ACTIVE_MOUNTS", "CPU_UTIL"})
			}
		} else {
			if wide {
				t.AppendHeader(table.Row{"WEKACLIENT", "NODE", "WEKACONTAINER", "WC_STATUS", "POD", "JOINED", "CONTAINER_ID", "MGMT_IPS", "ACTIVE_MOUNTS", "CPU_UTIL", "NODE_SELECTOR"})
			} else {
				t.AppendHeader(table.Row{"WEKACLIENT", "NODE", "WEKACONTAINER", "WC_STATUS", "POD", "JOINED", "CONTAINER_ID", "MGMT_IP", "ACTIVE_MOUNTS", "CPU_UTIL"})
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
			if includeNamespaceColumn {
				if wide {
					t.AppendRow(table.Row{clientNS, clientName, "<nodes?>", "<none>", "FAIL", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", selectorStr})
				} else {
					t.AppendRow(table.Row{clientNS, clientName, "<nodes?>", "<none>", "FAIL", "n/a", "n/a", "n/a", "n/a", "n/a"})
				}
			} else {
				if wide {
					t.AppendRow(table.Row{clientName, "<nodes?>", "<none>", "FAIL", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a"})
				} else {
					t.AppendRow(table.Row{clientName, "<nodes?>", "<none>", "FAIL", "n/a", "n/a", "n/a", "n/a", "n/a"})
				}
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
				if wide {
					t.AppendRow(table.Row{clientNS, clientName, n.Name, wContName, wContStatus, podPhase, joined, containerID, mgmtIPsAll, activeMounts, cpuUtil, selectorStr})
				} else {
					t.AppendRow(table.Row{clientNS, clientName, n.Name, wContName, wContStatus, podPhase, joined, containerID, mgmtIPShort, activeMounts, cpuUtil})
				}
			} else {
				if wide {
					t.AppendRow(table.Row{clientName, n.Name, wContName, wContStatus, podPhase, joined, containerID, mgmtIPsAll, activeMounts, cpuUtil, selectorStr})
				} else {
					t.AppendRow(table.Row{clientName, n.Name, wContName, wContStatus, podPhase, joined, containerID, mgmtIPShort, activeMounts, cpuUtil})
				}
			}
		}
	}

	t.Render()
	return output.String() + "\n", nil
}
