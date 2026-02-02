package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	operatorLogsNamespace string
	operatorLogsFollow    bool
	operatorLogsTail      int64
	operatorLogsSince     time.Duration
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View WEKA related logs",
}

var logsOperatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "Show logs of the WEKA operator controller manager",
	RunE:  runLogsOperator,
}

func init() {
	logsCmd.AddCommand(logsOperatorCmd)

	logsOperatorCmd.Flags().StringVarP(&operatorLogsNamespace, "namespace", "n", "weka-operator-system",
		"Namespace where the WEKA operator is running")

	logsOperatorCmd.Flags().BoolVarP(&operatorLogsFollow, "follow", "f", false,
		"Specify if the logs should be streamed")

	// kubectl default is usually -1 (all lines). We'll match that.
	logsOperatorCmd.Flags().Int64Var(&operatorLogsTail, "tail", -1,
		"Lines of recent log file to display. Defaults to -1 (all logs)")

	logsOperatorCmd.Flags().DurationVar(&operatorLogsSince, "since", 0,
		"Only return logs newer than a relative duration like 5s, 2m, or 3h")

	logsOperatorCmd.SilenceUsage = true
}

func runLogsOperator(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return err
	}

	// Labels you provided
	selector := "" +
		"app=weka-operator," +
		"app.kubernetes.io/component=weka-operator," +
		"app.kubernetes.io/created-by=weka-operator," +
		"control-plane=controller-manager"

	pods, err := clientset.CoreV1().Pods(operatorLogsNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return fmt.Errorf("failed to list operator pods in namespace %q: %w", operatorLogsNamespace, err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no operator pods found in namespace %q with selector %q", operatorLogsNamespace, selector)
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
	container := pickControllerManagerContainer(&pod)

	// Print meta info to stderr (like kubectl does for some streaming tools)
	fmt.Fprintf(os.Stderr, "==> %s/%s (container=%s)\n", operatorLogsNamespace, pod.Name, container)

	opts := &corev1.PodLogOptions{
		Container: container,
		Follow:    operatorLogsFollow,
	}

	// Only set TailLines if the user explicitly set --tail.
	// Negative means "all logs" => leave TailLines nil.
	if cmd.Flags().Changed("tail") && operatorLogsTail >= 0 {
		opts.TailLines = &operatorLogsTail
	}

	// --since: use SinceSeconds (int64) for relative duration
	if operatorLogsSince > 0 {
		sec := int64(operatorLogsSince.Seconds())
		if sec <= 0 {
			sec = 1
		}
		opts.SinceSeconds = &sec
	}

	req := clientset.CoreV1().Pods(operatorLogsNamespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to stream logs from pod %s/%s: %w", operatorLogsNamespace, pod.Name, err)
	}
	defer stream.Close()

	// Preserve ANSI colors by copying bytes directly.
	_, err = io.Copy(os.Stdout, stream)
	if err != nil {
		return fmt.Errorf("log stream ended: %w", err)
	}
	return nil
}

func pickControllerManagerContainer(pod *corev1.Pod) string {
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
