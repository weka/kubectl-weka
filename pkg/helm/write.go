package helm

import (
	"fmt"
	"github.com/weka/kubectl-weka/pkg/targzutils"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"sort"
	"strings"
)

func WriteChartToTar(tw *targzutils.TgzWriter, ch *chart.Chart, root string) error {
	chartYAML, err := yaml.Marshal(ch.Metadata)
	if err != nil {
		return fmt.Errorf("marshal Chart.yaml: %w", err)
	}
	if err := tw.WriteFile(root+"Chart.yaml", chartYAML); err != nil {
		return err
	}

	// Write values.yaml: preserve original file with comments, but apply any changes to ch.Values
	valuesFound := false
	if len(ch.Files) > 0 {
		for _, f := range ch.Files {
			if f != nil && f.Name == "values.yaml" {
				// Get original values.yaml content
				originalContent := f.Data

				// If ch.Values has been modified, merge changes into original content
				if len(ch.Values) > 0 {
					mergedContent, err := mergeValuesWithComments(originalContent, ch.Values)
					if err != nil {
						// Fallback to original if merge fails
						mergedContent = originalContent
					}
					if err := tw.WriteFile(root+"values.yaml", mergedContent); err != nil {
						return err
					}
				} else {
					// No changes, use original as-is
					if err := tw.WriteFile(root+"values.yaml", originalContent); err != nil {
						return err
					}
				}
				valuesFound = true
				break
			}
		}
	}

	// If values.yaml was not found in files, marshal from parsed values (fallback)
	if !valuesFound && len(ch.Values) > 0 {
		valuesYAML, err := yaml.Marshal(ch.Values)
		if err != nil {
			return fmt.Errorf("marshal values.yaml: %w", err)
		}
		if err := tw.WriteFile(root+"values.yaml", valuesYAML); err != nil {
			return err
		}
	}

	if err := writeChartFiles(tw, root, ch.Templates); err != nil {
		return err
	}
	if err := writeChartFiles(tw, root, ch.Files); err != nil {
		return err
	}

	for _, dep := range ch.Dependencies() {
		if dep == nil || dep.Metadata == nil {
			continue
		}
		depRoot := root + "charts/" + dep.Metadata.Name + "/"
		if err := WriteChartToTar(tw, dep, depRoot); err != nil {
			return err
		}
	}

	return nil
}

func writeChartFiles(tw *targzutils.TgzWriter, root string, files []*chart.File) error {
	if len(files) == 0 {
		return nil
	}

	sorted := make([]*chart.File, 0, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		sorted = append(sorted, f)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	for _, f := range sorted {
		name := strings.TrimLeft(f.Name, "/")
		if name == "" {
			continue
		}
		if err := tw.WriteFile(root+name, f.Data); err != nil {
			return err
		}
	}

	return nil
}
