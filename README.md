# kubectl-weka

`kubectl-weka` is a kubectl plugin that provides operational visibility, preflight validation, deployment planning, and comprehensive diagnostics for **WEKA deployments on Kubernetes**.

It extends `kubectl` with WEKA-specific commands for:
- **Preflight Validation** – Kubernetes cluster and node readiness checks
- **Deployment Planning** – Resource calculation and placement analysis
- **Instance Inspection** – WEKA client and cluster container visibility
- **Log Streaming** – Operator and container log access
- **Support Bundles** – Automated diagnostic data collection
- **Operational Analytics** – Beyond standard Kubernetes primitives

The plugin is designed to feel **kubectl-native** and integrates cleanly with Kubernetes RBAC, Krew, and CI/CD workflows.

---

## Table of Contents

- [Installation](#installation)
- [Commands Overview](#commands-overview)
  - [Preflight Commands](#preflight-commands)
  - [Get Commands](#get-commands)
    - [CSI Commands](#csi-commands)
  - [Plan Commands](#plan-commands)
  - [Logs Commands](#logs-commands)
  - [Support Bundle Commands](#support-bundle-commands)
- [Developer Guide](DEVELOPER_GUIDE.md)
- [CI/CD](#cicd)
- [Contributing](#contributing)
- [License](#license)

---

## Installation

### Via Krew (recommended)

```bash
kubectl krew install weka
```
After installation, the plugin is available as:

```bash
kubectl weka
```

## Manual installation

Download a prebuilt binary from the [GitHub releases](https://github.com/weka/kubectl-weka/releases)
page and place it in your $PATH as kubectl-weka.

### Example:
```bash
curl -LO https://github.com/weka/kubectl-weka/releases/download/vX.Y.Z/kubectl-weka_X.Y.Z_linux_amd64
chmod +x kubectl-weka_X.Y.Z_linux_amd64
mv kubectl-weka_X.Y.Z_linux_amd64 /usr/local/bin/kubectl-weka
```

### Building from Source

**Prerequisites:** Go 1.25.0 or later

#### Using Makefile (recommended)

```bash
# Clone the repository
git clone https://github.com/weka/kubectl-weka.git
cd kubectl-weka

# Build binary in current directory
make build

# Or install directly to GOPATH/bin
make install

# View available targets and build info
make help
```

The Makefile automatically:
- Extracts version from git tags (with v prefix, e.g., `v1.0.0`)
- If tag is ON current HEAD: uses tag as-is
- If tag is NOT on current HEAD: appends commit count, commit hash, and "dirty" flag if uncommitted changes exist
- Retrieves the latest commit hash
- Sets the build date to current UTC time
- Injects these values via ldflags into the binary

#### Version String Examples

| Scenario | Tag | HEAD | Working Dir | Result Version |
|----------|-----|------|-------------|-----------------|
| Release | v1.0.0 | v1.0.0 | clean | v1.0.0 |
| Release | v1.0.0 | v1.0.0 | dirty | v1.0.0-abc123d-dirty |
| Development | v1.0.0 | 5 commits after | clean | v1.0.0-5-abc123d |
| Development | v1.0.0 | 5 commits after | dirty | v1.0.0-5-abc123d-dirty |

#### Manual build

```bash
git clone https://github.com/weka/kubectl-weka.git
cd kubectl-weka

# Get current version info from git
TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

# Check if tag is on HEAD
if git describe --exact-match --tags >/dev/null 2>&1; then
  # Tag is on HEAD - use it as-is
  VERSION=$TAG
else
  # Tag is not on HEAD - add commit count and hash
  COMMITS=$(git rev-list --count $TAG..HEAD)
  VERSION="$TAG-$COMMITS-$COMMIT"
  
  # Add dirty flag if there are uncommitted changes
  if [ -n "$(git status --porcelain)" ]; then
    VERSION="$VERSION-dirty"
  fi
fi

go build -ldflags="-X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE" -o kubectl-weka .

# Verify the version
./kubectl-weka version
```

## Usage
General syntax:
```
kubectl weka <command> [flags]
```
Get help for any command:
```bash
kubectl weka help
kubectl weka help preflight
kubectl weka help get
```

## Commands Overview

`kubectl-weka` provides the following command categories:

```
kubectl weka <command> [subcommand] [flags]
```

### Command Categories

| Command | Purpose | Key Subcommands |
|---------|---------|-----------------|
| `version` | Display version information | None |
| `preflight` | Pre-deployment validation | `cluster`, `nodes` |
| `get` | Inspect WEKA resources | `cluster-instances`, `client-instances`, `nodes`, `policies`, `csi-drivers`, `csi-instances` |
| `plan` | Deployment planning | `cluster`, `client`, `converged` |
| `logs` | Stream logs | `operator` |
| `support-bundle` | Diagnostic data collection | `operator`, `cluster`, `client`, `csi`, `k8s`, `all` |

Get help for any command:
```bash
kubectl weka help
kubectl weka help preflight
kubectl weka help support-bundle
```

---

## Version Command

### `version`

**Purpose:** Display the version, commit hash, and build date of kubectl-weka.

**Usage:**
```bash
kubectl weka version
```

**Output Example:**
```
kubectl-weka version 1.0.0
commit: abc123def456
date: 2026-03-11T15:30:00Z
```

**Use Cases:**
- ✅ Verify which version of kubectl-weka is installed
- ✅ Check the exact build information for bug reports
- ✅ Confirm commit hash matches a specific release
- ✅ Verify build date for version tracking

---

## Preflight Commands

Preflight commands validate that a Kubernetes cluster and its nodes meet WEKA requirements **before** installation.

### `preflight cluster`

**Purpose:** Performs cluster-level validation to ensure Kubernetes environment compatibility.

**Usage:**
```bash
kubectl weka preflight cluster [NODE...] [flags]
```

**Flags:**
- `--node-selector <label>=<value>` – Filter nodes for node-specific cluster checks (e.g., CPU policy, CNI health)

**Checks Performed:**

| Check | Description |
|-------|-------------|
| **Kubernetes Version** | Validates K8s version ≥ 1.24 |
| **Managed Cluster Detection** | Warns if ROSA/OpenShift managed cluster detected |
| **Helm Permissions** | Verifies cluster-admin permissions for WEKA operator installation |
| **CSI Driver** | Checks for existing CSI drivers |
| **CPU Policy** | Validates kubelet CPU policy is set to `static` on selected nodes |
| **CNI Configuration** | Verifies CNI is properly configured on nodes |
| **NotReady Nodes** | Reports any NotReady nodes (skipped from validation) |

**Example:**
```bash
kubectl weka preflight cluster

# Check only nodes with specific label for CPU policy validation
kubectl weka preflight cluster --node-selector role=storage
```

**Output:**
```
Performing preflight verification for Kubernetes cluster

🔍 Connecting to cluster and discovering nodes... found 129 nodes
   ✓ 129 ready, ⚠ 0 not ready (checks will skip NotReady nodes to avoid timeouts)

⚙️  Running cluster validation checks (this may take a minute)...

✅ Kubernetes Version: 1.30.0 (>= 1.24.0 required)
✅ Managed Cluster (ROSA): Not a managed ROSA/OpenShift cluster
✅ Helm Install Permissions: Cluster-admin permissions available
✅ CSI Driver Check: No conflicting CSI drivers found
✅ CPU Policy (static): All 129 nodes have kubelet CPU policy set to 'static'
✅ CNI Configuration: CNI properly configured on all nodes
✅ NotReady Nodes: All nodes are ready
```

---

### `preflight nodes`

**Purpose:** Validates individual nodes for WEKA deployment by creating temporary pods on each node to perform comprehensive checks.

**Usage:**
```bash
kubectl weka preflight nodes [NODE...] [flags]
```

**Flags:**
- `--node-selector <label>=<value>` – Label selector to filter nodes
- `--summary-only` – Only print summary (no per-node details)
- `--failed-only` – Only show failed nodes
- `--fail-fast` – Stop on first failed node
- `--weka-dir-min-fail <GB>` – Minimum GB for WEKA directory to fail (default: 100)
- `--weka-dir-min-warn <GB>` – Minimum GB for WEKA directory to warn (default: 300)

**Checks Performed:**

| Check | Description | Pass Criteria |
|-------|-------------|---------------|
| **Operating System** | Validates OS compatibility | Ubuntu or RHCOS |
| **Kernel Version** | Checks kernel version | Compatible kernel |
| **CPU & Memory** | Validates available resources | Sufficient allocatable CPU, RAM, and hugepages |
| **Hugepages** | Verifies hugepages configuration | Configured and allocatable |
| **WEKA Directory** | Checks filesystem space | ≥100GB (fail), ≥300GB (warn) |
| **XFS Tools** | Validates mkfs.xfs availability | mkfs.xfs present |
| **WEKA Client** | Ensures no existing WEKA client | No existing installation |
| **Network** | Validates NIC configuration | Mellanox NIC detection, speed, bonding (LACP) |
| **NVMe Drives** | Detects and validates NVMe drives | Drive discovery and accessibility |

**Examples:**
```bash
# Check all nodes
kubectl weka preflight nodes

# Check only nodes with specific label
kubectl weka preflight nodes --node-selector role=storage

# Check specific nodes by name
kubectl weka preflight nodes node1 node2 node3

# Only show summary
kubectl weka preflight nodes --summary-only

# Only show failed nodes
kubectl weka preflight nodes --failed-only

# Stop on first failure
kubectl weka preflight nodes --fail-fast

# Custom WEKA directory thresholds
kubectl weka preflight nodes --weka-dir-min-fail=200 --weka-dir-min-warn=500
```

**Output:**
```
Performing preflight verification for Kubernetes nodes to host WEKA
✅ Checking total number of eligible nodes... (129)
Fetching pod resource usage...
Fetched 1422 pods across 129 nodes
Performing validation...
  srv-10000358: ✅ PASSED
     ✅ Operating System: Ubuntu 24.04.3 LTS
     ✅ Kernel: 6.8.0-41-generic
     ✅ CPU & Memory: 64 cores, 1024Gi RAM available
     ✅ Hugepages: 120000Mi configured and allocatable
     ✅ Weka Directory: 450Gi available on /opt/k8s-weka
     ✅ XFS Tools: mkfs.xfs available
     ✅ Weka Client: No existing installation
     ⚠️  Network Configuration: No Mellanox NICs detected — UDP mode only
     ✅ NVMe Drives: 8 drives detected

Summary:
  Eligible nodes:      129
  Nodes skipped:       0
  Nodes checked:       129
  Nodes passed:        127
  Nodes warned:        2
  Nodes failed:        0
  Unique OSes:         1
  Unique Kernels:      1
```

---

## Get Commands

Get commands provide visibility into WEKA custom resources and their runtime status.

### `get cluster-instances`

**Purpose:** Lists all WekaContainer instances for WEKA clusters, showing deployment status and resource allocation.

**Usage:**
```bash
kubectl weka get cluster-instances [CLUSTER_NAME] [flags]
```

**Flags:**
- `-n, --namespace <string>` – Kubernetes namespace (default: current namespace)
- `-A, --all-namespaces` – List instances across all namespaces
- `--wide` – Show additional columns (AGE, CPU_UTIL)
- `--no-headers` – Don't print headers

**Output Columns:**
- `NAMESPACE` – Kubernetes namespace (with `-A`)
- `WEKACLUSTER` – Name of the WekaCluster resource
- `NODE` – Kubernetes node name
- `WEKACONTAINER` – Name of the WekaContainer instance
- `WC_STATUS` – WekaContainer status (Running, PodNotRunning, etc.)
- `POD` – Pod phase (Running, Pending, etc.)
- `MGMT_IP` – Management IP address
- `CONTAINER_ID` – WEKA container ID
- `AGE` – Age of WekaContainer (with `--wide`)
- `CPU_UTIL` – CPU utilization (with `--wide`)

**Examples:**
```bash
# List all cluster instances in current namespace
kubectl weka get cluster-instances

# List instances for a specific cluster
kubectl weka get cluster-instances weka01

# List across all namespaces
kubectl weka get cluster-instances -A

# Show additional details
kubectl weka get cluster-instances --wide
```

---

### `get client-instances`

**Purpose:** Lists all WekaContainer instances for WEKA clients, showing mount status and connectivity.

**Usage:**
```bash
kubectl weka get client-instances [CLIENT_NAME] [flags]
```

**Flags:**
- `-n, --namespace <string>` – Kubernetes namespace
- `-A, --all-namespaces` – List across all namespaces
- `--wide` – Show additional columns
- `--no-headers` – Don't print headers

**Output Columns:**
- `WEKACLIENT` – Name of the WekaClient resource
- `NODE` – Kubernetes node name
- `NAMESPACE` – Kubernetes namespace (with `-A`)
- `WEKACONTAINER` – WekaContainer instance name
- `WC_STATUS` – Container status
- `POD` – Pod phase
- `JOINED` – Whether client has joined cluster
- `CONTAINER_ID` – WEKA container ID
- `MGMT_IP` – Management IP
- `ACTIVE_MOUNTS` – Number of active mounts
- `CPU_UTIL` – CPU usage (with `--wide`)

**Examples:**
```bash
# List all client instances
kubectl weka get client-instances

# List instances for specific client
kubectl weka get client-instances weka01-clients

# All namespaces with details
kubectl weka get client-instances -A --wide
```

---

### `get nodes`

**Purpose:** Lists Kubernetes nodes with WEKA-relevant information including resource availability and status.

**Usage:**
```bash
kubectl weka get nodes [flags]
```

**Flags:**
- `--node-selector <label>=<value>` – Filter nodes by label selector (e.g., `role=storage`, `weka.io/supports-backends=true`)
- `--wide` – Show additional resource columns (allocatable and allocated resources)
- `--no-headers` – Don't print table headers

**Output Columns:**
- `NAME` – Node name (sorted numerically: node1, node2, node11, etc.)
- `IP` – Internal IP address
- `OS` – Operating system
- `ARCH` – Architecture (amd64, arm64, etc.)
- `KERNEL` – Kernel version
- `STATUS` – Node readiness status with uptime (e.g., "Ready (45d 12h)", "NotReady")
  - "Ready (uptime)" – Node is ready with how long it's been running
  - "NotReady" – Node is not ready
- `HP_FREE` – Available hugepages (2Mi)
- `CORES_FREE` – Available CPU cores
- `RAM_FREE` – Available memory
- `CLTROLE` – Client role label value
- `BKNDROLE` – Backend role label value

**Wide Output Adds:**
- `HP_ALLOCATABLE`, `HP_ALLOCATED` – Hugepages allocation info
- `CORES_ALLOCATABLE`, `CORES_ALLOCATED` – CPU allocation info
- `RAM_ALLOCATABLE`, `RAM_ALLOCATED` – Memory allocation info

**Examples:**
```bash
# List all nodes
kubectl weka get nodes

# Filter by label
kubectl weka get nodes --node-selector role=storage

# Show wide output
kubectl weka get nodes --wide

# No headers (for scripting)
kubectl weka get nodes --no-headers
```

**Output Example:**
```
NAME             IP              OS              ARCH    KERNEL                    STATUS              HP_FREE      CORES_FREE   RAM_FREE
node1            10.240.1.10     Ubuntu 24.04    amd64   6.8.0-41-generic          Ready (45d 12h)     120GB        32 cores     512GB
node2            10.240.1.11     Ubuntu 24.04    amd64   6.8.0-41-generic          Ready (2d 5h 15m)   120GB        64 cores     1024GB
node11           10.240.1.12     Ubuntu 24.04    amd64   6.8.0-41-generic          NotReady            120GB        32 cores     512GB
```

---

### `get policies`

**Purpose:** Lists WekaPolicy resources that define automated WEKA operations.

**Usage:**
```bash
kubectl weka get policies [flags]
```

**Flags:**
- `-n, --namespace <string>` – Kubernetes namespace
- `-A, --all-namespaces` – List across all namespaces

---

### `get csi-drivers`

**Purpose:** Displays CSI (Container Storage Interface) driver deployment information, showing installation method, component status, and storage usage metrics.

**Usage:**
```bash
kubectl weka get csi-drivers [DRIVER_NAME] [flags]
```

**Arguments:**
- `DRIVER_NAME` (optional) – Show only a specific CSI driver by name (must contain `weka.io`)

**Flags:**
- `--only-helm` – Show only CSI drivers installed via Helm chart (label: `app.kubernetes.io/managed-by=Helm`)
- `--only-operator` – Show only CSI drivers installed by Weka operator (label: `app.kubernetes.io/created-by=weka-operator`)
- `--wide, -w` – Show additional columns (PVs, PVCs, Bound PVs)

**Output Columns (Default):**
- `CSI DRIVER` – CSI driver name (e.g., `weka.io`, `weka-csi.weka.io`)
- `MANAGED BY` – Installation method: `Helm`, `weka-operator`, or `Unknown`
- `NAMESPACE` – Kubernetes namespace where CSI components are deployed
- `CONTROLLER` – Controller component deployment name (or `<none>`)
- `NODE DAEMONSET` – Node component daemonset name (or `<none>`)
- `STORAGECLASSES` – Number of StorageClasses that refer to this driver
- `AGE` – Time since CSI driver was installed

**Output Columns (Wide: `--wide`):**
- All default columns, plus:
- `PVS` – Total number of PersistentVolumes using this CSI driver
- `PVCS` – Total number of PersistentVolumeClaims using this CSI driver
- `BOUND PVS` – Number of PersistentVolumes in `Bound` state (actively attached)

**Examples:**
```bash
# List all weka.io CSI drivers
kubectl weka get csi-drivers

# Show only a specific CSI driver
kubectl weka get csi-drivers weka.io
kubectl weka get csi-drivers weka-csi.weka.io

# Show drivers with storage usage details
kubectl weka get csi-drivers --wide
kubectl weka get csi-drivers -w

# Show only Helm-installed drivers
kubectl weka get csi-drivers --only-helm

# Show only Weka operator-installed drivers
kubectl weka get csi-drivers --only-operator

# Specific driver with wide format
kubectl weka get csi-drivers weka.io --wide

# Filter by installation method
kubectl weka get csi-drivers --only-helm --wide
```

**Example Output (Default):**
```
CSI DRIVER              MANAGED BY      NAMESPACE   CONTROLLER           NODE DAEMONSET       STORAGECLASSES   AGE
weka-csi.weka.io        Helm            csi-weka    csi-controller       csi-node             3                45d 12h
weka-infra.weka.io      weka-operator   weka-infra  weka-csi-controller  weka-csi-node        5                10d 5h
```

**Example Output (Wide Format):**
```
CSI DRIVER              MANAGED BY      NAMESPACE   CONTROLLER           NODE DAEMONSET       STORAGECLASSES   PVS   PVCS   BOUND PVS   AGE
weka-csi.weka.io        Helm            csi-weka    csi-controller       csi-node             3                42    38     38          45d 12h
weka-infra.weka.io      weka-operator   weka-infra  weka-csi-controller  weka-csi-node        5                127   120    115         10d 5h
```

**How It Works:**
1. **CSI Driver Discovery** – Lists all CSI driver resources (cluster-wide, non-namespaced) matching `*.weka.io`
2. **Component Matching** – Associates controller Deployments and node DaemonSets by reading `CSI_DRIVER_NAME` environment variable from first container
3. **Installation Detection** – Determines installation method via labels:
   - `app.kubernetes.io/managed-by=Helm` → "Helm"
   - `app.kubernetes.io/created-by=weka-operator` → "weka-operator"
   - No labels → "Unknown"
4. **Storage Metrics (Wide Mode)**:
   - **PVs**: Counts PersistentVolumes with matching `spec.csi.driver`
   - **PVCs**: Counts PersistentVolumeClaims with StorageClasses using the driver
   - **Bound PVs**: Filters PVs with `status.phase == Bound` (actively attached volumes)

**Use Cases:**
- ✅ Verify CSI driver installation status across cluster
- ✅ Monitor storage usage by CSI driver
- ✅ Identify underutilized or orphaned CSI drivers
- ✅ Check deployment health (missing components show `<none>`)
- ✅ Compare Helm vs operator-managed deployments
- ✅ Validate PVC/PV binding status

---

### `get csi-instances`

**Purpose:** Lists CSI driver pod instances (controller and node pods) showing deployment status and restart information.

**Usage:**
```bash
kubectl weka get csi-instances [DRIVER_NAME] [flags]
```

**Arguments:**
- `DRIVER_NAME` (optional) – Show only pods of a specific CSI driver by name

**Flags:**
- `-n, --namespace <string>` – Filter by Kubernetes namespace (shows all namespaces if not set)
- `-r, --role <string>` – Filter by pod role: `controller` or `node` (shows both if not set)
- `-w, --wide` – Show additional column: last restart time
- `--unhealthy` – Show only pods with frequent restarts (more than 1 restart in the last 5 minutes)

**Output Columns (Default):**
- `CSI DRIVER` – CSI driver name
- `NAMESPACE` – Kubernetes namespace where pod is deployed
- `NODE` – Kubernetes node where the pod is running
- `ROLE` – Pod role: `controller` (deployment pod) or `node` (daemonset pod)
- `POD NAME` – Name of the CSI pod
- `STATUS` – Pod status from container state: `Running`, `CrashLoopBackoff`, `ImagePullBackOff`, `Pending`, `Succeeded`, `Failed`, `Unknown`, or other container state reasons
- `RESTARTS` – Number of times the pod container(s) have restarted
- `AGE` – Time since the pod was created

**Wide Columns (`--wide`):**
- `LAST RESTART` – Time since the pod container was last restarted (shows `N/A` if never restarted)

**Examples:**
```bash
# List all CSI pods
kubectl weka get csi-instances

# Show pods for a specific driver
kubectl weka get csi-instances weka.io

# Show only controller pods
kubectl weka get csi-instances --role controller

# Show only node pods
kubectl weka get csi-instances --role node

# Filter by namespace
kubectl weka get csi-instances -n csi-weka

# Wide view with restart timing
kubectl weka get csi-instances --wide
kubectl weka get csi-instances -w

# Show only unhealthy pods (frequent restarts)
kubectl weka get csi-instances --unhealthy
kubectl weka get csi-instances --unhealthy --wide
kubectl weka get csi-instances --unhealthy -n csi-weka

# Combine filters with wide view
kubectl weka get csi-instances weka.io -n csi-weka --role controller --wide

# Specific driver with all filters
kubectl weka get csi-instances weka-csi.weka.io -n csi-weka --role node --wide
```

**Example Output (Default):**
```
CSI DRIVER              NAMESPACE   NODE        ROLE          POD NAME                      STATUS     RESTARTS   AGE
weka-csi.weka.io        csi-weka    node1       controller    csi-controller-0              Running    0          45d 12h
weka-csi.weka.io        csi-weka    node2       controller    csi-controller-1              Running    0          45d 12h
weka-csi.weka.io        csi-weka    node1       node          csi-node-abc12               Running    2          45d 11h
weka-csi.weka.io        csi-weka    node2       node          csi-node-def45               Running    0          45d 12h
weka-csi.weka.io        csi-weka    node3       node          csi-node-ghi78               Running    1          44d 8h
weka-infra.weka.io      weka-infra  node4       controller    weka-csi-controller-6f2h9    Running    0          10d 5h
weka-infra.weka.io      weka-infra  node5       node          weka-csi-node-5m8kl          Running    0          10d 5h
weka-infra.weka.io      weka-infra  node6       node          weka-csi-node-b3n2p          Pending    1          9d 14h
```

**Example Output (Wide: `--wide`):**
```
CSI DRIVER              NAMESPACE   NODE        ROLE          POD NAME                      STATUS     RESTARTS   LAST RESTART   AGE
weka-csi.weka.io        csi-weka    node1       controller    csi-controller-0              Running    0          N/A            45d 12h
weka-csi.weka.io        csi-weka    node2       controller    csi-controller-1              Running    0          N/A            45d 12h
weka-csi.weka.io        csi-weka    node1       node          csi-node-abc12               Running    2          3d 5h          45d 11h
weka-csi.weka.io        csi-weka    node2       node          csi-node-def45               Running    0          N/A            45d 12h
weka-csi.weka.io        csi-weka    node3       node          csi-node-ghi78               Running    1          14d 2h         44d 8h
weka-infra.weka.io      weka-infra  node4       controller    weka-csi-controller-6f2h9    Running    0          N/A            10d 5h
weka-infra.weka.io      weka-infra  node5       node          weka-csi-node-5m8kl          Running    0          N/A            10d 5h
weka-infra.weka.io      weka-infra  node6       node          weka-csi-node-b3n2p          Pending    1          2d 8h          9d 14h
```

**How It Works:**
1. **Driver Discovery** – Lists all CSI driver resources matching `*.weka.io`
2. **Pod Matching** – Finds all pods with `CSI_DRIVER_NAME` environment variable set to a matching driver
3. **Role Detection** – Determines pod role by:
   - Checking `app.kubernetes.io/component` label
   - Analyzing pod name patterns (controller/node keywords)
   - Identifying parent resource type (Deployment → controller, DaemonSet → node)
4. **Restart Monitoring** – Reports restart count from container status
5. **Filtering** – Applies optional namespace and role filters

**Use Cases:**
- ✅ Monitor CSI pod health and restart patterns
- ✅ Identify unhealthy or crashing CSI components
- ✅ Verify pod distribution across nodes (controller vs node)
- ✅ Troubleshoot CSI deployment issues
- ✅ Check pod age and stability
- ✅ Investigate restart loops or pod crashes

---

### `get csi-secrets`

**Purpose:** Lists and validates CSI-related secrets referenced by storage classes, checking for required parameters and configuration issues.

**Usage:**
```bash
kubectl weka get csi-secrets
```

**Output Columns:**
- `NAME` – Secret name
- `NAMESPACE` – Kubernetes namespace where the secret is stored
- `STORAGECLASS COUNT` – Number of storage classes referencing this secret
- `VALID` – Validation status: ✓ (valid) or ✗ (invalid)
- `DETAIL` – First validation error message (if any)

**Validation Checks:**
- ✅ Required parameters: `username`, `password`, `organization`, `endpoints`, `scheme`
- ✅ Scheme value must be either `http` or `https`
- ✅ No leading or trailing whitespace on parameter values

**Example Output:**
```
NAME                  NAMESPACE    STORAGECLASS COUNT   VALID   DETAIL
weka-csi-secret       csi-weka     2                    ✓       
weka-infra-secret     weka-infra   1                    ✗       Secret weka-infra-secret/weka-infra is missing required parameter: username
backup-secret         default      1                    ✗       Secret backup-secret/default parameter 'scheme' has invalid value 'ftp' (must be 'http' or 'https')
```

**How It Works:**
1. **Driver Discovery** – Identifies all WEKA CSI drivers (matching `*.weka.io`)
2. **Storage Class Analysis** – Finds all storage classes using WEKA CSI drivers as provisioner
3. **Secret Extraction** – Extracts secret references from storage class parameters:
   - `csi.storage.k8s.io/provisioner-secret-name` and `-namespace`
   - `csi.storage.k8s.io/controller-expand-secret-name` and `-namespace`
   - `csi.storage.k8s.io/controller-publish-secret-name` and `-namespace`
   - `csi.storage.k8s.io/node-stage-secret-name` and `-namespace`
   - `csi.storage.k8s.io/node-publish-secret-name` and `-namespace`
4. **Validation** – Validates each secret against CSI requirements
5. **Deduplication** – Groups by namespace/name and counts storage class references

**Important Note:** Secrets must have explicit namespace parameters in storage class definitions. If namespace is not specified, the secret is skipped.

**Use Cases:**
- ✅ Validate CSI secret configuration
- ✅ Identify misconfigured secrets
- ✅ Find unused or missing secrets
- ✅ Detect whitespace issues in configuration
- ✅ Verify scheme configuration (http vs https)

---

### CSI Commands

CSI (Container Storage Interface) commands provide comprehensive visibility into WEKA CSI driver deployments, health, and configuration.

#### `get csi-drivers`

**Purpose:** Lists all WEKA CSI drivers and their deployment information.

**Usage:**
```bash
kubectl weka get csi-drivers [DRIVER_NAME] [flags]
```

**Output Columns:**
- `CSI DRIVER` – Driver name
- `MANAGED BY` – Installation method (Helm or weka-operator)
- `NAMESPACE` – Namespace where CSI is deployed
- `CONTROLLER` – Deployment name for controller component
- `REPLICAS (CTRL)` – Number of controller instances
- `NODE DAEMONSET` – DaemonSet name for node component
- `REPLICAS (NODE)` – Number of node instances
- `STORAGE CLASSES` – Number of storage classes using this driver
- `AGE` – Time since driver was installed

**Use Cases:**
- ✅ Verify CSI driver deployments across namespaces
- ✅ Check CSI component scaling (controller/node replicas)
- ✅ Identify which storage classes use each driver
- ✅ Monitor CSI driver age and updates

#### `get csi-instances`

**Purpose:** Lists CSI driver pods (controller and node instances) with health status.

**Usage:**
```bash
kubectl weka get csi-instances [DRIVER_NAME] [flags]
```

**Flags:**
- `-n, --namespace <string>` – Filter by namespace
- `-r, --role <string>` – Filter by role (controller or node)
- `-w, --wide` – Show additional columns (last restart time)
- `--unhealthy` – Show only pods with frequent restarts (>1 in 5 minutes)

**Output Columns:**
- `CSI DRIVER` – Driver name
- `NAMESPACE` – Pod namespace
- `NODE` – Node where pod is running
- `ROLE` – Pod role (controller or node)
- `POD NAME` – Pod name
- `STATUS` – Pod status (Running, CrashLoopBackoff, etc.)
- `RESTARTS` – Number of container restarts
- `AGE` – Pod age
- `LAST RESTART` (--wide only) – Time since last restart

**Use Cases:**
- ✅ Monitor CSI pod health and restart patterns
- ✅ Identify unhealthy or crashing CSI components
- ✅ Verify pod distribution across nodes
- ✅ Troubleshoot CSI deployment issues
- ✅ Investigate restart loops

#### `get csi-secrets`

**Purpose:** Lists and validates CSI-related secrets referenced by storage classes.

**Usage:**
```bash
kubectl weka get csi-secrets
```

**Output Columns:**
- `NAME` – Secret name
- `NAMESPACE` – Secret namespace
- `STORAGECLASS COUNT` – Number of storage classes referencing the secret
- `VALID` – Validation status (✓ or ✗)
- `DETAIL` – Validation error details (if any)

**Validation Checks:**
- ✅ Required parameters: `username`, `password`, `organization`, `endpoints`, `scheme`
- ✅ Scheme must be `http` or `https`
- ✅ No leading/trailing whitespace on parameters

**Use Cases:**
- ✅ Validate CSI secret configuration
- ✅ Identify misconfigured secrets
- ✅ Find missing or invalid secrets
- ✅ Verify parameter values

---

## Plan Commands

Plan commands analyze WEKA YAML specifications and calculate resource requirements **before** deployment.

### `plan cluster`

**Purpose:** Analyzes a WekaCluster YAML specification to calculate resource requirements, validate drive availability, and recommend node allocation.

**Usage:**
```bash
kubectl weka plan cluster <file.yaml> [flags]
```

**Flags:**
- `--fail-fast` – Stop validation on first error
- `--no-headers` – Don't print table headers

**Features:**
- ✅ **Resource Calculations** – CPU cores, memory, hugepages per container type
- ✅ **Drive Validation** – Verifies NVMe drive availability (when cluster access available)
- ✅ **Node Placement** – Shows container placement with resource allocation bars
- ✅ **Node Requirements** – Minimum nodes with 10% spare capacity + fault tolerance recommendation
- ✅ **Offline Mode** – Works without cluster access (skips drive validation)
- ✅ **Anti-affinity Awareness** – Respects container placement rules

**Resource Formulas:**

**Compute Containers:**
- Cores: `(cpuPolicy == HT ? 2×cores : cores) + extra + 1`
- Hugepages: `3000Mi × cores + 200Mi` (or explicit override)
- Memory: `2700 + (800+4400)×cores + 4000 + additionalMemory`

**Drive Containers:**
- Cores: Same as Compute
- Hugepages: `1400Mi × cores + 200Mi × numDrives` (or `1000Mi × cores` if no drives)
- Memory: `4000 + (800+2200)×cores + 700×numDrives + 4000 + additionalMemory`

**S3/NFS Containers:**
- Cores: Same as Compute
- Hugepages: `1400Mi × cores + 200Mi`
- Memory: `16000 + 2450 + (2850+200)×cores + 450 + additionalMemory`

**Envoy Containers:** (paired with S3)
- Cores: `1`
- Hugepages: `0`
- Memory: `1024 + additionalMemory`

**Example:**
```bash
kubectl weka plan cluster cluster.yaml
```

**Output:**
```
=== Container Resource Requirements ===
┌────────────────┬───────┬───────────────────┬───────────────────────┬────────────────────┐
│ Container Type │ Count │ Cores/Container   │ Hugepages/Container   │ Memory/Container   │
├────────────────┼───────┼───────────────────┼───────────────────────┼────────────────────┤
│ Compute        │     8 │                25 │             36200 MiB │          69100 MiB │
│ Drive          │     8 │                 9 │              6400 MiB │          22800 MiB │
│ S3             │     2 │                 9 │              5800 MiB │          31450 MiB │
│ Envoy (S3)     │     2 │                 1 │                  0 MiB │           1024 MiB │
└────────────────┴───────┴───────────────────┴───────────────────────┴────────────────────┘

=== Container Placement on Nodes ===
Showing resource allocation: [ALREADY_USED] [WEKA] [FREE]

┌──────────────┬────────────────────────────────────┬──────────────────────────────────┐
│ NODE         │ CONTAINERS & RESOURCES             │ RESOURCE ALLOCATION              │
├──────────────┼────────────────────────────────────┼──────────────────────────────────┤
│ srv-10000351 │ <ALREADY_USED> [CORES: 2.0,        │ CPU:    [▓▓░░░░░░░░░░░░░░░░░░]   │
│              │  RAM: 8.0Gi, HP: 0.0Gi]            │ Mem:    [▓░░░░░░░░░░░░░░░░░░░░]   │
│              │ <COMPUTE> [CORES: 25,              │ HP:     [████████████░░░░░░░░░]   │
│              │  RAM: 67.5Gi, HP: 35.4Gi]          │                                  │
│              │ <DRIVE> [CORES: 9,                 │                                  │
│              │  RAM: 22.3Gi, HP: 6.3Gi, DRIVES: 4]│                                  │
└──────────────┴────────────────────────────────────┴──────────────────────────────────┘

=== Node Requirements (with 10% spare) ===
┌────────────────────────┬───────────┬─────────────┬──────────────────┬────────────────┐
│ Purpose                │ Min Nodes │ Cores/Node  │ Hugepages/Node   │ Memory/Node    │
├────────────────────────┼───────────┼─────────────┼──────────────────┼────────────────┤
│ Backend (Compute+Drive)│         8 │          37 │        46860 MiB │      95810 MiB │
│ Frontend (S3/NFS)      │         2 │          12 │         6380 MiB │      36949 MiB │
└────────────────────────┴───────────┴─────────────┴──────────────────┴────────────────┘

💡 Recommendation: At least 1 more node is recommended for fault tolerance.

=== Validation Results ===
✅ All validations passed
```

---

### `plan client`

**Purpose:** Analyzes a WekaClient YAML specification and calculates per-node resource requirements.

**Usage:**
```bash
kubectl weka plan client <file.yaml> [flags]
```

---

### `plan converged`

**Purpose:** Combined planning for both cluster and client deployments.

**Usage:**
```bash
kubectl weka plan converged <cluster-file.yaml> <client-file.yaml> [flags]
```

---

## Logs Commands

### `logs operator`

**Purpose:** Stream logs from the WEKA operator controller manager.

**Usage:**
```bash
kubectl weka logs operator [flags]
```

**Flags:**
- `-n, --namespace <string>` – Operator namespace (default: `weka-operator-system`)
- `-f, --follow` – Follow logs (stream continuously)
- `--tail <int>` – Number of lines to show from end
- `--since <duration>` – Show logs since relative time (e.g., `5m`, `1h`)
- `--previous` – Show logs from previous container instance (if pod restarted)

**Examples:**
```bash
# Stream operator logs
kubectl weka logs operator -f

# Show last 200 lines
kubectl weka logs operator --tail=200

# Show logs from last 10 minutes
kubectl weka logs operator --since=10m

# Show previous logs if pod restarted
kubectl weka logs operator --previous
```

---

## Support Bundle Commands

Support bundle commands collect comprehensive diagnostic information for troubleshooting and support cases.

**Note:** Node descriptions and a nodes table are **always collected** in all support-bundle modes (operator, cluster, client, csi, k8s, all). Use `--node-selector` flag to filter which nodes to collect.

### `support-bundle operator`

**Purpose:** Collects operator-related diagnostic data.

**Collected Data:**
- Operator controller manager logs (current + previous if restarted)
- Node-agent pod logs from all nodes (current + previous if restarted)
- Pod descriptions
- WekaPolicy resources
- Jobs created by policies

**Usage:**
```bash
kubectl weka support-bundle operator [flags]
```

**Common Flags** (all support-bundle commands):
- `--case-id <string>` – Case ID (Salesforce/Jira) to include in bundle name
- `-o, --output <dir>` – Output directory for bundle archive (default: current directory)
- `--include-sensitive-data` – Include sensitive data like Secrets (**⚠️ INSECURE**)
- `--debug` – Enable debug output showing collection progress

**Example:**
```bash
kubectl weka support-bundle operator --case-id SF-12345 -o /tmp
```

**Output:**
```
Support Bundle Collection Started
Bundle Name: weka-support-bundle-SF-12345-20260304-170001Z

Collecting support bundle data...
Running collector: Operator Logs
  Will collect:
    - Operator controller manager logs
    - Node-agent pod logs
    - Pod descriptions
  
✓ Collected logs from controller-manager (current: 45KB)
✓ Collected logs from 129 node-agent pods
✓ Collected 130 pod descriptions

Running collector: Operator Resources
  ✓ Collected 5 WekaPolicy resources
  ✓ Collected 3 related Jobs

Collection complete: 2 succeeded, 0 partial, 0 failed
✓ Support bundle created: weka-support-bundle-SF-12345-20260304-170001Z.tar.gz
```

---

### `support-bundle cluster`

**Purpose:** Collects cluster-related diagnostic data.

**Collected Data:**
- WekaCluster resource YAML
- WekaContainer resources and logs (current + previous)
- Pod descriptions
- Cluster instances output (`get cluster-instances`)

**Usage:**
```bash
kubectl weka support-bundle cluster [CLUSTER_NAME] [flags]
```

**Flags:**
- `-n, --namespace <string>` – Namespace
- `-A, --all-namespaces` – Collect from all namespaces
- Standard support-bundle flags

**Examples:**
```bash
# Collect all clusters in current namespace
kubectl weka support-bundle cluster

# Collect specific cluster
kubectl weka support-bundle cluster weka01

# Collect all clusters across all namespaces
kubectl weka support-bundle cluster -A
```

---

### `support-bundle client`

**Purpose:** Collects client-related diagnostic data.

**Collected Data:**
- WekaClient resource YAML
- WekaContainer resources and logs
- Pod descriptions
- Client instances output (`get client-instances`)

**Usage:**
```bash
kubectl weka support-bundle client [CLIENT_NAME] [flags]
```

---

### `support-bundle csi`

**Purpose:** Collects comprehensive CSI driver diagnostic data.

**Collected Data:**
- CSI drivers list and deployment information
- CSI instances (pod status, restart counts, health)
- Unhealthy CSI instances (wide view with restart details)
- Pod logs (current and previous) for all CSI pods organized by driver/role
- CSI secrets (with validation of required parameters)
- Storage classes using WEKA CSI drivers
- Persistent volumes (CSI driver references)
- Persistent volume claims (CSI driver references)

**Usage:**
```bash
kubectl weka support-bundle csi [flags]
```

**Flags:**
- `--case-id <ID>` – Case ID (Salesforce/Jira) to include in bundle name
- `-o, --output <dir>` – Output directory for the support bundle archive
- `-n, --namespace <string>` – Kubernetes namespace filter
- `-A, --all-namespaces` – Collect from all namespaces
- `--include-sensitive-data` – Include unredacted CSI secrets (⚠️ INSECURE)
- `--node-selector <label>=<value>` – Filter nodes for node descriptions
- `--debug` – Enable debug output

**Example:**
```bash
kubectl weka support-bundle csi --case-id SF-12345 -o /tmp
```

**Bundle Contents:**
```
weka-support-bundle-SF-12345-20260310-170001Z/
├── csi/
│   ├── csi-drivers.txt                    # CSI driver deployment info
│   ├── csi-instances.txt                  # All CSI pod instances
│   ├── csi-instances-unhealthy.txt        # Only pods with restart issues
│   ├── logs/                              # Pod logs organized by driver
│   │   ├── weka.io/
│   │   │   ├── controller/
│   │   │   │   └── pod-name/
│   │   │   │       ├── container1.log
│   │   │   │       └── container1.previous.log
│   │   │   └── node/
│   │   └── weka-csi.io/...
│   ├── secrets/                           # CSI secrets with validation
│   │   ├── weka.io/
│   │   │   └── storage-class-name/
│   │   │       └── secret-name/
│   │   │           └── secret.txt
│   │   └── errors.txt                     # Validation errors
│   ├── storage-classes.txt                # Storage classes using WEKA CSI
│   ├── persistent-volumes.txt             # PVs with CSI references
│   └── persistent-volume-claims.txt       # PVCs with CSI references
├── nodes/                                 # Node descriptions
│   └── nodes.txt                          # Nodes table
└── support-bundle.log                     # Collection log file
```



### `support-bundle k8s`

**Purpose:** Collects Kubernetes cluster preflight check results.

**Collected Data:**
- Complete output of `preflight cluster` checks
- Complete output of `preflight nodes` checks

**Usage:**
```bash
kubectl weka support-bundle k8s [flags]
```

**Flags:**
- `--node-selector <label>=<value>` – Filter nodes for preflight checks

---

### `support-bundle all`

**Purpose:** Collects ALL diagnostic data (umbrella command).

**Collected Data:**
- Operator logs and resources
- All clusters
- All clients  
- CSI components
- K8s preflight results

**Usage:**
```bash
kubectl weka support-bundle all [flags]
```

**Example:**
```bash
kubectl weka support-bundle all --case-id SF-12345 --debug
```

---

## Bundle Structure

All support bundles create a `.tar.gz` archive with the following structure:

```
weka-support-bundle-[case-id-]YYYYMMDD-HHMMSSZ.tar.gz
├── collection.log                    # Full collection log
├── nodes/                            # Always collected (all modes)
│   ├── nodes-table.txt               # kubectl weka get nodes output
│   ├── node-1_describe.yaml          # Individual node descriptions
│   ├── node-2_describe.yaml
│   └── ...
├── operator/
│   ├── logs/
│   │   ├── controller-manager_manager.log
│   │   ├── controller-manager_manager.previous.log
│   │   ├── node-agent-xxx_node-agent.log
│   │   └── ...
│   ├── pods/
│   │   ├── controller-manager_describe.yaml
│   │   └── ...
│   └── resources/
│       ├── WekaPolicy_default_policy1.yaml
│       └── ...
├── clusters/
│   ├── weka01/
│   │   ├── WekaCluster_default_weka01.yaml
│   │   ├── cluster-instances-weka01.txt
│   │   ├── containers/
│   │   ├── logs/
│   │   └── pods/
│   └── ...
├── clients/
│   ├── weka01-clients/
│   │   ├── WekaClient_default_weka01-clients.yaml
│   │   ├── client-instances-weka01-clients.txt
│   │   ├── containers/
│   │   ├── logs/
│   │   └── pods/
│   └── ...
├── csi/
│   └── ...
├── k8s/
│   ├── cluster-preflight.log
│   └── nodes-preflight.log
└── ...
```

**Key Points:**
- **nodes/** directory is **always collected** in all support bundle modes
- **nodes/nodes-table.txt** contains the same output as `kubectl weka get nodes` for quick reference
- **nodes/node-X_describe.yaml** contains full YAML descriptions for each node
- Node filtering with `--node-selector` applies to the nodes collection as well

---
This command performs cluster-level checks to ensure the Kubernetes environment is suitable for WEKA installation.
```bash
kubectl weka preflight cluster 
```

##### Checks include:

- Kubernetes version compatibility
- Managed cluster detection (ROSA / OpenShift)
- CNI presence
- Permissions to deploy Helm charts
- Cluster-level configuration requirements

##### Example output:
```bash
Validating Kubernetes version is 1.24+... PASS
Validating cluster is not ROSA / managed OpenShift... PASS
Validating permissions for Helm install (cluster-scope)... PASS
Validating CNI is configured... PASS
Validating cpu policy set to static... PASS
```

#### Node preflight
This command will create a temporary pod on each node to perform the checks.
Nodes that are `NotReady` are automatically skipped.
A node selector may be provided to limit the checks to specific nodes.

```bash
kubectl weka preflight nodes [NODE...] [flags]
```

##### Flags:
- `--node-selector <label>=<value>` – Label selector to filter nodes (e.g., if only part of nodes are targeted for WEKA)
- `--summary-only` – Only print summary (no per-node details)
- `--failed-only` – Only show failed nodes
- `--fail-fast` – Stop on first failed node
- `--weka-dir-min-fail <GB>` – Minimum GB for weka directory to FAIL (default: 100)
- `--weka-dir-min-warn <GB>` – Minimum GB for weka directory to WARN (default: 300)

##### Examples:
```bash
# Check all nodes
kubectl weka preflight nodes

# Check only nodes with specific label
kubectl weka preflight nodes --node-selector role=storage

# Check specific nodes by name
kubectl weka preflight nodes node1 node2 node3

# Only show summary (no per-node details)
kubectl weka preflight nodes --summary-only

# Only show failed nodes
kubectl weka preflight nodes --failed-only

# Custom weka directory thresholds (stricter requirements)
kubectl weka preflight nodes --weka-dir-min-fail=200 --weka-dir-min-warn=500
```

##### Checks include:
- OS and kernel (Ubuntu required)
- Hugepages configuration and availability
- Free memory and hugepages thresholds
- Weka directory space (configurable thresholds: FAIL < 100GB, WARN < 300GB by default)
- Filesystem layout (/opt/k8s-weka or /root/k8s-weka for RHCOS)
- XFS availability (mkfs.xfs)
- No existing WEKA client installation
- Mellanox NIC presence, speed, and bonding (LACP validation)
- Hardware introspection

##### Example output:
```bash
Performing preflight verification for Kubernetes nodes to host WEKA
Checking total number of nodes... PASS [12]

Validating node eligibility:
  srv-10000358: PASS
     os: PASS [Ubuntu 24.04.3 LTS]
     kernel: PASS [6.8.0-41-generic]
     hugepages: PASS [set=120000Mi allocatable=120000Mi]
     mem_free: PASS [2989Gi free]
     mellanox_nic: WARN [no Mellanox NICs detected — UDP mode only]
. . .
Summary:
  Eligible nodes:      10
  Nodes skipped:       0
  Nodes checked:       10
  Nodes passed:        8
  Nodes warned:        0
  Nodes failed:        2
  Failed nodes:        srv-10000351, srv-10000352
  Unique OSes:         1
  Unique Kernels:      1
```

### Inspect WEKA Instances
#### Inspecting WEKA Client Instances
WEKA clients are defined by WekaClient custom resources, which spawn per-node WekaContainer instances.
The target nodes are selected via node selectors defined in the WekaClient spec.
The command displays WEKA client instances and status (derived from WekaClient configuration)

```bash
kubectl weka get client-instances [-n <namespace>]
```

Optionally filter by a specific WekaClient:
```bash
kubectl weka get client-instances weka01-clients -n default
```

##### Displayed information includes:
- WekaClient name
- Node
- WekaContainer/Pod name
- WekaContainer status
- Pod status
- WEKA cluster join status
- WEKA Container ID
- Management IP(s)
- Number of active mounts
- CPU usage
##### Example output:
```bash
WEKACLIENT      NODE          NAMESPACE  WEKACONTAINER                WC_STATUS  POD      JOINED  CONTAINER_ID  MGMT_IP         ACTIVE_MOUNTS  CPU_UTIL
weka01-clients  srv-10000332  default    weka01-clients-srv-10000332  Running    Running  True    20            10.240.201.118  11             0.00
weka01-clients  srv-10000338  default    weka01-clients-srv-10000338  Running    Running  True    23            10.240.201.117  4              0.00
```
#### Inspecting WEKA Cluster Instances
WEKA clusters dynamically spawn multiple WekaContainer instances per role
(compute, drive, etc.), potentially dozens or hundreds across the cluster.
```bash
kubectl weka get cluster-instances [WEKACLUSTER] [-n <namespace>]
```

##### Output includes:
- WekaCluster name
- Namespace
- Node
- WekaContainer name
- WekaContainer status
- Pod status
- Management IP
- Container ID

Wide output adds:
- Age
- CPU utilization

##### Example output:
```bash
WEKACLUSTER  NAMESPACE  NODE          WEKACONTAINER                                        WC_STATUS      POD      MGMT_IP         CONTAINER_ID
weka01       default    <unknown>     weka01-drive-82e18ccc-5309-48f6-b095-b2e5701fdb6c    PodNotRunning  Pending  <none>
weka01       default    srv-10000351  weka01-compute-c6706d47-aef4-41da-a1ed-404b85754f97  Running        Running  10.240.201.105  13
weka01       default    srv-10000352  weka01-drive-e3fadbce-89ba-48a5-ac3f-7f160331439d    Running        Running  10.240.201.106  3
...
```
### Stream logs from the WEKA operator controller manager:
```bash
kubectl weka logs operator [-n <namespace>] [--follow] [--tail=<lines>] [--since=<duration>]
```

##### Options:

- `-n` / `--namespace` <string>  – override operator namespace (default: weka-operator-system)
- `-f` / `--follow` – follow logs
- `--tail` – number of lines to show from the end of the logs
- `--since` – relative time (e.g. 5m, 1h)

#### Examples:
```bash
kubectl weka logs operator -f
kubectl weka logs operator --tail=200
kubectl weka logs operator --since=10m
```
ANSI colors are preserved.

### Plan WEKA Cluster Deployment

The `plan` command analyzes a WekaCluster YAML specification file and provides detailed resource planning, helping you understand the infrastructure requirements before deployment.

#### Plan cluster resources
```bash
kubectl weka plan cluster <file.yaml> [--no-headers]
```

##### Description:
This command calculates resource requirements for each container type (Compute, Drive, S3, NFS, Envoy) based on your cluster specification and determines the minimum number of nodes needed.

##### Options:
- `--no-headers` – Don't print table headers (useful for scripting)

##### Features:
- **Resource Calculations** – Calculates CPU cores, memory, and hugepages for each container type
- **Drive Validation** – Verifies sufficient NVME drives available (with cluster access)
- **Node Requirements** – Shows minimum nodes needed with 10% spare capacity and fault tolerance recommendation
- **Offline Support** – Works without cluster access (skip drive validation)
- **Anti-affinity Aware** – Respects container placement rules (same role on different nodes)

##### Example usage:
```bash
# Analyze a cluster specification
kubectl weka plan cluster cluster.yaml

# Output without headers (for scripting)
kubectl weka plan cluster cluster.yaml --no-headers
```

##### Example cluster specification (cluster.yaml):
```yaml
apiVersion: weka.weka.io/v1alpha1
kind: WekaCluster
metadata:
  name: weka01
  namespace: default
spec:
  cpuPolicy: auto
  dynamicTemplate:
    computeContainers: 8
    computeCores: 12
    driveContainers: 8
    driveCores: 4
    numDrives: 4
    s3Containers: 2
    s3Cores: 4
  image: quay.io/weka.io/weka-in-container:4.4.10.200
  template: dynamic
```

##### Example output:
```
=== Container Resource Requirements ===
┌────────────────┬───────┬───────────────────┬───────────────────────┬────────────────────┐
│ Container Type │ Count │ Cores/Container   │ Hugepages/Container   │ Memory/Container   │
├────────────────┼───────┼───────────────────┼───────────────────────┼────────────────────┤
│ Compute        │     8 │                25 │             36200 MiB │          69100 MiB │
│ Drive          │     8 │                 9 │              6400 MiB │          22800 MiB │
│ S3             │     2 │                 9 │              5800 MiB │          31450 MiB │
│ Envoy (S3)     │     2 │                 1 │                  0 MiB │           1024 MiB │
└────────────────┴───────┴───────────────────┴───────────────────────┴────────────────────┘

=== Node Requirements (with 10% spare) ===
┌────────────────────────┬───────────┬─────────────┬──────────────────┬────────────────┬──────────────────────────────────────────────────┐
│ Purpose                │ Min Nodes │ Cores/Node  │ Hugepages/Node   │ Memory/Node    │ Description                                      │
├────────────────────────┼───────────┼─────────────┼──────────────────┼────────────────┼──────────────────────────────────────────────────┤
│ Backend (Compute+Drive)│         8 │          37 │        46860 MiB │      95810 MiB │ To accommodate 8 compute and 8 drive containers  │
│ Frontend (S3/NFS)      │         2 │          12 │         6380 MiB │      36949 MiB │ To accommodate 2 S3+Envoy containers             │
└────────────────────────┴───────────┴─────────────┴──────────────────┴────────────────┴──────────────────────────────────────────────────┘

💡 Recommendation: At least 1 more node of the required capacity is recommended to provide fault tolerance.
```

##### Resource Calculation Details:

**Compute Containers:**
- Hugepages: 3000Mi per core + 200Mi overhead (or explicit override)
- Cores: HT-aware (auto/dedicated_ht: 2×cores+extra+1, manual/shared: cores+extra+1)
- Memory: 2700 + (800+4400)×cores + 4000 + additional

**Drive Containers:**
- Hugepages: 1400Mi per core + 200Mi per drive (or 1000Mi per core if no drives)
- Cores: Same as Compute
- Memory: 4000 + (800+2200)×cores + 700×numDrives + 4000 + additional

**S3 Containers:**
- Hugepages: 1400Mi per core + 200Mi overhead
- Cores: Same as Compute
- Memory: 16000 + 2450 + (2850+200)×cores + 450 + additional

**NFS Containers:**
- Hugepages: 1400Mi per core + 200Mi overhead
- Cores: Same as Compute
- Memory: 16000 + 2450 + (2850+200)×cores + 450 + additional

**Envoy Containers:** (paired with S3)
- Hugepages: 0
- Cores: 1
- Memory: 1024 + additional

##### Container Placement Rules:
- Backend nodes can co-locate Compute + Drive containers
- Frontend nodes keep S3, NFS, and Envoy containers separate (one of each role per node)
- Each container type can appear only once per node
- Different types can share nodes

## Versioning and Releases

Versioning is automated using [Release Please](https://github.com/googleapis/release-please).

Builds are produced using [GoReleaser](https://goreleaser.com/).

### Supported Platforms

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

### Release Artifacts

- tar.gz archives (for Krew)
- Raw binaries
- Checksums and signatures

---

## Development

### Requirements

- **Go 1.22+**
- Access to a Kubernetes cluster with:
  - WEKA CRDs installed
  - WEKA Operator running (for operator tests)
  - WEKA Cluster/Client resources (for integration tests)

### Build Locally

```bash
git clone https://github.com/weka/kubectl-weka.git
cd kubectl-weka
go build -o kubectl-weka .
```

### Run Tests

```bash
# Unit tests only
go test -short ./...

# All tests (requires cluster access)
go test ./...
```

### Development Workflow

1. Create a feature branch
2. Make changes following [Conventional Commits](https://www.conventionalcommits.org/)
3. Run tests: `go test ./...`
4. Build: `go build -o kubectl-weka .`
5. Test locally: `./kubectl-weka <command>`
6. Create PR with descriptive title

### Adding New Features

See the [Developer Guide](DEVELOPER_GUIDE.md) for detailed instructions on:
- Adding new preflight checks
- Adding plan validations
- Adding support bundle collectors
- Creating new commands

---

## CI/CD

### Automated Builds on Pull Requests

Every pull request triggers automatic builds across 7 platform/architecture combinations:

**Platforms:**
- Linux: x86_64, ARM64, ARMv7
- macOS: x86_64, ARM64 (Apple Silicon)
- Windows: x86_64, ARM64

**What Happens:**
1. Go 1.25.0 is set up with module caching
2. Version is calculated from git state
3. Binary is compiled for each platform
4. Artifacts are uploaded to GitHub Actions for 30 days
5. Build summary is generated

**Getting Artifacts:**
1. Go to your Pull Request
2. Click the **Checks** tab
3. Select the build job for your platform
4. Scroll to **Artifacts** section
5. Download your platform's binary

### Automated Release Builds

When a release is published, binaries are automatically built and attached for all platforms:

**What Happens:**
1. All 7 platform combinations are built
2. Each binary is named with version, OS, and architecture
3. All binaries are attached to the GitHub Release

**Release Assets Named:**
- `kubectl-weka-v1.0.0-linux-amd64`
- `kubectl-weka-v1.0.0-linux-arm64`
- `kubectl-weka-v1.0.0-linux-arm`
- `kubectl-weka-v1.0.0-darwin-amd64`
- `kubectl-weka-v1.0.0-darwin-arm64`
- `kubectl-weka-v1.0.0-windows-amd64.exe`
- `kubectl-weka-v1.0.0-windows-arm64.exe`

**Download Release Binaries:**

Visit the [Releases](https://github.com/weka/kubectl-weka/releases) page and download the binary for your platform.

Example:
```bash
# Linux x86_64
curl -LO https://github.com/weka/kubectl-weka/releases/download/v1.0.0/kubectl-weka-v1.0.0-linux-amd64
chmod +x kubectl-weka-v1.0.0-linux-amd64
sudo mv kubectl-weka-v1.0.0-linux-amd64 /usr/local/bin/kubectl-weka

# macOS ARM64
curl -LO https://github.com/weka/kubectl-weka/releases/download/v1.0.0/kubectl-weka-v1.0.0-darwin-arm64
chmod +x kubectl-weka-v1.0.0-darwin-arm64
sudo mv kubectl-weka-v1.0.0-darwin-arm64 /usr/local/bin/kubectl-weka

# Windows x86_64
curl -LO https://github.com/weka/kubectl-weka/releases/download/v1.0.0/kubectl-weka-v1.0.0-windows-amd64.exe
# Place in PATH or use directly
```

### Automatic Release Management

The repository uses [release-please](https://github.com/googleapis/release-please) for automated versioning:

1. Commits following [Conventional Commits](https://www.conventionalcommits.org/) are analyzed
2. A release PR is automatically created with:
   - Updated CHANGELOG.md
   - Version bump (major/minor/patch)
3. When merged, a Git tag is created automatically
4. Release build workflow is triggered

---

## Contributing

Contributions are welcome! Please follow these guidelines:

### Code Style

- Use `go fmt` for formatting
- Follow Go best practices
- Add comments for exported functions
- Keep functions focused and small

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` – New features
- `fix:` – Bug fixes
- `docs:` – Documentation changes
- `refactor:` – Code refactoring
- `test:` – Test additions/changes
- `chore:` – Maintenance tasks

**Examples:**
```
feat: add support-bundle cluster command
fix: handle missing namespace in get client-instances
docs: update README with support-bundle examples
refactor: extract common validation logic
```

### Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add/update tests
5. Update documentation
6. Ensure all tests pass
7. Submit PR with clear description

### Design Principles

- **kubectl-native** – Commands should feel like built-in kubectl commands
- **Kubernetes RBAC** – Respect cluster permissions
- **Version agnostic** – Avoid hard dependencies on specific WEKA versions
- **Clear output** – Human-readable with optional machine formats
- **Error handling** – Provide actionable error messages

---

## Troubleshooting

### Common Issues

**"No WekaCluster resources found"**
- Check namespace with `-n <namespace>` or `-A` for all namespaces
- Verify WEKA CRDs are installed: `kubectl get crds | grep weka`

**"Failed to create host-check pod"**
- Check RBAC permissions
- Verify nodes are Ready: `kubectl get nodes`
- Check for pod security policies blocking privileged pods

**Preflight checks timeout**
- NotReady nodes are automatically skipped
- Use `--node-selector` to target specific nodes
- Check network connectivity to nodes

**Support bundle collection fails**
- Verify sufficient disk space in output directory
- Check RBAC permissions for reading pods/logs
- Use `--debug` flag to see detailed progress

### Getting Help

- **Documentation**: [README.md](README.md), [Developer Guide](DEVELOPER_GUIDE.md)
- **Issues**: [GitHub Issues](https://github.com/weka/kubectl-weka/issues)
- **Examples**: Check `cmd/` directory for implementation examples

---

## License

Apache License 2.0

See [LICENSE](LICENSE.md) for details.

---

## Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) – CLI framework
- [Controller-Runtime](https://github.com/kubernetes-sigs/controller-runtime) – Kubernetes client
- [go-pretty](https://github.com/jedib0t/go-pretty) – Table formatting
- [color](https://github.com/fatih/color) – Terminal colors

---

**Questions?** Open an issue or check the [Developer Guide](DEVELOPER_GUIDE.md).


