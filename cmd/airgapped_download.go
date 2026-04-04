package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/airgapped"
	"github.com/weka/kubectl-weka/pkg/logging"
	"strings"
	"time"
)

var (
	flagDownloadOutput       string
	flagWekaVersion          string
	flagOperatorVersion      string
	flagCSIVersion           string
	flagOperatorHelmPath     string
	flagCSIHelmPath          string
	flagDownloadArchitecture string
)

var airgappedDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download Docker images and Helm charts for air-gapped deployments",
	Long: `Download Docker images and Helm charts required for WEKA deployments in air-gapped environments.

Creates a single tar.gz archive containing:
- Downloaded image archives (one per component/architecture)
- Helm chart archives (operator and optionally CSI)
- JSON manifest file (manifest.json) describing all images and charts

The manifest specifies what images need to be uploaded and which charts are included.

Helm charts are automatically downloaded from the configured repository based on component versions.

Authentication (if downloading from private registries):
  Credentials are provided via environment variables based on the registry hostname.
  Environment variable naming pattern: REG_<REGISTRY>_USERNAME and REG_<REGISTRY>_PASSWORD
  where <REGISTRY> is the registry hostname with dots, colons, and slashes replaced by underscores.
  
  Examples:
    - quay.io:                  REG_QUAY_IO_USERNAME and REG_QUAY_IO_PASSWORD
    - docker.io:                REG_DOCKER_IO_USERNAME and REG_DOCKER_IO_PASSWORD
    - gcr.io:                   REG_GCR_IO_USERNAME and REG_GCR_IO_PASSWORD
    - registry.k8s.io:          REG_REGISTRY_K8S_IO_USERNAME and REG_REGISTRY_K8S_IO_PASSWORD
  
  Fallback options:
    - ~/.docker/config.json (if no environment variables are set)
    - Anonymous access for public registries

Examples:
  # Download WEKA software and WEKA Operator
  kubectl weka air-gapped download \
    --weka-version 5.1.0 \
    --operator-version 1.10.8

  # Use local Helm charts instead of downloading
  kubectl weka air-gapped download \
    --operator-helm-path ./weka-operator-chart \
    --csi-helm-path ./weka-csi-chart

  # Download specific architecture only
  kubectl weka air-gapped download \
    --weka-version 5.3.0 \
    --operator-version 1.2.0 \
    --architecture arm64`,
	RunE: runDownload,
}

func init() {
	airgappedCmd.AddCommand(airgappedDownloadCmd)

	// Output file (not directory anymore - single tar.gz)
	airgappedDownloadCmd.Flags().StringVar(&flagDownloadOutput, "output", "",
		"Output tar.gz file (auto-generated if not specified)")

	// Version flags
	airgappedDownloadCmd.Flags().StringVar(&flagWekaVersion, "weka-version", "",
		"WEKA software version (e.g., 5.3.0). If not specified, WEKA images are not downloaded")
	airgappedDownloadCmd.Flags().StringVar(&flagOperatorVersion, "operator-version", "",
		"WEKA Operator version (e.g., 1.2.0). Ignored if --operator-helm-path is specified")
	airgappedDownloadCmd.Flags().StringVar(&flagCSIVersion, "csi-version", "",
		"WEKA CSI Driver version (e.g., 2.1.0). Ignored if --csi-helm-path is specified")

	// Helm chart path flags
	airgappedDownloadCmd.Flags().StringVar(&flagOperatorHelmPath, "operator-helm-path", "",
		"Path to WEKA Operator Helm chart (local path, archive, or remote URL). Overrides version-based download")
	airgappedDownloadCmd.Flags().StringVar(&flagCSIHelmPath, "csi-helm-path", "",
		"Path to WEKA CSI Driver Helm chart (local path, archive, or remote URL). Overrides version-based download")

	// Architecture flag
	airgappedDownloadCmd.Flags().StringVar(&flagDownloadArchitecture, "architecture", "amd64,arm64",
		"Target architectures (default: amd64,arm64). Supported: amd64, arm64")

	airgappedDownloadCmd.SilenceUsage = true
}

func generateLogPath(outputFile string) string {
	t := time.Now().Unix()
	ret := strings.Replace(outputFile, ".tar.gz", fmt.Sprintf(".%d.log", t), 1)
	return ret
}

func runDownload(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	opts := airgapped.NewDownloadOptions(ctx, flagDownloadOutput, flagWekaVersion, flagOperatorVersion, flagCSIVersion, flagOperatorHelmPath, flagCSIHelmPath, flagDownloadArchitecture)

	logPath := generateLogPath(opts.OutputFile)
	logger := logging.GetLogger(ctx, logPath)
	defer logger.Close()
	opts.Ctx = logging.WithLogger(ctx, logger)

	logger.Info("logging to file", "logPath", logPath)

	// Validate all options are correct (at this stage at least)
	if err := opts.Validate(); err != nil {
		return err
	}

	// Normalize options: ensure all fields are populated
	// - If version is specified, construct helm path
	// - If helm path is specified, extract version from chart
	if err := opts.Normalize(); err != nil {
		return fmt.Errorf("failed to obtain all necessary date: %w", err)
	}

	return airgapped.Download(ctx, opts)
}
