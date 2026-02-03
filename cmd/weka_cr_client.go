package cmd

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

// newWekaCRClient returns a controller-runtime client with the WEKA API types
// registered into the scheme.
//
// We keep using client-go Clientset in other places for core resources (Pods,
// Nodes, etc.). This helper is strictly for interacting with WEKA CRDs using
// strongly-typed objects.
func newWekaCRClient(ctx context.Context, restCfg *rest.Config) (crclient.Client, error) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := wekaapi.AddToScheme(scheme); err != nil {
		return nil, err
	}

	c, err := crclient.New(restCfg, crclient.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	return c, nil
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
