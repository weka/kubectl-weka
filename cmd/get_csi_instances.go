package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	getCSIInstancesNamespace string
	getCSIInstancesRole      string
	getCSIInstancesWide      bool
	getCSIInstancesUnhealthy bool
)

var getCSIInstancesCmd = &cobra.Command{
	Use:   "csi-instances [DRIVER_NAME]",
	Short: "List CSI driver pods (controller and node instances)",
	Long: `Lists CSI driver pods showing deployment status, restart counts, and pod information.

Arguments:
  DRIVER_NAME      Optional: Show only a specific CSI driver by name

Flags:
  -n, --namespace <string>  Filter by Kubernetes namespace (shows all namespaces if not set)
  -r, --role <string>       Filter by pod role: 'controller' or 'node' (shows both if not set)
  -w, --wide                Show additional column: last restart time
  --unhealthy              Show only pods with frequent restarts (>1 restart in last 5 minutes)

Output Columns (default):
  CSI DRIVER     - CSI driver name
  NAMESPACE      - Kubernetes namespace where pod is deployed
  NODE           - Kubernetes node where pod is running
  ROLE           - Pod role: 'controller' or 'node'
  POD NAME       - Name of the CSI pod
  STATUS         - Pod status from container state (Running, CrashLoopBackoff, ImagePullBackOff, etc.)
  RESTARTS       - Number of pod container restarts
  AGE            - Time since pod was created

Wide columns (--wide):
  LAST RESTART   - Time since last pod container restart (N/A if never restarted)
`,
	RunE: runGetCSIInstances,
}

func init() {
	getCmd.AddCommand(getCSIInstancesCmd)

	getCSIInstancesCmd.Flags().StringVarP(&getCSIInstancesNamespace, "namespace", "n", "", "Filter by Kubernetes namespace")
	getCSIInstancesCmd.Flags().StringVarP(&getCSIInstancesRole, "role", "r", "", "Filter by pod role (controller or node)")
	getCSIInstancesCmd.Flags().BoolVarP(&getCSIInstancesWide, "wide", "w", false, "Show additional columns (last restart time)")
	getCSIInstancesCmd.Flags().BoolVar(&getCSIInstancesUnhealthy, "unhealthy", false, "Show only pods with frequent restarts (>1 restart in last 5 minutes)")
	getCSIInstancesCmd.SilenceUsage = true
}

func runGetCSIInstances(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Extract optional driver name argument
	var driverName string
	if len(args) > 0 {
		driverName = args[0]
	}

	// Validate role filter if provided
	if getCSIInstancesRole != "" && getCSIInstancesRole != "controller" && getCSIInstancesRole != "node" {
		return fmt.Errorf("invalid role: must be 'controller' or 'node'")
	}

	// Generate the output
	output, err := generateCSIInstancesOutput(ctx, KubeClients, driverName, getCSIInstancesNamespace, getCSIInstancesRole, getCSIInstancesWide, getCSIInstancesUnhealthy)
	if err != nil {
		return err
	}

	// Print the output
	fmt.Print(output)
	return nil
}

// CSIInstanceInfo holds information about a CSI pod instance
type CSIInstanceInfo struct {
	DriverName      string
	Namespace       string
	NodeName        string
	Role            string
	PodName         string
	PodStatus       string
	RestartCount    int32
	LastRestartTime *metav1.Time
	CreatedTime     metav1.Time
}

