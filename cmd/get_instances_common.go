package cmd

import (
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

func selectorMapToSelector(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	ls := labels.Set(m)
	return labels.SelectorFromSet(ls).String()
}
