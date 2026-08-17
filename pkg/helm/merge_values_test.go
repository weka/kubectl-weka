package helm

import (
	"context"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
)

func TestMergeValuesDeepKeepsSiblings(t *testing.T) {
	dst := map[string]interface{}{
		"images": map[string]interface{}{
			"csidriver":    "quay.io/weka.io/csi-wekafs",
			"csidriverTag": "2.9.2",
			"attacher":     "registry.k8s.io/sig-storage/csi-attacher:v4.12.0",
		},
	}
	// sparse update: the shape the air-gapped upload path produces
	src := map[string]interface{}{
		"images": map[string]interface{}{
			"csidriver": "reg.local:5000/csi-wekafs",
		},
	}

	MergeValuesDeep(dst, src)

	images := dst["images"].(map[string]interface{})
	if got := images["csidriver"]; got != "reg.local:5000/csi-wekafs" {
		t.Errorf("csidriver = %v, want the rewritten reference", got)
	}
	if got := images["csidriverTag"]; got != "2.9.2" {
		t.Errorf("csidriverTag = %v, want it preserved - an empty tag renders as \"image:v\"", got)
	}
	if got := images["attacher"]; got != "registry.k8s.io/sig-storage/csi-attacher:v4.12.0" {
		t.Errorf("attacher = %v, want it preserved", got)
	}
}

func TestMergeValuesDeepNestedAndNewKeys(t *testing.T) {
	dst := map[string]interface{}{
		"csi": map[string]interface{}{
			"image":               "quay.io/weka.io/csi-wekafs:v2.9.2",
			"installationEnabled": true,
			"kubeletPath":         "/var/lib/kubelet",
			"controller": map[string]interface{}{
				"resources": map[string]interface{}{"cpu": "1"},
				"replicas":  2,
			},
		},
	}
	src := map[string]interface{}{
		"csi": map[string]interface{}{
			"image": "reg.local:5000/csi-wekafs:v2.9.2",
			"controller": map[string]interface{}{
				"resources": map[string]interface{}{"cpu": "2"},
			},
		},
		"newTopLevel": "added",
	}

	MergeValuesDeep(dst, src)

	csi := dst["csi"].(map[string]interface{})
	if csi["installationEnabled"] != true || csi["kubeletPath"] != "/var/lib/kubelet" {
		t.Errorf("sibling keys under csi were dropped: %#v", csi)
	}
	controller := csi["controller"].(map[string]interface{})
	if controller["replicas"] != 2 {
		t.Errorf("csi.controller.replicas dropped: %#v", controller)
	}
	if got := controller["resources"].(map[string]interface{})["cpu"]; got != "2" {
		t.Errorf("csi.controller.resources.cpu = %v, want the update to win", got)
	}
	if dst["newTopLevel"] != "added" {
		t.Error("new top-level key not added")
	}
}

// a non-map update replaces whatever was there, including a map
func TestMergeValuesDeepScalarOverMap(t *testing.T) {
	dst := map[string]interface{}{"a": map[string]interface{}{"b": 1}}
	MergeValuesDeep(dst, map[string]interface{}{"a": "scalar"})
	if dst["a"] != "scalar" {
		t.Errorf("a = %#v, want the scalar to replace the map", dst["a"])
	}
}

// CreateUpdatedChartArchive must not lose chart values it was not asked to change
func TestCreateUpdatedChartArchivePreservesUntouchedValues(t *testing.T) {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{APIVersion: "v2", Name: "probe", Version: "1.0.0"},
		Values: map[string]interface{}{
			"images": map[string]interface{}{
				"csidriver":    "quay.io/weka.io/csi-wekafs",
				"csidriverTag": "2.9.2",
			},
			"csi": map[string]interface{}{
				"image":               "quay.io/weka.io/csi-wekafs:v2.9.2",
				"installationEnabled": true,
			},
		},
	}
	updated := map[string]interface{}{
		"images": map[string]interface{}{"csidriver": "reg.local:5000/csi-wekafs"},
		"csi":    map[string]interface{}{"image": "reg.local:5000/csi-wekafs:v2.9.2"},
	}

	out := filepath.Join(t.TempDir(), "probe.tgz")
	if err := CreateUpdatedChartArchive(context.Background(), ch, updated, out); err != nil {
		t.Fatalf("CreateUpdatedChartArchive: %v", err)
	}

	images := ch.Values["images"].(map[string]interface{})
	if images["csidriverTag"] != "2.9.2" {
		t.Errorf("images.csidriverTag lost: %#v", images)
	}
	csi := ch.Values["csi"].(map[string]interface{})
	if csi["installationEnabled"] != true {
		t.Errorf("csi.installationEnabled lost: %#v", csi)
	}
}
