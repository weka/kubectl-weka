package docker

import (
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
)

func TestGetAuthenticator(t *testing.T) {
	tests := []struct {
		name         string
		registry     string
		username     string
		password     string
		envVars      map[string]string
		expectError  bool
		expectAnon   bool
		expectBasic  bool
		expectedUser string
		expectedPass string
	}{
		{
			name:         "explicit credentials provided",
			registry:     "quay.io",
			username:     "testuser",
			password:     "testpass",
			expectError:  false,
			expectBasic:  true,
			expectedUser: "testuser",
			expectedPass: "testpass",
		},
		{
			name:        "only username provided - error",
			registry:    "quay.io",
			username:    "testuser",
			password:    "",
			expectError: true,
		},
		{
			name:        "only password provided - error",
			registry:    "quay.io",
			username:    "",
			password:    "testpass",
			expectError: true,
		},
		{
			name:     "environment variable for quay.io",
			registry: "quay.io",
			envVars: map[string]string{
				"REG_QUAY_IO_USERNAME": "envuser",
				"REG_QUAY_IO_PASSWORD": "envpass",
			},
			expectError:  false,
			expectBasic:  true,
			expectedUser: "envuser",
			expectedPass: "envpass",
		},
		{
			name:     "environment variable for registry.example.com",
			registry: "registry.example.com",
			envVars: map[string]string{
				"REG_REGISTRY_EXAMPLE_COM_USERNAME": "user",
				"REG_REGISTRY_EXAMPLE_COM_PASSWORD": "pass",
			},
			expectError:  false,
			expectBasic:  true,
			expectedUser: "user",
			expectedPass: "pass",
		},
		{
			name:     "environment variable for IP address registry",
			registry: "192.168.1.100:5000",
			envVars: map[string]string{
				"REG_192_168_1_100_USERNAME": "ipuser",
				"REG_192_168_1_100_PASSWORD": "ippass",
			},
			expectError:  false,
			expectBasic:  true,
			expectedUser: "ipuser",
			expectedPass: "ippass",
		},
		{
			name:     "environment variable for registry with port",
			registry: "registry.local:5000",
			envVars: map[string]string{
				"REG_REGISTRY_LOCAL_USERNAME": "localuser",
				"REG_REGISTRY_LOCAL_PASSWORD": "localpass",
			},
			expectError:  false,
			expectBasic:  true,
			expectedUser: "localuser",
			expectedPass: "localpass",
		},
		{
			name:     "environment variable for docker.io",
			registry: "docker.io",
			envVars: map[string]string{
				"REG_DOCKER_IO_USERNAME": "dockeruser",
				"REG_DOCKER_IO_PASSWORD": "dockerpass",
			},
			expectError:  false,
			expectBasic:  true,
			expectedUser: "dockeruser",
			expectedPass: "dockerpass",
		},
		{
			name:     "environment variable for registry.k8s.io",
			registry: "registry.k8s.io",
			envVars: map[string]string{
				"REG_REGISTRY_K8S_IO_USERNAME": "k8suser",
				"REG_REGISTRY_K8S_IO_PASSWORD": "k8spass",
			},
			expectError:  false,
			expectBasic:  true,
			expectedUser: "k8suser",
			expectedPass: "k8spass",
		},
		{
			name:        "only username in env - error",
			registry:    "quay.io",
			expectError: true,
			envVars: map[string]string{
				"REG_QUAY_IO_USERNAME": "user",
			},
		},
		{
			name:        "only password in env - error",
			registry:    "quay.io",
			expectError: true,
			envVars: map[string]string{
				"REG_QUAY_IO_PASSWORD": "pass",
			},
		},
		{
			name:        "no credentials - returns authenticator",
			registry:    "quay.io",
			expectError: false,
			// When no explicit credentials are provided, the function returns
			// an authenticator from DefaultKeychain (could be anonymous or configured)
			// We just verify it doesn't error out
		},
		{
			name:     "explicit credentials override env vars",
			registry: "quay.io",
			username: "explicit",
			password: "explicit_pass",
			envVars: map[string]string{
				"REG_QUAY_IO_USERNAME": "envuser",
				"REG_QUAY_IO_PASSWORD": "envpass",
			},
			expectError:  false,
			expectBasic:  true,
			expectedUser: "explicit",
			expectedPass: "explicit_pass",
		},
		{
			name:     "registry path with project (only registry part used for auth)",
			registry: "gcr.io/my-project",
			envVars: map[string]string{
				"REG_GCR_IO_USERNAME": "gcruser",
				"REG_GCR_IO_PASSWORD": "gcrpass",
			},
			expectError:  false,
			expectBasic:  true,
			expectedUser: "gcruser",
			expectedPass: "gcrpass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			// Clean up any other REG_ variables
			for _, env := range os.Environ() {
				if len(env) > 4 && env[:4] == "REG_" {
					key := env[:len(env)-1]
					if idx := len(key) - len(key) + 1; idx > 0 {
						for i, char := range key {
							if char == '=' {
								varName := key[:i]
								if _, exists := tt.envVars[varName]; !exists {
									t.Setenv(varName, "")
								}
								break
							}
						}
					}
				}
			}

			auth, err := GetAuthenticator(tt.registry, tt.username, tt.password)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.expectAnon {
				if auth != authn.Anonymous {
					t.Errorf("expected anonymous authenticator, got %T", auth)
				}
				return
			}

			// If we reach here and expectBasic is false, we just verified no error
			// (used for cases where we just check that function succeeds)
			if !tt.expectBasic {
				return
			}

			// Verify Basic authenticator
			basic, ok := auth.(*authn.Basic)
			if !ok {
				t.Errorf("expected Basic authenticator, got %T", auth)
				return
			}

			if basic.Username != tt.expectedUser {
				t.Errorf("expected username %q, got %q", tt.expectedUser, basic.Username)
			}
			if basic.Password != tt.expectedPass {
				t.Errorf("expected password %q, got %q", tt.expectedPass, basic.Password)
			}
		})
	}
}

