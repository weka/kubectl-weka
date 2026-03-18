package cmd

import (
	_ "embed"
	"k8s.io/api/core/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed resources/hostcheck.sh
var hostcheckScript string

func makeHostChecksPod(ns, nodeName, podName, labelKey, labelVal string) *v1.Pod {

	return &v1.Pod{
		ObjectMeta: v2.ObjectMeta{
			Name:      podName,
			Namespace: ns,
			Labels: map[string]string{
				labelKey: labelVal,
			},
		},
		Spec: v1.PodSpec{
			NodeName:      nodeName,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			RestartPolicy: v1.RestartPolicyNever,
			Tolerations: []v1.Toleration{
				{
					Operator: v1.TolerationOpExists, // Tolerate all taints
				},
			},
			Containers: []v1.Container{
				{
					Name:    "hostchecks",
					Image:   "busybox:1.36",
					Command: []string{"sh", "-c", hostcheckScript},
					SecurityContext: &v1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(false),
						ReadOnlyRootFilesystem:   boolPtr(true),
					},
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
}
