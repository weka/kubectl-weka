package docker

type ImageArchive struct {
	// Filename of the tar.gz bundle
	Filename string `json:"filename"`

	// Architecture (amd64, arm64, or "multi")
	Architecture string `json:"architecture"`

	// OriginalReference is the original image reference (tag or digest) that was downloaded
	// This is important for multi-arch images to know what manifest they were downloaded from
	// Examples: "quay.io/weka/image:v1.0", "quay.io/weka/image@sha256:abc123"
	OriginalReference string `json:"originalReference"`

	// Image references this archive contains
	ImageReferences []string `json:"imageReferences"`

	// Size in bytes
	Size int64 `json:"size"`

	// SHA256 checksum
	SHA256 string `json:"sha256"`
}
