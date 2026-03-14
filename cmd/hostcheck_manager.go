package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ============================================================================
// HostCheck Manager - Generic hostcheck execution and cleanup
// ============================================================================

// HostChecksMap is a map of node names to their hostcheck results
type HostChecksMap map[string]*HostChecksResult

// HostCheckOptions configures hostcheck execution
type HostCheckOptions struct {
	// Namespace to create hostcheck pods in (if empty, a temporary one is created)
	Namespace string

	// LabelKey for pod labeling (default: "app")
	LabelKey string

	// LabelValue for pod labeling (default: "weka-hostcheck")
	LabelValue string

	// Timeout for waiting for pods to complete (default: 2 minutes)
	Timeout time.Duration

	// Verbose output (default: false)
	Verbose bool

	// CleanupInBackground if true, cleanup happens asynchronously (default: false)
	CleanupInBackground bool
}

// DefaultHostCheckOptions returns default options for hostcheck execution
func DefaultHostCheckOptions() HostCheckOptions {
	return HostCheckOptions{
		LabelKey:            "app",
		LabelValue:          "weka-hostcheck",
		Timeout:             2 * time.Minute,
		Verbose:             false,
		CleanupInBackground: false,
	}
}

// RunHostChecks executes hostchecks on the specified nodes and returns results
// This is a generic function that can be used by any command needing hostcheck data
func RunHostChecks(ctx context.Context, nodes []corev1.Node, opts HostCheckOptions) (HostChecksMap, error) {
	hostChecksMap := make(HostChecksMap)

	if len(nodes) == 0 {
		return hostChecksMap, nil
	}

	// Apply defaults
	if opts.LabelKey == "" {
		opts.LabelKey = "app"
	}
	if opts.LabelValue == "" {
		opts.LabelValue = "weka-hostcheck"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 2 * time.Minute
	}

	// Create temporary namespace if not provided
	namespace := opts.Namespace
	needsCleanup := false
	if namespace == "" {
		namespace = fmt.Sprintf("kubectl-hostchk-%s", randomString(8))
		needsCleanup = true

		if opts.Verbose {
			fmt.Printf("Creating temporary namespace: %s\n", namespace)
		}

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "kubectl-weka",
					"app.kubernetes.io/component":  "hostcheck",
				},
			},
		}

		if err := KubeClients.CRClient.Create(ctx, ns); err != nil {
			return nil, fmt.Errorf("failed to create temporary namespace: %w", err)
		}
	}

	// Setup cleanup (either in foreground or background)
	cleanupFunc := func() {
		if !needsCleanup {
			return
		}

		// Use fresh context for cleanup (not affected by parent context cancellation)
		cleanupCtx := context.Background()

		if opts.Verbose {
			fmt.Printf("\nCleaning up temporary namespace: %s\n", namespace)
		}

		// Delete namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		if err := KubeClients.CRClient.Delete(cleanupCtx, ns); err != nil {
			if opts.Verbose {
				fmt.Printf("  Warning: Failed to delete namespace: %v\n", err)
			}
			return
		}

		// Wait for namespace deletion (with timeout)
		if !opts.CleanupInBackground {
			if opts.Verbose {
				fmt.Printf("  Waiting for namespace deletion...")
			}

			deleteTimeout := 30 * time.Second
			deleteDeadline := time.Now().Add(deleteTimeout)

			for time.Now().Before(deleteDeadline) {
				var checkNs corev1.Namespace
				err := KubeClients.CRClient.Get(cleanupCtx, ctrlclient.ObjectKey{Name: namespace}, &checkNs)
				if err != nil {
					// Namespace not found = deleted successfully
					if opts.Verbose {
						fmt.Printf(" ✅ Done\n")
					}
					return
				}
				time.Sleep(1 * time.Second)
			}

			if opts.Verbose {
				fmt.Printf(" (timeout reached, namespace may still be deleting in background)\n")
			}
		}
	}

	// Execute cleanup based on mode
	if opts.CleanupInBackground {
		// Schedule cleanup in background goroutine
		go cleanupFunc()
	} else {
		// Cleanup in foreground after function returns
		defer cleanupFunc()
	}

	if opts.Verbose {
		fmt.Printf("Creating hostcheck pods on %d nodes...\n", len(nodes))
	}

	// Create pods in the namespace
	createdPods := make(map[string]*corev1.Pod)
	for _, node := range nodes {
		podName := fmt.Sprintf("hostchk-%s-%s", sanitizeName(node.Name), randomString(5))
		pod := makeHostChecksPod(namespace, node.Name, podName, opts.LabelKey, opts.LabelValue)

		if err := KubeClients.CRClient.Create(ctx, pod); err != nil {
			if opts.Verbose {
				fmt.Printf("  [%s] Failed to create pod: %v\n", node.Name, err)
			}
			continue
		}
		createdPods[node.Name] = pod
	}

	if opts.Verbose {
		fmt.Printf("✅ Created %d hostcheck pods\n", len(createdPods))
	}

	// Wait for pods to complete and collect results
	deadline := time.Now().Add(opts.Timeout)
	collectionErrors := make(map[string]string)

	for time.Now().Before(deadline) {
		allCompleted := true

		for nodeName, pod := range createdPods {
			var currentPod corev1.Pod
			if err := KubeClients.CRClient.Get(ctx, ctrlclient.ObjectKey{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}, &currentPod); err != nil {
				continue
			}

			if currentPod.Status.Phase == corev1.PodSucceeded {
				// Pod completed, read logs
				if _, exists := hostChecksMap[nodeName]; !exists {
					logs, err := getPodLogs(ctx, currentPod.Namespace, currentPod.Name, "hostchecks")
					if err != nil {
						collectionErrors[nodeName] = fmt.Sprintf("failed to read pod logs: %v", err)
						if opts.Verbose {
							fmt.Printf("  [%s] Failed to read logs: %v\n", nodeName, err)
						}
					} else {
						var hc HostChecksResult
						if err := json.Unmarshal([]byte(logs), &hc); err != nil {
							collectionErrors[nodeName] = fmt.Sprintf("failed to parse hostcheck JSON: %v\nFirst 200 chars of output: %s",
								err, truncateString(logs, 200))
							if opts.Verbose {
								fmt.Printf("  [%s] Failed to parse JSON: %v\n", nodeName, err)
								fmt.Printf("       Output: %s\n", truncateString(logs, 200))
							}
						} else {
							hostChecksMap[nodeName] = &hc
							hc.InitializeNetworkInterfaceHierarchy()
							if opts.Verbose {
								fmt.Printf("  [%s] ✅ Collected hostcheck data\n", nodeName)
							}
						}
					}
				}
			} else if currentPod.Status.Phase != corev1.PodPending && currentPod.Status.Phase != corev1.PodRunning {
				// Pod failed - get error info
				if len(currentPod.Status.ContainerStatuses) > 0 {
					containerStatus := currentPod.Status.ContainerStatuses[0]
					if containerStatus.State.Terminated != nil {
						msg := containerStatus.State.Terminated.Message
						if msg == "" {
							msg = containerStatus.State.Terminated.Reason
						}
						collectionErrors[nodeName] = fmt.Sprintf("pod failed: %s", msg)
						if opts.Verbose {
							fmt.Printf("  [%s] Pod failed: %s\n", nodeName, msg)
						}
					}
				} else {
					collectionErrors[nodeName] = fmt.Sprintf("pod phase: %s", currentPod.Status.Phase)
					if opts.Verbose {
						fmt.Printf("  [%s] Pod failed with phase: %s\n", nodeName, currentPod.Status.Phase)
					}
				}
				delete(createdPods, nodeName)
			} else {
				allCompleted = false
			}
		}

		if allCompleted && len(hostChecksMap) == len(createdPods) {
			break
		}

		time.Sleep(2 * time.Second)
	}

	if opts.Verbose {
		fmt.Printf("✅ Collected hostcheck data from %d/%d nodes\n", len(hostChecksMap), len(nodes))
	}

	if len(hostChecksMap) == 0 {
		// Build detailed error message with all collection errors
		var errorDetails strings.Builder
		errorDetails.WriteString("failed to collect hostcheck information from any node")
		if len(collectionErrors) > 0 {
			errorDetails.WriteString(":\n")
			for nodeName, errMsg := range collectionErrors {
				errorDetails.WriteString(fmt.Sprintf("  [%s] %s\n", nodeName, errMsg))
			}
		}
		return nil, fmt.Errorf("%s", errorDetails.String())
	}

	return hostChecksMap, nil
}