func TestDetectRegistry(t *testing.T) {
	tests := []struct {
		repo     string
		expected string
	}{
		{
			repo:     "nginx",
			expected: "docker.io",
		},
		{
			repo:     "nginx:1.27.3",
			expected: "docker.io",
		},
		{
			repo:     "library/nginx",
			expected: "docker.io",
		},
		{
			repo:     "quay.io/weka.io/weka-in-container",
			expected: "quay.io",
		},
		{
			repo:     "quay.io/weka.io/weka-in-container@sha256:abc123",
			expected: "quay.io",
		},
		{
			repo:     "gcr.io/myproject/myimage",
			expected: "gcr.io",
		},
		{
			repo:     "registry.k8s.io/sig-storage/csi-attacher",
			expected: "registry.k8s.io",
		},
		{
			repo:     "localhost/image",
			expected: "localhost",
		},
		{
			repo:     "localhost:5000/image",
			expected: "localhost",
		},
		{
			repo:     "registry.io:5000/image",
			expected: "registry.io",
		},
		{
			repo:     "192.168.1.100:5000/image",
			expected: "192.168.1.100",
		},
		{
			repo:     "2.3.4.5:5000/weka",
			expected: "2.3.4.5",
		},
		{
			repo:     "registry.local/namespace/image",
			expected: "registry.local",
		},
		{
			repo:     "registry.example.com:5000/image@sha256:digest",
			expected: "registry.example.com",
		},
		{
			repo:     "myimage",
			expected: "docker.io",
		},
		{
			repo:     "docker.io/library/nginx",
			expected: "docker.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.repo, func(t *testing.T) {
			result := detectRegistry(tt.repo)
			if result != tt.expected {
				t.Errorf("detectRegistry(%q) = %q, expected %q", tt.repo, result, tt.expected)
			}
		})
	}
}

func TestGetAuthenticatorEnvironmentVariableSanitization(t *testing.T) {
	tests := []struct {
		registry string
		envKey   string
	}{
		{
			registry: "quay.io",
			envKey:   "REG_QUAY_IO",
		},
		{
			registry: "registry.example.com",
			envKey:   "REG_REGISTRY_EXAMPLE_COM",
		},
		{
			registry: "192.168.1.100:5000",
			envKey:   "REG_192_168_1_100",
		},
		{
			registry: "registry-prod.example.com",
			envKey:   "REG_REGISTRY_PROD_EXAMPLE_COM",
		},
		{
			registry: "gcr.io",
			envKey:   "REG_GCR_IO",
		},
		{
			registry: "localhost:5000",
			envKey:   "REG_LOCALHOST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.registry, func(t *testing.T) {
			// Set credentials using expected env var names
			t.Setenv(tt.envKey+"_USERNAME", "testuser")
			t.Setenv(tt.envKey+"_PASSWORD", "testpass")

			auth, err := GetAuthenticator(tt.registry, "", "")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			basic, ok := auth.(*authn.Basic)
			if !ok {
				t.Errorf("expected Basic authenticator, got %T", auth)
				return
			}

			if basic.Username != "testuser" {
				t.Errorf("expected username 'testuser', got %q", basic.Username)
			}
			if basic.Password != "testpass" {
				t.Errorf("expected password 'testpass', got %q", basic.Password)
			}
		})
	}
}
