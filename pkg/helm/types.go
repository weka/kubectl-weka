package helm

// HelmChartArchive describes a Helm chart included in the bundle
type HelmChartArchive struct {
	// Chart name (operator, csi)
	Name string `json:"name"`

	// Chart version
	Version string `json:"version"`

	// Filename in the tar.gz bundle (e.g., "operator-1.2.0.tgz")
	Filename string `json:"filename"`

	// Size in bytes
	Size int64 `json:"size"`

	// SHA256 checksum
	SHA256 string `json:"sha256"`

	// Repository is the original Helm repository source
	Repository string `json:"repository"`
}