// GetNodeDrivesFromHostChecks extracts drive information from hostcheck results
func (hcm HostChecksMap) GetNodeDrivesFromHostChecks(nodeName string) []NvmeDrive {
	if result, exists := hcm[nodeName]; exists {
		return result.NVMeDrives
	}
	return nil
}

// GetFreeDrivesCount returns the number of free (unmounted) drives on a node
func (hcm HostChecksMap) GetFreeDrivesCount(nodeName string) int {
	drives := hcm.GetNodeDrivesFromHostChecks(nodeName)
	freeDrives := 0
	for _, drive := range drives {
		if !drive.Mounted {
			freeDrives++
		}
	}
	return freeDrives
}

// GetTotalDrivesCount returns the total number of drives on a node
func (hcm HostChecksMap) GetTotalDrivesCount(nodeName string) int {
	if result, exists := hcm[nodeName]; exists {
		return result.NVMeDriveCount
	}
	return 0
}

// HasDrives checks if a node has any drives
func (hcm HostChecksMap) HasDrives(nodeName string) bool {
	return hcm.GetTotalDrivesCount(nodeName) > 0
}

// GetHTEnabled returns whether hyperthreading is enabled on a node
func (hcm HostChecksMap) GetHTEnabled(nodeName string) bool {
	if result, exists := hcm[nodeName]; exists {
		return result.HTEnabled
	}
	return false
}

