package targzutils

import (
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
