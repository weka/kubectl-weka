package clustercheck

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/utils"
	"k8s.io/api/core/v1"
	kubernetes2 "k8s.io/client-go/kubernetes"
	"strings"
	"time"
)

// CPUManagerConfigModule validates kubelet CPU manager configuration
type CPUManagerConfigModule struct{}

func init() {
	GlobalClusterCheckRegistry.Register(&CPUManagerConfigModule{})
}

func (m *CPUManagerConfigModule) Name() string {
	return "cpu_manager_config"
}

func (m *CPUManagerConfigModule) FriendlyName() string {
	return "CPU Manager Configuration"
}

func (m *CPUManagerConfigModule) Description() string {
	return "Validates kubelet CPU manager configuration is set to 'static'"
}

func (m *CPUManagerConfigModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: CPU manager policy configured as 'static' ({{.ConfiguredCount}}/{{.TotalNodes}} nodes)"
}

func (m *CPUManagerConfigModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CPUManagerConfigModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CPUManagerConfigModule) SuggestedResolutionTemplate() string {
	return "Set cpuManagerPolicy to 'static' in kubelet configuration on affected nodes. See: https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/"
}

func (m *CPUManagerConfigModule) Validate(ctx context.Context, clients *kubernetes.K8sClients) (interface{}, error) {
	return nil, fmt.Errorf("CPU manager config module requires nodes parameter - use ValidateWithParams")
}

func (m *CPUManagerConfigModule) ValidateWithParams(ctx context.Context, clients *kubernetes.K8sClients, params map[string]interface{}) (interface{}, error) {
	// Extract ready nodes from params - skip NotReady nodes to avoid timeouts
	var nodes []v1.Node
	if readyNodesParam, ok := params["readyNodes"].([]v1.Node); ok {
		nodes = readyNodesParam
	} else if nodesParam, ok := params["nodes"].([]v1.Node); ok {
		// Fallback to all nodes if readyNodes not provided
		nodes = nodesParam
	} else {
		return nil, fmt.Errorf("nodes parameter is required for CPU manager config check")
	}

	var notStaticNodes []string
	var unknownNodes []string
	var configuredNodes []string

	for i := range nodes {
		n := &nodes[i]

		pol, err := getCPUManagerPolicyViaConfigz(ctx, clients.Clientset, n.Name)
		if err != nil {
			unknownNodes = append(unknownNodes, n.Name)
			continue
		}

		if strings.ToLower(strings.TrimSpace(pol)) != "static" {
			notStaticNodes = append(notStaticNodes, n.Name)
		} else {
			configuredNodes = append(configuredNodes, n.Name)
		}
	}

	status := "success"
	issue := ""
	detail := ""
	affectedNodes := []string{}

	if len(notStaticNodes) > 0 || len(unknownNodes) > 0 {
		status = "error"
		parts := []string{}
		if len(notStaticNodes) > 0 {
			parts = append(parts, fmt.Sprintf("%d nodes not configured as 'static'", len(notStaticNodes)))
			affectedNodes = append(affectedNodes, notStaticNodes...)
		}
		if len(unknownNodes) > 0 {
			parts = append(parts, fmt.Sprintf("%d nodes could not be checked", len(unknownNodes)))
			affectedNodes = append(affectedNodes, unknownNodes...)
		}
		issue = strings.Join(parts, ", ")
		detail = issue
	} else {
		detail = fmt.Sprintf("All %d ready nodes configured as 'static'", len(configuredNodes))
	}

	return map[string]interface{}{
		"Status":          status,
		"Detail":          detail,
		"Issue":           issue,
		"ConfiguredCount": len(configuredNodes),
		"NotStaticNodes":  notStaticNodes,
		"UnknownNodes":    unknownNodes,
		"TotalNodes":      len(nodes),
		"AffectedNodes":   affectedNodes,
		"AffectedCount":   len(affectedNodes),
	}, nil
}

