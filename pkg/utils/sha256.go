package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CreateSHA256Signature calculates SHA256 hash of a file and stores it in a separate .sha256 file
// The signature file format follows the standard: "<hash>  <filename>"
// Example: "abc123...def  bundle.tar.gz" stored in "bundle.tar.gz.sha256"
func CreateSHA256Signature(filePath string) error {
	// Calculate SHA256 hash
	_, hash, err := FileSizeAndSHA256(filePath)
	if err != nil {
		return fmt.Errorf("calculate SHA256: %w", err)
	}

	// Get just the filename without path
	filename := filepath.Base(filePath)

	// Create signature file path
	signatureFile := filePath + ".sha256"

	// Write signature in standard format: "hash  filename"
	signatureContent := fmt.Sprintf("%s  %s\n", hash, filename)
	if err := os.WriteFile(signatureFile, []byte(signatureContent), 0o644); err != nil {
		return fmt.Errorf("write signature file %q: %w", signatureFile, err)
	}

	return nil
}

// VerifySHA256Signature verifies the SHA256 signature of a file
// Looks for a .sha256 file in the same directory as the bundle
// Returns (true, nil) if signature is valid, (false, nil) if signature file not found, (false, error) if mismatch
func VerifySHA256Signature(bundlePath string) (bool, error) {
	signatureFile := bundlePath + ".sha256"

	// Check if signature file exists
	sigContent, err := os.ReadFile(signatureFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // Signature file doesn't exist (non-fatal)
		}
		return false, fmt.Errorf("read signature file %q: %w", signatureFile, err)
	}

	// Parse signature file - format is "hash  filename"
	sigLine := strings.TrimSpace(string(sigContent))
	parts := strings.Fields(sigLine)
	if len(parts) < 1 {
		return false, fmt.Errorf("invalid signature file format: expected 'hash  filename'")
	}
	expectedHash := parts[0]

	// Calculate actual hash
	_, actualHash, err := FileSizeAndSHA256(bundlePath)
	if err != nil {
		return false, err
	}

	// Compare hashes
	if expectedHash != actualHash {
		return false, fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return true, nil
}

// FileSizeAndSHA256 calculates file size and SHA256 checksum
func FileSizeAndSHA256(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", fmt.Errorf("open %q for checksum: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return 0, "", fmt.Errorf("hash %q: %w", path, err)
	}

	return n, hex.EncodeToString(h.Sum(nil)), nil
}
