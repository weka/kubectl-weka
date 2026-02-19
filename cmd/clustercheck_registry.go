package cmd

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterCheckRegistry manages cluster check modules
type ClusterCheckRegistry struct {
	modules     map[string]ClusterCheckModule
	moduleOrder []string // maintain order of modules for consistent output
	mu          sync.RWMutex
}

// NewClusterCheckRegistry creates a new cluster check registry
func NewClusterCheckRegistry() *ClusterCheckRegistry {
	return &ClusterCheckRegistry{
		modules:     make(map[string]ClusterCheckModule),
		moduleOrder: []string{},
	}
}

// Register adds a module to the registry
func (r *ClusterCheckRegistry) Register(module ClusterCheckModule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := module.Name()
	r.modules[name] = module
	r.moduleOrder = append(r.moduleOrder, name)
}

// Get retrieves a module by name
func (r *ClusterCheckRegistry) Get(name string) (ClusterCheckModule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	mod, ok := r.modules[name]
	return mod, ok
}

// GetAllModules returns all registered modules in registration order
func (r *ClusterCheckRegistry) GetAllModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.moduleOrder
}

// ValidateAll runs all registered modules
func (r *ClusterCheckRegistry) ValidateAll(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	crClient client.Client,
	params map[string]interface{},
) (map[string]*ClusterCheckResult, error) {
	moduleNames := r.GetAllModules()
	if len(moduleNames) == 0 {
		return make(map[string]*ClusterCheckResult), nil
	}

	results := make(map[string]*ClusterCheckResult)

	for _, moduleName := range moduleNames {
		module, exists := r.Get(moduleName)
		if !exists {
			results[moduleName] = &ClusterCheckResult{
				ModuleName: moduleName,
				Status:     "error",
				Error:      fmt.Sprintf("Module %s not found", moduleName),
			}
			continue
		}

		// Use ValidateWithParams if parameters are provided
		var result interface{}
		var err error
		if len(params) > 0 {
			result, err = module.ValidateWithParams(ctx, clientset, crClient, params)
		} else {
			result, err = module.Validate(ctx, clientset, crClient)
		}

		if err != nil {
			errorTemplate := module.ErrorTemplate()
			results[moduleName] = &ClusterCheckResult{
				ModuleName:    moduleName,
				Status:        "error",
				Error:         fmt.Sprintf("Validation error: %v", err),
				ErrorTemplate: errorTemplate,
			}
		} else {
			// Extract status from the result data
			resultStatus := "success"
			if resultMap, ok := result.(map[string]interface{}); ok {
				if statusVal, ok := resultMap["Status"].(string); ok {
					resultStatus = statusVal
				}
			}

			results[moduleName] = &ClusterCheckResult{
				ModuleName:                  moduleName,
				Status:                      resultStatus,
				Data:                        result,
				SuccessTemplate:             module.SuccessTemplate(),
				WarningTemplate:             module.WarningTemplate(),
				ErrorTemplate:               module.ErrorTemplate(),
				SuggestedResolutionTemplate: module.SuggestedResolutionTemplate(),
			}
		}
	}

	return results, nil
}

// PrintCheckResults prints validation results for all checks
func (r *ClusterCheckRegistry) PrintCheckResults(results map[string]*ClusterCheckResult) {
	for _, result := range results {
		module, _ := r.Get(result.ModuleName)
		if module == nil {
			continue
		}

		// Build context params for interpolation
		contextParams := map[string]interface{}{
			"FriendlyName": module.FriendlyName(),
		}

		// Add any additional data from the module result
		if dataMap, ok := result.Data.(map[string]interface{}); ok {
			for k, v := range dataMap {
				contextParams[k] = v
			}
		}

		// Use Summary() to format the output
		displayText := result.Summary(contextParams)
		fmt.Println(displayText)
	}
}

// GlobalClusterCheckRegistry is the global instance
var GlobalClusterCheckRegistry *ClusterCheckRegistry

func init() {
	GlobalClusterCheckRegistry = NewClusterCheckRegistry()
	// Modules self-register via their own init() functions
	// No manual registration needed - just create the module and it will be automatically registered
}
