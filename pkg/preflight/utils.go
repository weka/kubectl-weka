package preflight

import (
	"context"
	"fmt"
	"k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetPodsMapByNode(ctx context.Context, crClient client.Client, output *PreflightOutput) map[string][]v1.Pod {
	podsByNode := make(map[string][]v1.Pod)
	{
		var allPods v1.PodList
		if err := crClient.List(ctx, &allPods); err != nil {
			if output != nil {
				output.Printf("Warning: failed to list pods: %v\n", err)
			} else {
				fmt.Printf("Warning: failed to list pods: %v\n", err)
			}
		} else {
			// Build a map of nodeName -> pods on that node
			for i := range allPods.Items {
				pod := &allPods.Items[i]
				if pod.Spec.NodeName == "" {
					continue
				}
				podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], *pod)
			}
			if output != nil {
				output.Printf("Fetched %d pods across %d nodes\n", len(allPods.Items), len(podsByNode))
			} else {
				fmt.Printf("Fetched %d pods across %d nodes\n", len(allPods.Items), len(podsByNode))
			}
		}
	}
	return podsByNode
}
