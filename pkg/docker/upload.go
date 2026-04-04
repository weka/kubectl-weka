package docker

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/targzutils"
	"github.com/weka/kubectl-weka/pkg/utils"
	"os"
	"path/filepath"
	"strings"
)

// UploadDockerImage wraps uploadDockerImage with automatic authenticator detection and setup.
// It extracts an OCI layout archive and uploads it to a registry.
// The imageURL can be in formats like:
//   - "registry/repo:tag" (tagged image)
//   - "registry/repo@sha256:..." (digest-based image)
//   - "registry/repo" (defaults to "latest" tag)
//
// The registry can include a port (e.g., "registry.scalar.lab:1234/repo")
func UploadDockerImage(
	ctx context.Context,
	archivePath string,
	imageURL string,
	username string,
	password string,
) error {
	logger := logging.GetLogger(ctx)

	// Parse image reference - separate registry and tag/digest
	var tag string
	imageWithoutTag := imageURL

	// Check if this is a digest-based reference (contains @sha256:)
	if strings.Contains(imageURL, "@sha256:") {
		// For digest references, we don't extract a tag
		// The entire imageURL is valid as-is
		tag = "" // Empty tag for digest-based references
		imageWithoutTag = imageURL

		// Detect registry from image URL for authentication
		registry := detectRegistry(imageURL)

		logger.Info("Uploading digest-based image", "archive", archivePath, "image", imageURL, "registry", registry)

		// Build authenticator with provided credentials or use default authentication
		var auth authn.Authenticator
		if username != "" && password != "" {
			auth = &authn.Basic{
				Username: username,
				Password: password,
			}
		} else {
			// Use default authenticator (will check docker config, keychains, etc.)
			auth = getAuthenticatorForRegistry(ctx, registry, authn.Anonymous)
		}

		// Call the core upload function with the digest-based reference
		return uploadDockerImage(ctx, archivePath, imageURL, "", auth)
	}

	// Tag-based reference - extract tag if present
	// Find the last slash (separates registry from repository)
	slashIdx := -1
	for i := len(imageURL) - 1; i >= 0; i-- {
		if imageURL[i] == '/' {
			slashIdx = i
			break
		}
	}

	// Look for colon after the last slash (that would be the tag)
	lastColonIdx := -1
	if slashIdx >= 0 {
		for i := len(imageURL) - 1; i > slashIdx; i-- {
			if imageURL[i] == ':' {
				lastColonIdx = i
				break
			}
		}
	} else {
		// No slash means the entire imageURL is just "image:tag"
		for i := len(imageURL) - 1; i >= 0; i-- {
			if imageURL[i] == ':' {
				lastColonIdx = i
				break
			}
		}
	}

	if lastColonIdx > slashIdx { // Colon after the last slash
		imageWithoutTag = imageURL[:lastColonIdx]
		tag = imageURL[lastColonIdx+1:]
	}

	if tag == "" {
		tag = "latest"
	}

	// Detect registry from image URL for authentication
	registry := detectRegistry(imageWithoutTag)

	logger.Debug("Uploading tagged image", "archive", archivePath, "image", imageWithoutTag, "tag", tag, "registry", registry)

	// Build authenticator with provided credentials or use default authentication
	var auth authn.Authenticator
	if username != "" && password != "" {
		auth = &authn.Basic{
			Username: username,
			Password: password,
		}
	} else {
		// Use default authenticator (will check docker config, keychains, etc.)
		auth = getAuthenticatorForRegistry(ctx, registry, authn.Anonymous)
	}

	// Call the core upload function with the properly parsed image URL
	return uploadDockerImage(ctx, archivePath, imageWithoutTag, tag, auth)
}

