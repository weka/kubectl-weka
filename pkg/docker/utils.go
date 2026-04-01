package docker

import (
	"errors"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	types2 "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/weka/kubectl-weka/pkg/utils"
	"strings"
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
	// TODO: find a good way to resolve unqualified URLs (e.g. nginx, library/nginx) to docker hub
	// and now check if the imageURL has / in it, if now, we probably need to change it to docker hub
	//slash := strings.LastIndex(imageURL, "/")
	//if slash == -1 {
	//	imageURL = "index.docker.io/" + imageURL
	//}

	return tag, imageURL, nil
}
