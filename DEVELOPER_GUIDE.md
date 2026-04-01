# kubectl-weka Developer Guide

This guide explains how to extend `kubectl-weka` with new functionality and provides comprehensive documentation of the system architecture and key packages.

## Table of Contents

- [Building](#building)
- [Architecture Overview](#architecture-overview)
  - [Core Packages](#core-packages)
  - [Command Groups](#command-groups)
- [Docker Package](#docker-package)
- [Logging Package](#logging-package)
- [Progress Package](#progress-package)
- [Targzutils Package](#targzutils-package)
- [Helm Package](#helm-package)
- [Air-Gapped Deployment](#air-gapped-deployment)
- [ResourcePrinter System](#resourceprinter-system)
- [Adding Preflight Checks](#adding-preflight-checks)
- [Adding Plan Validations](#adding-plan-validations)
- [Adding Support Bundle Collectors](#adding-support-bundle-collectors)
- [Adding New Commands](#adding-new-commands)
- [Testing Guidelines](#testing-guidelines)
- [Inline Function Documentation Standards](#inline-function-documentation-standards)

---

## Building

### Prerequisites

- Go 1.22+
- git (for version information extraction)
- make (for convenient building)

### Building with Makefile

The Makefile automates the build process and embeds version information via ldflags.

#### Available Targets

```bash
# Show available targets and current build information
make help

# Build binary in current directory
make build

# Install binary to GOPATH/bin
make install

# Remove built binary
make clean
```

#### How Version Information is Determined

The Makefile intelligently determines the version based on git state:

**Release Version (tag on HEAD):**
- Format: `v1.0.0` (exactly the tag, with v prefix)
- Used when building exactly at a git tag
- If working directory has uncommitted changes: `v1.0.0-abc123d-dirty`

**Development Version (commits after tag):**
- Format: `v1.0.0-5-abc123d` (tag-commit_count-commit_hash)
- Used when there are commits after the latest tag
- If working directory has uncommitted changes: `v1.0.0-5-abc123d-dirty`

---

## Architecture Overview

### Project Structure

```
kubectl-weka/
├── cmd/                          # Command implementations
│   ├── root.go                  # Root command setup
│   ├── version.go               # Version command
│   ├── airgapped*.go            # Air-gapped commands
│   ├── get*.go                  # Get commands
│   ├── logs*.go                 # Log commands
│   ├── plan*.go                 # Plan commands
│   ├── preflight*.go            # Preflight commands
│   └── supportbundle*.go        # Support bundle commands
│
├── pkg/                          # Reusable packages
│   ├── airgapped/               # Air-gapped deployment
│   │   ├── bundle.go            # Bundle structure and validation
│   │   ├── constants.go         # Constants and defaults
│   │   ├── download.go          # Download workflow
│   │   ├── help.go              # Help text
│   │   ├── types.go             # Type definitions
│   │   └── upload.go            # Upload workflow
│   │
│   ├── docker/                  # Docker image handling
│   │   ├── auth.go              # Authentication (.docker/config.json)
│   │   ├── auth_test.go         # Auth tests
│   │   ├── download.go          # Image downloading
│   │   ├── helpers.go           # Utility functions
│   │   ├── imagelib.go          # Image library operations
│   │   ├── progress.go          # Progress tracking
│   │   ├── types.go             # Type definitions
│   │   ├── upload.go            # Image uploading
│   │   ├── utils.go             # General utilities
│   │   ├── utils_test.go        # Utility tests
│   │
│   ├── helm/                    # Helm chart operations
│   │   ├── chart.go             # Chart loading and manipulation
│   │   ├── values.go            # Values manipulation
│   │   └── template.go          # Template handling
│   │
│   ├── logging/                 # Structured logging
│   │   └── logger.go            # Logger implementation
│   │
│   ├── progress/                # Progress tracking
│   │   └── render.go            # Progress rendering
│   │
│   ├── targzutils/              # Tar.gz operations
│   │   ├── extract.go           # Extraction with progress
│   │   ├── pack.go              # Archive creation
│   │   └── writer.go            # Custom writer implementations
│   │
│   ├── printer/                 # Output formatting
│   ├── kubernetes/              # K8s client management
│   ├── getters/                 # Resource getters
│   ├── utils/                   # General utilities
│   ├── types/                   # Shared types
│   ├── version/                 # Version handling
│   └── ... (other packages)
│
└── docs/                        # Documentation
    └── network-configuration.md
```

### Core Packages

| Package | Purpose | Key Functions |
|---------|---------|---------------|
| **docker** | Image registry operations | DownloadDockerImage, UploadDockerImage, UpdateTagForNewRegistry |
| **logging** | Structured logging | GetLogger, WithLogger |
| **progress** | Real-time progress display | RenderProgress, formatBytes |
| **targzutils** | Tar.gz compression/decompression | Extract, Pack, NewProgressReader |
| **helm** | Helm chart manipulation | LoadChart, CreateUpdatedChartArchive, GetNestedValue |
| **airgapped** | Air-gapped deployment | Download, Upload, extractAndValidateBundle |

---

## Docker Package

### Overview

The Docker package (`pkg/docker/`) handles Docker image download, upload, and registry operations for air-gapped deployments.

### Core Functions

#### DownloadDockerImage

Downloads a Docker image from a registry to a tar.gz file.

```go
// DownloadDockerImage downloads a Docker image from registry and saves it
func DownloadDockerImage(ctx context.Context, imageRef, outputFile, username, password string) error
```

**Parameters:**
- `ctx` - Context for cancellation and logging
- `imageRef` - Full image reference (e.g., `quay.io/weka.io/weka-in-container:5.3.0`)
- `outputFile` - Path to save the tar.gz archive
- `username`, `password` - Registry credentials (optional)

**Returns:** error if download fails

**Features:**
- Supports multi-architecture images
- Automatic authentication from .docker/config.json
- Progress tracking during download
- Automatic image pulling and export via container runtime

#### UploadDockerImage

Uploads a Docker image from tar.gz to a target registry.

```go
// UploadDockerImage uploads a Docker image to a registry
func UploadDockerImage(ctx context.Context, imageFile, targetRef, username, password string) error
```

**Parameters:**
- `ctx` - Context for cancellation and logging
- `imageFile` - Path to tar.gz image archive
- `targetRef` - Target image reference in new registry
- `username`, `password` - Registry credentials (optional)

**Returns:** error if upload fails

**Features:**
- Imports image from tar.gz
- Re-tags with new registry reference
- Pushes to target registry
- Progress tracking during upload

#### UpdateTagForNewRegistry

Rewrites an image reference to use a new registry.

```go
// UpdateTagForNewRegistry rewrites image reference for new registry
func UpdateTagForNewRegistry(imageRef, newRegistry string) string
```

**Parameters:**
- `imageRef` - Original image reference (e.g., `quay.io/weka.io/weka:1.0`)
- `newRegistry` - New registry URL (e.g., `registry.internal.com:5000`)

**Returns:** Updated image reference

**Example:**
```go
oldRef := "quay.io/weka.io/weka-in-container:5.3.0"
newRef := docker.UpdateTagForNewRegistry(oldRef, "registry.internal.com:5000")
// Result: "registry.internal.com:5000/weka-in-container:5.3.0"
```

### Authentication

The Docker package supports authentication via:

1. **.docker/config.json** - Standard Docker configuration file
2. **Username/Password** - Explicit credentials passed to functions
3. **Environment variables** - Standard Docker auth environment variables

**Example:**
```go
// Automatic auth from ~/.docker/config.json
err := docker.DownloadDockerImage(ctx, imageRef, outputFile, "", "")

// Explicit credentials
err := docker.DownloadDockerImage(ctx, imageRef, outputFile, "myuser", "mypass")
```

### Image Format

Downloaded images are stored as **Docker tar archives** (output of `docker save`):
- Format: `docker save image:tag > image.tar`
- Can be loaded with `docker load < image.tar`
- Supports multi-platform images

---

## Logging Package

### Overview

The Logging package (`pkg/logging/`) provides structured logging with context-based logger management, integrating seamlessly with `context.Context`.

### Core Functions

#### GetLogger

Retrieves a logger from context or creates a default logger.

```go
// GetLogger retrieves logger from context or returns default
func GetLogger(ctx context.Context) Logger
```

**Parameters:**
- `ctx` - Context that may contain a logger

**Returns:** Logger interface with methods: Info, Warn, Error, Debug

**Features:**
- Context-aware logger retrieval
- Graceful fallback to default logger
- Works with support bundle collection

**Example:**
```go
ctx := context.Background()
logger := logging.GetLogger(ctx)
logger.Info("Starting collection", "component", "operator")
logger.Warn("Potential issue detected", "code", 42)
logger.Error("Failed to collect", "error", err)
logger.Debug("Detailed information", "value", 123)
```

#### WithLogger

Sets a logger in context for passing to child functions.

```go
// WithLogger sets logger in context
func WithLogger(ctx context.Context, logger Logger) context.Context
```

**Parameters:**
- `ctx` - Base context
- `logger` - Logger to attach

**Returns:** New context with logger attached

**Example:**
```go
customLogger := NewLogger("debug")
ctx = logging.WithLogger(ctx, customLogger)
runCommand(ctx) // Will use custom logger
```

### Logger Interface

```go
type Logger interface {
    Info(msg string, keyvals ...interface{})
    Warn(msg string, keyvals ...interface{})
    Error(msg string, keyvals ...interface{})
    Debug(msg string, keyvals ...interface{})
}
```

All methods use key-value pairs for structured logging:
```go
logger.Info("Operation complete", "duration", "5s", "count", 42, "success", true)
// Output: [INFO] Operation complete duration=5s count=42 success=true
```

---

## Progress Package

### Overview

The Progress package (`pkg/progress/`) provides real-time progress bar rendering for long-running operations.

### Core Functions

#### RenderProgress

Displays a progress bar with percentage, bytes, and operation description.

```go
// RenderProgress prints a progress bar with percentage and sizes
func RenderProgress(current, total int64, category, operation string)
```

**Parameters:**
- `current` - Bytes currently processed
- `total` - Total bytes to process
- `category` - Category label (e.g., "download", "extract", "upload") - max 10 chars
- `operation` - Description of current operation (e.g., "Extracting filename.tar.gz")

**Features:**
- Percentage display with 2 decimal places
- Human-readable byte formatting
- Progress bar with visual feedback
- Automatic newline on completion (100%)
- Real-time updates via carriage return

**Example:**
```go
for i := 0; i <= 100; i++ {
    progress.RenderProgress(int64(i), 100, "download", "Downloading image.tar")
    time.Sleep(100 * time.Millisecond)
}
// Output:
// download [=========>                  ]  33.33% (33 B/100 B) Downloading image.tar
```

#### formatBytes

Converts bytes to human-readable format.

```go
// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string
```

**Returns:** Formatted string (e.g., "1.5 GB", "512 MB")

**Supported Units:** B, KB, MB, GB, TB

---

## Targzutils Package

### Overview

The Targzutils package (`pkg/targzutils/`) handles tar.gz compression and decompression with integrated progress tracking.

### Core Functions

#### Extract

Extracts a tar.gz file to a destination directory with progress tracking.

```go
// Extract extracts a tar.gz file with progress tracking
func Extract(ctx context.Context, tarGzPath, destDir string) error
```

**Parameters:**
- `ctx` - Context with logging
- `tarGzPath` - Path to .tar.gz file
- `destDir` - Destination directory

**Features:**
- Real-time progress updates every 100ms
- Accurate byte tracking of decompressed data
- Directory traversal attack prevention
- Proper permission preservation
- Security checks before extraction

**Example:**
```go
ctx := context.Background()
err := targzutils.Extract(ctx, "bundle.tar.gz", "/tmp/extracted")
// Output:
// extract [=========>                  ]  45.50% (500 MB/1100 MB) Extracting file.tar
```

#### Pack

Creates a tar.gz archive from a source directory.

```go
// Pack creates a tar.gz archive from directory
func Pack(ctx context.Context, srcDir, outputFile string) error
```

**Parameters:**
- `ctx` - Context with logging
- `srcDir` - Source directory to compress
- `outputFile` - Output tar.gz path

**Returns:** error if packing fails

### Progress Readers

#### NewProgressReader

Wraps a reader to track bytes during decompression.

```go
// NewProgressReader creates a reader that tracks bytes
func NewProgressReader(r io.Reader, totalSize int64, category string) *ProgressReader
```

**Usage:**
```go
file, _ := os.Open("bundle.tar.gz")
progReader := targzutils.NewProgressReader(file, fileSize, "extract")
gz, _ := gzip.NewReader(progReader)
// As data is read, progress updates every 100ms
```

#### NewTrackingReader

Provides fine-grained progress updates via callback.

```go
// NewTrackingReader creates reader with callback
func NewTrackingReader(r io.Reader, callback func()) *TrackingReader
```

**Usage:**
```go
callback := func() {
    fmt.Println("Still extracting...")
}
trackReader := targzutils.NewTrackingReader(tarReader, callback)
io.Copy(outFile, trackReader) // Callback fires every 100ms
```

---

## Helm Package

### Overview

The Helm package (`pkg/helm/`) provides utilities for loading Helm charts, updating values, and creating modified chart archives.

### Core Functions

#### LoadChart

Loads a Helm chart from various sources.

```go
// LoadChart loads a Helm chart from local, HTTP, or OCI source
func LoadChart(chartPath string) (*chart.Chart, error)
```

**Supports:**
- Local directory: `/path/to/chart`
- Local tar.gz: `/path/to/chart.tgz`
- HTTP URL: `https://charts.example.com/chart.tgz`
- OCI reference: `oci://registry.com/chart:version`

**Example:**
```go
// From local tar.gz
chart, err := helm.LoadChart("/tmp/operator.tgz")

// From OCI registry
chart, err := helm.LoadChart("oci://quay.io/weka.io/helm/weka-operator:1.10.0")
```

#### GetNestedValue

Retrieves values from nested maps using dot notation.

```go
// GetNestedValue gets value from nested map using dot notation
func GetNestedValue(values map[string]interface{}, path string) string
```

**Parameters:**
- `values` - Values map (typically from chart.Values)
- `path` - Dot-notation path (e.g., "csi.image" or "taskmon.defaultImage")

**Returns:** String value or empty string if not found

**Example:**
```go
values := map[string]interface{}{
    "csi": map[string]interface{}{
        "image": "quay.io/weka.io/weka-csi:1.0",
    },
}

image := helm.GetNestedValue(values, "csi.image")
// Result: "quay.io/weka.io/weka-csi:1.0"
```

#### SetNestedValue

Sets values in nested maps, creating intermediate maps as needed.

```go
// SetNestedValue sets value in nested map using dot notation
func SetNestedValue(values map[string]interface{}, path, value string)
```

**Features:**
- Creates intermediate maps if they don't exist
- Preserves existing structure
- Handles arbitrary nesting depth

**Example:**
```go
values := make(map[string]interface{})
helm.SetNestedValue(values, "csi.image", "registry.internal.com:5000/weka-csi:1.0")
// Result:
// {
//   "csi": {
//     "image": "registry.internal.com:5000/weka-csi:1.0"
//   }
// }
```

#### CreateUpdatedChartArchive

Creates a new Helm chart archive with updated values.

```go
// CreateUpdatedChartArchive creates new chart archive with updated values
func CreateUpdatedChartArchive(ctx context.Context, chart *chart.Chart, 
    updatedValues map[string]interface{}, outputPath string) error
```

**Features:**
- Preserves chart structure and metadata
- Updates values.yaml with new values
- Creates proper chart.tgz format
- Suitable for `helm install`

#### CreateOverrideValuesFile

Generates a values file for use with `helm install -f`.

```go
// CreateOverrideValuesFile creates values override file
func CreateOverrideValuesFile(values map[string]interface{}, outputPath string) error
```

**Output Format:** YAML file with only updated values (not full chart values)

**Usage:**
```bash
helm install weka-operator chart.tgz -f values-override.yaml
```

---

## Air-Gapped Deployment

### Overview

The Air-Gapped subsystem (`pkg/airgapped/`) provides a complete workflow for downloading WEKA components in internet-connected environments and deploying them in isolated (air-gapped) Kubernetes clusters.

### Workflow

```
Download Flow:
Internet → Download WEKA images + Helm charts → Validate → Create bundle with manifest + SHA256

Upload Flow:
Bundle → Extract + validate → Upload images to registry → Update Helm charts → Generate override values
```

### Core Types

#### BundleManifest

Represents the contents of a bundle.

```go
type BundleManifest struct {
    Version          string
    CreatedAt        string
    Components       map[string]*ComponentManifest  // WEKA images
    HelmCharts       map[string]*ChartData          // Helm chart info
    Architectures    []string                       // Supported architectures
}
```

#### ComponentManifest

Describes a component (e.g., "weka", "operator").

```go
type ComponentManifest struct {
    Name    string
    Version string
    Images  []*ImageArchive  // Per-architecture images
    Size    int64
}
```

### Core Functions

#### Download

Downloads all WEKA components for air-gapped deployment.

```go
// Download downloads WEKA images and Helm charts to a bundle
func Download(ctx context.Context, opts DownloadOptions) error
```

**Options:**
- `WekaVersion` - WEKA version to download
- `OperatorChartVersion` - Operator Helm chart version
- `CSIChartVersion` - CSI Helm chart version
- `Architectures` - List of architectures (amd64, arm64, etc.)
- `BundleFile` - Output bundle path
- `Username`, `Password` - Registry credentials

**Features:**
- Multi-architecture support
- Parallel downloads
- Progress tracking
- SHA256 signature generation
- Comprehensive manifest creation

#### Upload

Uploads components from bundle to air-gapped registry.

```go
// Upload uploads bundle contents to target registry
func Upload(ctx context.Context, opts UploadOptions) error
```

**Options:**
- `BundleFile` - Path to bundle.tar.gz
- `RegistryURL` - Target registry URL
- `Username`, `Password` - Registry credentials
- `Architecture` - Optional filter

**Features:**
- Bundle validation and extraction
- Image upload with progress
- Helm chart updates with new image URLs
- Override values file generation
- Directory structure creation

### Bundle Validation

Bundles are validated at multiple stages:

1. **SHA256 Signature** - Verifies bundle integrity
2. **Component Images** - Validates each image archive exists
3. **Helm Charts** - Verifies chart archives present
4. **File Permissions** - Ensures extracted files have correct permissions

---

## ResourcePrinter System

The ResourcePrinter system provides unified output formatting for all `kubectl weka get` commands.

### Overview

**Purpose**: Standardize resource output formatting across all commands

**Features**:
- Multiple output formats (table, wide, json, yaml, custom-columns)
- Column visibility control
- Custom value formatting
- Consistent kubectl-like behavior

### Key Types

```go
type ResourcePrinter interface {
    SetOptions(opts PrinterOptions)
    Print(columns []TableColumn, rows []TableRow, w io.Writer) error
}

type TableColumn struct {
    Name                 string
    VisibleInWide        bool  // Only shown with -o wide
    TableFormatFunctions []func(interface{}) string
}

type TableRow struct {
    Values map[string]interface{}
}
```

### Printer Implementations

- `TablePrinter` - Human-readable ASCII tables
- `JsonPrinter` - JSON output
- `YamlPrinter` - YAML output
- `CustomColumnsPrinter` - User-selected columns

---

## Adding Preflight Checks

Node and cluster preflight checks follow the registry pattern. See detailed examples in the original DEVELOPER_GUIDE section above.

### Step 1: Create a Module

Create a module file implementing the check interface with `Validate()` method.

### Step 2: Register

Register in module registry during initialization.

### Step 3: Test

Build and run: `./kubectl-weka preflight nodes`

---

## Adding Plan Validations

Plan validations analyze resource specifications before deployment using the same registry pattern as preflight checks.

### Step 1: Create Validation Module

Implement validation logic with resource-specific parameters.

### Step 2: Register Module

Register in `GlobalWekaConfigValidationRegistry`.

### Step 3: Test

Run: `./kubectl-weka plan cluster spec.yaml`

---

## Adding Support Bundle Collectors

Support bundle collectors gather diagnostic data organized in structured directories.

### Step 1: Create Collector

```go
type ExampleCollector struct {
    ResourceName string
}

func (c *ExampleCollector) Name() ModuleName {
    return "Example Data"
}

func (c *ExampleCollector) Collect(ctx context.Context) CollectorResult {
    // Collection logic
}
```

### Step 2: Register Collector

Add to `collectorsByMode()` function in `cmd/supportbundle_common.go`.

### Step 3: Test

Run: `./kubectl-weka support-bundle operator --debug`

---

## Adding New Commands

### Step 1: Define Command

Create `cmd/newcommand.go` with Cobra command definition.

### Step 2: Register Command

Add to root or parent command in `init()`.

### Step 3: Add Flags

Use standard flags: `-n/--namespace`, `-A/--all-namespaces`, `-o/--output`.

### Step 4: Test and Document

Test command, then update README.md with examples.

---

## Testing Guidelines

### Unit Tests

```go
func TestExample(t *testing.T) {
    // Test implementation
}
```

### Integration Tests

```go
func TestIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    // Test against real cluster
}
```

Run with: `go test ./...`

---

## Inline Function Documentation Standards

All exported functions should have documentation comments following Go conventions:

```go
// FunctionName describes what the function does in imperative form.
// Additional details about behavior, parameters, and return values.
//
// Parameters:
// - param1: description
// - param2: description
//
// Returns: description of return value or error
//
// Example:
//    result, err := FunctionName(...)
func FunctionName(param1 Type1, param2 Type2) (Result, error)
```

### Example

```go
// DownloadDockerImage downloads a Docker image from a registry and saves it as a tar archive.
// Supports authentication via .docker/config.json or explicit credentials.
// Progress updates are shown during download.
//
// Parameters:
// - ctx: Context for cancellation and logging
// - imageRef: Full image reference (e.g., "quay.io/weka.io/image:1.0")
// - outputFile: Path where tar archive will be saved
// - username, password: Optional credentials (uses .docker/config.json if empty)
//
// Returns: error if download fails (nil on success)
//
// Example:
//    err := docker.DownloadDockerImage(ctx, "quay.io/weka.io/weka:5.3.0", 
//        "/tmp/weka.tar", "", "")
func DownloadDockerImage(ctx context.Context, imageRef, outputFile, username, password string) error
```

---

## Release Checklist

Before creating a release:

1. ✅ All tests pass: `go test ./...`
2. ✅ Build succeeds: `go build -o kubectl-weka .`
3. ✅ Documentation updated for new features
4. ✅ CHANGELOG.md updated with all changes
5. ✅ Version bumped appropriately
6. ✅ Code formatted: `go fmt ./...`
7. ✅ No compilation warnings

---

## Additional Resources

- [Cobra Documentation](https://github.com/spf13/cobra)
- [Controller-Runtime Client](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client)
- [Kubernetes Client-Go](https://github.com/kubernetes/client-go)
- [Helm Go SDK](https://pkg.go.dev/helm.sh/helm/v3@v3.10.0/pkg/chart)

---

**Happy coding!** 🚀

---

## Building

### Prerequisites

- Go 1.25.0 or later
- git (for version information extraction)
- make (for convenient building)

### Building with Makefile

The Makefile automates the build process and embeds version information via ldflags.

#### Available Targets

```bash
# Show available targets and current build information
make help

# Build binary in current directory
make build

# Install binary to GOPATH/bin
make install

# Remove built binary
make clean
```

#### How Version Information is Determined

The Makefile intelligently determines the version based on git state:

**Release Version (tag on HEAD):**
- Format: `v1.0.0` (exactly the tag, with v prefix)
- Used when building exactly at a git tag
- If working directory has uncommitted changes: `v1.0.0-abc123d-dirty`

**Development Version (commits after tag):**
- Format: `v1.0.0-5-abc123d` (tag-commit_count-commit_hash)
- Used when there are commits after the latest tag
- If working directory has uncommitted changes: `v1.0.0-5-abc123d-dirty`

**Version Components:**
1. **Tag** – Extracted from git with v prefix preserved (e.g., `v1.0.0`)
2. **Commits Since Tag** – Only included if not on a tag (e.g., `-5`)
3. **Commit Hash** – Only included if not on a clean tag (e.g., `-abc123d`)
4. **Dirty Flag** – Added if working directory has uncommitted changes (e.g., `-dirty`)
5. **Commit** – Latest commit hash (short form)
6. **Date** – Current UTC timestamp in ISO 8601 format

#### Version Examples

| Scenario | Command | Version |
|----------|---------|---------|
| At release tag, clean | `git checkout v1.0.0` | `v1.0.0` |
| At release tag, dirty | `git checkout v1.0.0 && echo "change" > file.go` | `v1.0.0-abc123d-dirty` |
| 5 commits after tag, clean | After 5 commits on main | `v1.0.0-5-abc123d` |
| 5 commits after tag, dirty | 5 commits + uncommitted change | `v1.0.0-5-abc123d-dirty` |
| No tags (initial dev) | First repository | `v0.0.0-N-abc123d` |

#### Example Build Output

```bash
$ make build
Git Information:
  Latest Tag:     v1.0.0
  Tag on HEAD:    
  Working Dir:    clean
  Version:        v1.0.0-5-abc123d
  Commit:         abc123d
Building kubectl-weka v1.0.0-5-abc123d
  Commit: abc123d
  Date:   2026-03-11T15:30:00Z
```

#### After Tagging a Release

```bash
$ git tag v1.0.1
$ git push origin v1.0.1
$ make build
Git Information:
  Latest Tag:     v1.0.1
  Tag on HEAD:    v1.0.1
  Working Dir:    clean
  Version:        v1.0.1
  Commit:         abc123d
Building kubectl-weka v1.0.1
  Commit: abc123d
  Date:   2026-03-11T15:30:00Z
```

#### Verify Version Information

After building, verify the version information was correctly embedded:

```bash
./kubectl-weka version
# Output:
# kubectl-weka version v1.0.0-5-abc123d
# commit: abc123d
# date: 2026-03-11T15:30:00Z
```

### Manual Build

If you prefer not to use the Makefile:

```bash
# Determine version based on git state
TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

# Check if tag is on HEAD
if git describe --exact-match --tags >/dev/null 2>&1; then
  VERSION="$TAG"
else
  COMMITS=$(git rev-list --count $TAG..HEAD)
  VERSION="$TAG-$COMMITS-$COMMIT"
  
  if [ -n "$(git status --porcelain)" ]; then
    VERSION="$VERSION-dirty"
  fi
fi

# Build
go build -ldflags="-X github.com/weka/kubectl-weka/pkg/version.Version=$VERSION -X github.com/weka/kubectl-weka/pkg/version.Commit=$COMMIT -X github.com/weka/kubectl-weka/pkg/version.Date=$DATE" -o kubectl-weka .

# Install
go install -ldflags="-X github.com/weka/kubectl-weka/pkg/version.Version=$VERSION -X github.com/weka/kubectl-weka/pkg/version.Commit=$COMMIT -X github.com/weka/kubectl-weka/pkg/version.Date=$DATE" .
```

### Tagged Releases

For official releases, create a git tag:

```bash
# Create a semantic version tag
git tag v1.0.0
git push origin v1.0.0

# Now when you build, it will automatically detect it's a release
make build
# Version will be exactly: v1.0.0
```

Development continues after the tag:

```bash
# After 5 more commits
make build
# Version will be: v1.0.0-5-abc123d (5 commits after tag)
```

---

## Architecture Overview

### Project Structure

```
cmd/
├── root.go                          # Root command definition
├── preflight*.go                    # Preflight commands
├── plan*.go                         # Plan commands
├── get*.go                          # Get commands
├── logs*.go                         # Logs commands
├── supportbundle*.go                # Support bundle commands
├── hostcheck*.go                    # Host check modules & registry
├── clustercheck*.go                 # Cluster check modules & registry
├── wekaconfig*.go                   # WEKA config validation modules & registry
└── utils.go                         # Shared utilities
```

### Key Design Patterns

#### 1. **Registry Pattern**
Checks and collectors are registered with central registries:
- `GlobalHostCheckRegistry` – Node-level checks
- `GlobalClusterCheckRegistry` – Cluster-level checks
- `GlobalWekaConfigValidationRegistry` – WEKA resource validations

#### 2. **Module Interface**
All checks implement a standard interface for consistency.

#### 3. **Context-Based Execution**
Collectors receive context with clients, namespace, paths via `context.Context`.

#### 4. **Streaming Output**
Long-running operations use `PreflightOutput` for dual screen+file output.

---

## ResourcePrinter System

The ResourcePrinter system formats and outputs produced output in a structured table or YAML format.

### Overview

- ResourcePrinters are registered for each resource type (e.g., Pods, Services).
- Each printer implements the `ResourcePrinter` interface.
- Common flags:
  - `-o`/`--output`: Output format (table, yaml, json).
  - `--no-headers`: Omit table headers.

### Example Printer

```go
type PodResourcePrinter struct{}

func (p *PodResourcePrinter) PrintObj(obj runtime.Object, w io.Writer) error {
	pod := obj.(*v1.Pod)
	_, err := fmt.Fprintf(w, "Pod: %s\n", pod.Name)
	return err
}
```

### Table-Driven Tests

ResourcePrinters use table-driven tests for coverage:

```go
func TestPodResourcePrinter(t *testing.T) {
	printer := &PodResourcePrinter{}
	
	var buf bytes.Buffer
	err := printer.PrintObj(pod, &buf)
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !strings.Contains(buf.String(), "Pod:") {
		t.Errorf("output did not contain expected text")
	}
}
```

---

## ResourcePrinter System

The ResourcePrinter system provides a unified abstraction for formatting and displaying resource data in multiple output formats. This design enables consistent output across all `kubectl weka get` commands while supporting table, JSON, YAML, and custom-column formats.

### Overview

**Purpose**: Standardize resource output formatting across all `get` commands

**Features**:
- ✅ Multiple output formats (table, wide, json, yaml, custom-columns)
- ✅ Column visibility control (VisibleInWide attribute)
- ✅ Custom value formatting via TableFormatFunctions
- ✅ Consistent kubectl-like behavior
- ✅ Extensible for new formats

### Architecture

#### Core Types

**ResourcePrinter Interface** (`cmd/printer.go`):
```go
type ResourcePrinter interface {
	SetOptions(opts PrinterOptions)
	Print(columns []TableColumn, rows []TableRow, w io.Writer) error
}
```

**TableColumn** - Column definition:
```go
type TableColumn struct {
	Name                 string                           // Column header name
	VisibleInWide        bool                             // Only shown with -o wide
	TableFormatFunctions []func(interface{}) string       // Value formatting functions
}
```

**TableRow** - Data row:
```go
type TableRow struct {
	Values map[string]interface{}  // Column name -> value mapping
}
```

**PrinterOptions** - Output configuration:
```go
type PrinterOptions struct {
	ShowHeader        bool        // Include header row
	WideOutput        bool        // Show VisibleInWide columns
	ColumnsList       []string    // Explicitly selected columns
	HideColumnsList   []string    // Columns to hide (case-insensitive)
	HideEmptyColumns  bool        // Omit empty columns
	IndentByNumSpaces int         // Indentation for output
	TableStyle        TableStyle  // Table rendering style
}
```

#### Printer Implementations

**TablePrinter** - Human-readable tables (`cmd/table_printer.go`):
```go
type TablePrinter struct {
	opts PrinterOptions
}

func (tp *TablePrinter) Print(columns []TableColumn, rows []TableRow, w io.Writer) error {
	// Filters columns based on visibility and selection rules
	// Formats values using TableFormatFunctions
	// Renders as pretty table with go-pretty/v6/table
}
```

**JsonPrinter** - JSON output (`cmd/json_printer.go`):
```go
type JsonPrinter struct {
	opts PrinterOptions
}
```

**YamlPrinter** - YAML output (`cmd/yaml_printer.go`):
```go
type YamlPrinter struct {
	opts PrinterOptions
}
```

### Usage Pattern

#### Step 1: Define Columns

```go
columns := []TableColumn{
	{Name: "NAME", VisibleInWide: false},
	{Name: "IP", VisibleInWide: false},
	{Name: "STATUS", VisibleInWide: false},
	{Name: "AGE", VisibleInWide: true},  // Only with -o wide
	{Name: "CPU", VisibleInWide: true, TableFormatFunctions: []func(interface{}) string{
		func(val interface{}) string {
			if cpu, ok := val.(float64); ok {
				return fmt.Sprintf("%.2f", cpu)
			}
			return "-"
		},
	}},
}
```

**Column Visibility Rules**:
- `VisibleInWide: false` → Shown in default and wide output
- `VisibleInWide: true` → Shown only in wide output (`-o wide`)

#### Step 2: Build Rows

```go
var rows []TableRow
for _, item := range items {
	row := TableRow{Values: map[string]interface{}{
		"NAME":   item.Name,
		"IP":     item.IP,
		"STATUS": item.Status,
		"AGE":    item.CreationTime,
		"CPU":    item.CPUUsage,
	}}
	rows = append(rows, row)
}
```

#### Step 3: Get Printer from Flags

```go
printer, _ := GetPrinterFromFlags(
	flagOutput,           // "-o" flag value
	!flagNoHeaders,       // Show headers
	nil,                  // Hide columns list
	false,                // Hide empty columns
	0,                    // No indentation
	TableStyleMinimal,    // Table style
)
```

#### Step 4: Render Output

```go
var output strings.Builder
if err := printer.Print(columns, rows, &output); err != nil {
	return "", err
}
return output.String() + "\n", nil
```

### Format Selection Function

**GetPrinterFromFlags** (`cmd/printer_factory.go`):

```go
func GetPrinterFromFlags(
	outputFlag string,
	showHeader bool,
	hideColumnsList []string,
	hideEmptyColumns bool,
	indentByNumSpaces int,
	tableStyle TableStyle,
) (ResourcePrinter, []string) {
	// Parses output flag: "table" (default), "wide", "json", "yaml", "custom-columns=..."
	// Returns appropriate printer and column list
}
```

**Output Format Support**:

| Format | Value | Behavior |
|--------|-------|----------|
| Default | `""` or `"table"` | Table with default columns |
| Wide | `"wide"` | Table with VisibleInWide columns |
| JSON | `"json"` | JSON array of row objects |
| YAML | `"yaml"` | YAML array of row objects |
| Custom | `"custom-columns=COL1,COL2"` | Table with selected columns only |

### Value Formatting

**TableFormatFunctions** allow custom formatting of column values:

```go
type TableColumn struct {
	Name: "MEMORY",
	TableFormatFunctions: []func(interface{}) string{
		func(val interface{}) string {
			if bytes, ok := val.(int64); ok {
				return fmt.Sprintf("%.2fGB", float64(bytes)/(1024*1024*1024))
			}
			return "-"
		},
	},
}
```

**Real-World Example** - formatQuantityToGB (`cmd/get_nodes.go`):

```go
func formatQuantityToGB(val interface{}) string {
	qty, ok := val.(resource.Quantity)
	if !ok {
		if ptr, ok := val.(*resource.Quantity); ok && ptr != nil {
			qty = *ptr
		} else {
			return "-"
		}
	}
	
	bytes := qty.Value()
	// ... format bytes to human-readable GB/MB/KB
	return formatted
}

// Usage in column definition
{
	Name: "MEMORY_USABLE",
	TableFormatFunctions: []func(interface{}) string{formatQuantityToGB},
}
```

### Real-World Examples

#### Example 1: Simple Table (get nodes)

```go
columns := []TableColumn{
	{Name: "NAME", VisibleInWide: false},
	{Name: "IP", VisibleInWide: false},
	{Name: "STATUS", VisibleInWide: false},
	{Name: "CORES_FREE", VisibleInWide: false, 
	 TableFormatFunctions: []func(interface{}) string{formatQuantityToGB}},
	{Name: "RAM_FREE", VisibleInWide: false,
	 TableFormatFunctions: []func(interface{}) string{formatQuantityToGB}},
	{Name: "AGE", VisibleInWide: true},
	{Name: "CPU_UTIL", VisibleInWide: true},
}

// Usage:
// kubectl weka get nodes                    # Default columns
// kubectl weka get nodes -o wide            # Includes AGE, CPU_UTIL
// kubectl weka get nodes -o json            # JSON output
// kubectl weka get nodes -o custom-columns=NAME,IP,STATUS
```

#### Example 2: With Namespace Column (get cluster-instances -A)

```go
var columns []TableColumn
if allNamespaces {
	columns = []TableColumn{
		{Name: "NAMESPACE", VisibleInWide: false},  // Only with -A
		{Name: "WEKACLUSTER", VisibleInWide: false},
		// ... other columns
	}
} else {
	columns = []TableColumn{
		{Name: "WEKACLUSTER", VisibleInWide: false},
		// ... other columns (no NAMESPACE)
	}
}
```

#### Example 3: Custom Format Function

```go
// Define formatting function
func formatStatus(val interface{}) string {
	if status, ok := val.(string); ok {
		switch status {
		case "Running":
			return "✓ Running"
		case "Pending":
			return "⏳ Pending"
		case "Failed":
			return "✗ Failed"
		}
	}
	return "-"
}

// Use in column
{
	Name: "STATUS",
	TableFormatFunctions: []func(interface{}) string{formatStatus},
}
```

### Command Implementation Template

Here's the standard pattern for implementing a new `get` command with ResourcePrinter:

```go
var flagOutput string

func init() {
	getCmd.AddCommand(getExampleCmd)
	getExampleCmd.Flags().StringVarP(&flagOutput, "output", "o", "", 
		"Output format: table, wide, json, yaml, custom-columns=<COLS...>")
}

func runGetExample(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	printer, _ := GetPrinterFromFlags(flagOutput, true, nil, false, 0, TableStyleMinimal)
	output, err := generateExampleOutput(ctx, KubeClients, printer)
	if err != nil {
		return err
	}
	
	fmt.Print(output)
	return nil
}

func generateExampleOutput(ctx context.Context, clients *K8sClients, printer ResourcePrinter) (string, error) {
	// Fetch data
	// ... your logic to retrieve data ...
	
	// Define columns
	columns := []TableColumn{
		{Name: "FIELD1", VisibleInWide: false},
		{Name: "FIELD2", VisibleInWide: false},
		{Name: "WIDE_ONLY", VisibleInWide: true},
	}
	
	// Build rows
	var rows []TableRow
	for _, item := range items {
		row := TableRow{Values: map[string]interface{}{
			"FIELD1": item.Field1,
			"FIELD2": item.Field2,
			"WIDE_ONLY": item.WideOnlyField,
		}}
		rows = append(rows, row)
	}
	
	// Render
	var buf strings.Builder
	_ = printer.Print(columns, rows, &buf)
	return buf.String() + "\n", nil
}
```

### Testing ResourcePrinter Output

```go
func TestResourcePrinter(t *testing.T) {
	columns := []TableColumn{
		{Name: "NAME", VisibleInWide: false},
		{Name: "VALUE", VisibleInWide: false},
	}
	
	rows := []TableRow{
		{Values: map[string]interface{}{
			"NAME": "test1",
			"VALUE": "value1",
		}},
	}
	
	// Test table output
	printer := &TablePrinter{}
	printer.SetOptions(PrinterOptions{ShowHeader: true})
	
	var buf strings.Builder
	err := printer.Print(columns, rows, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Error("header not found in output")
	}
	if !strings.Contains(output, "test1") {
		t.Error("data not found in output")
	}
}
```

### Extending for New Output Formats

To add a new output format:

1. **Create new printer** (e.g., `cmd/xml_printer.go`):
```go
type XmlPrinter struct {
	opts PrinterOptions
}

func (xp *XmlPrinter) SetOptions(opts PrinterOptions) {
	xp.opts = opts
}

func (xp *XmlPrinter) Print(columns []TableColumn, rows []TableRow, w io.Writer) error {
	// Implement XML formatting
}
```

2. **Register in GetPrinterFromFlags** (`cmd/printer_factory.go`):
```go
case "xml":
	printer = &XmlPrinter{}
```

3. **Update documentation** with new format option

### Best Practices

1. **Always define VisibleInWide** explicitly
2. **Use TableFormatFunctions** for non-string values
3. **Handle nil values** gracefully (return "-")
4. **Sort rows consistently** before passing to printer
5. **Test all output formats** (table, wide, json, yaml)
6. **Document column visibility** in command help text

---

### Node Preflight Checks

Node preflight checks validate individual Kubernetes nodes by deploying temporary pods to inspect node conditions.

#### Step 1: Create a New Module

Create a new file in `cmd/` (e.g., `hostcheck_module_example.go`):

```go
package cmd

import (
	"context"
	"fmt"
)

// ExampleHostCheckModule validates example node configuration
type ExampleHostCheckModule struct{}

func (m *ExampleHostCheckModule) Name() ModuleName {
	return "example_check"
}

func (m *ExampleHostCheckModule) FriendlyName() string {
	return "Example Check"
}

func (m *ExampleHostCheckModule) AppliesTo() []string {
	// Specify which commands this module runs in
	return []string{"preflight_nodes"}
}

func (m *ExampleHostCheckModule) Validate(
	ctx context.Context,
	node *corev1.Node,
	hostCheck *HostCheckFacts,
	params map[string]interface{},
) (*HostCheckResult, error) {
	// Access host check facts collected from the node
	// hostCheck contains: OSRelease, KernelVersion, NVMeDrives, etc.
	
	// Perform validation logic
	isValid := true // Your validation logic here
	detailValue := "some detail"
	
	if !isValid {
		return &HostCheckResult{
			ModuleName: m.Name(),
			Status:     "error",
			Data: map[string]interface{}{
				"Issue": "Example check failed",
				"Detail": detailValue,
			},
			ErrorTemplate:               "Example check failed: {{.Detail}}",
			SuggestedResolutionTemplate: "Fix the issue by doing X on node {{.NodeName}}",
		}, nil
	}
	
	// Success case
	return &HostCheckResult{
		ModuleName: m.Name(),
		Status:     "success",
		Data: map[string]interface{}{
			"Detail": detailValue,
		},
		SuccessTemplate: "Example check passed: {{.Detail}}",
	}, nil
}

func (m *ExampleHostCheckModule) SuccessTemplate() string {
	return "{{.FriendlyName}}: {{.Detail}}"
}

func (m *ExampleHostCheckModule) WarningTemplate() string {
	return "{{.FriendlyName}}: ⚠️  {{.Warning}}"
}

func (m *ExampleHostCheckModule) ErrorTemplate() string {
	return "{{.FriendlyName}}: ❌ {{.Issue}}"
}

func (m *ExampleHostCheckModule) SuggestedResolutionTemplate() string {
	return "Run: some command --fix"
}
```

#### Step 2: Register the Module

In `cmd/hostcheck_modules.go`, add to the `init()` function:

```go
func init() {
	// ...existing modules...
	
	// Register your new module
	GlobalHostCheckRegistry.Register(&ExampleHostCheckModule{})
}
```

#### Step 3: Test

```bash
go build -o kubectl-weka .
./kubectl-weka preflight nodes
```

Your new check will automatically run on all nodes.

---

### Template Interpolation

Templates use Go's `text/template` syntax with data from `HostCheckResult.Data`:

```go
Data: map[string]interface{}{
	"Issue": "Low disk space",
	"AvailableGB": 50,
	"RequiredGB": 100,
}

ErrorTemplate: "{{.FriendlyName}}: {{.Issue}} ({{.AvailableGB}}GB available, {{.RequiredGB}}GB required)"
```

**Output:** `Example Check: Low disk space (50GB available, 100GB required)`

---

### Cluster Preflight Checks

Cluster-level checks validate Kubernetes cluster configuration and permissions.

#### Step 1: Create a Cluster Check Module

Create `cmd/clustercheck_module_example.go`:

```go
package cmd

import (
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ExampleClusterCheckModule struct{}

func (m *ExampleClusterCheckModule) Name() ModuleName {
	return "example_cluster_check"
}

func (m *ExampleClusterCheckModule) FriendlyName() string {
	return "Example Cluster Check"
}

func (m *ExampleClusterCheckModule) Validate(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	crClient client.Client,
	params map[string]interface{},
) (*ClusterCheckResult, error) {
	// Access cluster information via clientset or CRClient
	
	// Perform validation
	isValid := true
	
	if !isValid {
		return &ClusterCheckResult{
			ModuleName: m.Name(),
			Status:     "error",
			Data: map[string]interface{}{
				"Issue": "Cluster check failed",
			},
			ErrorTemplate:               "{{.Issue}}",
			SuggestedResolutionTemplate: "Fix by running: kubectl apply -f fix.yaml",
		}, nil
	}
	
	return &ClusterCheckResult{
		ModuleName: m.Name(),
		Status:     "success",
		Data: map[string]interface{}{
			"Version": "1.30.0",
		},
		SuccessTemplate: "{{.FriendlyName}}: {{.Version}}",
	}, nil
}

func (m *ExampleClusterCheckModule) SuccessTemplate() string {
	return "✅ {{.FriendlyName}}: Success"
}

func (m *ExampleClusterCheckModule) WarningTemplate() string {
	return "⚠️  {{.FriendlyName}}: {{.Warning}}"
}

func (m *ExampleClusterCheckModule) ErrorTemplate() string {
	return "❌ {{.FriendlyName}}: {{.Issue}}"
}

func (m *ExampleClusterCheckModule) SuggestedResolutionTemplate() string {
	return "Fix the issue"
}
```

#### Step 2: Register the Module

In `cmd/clustercheck_modules.go`:

```go
func init() {
	// ...existing modules...
	GlobalClusterCheckRegistry.Register(&ExampleClusterCheckModule{})
}
```

---

## Adding Plan Validations

Plan validations analyze WEKA resource specifications (WekaCluster, WekaClient) before deployment.

### WekaCluster Validations

#### Step 1: Create a Validation Module

Create `cmd/wekaconfig_module_example.go`:

```go
package cmd

import (
	"context"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

type ExampleClusterValidationModule struct{}

func (m *ExampleClusterValidationModule) Name() ModuleName {
	return "example_cluster_validation"
}

func (m *ExampleClusterValidationModule) FriendlyName() string {
	return "Example Cluster Validation"
}

func (m *ExampleClusterValidationModule) AppliesTo() []WekaConfigType {
	return []WekaConfigType{WekaConfigTypeCluster}
}

func (m *ExampleClusterValidationModule) Validate(
	ctx context.Context,
	config *WekaConfigValidationContext,
) (interface{}, error) {
	cluster := config.Cluster
	
	// Validate cluster specification
	if cluster.Spec.Dynamic == nil {
		return map[string]interface{}{
			"Status": "error",
			"Issue":  "Dynamic template is required",
		}, nil
	}
	
	// Success
	return map[string]interface{}{
		"Status": "success",
		"Detail": "Validation passed",
	}, nil
}

func (m *ExampleClusterValidationModule) SuccessTemplate() string {
	return "✅ {{.FriendlyName}}: {{.Detail}}"
}

func (m *ExampleClusterValidationModule) WarningTemplate() string {
	return "⚠️  {{.FriendlyName}}: {{.Warning}}"
}

func (m *ExampleClusterValidationModule) ErrorTemplate() string {
	return "❌ {{.FriendlyName}}: {{.Issue}}"
}

func (m *ExampleClusterValidationModule) SuggestedResolutionTemplate() string {
	return "Set spec.dynamic in your WekaCluster YAML"
}
```

#### Step 2: Register the Module

In `cmd/wekaconfig_modules.go`:

```go
func init() {
	// ...existing modules...
	GlobalWekaConfigValidationRegistry.Register(&ExampleClusterValidationModule{})
}
```

### WekaClient Validations

Similar to cluster validations, but specify `WekaConfigTypeClient` in `AppliesTo()`:

```go
func (m *ExampleClientValidationModule) AppliesTo() []WekaConfigType {
	return []WekaConfigType{WekaConfigTypeClient}
}

func (m *ExampleClientValidationModule) Validate(
	ctx context.Context,
	config *WekaConfigValidationContext,
) (interface{}, error) {
	client := config.Client
	
	// Validate client specification
	// ...
}
```

---

## Adding Support Bundle Collectors

Support bundle collectors gather diagnostic data and save it to files.

### Step 1: Create a Collector

Create `cmd/supportbundle_example.go`:

```go
package cmd

import (
	"context"
	"fmt"
	"path/filepath"
)

// ExampleCollector collects example diagnostic data
type ExampleCollector struct {
	ResourceName string // Optional: filter by resource name
}

func (c *ExampleCollector) Name() ModuleName {
	return "Example Data"
}

func (c *ExampleCollector) Start(ctx context.Context) {
	logger := GetLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "example data files")
}

func (c *ExampleCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string
	
	logger := GetLogger(ctx)
	clients := getClients(ctx)
	bundlePath := getBundlePath(ctx)
	namespace := getNamespace(ctx)
	allNamespaces := getAllNamespaces(ctx)
	
	// Perform data collection
	logger.Debug("Collecting example data", "namespace", namespace)
	
	// Example: List resources
	// var resourceList SomeResourceList
	// if err := clients.CRClient.List(ctx, &resourceList, options...); err != nil {
	//     return CollectorResult{
	//         Status: StatusFailure,
	//         Error: fmt.Errorf("failed to list resources: %w", err),
	//     }
	// }
	
	// Example: Collect and write data
	exampleData := "collected data content"
	filePath := filepath.Join("example", "data.txt")
	if err := writeToFile(bundlePath, filePath, exampleData); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to write data: %v", err))
		logger.Debug("⚠️  Failed to write file", "error", err)
	} else {
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected data", "file", filePath)
	}
	
	// Determine status
	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}
	
	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}
}

func (c *ExampleCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := GetLogger(ctx)
	
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files", len(result.FilesCreated))
		if len(result.Warnings) > 0 {
			logger.Info("Non-fatal warnings found", "count", len(result.Warnings))
			for _, warning := range result.Warnings {
				logger.Info("Warning", "message", warning)
			}
		}
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}
```

### Step 2: Register the Collector

In `cmd/supportbundle_common.go`, add to `collectorsByMode()`:

```go
func collectorsByMode(mode CollectionMode, resourceName string) []Collector {
	switch mode {
	case ModeAll:
		return []Collector{
			&ExampleCollector{ResourceName: resourceName},
			// ... other collectors ...
		}
	// ...
	}
}
```

**Real-World Examples:**
- `NodesDescriptionCollector` (`cmd/supportbundle_nodes.go`) – Collects node descriptions, nodes table, and **host checks in JSON** in **all** support-bundle modes
  - Uses `GlobalHostCheckRegistry.GetHostChecksForNodes()` for caching efficiency
  - Dumps `HostChecksResult` as pretty-printed JSON to `node-hostchecks/{nodeName}_hostcheck.json`
  - Collects hardware info: CPU, memory, NVMe, Mellanox NICs, LACP bonds, WEKA directory status
- `OperatorLogsCollector` (`cmd/supportbundle_operator.go`) – Collects operator-specific logs and resources
- `ClusterResourcesCollector` (`cmd/supportbundle_cluster.go`) – Collects cluster-specific data with namespace filtering

These collectors demonstrate best practices for parallel collection, error handling, and organized output.

### Special Case: Host Checks Collection

The `NodesDescriptionCollector` includes a specialized `collectHostChecks()` method that:

1. **Uses Registry Caching**
   ```go
   hostChecksMap, err := GlobalHostCheckRegistry.GetHostChecksForNodes(ctx, *nodes)
   ```
   This leverages cached results if host checks were already run elsewhere (e.g., `preflight nodes`)

2. **Pretty-Prints JSON**
   ```go
   jsonData, err := json.MarshalIndent(hostCheckResult, "", "  ")
   ```
   Output is human-readable with 2-space indentation

3. **Organized Output Structure**
   - Directory: `node-hostchecks/`
   - Files: `{nodeName}_hostcheck.json`
   - One file per node with complete host check data

4. **Extended Hostcheck Data**
   
   The hostcheck now collects comprehensive information:
   
   - **OS and System**
     - OS release information
     - Kernel version
     - CPU model, family, architecture
   
   - **Network Interfaces** (generic section)
     - All Ethernet and InfiniBand interfaces
     - Connection type and speeds (max and effective)
     - PCI addresses for hardware mapping
     - MTU, MAC address, bonding information
     - Network metrics: bytes/packets in/out
     - Error tracking: errors, drops, collisions, overruns, CRC errors
   
   - **Mellanox-Specific Interfaces** (backward compatible)
     - Mellanox NIC detection
     - LACP bond configuration
     - Bond-specific information
   
   - **Storage**
     - NVMe drive inventory with PCI addresses
     - Drive models, serial numbers, sizes
     - Mount point information
   
   - **Compute Resources**
     - CPU cores (physical and logical)
     - Hyperthreading status
     - Memory capacity and available memory
     - Hugepage configuration
   
   - **WEKA Resources**
     - WEKA directory availability
     - Available storage space
     - WEKA client status
     - XFS tools detection

**Example JSON Output with Network Interfaces:**
```json
{
  "os_release": "Ubuntu 22.04 LTS",
  "kernel_version": "5.15.0-1234-aws",
  "network_interfaces": [
    {
      "name": "eth0",
      "type": "ethernet",
      "ip": "10.0.0.1/24",
      "mtu": 1500,
      "max_speed": "10Gbps",
      "effective_speed": "10Gbps",
      "pci_address": "0000:01:00.0",
      "model": "Intel I350",
      "status": "up",
      "metrics": {
        "bytes_in": 5000000000,
        "bytes_out": 3000000000,
        "packets_in": 5000000,
        "errors_in": 0,
        "crc_errors": 0
      }
    },
    {
      "name": "ib0",
      "type": "infiniband",
      "ip": "192.168.1.10/24",
      "mtu": 2048,
      "max_speed": "400Gbps",
      "effective_speed": "400Gbps",
      "pci_address": "0000:3d:00.0",
      "model": "Mellanox ConnectX-7",
      "status": "up",
      "metrics": {
        "bytes_in": 50000000000,
        "bytes_out": 30000000000
      }
    }
  ],
  "network_interface_count": 2,
  "nvme_drives": [
    {
      "device_name": "nvme0n1",
      "device_path": "/dev/nvme0n1",
      "serial": "SERIAL123",
      "model": "Samsung 970 EVO",
      "size": 1099511627776,
      "pci_address": "0000:01:00.0",
      "mounted": true,
      "mount_point": "/mnt/nvme0n1"
    }
  ],
  "weka_dir_ok": true,
  "weka_dir_path": "/mnt/weka",
  "weka_dir_avail_bytes": 1649267441664,
  "physical_cores": 32,
  "logical_cores": 64,
  "memory_bytes": 274877906944,
  "cpu_model": "Intel Xeon Platinum",
  "nvme_drive_count": 4,
  ...
}
```

### Collector Best Practices

1. **Use Context Helpers**
   ```go
   logger := GetLogger(ctx)
   clients := getClients(ctx)
   bundlePath := getBundlePath(ctx)
   namespace := getNamespace(ctx)
   ```

2. **Handle Errors Gracefully**
   - Non-critical errors → Add to `warnings`, return `StatusPartial`
   - Critical errors → Return `StatusFailure` with `Error` set

3. **Log Progress**
   ```go
   logger.Debug("✓ Collected resource", "namespace", ns, "name", name)
   logger.Debug("⚠️  Failed to collect", "error", err)
   ```

4. **Organize Files**
   ```go
   filepath.Join("category", "subcategory", "filename.ext")
   ```

5. **Use Parallel Collection for Scalability**
   ```go
   logFiles, warnings := CollectPodLogsParallel(ctx, clients, namespace, pods, "logs", 10)
   ```

---

## Adding New Commands

### Step 1: Define the Command

Create `cmd/newcommand.go`:

```go
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "newcommand [args]",
	Short: "Brief description of the command",
	Long:  `Detailed description of what the command does.`,
	Args:  cobra.ExactArgs(1), // or cobra.NoArgs, cobra.MinimumNArgs(1), etc.
	RunE:  runNewCommand,
}

func init() {
	// Add to root command
	rootCmd.AddCommand(newCmd)
	
	// Add flags
	newCmd.Flags().StringVarP(&someFlag, "flag", "f", "default", "Flag description")
	newCmd.SilenceUsage = true
}

var someFlag string

func runNewCommand(cmd *cobra.Command, args []string) error {
	// Command implementation
	fmt.Println("Running new command with arg:", args[0])
	return nil
}
```

### Step 2: Add Tests

Create `cmd/newcommand_test.go`:

```go
package cmd

import (
	"testing"
)

func TestNewCommand(t *testing.T) {
	// Test implementation
}
```

### Command Design Guidelines

1. **Follow kubectl Conventions**
   - Use standard flags: `-n/--namespace`, `-A/--all-namespaces`, `--no-headers`
   - Support `--output` for different formats (table, json, yaml)

2. **Error Handling**
   ```go
   if err != nil {
       return fmt.Errorf("failed to do something: %w", err)
   }
   ```

3. **Silent Usage on Errors**
   ```go
   newCmd.SilenceUsage = true
   ```

4. **Use Color Output**
   ```go
   import "github.com/fatih/color"
   
   green := color.New(color.FgGreen).SprintFunc()
   red := color.New(color.FgRed).SprintFunc()
   fmt.Printf("%s Success\n", green("✓"))
   ```

---

## Testing Guidelines

### Unit Tests

```go
func TestExampleModule(t *testing.T) {
	module := &ExampleModule{}
	
	// Test Name()
	if module.Name() != "example" {
		t.Errorf("Expected name 'example', got '%s'", module.Name())
	}
	
	// Test Validate()
	result, err := module.Validate(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("Expected success, got %s", result.Status)
	}
}
```

### Integration Tests

Require access to a Kubernetes cluster:

```go
func TestPreflight(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// Test against real cluster
}
```

Run with: `go test -v ./...` (skip integration: `go test -short ./...`)

---

## Code Organization Best Practices

### File Naming Conventions

- `<command>_<subcommand>.go` – Command implementations
- `<feature>_module.go` – Individual modules
- `<feature>_modules.go` – Module registration
- `<feature>_registry.go` – Registry implementation
- `<feature>_common.go` – Shared utilities

### Module Registration Pattern

All modules follow this pattern:

1. **Define the interface** (e.g., `HostCheckModule`)
2. **Implement modules** (e.g., `OSHostCheckModule`)
3. **Register in `init()`** (e.g., `GlobalHostCheckRegistry.Register()`)
4. **Registry validates all** (e.g., `ValidateAll()`)

### Context-Based Passing

Use `context.Context` with helper functions:

```go
// Set values
ctx = withClients(ctx, clients)
ctx = withBundlePath(ctx, path)
ctx = withLogger(ctx, logger)

// Get values
clients := getClients(ctx)
bundlePath := getBundlePath(ctx)
logger := GetLogger(ctx)
```

---

## Debugging Tips

### Enable Debug Logging

```bash
kubectl weka support-bundle all --debug
```

### Check Module Registration

Add debug output in `init()`:

```go
func init() {
	fmt.Println("Registering ExampleModule")
	GlobalHostCheckRegistry.Register(&ExampleModule{})
}
```

### Test Individual Modules

Create a test command:

```go
func runTestModule(cmd *cobra.Command, args []string) error {
	module := &ExampleModule{}
	result, err := module.Validate(ctx, params)
	fmt.Printf("Result: %+v\nError: %v\n", result, err)
	return nil
}
```

---

## Common Patterns

### Numeric/Natural Sorting

```go
// Sort items numerically (node1, node2, node11 instead of node1, node11, node2)
sort.Slice(items, func(i, j int) bool {
	return compareNodeNames(items[i].Name, items[j].Name) < 0
})
```

The `compareNodeNames()` function in `plan_common.go` handles numeric comparison and is reused across multiple commands (get nodes, plan output, etc.) for consistency.

### Parallel Data Collection

```go
// Collect logs in parallel
logFiles, warnings := CollectPodLogsParallel(
	ctx,
	clients,
	namespace,
	podNames,
	"logs",      // subdirectory
	10,          // max concurrency
)
```

### Resource Collection

```go
var resourceList SomeResourceList
opts := []crclient.ListOption{
	crclient.InNamespace(namespace),
	crclient.MatchingLabels{"app": "weka"},
}
if err := clients.CRClient.List(ctx, &resourceList, opts...); err != nil {
	return err
}

for _, resource := range resourceList.Items {
	// Process each resource
}
```

### YAML Serialization

```go
import "sigs.k8s.io/yaml"

yamlData, err := yaml.Marshal(object)
if err != nil {
	return err
}

// Optionally redact sensitive data
if !includeSensitive {
	yamlString = redactSensitiveData(string(yamlData))
}
```

---

## Release Checklist

Before creating a pull request:

1. ✅ **Tests Pass**: `go test ./...`
2. ✅ **Build Succeeds**: `go build -o kubectl-weka .`
3. ✅ **Conventional Commits**: Use `feat:`, `fix:`, `docs:`, `refactor:` prefixes
4. ✅ **Documentation Updated**: Update README.md if adding commands
5. ✅ **Module Registered**: Ensure `init()` registers new modules
6. ✅ **Code Formatted**: Run `go fmt ./...`
7. ✅ **Linter Clean**: Run `golangci-lint run` (if available)

---

## Additional Resources

- [Cobra Documentation](https://github.com/spf13/cobra)
- [Controller-Runtime Client](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client)
- [Kubernetes Client-Go](https://github.com/kubernetes/client-go)
- [WEKA K8s API CRDs](https://github.com/weka/weka-k8s-api)

---

## Getting Help

- Open an issue on GitHub
- Review existing modules for examples
- Check the `cmd/` directory for similar implementations

---

**Happy coding!** 🚀

