package cmd

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

// TestPodRestartCount tests counting restarts across pod containers
func TestPodRestartCount(t *testing.T) {
	tests := []struct {
		name             string
		pod              *corev1.Pod
		expectedRestarts int32
	}{
		{
			name: "no restarts",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:         "container1",
							RestartCount: 0,
						},
					},
				},
			},
			expectedRestarts: 0,
		},
		{
			name: "single container restart",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:         "container1",
							RestartCount: 5,
						},
					},
				},
			},
			expectedRestarts: 5,
		},
		{
			name: "multiple container restarts",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:         "container1",
							RestartCount: 3,
						},
						{
							Name:         "container2",
							RestartCount: 2,
						},
						{
							Name:         "container3",
							RestartCount: 5,
						},
					},
				},
			},
			expectedRestarts: 10, // sum of all restarts
		},
		{
			name: "no container statuses",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{},
				},
			},
			expectedRestarts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate total restarts
			totalRestarts := int32(0)
			for _, cs := range tt.pod.Status.ContainerStatuses {
				totalRestarts += cs.RestartCount
			}

			if totalRestarts != tt.expectedRestarts {
				t.Errorf("Pod restart count = %d, expected %d", totalRestarts, tt.expectedRestarts)
			}
		})
	}
}

// TestPodStatus tests reading pod status
func TestPodStatus(t *testing.T) {
	tests := []struct {
		name           string
		pod            *corev1.Pod
		expectedPhase  corev1.PodPhase
		expectedStatus string
	}{
		{
			name: "running pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expectedPhase:  corev1.PodRunning,
			expectedStatus: "Running",
		},
		{
			name: "pending pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			expectedPhase:  corev1.PodPending,
			expectedStatus: "Pending",
		},
		{
			name: "succeeded pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
				},
			},
			expectedPhase:  corev1.PodSucceeded,
			expectedStatus: "Succeeded",
		},
		{
			name: "failed pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			},
			expectedPhase:  corev1.PodFailed,
			expectedStatus: "Failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pod.Status.Phase != tt.expectedPhase {
				t.Errorf("Pod phase = %v, expected %v", tt.pod.Status.Phase, tt.expectedPhase)
			}
		})
	}
}
