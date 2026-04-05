package hostcheck

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"github.com/weka/kubectl-weka/pkg/version"

	corev1 "k8s.io/api/core/v1"
)

// CachedHostCheckResult wraps a hostcheck result with metadata for cache validation
type CachedHostCheckResult struct {
	// Result is the actual hostcheck result
	Result *HostChecksResult `json:"result"`

	// Timestamp is when this result was cached
	Timestamp time.Time `json:"timestamp"`

	// BootID is the node's boot ID at the time of caching
	// If boot ID changes, the cache is invalidated
	BootID string `json:"boot_id"`

	// PluginVersion is the version of kubectl-weka that created this cache entry
	// Used to invalidate cache on minor/major version changes
	PluginVersion string `json:"plugin_version"`
}

// HostCheckResultCache manages cached hostcheck results with boot ID validation
type HostCheckResultCache struct {
	mu      sync.RWMutex
	cache   map[string]*CachedHostCheckResult // nodeName -> CachedHostCheckResult
	bootIDs map[string]string                 // nodeName -> bootID
}

// NewHostCheckResultCache creates a new cache
func NewHostCheckResultCache() *HostCheckResultCache {
	return &HostCheckResultCache{
		cache:   make(map[string]*CachedHostCheckResult),
		bootIDs: make(map[string]string),
	}
}

// GetBootIDFromNode extracts the boot ID from a Kubernetes node object
// Returns empty string if not available
func GetBootIDFromNode(node *corev1.Node) string {
	if node == nil {
		return ""
	}
	return node.Status.NodeInfo.SystemUUID
}

// Get retrieves a cached result if it's still valid for the given node
// Returns nil if cache miss, boot ID has changed, or plugin version incompatibility detected
func (c *HostCheckResultCache) Get(nodeName string, node *corev1.Node) *HostChecksResult {
	if nodeName == "" {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.cache[nodeName]
	if !exists {
		return nil
	}

	// Check if boot ID matches (indicates node hasn't rebooted)
	currentBootID := GetBootIDFromNode(node)
	if currentBootID == "" {
		// If we can't get boot ID, use cached result (better than nothing)
		// But still check version compatibility
		if !isVersionCompatible(cached.PluginVersion) {
			return nil
		}
		return cached.Result
	}

	if cached.BootID != currentBootID {
		// Node has rebooted, cache is invalid
		return nil
	}

	// Check plugin version compatibility (minor/major version changes invalidate cache)
	if !isVersionCompatible(cached.PluginVersion) {
		return nil
	}

	// Cache is still valid
	return cached.Result
}

// Set stores a hostcheck result in the cache with the node's boot ID and plugin version
func (c *HostCheckResultCache) Set(nodeName string, node *corev1.Node, result *HostChecksResult) {
	if nodeName == "" || result == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	bootID := GetBootIDFromNode(node)
	c.cache[nodeName] = &CachedHostCheckResult{
		Result:        result,
		Timestamp:     time.Now(),
		BootID:        bootID,
		PluginVersion: version.Version,
	}
	c.bootIDs[nodeName] = bootID
}

// InvalidateNode removes a cached result for a specific node
func (c *HostCheckResultCache) InvalidateNode(nodeName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, nodeName)
	delete(c.bootIDs, nodeName)
}

// InvalidateAll clears all cached results
func (c *HostCheckResultCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CachedHostCheckResult)
	c.bootIDs = make(map[string]string)
}

// GetCacheSize returns the number of cached results
func (c *HostCheckResultCache) GetCacheSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}

// GetCacheStats returns statistics about the cache
func (c *HostCheckResultCache) GetCacheStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := map[string]interface{}{
		"size": len(c.cache),
		"nodes": func() []map[string]interface{} {
			var nodes []map[string]interface{}
			for nodeName, cached := range c.cache {
				nodes = append(nodes, map[string]interface{}{
					"name":      nodeName,
					"bootID":    cached.BootID,
					"timestamp": cached.Timestamp,
				})
			}
			return nodes
		}(),
	}

	return stats
}

// MarshalJSON serializes the cache state (for debugging/logging)
func (c *HostCheckResultCache) MarshalJSON() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return json.Marshal(c.cache)
}

// GetValidCacheEntries returns only cache entries that still have matching boot IDs
// Also filters out entries with incompatible plugin versions (minor/major changes)
// Accepts a map of node names to their current kernel status info
func (c *HostCheckResultCache) GetValidCacheEntries(nodes map[string]*corev1.Node) HostChecksMap {
	c.mu.RLock()
	defer c.mu.RUnlock()

	validEntries := make(HostChecksMap)

	for nodeName, cached := range c.cache {
		node, exists := nodes[nodeName]
		if !exists {
			// Node no longer exists, skip
			continue
		}

		// Check version compatibility first
		if !isVersionCompatible(cached.PluginVersion) {
			// Version incompatible, skip this cached entry
			continue
		}

		currentBootID := GetBootIDFromNode(node)
		if currentBootID == "" || cached.BootID == currentBootID {
			// Either we can't verify boot ID (use cached) or IDs match
			validEntries[nodeName] = cached.Result
		}
	}

	return validEntries
}

