package getters

import (
	"context"
	"fmt"

	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetWekaContainers(ctx context.Context, c client.Client, ns string, allNS bool, name string) ([]v1alpha1.WekaContainer, error) {
	if name != "" {
		var wc v1alpha1.WekaContainer
		err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &wc)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get WekaContainer %q in namespace %q: %w", name, ns, err)
		}
		return []v1alpha1.WekaContainer{wc}, nil
	}

	var lst v1alpha1.WekaContainerList
	opts := []client.ListOption{}
	if !allNS {
		opts = append(opts, client.InNamespace(ns))
	}
	if err := c.List(ctx, &lst, opts...); err != nil {
		return nil, fmt.Errorf("failed to list WekaContainer CRs: %w", err)
	}
	return lst.Items, nil
}
