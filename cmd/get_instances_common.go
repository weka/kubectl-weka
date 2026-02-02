package cmd

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"time"
)

func inferWekaContainerStatus(u *unstructured.Unstructured) string {
	for _, path := range [][]string{
		{"status", "phase"},
		{"status", "state"},
		{"status", "status"},
	} {
		if s := getString(u.Object, path...); s != "" {
			return s
		}
	}
	// fallback: JoinedCluster is often useful
	j := findConditionStatus(u, "JoinedCluster")
	if j != "<none>" {
		return "JoinedCluster=" + j
	}
	return "<unknown>"
}

func humanAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	// kubectl-ish compact
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d / (24 * time.Hour))
	if days < 365 {
		return fmt.Sprintf("%dd", days)
	}
	years := days / 365
	return fmt.Sprintf("%dy", years)
}

func getString(obj map[string]any, fields ...string) string {
	cur := any(obj)
	for _, f := range fields {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = m[f]
		if !ok {
			return ""
		}
	}
	switch x := cur.(type) {
	case string:
		return x
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%v", x)
	default:
		return ""
	}
}

func getStringSlice(obj map[string]any, fields ...string) []string {
	cur := any(obj)
	for _, f := range fields {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[f]
		if !ok {
			return nil
		}
	}
	arr, ok := cur.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func discoverGVR(disc discovery.DiscoveryInterface, group string, versions []string, resources []string) (schema.GroupVersionResource, error) {
	for _, v := range versions {
		gv := group + "/" + v
		l, err := disc.ServerResourcesForGroupVersion(gv)
		if err != nil || l == nil {
			continue
		}
		for _, r := range l.APIResources {
			for _, want := range resources {
				if r.Name == want {
					return schema.GroupVersionResource{Group: group, Version: v, Resource: want}, nil
				}
			}
		}
	}
	return schema.GroupVersionResource{}, fmt.Errorf("not found (group=%s resources=%v)", group, resources)
}

func selectorMapToSelector(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	ls := labels.Set(m)
	return labels.SelectorFromSet(ls).String()
}

func getStringMap(obj map[string]any, fields ...string) map[string]string {
	cur := any(obj)
	for _, f := range fields {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[f]
		if !ok {
			return nil
		}
	}
	raw, ok := cur.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func findConditionStatus(u *unstructured.Unstructured, condType string) string {
	conds, ok, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil {
		return "<none>"
	}
	if !ok || len(conds) == 0 {
		return "<none>"
	}
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		if t != condType {
			continue
		}
		s, _ := m["status"].(string)
		if s == "" {
			return "<none>"
		}
		return s
	}
	return "<none>"
}
