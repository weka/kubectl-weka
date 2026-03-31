package getters

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	v2 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"
	"strings"
	"time"
)

// CSIInstanceInfo holds information about a CSI pod instance
type CSIInstanceInfo struct {
	DriverName      string
	Namespace       string
	NodeName        string
	Role            string
	PodName         string
	PodStatus       string
	RestartCount    int32
	LastRestartTime *v1.Time
	CreatedTime     v1.Time
}

// GenerateCSIInstancesOutput generates the CSI instances table as a string
func GenerateCSIInstancesOutput(ctx context.Context, clients *kubernetes.K8sClients, driverName, namespace, roleFilter string, unhealthy bool, printerObj printer.ResourcePrinter) (string, error) {
	crClient := clients.CRClient

	// List all CSIDriver resources (cluster-wide, non-namespaced)
	var csiDriverList v2.CSIDriverList
	if err := crClient.List(ctx, &csiDriverList); err != nil {
		return "", fmt.Errorf("failed to list CSIDrivers: %w", err)
	}

	// Filter for weka.io CSI drivers
	var wekaCsiDrivers []v2.CSIDriver
	for _, driver := range csiDriverList.Items {
		if kubernetes.IsWekaCSI(driver.Name) {
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
		if !IsPodBelongsToCSIDriver(&pod, driverMap) {
			continue
		}

		// Get the CSI driver name from the pod
		csiDriver := kubernetes.GetCSIDriverFromPod(&pod)
		if csiDriver == "" {
			continue
		}

		// Determine the pod role (controller or node)
		role := DeterminePodRole(&pod)
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
		metrics := kubernetes.GetPodRestartMetrics(&pod)
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

	// Apply unhealthy filter (>1 restart in last 5 minutes)
	if unhealthy {
		var healthyInstances []CSIInstanceInfo
		opts := kubernetes.DefaultPodHealthCheckOptions()

		for _, inst := range instances {
			// Use the utility function to check if pod is unhealthy with pre-computed values
			if kubernetes.IsPodUnhealthyWithValues(inst.RestartCount, inst.LastRestartTime, &opts) {
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
			return kubernetes.CompareNodeNames(instances[i].DriverName, instances[j].DriverName) < 0
		}
		if instances[i].Namespace != instances[j].Namespace {
			return instances[i].Namespace < instances[j].Namespace
		}
		if instances[i].NodeName != instances[j].NodeName {
			return kubernetes.CompareNodeNames(instances[i].NodeName, instances[j].NodeName) < 0
		}
		if instances[i].Role != instances[j].Role {
			// "controller" before "node"
			return instances[i].Role == "controller"
		}
		return instances[i].PodName < instances[j].PodName
	})

	// Define columns
	columns := []printer.TableColumn{
		{Name: "CSI DRIVER", VisibleInWide: false},
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "NODE", VisibleInWide: false},
		{Name: "ROLE", VisibleInWide: false},
		{Name: "POD NAME", VisibleInWide: false},
		{Name: "STATUS", VisibleInWide: false},
		{Name: "RESTARTS", VisibleInWide: true},
		{Name: "LAST RESTART", VisibleInWide: true, FormatFuncs: printer.TableFormatFunctions{utils.HumanAge}},
		{Name: "AGE", VisibleInWide: false, FormatFuncs: printer.TableFormatFunctions{utils.HumanAge}},
	}
	// Build rows
	var rows []printer.TableRow
	for _, info := range instances {
		var lastRestart time.Time
		if info.LastRestartTime != nil {
			lastRestart = info.LastRestartTime.Time
		}
		age := info.CreatedTime.Time
		row := printer.TableRow{Values: map[string]interface{}{
			"CSI DRIVER":   info.DriverName,
			"NAMESPACE":    info.Namespace,
			"NODE":         info.NodeName,
			"ROLE":         info.Role,
			"POD NAME":     info.PodName,
			"STATUS":       info.PodStatus,
			"RESTARTS":     info.RestartCount,
			"LAST RESTART": lastRestart,
			"AGE":          age,
		}}
		rows = append(rows, row)
	}

	// Render output
	var sb strings.Builder
	_ = printerObj.Print(columns, rows, &sb)
	return sb.String(), nil
}

// IsPodBelongsToCSIDriver checks if a pod belongs to any of the CSI drivers
// by checking if the CSI_DRIVER_NAME environment variable matches one of the provided drivers
func IsPodBelongsToCSIDriver(pod *corev1.Pod, driverMap map[string]bool) bool {
	// Get the CSI driver name from the pod
	driverName := kubernetes.GetCSIDriverFromPod(pod)
	if driverName == "" {
		return false
	}

	// Check if the driver is in our map
	return driverMap[driverName]
}

// DeterminePodRole determines if a pod is a controller or node instance
func DeterminePodRole(pod *corev1.Pod) string {
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
