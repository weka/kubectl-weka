# kubectl-weka

`kubectl-weka` is a kubectl plugin that provides operational visibility, preflight validation, and diagnostics for **WEKA deployments on Kubernetes**.

It extends `kubectl` with WEKA-specific commands for:
- Kubernetes and node preflight checks
- Inspecting WEKA client and cluster instances
- Viewing WEKA operator logs
- Operational analytics beyond standard Kubernetes primitives

The plugin is designed to feel **kubectl-native** and integrates cleanly with Kubernetes RBAC, Krew, and CI/CD workflows.

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
### Preflight Checks

Preflight commands validate that a Kubernetes cluster and its nodes meet
the requirements before installing WEKA.

#### Kubernetes cluster preflight
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

Versioning is automated using Release Please

Builds are produced using GoReleaser

#### Supported platforms:

- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64

#### Releases include:

- tar.gz archives (for Krew)
- Raw binaries

## Development Requirements
- Go 1.22+
- Access to a Kubernetes cluster with WEKA CRDs, WEKA Operator, WEKA Cluster & Client installed
 
#### Build locally
```
go build -o kubectl-weka .
```

#### Run tests
```
go test ./...
```

## Contributing

Contributions are welcome.

Please:

Use Conventional Commits (feat:, fix:, chore:)

Keep commands kubectl-native in behavior and UX

Avoid hard dependencies on specific WEKA versions unless required

## License

Apache License 2.0
