package cmd

import (
	"context"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/cobra"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// GetCRD fetches the CRD object by name.
func GetCRD(ctx context.Context, restCfg *rest.Config, crdName string) (*apiextv1.CustomResourceDefinition, error) {
	ext, err := apiextclient.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	return ext.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
}

// PickCRDVersion chooses a version: prefer served+storage, otherwise first served.
func PickCRDVersion(crd *apiextv1.CustomResourceDefinition) (string, error) {
	for _, v := range crd.Spec.Versions {
		if v.Served && v.Storage {
			return v.Name, nil
		}
	}
	for _, v := range crd.Spec.Versions {
		if v.Served {
			return v.Name, nil
		}
	}
	return "", fmt.Errorf("CRD %s has no served versions", crd.Name)
}

// AttachScopeAutoEnforce hides namespace flags in help/usage and rejects them at runtime for cluster-scoped CRs.
// This is important because namespace flags are persistent on root.
func AttachScopeAutoEnforce(cmd *cobra.Command, crdName string) {
	// Hide flags for help/usage automatically
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		applyScopeHiding(c, crdName)
		defaultHelp(c, args)
	})

	defaultUsage := cmd.UsageFunc()
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		applyScopeHiding(c, crdName)
		return defaultUsage(c)
	})

	// Enforce at runtime
	origPreRunE := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		scope, err := detectCRDScope(context.Background(), crdName)
		if err == nil && scope == apiextv1.ClusterScoped {
			if flagNamespace != "" {
				return fmt.Errorf("--namespace/-n is not valid for cluster-scoped resource %s", crdName)
			}
			if flagAllNamespaces {
				return fmt.Errorf("--all-namespaces/-A is not valid for cluster-scoped resource %s", crdName)
			}
		}
		if origPreRunE != nil {
			return origPreRunE(c, args)
		}
		return nil
	}
}

func detectCRDScope(ctx context.Context, crdName string) (apiextv1.ResourceScope, error) {
	crd, err := GetCRD(ctx, mustRestConfigFromKubeconfig(), crdName)
	if err != nil {
		return "", err
	}
	return crd.Spec.Scope, nil
}

func applyScopeHiding(cmd *cobra.Command, crdName string) {
	scope, err := detectCRDScope(context.Background(), crdName)
	if err != nil || scope != apiextv1.ClusterScoped {
		return
	}
	hideInheritedFlag(cmd, "namespace")
	hideInheritedFlag(cmd, "all-namespaces")
}

func hideInheritedFlag(cmd *cobra.Command, name string) {
	if f := cmd.InheritedFlags().Lookup(name); f != nil {
		f.Hidden = true
	}
	if f := cmd.Flags().Lookup(name); f != nil {
		f.Hidden = true
	}
}

// Helper: load rest config once for scope detection
func mustRestConfigFromKubeconfig() *rest.Config {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)
	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		// In plugin context, this should be fatal
		panic(err)
	}
	return restCfg
}