// generateCSIInstancesOutput generates the CSI instances table as a string
func generateCSIInstancesOutput(ctx context.Context, clients *K8sClients, driverName, namespace, roleFilter string, wide, unhealthy bool) (string, error) {
	crClient := clients.CRClient

	// List all CSIDriver resources (cluster-wide, non-namespaced)
	var csiDriverList storagev1.CSIDriverList
	if err := crClient.List(ctx, &csiDriverList); err != nil {
		return "", fmt.Errorf("failed to list CSIDrivers: %w", err)
	}

	// Filter for weka.io CSI drivers
	var wekaCsiDrivers []storagev1.CSIDriver
	for _, driver := range csiDriverList.Items {
		if isWekaCSI(driver.Name) {
			// If driverName is specified, only include matching driver
			if driverName != "" && driver.Name != driverName {
				continue
			}
			wekaCsiDrivers = append(wekaCsiDrivers, driver)
		}
	}

	if len(wekaCsiDrivers) == 0 {
		return "No CSI drivers found.\n", nil
	}

	// List all pods across all namespaces
	var podList corev1.PodList
	if err := crClient.List(ctx, &podList); err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	// Build a map of driver names for quick lookup
	driverMap := make(map[string]bool)
	for _, driver := range wekaCsiDrivers {
		driverMap[driver.Name] = true
	}

	// Extract CSI instances from pods
	var instances []CSIInstanceInfo

	for _, pod := range podList.Items {
		// Skip pods that don't belong to any of our CSI drivers
		if !isPodBelongsToCSIDriver(&pod, driverMap) {
			continue
		}

		// Get the CSI driver name from the pod
		csiDriver := getCSIDriverFromPod(&pod)
		if csiDriver == "" {
			continue
		}

		// Determine the pod role (controller or node)
		role := determinePodRole(&pod)
		if role == "" {
			continue
		}

		// Apply namespace filter
		if namespace != "" && pod.Namespace != namespace {
			continue
		}

		// Apply role filter
		if roleFilter != "" && role != roleFilter {
			continue
		}

		// Get restart metrics using utility function
		metrics := GetPodRestartMetrics(&pod)
		restartCount := metrics.RestartCount
		lastRestartTime := metrics.LastRestartTime

		podStatus := string(pod.Status.Phase) // Default to phase
		var containerStatus *corev1.ContainerStatus

		// Get first container for pod status detection
		if len(pod.Status.ContainerStatuses) > 0 {
			containerStatus = &pod.Status.ContainerStatuses[0]
		} else if len(pod.Status.InitContainerStatuses) > 0 {
			// If no regular container status yet, use init container for pod status detection
			if pod.Status.InitContainerStatuses[0].State.Waiting != nil {
				podStatus = pod.Status.InitContainerStatuses[0].State.Waiting.Reason
			}
		}

		// Get actual pod status from container state
		if containerStatus != nil {
			podStatus = getPodActualStatus(&pod, containerStatus)
		}

		instances = append(instances, CSIInstanceInfo{
			DriverName:      csiDriver,
			Namespace:       pod.Namespace,
			NodeName:        pod.Spec.NodeName,
			Role:            role,
			PodName:         pod.Name,
			PodStatus:       podStatus,
			RestartCount:    restartCount,
			LastRestartTime: lastRestartTime,
			CreatedTime:     pod.CreationTimestamp,
		})
	}

	if len(instances) == 0 {
		return "No CSI instances found.\n", nil
	}

	// Apply unhealthy filter (>1 restart in last 5 minutes)
	if unhealthy {
		var healthyInstances []CSIInstanceInfo
		opts := DefaultPodHealthCheckOptions()

		for _, inst := range instances {
			// Use the utility function to check if pod is unhealthy with pre-computed values
			if IsPodUnhealthyWithValues(inst.RestartCount, inst.LastRestartTime, &opts) {
				healthyInstances = append(healthyInstances, inst)
			}
		}
		instances = healthyInstances

		if len(instances) == 0 {
			return "No unhealthy CSI instances found (no pods with >1 restart in last 5 minutes).\n", nil
		}
	}

	// Sort by driver name, then namespace, then node, then role, then pod name
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].DriverName != instances[j].DriverName {
			return compareNodeNames(instances[i].DriverName, instances[j].DriverName) < 0
		}
		if instances[i].Namespace != instances[j].Namespace {
			return instances[i].Namespace < instances[j].Namespace
		}
		if instances[i].NodeName != instances[j].NodeName {
			return compareNodeNames(instances[i].NodeName, instances[j].NodeName) < 0
		}
		if instances[i].Role != instances[j].Role {
			// "controller" before "node"
			return instances[i].Role == "controller"
		}
		return instances[i].PodName < instances[j].PodName
	})

	// Generate table output
	t := table.NewWriter()
	styleTableMinimal(t)

	// Set header
	if wide {
		t.AppendHeader(table.Row{"CSI DRIVER", "NAMESPACE", "NODE", "ROLE", "POD NAME", "STATUS", "RESTARTS", "LAST RESTART", "AGE"})
	} else {
		t.AppendHeader(table.Row{"CSI DRIVER", "NAMESPACE", "NODE", "ROLE", "POD NAME", "STATUS", "RESTARTS", "AGE"})
	}

	// Append rows
	for _, info := range instances {
		age := humanAge(metav1.Now().Sub(info.CreatedTime.Time))

		if wide {
			lastRestart := "N/A"
			if info.LastRestartTime != nil {
				lastRestart = humanAge(metav1.Now().Sub(info.LastRestartTime.Time))
			}
			t.AppendRow(table.Row{
				info.DriverName,
				info.Namespace,
				info.NodeName,
				info.Role,
				info.PodName,
				info.PodStatus,
				info.RestartCount,
				lastRestart,
				age,
			})
		} else {
			t.AppendRow(table.Row{
				info.DriverName,
				info.Namespace,
				info.NodeName,
				info.Role,
				info.PodName,
				info.PodStatus,
				info.RestartCount,
				age,
			})
		}
	}

	return t.Render() + "\n", nil
}

