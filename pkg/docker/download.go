package docker

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/targzutils"
	"github.com/weka/kubectl-weka/pkg/utils"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
)

// Download wraps the unexported downloadDockerImage with additional logic to detect registry and get appropriate authentication.
func Download(
	ctx context.Context,
	imageURL string,
	tag string,
	architectures []string,
	osType string,
	defaultAuth authn.Authenticator,
) (*ImageArchive, error) {
	// Check if we got the tag separately or not
	tag, imageURL, err := NormalizeImageUrlAndTag(imageURL, tag)
	if err != nil {
		return nil, fmt.Errorf("normalize image url and tag: %w", err)
	}

	// Detect registry from repo
	registry := detectRegistry(imageURL)

	// Get authenticator for this specific registry
	auth := getAuthenticatorForRegistry(ctx, registry, defaultAuth)

	// Download using single architecture
	return downloadDockerImage(ctx, imageURL, tag, architectures, osType, auth)
}

// downloadDockerImage pulls a remote image or image index, filters it by OS/arch if requested,
// stores it as an OCI layout on disk, tars+gzips that layout into a single bundle, and returns
// metadata for the resulting archive.
//
// Behavior:
//   - multi-arch remote => writes an OCI layout containing all matching platform images
//   - single-arch remote => logs a warning and returns only that image if it matches filters
//   - no match => error
func downloadDockerImage(
	ctx context.Context,
	imageURL string,
	tag string,
	architectures []string,
	osType string,
	auth authn.Authenticator,
) (*ImageArchive, error) {
	logger := logging.GetLogger(ctx)

	ref, err := buildReference(imageURL, tag)
	if err != nil {
		return nil, fmt.Errorf("parse image reference: %w", err)
	}

	downloadProgress := newProgressState(logger, "download", 0)
	downloadProgress.setImageName(strings.Join([]string{imageURL, tag}, ":"))
	defer downloadProgress.finish()

	ctx = withProgressState(ctx, downloadProgress)

	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuth(auth),
	}

	logger.Debug("Downloading docker image", "url", imageURL)

	desc, err := remote.Get(ref, remoteOpts...)
	if err != nil {
		wrappedErr := handleAuthenticationError(ctx, ref.String(), err)
		return nil, fmt.Errorf("fetch remote descriptor %q: %w", ref.Name(), wrappedErr)
	}

	if architectures == nil || architectures[0] == "" {
		architectures = []string{"amd64", "arm64"} // omit other exotic things we do not support
	}
	wantArch := utils.NormalizeSet(architectures)
	wantOS := utils.NormalizeValue(osType)

	switch {
	case isIndexMediaType(desc.MediaType):
		idx, err := desc.ImageIndex()
		if err == nil {
			if total, err := indexTotalBlobSize(idx, wantArch, wantOS); err == nil {
				downloadProgress.total = total
			}
		}

	case isImageMediaType(desc.MediaType):
		img, err := desc.Image()
		if err == nil {
			if total, err := imageTotalBlobSize(img); err == nil {
				downloadProgress.total = total
			}
		}
	}

	workDir, err := utils.MkdirTemp("", "oci-download-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	layoutDir := filepath.Join(workDir, "layout")
	lp, err := layout.Write(layoutDir, empty.Index)
	if err != nil {
		return nil, fmt.Errorf("initialize OCI layout: %w", err)
	}

	var imageRefs []string
	var selectedArchs []string
	switch {
	case isIndexMediaType(desc.MediaType):
		logger.Debug("Remote reference is multi-arch image index; filtering and downloading matching images", "url", imageURL, "requestedArchitectures", architectures, "requestedOS", osType)
		if err := downloadFromIndex(ctx, desc, lp, ref, wantArch, wantOS, &imageRefs, &selectedArchs); err != nil {
			return nil, err
		}

	case isImageMediaType(desc.MediaType):
		logger.Debug("Remote reference is single-arch image; checking if it matches requested filters", "url", imageURL, "requestedArchitectures", architectures, "requestedOS", osType)
		if err := downloadSingleImage(ctx, desc, lp, ref, wantArch, wantOS, &imageRefs, &selectedArchs); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("invalid image media type %s", desc.MediaType)
	}

	if len(imageRefs) == 0 {
		logger.Error("no image references found", "url", imageURL)
		return nil, fmt.Errorf("no images selected for %q after filtering by arch=%v os=%q", ref.Name(), architectures, osType)
	}

	sort.Strings(imageRefs)
	sort.Strings(selectedArchs)

	archiveBase := fmt.Sprintf("%s-%s", utils.SanitizeFilename(ref.Context().RepositoryStr()), utils.SanitizeFilename(ref.Identifier()))
	if archiveBase == "-" || archiveBase == "" {
		archiveBase = "image"
	}
	outPath := filepath.Join(workDir, archiveBase+".oci.tar.gz")
	logger.Debug("Creating OCI archive", "path", outPath, "imageReferences", imageRefs, "architectures", selectedArchs)

	progressCallback := func(filesAdded int, bytesAdded int64) {
	}

	if err := targzutils.PackWithProgressBar(layoutDir, outPath, progressCallback); err != nil {
		return nil, fmt.Errorf("create OCI archive: %w", err)
	}

	size, sum, err := utils.FileSizeAndSHA256(outPath)
	if err != nil {
		return nil, err
	}

	archField := "multi"
	if len(selectedArchs) == 1 {
		archField = selectedArchs[0]
	}

	// Build the original reference (the full image URL with tag that was downloaded)
	originalRef := imageURL
	if tag != "" {
		originalRef = fmt.Sprintf("%s:%s", imageURL, tag)
	}

	logger.Debug("Download complete", "size", size, "sum", sum, "architectures", archField, "OS", osType)
	return &ImageArchive{
		Filename:          outPath,
		Architecture:      archField,
		OriginalReference: originalRef,
		ImageReferences:   imageRefs,
		Size:              size,
		SHA256:            sum,
	}, nil
}

