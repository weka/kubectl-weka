package cmd

import (
	"fmt"
	"strings"
	"time"
)

// ============================================================================
// Modular HostCheck System
// ============================================================================

// HostCheckModule defines the interface for a hostcheck validation module
type HostCheckModule interface {
	// Name returns the unique name of this module
	Name() string

	// Validate performs the validation and returns results
	// Receives the pod output and should extract/parse what it needs
	Validate(podOutput string) (interface{}, error)

	// Description returns a human-readable description of what this module checks
	Description() string
}

// HostCheckRegistry manages all available hostcheck modules
// This is part of the public API and will be used by plan_clients and other future commands
type HostCheckRegistry struct {
	modules map[string]HostCheckModule
	order   []string // Preserve registration order
}

// NewHostCheckRegistry creates a new registry
// This is part of the public API for the modular hostcheck system
// and will be used by plan_clients and other future commands.
// nolint:unused
func NewHostCheckRegistry() *HostCheckRegistry {
	return &HostCheckRegistry{
		modules: make(map[string]HostCheckModule),
		order:   []string{},
	}
}

// Register adds a module to the registry
func (r *HostCheckRegistry) Register(module HostCheckModule) error {
	name := module.Name()
	if _, exists := r.modules[name]; exists {
		return fmt.Errorf("hostcheck module '%s' already registered", name)
	}
	r.modules[name] = module
	r.order = append(r.order, name)
	return nil
}

// Get retrieves a module by name
func (r *HostCheckRegistry) Get(name string) (HostCheckModule, error) {
	module, exists := r.modules[name]
	if !exists {
		return nil, fmt.Errorf("hostcheck module '%s' not found", name)
	}
	return module, nil
}

// ListModules returns all registered module names in registration order
func (r *HostCheckRegistry) ListModules() []string {
	result := make([]string, len(r.order))
	copy(result, r.order)
	return result
}

// ValidateAll runs all registered modules and returns results
func (r *HostCheckRegistry) ValidateAll(podOutput string) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	var errors []string

	for _, name := range r.order {
		module := r.modules[name]
		result, err := module.Validate(podOutput)

		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
			results[name] = nil
		} else {
			results[name] = result
		}
	}

	var err error
	if len(errors) > 0 {
		err = fmt.Errorf("hostcheck errors: %s", strings.Join(errors, "; "))
	}

	return results, err
}

// ValidateSelected runs only specified modules
func (r *HostCheckRegistry) ValidateSelected(podOutput string, moduleNames ...string) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	var errors []string

	for _, name := range moduleNames {
		module, err := r.Get(name)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}

		result, err := module.Validate(podOutput)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
			results[name] = nil
		} else {
			results[name] = result
		}
	}

	var err error
	if len(errors) > 0 {
		err = fmt.Errorf("hostcheck errors: %s", strings.Join(errors, "; "))
	}

	return results, err
}

// ============================================================================
// Example HostCheck Modules (can be extended)
// ============================================================================

// HostCheckModuleStub is a placeholder for future module implementations
// This is part of the public API and can be used to create custom hostcheck modules
type HostCheckModuleStub struct {
	name        string
	description string
}

// NewHostCheckModuleStub creates a new stub module
// This is part of the public API and will be used by custom hostcheck module implementations
// nolint:unused
func NewHostCheckModuleStub(name, description string) *HostCheckModuleStub {
	return &HostCheckModuleStub{
		name:        name,
		description: description,
	}
}

func (m *HostCheckModuleStub) Name() string {
	return m.name
}

func (m *HostCheckModuleStub) Description() string {
	return m.description
}

func (m *HostCheckModuleStub) Validate(podOutput string) (interface{}, error) {
	// Placeholder: return success
	return map[string]interface{}{
		"status": "ok",
		"module": m.name,
	}, nil
}

// ============================================================================
// HostCheck Result Aggregator
// ============================================================================

// HostCheckAggregator collects results from multiple modules into a structured format
// This is part of the public API and will be used by plan_clients and other future commands
type HostCheckAggregator struct {
	registry  *HostCheckRegistry
	timestamp time.Time
	nodeName  string
}

// NewHostCheckAggregator creates a new aggregator
// This is part of the public API for the modular hostcheck system
// and will be used by plan_clients and other future commands.
// nolint:unused
func NewHostCheckAggregator(registry *HostCheckRegistry, nodeName string) *HostCheckAggregator {
	return &HostCheckAggregator{
		registry:  registry,
		timestamp: time.Now(),
		nodeName:  nodeName,
	}
}

// AggregatedResult represents the combined results of all hostcheck modules
type AggregatedResult struct {
	NodeName      string                 `json:"node_name"`
	Timestamp     time.Time              `json:"timestamp"`
	ModuleResults map[string]interface{} `json:"modules"`
	Errors        []string               `json:"errors,omitempty"`
	Status        string                 `json:"status"` // "success" or "partial" or "failure"
}

// Aggregate runs all modules and returns aggregated results
func (a *HostCheckAggregator) Aggregate(podOutput string) *AggregatedResult {
	results, err := a.registry.ValidateAll(podOutput)

	status := "success"
	var errors []string

	if err != nil {
		status = "partial"
		errors = append(errors, err.Error())
	}

	// Check if any module failed
	for _, result := range results {
		if result == nil {
			status = "partial"
			break
		}
	}

	return &AggregatedResult{
		NodeName:      a.nodeName,
		Timestamp:     a.timestamp,
		ModuleResults: results,
		Errors:        errors,
		Status:        status,
	}
}