// uploadDockerImage is the core upload function that handles the actual image upload.
// It unpacks an OCI layout archive and uploads either:
//   - a single image manifest if the layout contains exactly one image descriptor
//   - an image index if the layout contains multiple descriptors or nested indexes
//
// For digest-based references, tag should be empty and the imageURL should be complete with @sha256:...
func uploadDockerImage(
	ctx context.Context,
	archivePath string,
	imageURL string,
	tag string,
	auth authn.Authenticator,
) error {
	// Build reference - handle both tagged and digest-based references
	var ref name.Reference
	var err error

	if tag == "" {
		// Digest-based reference - imageURL should already contain @sha256:...
		// Use name.ParseReference which properly handles full references
		ref, err = name.ParseReference(imageURL)
		if err != nil {
			// If strict parsing fails, try with WeakValidation
			ref, err = name.ParseReference(imageURL, name.WeakValidation)
		}
	} else {
		// Tag-based reference - combine imageURL and tag
		fullRef := fmt.Sprintf("%s:%s", imageURL, tag)
		ref, err = name.ParseReference(fullRef)
		if err != nil {
			// If strict parsing fails, try with WeakValidation
			ref, err = name.ParseReference(fullRef, name.WeakValidation)
		}
	}

	if err != nil {
		return fmt.Errorf("parse destination reference: %w", err)
	}

	workDir, err := utils.MkdirTemp("", "oci-upload-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}

	layoutDir := filepath.Join(workDir, "layout")
	if err := os.MkdirAll(layoutDir, 0o755); err != nil {
		return fmt.Errorf("create layout dir: %w", err)
	}

	if err := targzutils.Extract(ctx, archivePath, layoutDir); err != nil {
		return fmt.Errorf("extract OCI archive %q: %w", archivePath, err)
	}

	idx, err := layout.ImageIndexFromPath(layoutDir)
	if err != nil {
		return fmt.Errorf("load OCI image layout from %q: %w", layoutDir, err)
	}

	im, err := idx.IndexManifest()
	if err != nil {
		return fmt.Errorf("read OCI layout index manifest: %w", err)
	}

	logger := logging.GetLogger(ctx)

	totalUploadSize, err := ociLayoutTotalBlobSize(layoutDir)
	if err != nil {
		logger.Warn("failed to estimate upload size", "error", err)
	}

	uploadProgress := newProgressState(logger, "upload", totalUploadSize)
	uploadProgress.setImageName(strings.Join([]string{imageURL, tag}, ":"))
	uploadProgress.renderLine() // Render initial progress bar immediately
	defer uploadProgress.finish()

	ctx = withProgressState(ctx, uploadProgress)

	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuth(auth),
	}

	// If the OCI layout contains exactly one image manifest descriptor, push it as a plain image.
	// Otherwise push each image in the index individually, then push the index itself for multi-arch.
	if len(im.Manifests) == 1 && isImageMediaType(im.Manifests[0].MediaType) {
		img, err := idx.Image(im.Manifests[0].Digest)
		if err != nil {
			return fmt.Errorf("load single image %s from OCI layout: %w", im.Manifests[0].Digest.String(), err)
		}
		// Wrap image with progress tracking to report layer uploads
		wrappedImg, err := wrapImageWithProgress(img, uploadProgress)
		if err != nil {
			return fmt.Errorf("wrap image with progress: %w", err)
		}
		if err := remote.Write(ref, wrappedImg, remoteOpts...); err != nil {
			return fmt.Errorf("push image to %q: %w", ref.Name(), err)
		}
		return nil
	}

	// Multi-arch: push each individual image in the index first
	logger.Info("Pushing multi-arch images", "count", len(im.Manifests), "target", ref.Name())
	for i, desc := range im.Manifests {
		logger.Debug("Pushing image", "index", i, "digest", desc.Digest.String(), "mediaType", desc.MediaType)

		if isImageMediaType(desc.MediaType) {
			// It's an image manifest, push it
			img, err := idx.Image(desc.Digest)
			if err != nil {
				return fmt.Errorf("load image %s from OCI layout: %w", desc.Digest.String(), err)
			}
			// Wrap image with progress tracking to report layer uploads
			wrappedImg, err := wrapImageWithProgress(img, uploadProgress)
			if err != nil {
				return fmt.Errorf("wrap image with progress: %w", err)
			}
			if err := remote.Write(ref, wrappedImg, remoteOpts...); err != nil {
				wrappedErr := handleAuthenticationError(ctx, ref.String(), err)
				return fmt.Errorf("push image to %q: %w", ref.Name(), wrappedErr)
			}
			if err := remote.Write(ref, wrappedImg, remoteOpts...); err != nil {
				return fmt.Errorf("push image %s to %q: %w", desc.Digest.String(), ref.Name(), err)
			}
			// Update progress after each image is uploaded
			uploadProgress.renderLine()
		} else if desc.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" ||
			desc.MediaType == "application/vnd.oci.image.index.v1+json" {
			// It's a nested index, push it
			nestedIdx, err := idx.ImageIndex(desc.Digest)
			if err != nil {
				return fmt.Errorf("load image index %s from OCI layout: %w", desc.Digest.String(), err)
			}
			if err := remote.WriteIndex(ref, nestedIdx, remoteOpts...); err != nil {
				wrappedErr := handleAuthenticationError(ctx, ref.String(), err)
				return fmt.Errorf("push image index %s to %q: %w", desc.Digest.String(), ref.Name(), wrappedErr)
			}
			// Update progress after each index is uploaded
			uploadProgress.renderLine()
		}
	}

	// Finally, push the main index itself
	logger.Debug("Pushing image index", "target", ref.Name())

	if err := remote.WriteIndex(ref, idx, remoteOpts...); err != nil {
		return fmt.Errorf("push image index to %q: %w", ref.Name(), err)
	}
	uploadProgress.renderLine()
	return nil
}
