package logs

import (
	"time"
)

// AggregatedLogOptions contains options for aggregating logs from multiple sources
type AggregatedLogOptions struct {
	Follow             bool
	Tail               int64
	Since              time.Duration
	Previous           bool
	TailFlagSet        bool // Indicates if --tail was explicitly set
	LimitConcurrent    int
	AddContainerPrefix bool
	NodeSelector       string
}

// WekaLogsOptions contains options for fetching logs from multiple WEKA cluster pods
type WekaLogsOptions struct {
	OwnerName     string
	OwnerKind     string
	Namespace     string
	AllNamespaces bool
	Role          string
	ContainerName string
	ContainerID   int
	Aggregation   AggregatedLogOptions
}

// LogLine represents a log line with timestamp for sorting
type LogLine struct {
	Timestamp time.Time
	PodName   string
	RawLine   string
	TimeStr   string
}
