package supportbundle

import (
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestIsWekaCSI tests the isWekaCSI function
func TestIsWekaCSI(t *testing.T) {
	tests := []struct {
		name       string
		driverName string
		expected   bool
	}{
		{
			name:       "weka.io driver",
			driverName: "weka.io",
			expected:   true,
		},
		{
			name:       "weka-csi.weka.io driver",
			driverName: "weka-csi.weka.io",
			expected:   true,
		},
		{
			name:       "custom.weka.io driver",
			driverName: "custom.weka.io",
			expected:   true,
		},
		{
			name:       "non-weka driver",
			driverName: "aws-ebs.csi.amazonaws.com",
			expected:   false,
		},
		{
			name:       "empty string",
			driverName: "",
			expected:   false,
		},
		{
			name:       "partial match",
			driverName: "weka",
			expected:   false,
		},
		{
			name:       "case sensitivity",
			driverName: "Weka.io",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kubernetes.IsWekaCSI(tt.driverName)
			if result != tt.expected {
				t.Errorf("isWekaCSI(%q) = %v, expected %v", tt.driverName, result, tt.expected)
			}
		})
	}
}

// TestExtractSecretReferencesFromStorageClass tests secret extraction
func TestExtractSecretReferencesFromStorageClass(t *testing.T) {
	tests := []struct {
		name       string
		parameters map[string]string
		expected   int // number of expected secrets
	}{
		{
			name: "no secret parameters",
			parameters: map[string]string{
				"provisioner": "weka.io",
			},
			expected: 0,
		},
		{
			name: "single secret with namespace",
			parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":      "my-secret",
				"csi.storage.k8s.io/provisioner-secret-namespace": "default",
			},
			expected: 1,
		},
		{
			name: "multiple secrets",
			parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":       "prov-secret",
				"csi.storage.k8s.io/provisioner-secret-namespace":  "default",
				"csi.storage.k8s.io/node-publish-secret-name":      "node-secret",
				"csi.storage.k8s.io/node-publish-secret-namespace": "kube-system",
			},
			expected: 2,
		},
		{
			name: "secret without namespace is skipped",
			parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name": "my-secret",
				// no namespace specified
			},
			expected: 0,
		},
		{
			name:       "empty parameters",
			parameters: map[string]string{},
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a real storage class
			sc := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-sc",
				},
				Provisioner: "weka.io",
				Parameters:  tt.parameters,
			}

			result := kubernetes.ExtractSecretReferencesFromStorageClass(sc)
			if len(result) != tt.expected {
				t.Errorf("extractSecretReferencesFromStorageClass() returned %d secrets, expected %d", len(result), tt.expected)
			}
		})
	}
}

// TestValidateSecretContent tests secret validation
func TestValidateSecretContent(t *testing.T) {
	tests := []struct {
		name          string
		secretData    map[string][]byte
		expectedCount int // number of expected errors
	}{
		{
			name: "valid secret",
			secretData: map[string][]byte{
				"username":     []byte("admin"),
				"password":     []byte("secret123"),
				"organization": []byte("myorg"),
				"endpoints":    []byte("10.0.0.1:14000"),
				"scheme":       []byte("https"),
			},
			expectedCount: 0,
		},
		{
			name: "missing required parameter",
			secretData: map[string][]byte{
				"username":     []byte("admin"),
				"password":     []byte("secret123"),
				"organization": []byte("myorg"),
				// missing endpoints
				"scheme": []byte("https"),
			},
			expectedCount: 1,
		},
		{
			name: "invalid scheme",
			secretData: map[string][]byte{
				"username":     []byte("admin"),
				"password":     []byte("secret123"),
				"organization": []byte("myorg"),
				"endpoints":    []byte("10.0.0.1:14000"),
				"scheme":       []byte("ftp"), // invalid
			},
			expectedCount: 1,
		},
		{
			name: "whitespace in value",
			secretData: map[string][]byte{
				"username":     []byte(" admin"), // leading space
				"password":     []byte("secret123"),
				"organization": []byte("myorg"),
				"endpoints":    []byte("10.0.0.1:14000"),
				"scheme":       []byte("https"),
			},
			expectedCount: 1,
		},
		{
			name:          "empty secret",
			secretData:    map[string][]byte{},
			expectedCount: 5, // all 5 required fields missing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: tt.secretData,
			}

			result := kubernetes.ValidateSecretContent(secret)
			if len(result) != tt.expectedCount {
				t.Errorf("validateSecretContent() returned %d errors, expected %d", len(result), tt.expectedCount)
				for _, err := range result {
					t.Logf("  Error: %s", err)
				}
			}
		})
	}
}
