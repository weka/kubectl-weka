package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/airgapped"
)

var (
	flagUploadBundle       string
	flagUploadRegistry     string
	flagUploadArchitecture string
)

var airgappedUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload Docker images to a custom registry",
	Long: `Upload Docker images to a custom registry for air-gapped deployments.

This command uploads images from a tar.gz bundle created by the download command.
The bundle contains the image archives and a manifest file (manifest.json) that
specifies what images need to be uploaded.

The manifest ensures only the necessary images are uploaded to the registry.

Authentication:
  Credentials are provided via environment variables based on the registry hostname.
  Environment variable naming pattern: REG_<REGISTRY>_USERNAME and REG_<REGISTRY>_PASSWORD
  where <REGISTRY> is the registry hostname with dots, colons, and slashes replaced by underscores.
  
  Examples:
    - quay.io:                  REG_QUAY_IO_USERNAME and REG_QUAY_IO_PASSWORD
    - registry.example.com:     REG_REGISTRY_EXAMPLE_COM_USERNAME and REG_REGISTRY_EXAMPLE_COM_PASSWORD
    - 192.168.1.100:5000:       REG_192_168_1_100_USERNAME and REG_192_168_1_100_PASSWORD
    - docker.io:                REG_DOCKER_IO_USERNAME and REG_DOCKER_IO_PASSWORD
    - gcr.io:                   REG_GCR_IO_USERNAME and REG_GCR_IO_PASSWORD
    - registry.k8s.io:          REG_REGISTRY_K8S_IO_USERNAME and REG_REGISTRY_K8S_IO_PASSWORD
  
  Fallback options:
    - ~/.docker/config.json (if no environment variables are set)
    - Anonymous access for public registries

Architecture:
  Optionally specify which architectures to upload (default: all available). If your registry does not support multi-arch upload, you can filter by architecture using the --architecture flag.
  Supported: amd64, arm64

Examples:
  # Upload with credentials via environment variables
  export REG_QUAY_IO_USERNAME=myusername
  export REG_QUAY_IO_PASSWORD=mypassword
  kubectl weka air-gapped upload \
    --bundle ./weka-airgapped-bundle.tar.gz \
    --registry quay.io/myorg

  # Upload to private registry with IP address
  export REG_192_168_1_100_USERNAME=admin
  export REG_192_168_1_100_PASSWORD=secret
  kubectl weka air-gapped upload \
    --bundle ./weka-airgapped-bundle.tar.gz \
    --registry 192.168.1.100:5000

  # Upload specific architecture only
  kubectl weka air-gapped upload \
    --bundle ./weka-airgapped-bundle.tar.gz \
    --registry internal-registry:5000 \
    --architecture arm64`,
	RunE: runUpload,
}

func init() {
	airgappedCmd.AddCommand(airgappedUploadCmd)

	// Bundle file (from download command output)
	airgappedUploadCmd.Flags().StringVar(&flagUploadBundle, "bundle", "",
		"Tar.gz bundle file (from download command) (required)")
	airgappedUploadCmd.MarkFlagRequired("bundle")

	// Registry configuration
	airgappedUploadCmd.Flags().StringVar(&flagUploadRegistry, "registry", "",
		"Target registry URL (required)")
	airgappedUploadCmd.MarkFlagRequired("registry")

	// Architecture
	airgappedUploadCmd.Flags().StringVar(&flagUploadArchitecture, "architecture", "",
		"Target architectures to upload (optional, default: all). Supported: amd64, arm64")

	airgappedUploadCmd.RegisterFlagCompletionFunc("bundle", completionListAllTarGzFilesInDirectory)
	airgappedUploadCmd.RegisterFlagCompletionFunc("architecture", completionListArchitectures)

	airgappedUploadCmd.SilenceUsage = true
}

func runUpload(_ *cobra.Command, _ []string) error {
	ctx := context.Background()
	opts := airgapped.UploadOptions{
		BundleFile:   flagUploadBundle,
		RegistryURL:  flagUploadRegistry,
		Architecture: flagUploadArchitecture,
	}
	if err := airgapped.Upload(ctx, opts); err != nil {
		return fmt.Errorf("failed to upload images: %w", err)
	}
	return nil
}
