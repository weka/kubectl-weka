package hostcheck

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/utils"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed resources/hostcheck.sh
var hostcheckScript string

func hostPathTypePtr(t v1.HostPathType) *v1.HostPathType { return &t }

// isOpenShiftCluster detects if the current cluster is OpenShift
// by checking for OpenShift API groups
// ctx is provided for potential future use (e.g., with timeouts on discovery calls)
func isOpenShiftCluster(ctx context.Context, clients *kubernetes.K8sClients) bool {
	_ = ctx // Silence unused warning - may be needed for future timeout handling
	// Check for OpenShift API groups
	apiGroups, err := clients.Clientset.Discovery().ServerGroups()
	if err != nil {
		return false // Assume not OpenShift if we can't check
	}

	for _, group := range apiGroups.Groups {
		if group.Name == "project.openshift.io" ||
			group.Name == "route.openshift.io" ||
			group.Name == "security.openshift.io" {
			return true
		}
	}
	return false
}

// createHostCheckServiceAccount creates a service account for hostcheck pods
// with appropriate permissions for accessing host information
func createHostCheckServiceAccount(ctx context.Context, clients *kubernetes.K8sClients, namespace string) error {
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hostcheck",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kubectl-weka",
				"app.kubernetes.io/component":  "hostcheck",
			},
		},
	}

	// Try to create, ignore if already exists
	if err := clients.CRClient.Create(ctx, sa); err != nil {
		// It's OK if it already exists
		if err != ctrlclient.IgnoreAlreadyExists(err) {
			return fmt.Errorf("failed to create service account: %w", err)
		}
	}

	return nil
}

// bindServiceAccountToPrivilegedSCC attempts to bind the hostcheck service account
// to the privileged Security Context Constraint on OpenShift
func bindServiceAccountToPrivilegedSCC(ctx context.Context, clients *kubernetes.K8sClients, namespace string) error {
	sccName := "privileged"
	saName := "hostcheck"
	saReference := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, saName)

	// Create unstructured object for the privileged SCC
	scc := &unstructured.Unstructured{}
	scc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "security.openshift.io",
		Version: "v1",
		Kind:    "SecurityContextConstraints",
	})
	scc.SetName(sccName)

	// Try to get the existing SCC
	if err := clients.CRClient.Get(ctx, ctrlclient.ObjectKey{Name: sccName}, scc); err != nil {
		// SCC not found - this might happen on non-OpenShift clusters
		// Print instructions for manual binding and return silently
		fmt.Printf("ℹ️  OpenShift SCC binding: privileged SCC not found (this is expected on non-OpenShift clusters)\n")
		fmt.Printf("   If running on OpenShift, ask your cluster admin to run:\n")
		fmt.Printf("   oc adm policy add-scc-to-user privileged -z %s -n %s\n", saName, namespace)
		return nil
	}

	// Get the users field from the SCC
	users, found, err := unstructured.NestedStringSlice(scc.Object, "users")
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not read SCC users field: %v\n", err)
		fmt.Printf("   Manual binding required: oc adm policy add-scc-to-user privileged -z %s -n %s\n", saName, namespace)
		return nil
	}

	if !found {
		users = []string{}
	}

	// Check if the service account is already in the users list
	for _, user := range users {
		if user == saReference {
			// Already bound
			fmt.Printf("✅ Service account '%s' already bound to privileged SCC\n", saName)
			return nil
		}
	}

	// Add the service account to the users list
	users = append(users, saReference)

	// Update the SCC with the new users list
	if err := unstructured.SetNestedStringSlice(scc.Object, users, "users"); err != nil {
		fmt.Printf("⚠️  Warning: Could not update SCC users list: %v\n", err)
		fmt.Printf("   Manual binding required: oc adm policy add-scc-to-user privileged -z %s -n %s\n", saName, namespace)
		return nil
	}

	// Try to update the SCC
	if err := clients.CRClient.Update(ctx, scc); err != nil {
		// If update fails, it might be due to permissions
		fmt.Printf("⚠️  Warning: Could not automatically bind service account to SCC: %v\n", err)
		fmt.Printf("   This requires admin privileges. Ask your cluster admin to run:\n")
		fmt.Printf("   oc adm policy add-scc-to-user privileged -z %s -n %s\n", saName, namespace)
		return nil // Non-fatal - pod might still work if SCC binding is already done
	}

	fmt.Printf("✅ Successfully bound service account '%s' to privileged SCC\n", saName)
	return nil
}


