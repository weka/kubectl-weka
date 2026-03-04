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
  - [Plan Commands](#plan-commands)
  - [Logs Commands](#logs-commands)
  - [Support Bundle Commands](#support-bundle-commands)
- [Developer Guide](DEVELOPER_GUIDE.md)
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
| `preflight` | Pre-deployment validation | `cluster`, `nodes` |
| `get` | Inspect WEKA resources | `cluster-instances`, `client-instances`, `nodes`, `policies` |
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

**Purpose:** Lists Kubernetes nodes with WEKA-relevant labels and resource information.

**Usage:**
```bash
kubectl weka get nodes [flags]
```

**Flags:**
- `--node-selector <label>=<value>` – Filter nodes by label
- `--wide` – Show additional resource details

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

**Purpose:** Collects CSI driver diagnostic data.

**Usage:**
```bash
kubectl weka support-bundle csi [flags]
```

---

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
│   └── ...
├── k8s/
│   ├── cluster-preflight.log
│   └── nodes-preflight.log
└── ...
```

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


