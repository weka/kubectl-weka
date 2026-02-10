package cmd

import (
	"context"
	"fmt"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ============================================================================
// Kubernetes Client Management
// ============================================================================

// K8sClients provides both kubernetes.Clientset and controller-runtime client.Client
// with caching support for efficient resource access.
//
// This client replaces the old CachedClient and provides:
// - Clientset for standard K8s operations (pods, services, nodes, etc.)
// - CRClient for custom resources with automatic caching (Get/List operations)
// - Unified lifecycle management with a single Stop() method
//
// Usage:
//
//	unifiedClient, err := NewK8sClients(ctx)
//	if err != nil {
//	    return err
//	}
//	defer unifiedClient.Stop()
//
//	// Use Clientset for standard resources
//	pods, _ := unifiedClient.Clientset.CoreV1().Pods("default").List(...)
//
//	// Use CRClient for custom resources (cached)
//	var clusters wekaapi.WekaClusterList
//	unifiedClient.CRClient.List(ctx, &clusters)
type K8sClients struct {
	Clientset *kubernetes.Clientset // For standard K8s operations (pods, services, etc.)
	CRClient  client.Client         // For custom resources with caching
	cache     cache.Cache           // Internal cache
	cancel    context.CancelFunc    // Cleanup function
}

// Stop stops the cache and cleans up resources
func (u *K8sClients) Stop() {
	if u.cancel != nil {
		u.cancel()
	}
}

// GetKubeConfig retrieves the Kubernetes configuration
func GetKubeConfig() (*rest.Config, error) {
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	return restCfg, nil
}

// GetKubeNamespace retrieves the current namespace from kubeconfig
func GetKubeNamespace() (string, error) {
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	ns, _, err := kubeCfg.Namespace()
	if err != nil {
		return "", err
	}
	if ns == "" {
		ns = "default"
	}
	return ns, nil
}

// NewK8sClients creates a new K8sClients with both Clientset and controller-runtime client
// The client includes caching for efficient resource access
// The caller should call Stop() on the returned client when done to clean up resources
func NewK8sClients(ctx context.Context) (*K8sClients, error) {
	cfg, err := GetKubeConfig()
	if err != nil {
		return nil, err
	}

	// Create standard kubernetes clientset
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Create scheme with both core and Weka CRDs
	runtimeScheme := runtime.NewScheme()
	if err := scheme.AddToScheme(runtimeScheme); err != nil {
		return nil, fmt.Errorf("failed to add core scheme: %w", err)
	}
	if err := wekaapi.AddToScheme(runtimeScheme); err != nil {
		return nil, fmt.Errorf("failed to add weka scheme: %w", err)
	}

	// Create a cache for efficient listing and watching
	cacheObj, err := cache.New(cfg, cache.Options{
		Scheme: runtimeScheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Start the cache in the background
	cacheCtx, cancel := context.WithCancel(ctx)
	go func() {
		_ = cacheObj.Start(cacheCtx)
	}()

	// Wait for cache to sync
	if !cacheObj.WaitForCacheSync(cacheCtx) {
		cancel()
		return nil, fmt.Errorf("failed to sync cache")
	}

	// Create a standard client for writes (and fallback reads)
	directClient, err := client.New(cfg, client.Options{Scheme: runtimeScheme})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create CR client: %w", err)
	}

	// Wrap the client to use cache for reads
	cachedCRClient := &cachedClientImpl{
		Client: directClient,
		cache:  cacheObj,
	}

	return &K8sClients{
		Clientset: clientset,
		CRClient:  cachedCRClient,
		cache:     cacheObj,
		cancel:    cancel,
	}, nil
}

// cachedClientImpl is a client wrapper that uses cache for Get and List operations
type cachedClientImpl struct {
	client.Client
	cache cache.Cache
}

// Get retrieves an object from the cache
func (c *cachedClientImpl) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.cache.Get(ctx, key, obj, opts...)
}

// List retrieves a list of objects from the cache
func (c *cachedClientImpl) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.cache.List(ctx, list, opts...)
}
