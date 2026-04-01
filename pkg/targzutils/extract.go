package targzutils

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/progress"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProgressReader wraps a reader and tracks bytes read
type ProgressReader struct {
	reader     io.Reader
	totalSize  int64
	bytesRead  int64
	category   string
	lastUpdate time.Time
	updateFreq time.Duration
	mu         sync.Mutex
}

// NewProgressReader creates a new progress-tracking reader
func NewProgressReader(r io.Reader, totalSize int64, category string) *ProgressReader {
	return &ProgressReader{
		reader:     r,
		totalSize:  totalSize,
		category:   category,
		updateFreq: 100 * time.Millisecond, // Update every 100ms
		lastUpdate: time.Now(),
	}
}

// Read implements io.Reader and tracks bytes
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	if n > 0 {
		pr.mu.Lock()
		pr.bytesRead += int64(n)
		// Update progress periodically
		if time.Since(pr.lastUpdate) >= pr.updateFreq {
			progress.RenderProgress(pr.bytesRead, pr.totalSize, pr.category, "Extracting...")
			pr.lastUpdate = time.Now()
		}
		pr.mu.Unlock()
	}
	return n, err
}

// BytesRead returns the total bytes read so far
func (pr *ProgressReader) BytesRead() int64 {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.bytesRead
}

// TrackingReader wraps a reader and calls a callback periodically
type TrackingReader struct {
	reader    io.Reader
	callback  func()
	bytesRead int64
	lastCall  time.Time
	freq      time.Duration
}

// NewTrackingReader creates a new tracking reader with callback
func NewTrackingReader(r io.Reader, callback func()) *TrackingReader {
	return &TrackingReader{
		reader:   r,
		callback: callback,
		lastCall: time.Now(),
		freq:     100 * time.Millisecond, // Call every 100ms
	}
}

// Read implements io.Reader
func (tr *TrackingReader) Read(p []byte) (n int, err error) {
	n, err = tr.reader.Read(p)
	if n > 0 {
		tr.bytesRead += int64(n)
		// Call callback periodically
		if time.Since(tr.lastCall) >= tr.freq {
			if tr.callback != nil {
				tr.callback()
			}
			tr.lastCall = time.Now()
		}
	}
	return n, err
}

// Extract extracts a tar.gz file to a destination directory with progress tracking
func Extract(ctx context.Context, tarGzPath string, destDir string) error {
	logger := logging.GetLogger(ctx)

	f, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("open tar.gz file %q: %w", tarGzPath, err)
	}
	defer func() {
		_ = f.Close()
	}()

	// Get total file size for final completion message
	fileInfo, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat tar.gz file: %w", err)
	}
	totalSize := fileInfo.Size()

	// Wrap file with progress tracking reader
	progReader := NewProgressReader(f, totalSize, "extract")

	gr, err := gzip.NewReader(progReader)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer func() {
		_ = gr.Close()
	}()

	tr := tar.NewReader(gr)
	lastFileName := ""

	for {
		header, err := tr.Next()
		if err == io.EOF {
			progress.RenderProgress(totalSize, totalSize, "extract", "Extraction complete")
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		targetPath := filepath.Join(destDir, header.Name)
		lastFileName = filepath.Base(header.Name)

		// Prevent directory traversal attacks
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)) {
			return fmt.Errorf("tar file contains path outside extraction directory: %q", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create directory %q: %w", targetPath, err)
			}

		case tar.TypeReg:
			// Create parent directories if needed
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create parent directory for %q: %w", targetPath, err)
			}

			// Create and write file
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("create file %q: %w", targetPath, err)
			}

			// Use trackingReader to update progress during file write
			trackReader := NewTrackingReader(tr, func() {
				progress.RenderProgress(progReader.BytesRead(), totalSize, "extract", "Extracting "+lastFileName)
			})

			if _, err := io.Copy(outFile, trackReader); err != nil {
				_ = outFile.Close()
				return fmt.Errorf("write file %q: %w", targetPath, err)
			}

			if err := outFile.Close(); err != nil {
				return fmt.Errorf("close file %q: %w", targetPath, err)
			}

			// Set file permissions
			if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
				logger.Warn("failed to set file permissions", "file", targetPath, "error", err)
			}

			// Flush progress immediately after extracting each file
			progress.RenderProgress(progReader.BytesRead(), totalSize, "extract", "Extracting "+lastFileName)

		default:
			logger.Warn("skipping unsupported tar entry type", "type", header.Typeflag, "name", header.Name)
		}
	}

	return nil
}
