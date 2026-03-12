# Known Issues and Limitations

This document lists known limitations, constraints, and issues with the kubectl-weka hostcheck system.

## Cloud Instance Detection

### GCP Detection Limitations

**Issue**: GCP cloud provider detection does not work in CNI pod networks.

**Root Cause**: 
- GCP metadata service is only accessible via `metadata.google.internal` DNS hostname
- DNS resolution and network access to metadata endpoint requires host network namespace
- CNI networks are isolated from host network, preventing metadata access

**Workaround**: 
- Deploy pod with `hostNetwork: true` flag
- On OpenShift, this requires additional SCC and RBAC configuration
- On vanilla Kubernetes, add `hostNetwork: true` to pod spec

**Impact**: 
- `cloud_info.provider` will not be populated for GCP when running in CNI network
- `is_cloud_instance` will still be `false` for GCP nodes in CNI networks
- AWS and Azure detection unaffected (filesystem-based detection)

**See Also**: 
- [HOSTNETWORK_IMPLICATIONS.md](HOSTNETWORK_IMPLICATIONS.md)
- [KUBECTL_WEKA_HOSTCHECK_STRATEGY.md](KUBECTL_WEKA_HOSTCHECK_STRATEGY.md)

---

### AWS Metadata Limitations

**Issue**: AWS instance metadata (region, AZ, instance type) requires IMDSv2 access.

**Root Cause**:
- IMDSv2 requires token from `169.254.169.254` link-local address
- Link-local addresses are not routable across network namespaces
- CNI networks cannot reach host's `169.254.169.254`

**Workaround**:
- Use `hostNetwork: true` to access IMDSv2
- Without IMDSv2, AWS provider is detected but metadata is empty
- AWS detection itself (via `/sys/hypervisor/uuid`) still works

**Impact**:
- AWS: Provider detected ✅, Region/AZ/InstanceType empty ⚠️
- Severity: LOW (provider detection is the critical part)

**Mitigation**:
- For AWS/Azure primary deployments, you get the essential detection (provider name)
- Full metadata is optional/bonus information

---

### Azure Metadata Limitations

**Issue**: Same as AWS - metadata endpoint requires host network access.

**Root Cause**: 
- Azure metadata service at `169.254.169.254`
- Not accessible from CNI pod networks

**Workaround**: 
- Use `hostNetwork: true`

**Impact**:
- Azure: Provider detected ✅, Region/InstanceType empty ⚠️
- Severity: LOW (same as AWS)

---

## Server Hardware Detection

### DMI Data Availability

**Issue**: `/sys/class/dmi/id` may not have complete or accurate data on virtual machines.

**Root Cause**:
- Hypervisors may not fully populate DMI tables
- Some virtualization platforms provide minimal DMI info
- Container host filesystem may have incomplete DMI data

**Impact**:
- Some fields (vendor, model, serial) may be empty on VMs
- BIOS version may be unavailable on virtualized systems
- Works well on bare metal, inconsistent on VMs

**Affected Fields**:
- `server_info.vendor` - May be empty (VM) or generic (VirtualBox, VMware)
- `server_info.serial_number` - Often unavailable in VMs
- `server_info.system_uuid` - May be different on each boot (some VMs)