// SaveToFile persists the cache to a JSON file
func (c *HostCheckResultCache) SaveToFile(filePath string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Marshal cache to JSON
	data, err := json.MarshalIndent(c.cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache to JSON: %w", err)
	}

	// Encrypt data
	encryptedData, err := encryptData(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt cache data: %w", err)
	}

	// Write to file with restricted permissions (owner read/write only)
	if err := os.WriteFile(filePath, []byte(encryptedData), 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// LoadFromFile loads the cache from a JSON file
// Returns nil silently if file doesn't exist
func (c *HostCheckResultCache) LoadFromFile(filePath string) error {
	// Check if file exists
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - this is normal
			return nil
		}
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	// Decrypt data
	decryptedData, err := decryptData(string(data))
	if err != nil {
		return fmt.Errorf("failed to decrypt cache data: %w", err)
	}

	// Unmarshal JSON
	var cachedData map[string]*CachedHostCheckResult
	if err := json.Unmarshal(decryptedData, &cachedData); err != nil {
		return fmt.Errorf("failed to unmarshal cache JSON: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Load into cache
	c.cache = cachedData

	// Rebuild bootIDs map from cache
	c.bootIDs = make(map[string]string)
	for nodeName, cached := range cachedData {
		c.bootIDs[nodeName] = cached.BootID
	}

	return nil
}

// ============================================================================
// Encryption Helpers - AES-256-GCM encryption for cache file
// ============================================================================

// getEncryptionKey derives an encryption key from the current user and hostname
// This ensures different users/machines have different keys
func getEncryptionKey() ([]byte, error) {
	// Get current user for key derivation
	currentUser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("could not get current user: %w", err)
	}

	// Get hostname for additional key material
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Create key material from user + hostname + salt
	keyMaterial := fmt.Sprintf("weka-kubectl-hostcheck-cache-v1:%s:%s", currentUser.Username, hostname)

	// Derive 32-byte (256-bit) key using SHA256
	hash := sha256.Sum256([]byte(keyMaterial))
	return hash[:], nil
}

// encryptData encrypts data using AES-256-GCM
// Returns base64-encoded ciphertext with nonce prepended
func encryptData(plaintext []byte) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return "", fmt.Errorf("failed to derive encryption key: %w", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Return base64-encoded ciphertext (nonce + encrypted data)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptData decrypts data that was encrypted with encryptData
// Expects base64-encoded ciphertext with nonce prepended
func decryptData(encoded string) ([]byte, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and actual ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// ============================================================================
// Version Compatibility Helpers - Cache invalidation on version changes
// ============================================================================

// isVersionCompatible checks if a cached plugin version is compatible with the current version
// Returns false if the minor or major version has changed (indicating incompatibility)
// Returns true if: version is empty AND current is "dev", or same minor version
func isVersionCompatible(cachedPluginVersion string) bool {
	// Empty version (old cache format without version tracking)
	// Special handling: only treat as compatible if current version is also "dev"
	// This allows dev builds to use old caches, but invalidates on production release
	if cachedPluginVersion == "" {
		// If current version is "dev", treat old cache as compatible (dev consistency)
		if version.Version == "dev" {
			return true
		}
		// If upgrading from dev to production, invalidate old caches
		return false
	}

	// Parse both versions
	currentVersion := version.ParseSemver(version.Version)
	cachedVersion := version.ParseSemver(cachedPluginVersion)

	// Check if minor version (or major) has changed
	// If minor version has changed, cache is invalid and requires redownload
	return currentVersion.IsSameMinorVersion(cachedVersion)
}

// GetVersionChangeStatus returns a human-readable message about version compatibility
// Used for logging/debugging
func GetVersionChangeStatus(cachedPluginVersion string) string {
	if cachedPluginVersion == "" {
		return "cache version tracking not available"
	}

	currentVersion := version.ParseSemver(version.Version)
	cachedVersion := version.ParseSemver(cachedPluginVersion)

	if currentVersion.IsSameMinorVersion(cachedVersion) {
		return fmt.Sprintf("compatible (cached: %s, current: %s)",
			cachedPluginVersion, version.Version)
	}

	return fmt.Sprintf("incompatible version change (cached: %s, current: %s) - redownload required",
		cachedPluginVersion, version.Version)
}
