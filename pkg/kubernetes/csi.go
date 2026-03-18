package kubernetes

import "k8s.io/api/core/v1"

// GetCSIDriverFromPod extracts the CSI driver name from a pod
// Returns empty string if CSI_DRIVER_NAME env var is not set
func GetCSIDriverFromPod(pod *v1.Pod) string {
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
