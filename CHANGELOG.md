# Changelog

## Version 0.2.0 (Unreleased)

### Features

* **Air-Gapped Deployment Subsystem** (`pkg/airgapped/`) – Offline deployment workflow:
  * Bundle creation with image downloading and chart packaging
  * Bundle validation with SHA256 signature verification
  * Image upload to air-gapped registries
  * Helm chart update with new image URLs
  * Override values file generation
  * Support for multiple architectures (amd64, arm64)
  * Comprehensive manifest with component tracking

* **Progress Tracking for Download and Extraction** – Real-time progress updates during bundle operations:
  * `download` command: Shows progress bar with bytes downloaded/total
  * `extract` command: Tracks actual bytes extracted with per-file granularity
  * Progress updates every 100ms for smooth user experience
  * Accurate byte progress instead of file count estimates
  * Periodic flushing ensures progress appears immediately

* **Improved Tar.gz Handling** – Enhanced extraction and packing:
  * `ProgressReader` wraps reader to track bytes during gzip decompression
  * `TrackingReader` provides callbacks for fine-grained progress during file extraction
  * Periodic progress updates during large file extractions
  * Support for multiple tar entry types with proper handling
  * Directory traversal attack prevention

* **Docker Package** (`pkg/docker/`) – Docker image management:
  * `DownloadDockerImage()` - Download images from registries with authentication
  * `UploadDockerImage()` - Push images to target registries
  * `UpdateTagForNewRegistry()` - Rewrite image references for air-gapped registries
  * Authentication support (.docker/config.json and credentials)
  * Multi-architecture image handling (amd64, arm64)
  * Progress tracking during upload/download

* **Logging Package** (`pkg/logging/`) – Structured logging framework:
  * Structured logging with context-based logger management
  * Log level configuration (debug, info, warn, error)
  * Integration with `context.Context` for passing loggers
  * Formatted output for console and files
  * Support for log aggregation in support bundles

* **Progress Package** (`pkg/progress/`) – Real-time progress tracking:
  * `RenderProgress()` - Display progress bars with percentage and byte counts
  * Human-readable byte formatting (B, KB, MB, GB, TB)
  * Carriage return animation for smooth updates
  * Automatic newline on completion
  * Used by download, upload, and extract operations

* **Targzutils Package** (`pkg/targzutils/`) – Tar.gz utilities:
  * `Extract()` - Extract tar.gz with progress tracking
  * `Pack()` - Create tar.gz archives
  * `NewProgressReader()` - Track bytes during decompression
  * `NewTrackingReader()` - Fine-grained progress tracking
  * Security: Directory traversal prevention
  * Performance: Periodic progress updates to avoid overhead

* **Helm Package** (`pkg/helm/`) – Helm chart manipulation:
  * `LoadChart()` - Load charts from local, HTTP, OCI sources
  * `CreateUpdatedChartArchive()` - Create new chart with updated values
  * `CreateOverrideValuesFile()` - Generate values-override files
  * `GetNestedValue()` - Extract values using dot notation
  * `SetNestedValue()` - Set nested values with structure preservation
  * `ExtractVersionFromHelmChart()` - Get chart version info

### Improvements

* Better progress feedback during long-running operations
* More accurate byte-level tracking instead of file count estimates
* Enhanced error messages with actionable guidance
* Improved code organization with focused packages

---

## Version 0.1.0 (2026-03-31)

### Features

* **logs wekacluster Command** – Stream logs from all containers in a WEKA cluster:
  * Real-time streaming with proper timestamp ordering across multiple pods
  * Parallel log fetching with configurable concurrency control
  * Flexible filtering: role (compute|s3|drive|envoy|nfs), container name, container ID, node labels
  * Optional container name prefix in output
  * All standard log options: --follow, --tail, --since, --previous
  * Time-window buffering ensures correct timestamp ordering with minimal latency
  * Graceful error handling - continues with available logs on failures

* **logs wekaclient Command** – Stream logs from all containers in a WEKA client:
  * Identical functionality to logs wekacluster but for client resources
  * Same filtering, streaming, and output options
  * Proper client ownership filtering using refactored FilterOwnerContainers

* **logs wekacontainer Command** – Stream logs from arbitrary WekaContainers:
  * No cluster or client ownership filtering - streams all WekaContainers
  * Search across single namespace or all namespaces with `-A` flag
  * Filter by container name and container ID
  * Same real-time streaming and filtering as other log commands
  * Node selector filtering support
  * Useful for cross-namespace container inspection and generic access

* **Real-Time Log Synchronization** – Improved log streaming architecture:
  * Time-window buffering (2-second default) maintains timestamp order while allowing real-time output
  * Logs appear immediately without artificial delays
  * Safe output detection prevents log reordering
  * Final flush ensures no logs are lost
  * Works seamlessly with --follow mode