// GetPhysicalCores returns the number of physical cores on a node
func (hcm HostChecksMap) GetPhysicalCores(nodeName string) int {
	if result, exists := hcm[nodeName]; exists {
		return result.PhysicalCores
	}
	return 0
}

// GetLogicalCores returns the number of logical cores on a node
func (hcm HostChecksMap) GetLogicalCores(nodeName string) int {
	if result, exists := hcm[nodeName]; exists {
		return result.LogicalCores
	}
	return 0
}

// SaveToFile saves the hostcheck results to a JSON file
func (hcm HostChecksMap) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(hcm, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hostcheck results: %w", err)
	}

	if err := writeFile(filename, string(data)); err != nil {
		return fmt.Errorf("failed to write hostcheck results to file: %w", err)
	}

	return nil
}

// FormatSummary returns a formatted summary of hostcheck results
func (hcm HostChecksMap) FormatSummary() string {
	if len(hcm) == 0 {
		return "No hostcheck data available"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("HostCheck Summary (%d nodes):\n", len(hcm)))

	totalDrives := 0
	nodesWithDrives := 0
	htEnabledCount := 0

	for _, result := range hcm {
		if result.NVMeDriveCount > 0 {
			totalDrives += result.NVMeDriveCount
			nodesWithDrives++
		}
		if result.HTEnabled {
			htEnabledCount++
		}
	}

	sb.WriteString(fmt.Sprintf("  - Nodes with drives: %d/%d\n", nodesWithDrives, len(hcm)))
	sb.WriteString(fmt.Sprintf("  - Total drives: %d\n", totalDrives))
	sb.WriteString(fmt.Sprintf("  - Nodes with HT enabled: %d/%d\n", htEnabledCount, len(hcm)))

	return sb.String()
}

// ============================================================================
// Registry Extension Methods - Cached Execution
// ============================================================================

