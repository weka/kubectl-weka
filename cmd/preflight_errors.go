package cmd

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

type SkipHostCheckError struct {
	Node   string
	Reason string
}

func (e SkipHostCheckError) Error() string {
	return fmt.Sprintf("skip host checks on %s: %s", e.Node, e.Reason)
}

func pendingReason(p *corev1.Pod) string {
	// Try PodScheduled condition message
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && strings.TrimSpace(c.Message) != "" {
			return c.Message
		}
	}
	// Try container waiting reasons
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			r := strings.TrimSpace(cs.State.Waiting.Reason)
			m := strings.TrimSpace(cs.State.Waiting.Message)
			if r != "" && m != "" {
				return r + ": " + m
			}
			if r != "" {
				return r
			}
			if m != "" {
				return m
			}
		}
	}
	return ""
}
