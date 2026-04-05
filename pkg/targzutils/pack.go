package targzutils

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/weka/kubectl-weka/pkg/logging"
)

// PackDirectory creates a tar.gz archive from a source directory
func PackDirectory(ctx context.Context, sourceDir, targetPath string) error {
	logger := logging.GetLogger(ctx)
	logger.Debug("Creating archive", "path", targetPath)

	err := PackWithProgressBar(sourceDir, targetPath, nil)
	return err
}

// PackDirectoryWithProgress creates a tar.gz archive with progress callback
// progressFn is called with (filesAdded, bytesAdded) after each file chunk
func PackDirectoryWithProgress(ctx context.Context, sourceDir, targetPath string, progressFn func(filesAdded int, bytesAdded int64)) error {
	logger := logging.GetLogger(ctx)
	logger.Debug("Creating archive with progress", "path", targetPath)

	err := PackWithProgressBar(sourceDir, targetPath, progressFn)
	return err
}

// PackWithProgressBar creates a tar.gz archive from a directory with optional progress callback
// progressFn is called after each file chunk is added (useful for large archives)
// Set progressFn to nil to skip progress reporting
func PackWithProgressBar(srcDir, outFile string, progressFn func(filesAdded int, bytesAdded int64)) error {
	tw, err := NewTgzWriter(outFile)
	if err != nil {
		return fmt.Errorf("create tar.gz file %q: %w", outFile, err)
	}
	defer func() {
		_ = tw.Close()
	}()

	var filesAdded int
	var bytesAdded int64

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		rel = filepath.ToSlash(rel)

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel

		if info.IsDir() {
			if !strings.HasSuffix(hdr.Name, "/") {
				hdr.Name += "/"
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()

		// Copy with chunked reading for progress reporting
		if progressFn != nil {
			const chunkSize = 10 * 1024 * 1024 // 10MB chunks
			buf := make([]byte, chunkSize)
			for {
				n, err := in.Read(buf)
				if n > 0 {
					if _, writeErr := tw.Write(buf[:n]); writeErr != nil {
						return writeErr
					}
					bytesAdded += int64(n)
					filesAdded++
					progressFn(filesAdded, bytesAdded)
				}
				if err != nil {
					if err != io.EOF {
						return err
					}
					break
				}
			}
		} else {
			// Simple copy without progress
			if _, err := io.Copy(tw, in); err != nil {
				return err
			}
		}

		return nil
	})
}

// CalculateDirectorySize recursively calculates the total size of all files in a directory
// Returns (fileCount, totalBytes)
func CalculateDirectorySize(rootDir string) (int, int64, error) {
	var fileCount int
	var totalBytes int64

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-regular files
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}

		fileCount++
		totalBytes += info.Size()
		return nil
	})

	return fileCount, totalBytes, err
}

