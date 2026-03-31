package kubernetes

import (
	"context"
	"io"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
)

// GetPodLogs retrieves logs from a pod container
func GetPodLogs(ctx context.Context, clients *K8sClients, namespace, podName, containerName string) (string, error) {
	clientset := clients.Clientset

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Container: containerName,
	})

	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// PodHealthCheckOptions contains optional parameters for pod health validation
type PodHealthCheckOptions struct {
	RestartThreshold     int32         // Restart count threshold (default: 2)
	LastRestartThreshold time.Duration // Time threshold for last restart (default: 5 minutes)
}

// DefaultPodHealthCheckOptions returns the default pod health check options
func DefaultPodHealthCheckOptions() PodHealthCheckOptions {
	return PodHealthCheckOptions{
		RestartThreshold:     2,
		LastRestartThreshold: 5 * time.Minute,
	}
}

// IsPodUnhealthy checks if a pod is unhealthy based on restart count and last restart time.
// A pod is considered unhealthy if:
// - It has more restarts than the threshold, AND
// - The last restart was within the threshold time duration
//
// The function examines ALL containers (regular and init) to:
// - Sum up total restart count across all containers
// - Find the most recent termination time across all containers
//
// Parameters:
//
//	pod: The pod to check
//	opts: Optional parameters (uses defaults if nil)
//
// Returns true if the pod meets the unhealthy criteria
func IsPodUnhealthy(pod *v1.Pod, opts *PodHealthCheckOptions) bool {
	// Use defaults if options not provided
	if opts == nil {
		opts = &PodHealthCheckOptions{
			RestartThreshold:     2,
			LastRestartThreshold: 5 * time.Minute,
		}
	}

	// Calculate total restart count across all containers
	var totalRestarts int32

	// Sum restarts from regular containers
	if pod.Status.ContainerStatuses != nil {
		for i := range pod.Status.ContainerStatuses {
			totalRestarts += pod.Status.ContainerStatuses[i].RestartCount
		}
	}

	// Sum restarts from init containers
	if pod.Status.InitContainerStatuses != nil {
		for i := range pod.Status.InitContainerStatuses {
			totalRestarts += pod.Status.InitContainerStatuses[i].RestartCount
		}
	}

	// If restart count doesn't exceed threshold, pod is healthy
	if totalRestarts <= opts.RestartThreshold {
		return false
	}

	// Find the most recent termination time across all containers
	var lastRestartTime *v2.Time

	// Check regular containers for most recent termination
	if pod.Status.ContainerStatuses != nil {
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			if cs.LastTerminationState.Terminated != nil {
				finishedTime := &cs.LastTerminationState.Terminated.FinishedAt
				// Track the most recent termination time
				if lastRestartTime == nil || finishedTime.After(lastRestartTime.Time) {
					lastRestartTime = finishedTime
				}
			}
		}
	}

	// Check init containers for most recent termination
	if pod.Status.InitContainerStatuses != nil {
		for i := range pod.Status.InitContainerStatuses {
			cs := &pod.Status.InitContainerStatuses[i]
			if cs.LastTerminationState.Terminated != nil {
				finishedTime := &cs.LastTerminationState.Terminated.FinishedAt
				// Track the most recent termination time
				if lastRestartTime == nil || finishedTime.After(lastRestartTime.Time) {
					lastRestartTime = finishedTime
				}
			}
		}
	}

	// Pod is unhealthy if:
	// 1. Restart count exceeds threshold (already checked above), AND
	// 2. Last restart time is set (has actually restarted), AND
	// 3. Last restart was within the threshold duration
	if lastRestartTime != nil {
		thresholdTime := v2.Now().Add(-opts.LastRestartThreshold)
		if lastRestartTime.After(thresholdTime) {
			return true
		}
	}

	return false
}

// IsPodUnhealthyWithValues checks if a pod is unhealthy based on pre-computed restart count and last restart time.
// This is a wrapper around IsPodUnhealthy for cases where restart metrics have already been aggregated.
//
// It constructs a minimal pod object from the provided values and delegates to IsPodUnhealthy.
//
// Parameters:
//
//	restartCount: Total number of restarts across all containers
//	lastRestartTime: Timestamp of the most recent container termination (nil if never restarted)
//	opts: Optional parameters (uses defaults if nil)
//
// Returns true if the pod meets the unhealthy criteria
func IsPodUnhealthyWithValues(restartCount int32, lastRestartTime *v2.Time, opts *PodHealthCheckOptions) bool {
	// Construct a minimal pod object with the provided values
	pod := &v1.Pod{
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					RestartCount: restartCount,
				},
			},
		},
	}

	// Set last termination state only if lastRestartTime is not nil
	if lastRestartTime != nil {
		pod.Status.ContainerStatuses[0].LastTerminationState = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				FinishedAt: *lastRestartTime,
			},
		}
	}

	// Delegate to the main function
	return IsPodUnhealthy(pod, opts)
}

