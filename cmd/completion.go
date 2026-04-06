package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/weka/kubectl-weka/pkg/completion"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

func completionListNamespaces(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	clients, err := kubernetes.NewK8sClients(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := completion.GetCandidatesMatchingCompletion(
		cmd,
		args,
		toComplete,
		&corev1.Namespace{},
		clients.CRClient,
	)
	if out != nil {
		return out.Strings(), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp

}

// completionListWekaClients provides completion for WekaClient names based on existing WekaClient resources in the cluster,
// filtered by namespace flags. Should be called as completion for --client flag in other commands
func completionListWekaClients(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clients, err := kubernetes.NewK8sClients(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completion.GetCandidatesMatchingCompletion(
		cmd,
		args,
		toComplete,
		&v1alpha1.WekaClient{},
		clients.CRClient,
	).Strings(), cobra.ShellCompDirectiveNoFileComp
}

// completionListWekaClientsAsArgs provides completion for WekaClient names for commands that take WekaClient as a positional argument
// (like get client-instances), based on existing WekaClient resources in the cluster, filtered by namespace flags,
// but also suggests possible flags if no client name is provided or if a client name is already provided
// (flags are only suggested if they are not already used in the args)
func completionListWekaClientsAsArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var drivers []string
	if len(args) < 1 {
		drivers, _ = completionListWekaClients(cmd, args, toComplete)
	}
	// Merge with possible flags
	flags := completion.SuggestAllUnusedFlagsWithUsageForCompletion(cmd, args, toComplete)
	return append(drivers, flags...), cobra.ShellCompDirectiveNoFileComp
}

func completionListWekaClusters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clients, err := kubernetes.NewK8sClients(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completion.GetCandidatesMatchingCompletion(
		cmd,
		args,
		toComplete,
		&v1alpha1.WekaCluster{},
		clients.CRClient,
	).Strings(), cobra.ShellCompDirectiveNoFileComp
}

// completionListWekaClustersAsArgs provides completion for WekaClient names for commands that take WekaClient as a positional argument
// (like get client-instances), based on existing WekaClient resources in the cluster, filtered by namespace flags,
// but also suggests possible flags if no client name is provided or if a client name is already provided
// (flags are only suggested if they are not already used in the args)
func completionListWekaClustersAsArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var drivers []string
	if len(args) < 1 {
		drivers, _ = completionListWekaClusters(cmd, args, toComplete)
	}
	// Merge with possible flags
	flags := completion.SuggestAllUnusedFlagsWithUsageForCompletion(cmd, args, toComplete)
	return append(drivers, flags...), cobra.ShellCompDirectiveNoFileComp
}

func completionListWekaContainers(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clients, err := kubernetes.NewK8sClients(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completion.GetCandidatesMatchingCompletion(
		cmd,
		args,
		toComplete,
		&v1alpha1.WekaContainer{},
		clients.CRClient,
	).Strings(), cobra.ShellCompDirectiveNoFileComp
}

// completionListWekaCsiDriversAsArgs provides completion for CSI driver names for get csi-drivers command, based on
// existing CSIDriver resources in the cluster, filtered by namespace flags, but also suggests possible flags if no
// driver name is provided or if a driver name is already provided (flags are only suggested if they are not already used in the args)
// this function should be called for positional arguments (as cmd.ValidArgsFunction) and not for flags, to provide
// a more user-friendly experience when the user starts typing the command without any args, as well as when they have
// already provided a driver name and are now looking for flags to further filter or format the output
func completionListWekaCsiDriversAsArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var drivers []string
	if len(args) < 1 {
		drivers, _ = completionListWekaCsiDrivers(cmd, args, toComplete)
	}
	// Merge with possible flags
	flags := completion.SuggestAllUnusedFlagsWithUsageForCompletion(cmd, args, toComplete)
	return append(drivers, flags...), cobra.ShellCompDirectiveNoFileComp
}

// completionListWekaCsiDrivers provides completion for CSI driver names for get csi-drivers command, based on existing
// CSIDriver resources in the cluster, filtered by namespace flags. Should be called as completion for flag --driver
func completionListWekaCsiDrivers(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clients, err := kubernetes.NewK8sClients(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completion.GetCandidatesMatchingCompletion(
		cmd,
		args,
		toComplete,
		&storagev1.CSIDriver{},
		clients.CRClient,
	).Strings(), cobra.ShellCompDirectiveNoFileComp
}

// completionListNodeSelectors provides completion for comma-separated node selectors (key=value[,key2=value2,...])
func completionListNodeSelectors(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clients, err := kubernetes.NewK8sClients(context.Background())
	if err != nil {
		fmt.Printf("AAAA")
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer clients.Stop()
	completions := completion.CompleteNodeSelector(ctx, clients.CRClient, toComplete)
	// Determine if we are completing a value (after = in the last segment)
	segments := strings.Split(toComplete, ",")
	current := segments[len(segments)-1]
	if eq := strings.Index(current, "="); eq >= 0 {
		// If the only completion is a comma, return it with NoSpace (so user can add another selector)
		if len(completions) == 1 && completions[0] == "," {
			return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
		}
		// Completing a value: do not add space after completion
		return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}
	// Otherwise, normal completion (keys, etc)
	return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// =======================================================================
// Github-based completions (fetching release versions from public repos)
// =======================================================================

// completionListOperatorVersions provides all releases of github PUBLIC releases from public github repo https://github.com/weka/weka-operator and strips the v from them
func completionListOperatorVersions(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	versions, err := completion.FetchGithubReleaseVersions("weka/weka-operator", toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return versions, cobra.ShellCompDirectiveNoFileComp
}

// completionListCSIVersions provides all releases of github PUBLIC releases from public github repo https://github.com/weka/csi-wekafs and strips the v from them
func completionListCSIVersions(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	versions, err := completion.FetchGithubReleaseVersions("weka/csi-wekafs", toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return versions, cobra.ShellCompDirectiveNoFileComp
}

// ===============================================================================================
// output format completions (generic function for managing -o / --option + wrappers per command)
// ===============================================================================================

// completionListOutputFormats is the generic function that provides completion for -o/--output flag,
// supports formats and custom-columns, with column suggestions
func completionListOutputFormats(_ *cobra.Command, _ []string, toComplete string, columns []string) ([]string, cobra.ShellCompDirective) {
	formats := []string{"wide", "json", "yaml", "custom-columns="}
	var suggestions []string
	if strings.HasPrefix(toComplete, "custom-columns=") {
		prefix := "custom-columns="
		entered := strings.TrimPrefix(toComplete, prefix)
		if entered == "" {
			for _, col := range columns {
				suggestions = append(suggestions, col)
			}
			return suggestions, cobra.ShellCompDirectiveNoFileComp
		}
		parts := strings.Split(entered, ",")
		used := map[string]struct{}{}
		for _, p := range parts {
			used[p] = struct{}{}
		}
		last := parts[len(parts)-1]
		// If last part is empty (ends with comma), suggest all unused columns (just the column name)
		if last == "" {
			for _, col := range columns {
				if _, ok := used[col]; !ok {
					suggestions = append(suggestions, col)
				}
			}
			return suggestions, cobra.ShellCompDirectiveNoFileComp
		}
		// If last part matches a column exactly, suggest comma
		for _, col := range columns {
			if last == col {
				return []string{","}, cobra.ShellCompDirectiveNoFileComp
			}
		}
		// Otherwise, suggest columns that match the last part (just the column name)
		for _, col := range columns {
			if _, ok := used[col]; ok && col != last {
				continue
			}
			if strings.HasPrefix(col, last) {
				suggestions = append(suggestions, col)
			}
		}
		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}
	// Only suggest formats at the top level
	for _, f := range formats {
		if strings.HasPrefix(f, toComplete) {
			suggestions = append(suggestions, f)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completionGetClientInstancesOutput completes the -o/--output flag for get client-instances
func completionGetClientInstancesOutput(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	columns := []string{
		"NAMESPACE", "WEKACLIENT", "NODE", "WEKACONTAINER", "WC_STATUS", "POD_STATUS",
		"JOINED", "CONTAINER_ID", "MGMT_IPS", "MGMT_IP", "ACTIVE_MOUNTS", "CPU_UTIL", "NODE_SELECTOR",
	}
	return completionListOutputFormats(cmd, args, toComplete, columns)
}

// completionGetClientInstancesOutput completes the -o/--output flag for get cluster-instances
func completionGetClusterInstancesOutput(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	columns := []string{
		"NAMESPACE", "WEKACLUSTER", "NODE", "WEKACONTAINER", "WC_STATUS", "POD_STATUS",
		"MGMT_IP", "CONTAINER_ID", "CPU_UTIL", "AGE",
	}
	return completionListOutputFormats(cmd, args, toComplete, columns)
}

// completionGetCsiDriversOutput completes the -o/--output flag for get csi-drivers
func completionGetCsiDriversOutput(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	columns := []string{
		"CSI_DRIVER", "MANAGED_BY", "NAMESPACE", "CONTROLLER", "NODE_DAEMONSET",
		"STORAGECLASSES", "PVS", "PVCS", "BOUND_PVS", "AGE",
	}
	return completionListOutputFormats(cmd, args, toComplete, columns)
}

// completionGetCsiInstancesOutput completes the -o/--output flag for get csi-instances
func completionGetCsiInstancesOutput(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	columns := []string{
		"CSI_DRIVER", "NAMESPACE", "NODE", "ROLE", "POD_NAME",
		"STATUS", "RESTARTS", "LAST_RESTART", "AGE",
	}
	return completionListOutputFormats(cmd, args, toComplete, columns)
}

// completionGetNodesOutput completes the -o/--output flag for get nodes
func completionGetNodesOutput(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	columns := []string{
		"NAME", "IP", "OS", "ARCH", "KERNEL", "STATUS", "HP2MI_USABLE",
		"HP2MI_ALLOC", "HP2MI_FREE", "CORES_USABLE", "CORES_ALLOC", "CORES_FREE",
		"RAM_USABLE", "RAM_ALLOC", "RAM_FREE",
	}
	return completionListOutputFormats(cmd, args, toComplete, columns)
}

// completionGetPoliciesOutput completes the -o/--output flag for get policies
func completionGetPoliciesOutput(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	columns := []string{"NAMESPACE", "NAME", "AGE", "TYPE", "STATUS", "PROGRESS"}
	return completionListOutputFormats(cmd, args, toComplete, columns)
}

// ===============================================================================================
// misc hardcoded completions (not based on dynamic data, but still specific to certain commands)
// ===============================================================================================

// completionGetCsiInstancesRoles provides completion for the ROLE column in get csi-instances, which can be either "controller" or "node"
func completionGetCsiInstancesRoles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	roles := []string{"controller", "node"}
	var suggestions []string
	for _, role := range roles {
		if strings.HasPrefix(role, toComplete) {
			suggestions = append(suggestions, role)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completionListWekaContainerRoles provides completion for the role label of WekaContainers, which can be "compute", "drive", "envoy", "nfs", "client" or "s3"
func completionListWekaContainerRoles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	roles := []string{"compute", "drive", "envoy", "nfs", "client", "s3"}
	var suggestions []string
	for _, role := range roles {
		if strings.HasPrefix(role, toComplete) {
			suggestions = append(suggestions, role)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completionListArchitectures provides completions for architectures (amd64, arm64) as a comma-separated list
func completionListArchitectures(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	archs := []string{"amd64", "arm64"}
	// Split by comma, trim spaces
	parts := strings.Split(toComplete, ",")
	used := map[string]struct{}{}
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" && i == len(parts)-1 {
			continue // last empty part, means user is typing next arch
		}
		used[p] = struct{}{}
	}
	// If last part is empty (ends with comma), suggest remaining archs
	if strings.HasSuffix(toComplete, ",") {
		var suggestions []string
		for _, arch := range archs {
			if _, ok := used[arch]; !ok {
				suggestions = append(suggestions, arch)
			}
		}
		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}
	// Otherwise, suggest archs that match the last part and are not used
	last := strings.TrimSpace(parts[len(parts)-1])
	var suggestions []string
	for _, arch := range archs {
		if _, ok := used[arch]; ok && arch != last {
			continue
		}
		if strings.HasPrefix(arch, last) {
			suggestions = append(suggestions, arch)
		}
	}
	// If the last part is a full arch, suggest comma
	for _, arch := range archs {
		if last == arch {
			return []string{","}, cobra.ShellCompDirectiveNoFileComp
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

//==========================================================================================
//Filesystem-based completion that works on matching particular rules for files/directories
//==========================================================================================

// completionListAllTarGzFilesInDirectory provides completion for .tar.gz and .tgz files in the current directory, as well as directories for navigation
func completionListAllTarGzFilesInDirectory(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	matchFunc := func(entry os.DirEntry, _ string) bool {
		if entry.IsDir() {
			return true // Always suggest directories for navigation
		}
		return strings.HasSuffix(entry.Name(), ".tar.gz") || strings.HasSuffix(entry.Name(), ".tgz")
	}
	suggestions, err := completion.ListPatternMatchesInDirectory(toComplete, matchFunc)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completionListAllYamlFilesInDirectory provides completion for .yaml and .yml files in the current directory, as well as directories for navigation
func completionListAllYamlFilesInDirectory(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	matchFunc := func(entry os.DirEntry, _ string) bool {
		if entry.IsDir() {
			return true // Always suggest directories for navigation
		}
		return strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")
	}

	suggestions, err := completion.ListPatternMatchesInDirectory(toComplete, matchFunc)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}
