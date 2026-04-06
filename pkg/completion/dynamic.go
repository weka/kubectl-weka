package completion

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/config"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getAllClientObjects is a helper function that lists all objects of the same type as the provided object, using the provided client.
// It returns a slice of client.Object, which can be used for generic processing (e.g., filtering by namespace, partial name match, etc.) in completion functions.
// TODO: find a better way to implement it without pointing out each type explicitly, maybe using reflection or a map of GVK to list functions, but for now this is straightforward and works well enough
func getAllClientObjects(ctx context.Context, crClient client.Client, object client.Object) ([]client.Object, error) {
	switch object.(type) {
	case *v1alpha1.WekaClient:
		var lst v1alpha1.WekaClientList
		err := crClient.List(ctx, &lst)
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, len(lst.Items))
		for i := range lst.Items {
			objs[i] = &lst.Items[i]
		}
		return objs, nil
	case *v1alpha1.WekaCluster:
		var lst v1alpha1.WekaClusterList
		err := crClient.List(ctx, &lst)
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, len(lst.Items))
		for i := range lst.Items {
			objs[i] = &lst.Items[i]
		}
		return objs, nil
	case *v1alpha1.WekaContainer:
		var lst v1alpha1.WekaContainerList
		err := crClient.List(ctx, &lst)
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, len(lst.Items))
		for i := range lst.Items {
			objs[i] = &lst.Items[i]
		}
		return objs, nil

	case *v1alpha1.WekaPolicy:
		var lst v1alpha1.WekaPolicyList
		err := crClient.List(ctx, &lst)
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, len(lst.Items))
		for i := range lst.Items {
			objs[i] = &lst.Items[i]
		}
		return objs, nil
	case *v1.Namespace:
		var lst v1.NamespaceList
		err := crClient.List(ctx, &lst)
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, len(lst.Items))
		for i := range lst.Items {
			objs[i] = &lst.Items[i]
		}
		return objs, nil
	case *v1.Secret:
		var lst v1.SecretList
		err := crClient.List(ctx, &lst)
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, len(lst.Items))
		for i := range lst.Items {
			objs[i] = &lst.Items[i]
		}
		return objs, nil
	case *storagev1.CSIDriver:
		var lst storagev1.CSIDriverList
		err := crClient.List(ctx, &lst)
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, len(lst.Items))
		for i := range lst.Items {
			objs[i] = &lst.Items[i]
		}
		return objs, nil
	case *v1.Node:
		var lst v1.NodeList
		err := crClient.List(ctx, &lst)
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, len(lst.Items))
		for i := range lst.Items {
			objs[i] = &lst.Items[i]
		}
		return objs, nil
	default:
		panic(fmt.Sprintf("unknown type %T", object))
	}
}

// getAllClientObjectCandidates is a helper function that retrieves all objects of the same type as the provided object,
// using the provided client, and returns them as a slice of Object (a simplified struct with just name, namespace, and labels).
func getAllClientObjectCandidates(ctx context.Context, crClient client.Client, object client.Object) (*Objects, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objects, err := getAllClientObjects(ctx, crClient, object)
	if err != nil {
		return nil, err
	}
	out := &Objects{}
	for _, obj := range objects {
		candidate := Object{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
			Labels:    obj.GetLabels(),
		}
		*out = append(*out, candidate)
	}
	return out, nil
}

// getAllClientObjectCandidatesCached is a wrapper around getAllClientObjectCandidates that adds caching functionality.
// It first checks if the candidates are available in the cache and valid, and returns them if so.
// If not, it fetches fresh data using getAllClientObjectCandidates, stores it in the cache, and then returns it.
// The force parameter can be used to invalidate the cache and fetch fresh data even if the cache is still valid.
func getAllClientObjectCandidatesCached(ctx context.Context, crClient client.Client, object client.Object, force bool) (*Objects, error) {
	cacheKey := inferCacheKeyFromObject(object)
	usePersistency := config.Get().Cache.UsePersistentCaching
	if usePersistency {
		// Try cache first unless force is true, in which case we invalidate the cache and fetch fresh data
		if force {
			InvalidateCompletionCache(cacheKey)
		} else {
			ttl := config.Get().Cache.Completion.CompletionTTLs.Namespaces
			if cached, ok := LoadCompletionCache(cacheKey, ttl); ok {
				return cached, nil
			}
		}
	}

	// actually fetch the objects
	objects, err := getAllClientObjectCandidates(ctx, crClient, object)
	if err != nil {
		return nil, err
	}

	if usePersistency {
		SaveCompletionCache(cacheKey, objects)
	}
	return objects, nil
}

