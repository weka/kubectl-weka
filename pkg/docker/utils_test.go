package docker

import (
	"testing"
)

func TestExtractTagFromImage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Tagged images
		{
			name:  "simple tagged image",
			input: "nginx:1.27.3",
			want:  "1.27.3",
		},
		{
			name:  "namespaced tagged image",
			input: "myns/nginx:1.27.3",
			want:  "1.27.3",
		},
		{
			name:  "registry with tagged image",
			input: "registry.example.com/image:v1.0",
			want:  "v1.0",
		},
		{
			name:  "localhost registry with port and tagged image",
			input: "localhost:5000/image:v1",
			want:  "v1",
		},
		{
			name:  "registry with namespace and tagged image",
			input: "quay.io/brancz/kube-rbac-proxy:v0.21.0",
			want:  "v0.21.0",
		},

		// Untagged images (should return "latest")
		{
			name:  "simple untagged image",
			input: "nginx",
			want:  "latest",
		},
		{
			name:  "namespaced untagged image",
			input: "myns/nginx",
			want:  "latest",
		},
		{
			name:  "registry untagged image",
			input: "registry.example.com/image",
			want:  "latest",
		},
		{
			name:  "localhost registry untagged image",
			input: "localhost:5000/image",
			want:  "latest",
		},
		{
			name:  "quay.io untagged image",
			input: "quay.io/brancz/kube-rbac-proxy",
			want:  "latest",
		},

		// Registry with port (colon before slash)
		{
			name:  "registry with port no tag",
			input: "localhost:5000/nginx",
			want:  "latest",
		},
		{
			name:  "registry with port and namespace no tag",
			input: "registry.scalar.lab:1234/myns/image",
			want:  "latest",
		},
		{
			name:  "registry with port and tag",
			input: "localhost:5000/nginx:latest",
			want:  "latest",
		},
		{
			name:  "registry with port, namespace and tag",
			input: "registry.scalar.lab:1234/myns/image:v1.2.3",
			want:  "v1.2.3",
		},

		// Edge cases
		{
			name:  "tag with dots",
			input: "image:v1.2.3.4-beta",
			want:  "v1.2.3.4-beta",
		},
		{
			name:  "docker hub official image",
			input: "library/nginx:1.27.3",
			want:  "1.27.3",
		},
		{
			name:  "docker hub namespace",
			input: "weka/csi-wekafs:v2.8.1",
			want:  "v2.8.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTagFromImage(tt.input)
			if got != tt.want {
				t.Errorf("ExtractTagFromImage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeImageUrlAndTag(t *testing.T) {
	tests := []struct {
		name      string
		imageURL  string
		tag       string
		wantTag   string
		wantURL   string
		wantError bool
	}{
		// Tag provided separately
		{
			name:     "tag provided separately",
			imageURL: "nginx",
			tag:      "1.27.3",
			wantTag:  "1.27.3",
			wantURL:  "nginx",
		},
		{
			name:     "namespaced image with separate tag",
			imageURL: "myns/nginx",
			tag:      "1.27.3",
			wantTag:  "1.27.3",
			wantURL:  "myns/nginx",
		},
		{
			name:     "registry image with separate tag",
			imageURL: "registry.example.com/image",
			tag:      "v1.0",
			wantTag:  "v1.0",
			wantURL:  "registry.example.com/image",
		},

		// Tag in URL, no separate tag provided
		{
			name:     "simple tagged image no separate tag",
			imageURL: "nginx:1.27.3",
			tag:      "",
			wantTag:  "1.27.3",
			wantURL:  "nginx",
		},
		{
			name:     "namespaced tagged image no separate tag",
			imageURL: "myns/nginx:1.27.3",
			tag:      "",
			wantTag:  "1.27.3",
			wantURL:  "myns/nginx",
		},
		{
			name:     "registry tagged image no separate tag",
			imageURL: "quay.io/brancz/kube-rbac-proxy:v0.21.0",
			tag:      "",
			wantTag:  "v0.21.0",
			wantURL:  "quay.io/brancz/kube-rbac-proxy",
		},

		// Untagged images (should default to "latest")
		{
			name:     "untagged simple image",
			imageURL: "nginx",
			tag:      "",
			wantTag:  "latest",
			wantURL:  "nginx",
		},
		{
			name:     "untagged namespaced image",
			imageURL: "myns/nginx",
			tag:      "",
			wantTag:  "latest",
			wantURL:  "myns/nginx",
		},
		{
			name:     "untagged registry image",
			imageURL: "registry.example.com/image",
			tag:      "",
			wantTag:  "latest",
			wantURL:  "registry.example.com/image",
		},

		// Registry with port
		{
			name:     "localhost registry with port tagged",
			imageURL: "localhost:5000/image:v1",
			tag:      "",
			wantTag:  "v1",
			wantURL:  "localhost:5000/image",
		},
		{
			name:     "localhost registry with port untagged",
			imageURL: "localhost:5000/image",
			tag:      "",
			wantTag:  "latest",
			wantURL:  "localhost:5000/image",
		},
		{
			name:     "custom registry with port and namespace",
			imageURL: "registry.scalar.lab:1234/myns/image:v2.0",
			tag:      "",
			wantTag:  "v2.0",
			wantURL:  "registry.scalar.lab:1234/myns/image",
		},

		// Separate tag overrides URL tag
		{
			name:     "separate tag overrides URL tag",
			imageURL: "nginx:1.27.3",
			tag:      "1.28.0",
			wantTag:  "1.28.0",
			wantURL:  "nginx",
		},
		{
			name:     "separate tag with registry URL tag",
			imageURL: "quay.io/brancz/kube-rbac-proxy:v0.21.0",
			tag:      "v0.22.0",
			wantTag:  "v0.22.0",
			wantURL:  "quay.io/brancz/kube-rbac-proxy",
		},

		// Edge cases
		{
			name:     "tag with dots and dashes",
			imageURL: "image:v1.2.3-beta.4",
			tag:      "",
			wantTag:  "v1.2.3-beta.4",
			wantURL:  "image",
		},
		{
			name:     "docker hub official image tagged",
			imageURL: "library/nginx:1.27.3",
			tag:      "",
			wantTag:  "1.27.3",
			wantURL:  "library/nginx",
		},
		{
			name:     "weka container image",
			imageURL: "quay.io/weka.io/csi-wekafs:v2.8.1",
			tag:      "",
			wantTag:  "v2.8.1",
			wantURL:  "quay.io/weka.io/csi-wekafs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTag, gotURL, err := NormalizeImageUrlAndTag(tt.imageURL, tt.tag)

			if (err != nil) != tt.wantError {
				t.Errorf("NormalizeImageUrlAndTag(%q, %q) error = %v, wantError %v", tt.imageURL, tt.tag, err, tt.wantError)
				return
			}

			if gotTag != tt.wantTag {
				t.Errorf("NormalizeImageUrlAndTag(%q, %q) tag = %q, want %q", tt.imageURL, tt.tag, gotTag, tt.wantTag)
			}

			if gotURL != tt.wantURL {
				t.Errorf("NormalizeImageUrlAndTag(%q, %q) url = %q, want %q", tt.imageURL, tt.tag, gotURL, tt.wantURL)
			}
		})
	}
}
