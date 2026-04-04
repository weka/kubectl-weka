package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	ConsoleLogLevelDefault = slog.LevelInfo
	ConsoleLogLevelVarName = "CONSOLE_LOGLEVEL"
)

var (
	ConsoleLogLevel = ConsoleLogLevelDefault
)

func init() {
	switch strings.ToLower(os.Getenv(ConsoleLogLevelVarName)) {
	case "debug":
		ConsoleLogLevel = slog.LevelDebug
	case "info":
		ConsoleLogLevel = slog.LevelInfo
	case "warn":
		ConsoleLogLevel = slog.LevelWarn
	case "error":
		ConsoleLogLevel = slog.LevelError
	}
}

type Logger struct {
	*slog.Logger
	logFile *os.File
}

func (l *Logger) Close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

func (l *Logger) Sync() {
	if l.logFile != nil {
		l.logFile.Sync()
	}
}

// NewLoggerWithFile creates a logger that outputs to both console (stderr) and a file
// The console handler logs at the configured level (info or debug based on supportBundleDebug)
// The file handler always logs at debug level to capture all information
// Format: [HH:MM:SS] LEVEL message
func NewLoggerWithFile(logFilePath string) *Logger {

	// Determine console log level based on debug flag
	consoleLevel := ConsoleLogLevel

	// Create handlers with custom formatter
	consoleHandler := &simpleHandler{
		writer: os.Stderr,
		level:  consoleLevel,
	}
	var fileHandler *simpleHandler
	var logFile *os.File
	if logFilePath != "" {
		l, err := os.Create(logFilePath)
		if err != nil {
			fmt.Printf("Failed to open log file %s: %s\n", logFilePath, err)
		} else {
			logFile = l
			fileHandler = &simpleHandler{
				writer: logFile,
				level:  slog.LevelDebug, // Always capture debug in file
			}
		}
	}

	// Create a multi-handler by wrapping both handlers
	mh := &multiHandler{
		console: consoleHandler,
		file:    fileHandler,
	}

	return &Logger{
		Logger:  slog.New(mh),
		logFile: logFile,
	}

}

type simpleHandler struct {
	writer io.Writer
	level  slog.Level
}

func (sh *simpleHandler) Enabled(_ context.Context, level slog.Level) bool {
	if sh != nil {
		return level >= sh.level
	}
	return false
}

func (sh *simpleHandler) Handle(ctx context.Context, r slog.Record) error {
	if !sh.Enabled(ctx, r.Level) {
		return nil
	}

	// ANSI color codes
	reset, dim := "\033[0m", "\033[90m"
	red, yellow, blue := "\033[31m", "\033[33m", "\033[36m"

	var levelColor, levelStr string
	switch r.Level {
	case slog.LevelDebug:
		levelColor, levelStr = dim, "DEBUG"
	case slog.LevelInfo:
		levelColor, levelStr = blue, "INFO"
	case slog.LevelWarn:
		levelColor, levelStr = yellow, "WARN"
	case slog.LevelError:
		levelColor, levelStr = red, "ERROR"
	default:
		levelColor, levelStr = reset, r.Level.String()
	}

	timestamp := r.Time.Format("15:04:05")
	line := fmt.Sprintf("%s[%s]%s %s%s%s %s", dim, timestamp, reset, levelColor, levelStr, reset, r.Message)

	if r.NumAttrs() > 0 {
		r.Attrs(func(a slog.Attr) bool {
			line += fmt.Sprintf(" %s%s%s=%v", blue, a.Key, reset, a.Value)
			return true
		})
	}

	line += "\n"
	_, err := io.WriteString(sh.writer, line)
	return err
}

func (sh *simpleHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return sh
}

func (sh *simpleHandler) WithGroup(_ string) slog.Handler {
	return sh
}

// multiHandler implements slog.Handler to write to multiple sinks
type multiHandler struct {
	console slog.Handler
	file    slog.Handler
}

func (mh *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return mh.console.Enabled(ctx, level) || mh.file.Enabled(ctx, level)
}

func (mh *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	if mh.console.Enabled(ctx, r.Level) {
		if err := mh.console.Handle(ctx, r); err != nil {
			return err
		}
	}
	if mh.file != nil && mh.file.Enabled(ctx, r.Level) {
		if err := mh.file.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (mh *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if mh.file != nil {
		return &multiHandler{
			console: mh.console.WithAttrs(attrs),
			file:    mh.file.WithAttrs(attrs),
		}
	}
	return &multiHandler{
		console: mh.console.WithAttrs(attrs),
	}
}

func (mh *multiHandler) WithGroup(name string) slog.Handler {
	if mh.file != nil {
		return &multiHandler{
			console: mh.console.WithGroup(name),
			file:    mh.file.WithGroup(name),
		}
	}
	return &multiHandler{
		console: mh.console.WithGroup(name),
	}
}
