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
kubectl weka preflight nodes [--node-selector <label>=<value>]
```
##### Checks include:
- OS and kernel
- Hugepages configuration and availability
- Free memory thresholds
- Filesystem layout (/opt/k8s-weka or /root/k8s-weka)
- XFS availability
- Mellanox NIC presence, speed, and bonding (LACP)
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
