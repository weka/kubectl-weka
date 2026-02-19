package cmd

import (
	"context"
	"fmt"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

// ImageValidationModule validates image configuration
type ImageValidationModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&ImageValidationModule{})
}

func (m *ImageValidationModule) Name() string {
	return "image_validation"
}

func (m *ImageValidationModule) FriendlyName() string {
	return "Image Configuration"
}

func (m *ImageValidationModule) Description() string {
	return "Validates WEKA image is specified and valid"
}

func (m *ImageValidationModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Image}}"
}

func (m *ImageValidationModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ImageValidationModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ImageValidationModule) SuggestedResolutionTemplate() string {
	return "Specify a valid WEKA image in the format: registry/repository:tag (e.g., quay.io/weka.io/weka-in-container:4.4.10.200)"
}

func (m *ImageValidationModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeCluster, WekaConfigTypeClient}
}

func (m *ImageValidationModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	var image string
	var objectType string

	if config.Cluster != nil {
		image = config.Cluster.Spec.Image
		objectType = "WekaCluster"
	} else if config.Client != nil {
		image = config.Client.Spec.Image
		objectType = "WekaClient"
	}

	status := "success"
	issue := ""

	if image == "" {
		status = "error"
		issue = fmt.Sprintf("Image not specified in %s", objectType)
	} else if !strings.Contains(image, ":") {
		status = "warning"
		issue = fmt.Sprintf("Image '%s' does not specify a tag - will use 'latest' which is not recommended for production", image)
	}

	return map[string]interface{}{
		"Status":     status,
		"Issue":      issue,
		"Image":      image,
		"ObjectType": objectType,
	}, nil
}

// ClientTargetValidationModule validates WekaClient target configuration
type ClientTargetValidationModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&ClientTargetValidationModule{})
}

func (m *ClientTargetValidationModule) Name() string {
	return "client_target_validation"
}

func (m *ClientTargetValidationModule) FriendlyName() string {
	return "Client Target Configuration"
}

func (m *ClientTargetValidationModule) Description() string {
	return "Validates WekaClient has either targetCluster or joinIpPorts configured"
}

func (m *ClientTargetValidationModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *ClientTargetValidationModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ClientTargetValidationModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ClientTargetValidationModule) SuggestedResolutionTemplate() string {
	return "Set either 'targetCluster' to connect to a WekaCluster in the same Kubernetes cluster, or 'joinIps' to connect to an external WEKA cluster"
}

func (m *ClientTargetValidationModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeClient}
}

func (m *ClientTargetValidationModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("client target validation requires WekaClient")
	}

	client := config.Client
	hasTargetCluster := client.Spec.TargetCluster.Name != ""
	hasJoinIps := len(client.Spec.JoinIps) > 0

	status := "success"
	issue := ""
	detail := ""

	if !hasTargetCluster && !hasJoinIps {
		status = "error"
		issue = "Neither targetCluster nor joinIps is configured - client cannot connect to any WEKA cluster"
	} else if hasTargetCluster && hasJoinIps {
		status = "warning"
		issue = "Both targetCluster and joinIps are configured - targetCluster will take precedence"
		detail = fmt.Sprintf("Target cluster: %s/%s (joinIps will be ignored)",
			client.Spec.TargetCluster.Namespace, client.Spec.TargetCluster.Name)
	} else if hasTargetCluster {
		targetNs := client.Spec.TargetCluster.Namespace
		if targetNs == "" {
			targetNs = client.Namespace
		}
		detail = fmt.Sprintf("Target cluster: %s/%s", targetNs, client.Spec.TargetCluster.Name)
	} else {
		detail = fmt.Sprintf("External cluster: %s", strings.Join(client.Spec.JoinIps, ", "))
	}

	return map[string]interface{}{
		"Status":           status,
		"Issue":            issue,
		"Detail":           detail,
		"HasTargetCluster": hasTargetCluster,
		"HasJoinIps":       hasJoinIps,
		"TargetCluster":    fmt.Sprintf("%s/%s", client.Spec.TargetCluster.Namespace, client.Spec.TargetCluster.Name),
		"JoinIps":          client.Spec.JoinIps,
	}, nil
}