func downloadFromIndex(
	ctx context.Context,
	desc *remote.Descriptor,
	lp layout.Path,
	baseRef name.Reference,
	wantArch map[string]struct{},
	wantOS string,
	imageRefs *[]string,
	selectedArchs *[]string,
) error {
	logger := logging.GetLogger(ctx)
	idx, err := desc.ImageIndex()
	if err != nil {
		return fmt.Errorf("resolve image index for %q: %w", baseRef.Name(), err)
	}

	im, err := idx.IndexManifest()
	if err != nil {
		return fmt.Errorf("read image index manifest for %q: %w", baseRef.Name(), err)
	}

	seenArch := map[string]struct{}{}

	for _, m := range im.Manifests {
		if m.Platform == nil {
			continue
		}

		arch := utils.NormalizeValue(m.Platform.Architecture)
		osName := utils.NormalizeValue(m.Platform.OS)

		if !matchesArch(arch, wantArch) || !matchesOS(osName, wantOS) {
			continue
		}

		// If there are multiple matching entries for the same arch, keep the first.
		if _, exists := seenArch[arch]; exists {
			slog.WarnContext(ctx,
				"multiple matching manifests found for same architecture; keeping first",
				"image", baseRef.Name(),
				"architecture", arch,
				"os", osName,
				"digest", m.Digest.String(),
			)
			continue
		}

		img, err := idx.Image(m.Digest)
		if err != nil {
			return fmt.Errorf("resolve image %s from index %q: %w", m.Digest.String(), baseRef.Name(), err)
		}
		cfg, err := img.ConfigFile()
		if err == nil {
			logger.Debug("Downloading image for platform",
				"image", baseRef.Name(),
				"architecture", arch,
				"os", osName,
				"digest", m.Digest.String(),
				"layers", len(cfg.RootFS.DiffIDs),
			)
		}

		wrappedImg, err := wrapImageWithProgress(img, getProgressState(ctx))
		if err != nil {
			return fmt.Errorf("wrap image %s/%s for progress: %w", osName, arch, err)
		}

		logger.Debug("Downloading image for platform",
			"image", baseRef.Name(),
			"architecture", arch,
			"os", osName,
			"digest", m.Digest.String(),
		)

		if err := lp.AppendImage(wrappedImg, layout.WithPlatform(*m.Platform)); err != nil {
			return fmt.Errorf("append %s/%s image %s to OCI layout: %w", osName, arch, m.Digest.String(), err)
		}

		*imageRefs = append(*imageRefs, fmt.Sprintf("%s@%s", baseRef.Context().Name(), m.Digest.String()))
		*selectedArchs = append(*selectedArchs, arch)
		seenArch[arch] = struct{}{}
	}

	if len(*imageRefs) == 0 {
		return fmt.Errorf("image index %q does not contain any matching images for arch=%v os=%q", baseRef.Name(), utils.KeysOf(wantArch), wantOS)
	}

	return nil
}

func downloadSingleImage(
	ctx context.Context,
	desc *remote.Descriptor,
	lp layout.Path,
	baseRef name.Reference,
	wantArch map[string]struct{},
	wantOS string,
	imageRefs *[]string,
	selectedArchs *[]string,
) error {
	img, err := desc.Image()
	if err != nil {
		return fmt.Errorf("resolve image %q: %w", baseRef.Name(), err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return fmt.Errorf("read config for single image %q: %w", baseRef.Name(), err)
	}

	actualArch := utils.NormalizeValue(cfg.Architecture)
	actualOS := utils.NormalizeValue(cfg.OS)

	if !matchesArch(actualArch, wantArch) {
		return fmt.Errorf("single-arch image %q is %s, which does not match requested architectures %v", baseRef.Name(), actualArch, utils.KeysOf(wantArch))
	}
	if wantOS != "" && !matchesOS(actualOS, wantOS) {
		return fmt.Errorf("single-arch image %q is %s/%s, which does not match requested os=%q", baseRef.Name(), actualOS, actualArch, wantOS)
	}

	wrappedImg, err := wrapImageWithProgress(img, getProgressState(ctx))
	if err != nil {
		return fmt.Errorf("wrap single image for progress: %w", err)
	}

	if err := lp.AppendImage(wrappedImg, layout.WithPlatform(v1.Platform{
		OS:           actualOS,
		Architecture: actualArch,
	})); err != nil {
		return fmt.Errorf("append single image to OCI layout: %w", err)
	}

	d, err := img.Digest()
	if err != nil {
		return fmt.Errorf("compute digest for single image %q: %w", baseRef.Name(), err)
	}

	*imageRefs = append(*imageRefs, fmt.Sprintf("%s@%s", baseRef.Context().Name(), d.String()))
	*selectedArchs = append(*selectedArchs, actualArch)
	return nil
}