* **Container Prefix Support** – Optional container identification in log output:
  * Default: includes pod and container name prefix: `[pod/container]`
  * `--no-prefix` flag for clean output without prefixes
  * Helps identify log sources in multi-container deployments

* **Node Selector Filtering** – Filter logs by node labels:
  * Comma-separated key=value pairs (e.g., `disk=ssd,region=us-west`)
  * AND logic - all labels must match
  * Works with all log commands

* **Concurrency Control** – Limit parallel log streams:
  * `--limit-concurrent` flag (default: 10, 0=unlimited)
  * Semaphore pattern prevents resource exhaustion
  * Useful for large clusters with many containers

* **Generic Object List Handler** – Eliminated code duplication:
  * `createEmptyListForKind()` factory function for creating appropriate ObjectList types
  * `getItemsFromObjectList()` generic helper using reflection to extract items
  * ~60% reduction in boilerplate code for handling multiple object types
  * Works with any ObjectList type: WekaClusterList, WekaClientList, WekaPolicyList, DeploymentList

* **Unified Output Format Flags** – All `kubectl weka get` commands now use standard `-o/--output` flag:
  * Replaces previous `--wide` boolean flag across all get subcommands
  * Supports multiple output formats: `table` (default), `wide`, `json`, `yaml`, `custom-columns=<COLS...>`
  * Consistent with native `kubectl` command behavior
  * Implemented via unified ResourcePrinter abstraction
  * Affected commands: `get nodes`, `get cluster-instances`, `get client-instances`, `get csi-instances`, `get csi-drivers`, `get csi-secrets`
  * Column visibility now controlled by `VisibleInWide` attribute in TableColumn definitions
  * See [OUTPUT_FLAG_REFACTORING.md](OUTPUT_FLAG_REFACTORING.md) for migration details

* **Extended Hostcheck Information** – Comprehensive network and storage device data:
  * Generic network interface collection: Ethernet and InfiniBand interfaces
  * Network traffic metrics: bytes/packets in/out, errors, drops, collisions, overruns, CRC errors
  * Network speed information: maximum speed and effective/negotiated speed
  * PCI address identification: for all network interfaces and NVMe drives
  * Separate sections for generic network interfaces and Mellanox-specific interfaces
  * Interface bonding support: track bond master/slave relationships
  * MTU and MAC address information
  * Interface status (up/down) detection
  * Backward compatible with existing Mellanox interface data

* **Host Checks Collection in Support Bundle** – New section collects hardware and system information:
  * Automatically runs on all nodes in all support-bundle commands
  * Uses `GlobalHostCheckRegistry` with intelligent caching to avoid redundant execution
  * Outputs pretty-printed JSON files: `node-hostchecks/{nodeName}_hostcheck.json`
  * Collected data includes: OS, kernel, CPU, memory, NVMe drives, Mellanox NICs, LACP bonds, WEKA directory status
  * Graceful error handling - continues if individual node checks fail

* **GitHub Actions CI/CD Workflows** – Complete multi-architecture automated build pipeline:
  * `build-pr.yaml` – Automated builds on pull requests for 7 platform/architecture combinations
  * `release-build.yaml` – Automated builds and asset publishing for GitHub Releases
  * Multi-platform support: Linux (amd64, arm64, armv7), macOS (amd64, arm64), Windows (amd64, arm64)
  * Release assets uploaded as raw binaries with architecture-specific names
  * Binaries named: `kubectl-weka-{version}-{os}-{arch}[.exe]`
  * PR artifacts available for 30 days for testing
  * Full version information embedded via ldflags

* **Build System with Makefile** – Complete build automation:
  * `make build` – Build binary in current directory
  * `make install` – Install binary to GOPATH/bin
  * `make clean` – Remove built binary
  * `make help` – Show build information
  * Version information automatically extracted from git tags and embedded via ldflags

* **Intelligent Versioning** – Version string adapts to git state:
  * Release version (tag on HEAD): uses tag as-is (e.g., `v0.1.0`)
  * Development version (commits after tag): includes commit count and hash (e.g., `v0.1.0-5-abc123d`)
  * Dirty detection: appends `-dirty` flag if working directory has uncommitted changes
  * Preserves 'v' prefix for consistency with kubectl utilities

* **Version Command** – New `kubectl weka version` command:
  * Displays version, commit hash, and build date
  * Useful for verifying installation and reporting bugs
  * Build information set at compile time via ldflags

* **CSI Commands** – New comprehensive CSI driver visibility commands:
  * `kubectl weka get csi-drivers` – List WEKA CSI drivers with deployment info and scaling
  * `kubectl weka get csi-instances` – List CSI pods with health status, restart counts, and unhealthy filter
  * `kubectl weka get csi-secrets` – List and validate CSI secrets with configuration checks