// HotSpareValidationModule validates hotSpare configuration for WekaCluster
type HotSpareValidationModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&HotSpareValidationModule{})
}

func (m *HotSpareValidationModule) Name() string {
	return "hotspare_validation"
}

func (m *HotSpareValidationModule) FriendlyName() string {
	return "Hot Spare Configuration"
}

func (m *HotSpareValidationModule) Description() string {
	return "Validates hot spare is configured for production clusters"
}

func (m *HotSpareValidationModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: Hot spare set to {{.HotSpare}}"
}

func (m *HotSpareValidationModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *HotSpareValidationModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *HotSpareValidationModule) SuggestedResolutionTemplate() string {
	return "Set 'hotSpare' to at least 1 for production clusters to handle drive failures. Recommended: 1-2 for small clusters, 2-3 for large clusters"
}

func (m *HotSpareValidationModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeCluster}
}

func (m *HotSpareValidationModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	if config.Cluster == nil {
		return nil, fmt.Errorf("hotspare validation requires WekaCluster")
	}

	hotSpare := config.Cluster.Spec.HotSpare

	status := "success"
	issue := ""

	if hotSpare == 0 {
		status = "warning"
		issue = "Hot spare is set to 0. At least 1 hot spare is recommended for production clusters to handle drive failures"
	}

	return map[string]interface{}{
		"Status":   status,
		"Issue":    issue,
		"HotSpare": hotSpare,
	}, nil
}

// NetworkUDPModeValidationModule validates UDP mode configuration
type NetworkUDPModeValidationModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&NetworkUDPModeValidationModule{})
}

func (m *NetworkUDPModeValidationModule) Name() string {
	return "network_udp_mode_validation"
}

func (m *NetworkUDPModeValidationModule) FriendlyName() string {
	return "Network UDP Mode"
}

func (m *NetworkUDPModeValidationModule) Description() string {
	return "Validates network UDP mode configuration"
}

func (m *NetworkUDPModeValidationModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: UDP mode disabled (recommended for production)"
}

func (m *NetworkUDPModeValidationModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NetworkUDPModeValidationModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NetworkUDPModeValidationModule) SuggestedResolutionTemplate() string {
	return "For production environments, set 'network.udpMode' to false to use RDMA/RoCE for better performance"
}

func (m *NetworkUDPModeValidationModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeCluster}
}

func (m *NetworkUDPModeValidationModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	if config.Cluster == nil {
		return nil, fmt.Errorf("UDP mode validation requires WekaCluster")
	}

	udpMode := config.Cluster.Spec.Network.UdpMode

	status := "success"
	issue := ""

	if udpMode {
		status = "warning"
		issue = "UDP mode is enabled. This is not recommended for fast-performance production environments. Consider using RDMA/RoCE instead"
	}

	return map[string]interface{}{
		"Status":  status,
		"Issue":   issue,
		"UdpMode": udpMode,
	}, nil
}

// EthDeviceValidationModule validates network interface name
type EthDeviceValidationModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&EthDeviceValidationModule{})
}

func (m *EthDeviceValidationModule) Name() string {
	return "ethdevice_validation"
}

func (m *EthDeviceValidationModule) FriendlyName() string {
	return "Network Interface Name"
}

func (m *EthDeviceValidationModule) Description() string {
	return "Validates network interface (ethDevice) name format"
}

func (m *EthDeviceValidationModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: '{{.EthDevice}}' is valid"
}

func (m *EthDeviceValidationModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *EthDeviceValidationModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *EthDeviceValidationModule) SuggestedResolutionTemplate() string {
	return "Use a valid network interface name (e.g., eth0, bond0, mlnx0, bond0.12 for VLAN). Avoid special characters except dot (.) for VLAN interfaces"
}

func (m *EthDeviceValidationModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeCluster}
}

