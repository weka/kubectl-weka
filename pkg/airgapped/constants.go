package airgapped

// Helm chart repository base URLs and paths (hardcoded)
const (
	// Base repository URLs
	// Operator uses OCI format from Quay.io
	defaultOperatorHelmURL = "oci://quay.io/weka.io/helm/weka-operator"
	// CSI uses GitHub releases
	defaultCSIHelmURL = "https://github.com/weka/csi-wekafs/releases/download"

	// Chart name patterns for constructing full URLs
	operatorChartPattern = "weka-operator"
	csiChartPattern      = "csi-wekafsplugin"

	// Helm chart archive extension

)

// csiSidecarImagePaths maps the dot-notation paths of CSI sidecar images in the
// csi-wekafsplugin chart values to a human readable description.
// It is the single source of truth for both download (bundle.go) and chart
// rewriting on upload (upload.go) - a sidecar missing here is silently skipped.
var csiSidecarImagePaths = map[string]string{
	"images.livenessprobesidecar": "CSI liveness probe sidecar",
	"images.attachersidecar":      "CSI attacher sidecar",
	"images.provisionersidecar":   "CSI provisioner sidecar",
	"images.registrarsidecar":     "CSI registrar sidecar",
	"images.resizersidecar":       "CSI resizer sidecar",
	"images.snapshottersidecar":   "CSI snapshotter sidecar",
	"images.healthmonitorsidecar": "CSI external health monitor sidecar",
}

// operatorImagePaths maps the dot-notation paths of images in the weka-operator
// chart values to a human readable description. Like csiSidecarImagePaths it is
// shared by image listing, download and chart rewriting on upload.
// The operator image itself is built from image.repository and the chart
// appVersion and is handled separately.
var operatorImagePaths = map[string]string{
	"sign-drive-image":       "sign drive image",
	"kubeProxyImage":         "kube proxy image",
	"maintenanceImage":       "maintenance image",
	"taskmon.defaultImage":   "taskmon default image",
	"metrics.clusters.image": "metrics clusters image",
	"csi.image":              "CSI driver image",
	"csi.provisionerImage":   "CSI provisioner image",
	"csi.attacherImage":      "CSI attacher image",
	"csi.livenessProbeImage": "CSI liveness probe image",
	"csi.resizerImage":       "CSI resizer image",
	"csi.snapshotterImage":   "CSI snapshotter image",
	"csi.registrarImage":     "CSI registrar image",
	// csi.healthMonitorImage is only present in operator charts that deploy the
	// external health monitor sidecar; older charts skip it as an empty value.
	"csi.healthMonitorImage": "CSI external health monitor image",
}
