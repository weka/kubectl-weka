package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func detectCRDScope(ctx context.Context, crdName string) (apiextv1.ResourceScope, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	restCfg, err := cfg.ClientConfig()
	if err != nil {
		return "", err
	}

	ext, err := apiextclient.NewForConfig(restCfg)
	if err != nil {
		return "", err
	}

	crd, err := ext.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return crd.Spec.Scope, nil
}

// Apply scope enforcement + automatic hiding for a command that uses CRD "crdName".
func AttachScopeAutoEnforce(cmd *cobra.Command, crdName string) {
	// On help/usage, detect scope and hide namespace flags before printing.
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

	// Enforce at runtime (reject -n/-A for cluster-scoped)
	origPreRunE := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		scope, err := detectCRDScope(context.Background(), crdName)
		if err == nil && scope == apiextv1.ClusterScoped {
			// These are root persistent flags in your plugin:
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

func applyScopeHiding(cmd *cobra.Command, crdName string) {
	scope, err := detectCRDScope(context.Background(), crdName)
	if err != nil {
		// If we can't detect, don't hide; better to show flags than hide incorrectly.
		return
	}
	if scope != apiextv1.ClusterScoped {
		return
	}

	// Hide inherited/persistent flags on this command.
	hideInheritedFlag(cmd, "namespace")
	hideInheritedFlag(cmd, "all-namespaces")
}

func hideInheritedFlag(cmd *cobra.Command, name string) {
	// InheritedFlags() includes persistent flags from parents.
	if f := cmd.InheritedFlags().Lookup(name); f != nil {
		f.Hidden = true
	}
	// Also hide local, in case you later add local flags with same name.
	if f := cmd.Flags().Lookup(name); f != nil {
		f.Hidden = true
	}
}
