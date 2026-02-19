package cmd

import (
	"fmt"
	"strings"
	"sync"
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

	// SuccessTemplate returns a message template for success reporting
	// Can include placeholders like {{.NodeName}}, {{.FriendlyName}}, etc.
	// Optional: return empty string if not applicable
	SuccessTemplate() string

	// WarningTemplate returns a message template for warning reporting
	// Can include placeholders like {{.NodeName}}, {{.FriendlyName}}, etc.
	// Optional: return empty string if not applicable
	WarningTemplate() string

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
	if r.SuggestedResolutionTemplate == "" {
		return ""
	}
	return interpolateTemplate(r.SuggestedResolutionTemplate, params)
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
// and command-specific validation configurations with caching
type HostCheckRegistry struct {
	// Modules: available validation modules
	modules map[string]HostCheckModule
	order   []string // Preserve module registration order

	// Command configs: which modules each command validates against
	commands map[string]*CommandHostCheckConfig

	// Cache: cached hostcheck results to avoid re-running
	cache struct {
		mu          sync.RWMutex
		results     HostChecksMap
		nodes       []string // Node names that were checked
		lastUpdated time.Time
	}
}

// CommandHostCheckConfig defines which validation modules a command needs
// HostChecks always collect the SAME complete data (HostChecksResult)
// This config only specifies which subset of modules to validate against
type CommandHostCheckConfig struct {
	// Command name (e.g., "plan_cluster", "preflight_nodes")
	CommandName string

	// ModuleNames lists the validation modules to run on hostcheck results
	// e.g., ["network", "nvme_drives", "cpu_memory"]
	// The hostcheck data is always the same - only validation differs
	ModuleNames []string
}

// NewHostCheckRegistry creates a new registry
func NewHostCheckRegistry() *HostCheckRegistry {
	registry := &HostCheckRegistry{
		modules:  make(map[string]HostCheckModule),
		order:    []string{},
		commands: make(map[string]*CommandHostCheckConfig),
	}
	registry.cache.results = make(HostChecksMap)
	return registry
}

// NewStandardModuleRegistry creates a registry with all standard modules and command configs
func NewStandardModuleRegistry() *HostCheckRegistry {
	registry := NewHostCheckRegistry()

	// Register all standard validation modules
	_ = registry.RegisterModule(&OSModule{})
	_ = registry.RegisterModule(&WekaDirModule{})
	_ = registry.RegisterModule(&XFSModule{})
	_ = registry.RegisterModule(&WekaClientModule{})
	_ = registry.RegisterModule(&NetworkModule{})
	_ = registry.RegisterModule(&CPUModule{})
	_ = registry.RegisterModule(&KernelModule{})
	_ = registry.RegisterModule(&NVMeDrivesModule{})

	// Register command validation configurations
	_ = registry.RegisterCommand(&CommandHostCheckConfig{
		CommandName: "preflight_nodes",
		ModuleNames: []string{
			"os", "kernel", "cpu_memory", "weka_dir",
			"xfs", "weka_client", "network", "nvme_drives",
		},
	})

	_ = registry.RegisterCommand(&CommandHostCheckConfig{
		CommandName: "plan_cluster",
		ModuleNames: []string{"network", "nvme_drives", "cpu_memory"},
	})

	_ = registry.RegisterCommand(&CommandHostCheckConfig{
		CommandName: "plan_client",
		ModuleNames: []string{"cpu_memory"},
	})

	_ = registry.RegisterCommand(&CommandHostCheckConfig{
		CommandName: "plan_converged",
		ModuleNames: []string{"network", "nvme_drives", "cpu_memory"},
	})

	return registry
}

// Global registry instance (modules + commands + cache)
var GlobalHostCheckRegistry *HostCheckRegistry

// InitializeHostCheckRegistry sets up the global registry
func InitializeHostCheckRegistry() {
	GlobalHostCheckRegistry = NewStandardModuleRegistry()
}

// RegisterModule adds a validation module to the registry
func (r *HostCheckRegistry) RegisterModule(module HostCheckModule) error {
	name := module.Name()
	if _, exists := r.modules[name]; exists {
		return fmt.Errorf("hostcheck module '%s' already registered", name)
	}
	r.modules[name] = module
	r.order = append(r.order, name)
	return nil
}

// RegisterCommand adds a command's validation configuration
func (r *HostCheckRegistry) RegisterCommand(config *CommandHostCheckConfig) error {
	if config.CommandName == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	if _, exists := r.commands[config.CommandName]; exists {
		return fmt.Errorf("command '%s' already registered", config.CommandName)
	}

	r.commands[config.CommandName] = config
	return nil
}

// GetCommand retrieves a command's validation configuration
func (r *HostCheckRegistry) GetCommand(commandName string) (*CommandHostCheckConfig, bool) {
	config, exists := r.commands[commandName]
	return config, exists
}

// GetRequiredModules returns the list of validation modules a command needs
func (r *HostCheckRegistry) GetRequiredModules(commandName string) []string {
	config, exists := r.commands[commandName]
	if !exists {
		return nil
	}
	return config.ModuleNames
}

// ============================================================================
// Cache Management
// ============================================================================

// ClearCache clears the hostcheck results cache
func (r *HostCheckRegistry) ClearCache() {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()

	r.cache.results = make(HostChecksMap)
	r.cache.nodes = nil
	r.cache.lastUpdated = time.Time{}
}

// GetCacheInfo returns information about the cache state
func (r *HostCheckRegistry) GetCacheInfo() (nodeCount int, lastUpdated time.Time) {
	r.cache.mu.RLock()
	defer r.cache.mu.RUnlock()

	return len(r.cache.results), r.cache.lastUpdated
}

// ============================================================================
// Module Access
// ============================================================================

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
		"status": "success",
		"module": m.name,
	}, nil
}

// ============================================================================
// Note: Aggregation functionality is now provided by the merged registry's
// ValidateAll() and ValidateWithModules() methods
// ============================================================================
