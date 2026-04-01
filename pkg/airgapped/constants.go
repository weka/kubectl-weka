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
