package supportbundle

import (
	"context"
)

// ============================================================================
// Collector Status and Result Types
// ============================================================================

// CollectorStatus represents the status of a collector execution
type CollectorStatus string

const (
	StatusSuccess CollectorStatus = "success" // All data collected successfully
	StatusPartial CollectorStatus = "partial" // Some data collected, some errors occurred (non-critical)
	StatusFailure CollectorStatus = "failure" // Critical error - unable to collect data
)

// CollectorResult represents the result of a collector execution
type CollectorResult struct {
	Status       CollectorStatus
	FilesCreated []string
	Error        error    // Critical error that prevented collection
	Warnings     []string // Non-fatal warnings encountered during collection
}

// ============================================================================
// Collector Interface
// ============================================================================

// Collector is the interface that all support bundle collectors must implement
type Collector interface {
	// Name returns a human-readable name for this collector
	Name() string

	// Start is called before collection begins - used for reporting what will be collected
	Start(ctx context.Context)

	// Collect performs the collection and returns the result
	Collect(ctx context.Context) CollectorResult

	// Finish is called after collection completes - used for reporting errors and summary
	Finish(ctx context.Context, result CollectorResult)
}