* **CSI Support Bundle** – New `kubectl weka support-bundle csi` command:
  * Collects CSI driver deployment information
  * Gathers CSI pod logs (current and previous) organized by driver and role
  * Validates CSI secrets with detailed error reporting
  * Extracts storage classes, persistent volumes, and persistent volume claims
  * Creates organized archive with detailed diagnostic data

* **CSI Secret Validation** – Comprehensive validation of CSI secrets:
  * Checks for required parameters (username, password, organization, endpoints, scheme)
  * Validates scheme values (http/https)
  * Detects leading/trailing whitespace issues
  * Reports validation errors to console, log file, and archive

* **CSI Pod Health Monitoring** – Enhanced pod status and health checks:
  * Displays actual pod status from container state (CrashLoopBackoff, ImagePullBackOff, etc.)
  * Aggregates restart counts across all containers
  * Tracks most recent restart time across all containers
  * Filters for unhealthy pods (>1 restart in 5 minutes)
  * Wide view mode shows detailed restart timing

* **Preflight Checks** – Validation commands for cluster and node readiness:
  * `kubectl weka preflight cluster` – Validate Kubernetes cluster readiness for WEKA deployment
  * `kubectl weka preflight nodes` – Check individual node readiness and hardware
  * Kernel validation, CPU/memory availability checks
  * Storage and network configuration validation

* **Planning Commands** – Resource requirement and deployment planning:
  * `kubectl weka plan cluster` – Calculate resource requirements for cluster deployment
  * `kubectl weka plan client` – Calculate resource requirements for client deployment
  * `kubectl weka plan converged` – Plan combined cluster and client deployment
  * Network and placement analysis, mutual compatibility checks, and detailed output

* **Get Commands** – Comprehensive resource listing with flexible output:
  * `kubectl weka get cluster-instances` – List WEKA clusters
  * `kubectl weka get client-instances` – List WEKA clients
  * `kubectl weka get nodes` – List Kubernetes nodes with WEKA-specific details
  * `kubectl weka get policies` – List WEKA policies
  * Multiple output formats: table, wide, JSON, YAML, custom columns

* **Support Bundle** – Comprehensive diagnostic collection:
  * `kubectl weka support-bundle operator` – Operator diagnostics
  * `kubectl weka support-bundle cluster` – Cluster component logs and diagnostics
  * `kubectl weka support-bundle client` – Client component logs and diagnostics
  * `kubectl weka support-bundle csi` – CSI driver diagnostics
  * `kubectl weka support-bundle k8s` – Kubernetes configuration validation
  * `kubectl weka support-bundle all` – Complete diagnostic collection
  * Organized archives with timestamped output
  * Host checks integration for hardware information
  * Graceful error handling to maximize collected data
  * Detailed logging of support bundle execution and results
  * Validation of collected data with error reporting in console and logs
  * Comprehensive documentation for support bundle usage and contents
  * Extensible architecture for adding new support bundle types and data collectors in the future
  * Optional collection of sensitive data with user confirmation and secure handling. Usually not required for standard support bundles, but available for advanced diagnostics when needed.

## Prototype (2026-02-02)

### Features

* add automatic release workflows ([0ece53c](https://github.com/weka/kubectl-weka/commit/0ece53c5cc69cdf6cd69f0494b262c4a0538311e))
* add Krew manifest ([b3493a9](https://github.com/weka/kubectl-weka/commit/b3493a94c25159ffcff3c9bc551a198090eb434f))
* get nodes, get policies ([d2397a6](https://github.com/weka/kubectl-weka/commit/d2397a680f9a2e019db950fac921ed1f2646ce05))
* kubectl weka get client-instances ([7fd182b](https://github.com/weka/kubectl-weka/commit/7fd182b22cf6c10c9ea2d67d38fa3a3c79029b35))
* kubectl weka get cluster-instances ([fce610c](https://github.com/weka/kubectl-weka/commit/fce610c12f3a973f93e933a5d4c87812eb61fabd))
* kubectl weka logs operator ([fd5bfce](https://github.com/weka/kubectl-weka/commit/fd5bfce7822e2859362e9b713c37b5cbc2d6ee90))
* rename verify to preflight and extend node preflight checks ([1151e46](https://github.com/weka/kubectl-weka/commit/1151e46fabf837ff75bf500ce59c116663ee5b2c))
* rudimentary verify node ([25e1122](https://github.com/weka/kubectl-weka/commit/25e112271e39301bc70f65fc1bc219d71f34c52f))
* rudimentary verify node ([2550a9c](https://github.com/weka/kubectl-weka/commit/2550a9c7542dfe8d52383d0abe56d798549f0d29))

