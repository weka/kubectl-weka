package wekaconfig

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/utils"
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

func (m *ImageValidationModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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

func (m *ClientTargetValidationModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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

func (m *HotSpareValidationModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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

func (m *NetworkUDPModeValidationModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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

func (m *EthDeviceValidationModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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

func (m *CoresNumberValidationModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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

func (m *TargetClusterExistsValidationModule) Validate(ctx context.Context, clients *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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

	err := clients.CRClient.Get(ctx, clusterKey, &cluster)

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

func (m *DriversDistServiceValidationModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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

func (m *DriversDistServiceConsistencyModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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
	if clusterService == "" {
		status = "warning"
		issue = fmt.Sprintf("Client has driversDistService configured (%s) but cluster does not (using default)", clientService)
	} else if clientService == "" {
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

func (m *ClusterClientCompatibilityModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
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
	clusterVersion, err := utils.ParseWekaVersion(clusterImage)
	if err != nil {
		return map[string]interface{}{
			"Status": "warning",
			"Issue": fmt.Sprintf(
				"Different images detected (cluster: %s, client: %s) - unable to parse versions for compatibility check",
				clusterImage, clientImage),
		}, nil
	}

	clientVersion, err := utils.ParseWekaVersion(clientImage)
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

// NodeSelectorConflictModule validates that different roles don't have conflicting nodeSelectors
// This is WekaCluster-only validation
type NodeSelectorConflictModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&NodeSelectorConflictModule{})
}

func (m *NodeSelectorConflictModule) Name() string {
	return "node_selector_conflict"
}

func (m *NodeSelectorConflictModule) FriendlyName() string {
	return "Role Node Selector Conflicts"
}

func (m *NodeSelectorConflictModule) Description() string {
	return "Validates that different roles don't have conflicting nodeSelectors"
}

func (m *NodeSelectorConflictModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: No conflicting nodeSelectors detected"
}

func (m *NodeSelectorConflictModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NodeSelectorConflictModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NodeSelectorConflictModule) SuggestedResolutionTemplate() string {
	return "Ensure that NFS and S3 roles have different nodeSelectors to prevent conflicts. Use 'roleNodeSelector' to specify different selectors per role"
}

func (m *NodeSelectorConflictModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeCluster}
}

func (m *NodeSelectorConflictModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
	if config.Cluster == nil {
		return nil, fmt.Errorf("node selector conflict validation requires WekaCluster")
	}

	cluster := config.Cluster
	globalSelector := cluster.Spec.NodeSelector
	roleSelectors := cluster.Spec.RoleNodeSelector

	status := "success"
	issue := ""

	// Helper function to check if two selectors are identical
	selectorsEqual := func(a, b map[string]string) bool {
		if len(a) != len(b) {
			return false
		}
		for k, v := range a {
			if b[k] != v {
				return false
			}
		}
		return true
	}

	// Check for conflicts between different roles
	conflictingRoles := []string{}

	// Get all role selectors - convert pointers to values
	nfsSelector := globalSelector
	if roleSelectors.Nfs != nil {
		nfsSelector = *roleSelectors.Nfs
	}

	s3Selector := globalSelector
	if roleSelectors.S3 != nil {
		s3Selector = *roleSelectors.S3
	}

	computeSelector := globalSelector
	if roleSelectors.Compute != nil {
		computeSelector = *roleSelectors.Compute
	}

	driveSelector := globalSelector
	if roleSelectors.Drive != nil {
		driveSelector = *roleSelectors.Drive
	}

	// NFS and S3 conflict is CRITICAL
	if selectorsEqual(nfsSelector, s3Selector) {
		status = "error"
		issue = "NFS and S3 roles have the same nodeSelector. This is not allowed as clients and protocol services cannot be on the same node"
		return map[string]interface{}{
			"Status": status,
			"Issue":  issue,
		}, nil
	}

	// Check for other role conflicts (warning level)
	if selectorsEqual(computeSelector, driveSelector) &&
		!selectorsEqual(computeSelector, globalSelector) {
		conflictingRoles = append(conflictingRoles, "compute and drive")
	}

	if selectorsEqual(computeSelector, nfsSelector) &&
		!selectorsEqual(computeSelector, globalSelector) {
		conflictingRoles = append(conflictingRoles, "compute and nfs")
	}

	if selectorsEqual(computeSelector, s3Selector) &&
		!selectorsEqual(computeSelector, globalSelector) {
		conflictingRoles = append(conflictingRoles, "compute and s3")
	}

	if selectorsEqual(driveSelector, nfsSelector) &&
		!selectorsEqual(driveSelector, globalSelector) {
		conflictingRoles = append(conflictingRoles, "drive and nfs")
	}

	if selectorsEqual(driveSelector, s3Selector) &&
		!selectorsEqual(driveSelector, globalSelector) {
		conflictingRoles = append(conflictingRoles, "drive and s3")
	}

	if len(conflictingRoles) > 0 {
		status = "warning"
		issue = fmt.Sprintf("Multiple roles have the same nodeSelector: %s. While this is allowed, ensure this is intentional",
			strings.Join(conflictingRoles, ", "))
	}

	return map[string]interface{}{
		"Status":           status,
		"Issue":            issue,
		"ConflictingRoles": conflictingRoles,
	}, nil
}

// ClusterClientNodeConflictModule validates that client doesn't share nodes with NFS or S3
// This is converged-level validation
type ClusterClientNodeConflictModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&ClusterClientNodeConflictModule{})
}

func (m *ClusterClientNodeConflictModule) Name() string {
	return "cluster_client_node_conflict"
}

func (m *ClusterClientNodeConflictModule) FriendlyName() string {
	return "Client-NFS-S3 Node Conflict"
}

func (m *ClusterClientNodeConflictModule) Description() string {
	return "Validates that client doesn't share nodes with NFS or S3 protocols in converged deployment"
}

func (m *ClusterClientNodeConflictModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: Client nodeSelector is separate from NFS and S3"
}

func (m *ClusterClientNodeConflictModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ClusterClientNodeConflictModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ClusterClientNodeConflictModule) SuggestedResolutionTemplate() string {
	return "Ensure client nodeSelector is different from both NFS and S3 roleNodeSelectors. Clients and protocol services cannot run on the same nodes"
}

func (m *ClusterClientNodeConflictModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeCluster, WekaConfigTypeClient}
}

func (m *ClusterClientNodeConflictModule) Validate(ctx context.Context, _ *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
	// Only validate if BOTH cluster and client are present
	if config.Cluster == nil || config.Client == nil {
		return map[string]interface{}{
			"Status": "success",
			"Skip":   true,
		}, nil
	}

	cluster := config.Cluster
	client := config.Client

	clientSelector := client.Spec.NodeSelector
	globalClusterSelector := cluster.Spec.NodeSelector
	roleSelectors := cluster.Spec.RoleNodeSelector

	nfsSelector := globalClusterSelector
	if roleSelectors.Nfs != nil {
		nfsSelector = *roleSelectors.Nfs
	}

	s3Selector := globalClusterSelector
	if roleSelectors.S3 != nil {
		s3Selector = *roleSelectors.S3
	}

	// Helper to check if selectors are equal
	selectorsEqual := func(a, b map[string]string) bool {
		if len(a) != len(b) {
			return false
		}
		for k, v := range a {
			if b[k] != v {
				return false
			}
		}
		return true
	}

	// Check for conflicts
	clientConflictsWithNFS := selectorsEqual(clientSelector, nfsSelector)
	clientConflictsWithS3 := selectorsEqual(clientSelector, s3Selector)

	status := "success"
	issue := ""

	if clientConflictsWithNFS && clientConflictsWithS3 {
		status = "error"
		issue = "Client nodeSelector matches both NFS and S3 nodeSelectors. Clients cannot share nodes with protocol services"
	} else if clientConflictsWithNFS {
		status = "error"
		issue = "Client nodeSelector matches NFS roleNodeSelector. Clients and NFS services cannot run on the same nodes"
	} else if clientConflictsWithS3 {
		status = "error"
		issue = "Client nodeSelector matches S3 roleNodeSelector. Clients and S3 services cannot run on the same nodes"
	}

	return map[string]interface{}{
		"Status":                 status,
		"Issue":                  issue,
		"ClientConflictsWithNFS": clientConflictsWithNFS,
		"ClientConflictsWithS3":  clientConflictsWithS3,
	}, nil
}

// ActualNodeConflictModule validates that actual matched nodes don't conflict
// This checks if different selectors match the same actual nodes in the cluster
type ActualNodeConflictModule struct{}

func init() {
	GlobalWekaConfigValidationRegistry.Register(&ActualNodeConflictModule{})
}

func (m *ActualNodeConflictModule) Name() string {
	return "actual_node_conflict"
}

func (m *ActualNodeConflictModule) FriendlyName() string {
	return "Actual Node Conflicts"
}

func (m *ActualNodeConflictModule) Description() string {
	return "Validates that actual nodes matched by selectors don't conflict (client/nfs/s3)"
}

func (m *ActualNodeConflictModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: No actual node conflicts detected"
}

func (m *ActualNodeConflictModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ActualNodeConflictModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *ActualNodeConflictModule) SuggestedResolutionTemplate() string {
	return "Verify that your nodeSelectors are exclusive between client, NFS, and S3 roles. The actual nodes matched by these selectors must not overlap"
}

func (m *ActualNodeConflictModule) AppliesTo() []WekaConfigObjectType {
	return []WekaConfigObjectType{WekaConfigTypeCluster, WekaConfigTypeClient}
}

func (m *ActualNodeConflictModule) Validate(ctx context.Context, clients *kubernetes.K8sClients, config *WekaConfigContext) (interface{}, error) {
	// Only validate if BOTH cluster and client are present
	if config.Cluster == nil || config.Client == nil {
		return map[string]interface{}{
			"Status": "success",
			"Skip":   true,
		}, nil
	}

	cluster := config.Cluster
	client := config.Client

	// Get all nodes from the cluster to match against selectors
	nodes, err := kubernetes.GetClusterNodes(ctx, clients.CRClient)
	if err != nil {
		// If we can't get nodes, we can't validate - skip this check
		return map[string]interface{}{
			"Status": "success",
			"Skip":   true,
		}, nil
	}

	clientSelector := client.Spec.NodeSelector
	globalClusterSelector := cluster.Spec.NodeSelector
	roleSelectors := cluster.Spec.RoleNodeSelector

	nfsSelector := globalClusterSelector
	if roleSelectors.Nfs != nil {
		nfsSelector = *roleSelectors.Nfs
	}

	s3Selector := globalClusterSelector
	if roleSelectors.S3 != nil {
		s3Selector = *roleSelectors.S3
	}

	// Helper to get nodes matching a selector
	getMatchingNodes := func(selector map[string]string) map[string]bool {
		matching := make(map[string]bool)
		for _, node := range nodes {
			if kubernetes.MatchesSelector(node, selector) {
				matching[node.Name] = true
			}
		}
		return matching
	}

	clientNodes := getMatchingNodes(clientSelector)
	nfsNodes := getMatchingNodes(nfsSelector)
	s3Nodes := getMatchingNodes(s3Selector)

	status := "success"
	issue := ""
	var conflictingNodes []string

	// Check for overlaps
	checkOverlap := func(nodes1, nodes2 map[string]bool) (bool, []string) {
		conflicts := []string{}
		for node := range nodes1 {
			if nodes2[node] {
				conflicts = append(conflicts, node)
			}
		}
		if len(conflicts) > 0 {
			return true, conflicts
		}
		return false, nil
	}

	// Check all combinations that should NOT overlap
	clientNFSOverlap, clientNFSNodes := checkOverlap(clientNodes, nfsNodes)
	clientS3Overlap, clientS3Nodes := checkOverlap(clientNodes, s3Nodes)
	nfsS3Overlap, nfsS3Nodes := checkOverlap(nfsNodes, s3Nodes)

	if clientNFSOverlap && clientS3Overlap {
		status = "error"
		conflictingNodes = append(clientNFSNodes, clientS3Nodes...)
		issue = fmt.Sprintf("Client nodes overlap with both NFS and S3 nodes: %s",
			strings.Join(conflictingNodes, ", "))
	} else if nfsS3Overlap {
		status = "error"
		conflictingNodes = nfsS3Nodes
		issue = fmt.Sprintf("NFS and S3 nodes overlap: %s. These services cannot run on the same nodes",
			strings.Join(conflictingNodes, ", "))
	} else if clientNFSOverlap {
		status = "error"
		conflictingNodes = clientNFSNodes
		issue = fmt.Sprintf("Client nodes overlap with NFS nodes: %s",
			strings.Join(conflictingNodes, ", "))
	} else if clientS3Overlap {
		status = "error"
		conflictingNodes = clientS3Nodes
		issue = fmt.Sprintf("Client nodes overlap with S3 nodes: %s",
			strings.Join(conflictingNodes, ", "))
	}

	return map[string]interface{}{
		"Status":           status,
		"Issue":            issue,
		"ConflictingNodes": conflictingNodes,
		"ClientNFSOverlap": clientNFSOverlap,
		"ClientS3Overlap":  clientS3Overlap,
		"NFSS3Overlap":     nfsS3Overlap,
	}, nil
}
