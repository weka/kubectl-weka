package completion

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/user"

	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/weka/kubectl-weka/pkg/config"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Cache structure for storing completion data with a timestamp
type Cache struct {
	Timestamp int64    `yaml:"timestamp"`
	Values    *Objects `yaml:"values"`
}

// getCompletionCacheDir returns the cache directory for completions
func getCompletionCacheDir() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	dir := filepath.Join(usr.HomeDir, ".weka", "kubectl-weka", "completion_cache")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// SaveCompletionCache saves a list of Object to disk with a timestamp
func SaveCompletionCache(key string, values *Objects) {
	dir := getCompletionCacheDir()
	if dir == "" {
		return
	}
	data := Cache{
		Timestamp: time.Now().Unix(),
		Values:    values,
	}
	file := filepath.Join(dir, key+".json")
	dataJson, err := json.Marshal(data)
	if err != nil {
		return
	}
	if config.Get().Cache.Completion.Encrypt {
		encrypted, err := encryptDataCompletion(dataJson)
		if err != nil {
			return
		}
		_ = os.WriteFile(file, []byte(encrypted), 0o644)
	} else {
		_ = os.WriteFile(file, dataJson, 0o644)
	}
}

// LoadCompletionCache loads a list of Object from disk if valid (ttlSeconds)
func LoadCompletionCache(key string, ttlSeconds int64) (*Objects, bool) {
	dir := getCompletionCacheDir()
	if dir == "" {
		return nil, false
	}
	file := filepath.Join(dir, key+".json")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, false
	}
	var cache Cache
	var decrypted []byte
	if config.Get().Cache.Completion.Encrypt {
		// Try decrypt first, fallback to plain JSON if fails
		decrypted, err = decryptDataCompletion(string(data))
		if err != nil {
			// fallback: try as plain JSON
			decrypted = data
		}
	} else {
		decrypted = data
	}
	if err := json.Unmarshal(decrypted, &cache); err != nil {
		return nil, false
	}
	if time.Now().Unix()-cache.Timestamp > ttlSeconds {
		return nil, false
	}
	return cache.Values, true
}

// Encryption helpers for completion cache (AES-256-GCM, user+host derived key)

// getEncryptionKeyCompletion derives a 32-byte encryption key based on the current user and hostname, hashed with SHA-256
func getEncryptionKeyCompletion() ([]byte, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("could not get current user: %w", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	keyMaterial := fmt.Sprintf("weka-kubectl-completion-cache-v1:%s:%s", currentUser.Username, hostname)
	hash := sha256.Sum256([]byte(keyMaterial))
	return hash[:], nil
}

// encryptDataCompletion encrypts the data using AES-256-GCM and returns a base64-encoded string
func encryptDataCompletion(plaintext []byte) (string, error) {
	key, err := getEncryptionKeyCompletion()
	if err != nil {
		return "", fmt.Errorf("failed to derive encryption key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptDataCompletion decrypts the base64-encoded, AES-256-GCM encrypted data, returning the plaintext
func decryptDataCompletion(encoded string) ([]byte, error) {
	key, err := getEncryptionKeyCompletion()
	if err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return plaintext, nil
}

// InvalidateCompletionCache deletes the cache file for a given key
func InvalidateCompletionCache(key string) {
	dir := getCompletionCacheDir()
	if dir == "" {
		return
	}
	file := filepath.Join(dir, key+".json")
	_ = os.Remove(file)
}

// inferCacheKeyFromObject returns a string key for caching based on the type of the Kubernetes object
func inferCacheKeyFromObject(object client.Object) string {
	switch object.(type) {
	case *corev1.Node:
		return "node"
	case *corev1.Pod:
		return "pod"
	case *v1alpha1.WekaPolicy:
		return "weka_policy"
	case *v1alpha1.WekaClient:
		return "weka_client"
	case *v1alpha1.WekaContainer:
		return "weka_container"
	case *v1alpha1.WekaCluster:
		return "weka_cluster"
	case *v1.CSIDriver:
		return "csi_driver"
	case *corev1.Namespace:
		return "namespace"
	case *appsv1.Deployment:
		return "deployment"
	case *appsv1.DaemonSet:
		return "daemonset"
	case *appsv1.ReplicaSet:
		return "replicaset"
	case *appsv1.StatefulSet:
		return "statefulset"
	case *corev1.Service:
		return "service"
	case *corev1.ServiceAccount:
		return "serviceaccount"
	case *corev1.PersistentVolumeClaim:
		return "persistent_claim"
	case *corev1.PersistentVolume:
		return "persistent_volume"
	case *corev1.Secret:
		return "secret"

	default:
		panic(fmt.Sprintf("unknown type %T", object))
	}
}