// GetCandidatesMatchingCompletion is the main function that completion functions should call to get a list of
// candidate objects matching the current completion input.
func GetCandidatesMatchingCompletion(
	cmd *cobra.Command,
	args []string,
	toComplete string,
	object client.Object,
	crClient client.Client,
) *Objects {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objects, err := getAllClientObjectCandidatesCached(ctx, crClient, object, false)
	if err != nil {
		return nil
	}

	// a case when user tries to complete with entry that does not exist in cache, force reload
	matching := objects.FilterSuggestionsByPartialMatch(toComplete)
	if len(*objects) > 0 && toComplete != "" && len(*matching) == 0 {
		objects, err = getAllClientObjectCandidatesCached(ctx, crClient, object, false)
		if err != nil {
			return nil
		}
	}
	// Namespace logic: if not set and not all-namespaces, use kubeconfig context
	namespace, _ := cmd.Flags().GetString("namespace")
	allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")
	if namespace == "" && !allNamespaces {
		if ns, err := kubernetes.GetKubeNamespace(); err == nil {
			namespace = ns
		}
	}

	out := &Objects{}
	out = objects.FilterByNamespaces(namespace, allNamespaces)
	out = out.FilterSuggestionsByPartialMatch(toComplete)
	if len(args) == 1 {
		out = out.FilterSuggestionsByPartialMatch(args[0])
	}
	return out
}

// CompleteNodeSelector provides context-aware completion for --node-selector flag.
// It suggests label keys (with =) if completing a key, values for a key if completing a value,
// and handles multi-selector input (comma-separated selectors).
func CompleteNodeSelector(ctx context.Context, crClient client.Client, toComplete string) []string {
	// Load all node label keys and values from cache
	object := &v1.Node{}
	nodeObjects, err := getAllClientObjectCandidatesCached(ctx, crClient, object, false)
	if err != nil {
		return nil
	}
	// Build map: key -> set of values
	labelValues := make(map[string]map[string]struct{})
	for _, nodeObject := range *nodeObjects {
		for k, v := range nodeObject.Labels {
			if _, ok := labelValues[k]; !ok {
				labelValues[k] = make(map[string]struct{})
			}
			labelValues[k][v] = struct{}{}
		}
	}

	// Parse toComplete into selectors
	// e.g. "a=b,c=d,e=" -> selectors: [a=b c=d e=]
	// We'll split by ',' but preserve empty last segment
	segments := strings.Split(toComplete, ",")
	usedKeys := make(map[string]struct{})
	for i := 0; i < len(segments)-1; i++ {
		seg := segments[i]
		if eq := strings.Index(seg, "="); eq > 0 {
			key := seg[:eq]
			usedKeys[key] = struct{}{}
		}
	}
	current := segments[len(segments)-1]

	// If current is empty (ends with ,), suggest keys not yet used
	if current == "" {
		var keys []string
		for k := range labelValues {
			if _, used := usedKeys[k]; !used {
				keys = append(keys, escapeNodeSelectorKey(k)+"=")
			}
		}
		slices.Sort(keys)
		return keys
	}

	// If current contains '=', suggest values for that key
	if eq := strings.Index(current, "="); eq >= 0 {
		key := current[:eq]
		prefix := current[eq+1:]
		valuesSet, ok := labelValues[key]
		if !ok {
			return nil
		}
		var values []string
		for v := range valuesSet {
			if strings.HasPrefix(v, prefix) {
				values = append(values, escapeNodeSelectorValue(v))
			}
		}
		// If empty value is present, suggest "" (empty string)
		if _, hasEmpty := valuesSet[""]; hasEmpty && strings.HasPrefix("", prefix) {
			values = append(values, "")
		}
		slices.Sort(values)
		// If the prefix matches exactly one value, suggest a comma for next selector (with description for zsh)
		if len(values) == 1 && values[0] == prefix {
			return []string{",\tAdd another selector"}
		}
		return values
	}

	// Otherwise, suggest keys not yet used, filtered by prefix
	var keys []string
	for k := range labelValues {
		if _, used := usedKeys[k]; !used && strings.HasPrefix(k, current) {
			keys = append(keys, escapeNodeSelectorKey(k)+"=")
		}
	}
	slices.Sort(keys)
	return keys
}

// escapeNodeSelectorKey escapes '=' and ',' in label keys
func escapeNodeSelectorKey(k string) string {
	k = strings.ReplaceAll(k, "=", "\\=")
	k = strings.ReplaceAll(k, ",", "\\,")
	return k
}

// escapeNodeSelectorValue escapes '=' and ',' in label values
func escapeNodeSelectorValue(v string) string {
	v = strings.ReplaceAll(v, "=", "\\=")
	v = strings.ReplaceAll(v, ",", "\\,")
	return v
}
