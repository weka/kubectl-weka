package logs

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// OperatorLogsOptions contains options for fetching operator logs
type OperatorLogsOptions struct {
	Namespace   string
	Follow      bool
	Tail        int64
	Since       time.Duration
	Previous    bool
	TailFlagSet bool // Indicates if --tail was explicitly set
}

// StreamOperatorLogs streams logs from the WEKA operator controller manager pod
func StreamOperatorLogs(ctx context.Context, clientset kubernetes.Interface, opts OperatorLogsOptions) error {
	// Labels for WEKA operator pod
	selector := "" +
		"app=weka-operator," +
		"app.kubernetes.io/component=weka-operator," +
		"app.kubernetes.io/created-by=weka-operator," +
		"control-plane=controller-manager"

	// List operator pods
	pods, err := clientset.CoreV1().Pods(opts.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return fmt.Errorf("failed to list operator pods in namespace %q: %w", opts.Namespace, err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no operator pods found in namespace %q with selector %q", opts.Namespace, selector)
	}

	// Prefer Running; among them prefer newest
	sort.Slice(pods.Items, func(i, j int) bool {
		pi, pj := pods.Items[i], pods.Items[j]
		ri := pi.Status.Phase == corev1.PodRunning
		rj := pj.Status.Phase == corev1.PodRunning
		if ri != rj {
			return ri
		}
		return pi.CreationTimestamp.Time.After(pj.CreationTimestamp.Time)
	})

	pod := pods.Items[0]
	container := PickControllerManagerContainer(&pod)

	// Print meta info to stderr (like kubectl does for some streaming tools)
	fmt.Fprintf(os.Stderr, "==> %s/%s (container=%s)\n", opts.Namespace, pod.Name, container)

	// Build log options
	logOpts := &corev1.PodLogOptions{
		Container: container,
		Follow:    opts.Follow,
		Previous:  opts.Previous,
	}

	// Only set TailLines if the user explicitly set --tail.
	// Negative means "all logs" => leave TailLines nil.
	if opts.TailFlagSet && opts.Tail >= 0 {
		logOpts.TailLines = &opts.Tail
	}

	// --since: use SinceSeconds (int64) for relative duration
	if opts.Since > 0 {
		sec := int64(opts.Since.Seconds())
		if sec <= 0 {
			sec = 1
		}
		logOpts.SinceSeconds = &sec
	}

	// Get log stream
	req := clientset.CoreV1().Pods(opts.Namespace).GetLogs(pod.Name, logOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to stream logs from pod %s/%s: %w", opts.Namespace, pod.Name, err)
	}
	defer stream.Close()

	// Preserve ANSI colors by copying bytes directly.
	_, err = io.Copy(os.Stdout, stream)
	if err != nil {
		return fmt.Errorf("log stream ended: %w", err)
	}
	return nil
}

// PickControllerManagerContainer selects the controller manager container from a pod
func PickControllerManagerContainer(pod *corev1.Pod) string {
	// Prefer common kubebuilder manager container names
	for _, want := range []string{"manager", "controller-manager", "weka-operator-controller-manager"} {
		for _, c := range pod.Spec.Containers {
			if c.Name == want {
				return c.Name
			}
		}
	}
	if len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Name
	}
	return "manager"
}
