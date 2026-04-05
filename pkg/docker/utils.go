package docker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	types2 "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/weka/kubectl-weka/pkg/utils"
)

// UpdateTagForNewRegistry rewrites an image reference to use a new registry
// It extracts only the image name (last component) and combines it with the new registry
// Note: If the original reference has a digest, it strips it and uses a default tag instead
// because digest validation can fail when pushing to a different registry
// Input: "quay.io/brancz/kube-rbac-proxy@sha256:059a43ab..." with newRegistry "registry.scalar.lab:1234"
// Output: "registry.scalar.lab:1234/kube-rbac-proxy:latest"
// Input: "quay.io/brancz/kube-rbac-proxy:v0.21.0" with newRegistry "registry.scalar.lab:1234"
// Output: "registry.scalar.lab:1234/kube-rbac-proxy:v0.21.0"
func UpdateTagForNewRegistry(originalRef string, newRegistry string) string {
	// 1. Ensure that new registry has a canonical URL with port and optional namespace
	parts := strings.Split(newRegistry, "/")
	registryPath := ""
	if len(parts) > 1 {
		// the registry is not namespaced...
		registryPath = strings.Join(parts[1:], "/")
	}
	// ensure default port 5000 is added if not provided
	hostPart := parts[0]
	pieces := strings.Split(hostPart, ":")
	if len(pieces) < 2 {
		hostPart = hostPart + ":5000"
	}
	if registryPath != "" {
		newRegistry = hostPart + "/" + registryPath
	} else {
		newRegistry = hostPart
	}

	// 2. Find the last slash - everything after it is the image name with tag/digest
	lastSlashIdx := strings.LastIndex(originalRef, "/")
	if lastSlashIdx == -1 {
		// No slash found, the entire reference is just the image name
		return newRegistry + "/" + originalRef
	}

	// Extract the image name part (everything after the last slash)
	imageName := originalRef[lastSlashIdx+1:]

	// 3. If the image name has a digest (@sha256:...), strip it and use "latest" as tag
	// because digest validation can fail when uploading to a different registry
	if digestIdx := strings.Index(imageName, "@"); digestIdx != -1 {
		imageName = imageName[:digestIdx] + ":latest"
	} else if colonIdx := strings.Index(imageName, ":"); colonIdx == -1 {
		// If there's no tag or digest, add default "latest" tag
		imageName = imageName + ":latest"
	}
	// else: already has a tag, keep it

	// Combine new registry with image name
	return newRegistry + "/" + imageName
}

func buildReference(imageURL, tag string) (name.Reference, error) {
	imageURL = strings.TrimSpace(imageURL)
	tag = strings.TrimSpace(tag)

	if imageURL == "" {
		return nil, errors.New("imageURL is empty")
	}

	if tag != "" {
		return name.NewTag(fmt.Sprintf("%s:%s", imageURL, tag), name.WeakValidation)
	}

	return name.ParseReference(imageURL, name.WeakValidation)
}

func matchesArch(actual string, want map[string]struct{}) bool {
	if len(want) == 0 {
		return true
	}
	_, ok := want[utils.NormalizeValue(actual)]
	return ok
}

func matchesOS(actual, want string) bool {
	if want == "" {
		return true
	}
	return utils.NormalizeValue(actual) == utils.NormalizeValue(want)
}

func isIndexMediaType(mt types2.MediaType) bool {
	return strings.Contains(string(mt), "manifest.list") || strings.Contains(string(mt), "image.index")
}

func isImageMediaType(mt types2.MediaType) bool {
	return strings.Contains(string(mt), "manifest.v2+json") || strings.Contains(string(mt), "image.manifest")
}

// ExtractRepositoryFromImage extracts the repository part (everything before last colon and tag)
// Examples:
// "registry.example.com/image:v1.0" → "registry.example.com/image"
// "localhost:5000/image:v1" → "localhost:5000/image"
func ExtractRepositoryFromImage(imageRef string) string {
	// Find the last colon which separates repo from tag
	lastColon := strings.LastIndex(imageRef, ":")
	if lastColon > 0 {
		return imageRef[:lastColon]
	}
	return imageRef
}

// ExtractTagFromImage extracts the tag part (everything after last colon)
// Examples:
// "registry.example.com/image:v1.0" → "v1.0"
// "registry.example.com/namespace/image" → "latest"
// "localhost:5000/image:v1" → "v1"
// "localhost:5000/namespace/image:v1" → "v1"
// "image" (no tag) → "latest", docker simple untagged URL
// "namespace/image" → "latest", docker namespaced and untagged URL
// "image:v1" → "v1, docker simple tagged URL
// "namespace/image:v1" → "v1", docker namespaced tagged URL
func ExtractTagFromImage(imageRef string) string {
	lastColon := strings.LastIndex(imageRef, ":")
	if lastColon >= 0 {
		slash := strings.LastIndex(imageRef, "/")
		if slash >= 0 {
			// this means that the URL is in form localhost:5000/misc/nginx and we should not treat the last colon as tag separator
			if lastColon < slash {
				return "latest"
			}
		}
		return imageRef[lastColon+1:]
	}
	return "latest"
}