// GetHostChecksForNodes executes hostchecks on the specified nodes (or returns cached results)
// This method is smart about caching:
// - If nodes are already checked, returns cached results immediately
// - If new nodes are requested, runs hostchecks only on new nodes
// - Filters out NotReady nodes (they can't run pods)
// - Thread-safe for concurrent access
func (r *HostCheckModuleRegistry) GetHostChecksForNodes(
	ctx context.Context,
	nodes []corev1.Node,
) (HostChecksMap, error) {
	if len(nodes) == 0 {
		return make(HostChecksMap), nil
	}

	// Filter to only Ready nodes - NotReady nodes can't run hostcheck pods
	readyNodes := FilterReadyNodes(nodes)
	skippedCount := len(nodes) - len(readyNodes)

	if skippedCount > 0 {
		fmt.Printf("ℹ️  Skipping %d NotReady node(s) - they cannot run hostcheck pods\n", skippedCount)
	}

	if len(readyNodes) == 0 {
		return make(HostChecksMap), nil
	}

	// Check cache first (read lock)
	r.cache.mu.RLock()
	cacheHit := true
	var uncachedNodes []corev1.Node

	// Build set of requested node names (only ready nodes)
	requestedNodeNames := make(map[string]bool)
	for _, node := range readyNodes {
		requestedNodeNames[node.Name] = true

		// Check if this node is in cache
		if _, exists := r.cache.results[node.Name]; !exists {
			cacheHit = false
			uncachedNodes = append(uncachedNodes, node)
		}
	}

	// If all nodes are cached, return immediately
	if cacheHit {
		// Build result map with only requested nodes
		result := make(HostChecksMap)
		for nodeName := range requestedNodeNames {
			if cachedResult, exists := r.cache.results[nodeName]; exists {
				result[nodeName] = cachedResult
			}
		}
		r.cache.mu.RUnlock()

		fmt.Printf("✅ Using cached hostcheck results for %d node(s)\n", len(result))
		return result, nil
	}
	r.cache.mu.RUnlock()

	// Cache miss - need to run hostchecks on uncached nodes
	if len(uncachedNodes) < len(nodes) {
		fmt.Printf("⚙️  Using cached results for %d node(s), running hostchecks on %d new node(s)\n",
			len(nodes)-len(uncachedNodes), len(uncachedNodes))
	}

	// Run hostchecks on uncached nodes with default options
	opts := HostCheckOptions{
		Verbose:             true,
		CleanupInBackground: false,
		Timeout:             2 * time.Minute,
	}

	newResults, err := RunHostChecks(ctx, uncachedNodes, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to run hostchecks: %w", err)
	}

	// Update cache (write lock)
	r.cache.mu.Lock()
	for nodeName, result := range newResults {
		r.cache.results[nodeName] = result

		// Add to tracked nodes if not already there
		found := false
		for _, n := range r.cache.nodes {
			if n == nodeName {
				found = true
				break
			}
		}
		if !found {
			r.cache.nodes = append(r.cache.nodes, nodeName)
		}
	}
	r.cache.lastUpdated = time.Now()

	// Build complete result map with both cached and new results
	result := make(HostChecksMap)
	for nodeName := range requestedNodeNames {
		if cachedResult, exists := r.cache.results[nodeName]; exists {
			result[nodeName] = cachedResult
		}
	}
	r.cache.mu.Unlock()

	return result, nil
}

// ValidateWithModules validates hostcheck results using specified modules
// params can be used for parameterized validation (e.g., {"ethDevice": "bond0"})
func (r *HostCheckModuleRegistry) ValidateWithModules(
	commandName string,
	hostChecksMap HostChecksMap,
	params map[string]interface{},
) (map[string]map[string]*HostCheckModuleResult, error) {

	config, exists := r.GetCommand(commandName)
	if !exists {
		return nil, fmt.Errorf("command '%s' not registered", commandName)
	}

	// Results: map[nodeName]map[moduleName]*HostCheckModuleResult
	results := make(map[string]map[string]*HostCheckModuleResult)

	for nodeName, hostCheck := range hostChecksMap {
		nodeResults := make(map[string]*HostCheckModuleResult)

		for _, moduleName := range config.ModuleNames {
			module, err := r.Get(moduleName)
			if err != nil {
				// Module not found, skip
				continue
			}

			// Convert hostcheck to JSON for module validation
			hostCheckJSON, err := json.Marshal(hostCheck)
			if err != nil {
				nodeResults[moduleName] = &HostCheckModuleResult{
					ModuleName: moduleName,
					Status:     "error",
					Error:      fmt.Sprintf("Failed to marshal hostcheck: %v", err),
				}
				continue
			}

			// Use ValidateWithParams if parameters are provided
			var result interface{}

			// Create a copy of params to add node-specific data
			nodeParams := make(map[string]interface{})
			for k, v := range params {
				nodeParams[k] = v
			}
			// Add node-specific data
			nodeParams["nodeName"] = nodeName
			nodeParams["hostChecksResult"] = hostCheck

			if len(nodeParams) > 0 {
				result, err = module.ValidateWithParams(string(hostCheckJSON), nodeParams)
			} else {
				result, err = module.Validate(string(hostCheckJSON))
			}

			if err != nil {
				// Get the module to extract its error template
				module, _ := r.Get(moduleName)
				errorTemplate := ""
				if module != nil {
					errorTemplate = module.ErrorTemplate()
				}

				nodeResults[moduleName] = &HostCheckModuleResult{
					ModuleName:    moduleName,
					Status:        "error",
					Error:         fmt.Sprintf("Validation error: %v", err),
					ErrorTemplate: errorTemplate,
					Params:        map[string]interface{}{"NodeName": nodeName},
				}
			} else {
				// Get the module to extract its templates
				module, _ := r.Get(moduleName)
				successTemplate := ""
				warningTemplate := ""
				errorTemplate := ""
				suggestedResolutionTemplate := ""
				if module != nil {
					successTemplate = module.SuccessTemplate()
					warningTemplate = module.WarningTemplate()
					errorTemplate = module.ErrorTemplate()
					suggestedResolutionTemplate = module.SuggestedResolutionTemplate()
				}

				// Extract status from the result data if provided
				resultStatus := "success"
				if resultMap, ok := result.(map[string]interface{}); ok {
					if statusVal, ok := resultMap["Status"].(string); ok {
						resultStatus = statusVal
					}
				}

				nodeResults[moduleName] = &HostCheckModuleResult{
					ModuleName:                  moduleName,
					Status:                      resultStatus,
					Data:                        result,
					SuccessTemplate:             successTemplate,
					WarningTemplate:             warningTemplate,
					ErrorTemplate:               errorTemplate,
					SuggestedResolutionTemplate: suggestedResolutionTemplate,
					Params:                      map[string]interface{}{"NodeName": nodeName},
				}
			}
		}

		results[nodeName] = nodeResults
	}

	return results, nil
}