// PodRestartMetrics holds restart-related metrics for a pod
type PodRestartMetrics struct {
	RestartCount    int32    // Total number of restarts across all containers
	LastRestartTime *v2.Time // Timestamp of the most recent container termination
}

// GetPodRestartMetrics calculates restart metrics for a pod by examining all containers.
// It sums restart counts from all regular and init containers, and finds the most recent
// termination time across all containers.
//
// Parameters:
//
//	pod: The pod to analyze
//
// Returns PodRestartMetrics with aggregated restart information
func GetPodRestartMetrics(pod *v1.Pod) PodRestartMetrics {
	metrics := PodRestartMetrics{}

	// Sum restarts from regular containers
	if pod.Status.ContainerStatuses != nil {
		for i := range pod.Status.ContainerStatuses {
			metrics.RestartCount += pod.Status.ContainerStatuses[i].RestartCount
		}
	}

	// Sum restarts from init containers
	if pod.Status.InitContainerStatuses != nil {
		for i := range pod.Status.InitContainerStatuses {
			metrics.RestartCount += pod.Status.InitContainerStatuses[i].RestartCount
		}
	}

	// Find the most recent termination time across all containers
	// Check regular containers for most recent termination
	if pod.Status.ContainerStatuses != nil {
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			if cs.LastTerminationState.Terminated != nil {
				finishedTime := &cs.LastTerminationState.Terminated.FinishedAt
				// Track the most recent termination time
				if metrics.LastRestartTime == nil || finishedTime.After(metrics.LastRestartTime.Time) {
					metrics.LastRestartTime = finishedTime
				}
			}
		}
	}

	// Check init containers for most recent termination
	if pod.Status.InitContainerStatuses != nil {
		for i := range pod.Status.InitContainerStatuses {
			cs := &pod.Status.InitContainerStatuses[i]
			if cs.LastTerminationState.Terminated != nil {
				finishedTime := &cs.LastTerminationState.Terminated.FinishedAt
				// Track the most recent termination time
				if metrics.LastRestartTime == nil || finishedTime.After(metrics.LastRestartTime.Time) {
					metrics.LastRestartTime = finishedTime
				}
			}
		}
	}

	return metrics
}

// FirstInternalIP returns the first internal IP address of a node
func FirstInternalIP(node *v1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address
		}
	}
	return "<unknown>"
}

// Requests for a pod per scheduling semantics:
// - sum regular containers
// - init containers: take max
// - add overhead if present
func PodRequests(p *v1.Pod) (mem resource.Quantity, hp resource.Quantity) {
	sumMem := resource.MustParse("0")
	sumHP := resource.MustParse("0")

	for i := range p.Spec.Containers {
		c := &p.Spec.Containers[i]
		sumMem.Add(QuantityOrZero(c.Resources.Requests, v1.ResourceMemory))
		sumHP.Add(QuantityOrZero(c.Resources.Requests, "hugepages-2Mi"))
	}

	maxInitMem := resource.MustParse("0")
	maxInitHP := resource.MustParse("0")
	for i := range p.Spec.InitContainers {
		c := &p.Spec.InitContainers[i]
		m := QuantityOrZero(c.Resources.Requests, v1.ResourceMemory)
		h := QuantityOrZero(c.Resources.Requests, "hugepages-2Mi")
		if m.Cmp(maxInitMem) > 0 {
			maxInitMem = m
		}
		if h.Cmp(maxInitHP) > 0 {
			maxInitHP = h
		}
	}

	mem = sumMem.DeepCopy()
	if maxInitMem.Cmp(mem) > 0 {
		mem = maxInitMem.DeepCopy()
	}
	hp = sumHP.DeepCopy()
	if maxInitHP.Cmp(hp) > 0 {
		hp = maxInitHP.DeepCopy()
	}

	if p.Spec.Overhead != nil {
		if ov, ok := (p.Spec.Overhead)[v1.ResourceMemory]; ok {
			mem.Add(ov)
		}
		if ov, ok := (p.Spec.Overhead)["hugepages-2Mi"]; ok {
			hp.Add(ov)
		}
	}

	return mem, hp
}
