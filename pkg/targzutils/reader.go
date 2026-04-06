package targzutils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// TgzReader manages reading tar.gz archives with proper resource cleanup
// It embeds *tar.Reader so all tar.Reader methods are available
type TgzReader struct {
	*tar.Reader
	file *os.File
	gzr  *gzip.Reader
}

// NewTgzReader creates a new TgzReader for reading from a tar.gz file
// Returns a TgzReader instance
// Example usage:
//
//	reader, err := targzutils.NewTgzReader(filePath)
//	if err != nil {
//		return err
//	}
//	defer reader.Close()
//
//	// Use reader as a tar.Reader (embedded methods) or call ReadFile()
//	data, err := reader.ReadFile("manifest.json")
//	// Or iterate through files:
//	for {
//		header, err := reader.Next()
//		if err == io.EOF { break }
//		// handle file
//	}
func NewTgzReader(path string) (*TgzReader, error) {
	// Open the tar.gz file
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open tar.gz file %q: %w", path, err)
	}

	// Create gzip reader
	gzr, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("create gzip reader: %w", err)
	}

	// Create tar reader
	tarr := tar.NewReader(gzr)

	reader := &TgzReader{
		Reader: tarr,
		file:   f,
		gzr:    gzr,
	}
	return reader, nil
}

// Close closes the TgzReader and underlying file handles
func (tr *TgzReader) Close() error {
	// Close gzip reader
	if err := tr.gzr.Close(); err != nil {
		_ = tr.file.Close()
		return fmt.Errorf("close gzip reader: %w", err)
	}

	// Close file
	if err := tr.file.Close(); err != nil {
		return fmt.Errorf("close tar.gz file: %w", err)
	}

	return nil
}

// ReadFile reads a single file from the tar archive and returns its contents
// It iterates through the archive to find the first file with the matching name
// Note: This resets the archive position, so subsequent calls to Next() will start after this file
func (tr *TgzReader) ReadFile(targetName string) ([]byte, error) {
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("file %q not found in archive", targetName)
			}
			return nil, fmt.Errorf("read tar header: %w", err)
		}

		// Check if this is the file we're looking for
		if header.Name == targetName {
			// Read the file content
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read file %q from archive: %w", targetName, err)
			}
			return data, nil
		}
	}
}

// ListFiles returns a list of all file names in the tar archive
// After this call, the archive position is reset to the beginning via Close/Reopen
func (tr *TgzReader) ListFiles() ([]string, error) {
	var files []string

	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read tar header: %w", err)
		}

		// Only include regular files (not directories)
		if header.Typeflag == tar.TypeReg {
			files = append(files, header.Name)
		}
	}

	return files, nil
}

// ReadFileToWriter reads a file from the archive and writes it to the provided writer
// Returns the number of bytes written
func (tr *TgzReader) ReadFileToWriter(targetName string, writer io.Writer) (int64, error) {
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return 0, fmt.Errorf("file %q not found in archive", targetName)
			}
			return 0, fmt.Errorf("read tar header: %w", err)
		}

		if header.Name == targetName {
			// Copy file content to writer
			n, err := io.Copy(writer, tr)
			if err != nil {
				return n, fmt.Errorf("copy file %q from archive: %w", targetName, err)
			}
			return n, nil
		}
	}
}

// Iterate calls the callback function for each file in the archive
// The callback receives the file header and data
// If the callback returns an error, iteration stops
func (tr *TgzReader) Iterate(callback func(header *tar.Header, data []byte) error) error {
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read tar header: %w", err)
		}

		// Only process regular files
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Read file content
		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("read file %q from archive: %w", header.Name, err)
		}

		// Call the callback
		if err := callback(header, data); err != nil {
			return err
		}
	}

	return nil
}

// IterateWithFilter calls the callback function only for files matching the filter
// The filter function returns true to include the file
func (tr *TgzReader) IterateWithFilter(filter func(header *tar.Header) bool, callback func(header *tar.Header, data []byte) error) error {
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read tar header: %w", err)
		}

		// Only process regular files
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Check filter
		if !filter(header) {
			continue
		}

		// Read file content
		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("read file %q from archive: %w", header.Name, err)
		}

		// Call the callback
		if err := callback(header, data); err != nil {
			return err
		}
	}

	return nil
}

// GetFileSize returns the size of a file in the archive
// Returns -1 if file not found
func (tr *TgzReader) GetFileSize(targetName string) (int64, error) {
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return -1, fmt.Errorf("file %q not found in archive", targetName)
			}
			return -1, fmt.Errorf("read tar header: %w", err)
		}

		if header.Name == targetName {
			return header.Size, nil
		}
	}
}
