package targzutils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewTgzWriter(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}
	defer func() {
		_ = writer.Close()
	}()

	if writer == nil {
		t.Error("NewTgzWriter should return non-nil writer")
	}

	if writer.file == nil {
		t.Error("TgzWriter should have file reference")
	}

	if writer.gzw == nil {
		t.Error("TgzWriter should have gzip writer")
	}
}

func TestNewTgzWriterInvalidPath(t *testing.T) {
	// Try to write to a directory that doesn't exist with no create permission
	writer, err := NewTgzWriter("/invalid/path/that/does/not/exist/file.tar.gz")
	if err == nil {
		_ = writer.Close()
		t.Error("NewTgzWriter should return error for invalid path")
	}
}

func TestTgzWriterClose(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("Archive file not created: %v", err)
	}

	// Verify it's a valid gzip file
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("Failed to open archive: %v", err)
	}
	defer func() {
		_ = f.Close()
	}()

	_, err = gzip.NewReader(f)
	if err != nil {
		t.Errorf("Archive is not a valid gzip file: %v", err)
	}
}

func TestWriteFile(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	testContent := []byte("Hello, World!")
	err = writer.WriteFile("test.txt", testContent)
	if err != nil {
		t.Errorf("WriteFile failed: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify content was written (basic check)
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("Archive not created: %v", err)
	}
}

func TestWriteFileWithPath(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	// Write files with paths
	testContent1 := []byte("File 1")
	testContent2 := []byte("File 2")

	err = writer.WriteFile("dir/file1.txt", testContent1)
	if err != nil {
		t.Errorf("WriteFile failed: %v", err)
	}

	err = writer.WriteFile("dir/subdir/file2.txt", testContent2)
	if err != nil {
		t.Errorf("WriteFile failed: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify archive was created
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("Archive not created: %v", err)
	}
}

func TestExtractTarGz(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")
	extractDir := filepath.Join(tmpdir, "extracted")

	// Create archive with test files
	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	testContent := []byte("test content")
	err = writer.WriteFile("test.txt", testContent)
	if err != nil {
		t.Errorf("WriteFile failed: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Extract archive
	ctx := context.Background()
	err = Extract(ctx, archivePath, extractDir)
	if err != nil {
		t.Errorf("Extract failed: %v", err)
	}

	// Verify extracted file
	extractedFile := filepath.Join(extractDir, "test.txt")
	content, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Errorf("Failed to read extracted file: %v", err)
	}

	if !bytes.Equal(content, testContent) {
		t.Errorf("Extracted content mismatch: got %q, want %q", string(content), string(testContent))
	}
}

func TestExtractTarGzInvalidPath(t *testing.T) {
	ctx := context.Background()
	err := Extract(ctx, "/invalid/archive/path.tar.gz", "/tmp")
	if err == nil {
		t.Error("Extract should return error for invalid archive path")
	}
}

func TestTarGzRoundTrip(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")
	extractDir := filepath.Join(tmpdir, "extracted")

	// Create multiple files with different content
	files := map[string][]byte{
		"file1.txt":            []byte("Content of file 1"),
		"dir/file2.txt":        []byte("Content of file 2"),
		"dir/subdir/file3.txt": []byte("Content of file 3"),
	}

	// Write files to archive
	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	for name, content := range files {
		err = writer.WriteFile(name, content)
		if err != nil {
			t.Errorf("WriteFile failed for %s: %v", name, err)
		}
	}

	err = writer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Extract archive
	ctx := context.Background()
	err = Extract(ctx, archivePath, extractDir)
	if err != nil {
		t.Errorf("Extract failed: %v", err)
	}

	// Verify all files were extracted with correct content
	for name, expectedContent := range files {
		extractedPath := filepath.Join(extractDir, name)
		actualContent, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", name, err)
			continue
		}

		if !bytes.Equal(actualContent, expectedContent) {
			t.Errorf("File %s content mismatch: got %q, want %q", name, string(actualContent), string(expectedContent))
		}
	}
}

// Tests for TgzReader

func TestNewTgzReader(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	// Create a test archive
	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}
	err = writer.WriteFile("test.txt", []byte("test"))
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatalf("Writer.Close failed: %v", err)
	}

	// Test reading
	reader, err := NewTgzReader(archivePath)
	if err != nil {
		t.Fatalf("NewTgzReader failed: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	if reader == nil {
		t.Error("NewTgzReader should return non-nil reader")
	}

	if reader.file == nil {
		t.Error("TgzReader should have file reference")
	}

	if reader.gzr == nil {
		t.Error("TgzReader should have gzip reader")
	}
}

func TestNewTgzReaderInvalidPath(t *testing.T) {
	reader, err := NewTgzReader("/invalid/path/that/does/not/exist/file.tar.gz")
	if err == nil {
		_ = reader.Close()
		t.Error("NewTgzReader should return error for invalid path")
	}
}

func TestTgzReaderReadFile(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	// Create test archive with multiple files
	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	testContent := []byte("Hello, World!")
	err = writer.WriteFile("test.txt", testContent)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = writer.WriteFile("other.txt", []byte("Other content"))
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Writer.Close failed: %v", err)
	}

	// Test reading specific file
	reader, err := NewTgzReader(archivePath)
	if err != nil {
		t.Fatalf("NewTgzReader failed: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	data, err := reader.ReadFile("test.txt")
	if err != nil {
		t.Errorf("ReadFile failed: %v", err)
	}

	if !bytes.Equal(data, testContent) {
		t.Errorf("ReadFile content mismatch: got %q, want %q", string(data), string(testContent))
	}
}

func TestTgzReaderReadFileMissing(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	// Create test archive
	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	err = writer.WriteFile("test.txt", []byte("test"))
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Writer.Close failed: %v", err)
	}

	// Test reading non-existent file
	reader, err := NewTgzReader(archivePath)
	if err != nil {
		t.Fatalf("NewTgzReader failed: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	_, err = reader.ReadFile("nonexistent.txt")
	if err == nil {
		t.Error("ReadFile should return error for missing file")
	}
}

func TestTgzReaderListFiles(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	// Create test archive with multiple files
	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	expectedFiles := []string{"file1.txt", "dir/file2.txt", "dir/subdir/file3.txt"}
	for _, name := range expectedFiles {
		err = writer.WriteFile(name, []byte("content"))
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Writer.Close failed: %v", err)
	}

	// Test listing files
	reader, err := NewTgzReader(archivePath)
	if err != nil {
		t.Fatalf("NewTgzReader failed: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	files, err := reader.ListFiles()
	if err != nil {
		t.Errorf("ListFiles failed: %v", err)
	}

	if len(files) != len(expectedFiles) {
		t.Errorf("ListFiles returned %d files, expected %d", len(files), len(expectedFiles))
	}

	// Verify all expected files are present
	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}

	for _, expected := range expectedFiles {
		if !fileSet[expected] {
			t.Errorf("Expected file %q not found in listing", expected)
		}
	}
}

func TestTgzReaderIterate(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	// Create test archive
	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	testFiles := map[string][]byte{
		"file1.txt": []byte("Content 1"),
		"file2.txt": []byte("Content 2"),
		"file3.txt": []byte("Content 3"),
	}

	for name, content := range testFiles {
		err = writer.WriteFile(name, content)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Writer.Close failed: %v", err)
	}

	// Test iteration
	reader, err := NewTgzReader(archivePath)
	if err != nil {
		t.Fatalf("NewTgzReader failed: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	count := 0
	err = reader.Iterate(func(header *tar.Header, data []byte) error {
		count++
		expectedContent, exists := testFiles[header.Name]
		if !exists {
			t.Errorf("Unexpected file in archive: %s", header.Name)
		} else if !bytes.Equal(data, expectedContent) {
			t.Errorf("File %s content mismatch: got %q, want %q", header.Name, string(data), string(expectedContent))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Iterate failed: %v", err)
	}

	if count != len(testFiles) {
		t.Errorf("Iterate visited %d files, expected %d", count, len(testFiles))
	}
}

func TestTgzReaderGetFileSize(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	testContent := []byte("This is test content")

	// Create test archive
	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	err = writer.WriteFile("test.txt", testContent)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Writer.Close failed: %v", err)
	}

	// Test getting file size
	reader, err := NewTgzReader(archivePath)
	if err != nil {
		t.Fatalf("NewTgzReader failed: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	size, err := reader.GetFileSize("test.txt")
	if err != nil {
		t.Errorf("GetFileSize failed: %v", err)
	}

	expectedSize := int64(len(testContent))
	if size != expectedSize {
		t.Errorf("GetFileSize returned %d, expected %d", size, expectedSize)
	}
}

func TestTgzReaderReadFileToWriter(t *testing.T) {
	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "test.tar.gz")

	testContent := []byte("Test content for writer")

	// Create test archive
	writer, err := NewTgzWriter(archivePath)
	if err != nil {
		t.Fatalf("NewTgzWriter failed: %v", err)
	}

	err = writer.WriteFile("test.txt", testContent)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Writer.Close failed: %v", err)
	}

	// Test reading to writer
	reader, err := NewTgzReader(archivePath)
	if err != nil {
		t.Fatalf("NewTgzReader failed: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	buf := &bytes.Buffer{}
	n, err := reader.ReadFileToWriter("test.txt", buf)
	if err != nil {
		t.Errorf("ReadFileToWriter failed: %v", err)
	}

	if n != int64(len(testContent)) {
		t.Errorf("ReadFileToWriter returned %d bytes, expected %d", n, len(testContent))
	}

	if !bytes.Equal(buf.Bytes(), testContent) {
		t.Errorf("ReadFileToWriter content mismatch: got %q, want %q", buf.String(), string(testContent))
	}
}
