package airgapped

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/weka/kubectl-weka/pkg/helm"
	"helm.sh/helm/v3/pkg/chart"
)

// Component names reported for each image reference
const (
	ComponentOperator = "operator"
	ComponentCSI      = "csi"
	ComponentWeka     = "weka"
)

// ImageRef is an image required by an air-gapped deployment, along with the
// chart values path it was resolved from
type ImageRef struct {
	Component   string `json:"component" yaml:"component"`
	Path        string `json:"path,omitempty" yaml:"path,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Image       string `json:"image" yaml:"image"`
	// Discovered is set for references found by scanning the chart values that
	// the download does not include in the bundle
	Discovered bool `json:"discovered,omitempty" yaml:"discovered,omitempty"`
}

// imageRefPattern matches strings that look like a registry image reference
// with an explicit tag or digest, e.g. registry.k8s.io/sig-storage/livenessprobe:v2.19.0
var imageRefPattern = regexp.MustCompile(`^[a-z0-9.-]+(:[0-9]+)?/[^\s]+(:[^\s/]+|@sha256:[a-f0-9]{64})$`)

// operatorImageRefs resolves every image the bundle takes from the operator chart
func operatorImageRefs(ch *chart.Chart) []ImageRef {
	if ch == nil || ch.Values == nil {
		return nil
	}

	var refs []ImageRef
	for _, path := range sortedKeys(operatorImagePaths) {
		if img := helm.GetNestedValue(ch.Values, path); img != "" {
			refs = append(refs, ImageRef{Component: ComponentOperator, Path: path, Description: operatorImagePaths[path], Image: img})
		}
	}

	// The operator image is built from image.repository + chart appVersion
	if repo := helm.GetNestedValue(ch.Values, "image.repository"); repo != "" {
		tag := ch.Metadata.AppVersion
		if tag == "" {
			tag = ch.Metadata.Version
		}
		refs = append(refs, ImageRef{Component: ComponentOperator, Path: "image.repository", Description: "operator image", Image: repo + ":" + tag})
	}
	return refs
}

// csiImageRefs resolves every image the bundle takes from the CSI plugin chart
func csiImageRefs(ch *chart.Chart) []ImageRef {
	if ch == nil || ch.Values == nil {
		return nil
	}

	var refs []ImageRef
	for _, path := range sortedKeys(csiSidecarImagePaths) {
		if img := helm.GetNestedValue(ch.Values, path); img != "" {
			refs = append(refs, ImageRef{Component: ComponentCSI, Path: path, Description: csiSidecarImagePaths[path], Image: img})
		}
	}

	// The CSI driver image is built from images.csidriver + images.csidriverTag,
	// matching the chart template "{{ .images.csidriver }}:v{{ .images.csidriverTag }}"
	if driver := helm.GetNestedValue(ch.Values, "images.csidriver"); driver != "" {
		tag := helm.GetNestedValue(ch.Values, "images.csidriverTag")
		if tag == "" {
			tag = ch.Metadata.AppVersion
		}
		if tag == "" {
			tag = ch.Metadata.Version
		}
		refs = append(refs, ImageRef{Component: ComponentCSI, Path: "images.csidriver", Description: "CSI driver image", Image: driver + ":v" + tag})
	}
	return refs
}

// wekaImageRef returns the WEKA software image for the given version
func wekaImageRef(version string) ImageRef {
	return ImageRef{Component: ComponentWeka, Description: "WEKA software image", Image: WekaInContainerImageBase + ":" + version}
}

// discoverImageRefs scans all string values of a chart for image references not
// in known, so that images added by newer chart releases surface instead of being
// silently left out of the bundle
func discoverImageRefs(component string, ch *chart.Chart, known []ImageRef) []ImageRef {
	if ch == nil || ch.Values == nil {
		return nil
	}

	knownImages := make(map[string]bool, len(known))
	for _, ref := range known {
		knownImages[ref.Image] = true
	}

	found := map[string]string{}
	walkStringValues(ch.Values, "", func(path, value string) {
		if imageRefPattern.MatchString(value) && !knownImages[value] {
			found[path] = value
		}
	})

	var refs []ImageRef
	for _, path := range sortedKeys(found) {
		refs = append(refs, ImageRef{Component: component, Path: path, Image: found[path], Discovered: true})
	}
	return refs
}

// walkStringValues calls fn for every string leaf in a chart values tree, with its
// dot-notation path (list elements are addressed as path[i])
func walkStringValues(value interface{}, path string, fn func(path, value string)) {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, child := range v {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			walkStringValues(child, childPath, fn)
		}
	case []interface{}:
		for i, child := range v {
			walkStringValues(child, fmt.Sprintf("%s[%d]", path, i), fn)
		}
	case string:
		fn(path, v)
	}
}

// ListImages returns every image the download command puts into the bundle for
// the given (normalized) options. When discover is set, image references found
// in the chart values but not included in the bundle are appended to the result.
func ListImages(opts *DownloadOptions, discover bool) []ImageRef {
	var refs []ImageRef

	operator := operatorImageRefs(opts.OperatorChart)
	refs = append(refs, operator...)
	if discover {
		refs = append(refs, discoverImageRefs(ComponentOperator, opts.OperatorChart, operator)...)
	}

	csi := csiImageRefs(opts.CSIChart)
	refs = append(refs, csi...)
	if discover {
		refs = append(refs, discoverImageRefs(ComponentCSI, opts.CSIChart, csi)...)
	}

	if opts.WekaVersion != "" {
		refs = append(refs, wekaImageRef(opts.WekaVersion))
	}
	return refs
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