func (m *EthDeviceValidationModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	if config.Cluster == nil {
		return nil, fmt.Errorf("ethDevice validation requires WekaCluster")
	}

	ethDevice := config.Cluster.Spec.Network.EthDevice

	status := "success"
	issue := ""

	if ethDevice == "" {
		status = "error"
		issue = "Network interface (ethDevice) is not specified"
	} else if strings.Contains(ethDevice, ":") {
		status = "error"
		issue = fmt.Sprintf("Invalid network interface name '%s': colon (:) is not allowed", ethDevice)
	} else if !isValidEthDeviceName(ethDevice) {
		status = "warning"
		issue = fmt.Sprintf("Network interface name '%s' contains unusual characters - ensure it matches the actual interface name on nodes", ethDevice)
	}

	return map[string]interface{}{
		"Status":    status,
		"Issue":     issue,
		"EthDevice": ethDevice,
	}, nil
}

// isValidEthDeviceName checks if the network interface name is reasonable
func isValidEthDeviceName(name string) bool {
	if name == "" {
		return false
	}

	// Allow alphanumeric, underscore, hyphen, and dot (for VLAN)
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '-' || char == '.') {
			return false
		}
	}

	return true
}

// CoresNumberValidationModule validates coresNumber configuration for WekaClient
type CoresNumberValidationModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&CoresNumberValidationModule{})
}

func (m *CoresNumberValidationModule) Name() string {
	return "cores_number_validation"
}

func (m *CoresNumberValidationModule) FriendlyName() string {
	return "Cores Number Configuration"
}

func (m *CoresNumberValidationModule) Description() string {
	return "Validates coresNumber is set and valid"
}

func (m *CoresNumberValidationModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.CoresNumber}} core(s) configured"
}

func (m *CoresNumberValidationModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CoresNumberValidationModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CoresNumberValidationModule) SuggestedResolutionTemplate() string {
	return "Set 'coresNumber' to a positive integer (e.g., 1, 2, 4) based on your workload requirements"
}

func (m *CoresNumberValidationModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeClient}
}

func (m *CoresNumberValidationModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("cores number validation requires WekaClient")
	}

	coresNumber := config.Client.Spec.CoresNumber

	status := "success"
	issue := ""

	if coresNumber <= 0 {
		status = "error"
		issue = "CoresNumber must be greater than 0"
	}

	return map[string]interface{}{
		"Status":      status,
		"Issue":       issue,
		"CoresNumber": coresNumber,
	}, nil
}

// TargetClusterExistsValidationModule validates if targetCluster exists in Kubernetes
type TargetClusterExistsValidationModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&TargetClusterExistsValidationModule{})
}

func (m *TargetClusterExistsValidationModule) Name() string {
	return "target_cluster_exists_validation"
}

func (m *TargetClusterExistsValidationModule) FriendlyName() string {
	return "Target Cluster Existence"
}

func (m *TargetClusterExistsValidationModule) Description() string {
	return "Validates if target WekaCluster exists in Kubernetes"
}

func (m *TargetClusterExistsValidationModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: Target cluster '{{.TargetClusterRef}}' found in Kubernetes"
}

func (m *TargetClusterExistsValidationModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *TargetClusterExistsValidationModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *TargetClusterExistsValidationModule) SuggestedResolutionTemplate() string {
	return "If you plan to deploy both cluster and client on the same Kubernetes cluster, run 'kubectl weka plan converged' instead. Otherwise, ensure the WekaCluster '{{.TargetClusterRef}}' exists before deploying the client"
}

func (m *TargetClusterExistsValidationModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeClient}
}

func (m *TargetClusterExistsValidationModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("target cluster validation requires WekaClient")
	}

	client := config.Client

	// Only validate if targetCluster is specified
	if client.Spec.TargetCluster.Name == "" {
		// Not using targetCluster, skip this validation
		return map[string]interface{}{
			"Status": "success",
			"Skip":   true,
		}, nil
	}

	targetNs := client.Spec.TargetCluster.Namespace
	if targetNs == "" {
		targetNs = client.Namespace
	}
	targetClusterRef := fmt.Sprintf("%s/%s", targetNs, client.Spec.TargetCluster.Name)

	// Check if cluster exists
	var cluster wekaapi.WekaCluster
	clusterKey := ctrlclient.ObjectKey{
		Namespace: targetNs,
		Name:      client.Spec.TargetCluster.Name,
	}

	err := KubeClients.CRClient.Get(ctx, clusterKey, &cluster)

	status := "success"
	issue := ""
	exists := err == nil

	if !exists {
		status = "warning"
		issue = fmt.Sprintf("Target cluster '%s' does not exist in Kubernetes. Are you sure? If you plan to deploy a cluster on same Kubernetes cluster, it is recommended to run 'kubectl weka plan converged' instead", targetClusterRef)
	}

	return map[string]interface{}{
		"Status":           status,
		"Issue":            issue,
		"TargetClusterRef": targetClusterRef,
		"Exists":           exists,
	}, nil
}