// ValidateAll runs all validation modules for a command on cached hostcheck data
// params can be used for parameterized validation (e.g., {"ethDevice": "bond0"})
func (r *HostCheckModuleRegistry) ValidateAll(
	ctx context.Context,
	commandName string,
	nodes []corev1.Node,
	params map[string]interface{},
) (map[string]map[string]*HostCheckModuleResult, error) {

	// Get hostchecks (cached or fresh)
	hostChecksMap, err := r.GetHostChecksForNodes(ctx, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to get hostchecks: %w", err)
	}

	if len(hostChecksMap) == 0 {
		return make(map[string]map[string]*HostCheckModuleResult), nil
	}

	// Validate using registered modules with params
	return r.ValidateWithModules(commandName, hostChecksMap, params)
}

// FormatNodeValidationResults formats the validation results for a single node as a string
// Returns the formatted output and overall status: "success", "partial", or "failure"
func (r *HostCheckModuleRegistry) FormatNodeValidationResults(
	nodeName string,
	commandName string,
	moduleResults map[string]*HostCheckModuleResult,
) (string, string) {
	var output strings.Builder

	// Determine overall status from module results
	overallStatus := "success"
	hasWarning := false
	hasError := false

	for _, mr := range moduleResults {
		if mr.Status == "error" {
			hasError = true
		} else if mr.Status == "warning" {
			hasWarning = true
		}
	}

	if hasError {
		overallStatus = "failure"
	} else if hasWarning {
		overallStatus = "partial"
	}

	// Format each module result using Summary()
	config, _ := r.GetCommand(commandName)
	var checkOrder []string
	if config != nil {
		checkOrder = config.ModuleNames
	}

	for _, moduleName := range checkOrder {
		moduleResult, exists := moduleResults[moduleName]
		if !exists {
			continue
		}

		// Map module name to friendly name
		friendlyName := extractFriendlyName(moduleName)

		// Create context params for Summary()
		contextParams := map[string]interface{}{
			"FriendlyName": friendlyName,
			"NodeName":     nodeName,
		}

		// Add any additional data from the module result
		if moduleResult.Data != nil {
			if dataMap, ok := moduleResult.Data.(map[string]interface{}); ok {
				for k, v := range dataMap {
					contextParams[k] = v
				}
			}
		}

		// Use Summary() to format the output
		displayText := moduleResult.Summary(contextParams)
		displayText = indentText(displayText, 5, 2) // Indent module details for better readability
		// Handle multiline details (like network configuration)
		output.WriteString(displayText + "\n")
	}

	return output.String(), overallStatus
}