func StripTagFromImage(imageRef string) string {
	tag := ExtractTagFromImage(imageRef)
	newRef := strings.TrimSuffix(imageRef, fmt.Sprintf(":%s", tag))
	return newRef
}

// NormalizeImageUrlAndTag checks if the tag is provided separately or is part of the imageURL, and normalizes them accordingly.
func NormalizeImageUrlAndTag(imageURL, tag string) (string, string, error) {
	if tag == "" {
		tag = ExtractTagFromImage(imageURL)
		imageURL = strings.TrimSuffix(imageURL, fmt.Sprintf(":%s", tag))
	} else {
		// ensure that there is no duplicate tag
		t2 := ExtractTagFromImage(imageURL)
		if t2 == tag {
			imageURL = strings.TrimSuffix(imageURL, fmt.Sprintf(":%s", tag))
		} else if t2 != "latest" {
			// we had multiple tags, assume the explicit one is correct
			imageURL = strings.TrimSuffix(imageURL, fmt.Sprintf(":%s", t2))
		} else {
			// this is a normal case, no duplicate tag
		}
	}
	// NOTE: We do NOT normalize unqualified Docker Hub references here.
	// The normalization is available via NormalizeDockerReference() when explicitly needed
	// (e.g., during download/upload operations), but the general purpose NormalizeImageUrlAndTag
	// should preserve the original format for compatibility with existing code.

	return tag, imageURL, nil
}

// normalizeDockerReferenceInternal converts unqualified Docker Hub references to fully qualified form
// Examples:
// "nginx" → "docker.io/library/nginx"
// "library/nginx" → "docker.io/library/nginx"
// "myuser/myimage" → "docker.io/myuser/myimage"
// "quay.io/weka/image" → "quay.io/weka/image" (unchanged)
// "localhost:5000/image" → "localhost:5000/image" (unchanged)
func normalizeDockerReferenceInternal(imageRef string) string {
	// First, strip any digest (sha256:...) or tag from the reference
	imageRef = stripDigestAndTag(imageRef)

	// If already has a registry (contains . or :), return as-is
	if HasRegistry(imageRef) {
		return imageRef
	}

	// Otherwise it's a Docker Hub reference that needs to be fully qualified
	slashCount := strings.Count(imageRef, "/")

	if slashCount == 0 {
		// Simple image name (e.g., "nginx") → docker.io/library/nginx
		return "docker.io/library/" + imageRef
	} else if slashCount == 1 {
		// Namespaced image (e.g., "myuser/myimage" or "library/nginx") → docker.io/myuser/myimage
		return "docker.io/" + imageRef
	}

	// Should not reach here with valid Docker references, but return as-is if it does
	return imageRef
}

// IsDockerHubReference returns true if the image reference points to Docker Hub
// (i.e., it's an unqualified or library-prefixed Docker Hub image)
func IsDockerHubReference(imageRef string) bool {
	// Strip digest and tag first
	imageRef = stripDigestAndTag(imageRef)

	// If it has a registry indicator (. or :), it's not Docker Hub
	return !HasRegistry(imageRef)
}

// HasRegistry returns true if the image reference specifies a registry
// A registry is indicated by:
// - A dot (.) in the first component (e.g., "quay.io", "registry.example.com")
// - A colon in the first component (for port numbers, e.g., "localhost:5000")
func HasRegistry(imageRef string) bool {
	// Strip digest and tag to avoid false positives
	imageRef = stripDigestAndTag(imageRef)

	// Extract the first component (before the first /)
	slashIndex := strings.Index(imageRef, "/")
	if slashIndex == -1 {
		// No slash means it's just an image name like "nginx" or digest like "sha256:abc"
		// These don't have a registry
		return false
	}

	firstComponent := imageRef[:slashIndex]

	// Check if first component has registry indicators
	// . indicates a domain (quay.io, registry.example.com)
	// : indicates a port (localhost:5000, registry:5000)
	return strings.Contains(firstComponent, ".") || strings.Contains(firstComponent, ":")
}

// stripDigestAndTag removes @sha256:... or :tag from the end of an image reference
func stripDigestAndTag(imageRef string) string {
	// Remove digest (@sha256:...)
	if idx := strings.Index(imageRef, "@"); idx != -1 {
		imageRef = imageRef[:idx]
	}

	// Remove tag (:tag) but be careful with registry ports (host:5000/image)
	if idx := strings.LastIndex(imageRef, ":"); idx != -1 {
		// Check if this colon is part of a registry port or a tag
		// If there's a / after the colon, it's a port (host:5000/image)
		// If there's no / after, it's a tag (image:v1)
		if strings.Contains(imageRef[idx:], "/") {
			// This is a port number, don't strip
			return imageRef
		}
		// This is a tag, strip it
		imageRef = imageRef[:idx]
	}

	return imageRef
}

// NormalizeDockerReference converts unqualified Docker Hub references to fully qualified form
// This is a public wrapper around normalizeDockerReferenceInternal
// Examples:
// "nginx" → "docker.io/library/nginx"
// "library/nginx" → "docker.io/library/nginx"
// "myuser/myimage" → "docker.io/myuser/myimage"
// "quay.io/weka/image" → "quay.io/weka/image" (unchanged)
func NormalizeDockerReference(imageRef string) string {
	return normalizeDockerReferenceInternal(imageRef)
}
