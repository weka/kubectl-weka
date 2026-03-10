# kubectl-weka Quick Reference

## Installation

```bash
kubectl krew install weka
```

## Command Cheat Sheet

### Preflight Validation

```bash
# Cluster checks
kubectl weka preflight cluster

# Node checks (all nodes)
kubectl weka preflight nodes

# Node checks (filtered)
kubectl weka preflight nodes --node-selector role=storage

# Show summary only
kubectl weka preflight nodes --summary-only

# Show only failures
kubectl weka preflight nodes --failed-only
```

### Get Resources

```bash
# List cluster instances
kubectl weka get cluster-instances
kubectl weka get cluster-instances weka01
kubectl weka get cluster-instances -A --wide

# List client instances
kubectl weka get client-instances
kubectl weka get client-instances weka01-clients
kubectl weka get client-instances -A

# List nodes
kubectl weka get nodes
kubectl weka get nodes --node-selector role=storage

# List CSI drivers
kubectl weka get csi-drivers
kubectl weka get csi-drivers csi.weka.io
kubectl weka get csi-drivers --wide
kubectl weka get csi-drivers --only-helm
kubectl weka get csi-drivers csi.weka.io --wide

# List policies
kubectl weka get policies -A
```

### Plan Deployment

```bash
# Plan cluster
kubectl weka plan cluster cluster.yaml

# Plan client
kubectl weka plan client client.yaml

# Plan converged (cluster + client)
kubectl weka plan converged cluster.yaml client.yaml
```

### Stream Logs

```bash
# Follow operator logs
kubectl weka logs operator -f

# Show last 100 lines
kubectl weka logs operator --tail=100

# Show logs from last 5 minutes
kubectl weka logs operator --since=5m

# Show previous logs (if restarted)
kubectl weka logs operator --previous
```

### Collect Support Bundles

```bash
# Operator diagnostics
kubectl weka support-bundle operator --case-id SF-12345

# Cluster diagnostics
kubectl weka support-bundle cluster weka01

# Client diagnostics
kubectl weka support-bundle client weka01-clients

# All clusters/clients in namespace
kubectl weka support-bundle cluster -n default
kubectl weka support-bundle client -A

# K8s preflight results
kubectl weka support-bundle k8s

# Everything
kubectl weka support-bundle all --case-id SF-12345 --debug
```

## Common Flags

| Flag | Description | Commands |
|------|-------------|----------|
| `-n, --namespace` | Specify namespace | get, support-bundle |
| `-A, --all-namespaces` | All namespaces | get, support-bundle |
| `--wide` | Additional columns | get |
| `--no-headers` | Skip headers | get, plan |
| `-f, --follow` | Stream logs | logs |
| `--tail` | Last N lines | logs |
| `--since` | Time duration | logs |
| `--case-id` | Support case ID | support-bundle |
| `-o, --output` | Output directory | support-bundle |
| `--debug` | Debug logging | support-bundle |
| `--node-selector` | Filter nodes | preflight, get |
| `--summary-only` | Summary only | preflight |
| `--failed-only` | Failed only | preflight |
| `--fail-fast` | Stop on error | preflight, plan |

## Quick Troubleshooting

### No resources found
```bash
# Check namespace
kubectl weka get cluster-instances -A

# Verify CRDs installed
kubectl get crds | grep weka
```

### Preflight timeout
```bash
# Check node status
kubectl get nodes

# Use node selector
kubectl weka preflight nodes --node-selector role=storage
```

### Support bundle fails
```bash
# Enable debug
kubectl weka support-bundle all --debug

# Check permissions
kubectl auth can-i list pods --all-namespaces
```

## Resource Formulas (Plan)

### Compute Containers
- Cores: `(HT ? 2×cores : cores) + extra + 1`
- HP: `3000Mi × cores + 200Mi`
- Mem: `2700 + (800+4400)×cores + 4000 + additional`

### Drive Containers
- Cores: Same as compute
- HP: `1400Mi × cores + 200Mi × drives`
- Mem: `4000 + (800+2200)×cores + 700×drives + 4000 + additional`

### S3/NFS Containers
- Cores: Same as compute
- HP: `1400Mi × cores + 200Mi`
- Mem: `16000 + 2450 + (2850+200)×cores + 450 + additional`

### Envoy Containers
- Cores: `1`
- HP: `0`
- Mem: `1024 + additional`

## Output Formats

### Table (default)
```bash
kubectl weka get cluster-instances
```

### Wide (extra columns)
```bash
kubectl weka get cluster-instances --wide
```

### No Headers (scripting)
```bash
kubectl weka get cluster-instances --no-headers
```

## Common Workflows

### Pre-Installation Check
```bash
# 1. Validate cluster
kubectl weka preflight cluster

# 2. Validate nodes
kubectl weka preflight nodes --node-selector role=storage

# 3. Plan deployment
kubectl weka plan cluster cluster.yaml
```

### Troubleshooting Deployment
```bash
# 1. Check instances
kubectl weka get cluster-instances weka01
kubectl weka get client-instances weka01-clients

# 2. Check operator logs
kubectl weka logs operator -f --tail=100

# 3. Collect support bundle
kubectl weka support-bundle all --case-id SF-12345
```

### Monitoring
```bash
# Watch cluster instances
watch kubectl weka get cluster-instances

# Follow operator logs
kubectl weka logs operator -f

# Check resource allocation
kubectl weka get nodes --wide
```

## Support Bundle Contents

```
weka-support-bundle-<case-id>-<timestamp>.tar.gz
├── collection.log              # Collection log
├── operator/                   # Operator data
│   ├── logs/                   # Controller & node-agent logs
│   ├── pods/                   # Pod descriptions
│   └── resources/              # WekaPolicy, Jobs
├── clusters/                   # Cluster data
│   └── <cluster-name>/
│       ├── WekaCluster YAML
│       ├── cluster-instances.txt
│       ├── containers/         # WekaContainer YAMLs
│       ├── logs/               # Container logs
│       └── pods/               # Pod descriptions
├── clients/                    # Client data (similar structure)
├── csi/                        # CSI driver data
└── k8s/                        # Kubernetes preflight results
```

## Exit Codes

- `0` - Success
- `1` - General error
- `2` - Validation failed (preflight/plan)

## Environment Variables

None required. Uses standard kubectl configuration:
- `KUBECONFIG` - Path to kubeconfig file
- Standard kubectl environment variables

## Further Reading

- [README.md](README.md) - Complete command reference
- [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) - Extension guide
- [GitHub Issues](https://github.com/weka/kubectl-weka/issues) - Bug reports

---

**Pro Tip:** Use `kubectl weka help <command>` for detailed help on any command.

