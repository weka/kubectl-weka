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

	// ValidateWithParams performs parameterized validation
	// Allows passing custom parameters (e.g., ethDevice for network validation)
	// Returns same result format as Validate
	// Default implementation should call Validate and ignore params
	ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error)

	// Description returns a human-readable description of what this module checks
	Description() string

	// ErrorTemplate returns a message template for error reporting
	// Can include placeholders like {{.NodeName}}, {{.Issue}}, etc.
	// Optional: return empty string if not applicable
	ErrorTemplate() string

	// SuggestedResolutionTemplate returns a template for suggested fixes
	// Can include placeholders like {{.NodeName}}, {{.Command}}, etc.
	// Optional: return empty string if not applicable
	SuggestedResolutionTemplate() string
}

// HostCheckResult represents the result of a single module validation
type HostCheckResult struct {
	ModuleName                  string                 `json:"module_name"`
	Status                      string                 `json:"status"` // "success", "warning", "error"
	Data                        interface{}            `json:"data,omitempty"`
	SuccessTemplate             string                 `json:"success_template,omitempty"`
	WarningTemplate             string                 `json:"warning_template,omitempty"`
	ErrorTemplate               string                 `json:"error_template,omitempty"`
	SuggestedResolutionTemplate string                 `json:"suggested_resolution_template,omitempty"`
	Error                       string                 `json:"error,omitempty"`
	SuggestedFix                string                 `json:"suggested_fix,omitempty"`
	Params                      map[string]interface{} `json:"params,omitempty"` // Context params like nodeName, etc
}

// FormatError formats an error message using the module's error template
// Params can include things like: map[string]interface{}{"NodeName": "node1", "Issue": "..."}
func (r *HostCheckResult) FormatError(params map[string]interface{}) string {
	if r.ErrorTemplate == "" {
		return r.Error
	}
	return interpolateTemplate(r.ErrorTemplate, params)
}

// FormatWarning formats a warning message using the module's warning template
// Falls back to error template if warning template is not set
func (r *HostCheckResult) FormatWarning(params map[string]interface{}) string {
	if r.WarningTemplate != "" {
		return interpolateTemplate(r.WarningTemplate, params)
	}
	// Fallback to error template for warnings if no specific warning template
	if r.ErrorTemplate != "" {
		return interpolateTemplate(r.ErrorTemplate, params)
	}
	return r.Error
}

// FormatSuccess formats the success message using the success template
func (r *HostCheckResult) FormatSuccess(params map[string]interface{}) string {
	if r.SuccessTemplate == "" {
		return ""
	}
	return interpolateTemplate(r.SuccessTemplate, params)
}

// FormatSuggestedFix formats the suggested resolution using the template
func (r *HostCheckResult) FormatSuggestedFix(params map[string]interface{}) string {
	if r.SuggestedFix == "" {
		return ""
	}
	return r.SuggestedFix
}

// Summary returns a formatted string with status and relevant text
// Format depends on the status:
// - success: uses FormatSuccess output
// - warning: uses FormatWarning output
// - error: uses FormatError output
func (r *HostCheckResult) Summary(params map[string]interface{}) string {
	var text string
	switch r.Status {
	case "success":
		text = r.FormatSuccess(params)
	case "warning":
		text = r.FormatWarning(params)
	case "error":
		text = r.FormatError(params)
	default:
		text = "⚠️ UNKNOWN"
	}
	return text
}

// interpolateTemplate replaces {{.FieldName}} placeholders with values from params
func interpolateTemplate(template string, params map[string]interface{}) string {
	result := template
	for key, value := range params {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result
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

// ValidateAll runs all registered modules and returns results with error context
func (r *HostCheckRegistry) ValidateAll(podOutput string, contextParams map[string]interface{}) (map[string]*HostCheckResult, error) {
	results := make(map[string]*HostCheckResult)
	var errors []string

	for _, name := range r.order {
		module := r.modules[name]
		result, err := module.Validate(podOutput)

		hcResult := &HostCheckResult{
			ModuleName: name,
			Params:     contextParams,
		}

		if err != nil {
			hcResult.Status = "error"
			hcResult.Error = err.Error()
			hcResult.Data = nil
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
		} else {
			hcResult.Status = "success"
			hcResult.Data = result
		}

		results[name] = hcResult
	}

	var err error
	if len(errors) > 0 {
		err = fmt.Errorf("hostcheck errors: %s", strings.Join(errors, "; "))
	}

	return results, err
}

// ValidateSelected runs only specified modules with context parameters
func (r *HostCheckRegistry) ValidateSelected(podOutput string, contextParams map[string]interface{}, moduleNames ...string) (map[string]*HostCheckResult, error) {
	results := make(map[string]*HostCheckResult)
	var errors []string

	for _, name := range moduleNames {
		module, err := r.Get(name)

		hcResult := &HostCheckResult{
			ModuleName: name,
			Params:     contextParams,
		}

		if err != nil {
			hcResult.Status = "error"
			hcResult.Error = err.Error()
			errors = append(errors, err.Error())
			results[name] = hcResult
			continue
		}

		result, err := module.Validate(podOutput)
		if err != nil {
			hcResult.Status = "error"
			hcResult.Error = err.Error()
			hcResult.Data = nil
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
		} else {
			hcResult.Status = "success"
			hcResult.Data = result
		}

		results[name] = hcResult
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
	name               string
	description        string
	errorTemplate      string
	resolutionTemplate string
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

func (m *HostCheckModuleStub) ErrorTemplate() string {
	return m.errorTemplate
}

func (m *HostCheckModuleStub) SuggestedResolutionTemplate() string {
	return m.resolutionTemplate
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
	NodeName      string
	Timestamp     time.Time
	ModuleResults map[string]*HostCheckResult `json:"modules"`
	Errors        []string                    `json:"errors,omitempty"`
	Status        string                      `json:"status"` // "success" or "partial" or "failure"
}

// Aggregate runs all modules and returns aggregated results
func (a *HostCheckAggregator) Aggregate(podOutput string) *AggregatedResult {
	contextParams := map[string]interface{}{
		"NodeName": a.nodeName,
	}
	results, err := a.registry.ValidateAll(podOutput, contextParams)

	status := "success"
	var errors []string

	if err != nil {
		status = "partial"
		errors = append(errors, err.Error())
	}

	// Check if any module failed
	for _, result := range results {
		if result.Status == "error" {
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
