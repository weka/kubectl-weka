package cmd

// HostCheckModuleResult represents the result of a single module validation
type HostCheckModuleResult struct {
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
func (r *HostCheckModuleResult) FormatError(params map[string]interface{}) string {
	if r.ErrorTemplate == "" {
		return r.Error
	}
	return interpolateTemplate(r.ErrorTemplate, params)
}

// FormatWarning formats a warning message using the module's warning template
// Falls back to error template if warning template is not set
func (r *HostCheckModuleResult) FormatWarning(params map[string]interface{}) string {
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
func (r *HostCheckModuleResult) FormatSuccess(params map[string]interface{}) string {
	if r.SuccessTemplate == "" {
		return ""
	}
	return interpolateTemplate(r.SuccessTemplate, params)
}

// FormatSuggestedFix formats the suggested resolution using the template
func (r *HostCheckModuleResult) FormatSuggestedFix(params map[string]interface{}) string {
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
func (r *HostCheckModuleResult) Summary(params map[string]interface{}) string {
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