// getCSIDriverFromPod extracts the CSI driver name from a pod
// Returns empty string if CSI_DRIVER_NAME env var is not set
func getCSIDriverFromPod(pod *corev1.Pod) string {
	if len(pod.Spec.Containers) == 0 {
		return ""
	}

	container := &pod.Spec.Containers[0]
	if container.Env == nil {
		return ""
	}

	for _, envVar := range container.Env {
		if envVar.Name == "CSI_DRIVER_NAME" {
			return envVar.Value
		}
	}

	return ""
}

// isPodBelongsToCSIDriver checks if a pod belongs to any of the CSI drivers
// by checking if the CSI_DRIVER_NAME environment variable matches one of the provided drivers
func isPodBelongsToCSIDriver(pod *corev1.Pod, driverMap map[string]bool) bool {
	// Get the CSI driver name from the pod
	driverName := getCSIDriverFromPod(pod)
	if driverName == "" {
		return false
	}

	// Check if the driver is in our map
	return driverMap[driverName]
}

// determinePodRole determines if a pod is a controller or node instance
func determinePodRole(pod *corev1.Pod) string {
	labels := pod.GetLabels()

	// Check for component label
	if component, ok := labels["app.kubernetes.io/component"]; ok {
		if strings.Contains(component, "controller") {
			return "controller"
		}
		if strings.Contains(component, "node") {
			return "node"
		}
	}

	// Check pod name patterns
	podName := pod.Name
	if strings.Contains(podName, "controller") || strings.Contains(podName, "csi-controller") {
		return "controller"
	}
	if strings.Contains(podName, "node") || strings.Contains(podName, "csi-node") {
		return "node"
	}

	// Default detection based on ownership (DaemonSet vs Deployment)
	// DaemonSets are typically used for node components
	// Deployments are typically used for controller components
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return "node"
		}
		if ref.Kind == "Deployment" {
			return "controller"
		}
	}

	return ""
}

// getPodActualStatus determines the actual pod status by checking container state
// This returns more accurate status than pod.Status.Phase because it catches
// CrashLoopBackoff, ImagePullBackOff, and other container-level issues
func getPodActualStatus(pod *corev1.Pod, containerStatus *corev1.ContainerStatus) string {
	// Check if container is waiting (CrashLoopBackoff, ImagePullBackOff, etc.)
	if containerStatus.State.Waiting != nil {
		reason := containerStatus.State.Waiting.Reason
		if reason != "" {
			return reason
		}
	}

	// Check if container is terminated
	if containerStatus.State.Terminated != nil {
		reason := containerStatus.State.Terminated.Reason
		if reason != "" {
			return reason
		}
	}

	// Check if container is running
	if containerStatus.State.Running != nil {
		return "Running"
	}

	// Fall back to pod phase
	return string(pod.Status.Phase)
}
