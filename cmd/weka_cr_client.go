package cmd

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

// CachedClient wraps a controller-runtime client with caching capabilities
type CachedClient struct {
	crclient.Client
	cache  cache.Cache
	cancel context.CancelFunc
}

// Stop stops the cache and cleans up resources
func (c *CachedClient) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// newWekaCRClient returns a controller-runtime client with caching enabled.
// The cache is started in the background and the client is ready to use after
// the cache has synced.
//
// The caller should call Stop() on the returned CachedClient when done to
// clean up resources.
func newWekaCRClient(ctx context.Context, restCfg *rest.Config) (*CachedClient, error) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := wekaapi.AddToScheme(scheme); err != nil {
		return nil, err
	}

	// Create a cache for efficient listing and watching
	cacheObj, err := cache.New(restCfg, cache.Options{
		Scheme: scheme,
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
	directClient, err := crclient.New(restCfg, crclient.Options{Scheme: scheme})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Wrap the client to use cache for reads
	cachedClient := &cachedClientImpl{
		Client: directClient,
		cache:  cacheObj,
	}

	return &CachedClient{
		Client: cachedClient,
		cache:  cacheObj,
		cancel: cancel,
	}, nil
}

// cachedClientImpl is a client wrapper that uses cache for Get and List operations
type cachedClientImpl struct {
	crclient.Client
	cache cache.Cache
}

// Get retrieves an object from the cache
func (c *cachedClientImpl) Get(ctx context.Context, key crclient.ObjectKey, obj crclient.Object, opts ...crclient.GetOption) error {
	return c.cache.Get(ctx, key, obj, opts...)
}

// List retrieves a list of objects from the cache
func (c *cachedClientImpl) List(ctx context.Context, list crclient.ObjectList, opts ...crclient.ListOption) error {
	return c.cache.List(ctx, list, opts...)
}

// getRESTConfigAndDefaultNS returns REST config and the default namespace from
// kubeconfig.
func getRESTConfigAndDefaultNS() (*rest.Config, string, error) {
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return nil, "", err
	}

	ns, _, err := kubeCfg.Namespace()
	if err != nil {
		return nil, "", err
	}
	if ns == "" {
		ns = "default"
	}
	return restCfg, ns, nil
}
