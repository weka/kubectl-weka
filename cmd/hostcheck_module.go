package cmd

import (
	"fmt"
	"strings"
)

// ============================================================================
// Modular HostCheck System
// ============================================================================

// HostCheckModule defines the interface for a hostcheck validation module
type HostCheckModule interface {
	// Name returns the unique name of this module
	Name() ModuleName

	// Validate performs the validation and returns results
	// Receives the pod output and should extract/parse what it needs
	Validate(podOutput string) (HostCheckModuleResponse, error)

	// ValidateWithParams performs parameterized validation
	// Allows passing custom parameters (e.g., ethDevice for network validation)
	// Returns same result format as Validate
	// Default implementation should call Validate and ignore params
	ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error)

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

// interpolateTemplate replaces {{.FieldName}} placeholders with values from params
func interpolateTemplate(template string, params map[string]interface{}) string {
	result := template
	for key, value := range params {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result
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
	ModuleNames []ModuleName
}

// ============================================================================
// Example HostCheck Modules (can be extended)
// ============================================================================

// HostCheckModuleStub is a placeholder for future module implementations
// This is part of the public API and can be used to create custom hostcheck modules
type HostCheckModuleStub struct {
	name               ModuleName
	description        string
	errorTemplate      string
	resolutionTemplate string
}

// NewHostCheckModuleStub creates a new stub module
// This is part of the public API and will be used by custom hostcheck module implementations
// nolint:unused
func NewHostCheckModuleStub(name ModuleName, description string) *HostCheckModuleStub {
	return &HostCheckModuleStub{
		name:        name,
		description: description,
	}
}

func (m *HostCheckModuleStub) Name() ModuleName {
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

// HostCheckModuleResponse defines the interface for validation results returned by modules
// All modules must return a value implementing this interface
// Example fields: Status, ModuleName, Details, Error, etc.
type HostCheckModuleResponse interface {
	Status() CheckStatus
	ModuleName() ModuleName
	Details() string
	Error() error
	Map() map[string]interface{}
}

// BasicHostCheckModuleResponse is a simple implementation of HostCheckModuleResponse
// Can be used by stub and custom modules
// Extend as needed for real modules
type BasicHostCheckModuleResponse struct {
	status     CheckStatus
	moduleName ModuleName
	details    string
	err        error
}

func (r *BasicHostCheckModuleResponse) Status() CheckStatus    { return r.status }
func (r *BasicHostCheckModuleResponse) ModuleName() ModuleName { return r.moduleName }
func (r *BasicHostCheckModuleResponse) Details() string        { return r.details }
func (r *BasicHostCheckModuleResponse) Error() error           { return r.err }
func (r *BasicHostCheckModuleResponse) Map() map[string]interface{} {
	return map[string]interface{}{
		"Status":     r.status,
		"ModuleName": r.moduleName,
		"Details":    r.details,
		"Error":      r.err,
	}
}

// Update HostCheckModuleStub to return BasicHostCheckModuleResponse
func (m *HostCheckModuleStub) Validate(podOutput string) (HostCheckModuleResponse, error) {
	return &BasicHostCheckModuleResponse{
		status:     statusPass,
		moduleName: m.name,
		details:    "stub validation passed",
		err:        nil,
	}, nil
}

func (m *HostCheckModuleStub) ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error) {
	return m.Validate(podOutput)
}
