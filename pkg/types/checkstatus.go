package types

// CheckStatus represents the overall status of node checks
type CheckStatus string

const (
	StatusSkipped CheckStatus = "⏭️ SKIPPED (Node not ready)"
	StatusPass    CheckStatus = "✅ OK"
	StatusWarn    CheckStatus = "⚠️ WARNING"
	StatusFail    CheckStatus = "❌ FAILED"
)
