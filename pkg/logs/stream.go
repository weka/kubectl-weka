package logs

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getItemsFromObjectList extracts all items from any ObjectList type using reflection
func getItemsFromObjectList(list client.ObjectList) ([]client.Object, error) {
	// Use reflection to access the Items field
	var out []client.Object
	listValue := reflect.ValueOf(list).Elem()
	itemsField := listValue.FieldByName("Items")

	if !itemsField.IsValid() {
		return nil, fmt.Errorf("list does not have Items field")
	}

	if itemsField.Len() == 0 {
		return nil, nil
	}

	// Iterate through all items and convert each to client.Object
	for i := 0; i < itemsField.Len(); i++ {
		item := itemsField.Index(i)
		// Get the address of the item and convert to client.Object interface
		obj := item.Addr().Interface().(client.Object)
		out = append(out, obj)
	}

	return out, nil
}

// createEmptyListForKind creates an empty ObjectList of the appropriate type based on OwnerKind
func createEmptyListForKind(ownerKind string) (client.ObjectList, error) {
	switch ownerKind {
	case "WekaCluster":
		return &v1alpha1.WekaClusterList{}, nil
	case "WekaClient":
		return &v1alpha1.WekaClientList{}, nil
	case "WekaPolicy":
		return &v1alpha1.WekaPolicyList{}, nil
	case "Deployment":
		return &v1.DeploymentList{}, nil
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported owner kind: %s", ownerKind)
	}
}

func getOwnerObjects(ctx context.Context, clients *kubernetes.K8sClients, opts WekaLogsOptions) ([]client.Object, error) {
	var ret []client.Object
	var err error

	listOpts := []client.ListOption{}
	if !opts.AllNamespaces {
		listOpts = append(listOpts, client.InNamespace(opts.Namespace))
	}

	// Create empty list of appropriate type
	objectList, err := createEmptyListForKind(opts.OwnerKind)
	if err != nil {
		return nil, err
	}

	// Fetch the object(s)
	if objectList != nil {
		err = clients.CRClient.List(ctx, objectList, listOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s %q in namespace %q: %w", opts.OwnerKind, opts.OwnerName, opts.Namespace, err)
		}

		// Extract the items from the list
		items, err := getItemsFromObjectList(objectList)
		if err != nil {
			return nil, fmt.Errorf("failed to extract %s from list: %w", opts.OwnerKind, err)
		}
		for _, o := range items {
			if opts.OwnerName != "" {
				if o.GetName() == opts.OwnerName {
					ret = append(ret, o) // we have matching name
					if !opts.AllNamespaces {
						continue // we do not expect to get another object of same name...
					}
				}
			}
			ret = append(ret, o) // we don't have any limitations on object names, add it
		}
		return ret, nil
	}
	return nil, nil // we don't have owner object kind
}

// StreamWekaObjectLogs streams logs from all WekaContainers in a WEKA owner object (Cluster, Client, Policy, Deployment, etc.)
func StreamWekaObjectLogs(ctx context.Context, clients *kubernetes.K8sClients, opts WekaLogsOptions) error {

	// Get the appropriate owner object(s) based on the OwnerKind
	owners, err := getOwnerObjects(ctx, clients, opts)
	if err != nil {
		return err
	}
	if owners != nil {
		// we actually have owners
	}

	// Get all WekaContainers in the namespace
	var containerList v1alpha1.WekaContainerList
	err = clients.CRClient.List(ctx, &containerList, client.InNamespace(opts.Namespace))

	if err != nil {
		return fmt.Errorf("failed to list WekaContainers in namespace %q: %w", opts.Namespace, err)
	}

	// Filter containers by owner (owner is now guaranteed to be non-nil and of correct type)
	filteredContainers := kubernetes.FilterOwnerContainers(containerList.Items, owners...)

	// Apply additional filters
	filteredContainers = applyContainerFilters(filteredContainers, opts)

	if len(filteredContainers) == 0 {
		return fmt.Errorf("no WekaContainers found matching the specified filters")
	}

	// Get pods for the filtered containers
	podMap, err := getPodsForContainers(ctx, clients, filteredContainers)
	if err != nil {
		return fmt.Errorf("failed to find pods for WekaContainers: %w", err)
	}

	// Apply nodeSelector filter if specified
	if opts.Aggregation.NodeSelector != "" {
		podMap = filterPodsByNodeSelector(ctx, clients, opts.Namespace, podMap, opts.Aggregation.NodeSelector)
	}

	if len(podMap) == 0 {
		return fmt.Errorf("no pods found for the filtered WekaContainers")
	}

	// Warn if exceeding concurrency limit
	if len(podMap) > opts.Aggregation.LimitConcurrent && opts.Aggregation.LimitConcurrent > 0 {
		fmt.Printf("Warning: %d WekaContainers match the filters, but only %d will be processed in parallel due to --limit-concurrent. Consider increasing the limit if you want to see more logs.\n",
			len(podMap), opts.Aggregation.LimitConcurrent)
	}

	// Stream logs from all pods
	return streamLogsFromPods(ctx, clients.Clientset, opts.Aggregation, podMap)
}
