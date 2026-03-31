package plan

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/weka/kubectl-weka/pkg/utils"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"os"
	"strings"
)

// buildClusterContainerList builds the list of containers from a WekaCluster spec
func buildClusterContainerList(cluster *v1alpha1.WekaCluster) []ContainerRequirements {
	var containers []ContainerRequirements

	if cluster.Spec.Dynamic == nil {
		return containers
	}

	config := cluster.Spec.Dynamic
	usesHT := cluster.Spec.CpuPolicy == v1alpha1.CpuPolicyDedicatedHT || cluster.Spec.CpuPolicy == v1alpha1.CpuPolicyAuto
	additionalMemory := cluster.Spec.AdditionalMemory

	// Compute containers
	if config.ComputeContainers != nil && *config.ComputeContainers > 0 {
		req := calculateComputeRequirements(
			config.ComputeCores,
			0, // ComputeExtraCores - not in WekaConfig
			config.ComputeHugepages,
			additionalMemory.Compute,
			usesHT,
			cluster.Spec.RoleCoreIds.Compute,
		)
		req.Type = "compute"
		req.Count = *config.ComputeContainers

		reqNoHT := calculateComputeRequirements(
			config.ComputeCores,
			0,
			config.ComputeHugepages,
			additionalMemory.Compute,
			false,
			cluster.Spec.RoleCoreIds.Compute,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)
	}

	// Drive containers
	if config.DriveContainers != nil && *config.DriveContainers > 0 {
		req := calculateDriveRequirements(
			config.DriveCores,
			0, // DriveExtraCores - not in WekaConfig
			config.NumDrives,
			config.DriveHugepages,
			additionalMemory.Drive,
			usesHT,
			cluster.Spec.RoleCoreIds.Drive,
		)
		req.Type = "drive"
		req.Count = *config.DriveContainers
		req.Drives = config.NumDrives // Set drive requirements

		reqNoHT := calculateDriveRequirements(
			config.DriveCores,
			0,
			config.NumDrives,
			config.DriveHugepages,
			additionalMemory.Drive,
			false,
			cluster.Spec.RoleCoreIds.Drive,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)
	}

	// S3 containers
	if config.S3Containers > 0 {
		req := calculateS3Requirements(
			config.S3Cores,
			config.S3ExtraCores,
			config.S3FrontendHugepages,
			additionalMemory.S3,
			usesHT,
			cluster.Spec.RoleCoreIds.S3,
		)
		req.Type = "s3"
		req.Count = config.S3Containers

		reqNoHT := calculateS3Requirements(
			config.S3Cores,
			config.S3ExtraCores,
			config.S3FrontendHugepages,
			additionalMemory.S3,
			false,
			cluster.Spec.RoleCoreIds.S3,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)

		// Envoy containers (paired with S3)
		envoyReq := calculateEnvoyRequirements(additionalMemory.Envoy)
		envoyReq.Type = "envoy"
		envoyReq.Count = config.S3Containers
		containers = append(containers, envoyReq)
	}

	// NFS containers
	if config.NfsContainers > 0 {
		req := calculateNfsRequirements(
			config.NfsCores,
			config.NfsExtraCores,
			config.NfsFrontendHugepages,
			additionalMemory.Nfs,
			usesHT,
			cluster.Spec.RoleCoreIds.Nfs,
		)
		req.Type = "nfs"
		req.Count = config.NfsContainers

		reqNoHT := calculateNfsRequirements(
			config.NfsCores,
			config.NfsExtraCores,
			config.NfsFrontendHugepages,
			additionalMemory.Nfs,
			false,
			cluster.Spec.RoleCoreIds.Nfs,
		)
		req.CoresNoHT = reqNoHT.Cores

		containers = append(containers, req)
	}

	return containers
}

func printClusterContainerRequirements(containers []ContainerRequirements) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"TYPE", "COUNT", "CORES (HT ON)", "CORES (HT OFF)", "HUGEPAGES", "MEMORY", "DRIVES"})

	for _, c := range containers {
		if c.Count == 0 {
			continue
		}
		drivesStr := "-"
		if c.Drives > 0 {
			drivesStr = fmt.Sprintf("%d", c.Drives)
		}
		t.AppendRow(table.Row{
			utils.CapitalizeFirst(c.Type),
			c.Count,
			c.Cores,
			c.CoresNoHT,
			fmt.Sprintf("%d MiB", c.Hugepages),
			fmt.Sprintf("%d MiB", c.Memory),
			drivesStr,
		})
	}

	t.SetStyle(table.StyleLight)
	t.Render()
}

// validateContainerCompatibility checks that client, s3, and nfs containers don't coexist on the same node
// This is a requirement for current WEKA software versions
func validateContainerCompatibility(states map[string]*ConvergedNodeState) error {
	var errors []string

	for nodeName, state := range states {
		// Get container types on this node
		hasClient := false
		hasS3 := false
		hasNFS := false

		// Check cluster containers
		for _, container := range state.ClusterContainers {
			switch container.Type {
			case "s3":
				hasS3 = true
			case "nfs":
				hasNFS = true
			}
		}

		// Check client containers
		for _, container := range state.ClientContainers {
			if container.Type == "client" {
				hasClient = true
			}
		}

		// Validate incompatible combinations
		conflicts := []string{}
		if hasClient && hasS3 {
			conflicts = append(conflicts, "client and s3")
		}
		if hasClient && hasNFS {
			conflicts = append(conflicts, "client and nfs")
		}
		if hasS3 && hasNFS {
			conflicts = append(conflicts, "s3 and nfs")
		}

		if len(conflicts) > 0 {
			errors = append(errors, fmt.Sprintf("  ❌ Node %s: incompatible container types (%s) cannot coexist",
				nodeName, strings.Join(conflicts, ", ")))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("container compatibility violations found:\n%s\n\nClient, S3, and NFS containers cannot run on the same node in current WEKA software versions.\nPlease adjust nodeSelectors to prevent overlap.",
			strings.Join(errors, "\n"))
	}

	return nil
}