// DriversDistServiceValidationModule validates drivers distribution service configuration
// This module validates cluster OR client individually (not cross-validation)
type DriversDistServiceValidationModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&DriversDistServiceValidationModule{})
}

func (m *DriversDistServiceValidationModule) Name() string {
	return "drivers_dist_service_validation"
}

func (m *DriversDistServiceValidationModule) FriendlyName() string {
	return "Drivers Distribution Service"
}

func (m *DriversDistServiceValidationModule) Description() string {
	return "Validates drivers distribution service configuration"
}

func (m *DriversDistServiceValidationModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *DriversDistServiceValidationModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *DriversDistServiceValidationModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *DriversDistServiceValidationModule) SuggestedResolutionTemplate() string {
	return "Ensure driversDistService is configured correctly. For external services, use full URL (e.g., https://10.240.200.5:14000). For Kubernetes services, ensure the service exists"
}

func (m *DriversDistServiceValidationModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeCluster, WekaConfigTypeClient}
}

func (m *DriversDistServiceValidationModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	var driversDistService string

	// Get service from whichever object is being validated
	if config.Cluster != nil {
		driversDistService = config.Cluster.Spec.DriversDistService
	} else if config.Client != nil {
		driversDistService = config.Client.Spec.DriversDistService
	}

	// If not configured, skip silently (it's optional)
	if driversDistService == "" {
		return map[string]interface{}{
			"Status": "success",
			"Skip":   true,
		}, nil
	}

	status := "success"
	issue := ""
	detail := ""

	// Validate the service URL/path
	if !(strings.Contains(driversDistService, "http://") || strings.Contains(driversDistService, "https://")) {
		// External URL
		status = "error"
		issue = fmt.Sprintf("driversDistService '%s' does not appear to be a valid URL (missing http:// or https://)", driversDistService)
		detail = issue
		goto ERROR
	}
	if strings.Contains(driversDistService, "cluster.local") {
		// Kubernetes service
		detail = fmt.Sprintf("Kubernetes service configured: %s", driversDistService)
	} else if driversDistService == "https://drivers.weka.io" {
		detail = "Using default public drivers distribution service (https://drivers.weka.io)"
	} else {
		// Invalid format
		detail = fmt.Sprintf("Custom external service configured: %s", driversDistService)
	}
ERROR:
	return map[string]interface{}{
		"Status":             status,
		"Issue":              issue,
		"Detail":             detail,
		"DriversDistService": driversDistService,
	}, nil
}

// DriversDistServiceConsistencyModule validates consistency between cluster and client driversDistService
// This module only applies when BOTH cluster and client are present (e.g., in plan converged)
type DriversDistServiceConsistencyModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&DriversDistServiceConsistencyModule{})
}

func (m *DriversDistServiceConsistencyModule) Name() string {
	return "drivers_dist_service_consistency"
}

func (m *DriversDistServiceConsistencyModule) FriendlyName() string {
	return "Drivers Distribution Service Consistency"
}

func (m *DriversDistServiceConsistencyModule) Description() string {
	return "Validates that driversDistService is consistent between cluster and client"
}

func (m *DriversDistServiceConsistencyModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: Both cluster and client use same service: {{.Service}}"
}

func (m *DriversDistServiceConsistencyModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *DriversDistServiceConsistencyModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *DriversDistServiceConsistencyModule) SuggestedResolutionTemplate() string {
	return "Ensure driversDistService is set to the same value in both WekaCluster and WekaClient configurations"
}

