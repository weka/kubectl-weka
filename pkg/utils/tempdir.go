package utils

import (
	"context"
	"github.com/weka/kubectl-weka/pkg/logging"
	"os"
	"sync"
)

// Global cleanup tracker for temporary directories
var (
	cleanupMutex          sync.Mutex
	cleanupPaths          []string
	CleanupTempDirsOnExit bool
)

const (
	SkipTemporaryDirCleanup = "SKIP_TEMPORARY_DIR_CLEANUP"
)

// MkdirTemp creates a temporary directory and registers it for cleanup
func MkdirTemp(dir, pattern string) (string, error) {
	dir, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		return "", err
	}

	cleanupMutex.Lock()
	cleanupPaths = append(cleanupPaths, dir)
	cleanupMutex.Unlock()

	return dir, nil
}

// CleanupTemporaryDirectories removes all registered temporary directories
func CleanupTemporaryDirectories(ctx context.Context) {
	logger := logging.GetLogger(ctx)
	cleanupMutex.Lock()
	paths := cleanupPaths
	cleanupPaths = nil
	cleanupMutex.Unlock()

	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			logger.Warn("Failed to cleanup temporary directory", "path", path, "err", err)
		}
	}
	logger.Debug("Cleaned up temporary directories")
}

func CleanUpOnExit(ctx context.Context) {
	CleanupTempDirsOnExit = true
	switch os.Getenv(SkipTemporaryDirCleanup) {
	case "1":
		CleanupTempDirsOnExit = false
	case "true":
		CleanupTempDirsOnExit = false
	}
	if CleanupTempDirsOnExit {
		CleanupTemporaryDirectories(ctx)
	}
}
