package getters

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strings"
)

// GenerateClientInstancesOutput generates the client instances table as a string
func GenerateClientInstancesOutput(
	ctx context.Context,
	clients *kubernetes.K8sClients,
	namespace string,
	allNamespaces bool,
	targetName string,
	printerObj printer.ResourcePrinter,
) (string, error) {
	crClient := clients.CRClient
	k8s := clients.Clientset

	// ----- List/Get WekaClients (typed) -----
	var wekaClients []v1alpha1.WekaClient
	if targetName != "" {
		var wc v1alpha1.WekaClient
		err := crClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: targetName}, &wc)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Sprintf("WekaClient %q not found.\n", targetName), nil
			}
			return "", fmt.Errorf("failed to get WekaClient %q in namespace %q: %w", targetName, namespace, err)
		}
		wekaClients = []v1alpha1.WekaClient{wc}
	} else {
		var lst v1alpha1.WekaClientList
		opts := []client.ListOption{}
		if !allNamespaces {
			opts = append(opts, client.InNamespace(namespace))
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
	columns := []printer.TableColumn{
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
	var rows []printer.TableRow
	for _, wcClient := range wekaClients {
		clientNS := wcClient.GetNamespace()
		clientName := wcClient.GetName()

		selectorMap := wcClient.Spec.NodeSelector
		selectorStr := kubernetes.SelectorMapToSelector(selectorMap)

		// Use controller-runtime client instead of clientset for better caching
		var nodeList corev1.NodeList
		err := crClient.List(ctx, &nodeList, client.MatchingLabels(selectorMap))
		if err != nil {
			return "", fmt.Errorf("failed to fetch nodes: %w", err)
		}
		nodes := &nodeList

		for _, n := range nodes.Items {
			var (
				wContName    = "<none>"
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
			wContName = expectedWCName

			var wCont v1alpha1.WekaContainer
			err := crClient.Get(ctx, types.NamespacedName{Namespace: clientNS, Name: expectedWCName}, &wCont)
			if err != nil {
				if !errors.IsNotFound(err) {
					wcStatus = "<error>"
					podPhase = "<error>"
				}
			} else {
				wcStatus = GetWekaContainerStatus(&wCont)
				joined = FindConditionStatusTyped(wCont.Status.Conditions, "JoinedCluster")
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
				p, err := k8s.CoreV1().Pods(clientNS).Get(ctx, wContName, v1.GetOptions{})
				if err == nil {
					podPhase = string(p.Status.Phase)
				} else {
					podPhase = "<not-found>"
				}
			}
			row := printer.TableRow{Values: map[string]interface{}{}}
			row.Values["NAMESPACE"] = clientNS
			row.Values["WEKACLIENT"] = clientName
			row.Values["NODE"] = n.Name
			row.Values["WEKACONTAINER"] = wContName
			row.Values["WC_STATUS"] = wcStatus
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
	_ = printerObj.Print(columns, rows, &sb)
	return sb.String(), nil
}