func (m *DriversDistServiceConsistencyModule) AppliesTo() []WekaConfigObjectType {
	// This module only makes sense when both cluster AND client are present
	return []WekaConfigObjectType{WekaConfigTypeCluster, WekaConfigTypeClient}
}

func (m *DriversDistServiceConsistencyModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	// Only validate if BOTH cluster and client are present
	if config.Cluster == nil || config.Client == nil {
		return map[string]interface{}{
			"Status": "success",
			"Skip":   true,
		}, nil
	}

	clusterService := config.Cluster.Spec.DriversDistService
	clientService := config.Client.Spec.DriversDistService

	status := "success"
	issue := ""
	service := ""

	// If both are empty, that's fine (both use default)
	if clusterService == "" && clientService == "" {
		return map[string]interface{}{
			"Status": "success",
			"Skip":   true,
		}, nil
	}

	// If only one is configured, that might be intentional (one uses default)
	if clusterService == "" && clientService != "" {
		status = "warning"
		issue = fmt.Sprintf("Client has driversDistService configured (%s) but cluster does not (using default)", clientService)
	} else if clusterService != "" && clientService == "" {
		status = "warning"
		issue = fmt.Sprintf("Cluster has driversDistService configured (%s) but client does not (using default)", clusterService)
	} else if clusterService != clientService {
		// Both configured but different - this is a problem
		status = "warning"
		issue = fmt.Sprintf("Mismatch between cluster and client: cluster uses '%s', client uses '%s'", clusterService, clientService)
	} else {
		// Both configured and match - perfect!
		service = clusterService
	}

	return map[string]interface{}{
		"Status":         status,
		"Issue":          issue,
		"Service":        service,
		"ClusterService": clusterService,
		"ClientService":  clientService,
	}, nil
}

// ClusterClientCompatibilityModule validates that cluster and client are compatible
// This includes: targetCluster match (name and namespace), image version compatibility
type ClusterClientCompatibilityModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&ClusterClientCompatibilityModule{})
}

func (m *ClusterClientCompatibilityModule) Name() string {
	return "cluster_client_compatibility"
}

func (m *ClusterClientCompatibilityModule) FriendlyName() string {
	return "Cluster-Client Compatibility"
}

func (m *ClusterClientCompatibilityModule) Description() string {
	return "Validates that cluster and client are compatible (targetCluster match and image versions)"
}

func (m *ClusterClientCompatibilityModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *ClusterClientCompatibilityModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ClusterClientCompatibilityModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ClusterClientCompatibilityModule) SuggestedResolutionTemplate() string {
	return "Ensure client's targetCluster matches the WekaCluster and image versions are compatible"
}

func (m *ClusterClientCompatibilityModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeCluster, WekaConfigTypeClient}
}

