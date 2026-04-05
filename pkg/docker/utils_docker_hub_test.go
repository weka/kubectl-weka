package docker

import (
	"testing"
)

func TestNormalizeDockerReference(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Unqualified names (Docker Hub library images)
		{
			name:     "simple image name",
			input:    "nginx",
			expected: "docker.io/library/nginx",
		},
		{
			name:     "simple image with tag",
			input:    "nginx:latest",
			expected: "docker.io/library/nginx",
		},
		{
			name:     "simple image with version tag",
			input:    "nginx:1.27.3",
			expected: "docker.io/library/nginx",
		},

		// Library prefix (already namespaced)
		{
			name:     "library prefix without tag",
			input:    "library/nginx",
			expected: "docker.io/library/nginx",
		},
		{
			name:     "library prefix with tag",
			input:    "library/nginx:latest",
			expected: "docker.io/library/nginx",
		},

		// Namespaced Docker Hub images
		{
			name:     "namespaced image without tag",
			input:    "myuser/myimage",
			expected: "docker.io/myuser/myimage",
		},
		{
			name:     "namespaced image with tag",
			input:    "myuser/myimage:v1.0",
			expected: "docker.io/myuser/myimage",
		},

		// Already fully qualified (should not change)
		{
			name:     "full quay.io reference",
			input:    "quay.io/weka.io/weka-in-container:4.4.10.200",
			expected: "quay.io/weka.io/weka-in-container",
		},
		{
			name:     "registry with port",
			input:    "localhost:5000/myimage:latest",
			expected: "localhost:5000/myimage",
		},
		{
			name:     "registry.example.com reference",
			input:    "registry.example.com/namespace/image:tag",
			expected: "registry.example.com/namespace/image",
		},
		{
			name:     "private registry without namespace",
			input:    "myregistry.com/image:v1",
			expected: "myregistry.com/image",
		},

		// Edge cases
		{
			name:     "image with digest",
			input:    "nginx@sha256:abc123",
			expected: "docker.io/library/nginx",
		},
		{
			name:     "namespaced with digest",
			input:    "myuser/myimage@sha256:abc123",
			expected: "docker.io/myuser/myimage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeDockerReference(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeDockerReference(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsDockerHubReference(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Unqualified (Docker Hub)
		{
			name:     "simple image",
			input:    "nginx",
			expected: true,
		},
		{
			name:     "library prefix",
			input:    "library/nginx",
			expected: true,
		},
		{
			name:     "namespaced",
			input:    "myuser/myimage",
			expected: true,
		},

		// Qualified (not Docker Hub)
		{
			name:     "with registry",
			input:    "quay.io/weka/image",
			expected: false,
		},
		{
			name:     "localhost registry",
			input:    "localhost:5000/image",
			expected: false,
		},
		{
			name:     "registry with port",
			input:    "registry.example.com:5000/image",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDockerHubReference(tt.input)
			if result != tt.expected {
				t.Errorf("IsDockerHubReference(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasRegistry(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// No registry
		{
			name:     "simple image",
			input:    "nginx",
			expected: false,
		},
		{
			name:     "namespaced",
			input:    "namespace/image",
			expected: false,
		},

		// With registry
		{
			name:     "with registry",
			input:    "registry.example.com/image",
			expected: true,
		},
		{
			name:     "with port",
			input:    "localhost:5000/image",
			expected: true,
		},
		{
			name:     "with dot in name",
			input:    "quay.io/image",
			expected: true,
		},
		{
			name:     "with colon (port)",
			input:    "registry:5000/image",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasRegistry(tt.input)
			if result != tt.expected {
				t.Errorf("HasRegistry(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeImageUrlAndTagPreservesUnqualified(t *testing.T) {
	// This test verifies that NormalizeImageUrlAndTag preserves unqualified references
	// The Docker Hub normalization is available separately via NormalizeDockerReference
	// NOTE: NormalizeImageUrlAndTag returns (tag, url, error) not (url, tag, error)
	tests := []struct {
		name        string
		imageURL    string
		tag         string
		expectedURL string
		expectedTag string
	}{
		{
			name:        "unqualified image no tag",
			imageURL:    "nginx",
			tag:         "",
			expectedURL: "nginx",
			expectedTag: "latest",
		},
		{
			name:        "unqualified image with tag",
			imageURL:    "nginx:1.27.3",
			tag:         "",
			expectedURL: "nginx",
			expectedTag: "1.27.3",
		},
		{
			name:        "namespaced image",
			imageURL:    "myns/nginx",
			tag:         "",
			expectedURL: "myns/nginx",
			expectedTag: "latest",
		},
		{
			name:        "qualified image",
			imageURL:    "quay.io/weka/image",
			tag:         "",
			expectedURL: "quay.io/weka/image",
			expectedTag: "latest",
		},
		{
			name:        "library prefix",
			imageURL:    "library/nginx:1.27.3",
			tag:         "",
			expectedURL: "library/nginx",
			expectedTag: "1.27.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: NormalizeImageUrlAndTag returns (tag, url, error)
			returnedTag, returnedURL, err := NormalizeImageUrlAndTag(tt.imageURL, tt.tag)
			if err != nil {
				t.Fatalf("NormalizeImageUrlAndTag failed: %v", err)
			}
			if returnedURL != tt.expectedURL {
				t.Errorf("URL = %q, want %q", returnedURL, tt.expectedURL)
			}
			if returnedTag != tt.expectedTag {
				t.Errorf("Tag = %q, want %q", returnedTag, tt.expectedTag)
			}
		})
	}
}
