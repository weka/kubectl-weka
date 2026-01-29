package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var getNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Get node information in Weka format",
	RunE:  runGetNodes,
}

func init() {
	getCmd.AddCommand(getNodesCmd)
}

func runGetNodes(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)

	if !flagNoHeaders {
		fmt.Fprintln(w,
			"NAME\tIP\tOS\tARCH\tKERNEL\tHP_SET\tHP_FREE\tCORES\tRAM\tCLTROLE\tBKNDROLE",
		)
	}

	for _, n := range nodes.Items {
		printNodeRow(w, &n)
	}

	w.Flush()
	return nil
}

func printNodeRow(w *tabwriter.Writer, n *corev1.Node) {
	name := n.Name
	ip := firstInternalIP(n)
	osImage := n.Status.NodeInfo.OSImage
	arch := n.Status.NodeInfo.Architecture
	kernel := n.Status.NodeInfo.KernelVersion

	hpSet := n.Status.Capacity["hugepages-2Mi"]
	hpFree := n.Status.Allocatable["hugepages-2Mi"]

	cores := n.Status.Capacity[corev1.ResourceCPU]
	ram := n.Status.Capacity[corev1.ResourceMemory]

	cltRole := n.Labels["weka.io/supports-clients"]
	bkndRole := n.Labels["weka.io/supports-backends"]

	if cltRole == "" {
		cltRole = "<none>"
	}
	if bkndRole == "" {
		bkndRole = "<none>"
	}

	fmt.Fprintf(
		w,
		"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		name,
		ip,
		osImage,
		arch,
		kernel,
		hpSet.String(),
		hpFree.String(),
		cores.String(),
		ram.String(),
		cltRole,
		bkndRole,
	)
}
