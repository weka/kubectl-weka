package utils

import (
	"path/filepath"
	"testing"
)

func TestTryParseInt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNum int
		wantOk  bool
	}{
		{
			name:    "valid positive",
			input:   "42",
			wantNum: 42,
			wantOk:  true,
		},
		{
			name:    "valid negative",
			input:   "-42",
			wantNum: -42,
			wantOk:  true,
		},
		{
			name:    "zero",
			input:   "0",
			wantNum: 0,
			wantOk:  true,
		},
		{
			name:    "invalid non-numeric",
			input:   "abc",
			wantNum: 0,
			wantOk:  false,
		},
		{
			name:    "invalid empty",
			input:   "",
			wantNum: 0,
			wantOk:  false,
		},
		{
			name:    "invalid with spaces",
			input:   "42 abc",
			wantNum: 0,
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, ok := TryParseInt(tt.input)
			if num != tt.wantNum || ok != tt.wantOk {
				t.Errorf("TryParseInt(%q) = (%d, %v), want (%d, %v)",
					tt.input, num, ok, tt.wantNum, tt.wantOk)
			}
		})
	}
}

func TestMaxInt(t *testing.T) {
	tests := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{
			name:     "first larger",
			a:        10,
			b:        5,
			expected: 10,
		},
		{
			name:     "second larger",
			a:        5,
			b:        10,
			expected: 10,
		},
		{
			name:     "equal",
			a:        5,
			b:        5,
			expected: 5,
		},
		{
			name:     "negative",
			a:        -10,
			b:        -5,
			expected: -5,
		},
		{
			name:     "zero",
			a:        0,
			b:        10,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaxInt(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("MaxInt(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal text",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "with slashes",
			input:    "hello/world",
			expected: "hello_world",
		},
		{
			name:     "with colons",
			input:    "tag:latest",
			expected: "tag_latest",
		},
		{
			name:     "with @",
			input:    "image@sha256",
			expected: "image_sha256",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "image",
		},
		{
			name:     "spaces only",
			input:    "   ",
			expected: "image",
		},
		{
			name:     "complex",
			input:    "quay.io/weka/image:v1.0@sha256:abc",
			expected: "quay.io_weka_image_v1.0_sha256_abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFilePathJoinSafe(t *testing.T) {
	tests := []struct {
		name      string
		baseDir   string
		name_     string
		wantError bool
		wantPath  string
	}{
		{
			name:      "normal path",
			baseDir:   "/base",
			name_:     "subdir/file.txt",
			wantError: false,
			wantPath:  "/base/subdir/file.txt",
		},
		{
			name:      "empty name returns base",
			baseDir:   "/base",
			name_:     "",
			wantError: false,
			wantPath:  "/base",
		},
		{
			name:      "dot returns base",
			baseDir:   "/base",
			name_:     ".",
			wantError: false,
			wantPath:  "/base",
		},
		{
			name:      "path traversal attack",
			baseDir:   "/base",
			name_:     "../../../etc/passwd",
			wantError: true,
		},
		{
			name:      "absolute path escaping",
			baseDir:   "/base",
			name_:     "/etc/passwd",
			wantError: false, // Absolute path gets normalized, not rejected
			wantPath:  "/base/etc/passwd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FilePathJoinSafe(tt.baseDir, tt.name_)
			if (err != nil) != tt.wantError {
				t.Errorf("FilePathJoinSafe(%q, %q) error = %v, wantError %v", tt.baseDir, tt.name_, err, tt.wantError)
				return
			}
			if !tt.wantError && result != tt.wantPath {
				t.Errorf("FilePathJoinSafe(%q, %q) = %q, want %q", tt.baseDir, tt.name_, result, tt.wantPath)
			}
		})
	}
}

func TestWriteAndReadFile(t *testing.T) {
	tmpdir := t.TempDir()

	t.Run("write and read file", func(t *testing.T) {
		filename := filepath.Join(tmpdir, "test.txt")
		content := "Hello, World!"

		if err := WriteFile(filename, content); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		read, err := ReadFile(filename)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if read != content {
			t.Errorf("ReadFile returned %q, want %q", read, content)
		}
	})

	t.Run("read non-existent file", func(t *testing.T) {
		filename := filepath.Join(tmpdir, "nonexistent.txt")
		_, err := ReadFile(filename)
		if err == nil {
			t.Error("ReadFile should return error for non-existent file")
		}
	})
}

func TestFileExists(t *testing.T) {
	tmpdir := t.TempDir()

	t.Run("existing file", func(t *testing.T) {
		filename := filepath.Join(tmpdir, "test.txt")
		if err := WriteFile(filename, "test"); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		if !FileExists(filename) {
			t.Error("FileExists should return true for existing file")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		filename := filepath.Join(tmpdir, "nonexistent.txt")
		if FileExists(filename) {
			t.Error("FileExists should return false for non-existent file")
		}
	})

	t.Run("directory", func(t *testing.T) {
		// FileExists should return true for directories too (os.Stat succeeds)
		if !FileExists(tmpdir) {
			t.Error("FileExists should return true for directory")
		}
	})
}

func TestCreateAndVerifySHA256Signature(t *testing.T) {
	tmpdir := t.TempDir()

	t.Run("create and verify valid signature", func(t *testing.T) {
		filename := filepath.Join(tmpdir, "test.txt")
		if err := WriteFile(filename, "test content"); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		if err := CreateSHA256Signature(filename); err != nil {
			t.Fatalf("CreateSHA256Signature failed: %v", err)
		}

		sigFile := filename + ".sha256"
		if !FileExists(sigFile) {
			t.Error("Signature file should exist after CreateSHA256Signature")
		}

		valid, err := VerifySHA256Signature(filename)
		if err != nil {
			t.Fatalf("VerifySHA256Signature failed: %v", err)
		}

		if !valid {
			t.Error("VerifySHA256Signature should return true for valid signature")
		}
	})

	t.Run("verify when file modified", func(t *testing.T) {
		filename := filepath.Join(tmpdir, "test2.txt")
		if err := WriteFile(filename, "original content"); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		if err := CreateSHA256Signature(filename); err != nil {
			t.Fatalf("CreateSHA256Signature failed: %v", err)
		}

		// Modify the file
		if err := WriteFile(filename, "modified content"); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		valid, err := VerifySHA256Signature(filename)
		if err == nil {
			t.Error("VerifySHA256Signature should return error for modified file")
		}
		if valid {
			t.Error("VerifySHA256Signature should return false for modified file")
		}
	})

	t.Run("verify when signature missing", func(t *testing.T) {
		filename := filepath.Join(tmpdir, "test3.txt")
		if err := WriteFile(filename, "test content"); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		valid, err := VerifySHA256Signature(filename)
		if err != nil {
			t.Fatalf("VerifySHA256Signature should not error when signature missing")
		}

		if valid {
			t.Error("VerifySHA256Signature should return false when signature missing")
		}
	})
}

func TestFileSizeAndSHA256(t *testing.T) {
	tmpdir := t.TempDir()

	t.Run("calculate size and hash", func(t *testing.T) {
		filename := filepath.Join(tmpdir, "test.txt")
		content := "Hello, World!"

		if err := WriteFile(filename, content); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		size, hash, err := FileSizeAndSHA256(filename)
		if err != nil {
			t.Fatalf("FileSizeAndSHA256 failed: %v", err)
		}

		if size != int64(len(content)) {
			t.Errorf("FileSizeAndSHA256 returned size %d, want %d", size, len(content))
		}

		if hash == "" {
			t.Error("FileSizeAndSHA256 returned empty hash")
		}

		// Hash should be 64 hex characters (SHA256 is 256 bits = 32 bytes = 64 hex chars)
		if len(hash) != 64 {
			t.Errorf("FileSizeAndSHA256 returned hash of length %d, want 64", len(hash))
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, _, err := FileSizeAndSHA256(filepath.Join(tmpdir, "nonexistent.txt"))
		if err == nil {
			t.Error("FileSizeAndSHA256 should return error for non-existent file")
		}
	})
}
