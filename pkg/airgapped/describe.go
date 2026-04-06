package airgapped

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/weka/kubectl-weka/pkg/targzutils"
	"github.com/weka/kubectl-weka/pkg/utils"
)

func DescribeBundle(bundleFile string, verbose bool) error {
	if _, err := os.Stat(bundleFile); err != nil {
		return fmt.Errorf("bundle file not found: %w", err)
	}

	fmt.Printf("🔍 Verifying bundle integrity...\n")
	valid, err := utils.VerifySHA256Signature(bundleFile)
	if err != nil {
		return fmt.Errorf("bundle integrity check failed: %w", err)
	}
	if !valid {
		return fmt.Errorf("bundle integrity check failed: signature mismatch or missing .sha256 file")
	}
	fmt.Printf("✅ Bundle integrity verified\n\n")

	// Extract and parse manifest
	fmt.Printf("📦 Reading bundle manifest...\n")
	manifest, err := extractAndParseManifest(bundleFile)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// Display manifest
	displayBundleManifest(manifest, verbose)
	return nil
}

// extractAndParseManifest uses TgzReader to extract and parse the manifest
func extractAndParseManifest(bundleFile string) (*BundleManifest, error) {
	// Open the tar.gz file with TgzReader
	reader, err := targzutils.NewTgzReader(bundleFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	// Read manifest.json
	manifestData, err := reader.ReadFile("manifest.json")
	if err != nil {
		return nil, fmt.Errorf("failed to extract manifest: %w", err)
	}

	// Parse JSON
	var manifest BundleManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	return &manifest, nil
}

// displayBundleManifest pretty-prints the bundle manifest
func displayBundleManifest(manifest *BundleManifest, verbose bool) {
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("📋 Bundle Manifest\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")

	// Basic info
	fmt.Printf("📅 Created: %s\n", manifest.CreatedAt)
	fmt.Printf("📊 Total Size: %s\n", utils.HumanBytes(manifest.TotalSize))
	fmt.Printf("🏗️ Architectures: %s\n\n", strings.Join(manifest.Architectures, ", "))

	// Components
	if len(manifest.Components) > 0 {
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("🔧 Components (%d)\n", len(manifest.Components))
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		for name, component := range manifest.Components {
			if component == nil {
				continue
			}

			fmt.Printf("  📦 %s\n", strings.ToUpper(name))
			fmt.Printf("     Version: %s\n", component.Version)
			fmt.Printf("     Size: %s\n", utils.HumanBytes(component.Size))
			fmt.Printf("     Images: %d\n\n", len(component.Images))

			// List images for this component
			for _, img := range component.Images {
				if img == nil {
					continue
				}
				fmt.Printf("       🐳 Image: %s\n", filepath.Base(img.Filename))
				fmt.Printf("          Original URL: %s\n", img.OriginalReference)

				// Verbose-only fields
				if verbose {
					fmt.Printf("          Architecture: %s\n", img.Architecture)
					if len(img.ImageReferences) > 0 {
						fmt.Printf("          References:\n")
						for _, ref := range img.ImageReferences {
							fmt.Printf("            - %s\n", ref)
						}
					}
					fmt.Printf("          Size: %s\n", utils.HumanBytes(img.Size))
					fmt.Printf("          SHA256: %s\n", img.SHA256)
				}
				fmt.Printf("\n")
			}
		}
	}

	// Helm Charts
	if len(manifest.HelmCharts) > 0 {
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("📊 Helm Charts (%d)\n", len(manifest.HelmCharts))
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		for name, chart := range manifest.HelmCharts {
			if chart == nil {
				continue
			}

			fmt.Printf("  📈 %s\n", name)
			fmt.Printf("     Name: %s\n", chart.Name)
			fmt.Printf("     Version: %s\n", chart.Version)

			// Verbose-only fields
			if verbose {
				fmt.Printf("     Size: %s\n", utils.HumanBytes(chart.Size))
				fmt.Printf("     File: %s\n", filepath.Base(chart.Filename))
				if chart.Repository != "" {
					fmt.Printf("     Repository: %s\n", chart.Repository)
				}
				fmt.Printf("     SHA256: %s\n", chart.SHA256)
			}
			fmt.Printf("\n")
		}
	}
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("✅ Bundle ready for air-gapped deployment\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")

	// Deployment hint
	fmt.Printf("💡 Next steps:\n")
	fmt.Printf("   1. Transfer the bundle file to your air-gapped environment\n")
	fmt.Printf("   2. Run: kubectl weka air-gapped upload --bundle <bundle-file> --registry <registry-url>\n")
	fmt.Printf("   3. Follow the instructions to deploy WEKA components\n\n")
}
