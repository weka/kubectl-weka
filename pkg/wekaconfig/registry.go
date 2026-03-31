package wekaconfig

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"sync"
)

// WekaConfigValidationRegistry manages WEKA configuration validation modules
type WekaConfigValidationRegistry struct {
	modules     map[string]WekaConfigValidationModule
	moduleOrder []string // maintain order of modules for consistent output
	mu          sync.RWMutex
}

// NewWekaConfigValidationRegistry creates a new WEKA config validation registry
func NewWekaConfigValidationRegistry() *WekaConfigValidationRegistry {
	return &WekaConfigValidationRegistry{
		modules:     make(map[string]WekaConfigValidationModule),
		moduleOrder: []string{},
	}
}

// Register adds a module to the registry
func (r *WekaConfigValidationRegistry) Register(module WekaConfigValidationModule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := module.Name()
	r.modules[name] = module
	r.moduleOrder = append(r.moduleOrder, name)
}

// Get retrieves a module by name
func (r *WekaConfigValidationRegistry) Get(name string) (WekaConfigValidationModule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	mod, ok := r.modules[name]
	return mod, ok
}

// GetAllModules returns all registered modules in registration order
func (r *WekaConfigValidationRegistry) GetAllModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.moduleOrder
}

// ValidateAll runs all registered modules that apply to the given config context
func (r *WekaConfigValidationRegistry) ValidateAll(
	ctx context.Context,
	clients *kubernetes.K8sClients,
	config *WekaConfigContext,
) (map[string]*WekaConfigValidationResult, error) {
	moduleNames := r.GetAllModules()
	if len(moduleNames) == 0 {
		return make(map[string]*WekaConfigValidationResult), nil
	}

	results := make(map[string]*WekaConfigValidationResult)

	for _, moduleName := range moduleNames {
		module, exists := r.Get(moduleName)
		if !exists {
			results[moduleName] = &WekaConfigValidationResult{
				ModuleName: moduleName,
				Status:     "error",
				Error:      fmt.Sprintf("Module %s not found", moduleName),
			}
			continue
		}

		// Check if module applies to the current context
		if !moduleAppliesToContext(module, config) {
			continue // Skip modules that don't apply
		}

		// Run validation
		result, err := module.Validate(ctx, clients, config)

		if err != nil {
			errorTemplate := module.ErrorTemplate()
			results[moduleName] = &WekaConfigValidationResult{
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

			results[moduleName] = &WekaConfigValidationResult{
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

// moduleAppliesToContext checks if a module applies to the given validation context
func moduleAppliesToContext(module WekaConfigValidationModule, config *WekaConfigContext) bool {
	appliesTo := module.AppliesTo()

	for _, objectType := range appliesTo {
		switch objectType {
		case WekaConfigTypeCluster:
			if config.Cluster != nil {
				return true
			}
		case WekaConfigTypeClient:
			if config.Client != nil {
				return true
			}
		case WekaConfigTypeContainer:
			if len(config.Containers) > 0 {
				return true
			}
		}
	}

	return false
}

// PrintValidationResults prints validation results
func (r *WekaConfigValidationRegistry) PrintValidationResults(results map[string]*WekaConfigValidationResult) {
	for _, moduleName := range r.GetAllModules() {
		result, exists := results[moduleName]
		if !exists {
			continue
		}

		module, _ := r.Get(result.ModuleName)
		if module == nil {
			continue
		}

		// Check if this validation should be skipped (not printed)
		if dataMap, ok := result.Data.(map[string]interface{}); ok {
			if skip, exists := dataMap["Skip"].(bool); exists && skip {
				continue // Skip printing this validation
			}
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

// GlobalWekaConfigValidationRegistry is the global instance - initialized immediately to avoid nil during module init
var GlobalWekaConfigValidationRegistry = NewWekaConfigValidationRegistry()