func (m *ClusterClientCompatibilityModule) Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error) {
	// Only validate if BOTH cluster and client are present
	if config.Cluster == nil || config.Client == nil {
		return map[string]interface{}{
			"Status": "success",
			"Skip":   true,
		}, nil
	}

	client := config.Client
	cluster := config.Cluster

	// First, validate targetCluster match (if specified)
	if client.Spec.TargetCluster.Name != "" {
		// Validate namespace match
		targetNamespace := client.Spec.TargetCluster.Namespace
		if targetNamespace == "" {
			targetNamespace = client.Namespace
		}

		if targetNamespace != cluster.Namespace {
			return map[string]interface{}{
				"Status": "error",
				"Issue": fmt.Sprintf(
					"Client targetCluster namespace mismatch:\n"+
						"    Client '%s/%s' targets namespace: %s\n"+
						"    But WekaCluster is in namespace: %s",
					client.Namespace, client.Name,
					targetNamespace,
					cluster.Namespace),
			}, nil
		}

		// Validate name match
		if client.Spec.TargetCluster.Name != cluster.Name {
			return map[string]interface{}{
				"Status": "error",
				"Issue": fmt.Sprintf(
					"Client targetCluster name mismatch:\n"+
						"    Client '%s/%s' targets cluster: %s\n"+
						"    But WekaCluster name is: %s",
					client.Namespace, client.Name,
					client.Spec.TargetCluster.Name,
					cluster.Name),
			}, nil
		}
	}

	// Now validate image version compatibility
	clusterImage := cluster.Spec.Image
	clientImage := client.Spec.Image

	// If images are identical, full compatibility
	if clusterImage == clientImage {
		targetRef := fmt.Sprintf("%s/%s", cluster.Namespace, cluster.Name)
		return map[string]interface{}{
			"Status": "success",
			"Detail": fmt.Sprintf("Client and cluster compatible: targeting %s with matching image %s",
				targetRef, clusterImage),
		}, nil
	}

	// Parse versions from images
	clusterVersion, err := parseWekaVersion(clusterImage)
	if err != nil {
		return map[string]interface{}{
			"Status": "warning",
			"Issue": fmt.Sprintf(
				"Different images detected (cluster: %s, client: %s) - unable to parse versions for compatibility check",
				clusterImage, clientImage),
		}, nil
	}

	clientVersion, err := parseWekaVersion(clientImage)
	if err != nil {
		return map[string]interface{}{
			"Status": "warning",
			"Issue": fmt.Sprintf(
				"Different images detected (cluster: %s, client: %s) - unable to parse versions for compatibility check",
				clusterImage, clientImage),
		}, nil
	}

	// Compare major version
	if clusterVersion.Major != clientVersion.Major {
		return map[string]interface{}{
			"Status": "error",
			"Issue": fmt.Sprintf(
				"Major version mismatch detected (%d vs %d):\n"+
					"    Cluster image: %s (version %s)\n"+
					"    Client image:  %s (version %s)\n"+
					"    Client and cluster must use the same major version",
				clusterVersion.Major, clientVersion.Major,
				clusterImage, clusterVersion.String(),
				clientImage, clientVersion.String()),
		}, nil
	}

	// Compare minor version
	if clusterVersion.Minor != clientVersion.Minor {
		return map[string]interface{}{
			"Status": "error",
			"Issue": fmt.Sprintf(
				"Minor version mismatch detected (%d.%d vs %d.%d):\n"+
					"    Cluster image: %s (version %s)\n"+
					"    Client image:  %s (version %s)\n"+
					"    Client and cluster must use the same minor version",
				clusterVersion.Major, clusterVersion.Minor,
				clientVersion.Major, clientVersion.Minor,
				clusterImage, clusterVersion.String(),
				clientImage, clientVersion.String()),
		}, nil
	}

	// Same major.minor but different patch or build
	// Client version must be equal to or older than cluster version
	status := "success"
	detail := fmt.Sprintf("Client and cluster versions compatible: %s", clusterVersion.String())
	issue := ""

	if clientVersion.Patch < clusterVersion.Patch ||
		(clientVersion.Patch == clusterVersion.Patch && clientVersion.Build < clusterVersion.Build) {
		// Client is older - this may work but warn
		status = "warning"
		issue = fmt.Sprintf(
			"Client version is older than cluster version (not recommended):\n"+
				"    Cluster: %s (version %s)\n"+
				"    Client:  %s (version %s)\n"+
				"    Consider upgrading client to match cluster version",
			clusterImage, clusterVersion.String(),
			clientImage, clientVersion.String())
	} else if clientVersion.Patch > clusterVersion.Patch ||
		(clientVersion.Patch == clusterVersion.Patch && clientVersion.Build > clusterVersion.Build) {
		// Client is newer - not allowed
		status = "error"
		issue = fmt.Sprintf(
			"Client version is newer than cluster version (not allowed):\n"+
				"    Cluster image: %s (version %s)\n"+
				"    Client image:  %s (version %s)\n"+
				"    Client version must be equal to or older than cluster version.\n"+
				"    Please downgrade client to %s or upgrade cluster to match client version",
			clusterImage, clusterVersion.String(),
			clientImage, clientVersion.String(),
			clusterVersion.String())
	}

	return map[string]interface{}{
		"Status":         status,
		"Issue":          issue,
		"Detail":         detail,
		"ClusterImage":   clusterImage,
		"ClusterVersion": clusterVersion.String(),
		"ClientImage":    clientImage,
		"ClientVersion":  clientVersion.String(),
	}, nil
}
