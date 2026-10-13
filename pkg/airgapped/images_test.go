package airgapped

import (
	"reflect"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
)

func imagesOf(refs []ImageRef) []string {
	var out []string
	for _, ref := range refs {
		out = append(out, ref.Image)
	}
	return out
}

func TestCSIImageRefs(t *testing.T) {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{Name: "csi-wekafsplugin", Version: "2.9.2", AppVersion: "2.9.2"},
		Values:   csiChartValues(),
	}

	got := imagesOf(csiImageRefs(ch))
	want := []string{
		"registry.k8s.io/sig-storage/csi-attacher:v4.12.0",
		"registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.18.0",
		"registry.k8s.io/sig-storage/livenessprobe:v2.19.0",
		"registry.k8s.io/sig-storage/csi-provisioner:v6.3.0",
		"registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.17.0",
		"registry.k8s.io/sig-storage/csi-resizer:v2.2.1",
		"registry.k8s.io/sig-storage/csi-snapshotter:v8.6.0",
		// csidriverTag wins over the chart appVersion
		"quay.io/weka.io/csi-wekafs:v2.8.0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("csiImageRefs =\n%v\nwant\n%v", got, want)
	}
}

func TestCSIImageRefsDriverTagFallsBackToAppVersion(t *testing.T) {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{Name: "csi-wekafsplugin", Version: "2.9.2", AppVersion: "2.9.1"},
		Values: map[string]interface{}{
			"images": map[string]interface{}{"csidriver": "quay.io/weka.io/csi-wekafs"},
		},
	}

	got := imagesOf(csiImageRefs(ch))
	want := []string{"quay.io/weka.io/csi-wekafs:v2.9.1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("csiImageRefs = %v, want %v", got, want)
	}
}

func TestOperatorImageRefs(t *testing.T) {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{Name: "weka-operator", Version: "v1.10.0", AppVersion: "v1.10.0"},
		Values: map[string]interface{}{
			"sign-drive-image": "quay.io/weka.io/sign-drive:1.0",
			"maintenanceImage": "",
			"image":            map[string]interface{}{"repository": "quay.io/weka.io/weka-operator"},
			"csi": map[string]interface{}{
				"image":              "quay.io/weka.io/csi-wekafs:v2.8.0",
				"healthMonitorImage": "registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.18.0",
			},
		},
	}

	got := imagesOf(operatorImageRefs(ch))
	want := []string{
		"registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.18.0",
		"quay.io/weka.io/csi-wekafs:v2.8.0",
		"quay.io/weka.io/sign-drive:1.0",
		"quay.io/weka.io/weka-operator:v1.10.0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("operatorImageRefs =\n%v\nwant\n%v", got, want)
	}
}

func TestImageRefsNilChart(t *testing.T) {
	if refs := operatorImageRefs(nil); refs != nil {
		t.Errorf("operatorImageRefs(nil) = %v, want nil", refs)
	}
	if refs := csiImageRefs(nil); refs != nil {
		t.Errorf("csiImageRefs(nil) = %v, want nil", refs)
	}
}

func TestDiscoverImageRefs(t *testing.T) {
	values := csiChartValues()
	values["images"].(map[string]interface{})["newsidecar"] = "registry.k8s.io/sig-storage/new-sidecar:v1.0.0"
	values["extra"] = []interface{}{
		map[string]interface{}{"image": "localhost:5000/tools/busybox@sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}
	// strings that are not image references
	values["endpoint"] = "https://api.weka.io:443"
	values["mountPath"] = "/var/lib/kubelet/plugins"
	values["name"] = "csi-wekafsplugin"

	ch := &chart.Chart{Metadata: &chart.Metadata{Name: "csi-wekafsplugin", Version: "2.9.2"}, Values: values}

	got := discoverImageRefs(ComponentCSI, ch, csiImageRefs(ch))
	want := []ImageRef{
		{Component: ComponentCSI, Path: "extra[0].image", Image: "localhost:5000/tools/busybox@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Discovered: true},
		{Component: ComponentCSI, Path: "images.newsidecar", Image: "registry.k8s.io/sig-storage/new-sidecar:v1.0.0", Discovered: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("discoverImageRefs =\n%+v\nwant\n%+v", got, want)
	}
}

func TestListImages(t *testing.T) {
	opts := &DownloadOptions{
		WekaVersion: "5.1.0",
		CSIChart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "csi-wekafsplugin", Version: "2.9.2"},
			Values: map[string]interface{}{
				"images": map[string]interface{}{
					"csidriver":    "quay.io/weka.io/csi-wekafs",
					"csidriverTag": "2.8.0",
					"newsidecar":   "registry.k8s.io/sig-storage/new-sidecar:v1.0.0",
				},
			},
		},
	}

	got := imagesOf(ListImages(opts, false))
	want := []string{"quay.io/weka.io/csi-wekafs:v2.8.0", "quay.io/weka.io/weka-in-container:5.1.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListImages(discover=false) = %v, want %v", got, want)
	}

	refs := ListImages(opts, true)
	if len(refs) != 3 || !refs[1].Discovered || refs[1].Image != "registry.k8s.io/sig-storage/new-sidecar:v1.0.0" {
		t.Errorf("ListImages(discover=true) = %+v, want the new sidecar discovered after the CSI images", refs)
	}
}