func makeHostChecksPod(ns, nodeName, podName, labelKey, labelVal string, useOpenShiftSecurity bool) (*v1.Pod, error) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
			Labels: map[string]string{
				labelKey: labelVal,
				"app.kubernetes.io/managed-by": "kubectl-weka",
				"app.kubernetes.io/component":  "hostcheck",
			},
		},
		Spec: v1.PodSpec{
			NodeName:      nodeName,
			HostNetwork:   true,
			HostPID:       true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			RestartPolicy: v1.RestartPolicyNever,
			Tolerations: []v1.Toleration{
				{
					Operator: v1.TolerationOpExists, // Tolerate all taints
				},
			},
			ServiceAccountName: "hostcheck", // Use a dedicated service account
			Containers: []v1.Container{
				{
					Name:    "hostchecks",
					Image:   "busybox:1.36",
					Command: []string{"sh", "-c", hostcheckScript},
					SecurityContext: getHostCheckSecurityContext(useOpenShiftSecurity),
					VolumeMounts: []v1.VolumeMount{
						{Name: "host-root", MountPath: "/host", ReadOnly: true},
						{Name: "host-sys", MountPath: "/host-sys", ReadOnly: true},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "host-root",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/",
							Type: hostPathTypePtr(v1.HostPathDirectory),
						},
					},
				},
				{
					Name: "host-sys",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/sys",
							Type: hostPathTypePtr(v1.HostPathDirectory),
						},
					},
				},
			},
		},
	}

	// Note: Pod security labels are NOT set here
	// - On OpenShift: SCC binding on service account handles enforcement
	// - On standard K8s: These pods need elevated privileges inherently
	// Setting pod security labels would only generate warnings about
	// legitimate violations (hostNetwork, hostPath, runAsUser=0, etc.)

	return pod, nil
}

// getHostCheckSecurityContext returns appropriate SecurityContext for the environment
func getHostCheckSecurityContext(useOpenShiftSecurity bool) *v1.SecurityContext {
	if useOpenShiftSecurity {
		// OpenShift-compatible security context
		// The SCC binding on the service account handles privilege escalation
		// We set minimal required options here
		return &v1.SecurityContext{
			// Allow privilege escalation to access host resources
			// This is allowed by the privileged SCC
			AllowPrivilegeEscalation: utils.BoolPtr(true),
			ReadOnlyRootFilesystem:   utils.BoolPtr(true),
			// Run as root (necessary to read host filesystems)
			RunAsUser:    utils.Int64Ptr(0),
			RunAsNonRoot: utils.BoolPtr(false),
			// Capabilities needed for host access
			Capabilities: &v1.Capabilities{
				Add: []v1.Capability{
					"SYS_ADMIN",
					"SYS_PTRACE",
					"DAC_READ_SEARCH",
				},
				// Drop all others to be explicit
				Drop: []v1.Capability{
					"NET_RAW",
				},
			},
			// Note: SELinux context removed - let SCC handle it
			// spc_t was causing policy violations
			// Seccomp profile for policy compliance
			SeccompProfile: &v1.SeccompProfile{
				Type: v1.SeccompProfileTypeRuntimeDefault,
			},
		}
	}

	// Standard Kubernetes security context (minimal privileges)
	return &v1.SecurityContext{
		AllowPrivilegeEscalation: utils.BoolPtr(false),
		ReadOnlyRootFilesystem:   utils.BoolPtr(true),
		RunAsUser:                utils.Int64Ptr(0), // Run as root but non-escalating
		RunAsNonRoot:             utils.BoolPtr(false),
		Capabilities: &v1.Capabilities{
			Drop: []v1.Capability{"ALL"},
		},
		SeccompProfile: &v1.SeccompProfile{
			Type: v1.SeccompProfileTypeRuntimeDefault,
		},
	}
}
