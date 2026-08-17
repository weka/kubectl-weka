package airgapped

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/weka/kubectl-weka/pkg/helm"
	"helm.sh/helm/v3/pkg/chart"
)

// The rewritten chart must keep images.csidriverTag. The chart builds the driver
// reference as "{{ .images.csidriver }}:v{{ .images.csidriverTag }}", so losing the
// tag renders "<registry>/csi-wekafs:v" and every CSI pod lands in ImagePullBackOff.
func TestUpdatedCSIChartKeepsDriverTag(t *testing.T) {
	values := csiChartValues()
	imageMapping := map[string]string{
		"quay.io/weka.io/csi-wekafs:v2.8.0": "my-registry.local:5000/csi-wekafs:v2.8.0",
	}

	updated := updateCSIChartValues(values, imageMapping)

	ch := &chart.Chart{
		Metadata: &chart.Metadata{APIVersion: "v2", Name: "csi-wekafsplugin", Version: "2.9.2"},
		Values:   values,
	}
	out := filepath.Join(t.TempDir(), "csi.tgz")
	if err := helm.CreateUpdatedChartArchive(context.Background(), ch, updated, out); err != nil {
		t.Fatalf("CreateUpdatedChartArchive: %v", err)
	}

	if got := helm.GetNestedValue(ch.Values, "images.csidriverTag"); got != "2.8.0" {
		t.Errorf("images.csidriverTag = %q, want %q kept so the driver tag is not empty", got, "2.8.0")
	}
	if got := helm.GetNestedValue(ch.Values, "images.csidriver"); got != "my-registry.local:5000/csi-wekafs" {
		t.Errorf("images.csidriver = %q, want the rewritten repository", got)
	}
}

// csiChartValues mimics the images section of the csi-wekafsplugin chart values
func csiChartValues() map[string]interface{} {
	return map[string]interface{}{
		"images": map[string]interface{}{
			"livenessprobesidecar": "registry.k8s.io/sig-storage/livenessprobe:v2.19.0",
			"attachersidecar":      "registry.k8s.io/sig-storage/csi-attacher:v4.12.0",
			"provisionersidecar":   "registry.k8s.io/sig-storage/csi-provisioner:v6.3.0",
			"registrarsidecar":     "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.17.0",
			"resizersidecar":       "registry.k8s.io/sig-storage/csi-resizer:v2.2.1",
			"snapshottersidecar":   "registry.k8s.io/sig-storage/csi-snapshotter:v8.6.0",
			"healthmonitorsidecar": "registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.18.0",
			"csidriver":            "quay.io/weka.io/csi-wekafs",
			"csidriverTag":         "2.8.0",
		},
	}
}

// every sidecar in the chart values must be covered by csiSidecarImagePaths,
// otherwise it is not downloaded into the bundle nor rewritten on upload
func TestCSISidecarImagePathsCoverChartValues(t *testing.T) {
	values := csiChartValues()
	images := values["images"].(map[string]interface{})

	for key := range images {
		if key == "csidriver" || key == "csidriverTag" {
			continue
		}
		if _, ok := csiSidecarImagePaths["images."+key]; !ok {
			t.Errorf("sidecar images.%s is missing from csiSidecarImagePaths", key)
		}
	}
}

// the operator chart carries its own CSI image keys, deploying the sidecars itself
func TestUpdateOperatorChartValuesRewritesHealthMonitorImage(t *testing.T) {
	values := map[string]interface{}{
		"csi": map[string]interface{}{
			"image":              "quay.io/weka.io/csi-wekafs:v2.9.2",
			"snapshotterImage":   "registry.k8s.io/sig-storage/csi-snapshotter:v8.6.0",
			"healthMonitorImage": "registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.18.0",
		},
	}
	imageMapping := map[string]string{
		"registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.18.0": "my-registry.local:5000/csi-external-health-monitor-controller:v0.18.0",
	}

	updated := updateOperatorChartValues(values, imageMapping)

	want := "my-registry.local:5000/csi-external-health-monitor-controller:v0.18.0"
	if got := helm.GetNestedValue(updated, "csi.healthMonitorImage"); got != want {
		t.Errorf("csi.healthMonitorImage = %q, want %q", got, want)
	}
}

func TestUpdateCSIChartValuesRewritesHealthMonitorSidecar(t *testing.T) {
	values := csiChartValues()
	imageMapping := map[string]string{
		"registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.18.0": "my-registry.local:5000/csi-external-health-monitor-controller:v0.18.0",
		"registry.k8s.io/sig-storage/csi-attacher:v4.12.0":                           "my-registry.local:5000/csi-attacher:v4.12.0",
	}

	updated := updateCSIChartValues(values, imageMapping)

	for path, want := range map[string]string{
		"images.healthmonitorsidecar": "my-registry.local:5000/csi-external-health-monitor-controller:v0.18.0",
		"images.attachersidecar":      "my-registry.local:5000/csi-attacher:v4.12.0",
	} {
		if got := helm.GetNestedValue(updated, path); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}

	// sidecars with no uploaded counterpart must not appear in the overrides
	if got := helm.GetNestedValue(updated, "images.resizersidecar"); got != "" {
		t.Errorf("images.resizersidecar = %q, want empty (not in image mapping)", got)
	}
}
