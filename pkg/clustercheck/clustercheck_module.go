package clustercheck

import (
	"context"
	"strings"
	"text/template"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterCheckModule defines the interface for cluster validation modules
type ClusterCheckModule interface {
	// Name returns the unique identifier for this check module
	Name() string

	// FriendlyName returns the human-readable name for this check
	FriendlyName() string

	// Description returns a detailed description of what this check validates
	Description() string

	// SuccessTemplate returns the template for successful validation message
	SuccessTemplate() string

	// WarningTemplate returns the template for warning validation message
	WarningTemplate() string

	// ErrorTemplate returns the template for error validation message
	ErrorTemplate() string

	// SuggestedResolutionTemplate returns the template for suggested fix
	SuggestedResolutionTemplate() string

	// Validate performs the cluster validation check
	// Returns a map with Status, Detail, Issue, and other relevant fields
	Validate(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client) (interface{}, error)

	// ValidateWithParams performs validation with additional parameters
	ValidateWithParams(ctx context.Context, clientset *kubernetes.Clientset, crClient client.Client, params map[string]interface{}) (interface{}, error)
}

// ClusterCheckResult holds the result of a cluster check validation
type ClusterCheckResult struct {
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
func (r *ClusterCheckResult) Summary(params map[string]interface{}) string {
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
func (r *ClusterCheckResult) FormatSuccess(params map[string]interface{}) string {
	if r.SuccessTemplate == "" {
		return "✅ OK"
	}
	return interpolateClusterCheckTemplate(r.SuccessTemplate, params)
}

// FormatWarning formats the warning message using the template
func (r *ClusterCheckResult) FormatWarning(params map[string]interface{}) string {
	if r.WarningTemplate == "" {
		return "⚠️ WARNING"
	}
	return interpolateClusterCheckTemplate(r.WarningTemplate, params)
}

// FormatError formats the error message using the template
func (r *ClusterCheckResult) FormatError(params map[string]interface{}) string {
	if r.ErrorTemplate == "" {
		if r.Error != "" {
			return "❌ ERROR: " + r.Error
		}
		return "❌ ERROR"
	}
	return interpolateClusterCheckTemplate(r.ErrorTemplate, params)
}

// FormatSuggestedFix formats the suggested resolution using the template
func (r *ClusterCheckResult) FormatSuggestedFix(params map[string]interface{}) string {
	if r.SuggestedResolutionTemplate == "" {
		return ""
	}
	return interpolateClusterCheckTemplate(r.SuggestedResolutionTemplate, params)
}

// interpolateClusterCheckTemplate replaces {{.FieldName}} placeholders with values from params
func interpolateClusterCheckTemplate(tmplStr string, params map[string]interface{}) string {
	if tmplStr == "" {
		return ""
	}

	// Merge Data fields into params if Data is a map
	allParams := make(map[string]interface{})
	for k, v := range params {
		allParams[k] = v
	}

	tmpl, err := template.New("check").Parse(tmplStr)
	if err != nil {
		return tmplStr // Return original if parse fails
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, allParams); err != nil {
		return tmplStr // Return original if execution fails
	}

	return result.String()
}
