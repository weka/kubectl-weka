package docker

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/weka/kubectl-weka/pkg/logging"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

func GetAuthenticator(registry, username, password string) (authn.Authenticator, error) {
	registry = strings.TrimSpace(registry)

	// 1. Explicit username/password
	if username != "" || password != "" {
		if username == "" || password == "" {
			return nil, errors.New("both username and password must be provided together")
		}
		return &authn.Basic{
			Username: username,
			Password: password,
		}, nil
	}

	// Extract just the registry identifier (without port) for environment variable naming
	// This ensures consistent env var naming whether port is specified or not
	// e.g., "192.168.1.100:5000" -> "192.168.1.100" -> "REG_192_168_1_100"
	registryIdentifier := detectRegistry(registry)

	// Sanitize registry name for environment variable (replace dots, colons, slashes with underscores)
	sanitized := strings.ToUpper(strings.NewReplacer(
		".", "_",
		":", "_",
		"/", "_",
		"-", "_",
	).Replace(registryIdentifier))

	userEnvVarName := fmt.Sprintf("REG_%s_USERNAME", sanitized)
	passEnvVarName := fmt.Sprintf("REG_%s_PASSWORD", sanitized)

	// 2. Environment variables
	envUser := os.Getenv(userEnvVarName)
	envPass := os.Getenv(passEnvVarName)

	if envUser != "" || envPass != "" {
		if envUser == "" || envPass == "" {
			return nil, fmt.Errorf("%s and %s must both be set", userEnvVarName, passEnvVarName)
		}
		return &authn.Basic{
			Username: envUser,
			Password: envPass,
		}, nil
	}

	// 3. Docker config / credential helpers.go
	if registry != "" {
		ref, err := name.NewRegistry(registryIdentifier)
		if err != nil {
			return nil, fmt.Errorf("invalid registry %q: %w", registry, err)
		}

		auth, err := authn.DefaultKeychain.Resolve(ref)
		if err == nil {
			// Could still be anonymous, but that's fine
			return auth, nil
		}
	}

	// 4. Fallback to anonymous
	return authn.Anonymous, nil
}

// detectRegistry extracts the registry from an image repository string
// Examples:
//   - "nginx" -> "docker.io" (default Docker Hub)
//   - "nginx:1.27.3" -> "docker.io"
//   - "quay.io/weka.io/weka-in-container" -> "quay.io"
//   - "quay.io/weka.io/weka-in-container@sha256:abc123" -> "quay.io"
//   - "gcr.io/myproject/myimage" -> "gcr.io"
//   - "registry.k8s.io/sig-storage/csi-attacher" -> "registry.k8s.io"
//   - "localhost/image@sha256:abc123" -> "localhost"
//   - "localhost:5000/image" -> "localhost"
//   - "registry.io:5000/image" -> "registry.io"
//   - "192.168.1.100:5000/image" -> "192.168.1.100"
//   - "2.3.4.5:5000/weka" -> "2.3.4.5"
func detectRegistry(repo string) string {
	// Remove digest if present (e.g., @sha256:...)
	if digestIdx := strings.Index(repo, "@"); digestIdx != -1 {
		repo = repo[:digestIdx]
	}

	// Split by slash to get the registry part (before any path)
	parts := strings.Split(repo, "/")
	registryPart := parts[0]

	if colonIdx := strings.LastIndex(registryPart, ":"); colonIdx != -1 {
		registryPart = registryPart[:colonIdx]
	}

	// Check if this is a registry (has a dot or is localhost)
	if strings.Contains(registryPart, ".") || registryPart == "localhost" {
		return registryPart
	}

	// Default to Docker Hub
	return "docker.io"
}

// getAuthenticatorForRegistry returns the appropriate authenticator for a given registry
func getAuthenticatorForRegistry(ctx context.Context, registry string, defaultAuth authn.Authenticator) authn.Authenticator {
	logger := logging.GetLogger(ctx)
	var ret = defaultAuth
	// Try to get Docker credentials from env first, then fall back to default
	// Try Docker Hub credentials from environment or config
	auth, err := GetAuthenticator(registry, "", "")
	if err == nil && auth != authn.Anonymous {
		logger.Debug("Using secure credentials", "registry", registry)
		ret = auth
	} else {
		// Fall back to default (anonymous for public images)
		logger.Debug("Using anonymous access for Docker Hub", "registry", registry)
		ret = authn.Anonymous
	}

	return ret
}

// handleAuthenticationError provides a helpful message when authentication fails
func handleAuthenticationError(ctx context.Context, url string, originalErr error) error {
	logger := logging.GetLogger(ctx)

	// Check if this is a 401 (Unauthorized) error
	var err *transport.Error
	ok := errors.As(originalErr, &err)
	if ok {
		switch err.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			registry := detectRegistry(url)
			// Sanitize registry name for environment variable
			sanitized := strings.ToUpper(strings.NewReplacer(
				".", "_",
				":", "_",
				"/", "_",
				"-", "_",
			).Replace(registry))
			usernameEnvVar := fmt.Sprintf("REG_%s_USERNAME", sanitized)
			passwordEnvVar := fmt.Sprintf("REG_%s_PASSWORD", sanitized)

			logger.Error("Authentication failed for url - credentials required",
				"url", url,
				"error", originalErr,
				"suggestion", fmt.Sprintf("Try setting %s and %s environment variables", usernameEnvVar, passwordEnvVar))
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			logger.Error("Failed to authenticate url", "url", url, "originalErr", originalErr)
		default:
			logger.Error("Failed to authenticate url", "url", url, "originalErr", originalErr)
		}
	}
	return originalErr
}
