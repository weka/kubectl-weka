package targzutils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// TgzWriter manages writing tar.gz archives with proper resource cleanup
// It embeds *tar.Writer so all tar.Writer methods are available
type TgzWriter struct {
	*tar.Writer
	file *os.File
	gzw  *gzip.Writer
}

// NewTgzWriter creates a new TgzWriter for writing to a tar.gz file
// Returns a TgzWriter instance
// Example usage:
//
//	writer, err := targzutils.NewTgzWriter(outputPath)
//	if err != nil {
//		return err
//	}
//	defer writer.Close()
//
//	// Use writer as a tar.Writer (embedded methods) or call WriteFile()
//	writer.WriteFile("manifest.json", data)
//	// Or use tar.Writer methods directly:
//	writer.WriteHeader(header)
func NewTgzWriter(path string) (*TgzWriter, error) {
	// Create output file
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create tar.gz file %q: %w", path, err)
	}

	// Create gzip writer
	gzw := gzip.NewWriter(f)

	// Create tar writer
	tarw := tar.NewWriter(gzw)

	writer := &TgzWriter{
		Writer: tarw,
		file:   f,
		gzw:    gzw,
	}
	return writer, nil
}

func (tw *TgzWriter) Close() error {
	if err := tw.Writer.Close(); err != nil {
		_ = tw.gzw.Close()
		_ = tw.file.Close()
		return fmt.Errorf("close tar writer: %w", err)
	}

	// Close gzip writer
	if err := tw.gzw.Close(); err != nil {
		_ = tw.file.Close()
		return fmt.Errorf("close gzip writer: %w", err)
	}

	// Close file
	if err := tw.file.Close(); err != nil {
		return fmt.Errorf("close tar.gz file: %w", err)
	}

	return nil
}

// WriteFile writes a file to the tar archive
// name is the path within the archive (e.g., "manifest.json", or "templates/daemonset.yaml")
// data is the file contents
func (w *TgzWriter) WriteFile(name string, data []byte) error {
	hdr := &tar.Header{
		Name:     filepath.ToSlash(name),
		Mode:     0o644,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}

	if err := w.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header %q: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write tar body %q: %w", name, err)
	}
	return nil

}

// Flush flushes the tar and gzip writers to ensure all data is written to disk
func (w *TgzWriter) Flush() error {
	if err := w.Writer.Flush(); err == nil {
		if err := w.gzw.Flush(); err == nil {
			if w.file.Sync() != nil {
				return fmt.Errorf("sync tar.gz file: %w", err)
			}
			return err
		}
		return err
	}
	return nil
}

// AddFile reads a file from sourcePath and adds it to the tar archive at targetPath
func (w *TgzWriter) AddFile(sourcePath string, targetPath string) error {
	return w.AddFileWithProgress(sourcePath, targetPath, nil)
}

// AddFileWithProgress reads a file and adds it to tar archive with progress callback
// progressFn is called after each chunk is copied (useful for large files)
// Set progressFn to nil to skip progress reporting
func (w *TgzWriter) AddFileWithProgress(sourcePath string, targetPath string, progressFn func(copied int64)) error {
	f, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", sourcePath, err)
	}
	defer func() {
		_ = f.Close()
	}()

	// Get file info
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat source file %q: %w", sourcePath, err)
	}

	// Create tar header
	hdr := &tar.Header{
		Name:     filepath.ToSlash(targetPath),
		Mode:     0o644,
		Size:     info.Size(),
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}

	// Write header
	if err := w.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header for %q: %w", targetPath, err)
	}

	// Copy file content with progress updates
	if progressFn == nil {
		// No progress reporting, simple copy
		if _, err := io.Copy(w, f); err != nil {
			return fmt.Errorf("copy file content for %q: %w", targetPath, err)
		}
	} else {
		// Copy in chunks with progress reporting (10MB chunks)
		const chunkSize = 10 * 1024 * 1024 // 10MB
		buf := make([]byte, chunkSize)
		var totalCopied int64

		for {
			n, err := f.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					return fmt.Errorf("write tar content for %q: %w", targetPath, writeErr)
				}
				totalCopied += int64(n)
				progressFn(totalCopied)
			}
			if err != nil {
				if err != io.EOF {
					return fmt.Errorf("read source file %q: %w", sourcePath, err)
				}
				break
			}
		}
	}

	return nil
}
