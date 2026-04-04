package airgapped

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/helm"
	"github.com/weka/kubectl-weka/pkg/logging"
)

const WekaInContainerImageBase = "quay.io/weka.io/weka-in-container"

// Download handles downloading Docker images and Helm charts for air-gapped deployments
// It extracts image references from Helm charts or uses version-based sources
// and packages them into a single tar.gz bundle with a JSON manifest
func Download(ctx context.Context, opts *DownloadOptions) error {
	logger := logging.GetLogger(opts.Ctx)
	ctx = logging.WithLogger(ctx, logger)
	logger.Info("Starting image download for air-gapped deployment", "output", opts.OutputFile, "architectures", opts.Archs)

	b := &Bundle{
		components: make([]*ComponentManifest, 0),
		charts:     make([]*helm.HelmChartArchive, 0),
		opts:       opts,
	}

	// Download the bundle components
	if err := b.Download(ctx); err != nil {
		return err
	}

	// Create bundle with manifest
	logger.Info("Creating bundle with manifest")
	if err := b.Create(ctx, opts.OutputFile); err != nil {
		return fmt.Errorf("failed to create bundle: %w", err)
	}

	logger.Info("The bundle file should be copied to the airgapped site")
	logger.Info("Next step", "command", "kubectl weka air-gapped upload --bundle "+opts.OutputFile+" --registry <registry-url>")
	return nil
}
