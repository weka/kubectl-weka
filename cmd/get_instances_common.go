package cmd

import (
	"fmt"
	"time"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func inferWekaContainerStatusTyped(wc *wekaapi.WekaContainer) string {
	if wc == nil {
		return "<missing>"
	}
	// primary status field
	if s := string(wc.Status.Status); s != "" {
		return s
	}
	// fallback: JoinedCluster condition is often the most meaningful
	if j := findConditionStatusTyped(wc.Status.Conditions, "JoinedCluster"); j != "<none>" {
		return "JoinedCluster=" + j
	}
	return "<unknown>"
}

func findConditionStatusTyped(conds []metav1.Condition, condType string) string {
	if len(conds) == 0 {
		return "<none>"
	}
	for _, c := range conds {
		if c.Type != condType {
			continue
		}
		if c.Status == "" {
			return "<none>"
		}
		return string(c.Status)
	}
	return "<none>"
}

// -----------------------------
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

func selectorMapToSelector(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	ls := labels.Set(m)
	return labels.SelectorFromSet(ls).String()
}
