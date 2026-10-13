package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/airgapped"
	"github.com/weka/kubectl-weka/pkg/printer"
	"gopkg.in/yaml.v3"
)

var (
	flagListImagesOutput   string
	flagListImagesDiscover bool
)

var airgappedListImagesCmd = &cobra.Command{
	Use:   "list-images",
	Short: "List the images an air-gapped bundle needs, without downloading them",
	Long: `List every Docker image the 'download' command would put into an air-gapped bundle.

Only the Helm charts are fetched to resolve the image references; no images are
downloaded. Use it to review the image list, or to mirror the images with your
own tooling (skopeo, crane, a registry proxy).

Images are resolved exactly as 'download' does:
- Operator chart: the known image keys in values.yaml, plus image.repository:<appVersion>
- CSI chart: the images.*sidecar keys, plus images.csidriver:v<csidriverTag>
- WEKA software: quay.io/weka.io/weka-in-container:<weka-version>

With --discover, all chart values are also scanned for strings that look like
image references. Any the bundle would not include are listed as discovered;
these usually mean a newer chart added an image the plugin does not know yet.

Output formats:
  (default)  one image reference per line, suitable for scripting
  table      component, values path and image
  json/yaml  structured output

Examples:
  # Images for an operator and WEKA release
  kubectl weka air-gapped list-images --operator-version 1.10.8 --weka-version 5.1.0

  # Include image references the bundle would miss
  kubectl weka air-gapped list-images --operator-version 1.10.8 --csi-version 2.8.2 --discover -o table

  # Mirror the images with skopeo
  kubectl weka air-gapped list-images --csi-version 2.8.2 | \
    xargs -I{} skopeo copy --all docker://{} docker://my-registry.local:5000/{}`,
	RunE: runListImages,
}

func init() {
	airgappedCmd.AddCommand(airgappedListImagesCmd)

	airgappedListImagesCmd.Flags().StringVar(&flagWekaVersion, "weka-version", "",
		"WEKA software version (e.g., 5.3.0)")
	airgappedListImagesCmd.Flags().StringVar(&flagOperatorVersion, "operator-version", "",
		"WEKA Operator version (e.g., 1.2.0). Ignored if --operator-helm-path is specified")
	airgappedListImagesCmd.Flags().StringVar(&flagCSIVersion, "csi-version", "",
		"WEKA CSI Driver version (e.g., 2.1.0). Ignored if --csi-helm-path is specified")
	airgappedListImagesCmd.Flags().StringVar(&flagOperatorHelmPath, "operator-helm-path", "",
		"Path to WEKA Operator Helm chart (local path, archive, or remote URL). Overrides version-based download")
	airgappedListImagesCmd.Flags().StringVar(&flagCSIHelmPath, "csi-helm-path", "",
		"Path to WEKA CSI Driver Helm chart (local path, archive, or remote URL). Overrides version-based download")
	airgappedListImagesCmd.Flags().StringVarP(&flagListImagesOutput, "output", "o", "",
		"Output format. Supported: table, json, yaml (default: one image per line)")
	airgappedListImagesCmd.Flags().BoolVar(&flagListImagesDiscover, "discover", false,
		"Also scan chart values for image references the bundle would not include")

	airgappedListImagesCmd.RegisterFlagCompletionFunc("operator-version", completionListOperatorVersions)
	airgappedListImagesCmd.RegisterFlagCompletionFunc("csi-version", completionListCSIVersions)
	airgappedListImagesCmd.RegisterFlagCompletionFunc("output", cobra.FixedCompletions(
		[]string{"table", "json", "yaml"}, cobra.ShellCompDirectiveNoFileComp))

	airgappedListImagesCmd.SilenceUsage = true
}

func runListImages(_ *cobra.Command, _ []string) error {
	output := strings.ToLower(strings.TrimSpace(flagListImagesOutput))
	switch output {
	case "", "table", "wide", "json", "yaml":
	default:
		return fmt.Errorf("unsupported output format %q, supported: table, json, yaml", flagListImagesOutput)
	}

	opts := airgapped.NewDownloadOptions(context.Background(), "", flagWekaVersion, flagOperatorVersion, flagCSIVersion, flagOperatorHelmPath, flagCSIHelmPath, "")
	if err := opts.Validate(); err != nil {
		return err
	}
	// Fetches the Helm charts only; images are resolved from their values
	if err := opts.Normalize(); err != nil {
		return fmt.Errorf("failed to load Helm charts: %w", err)
	}

	refs := airgapped.ListImages(opts, flagListImagesDiscover)

	switch output {
	case "":
		printImageList(refs)
		return nil
	case "json":
		if refs == nil {
			refs = []airgapped.ImageRef{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(refs)
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(refs)
	default:
		return printImageTable(refs, output)
	}
}

// printImageList prints each unique image reference on its own line. Discovered
// references go to stderr, so that piping the output only passes what the bundle holds.
func printImageList(refs []airgapped.ImageRef) {
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.Discovered {
			fmt.Fprintf(os.Stderr, "discovered (not in bundle): %s (%s %s)\n", ref.Image, ref.Component, ref.Path)
			continue
		}
		if seen[ref.Image] {
			continue
		}
		seen[ref.Image] = true
		fmt.Println(ref.Image)
	}
}

func printImageTable(refs []airgapped.ImageRef, output string) error {
	columns := []printer.TableColumn{
		{Name: "COMPONENT"},
		{Name: "PATH"},
		{Name: "IMAGE"},
		{Name: "IN BUNDLE"},
		{Name: "DESCRIPTION", VisibleInWide: true},
	}

	rows := make([]printer.TableRow, 0, len(refs))
	for _, ref := range refs {
		rows = append(rows, printer.TableRow{Values: map[string]interface{}{
			"COMPONENT":   ref.Component,
			"PATH":        ref.Path,
			"IMAGE":       ref.Image,
			"IN BUNDLE":   !ref.Discovered,
			"DESCRIPTION": ref.Description,
		}})
	}

	p, _ := printer.GetPrinterFromFlags(output, true, nil, false, 0, printer.TableStyleMinimal)
	return p.Print(columns, rows, os.Stdout)
}
