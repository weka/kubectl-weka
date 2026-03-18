package getters

import (
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetWekaContainerStatus(wc *v1alpha1.WekaContainer) string {
	if wc == nil {
		return "<missing>"
	}
	// primary status field
	if s := string(wc.Status.Status); s != "" {
		return s
	}
	return "<unknown>"
}

func FindConditionStatusTyped(conds []v1.Condition, condType string) string {
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
