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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
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
	if currentNS == "" {
		currentNS = "default"
	}
	if getClientNodesNamespace != "" {
		currentNS = getClientNodesNamespace
	}

	k8s, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	crClient, err := newWekaCRClient(ctx, restCfg)
	if err != nil {
		return err
	}

	var targetName string
	if len(args) == 1 {
		targetName = args[0]
	}

	// ----- List/Get WekaClients (typed) -----
	var wekaClients []wekaapi.WekaClient
	if targetName != "" {
		if getClientNodesAllNamespaces {
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
		if !getClientNodesAllNamespaces {
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
	for _, wcClient := range wekaClients {
		clientNS := wcClient.GetNamespace()
		clientName := wcClient.GetName()

		selectorMap := wcClient.Spec.NodeSelector
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
				cpuUtil = string(wCont.Status.Stats.CpuUsage)
				if cpuUtil == "" {
					cpuUtil = "<none>"
				}

				// Pod has same name as the WekaContainer CR
				p, err := k8s.CoreV1().Pods(clientNS).Get(ctx, wContName, metav1.GetOptions{})
				if err == nil {
					podPhase = string(p.Status.Phase)
				} else {
					podPhase = "<not-found>"
				}
			}

			if getClientNodesWide {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					clientName, n.Name, clientNS, wContName, wContStatus, podPhase, joined, containerID, mgmtIPsAll, activeMounts, cpuUtil, selectorStr)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					clientName, n.Name, clientNS, wContName, wContStatus, podPhase, joined, containerID, mgmtIPShort, activeMounts, cpuUtil)
			}
		}
	}

	return nil
}
