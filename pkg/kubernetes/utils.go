package kubernetes

import (
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// QuantityOrZero returns the quantity value or zero if not found
func QuantityOrZero(resourceList v1.ResourceList, resourceName v1.ResourceName) resource.Quantity {
	val, ok := resourceList[resourceName]
	if !ok {
		return resource.Quantity{}
	}
	return val
}

// GetNamespaceFromFlags centralizes logic for namespace selection based on flags.
// Returns: namespace string, allNamespaces bool, error
func GetNamespaceFromFlags(allNamespaces bool, namespace string) (string, bool, error) {
	if allNamespaces {
		return "", true, nil
	}
	if namespace != "" {
		return namespace, false, nil
	}
	ns, err := GetKubeNamespace()
	if err != nil {
		return "", false, err
	}
	return ns, false, nil
}

// FilterOwnerContainers gets a list of WekaContainers and returns only those that have an owner reference matching the given owner(s) (WekaCluster or WekaClient)
func FilterOwnerContainers(all []v1alpha1.WekaContainer, owners ...client.Object) []v1alpha1.WekaContainer {
	var out []v1alpha1.WekaContainer

	if owners == nil || len(owners) == 0 {
		// do not filter if no owner provided
		return all
	}
	for _, wc := range all { // iterate over weka containers
		for _, owner := range owners {
			found := false
			kind := owner.GetObjectKind().GroupVersionKind().Kind
			for _, o := range wc.GetOwnerReferences() {
				if o.Kind != kind {
					continue
				}
				if o.UID == owner.GetUID() {
					out = append(out, wc)
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	return out
}