// kubelet configz via apiserver proxy:
// GET /api/v1/nodes/<node>/proxy/configz
func getCPUManagerPolicyViaConfigz(ctx context.Context, clientset *kubernetes2.Clientset, nodeName string) (string, error) {
	// Retry logic with exponential backoff for transient errors (503, timeouts, etc.)
	var lastErr error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		raw, err := clientset.CoreV1().
			RESTClient().
			Get().
			AbsPath("/api/v1/nodes", nodeName, "proxy", "configz").
			Do(ctx).
			Raw()
		if err == nil {
			// Success - parse the response
			var top map[string]any
			if err := json.Unmarshal(raw, &top); err != nil {
				return "", err
			}

			if kc, ok := top["kubeletconfig"].(map[string]any); ok {
				if v, ok := kc["cpuManagerPolicy"].(string); ok && v != "" {
					return v, nil
				}
			}

			for _, v := range top {
				m, ok := v.(map[string]any)
				if !ok {
					continue
				}
				if pol, ok := m["cpuManagerPolicy"].(string); ok && pol != "" {
					return pol, nil
				}
			}

			return "", fmt.Errorf("cpuManagerPolicy not found in configz")
		}

		lastErr = err
		errStr := err.Error()

		// Check if this is a transient error worth retrying
		// Transient: 503 Service Unavailable, timeout, temporary network issue
		isTransient := strings.Contains(errStr, "503") ||
			strings.Contains(errStr, "temporarily unable") ||
			strings.Contains(errStr, "currently unable to handle") ||
			strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "temporary failure")

		if !isTransient {
			// Permanent error - don't retry
			return "", err
		}

		// This is a transient error; retry with backoff
		if attempt < maxRetries-1 {
			// Exponential backoff: 100ms, 200ms, 400ms
			waitTime := time.Duration(100*(1<<uint(attempt))) * time.Millisecond
			select {
			case <-time.After(waitTime):
				// Continue to next attempt
			case <-ctx.Done():
				// Context cancelled
				return "", ctx.Err()
			}
		}
	}

	return "", fmt.Errorf("failed to get CPU manager policy after %d retries: %w", maxRetries, lastErr)
}

func checkCPUManagerPolicyStatic(ctx context.Context, clientset *kubernetes2.Clientset, nodes []v1.Node) (bool, string) {
	var notStatic []string
	var unknown []string
	var skipped []string

	for i := range nodes {
		n := &nodes[i]

		// Skip nodes that are not ready
		if !kubernetes.IsNodeReady(*n) {
			skipped = append(skipped, n.Name)
			continue
		}

		pol, err := getCPUManagerPolicyViaConfigz(ctx, clientset, n.Name)
		if err != nil {
			unknown = append(unknown, fmt.Sprintf("%s=%s", n.Name, utils.ShortErr(err)))
			continue
		}

		if strings.ToLower(strings.TrimSpace(pol)) != "static" {
			notStatic = append(notStatic, fmt.Sprintf("%s=%s", n.Name, pol))
		}
	}

	// Success only if all ready nodes are static AND no unknown errors
	if len(notStatic) == 0 && len(unknown) == 0 {
		if len(skipped) == 0 {
			return true, ""
		}
		// All ready nodes ok, but some skipped - still PASS with note
		return true, fmt.Sprintf("%d nodes skipped (NotReady)", len(skipped))
	}

	// FAIL if there are non-static nodes or unknown errors
	parts := make([]string, 0, 3)
	readyCount := len(nodes) - len(skipped)
	passCount := readyCount - len(notStatic) - len(unknown)

	if passCount > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes ok", passCount))
	}
	if len(notStatic) > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes not static: %s", len(notStatic), utils.JoinAndTruncate(notStatic, 3)))
	}
	if len(unknown) > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes unknown: %s", len(unknown), utils.JoinAndTruncate(unknown, 3)))
	}
	if len(skipped) > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes skipped (NotReady)", len(skipped)))
	}

	return false, strings.Join(parts, ", ")
}
