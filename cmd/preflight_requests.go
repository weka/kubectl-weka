package cmd

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// free = allocatable - sum(pod requests) (scheduler-style)
func computeFreeFromRequests(ctx context.Context, clientset *kubernetes.Clientset, nodeName string, hpAlloc resource.Quantity) (memFree resource.Quantity, hpFree resource.Quantity, warn string, err error) {
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return resource.Quantity{}, resource.Quantity{}, "", err
	}

	memReq := resource.MustParse("0")
	hpReq := resource.MustParse("0")

	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}
		pMem, pHP := podRequests(p)
		memReq.Add(pMem)
		hpReq.Add(pHP)
	}

	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return resource.Quantity{}, resource.Quantity{}, "", err
	}

	memAlloc := quantityOrZero(node.Status.Allocatable, corev1.ResourceMemory)

	memFree = memAlloc.DeepCopy()
	memFree.Sub(memReq)
	if memFree.Sign() < 0 {
		warn = "mem requested > allocatable (check requests?)"
		memFree = resource.MustParse("0")
	}

	hpFree = hpAlloc.DeepCopy()
	hpFree.Sub(hpReq)
	if hpFree.Sign() < 0 {
		warn = strings.TrimSpace(warn + " hp requested > allocatable")
		hpFree = resource.MustParse("0")
	}

	return memFree, hpFree, warn, nil
}

// Requests for a pod per scheduling semantics:
// - sum regular containers
// - init containers: take max
// - add overhead if present
func podRequests(p *corev1.Pod) (mem resource.Quantity, hp resource.Quantity) {
	sumMem := resource.MustParse("0")
	sumHP := resource.MustParse("0")

	for i := range p.Spec.Containers {
		c := &p.Spec.Containers[i]
		sumMem.Add(quantityOrZero(c.Resources.Requests, corev1.ResourceMemory))
		sumHP.Add(quantityOrZero(c.Resources.Requests, "hugepages-2Mi"))
	}

	maxInitMem := resource.MustParse("0")
	maxInitHP := resource.MustParse("0")
	for i := range p.Spec.InitContainers {
		c := &p.Spec.InitContainers[i]
		m := quantityOrZero(c.Resources.Requests, corev1.ResourceMemory)
		h := quantityOrZero(c.Resources.Requests, "hugepages-2Mi")
		if m.Cmp(maxInitMem) > 0 {
			maxInitMem = m
		}
		if h.Cmp(maxInitHP) > 0 {
			maxInitHP = h
		}
	}

	mem = sumMem.DeepCopy()
	if maxInitMem.Cmp(mem) > 0 {
		mem = maxInitMem.DeepCopy()
	}
	hp = sumHP.DeepCopy()
	if maxInitHP.Cmp(hp) > 0 {
		hp = maxInitHP.DeepCopy()
	}

	if p.Spec.Overhead != nil {
		if ov, ok := (p.Spec.Overhead)[corev1.ResourceMemory]; ok {
			mem.Add(ov)
		}
		if ov, ok := (p.Spec.Overhead)["hugepages-2Mi"]; ok {
			hp.Add(ov)
		}
	}

	return mem, hp
}