**Severity**: LOW (informational only, doesn't affect functionality)

---

### No dmidecode Fallback

**Issue**: Script only reads from `/sys/class/dmi/id`, does not use `dmidecode` command.

**Design Decision**: Intentional (requested by user).

**Impact**:
- More reliable in containers (no dependency on dmidecode binary)
- Some edge cases may not be detected
- Works on all Linux systems with sysfs

**Severity**: VERY LOW (sysfs is ubiquitous on modern Linux)

---

## Network Interface Detection

### VLAN Detection Limitations

**Issue**: VLAN detection relies on `/proc/net/vlan/config` or interface name parsing.

**Root Cause**:
- `/proc/net/vlan/config` may not exist on systems without VLAN support
- Falls back to interface name parsing (e.g., `eth0.100`)
- Heuristic-based parsing may miss edge cases

**Limitations**:
- Only detects standard VLAN naming (`eth0.100` or `eth0:100`)
- Does not detect VLANs with custom names
- May not work on older kernels without VLAN support

**Severity**: LOW (covers 99% of VLAN deployments)

---

### Bond Detection Dependencies

**Issue**: Bond detection depends on `/sys/class/net/{iface}/bonding/` directory structure.

**Root Cause**:
- Kernel module must be loaded for bonding support
- Requires sysfs to be mounted and accessible
- May not work on systems with minimal kernel configuration

**Impact**:
- Bonds detected correctly on systems with bonding support
- May not detect bonds on systems with bonding module not loaded
- Bond mode detection requires bonding module

**Severity**: LOW (bonding is standard on production systems)

---

### Bond Slave Resolution

**Issue**: Bond slaves are resolved by matching `BondSlaves` list with interface names.

**Root Cause**:
- Depends on accurate `bond_slaves` file in sysfs
- Requires parent NetworkInterfaces slice to be initialized

**Limitation**:
- `GetSlaves()` returns empty if parent pointers not initialized
- Must call `InitializeBondHierarchy()` after unmarshaling JSON
- Recursive bond hierarchies not supported (not a real use case)

**Severity**: MEDIUM (but mitigated by automatic initialization in validation modules)

---

## Cloud-Specific Limitations

### GCP No Filesystem Marker

**Issue**: GCP has no easy filesystem indicator (unlike AWS hypervisor UUID or Azure DMI).

**Impact**:
- Cannot reliably detect GCP without network metadata access
- Users on GCP need `hostNetwork: true` to be detected as cloud instance
- No workaround available

**Severity**: MEDIUM (affects GCP users only)

---

## Kubernetes/OpenShift Deployment

### OpenShift: hostNetwork Requires SCC + RBAC

**Issue**: Using `hostNetwork: true` on OpenShift requires SecurityContextConstraints.

**Root Cause**: 
- OpenShift default SCC is "restricted"
- Restricted SCC blocks `hostNetwork`, privileged containers, host mounts

**Required Configuration**:
- Create or bind to appropriate SCC (hostnetwork or privileged)
- Create RBAC ClusterRoleBinding to allow ServiceAccount to use SCC
- Annotate pod with SCC name

**Impact**:
- Cannot use `hostNetwork: true` on OpenShift without SCC setup
- Adds operational complexity for OpenShift deployments
- Vanilla Kubernetes unaffected (works immediately)

**Severity**: MEDIUM (well-documented workaround available)

**See Also**: [HOSTNETWORK_IMPLICATIONS.md](HOSTNETWORK_IMPLICATIONS.md)

---

### Pod Privilege Requirements

**Issue**: Pod must run privileged to access `/host` mount.

**Root Cause**:
- Accessing host filesystem requires elevated capabilities
- Standard containers cannot read arbitrary host paths

**Impact**:
- Pod cannot run with `securityContext.privileged: false`
- Requires elevated privileges even for read-only access
- May conflict with strict security policies

**Severity**: MEDIUM (elevated privileges required, but documented)

---

## Metadata Service Timeouts

### Network Timeout Sensitivity

**Issue**: Network metadata access uses 2-second timeout, may fail on slow networks.

**Implementation**: All curl requests use `-m 2` timeout flag.

**Limitation**:
- Slow or high-latency metadata services may timeout
- User won't know if metadata was unavailable or slow
- Fails silently (returns empty metadata)

**Impact**:
- On high-latency networks, metadata may not be collected
- User has no visibility into why metadata is missing
- Provider detection still works (filesystem-based)

**Severity**: LOW (metadata is bonus information, provider detection works)

---

## Data Collection Limitations

### CPU Information Accuracy

**Issue**: CPU detection may be inaccurate in containerized environments.

**Root Cause**:
- Cgroup limits affect what `/proc/cpuinfo` reports
- Physical core detection is heuristic-based
- Different kernel versions report differently

**Impact**:
- `physical_cores` may be higher than actual available cores
- `logical_cores` reflects container limits, not host
- HTEnabled detection may be inaccurate

**Severity**: LOW (informational only)

---

### Memory Information in Containers

**Issue**: Memory information reflects cgroup limits, not host total.

**Root Cause**:
- Pod's cgroup limits what `/proc/meminfo` reports
- Does not show true host memory

**Impact**:
- `memory_bytes` is cgroup-limited, not host actual
- `free_memory_bytes` reflects container availability
- Not accurate representation of host resources

**Severity**: LOW (expected behavior for containerized apps)

---

### NVMe Drive Detection

**Issue**: NVMe detection depends on specific sysfs paths that may vary by kernel version.

**Root Cause**:
- Device serial/model paths may differ
- Some systems may not expose all device attributes
- Partition detection has edge cases

**Limitations**:
- Device serial may be empty if not exposed in sysfs
- Model name parsing may fail on some drives
- Partition detection uses heuristic naming scheme

**Severity**: LOW (detection works for vast majority of drives)

---

## Script Execution Environment

### Dependency: curl Required (Optional)

**Issue**: Network-based cloud detection requires `curl` binary.

**Root Cause**:
- Network metadata access needs HTTP client
- Fallback to network commands not implemented

**Impact**:
- If curl is not available, network metadata access skipped silently
- Filesystem-based detection (AWS/Azure) still works
- GCP detection fails completely (no filesystem marker)

**Severity**: LOW (curl is standard on modern systems)

---

### Dependency: bash Features

**Issue**: Script uses bash-specific features (not POSIX sh).

**Root Cause**:
- Shebang is `#!/bin/bash` not `#!/bin/sh`
- Some systems may have limited bash support

**Impact**:
- Requires bash to be available
- Won't work with dash or other shells
- Works on all modern Linux systems (bash is ubiquitous)

**Severity**: VERY LOW (bash standard on all production Linux)

---

## Known Workarounds

### For GCP Detection in CNI Networks
- Option 1: Add `hostNetwork: true` + SCC/RBAC (OpenShift)
- Option 2: Accept that GCP won't be detected in CNI network
- Option 3: Run separate privileged DaemonSet with hostNetwork

### For Incomplete Server Info on VMs
- Consider deploying on bare metal for complete hardware info
- VMs will still work, just with limited hardware details
- Information is informational only, doesn't affect functionality

### For AWS/Azure Metadata in CNI Networks
- Use AWS/Azure-specific mechanisms to inject metadata
- Provider detection still works (sufficient for most use cases)
- Full metadata only needed in specialized scenarios

### For Missing curl Command
- Pre-install curl in container image
- Accept that network metadata won't be collected
- Filesystem-based detection is primary method

---

## Unsupported Configurations

### Not Supported: VLANs with Custom Names
- Only standard VLAN naming detected (e.g., `eth0.100`)
- Custom VLAN interface names not detected as VLANs
- Will appear as regular Ethernet interfaces

### Not Supported: Recursive Bond Hierarchies
- Bonds on top of other bonds not tested
- Unlikely to occur in production
- Would require recursive GetSlaves() implementation

### Not Supported: Kubernetes <1.20
- Uses features from Kubernetes 1.20+
- May not work on older clusters
- Not tested on Kubernetes < 1.20

### Not Supported: Non-Linux Hosts
- Script is Linux-specific
- Uses `/sys/class/`, `/proc/`, etc.
- Will not work on Windows, macOS, or other Unix variants

---

## Future Improvements

### Potential Enhancements
- [ ] GCP filesystem-based detection (if marker becomes available)
- [ ] Better VLAN detection (track by ID in addition to name)
- [ ] DMI data validation (check for common placeholder values)
- [ ] Configurable metadata timeout (currently hardcoded 2 seconds)
- [ ] Verbose logging option (troubleshoot detection failures)
- [ ] Support for additional cloud providers (Alibaba, Oracle, etc.)
- [ ] Custom NIC capability extensions

---

## Reporting Issues

If you encounter issues not listed here:

1. Check if running with appropriate privileges (`privileged: true`)
2. Check if running with appropriate host mounts (`/host`, `/sys`, `/proc`)
3. Check pod logs for error messages
4. Verify kernel version supports required features
5. On OpenShift, verify SCC and RBAC configuration

For bugs, please include:
- Kubernetes/OpenShift version
- Node OS and kernel version
- Pod network configuration
- Cloud provider (if cloud-hosted)
- Relevant error messages or missing data

