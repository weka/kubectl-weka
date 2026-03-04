package cmd

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// PreflightOutputLine represents a single line of preflight output
type PreflightOutputLine struct {
	Text      string
	IsError   bool
	IsWarning bool
	IsSuccess bool
	IsHeader  bool
}

// PreflightOutput manages streaming output from preflight checks
type PreflightOutput struct {
	Lines  chan PreflightOutputLine
	Done   chan struct{}
	mu     sync.Mutex
	buffer strings.Builder
	writer io.Writer // Optional writer for real-time output
}

// NewPreflightOutput creates a new preflight output manager
func NewPreflightOutput(writer io.Writer) *PreflightOutput {
	return &PreflightOutput{
		Lines:  make(chan PreflightOutputLine, 100),
		Done:   make(chan struct{}),
		writer: writer,
	}
}

// Println writes a line to the output
func (po *PreflightOutput) Println(text string) {
	po.Write(text+"\n", false, false, false, false)
}

// Printf writes formatted text to the output
func (po *PreflightOutput) Printf(format string, args ...interface{}) {
	po.Println(fmt.Sprintf(format, args...))
}

// Success writes a success line
func (po *PreflightOutput) Success(text string) {
	po.Write(text+"\n", false, false, true, false)
}

// Warning writes a warning line
func (po *PreflightOutput) Warning(text string) {
	po.Write(text+"\n", false, true, false, false)
}

// Error writes an error line
func (po *PreflightOutput) Error(text string) {
	po.Write(text+"\n", true, false, false, false)
}

// Header writes a header line
func (po *PreflightOutput) Header(text string) {
	po.Write(text+"\n", false, false, false, true)
}

// Write sends output to both the channel and the writer (if set)
func (po *PreflightOutput) Write(text string, isError, isWarning, isSuccess, isHeader bool) {
	line := PreflightOutputLine{
		Text:      text,
		IsError:   isError,
		IsWarning: isWarning,
		IsSuccess: isSuccess,
		IsHeader:  isHeader,
	}

	// Send to channel (non-blocking)
	select {
	case po.Lines <- line:
	default:
		// Channel full, skip (shouldn't happen with buffer size 100)
	}

	// Also write to real-time output if configured
	if po.writer != nil {
		_, _ = po.writer.Write([]byte(text))
	}

	// Add to internal buffer for final string generation
	po.mu.Lock()
	po.buffer.WriteString(text)
	po.mu.Unlock()
}

// Close closes the output channels
func (po *PreflightOutput) Close() {
	close(po.Lines)
	close(po.Done)
}

// GetFullOutput returns the complete output as a string
func (po *PreflightOutput) GetFullOutput() string {
	po.mu.Lock()
	defer po.mu.Unlock()
	return po.buffer.String()
}

// PreflightNodesResult represents the result of node preflight checks
type PreflightNodesResult struct {
	Output       string
	Success      bool
	PassedCount  int
	WarningCount int
	FailedCount  int
	SkippedCount int
	Error        error
}

// PreflightK8sResult represents the result of K8s preflight checks
type PreflightK8sResult struct {
	Output  string
	Success bool
	Error   error
}
