package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
	getClientInstancesWatch         bool
)

var getClientInstancesCmd = &cobra.Command{
	Use:   "client-instances [WEKACLIENT]",
	Short: "Display WEKA client instances and status (derived from WekaClient configuration)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGetClientInstances,
}

func init() {
	getCmd.AddCommand(getClientInstancesCmd)

	getClientInstancesCmd.Flags().BoolVarP(&getClientInstancesAllNamespaces, "all-namespaces", "A", false, "If present, list WekaClient resources across all namespaces")
	getClientInstancesCmd.Flags().StringVarP(&getClientInstancesNamespace, "namespace", "n", "", "Namespace. Defaults to current kubeconfig namespace")
	getClientInstancesCmd.Flags().BoolVar(&getClientInstancesNoHeaders, "no-headers", false, "Don't print headers")
	getClientInstancesCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")
	getClientInstancesCmd.Flags().BoolVar(&getClientInstancesWatch, "watch", false, "Watch for changes and recalculate information")

	getClientInstancesCmd.SilenceUsage = true
}

func runGetClientInstances(_ *cobra.Command, args []string) error {
	ctx := context.Background()

	currentNS, _, err := GetNamespaceFromFlags(getClientInstancesAllNamespaces, getClientInstancesNamespace)
	if err != nil {
		return err
	}
	var targetName string
	if len(args) == 1 {
		targetName = args[0]
		if getClientInstancesAllNamespaces {
			return fmt.Errorf("cannot use -A/--all-namespaces when specifying a WekaClient name; use -n to choose namespace")
		}
	}
	var hideColumnsList []string
	if !flagAllNamespaces {
		hideColumnsList = append(hideColumnsList, "NAMESPACE")
	}
	printer, _ := GetPrinterFromFlags(flagOutput, !getClientInstancesNoHeaders, hideColumnsList, false, 0, TableStyleMinimal)
	output, err := generateClientInstancesOutput(
		ctx,
		KubeClients,
		currentNS,
		getClientInstancesAllNamespaces,
		targetName,
		printer,
	)
	if err != nil {
		return err
	}

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
	printer ResourcePrinter,
) (string, error) {
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

	// Sort stable by ns/name
	sort.Slice(wekaClients, func(i, j int) bool {
		ai, aj := wekaClients[i], wekaClients[j]
		if ai.GetNamespace() != aj.GetNamespace() {
			return ai.GetNamespace() < aj.GetNamespace()
		}
		return ai.GetName() < aj.GetName()
	})

	// Define columns - include NAMESPACE only if showing all namespaces
	var columns []TableColumn
	columns = []TableColumn{
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "WEKACLIENT", VisibleInWide: false},
		{Name: "NODE", VisibleInWide: false},
		{Name: "WEKACONTAINER", VisibleInWide: false},
		{Name: "WC_STATUS", VisibleInWide: false},
		{Name: "POD_STATUS", VisibleInWide: false},
		{Name: "JOINED", VisibleInWide: false},
		{Name: "CONTAINER_ID", VisibleInWide: false},
		{Name: "MGMT_IPS", VisibleInWide: true},
		{Name: "MGMT_IP", VisibleInWide: false},
		{Name: "ACTIVE_MOUNTS", VisibleInWide: false},
		{Name: "CPU_UTIL", VisibleInWide: false},
		{Name: "NODE_SELECTOR", VisibleInWide: true},
	}

	// Build rows
	var rows []TableRow
	for _, wcClient := range wekaClients {
		clientNS := wcClient.GetNamespace()
		clientName := wcClient.GetName()

		selectorMap := wcClient.Spec.NodeSelector
		selectorStr := selectorMapToSelector(selectorMap)

		nodes, err := k8s.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selectorStr})
		if err != nil {
			return "", fmt.Errorf("failed to fetch nodes: %w", err)
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
			row := TableRow{Values: map[string]interface{}{}}
			row.Values["NAMESPACE"] = clientNS
			row.Values["WEKACLIENT"] = clientName
			row.Values["NODE"] = n.Name
			row.Values["WEKACONTAINER"] = wContName
			row.Values["WC_STATUS"] = wContStatus
			row.Values["POD_STATUS"] = podPhase
			row.Values["JOINED"] = joined
			row.Values["CONTAINER_ID"] = containerID
			row.Values["MGMT_IPS"] = mgmtIPsAll
			row.Values["MGMT_IP"] = mgmtIPShort
			row.Values["ACTIVE_MOUNTS"] = activeMounts
			row.Values["CPU_UTIL"] = cpuUtil
			row.Values["NODE_SELECTOR"] = selectorStr
			rows = append(rows, row)
		}
	}

	// Render output
	var sb strings.Builder
	_ = printer.Print(columns, rows, &sb)
	return sb.String() + "\n", nil
}
