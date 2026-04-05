package plan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/types"
	"github.com/weka/kubectl-weka/pkg/utils"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
)

func printNodeSelectorSummary(grouping RoleNodeGrouping, globalSelector map[string]string) {
	fmt.Printf("Global NodeSelector: %s (%d nodes)\n", SelectorToString(globalSelector), len(grouping.Global))

	for _, role := range []string{"compute", "drive", "s3", "nfs"} {
		if roleGroup, exists := grouping.ByRole[role]; exists && len(roleGroup.Nodes) > 0 {
			fmt.Printf("%s role: %s (%d nodes)\n",
				utils.CapitalizeFirst(role),
				SelectorToString(roleGroup.Selector),
				len(roleGroup.Nodes))
		}
	}
}

func getActiveNodes(states map[string]*ConvergedNodeState) []string {
	var active []string
	for nodeName, state := range states {
		if len(state.ClusterContainers) > 0 || len(state.ClientContainers) > 0 {
			active = append(active, nodeName)
		}
	}
	sort.Strings(active)
	return active
}

// CountNotReadyNodes counts how many nodes are not in Ready state
func CountNotReadyNodes(nodes []v1.Node) int {
	count := 0
	for _, node := range nodes {
		if !kubernetes.IsNodeReady(node) {
			count++
		}
	}
	return count
}

func ParseWekaResourceFile[T runtime.Object](filePath string) (T, error) {
	var result T

	data, err := GetFileContents(filePath)
	if err != nil {
		return result, err
	}

	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		return result, err
	}
	if err := v1alpha1.AddToScheme(s); err != nil {
		return result, err
	}

	decode := serializer.NewCodecFactory(s).UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return result, err
	}

	// Type assert to the expected type
	result, ok := obj.(T)
	if !ok {
		return result, fmt.Errorf("file does not contain expected resource type")
	}

	return result, nil
}

// createContainerLegend returns a colored version of the container type name
func createContainerLegend(containerType string) string {
	var color string
	switch containerType {
	case "compute":
		color = types.ColorCompute
	case "drive":
		color = types.ColorDrive
	case "s3":
		color = types.ColorS3
	case "nfs":
		color = types.ColorNFS
	case "envoy":
		color = types.ColorEnvoy
	case "client":
		color = types.ColorClient
	default:
		color = types.ColorUsed
	}
	return color + strings.Repeat(getContainerPattern(containerType), 2) + " " + types.ColorReset + utils.CapitalizeFirst(containerType)
}

// createResourceBar creates a visual bar showing USED + WEKA + FREE with colors for each container type
// getContainerPattern returns a unique pattern character for each container type
// Uses only full block and shades for easy readability in B/W
func getContainerPattern(cType string) string {
	// ⣿ ░ ▒ ▓ █ █ █ █
	switch cType {
	case "compute":
		return "█" // Full block
	case "drive":
		return "▒" // Dark shade
	case "s3":
		return "▓" // Medium shade
	case "nfs":
		return "█" // Light shade
	case "envoy":
		return "░" // Dark shade
	case "client":
		return "⣿" // Light shade
	default:
		return "█" // Full block as default
	}
}
