package cmd

import (
	"context"
	"strings"
	"text/template"

	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

// WekaConfigValidationModule defines the interface for WEKA configuration validation modules
type WekaConfigValidationModule interface {
	// Name returns the unique identifier for this validation module
	Name() string

	// FriendlyName returns the human-readable name for this validation
	FriendlyName() string

	// Description returns a detailed description of what this validation checks
	Description() string

	// SuccessTemplate returns the template for successful validation message
	SuccessTemplate() string

	// WarningTemplate returns the template for warning validation message
	WarningTemplate() string

	// ErrorTemplate returns the template for error validation message
	ErrorTemplate() string

	// SuggestedResolutionTemplate returns the template for suggested fix
	SuggestedResolutionTemplate() string

	// AppliesTo returns which object types this validation applies to
	AppliesTo() []WekaConfigObjectType

	// Validate performs the validation check
	// Returns a map with Status, Detail, Issue, and other relevant fields
	Validate(ctx context.Context, config *WekaConfigValidationContext) (interface{}, error)
}

// WekaConfigObjectType indicates which WEKA objects the validation applies to
type WekaConfigObjectType string

const (
	WekaConfigTypeCluster   WekaConfigObjectType = "cluster"
	WekaConfigTypeClient    WekaConfigObjectType = "client"
	WekaConfigTypeContainer WekaConfigObjectType = "container"
)

// WekaConfigValidationContext holds the objects being validated
type WekaConfigValidationContext struct {
	Cluster    *wekaapi.WekaCluster
	Client     *wekaapi.WekaClient
	Containers []wekaapi.WekaContainer // Generated containers from cluster or client
}

// WekaConfigValidationResult holds the result of a validation check
type WekaConfigValidationResult struct {
	ModuleName                  string
	Status                      string // "success", "warning", "error"
	Data                        interface{}
	Error                       string
	SuccessTemplate             string
	WarningTemplate             string
	ErrorTemplate               string
	SuggestedResolutionTemplate string
	Params                      map[string]interface{}
}

// Summary returns a formatted string based on the status
func (r *WekaConfigValidationResult) Summary(params map[string]interface{}) string {
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

// FormatSuccess formats the success message using the template
func (r *WekaConfigValidationResult) FormatSuccess(params map[string]interface{}) string {
	if r.SuccessTemplate == "" {
		return "✅ OK"
	}
	return interpolateWekaConfigTemplate(r.SuccessTemplate, params)
}

// FormatWarning formats the warning message using the template
func (r *WekaConfigValidationResult) FormatWarning(params map[string]interface{}) string {
	if r.WarningTemplate == "" {
		return "⚠️ WARNING"
	}
	return interpolateWekaConfigTemplate(r.WarningTemplate, params)
}

// FormatError formats the error message using the template
func (r *WekaConfigValidationResult) FormatError(params map[string]interface{}) string {
	if r.ErrorTemplate == "" {
		if r.Error != "" {
			return "❌ ERROR: " + r.Error
		}
		return "❌ ERROR"
	}
	return interpolateWekaConfigTemplate(r.ErrorTemplate, params)
}

// FormatSuggestedFix formats the suggested resolution using the template
func (r *WekaConfigValidationResult) FormatSuggestedFix(params map[string]interface{}) string {
	if r.SuggestedResolutionTemplate == "" {
		return ""
	}
	return interpolateWekaConfigTemplate(r.SuggestedResolutionTemplate, params)
}

// interpolateWekaConfigTemplate replaces {{.FieldName}} placeholders with values from params
func interpolateWekaConfigTemplate(tmplStr string, params map[string]interface{}) string {
	if tmplStr == "" {
		return ""
	}

	// Merge Data fields into params if Data is a map
	allParams := make(map[string]interface{})
	for k, v := range params {
		allParams[k] = v
	}

	tmpl, err := template.New("validation").Parse(tmplStr)
	if err != nil {
		return tmplStr // Return original if parse fails
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, allParams); err != nil {
		return tmplStr // Return original if execution fails
	}

	return result.String()
}
