package supportbundle

import (
	"context"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"log/slog"
)

// ============================================================================
// Context Keys and Helper Functions for Support Bundle Collectors
// ============================================================================

// Context keys for collector-related values
type contextKey string

const (
	ctxKeyClients              contextKey = "weka:clients"
	ctxKeyBundlePath           contextKey = "weka:bundle-path"
	ctxKeyNamespace            contextKey = "weka:namespace"
	ctxKeyAllNamespaces        contextKey = "weka:all-namespaces"
	ctxKeyCollectSensitiveData contextKey = "weka:collect-sensitive-data"
	ctxKeyLogger               contextKey = "weka:logger"
)

// ============================================================================
// Clients Context Helpers
// ============================================================================

func withClients(ctx context.Context, clients *kubernetes.K8sClients) context.Context {
	return context.WithValue(ctx, ctxKeyClients, clients)
}

func getClients(ctx context.Context) *kubernetes.K8sClients {
	if clients, ok := ctx.Value(ctxKeyClients).(*kubernetes.K8sClients); ok {
		return clients
	}
	return nil
}

// ============================================================================
// Bundle Path Context Helpers
// ============================================================================

func withBundlePath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, ctxKeyBundlePath, path)
}

func getBundlePath(ctx context.Context) string {
	if path, ok := ctx.Value(ctxKeyBundlePath).(string); ok {
		return path
	}
	return ""
}

// ============================================================================
// Namespace Context Helpers
// ============================================================================

func withNamespace(ctx context.Context, namespace string) context.Context {
	return context.WithValue(ctx, ctxKeyNamespace, namespace)
}

func getNamespace(ctx context.Context) string {
	if ns, ok := ctx.Value(ctxKeyNamespace).(string); ok {
		return ns
	}
	return ""
}

func withAllNamespaces(ctx context.Context, allNs bool) context.Context {
	return context.WithValue(ctx, ctxKeyAllNamespaces, allNs)
}

func getAllNamespaces(ctx context.Context) bool {
	if allNs, ok := ctx.Value(ctxKeyAllNamespaces).(bool); ok {
		return allNs
	}
	return false
}

// ============================================================================
// Sensitive Data Context Helpers
// ============================================================================

func withCollectSensitiveData(ctx context.Context, collect bool) context.Context {
	return context.WithValue(ctx, ctxKeyCollectSensitiveData, collect)
}

func getCollectSensitiveData(ctx context.Context) bool {
	if collect, ok := ctx.Value(ctxKeyCollectSensitiveData).(bool); ok {
		return collect
	}
	return false
}

// ============================================================================
// Logger Context Helpers
// ============================================================================

func withLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger, logger)
}

func GetLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
