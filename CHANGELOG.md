# Changelog

## Unreleased

### Features

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
  * Release version (tag on HEAD): uses tag as-is (e.g., `v1.0.0`)
  * Development version (commits after tag): includes commit count and hash (e.g., `v1.0.0-5-abc123d`)
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

## 1.0.0 (2026-02-02)


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

## Changelog
