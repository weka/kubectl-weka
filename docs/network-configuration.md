# Network Configuration Documentation

This document provides comprehensive information about how kubectl-weka handles network interface detection, validation, and speed/rate reporting for Ethernet and InfiniBand networks.

## Table of Contents

1. [Overview](#overview)
2. [Network Interface Types](#network-interface-types)
3. [Speed and Rate Metrics](#speed-and-rate-metrics)
4. [Network Validation](#network-validation)
5. [NIC Database](#nic-database)
6. [Hostcheck Data Collection](#hostcheck-data-collection)
7. [Troubleshooting](#troubleshooting)

## Overview

kubectl-weka properly distinguishes between **Ethernet networks** (using speed in Gbps/Mbps) and **InfiniBand networks** (using rate in GB/s). This separation ensures:

- ✅ Accurate capability detection
- ✅ Proper DPDK vs UDP mode determination
- ✅ Correct resource planning
- ✅ Accurate generation identification for InfiniBand

## Network Interface Types

### 1. Ethernet Interfaces

**Characteristics:**
- Detected from `/sys/class/net/*/speed`
- Speed stored in Mbps (multiply Gbps by 1000)
- Display format: "100Gbps", "400Gbps"
- Minimum speed for Weka: 10Gbps (10000 Mbps)

**Examples:**
- Intel X710 – 10Gbps (10000 Mbps)
- Mellanox ConnectX-4 – 100Gbps (100000 Mbps)
- Mellanox ConnectX-7 – 400Gbps (400000 Mbps)

### 2. InfiniBand Interfaces

**Characteristics:**
- Detected from `/sys/class/net/*/device/infiniband/*/ports/*/rate`
- Rate file contains format: "400 Gb/sec (4X NDR)"
- Rate stored in MB/s (convert using: Gbps × 1000 ÷ 8)
- Display format: "400GB/s 2xNDR", "100GB/s 2xHDR"
- Minimum rate for Weka: 10Gbps equivalent (1250 MB/s)

**Conversion Examples:**
```
8 Gbps InfiniBand → 1000 MB/s
100 Gbps InfiniBand → 12500 MB/s
200 Gbps InfiniBand → 25000 MB/s
400 Gbps InfiniBand → 50000 MB/s
```

### 3. Bond Interfaces

**Characteristics:**
- Type: "bond"
- Mode: Should be "802.3ad" (LACP) for Weka deployments
- Slaves: Must have exactly 2 slaves per Weka requirements
- Speed/Rate: Inherited from slave interfaces

**Detection:**
- Located at: `/sys/class/net/*/bonding/`
- Slave detection: `/sys/class/net/*/master` or similar
- Mode file: `/sys/class/net/*/bonding/mode`

### 4. VLAN Interfaces

**Characteristics:**
- Type: "vlan"
- Parent interface: Contains base interface name
- VLAN ID: Number from interface name (e.g., "eth0.100" → VLAN 100)
- Speed/Rate: Inherited from parent interface

**Detection:**
- Pattern: `<interface>.<vlan_id>`
- VLAN info: `/sys/class/net/*/dev_id`

## Speed and Rate Metrics

### Ethernet Speed (Mbps)

**Internal Storage:**
- Integer value in Mbps
- Example: 100000 for 100Gbps, 400000 for 400Gbps

**Display Conversion:**
```
Display = Speed_Mbps / 1000 = Speed in Gbps
Example: 100000 Mbps → 100Gbps
```

### InfiniBand Rate (MB/s)

**Internal Storage:**
- Integer value in MB/s (bytes per second)
- Example: 12500 for 100Gbps, 50000 for 400Gbps

**Conversion Formula:**
```
Rate_MB/s = Speed_Gbps × 1000 Mbps/Gbps ÷ 8 bits/byte
Rate_MB/s = Speed_Gbps × 125
```

**Display Conversion:**
```
Display_GB/s = Rate_MB/s / 1000
Display = "{GB/s}GB/s {Generation}"
Example: 50000 MB/s → "400GB/s 2xXDR"
```

### Generation Identification

InfiniBand generations are identified based on speed/rate:

| Generation | Gbps | MB/s | Display Suffix |
|-----------|------|------|----------------|
| SDR | 2 | 250 | 2xSDR |
| DDR | 4 | 500 | 2xDDR |
| QDR | 8 | 1000 | 2xQDR |
| FDR | 40 | 5000 | 2xFDR |
| EDR | 100 | 12500 | 2xEDR |
| HDR | 200 | 25000 | 2xHDR |
| NDR | 400 | 50000 | 2xNDR |
| XDR | 800+ | 100000+ | 2xXDR |

## Network Validation

### Hostcheck Network Validation

The `NetworkInterfacesModule` performs the following checks:

#### 1. Speed/Rate Threshold Check
```
Ethernet: MaxSpeed >= 10000 Mbps (10Gbps) → PASS
InfiniBand: MaxRate >= 1250 MB/s (10Gbps equivalent) → PASS
Below threshold → FAIL
```

#### 2. MTU Validation
```
Ethernet:
  ≥ 9000 → PASS
  1500 → WARN (UDP-only, no jumbo frames)

InfiniBand:
  ≥ 2048 → PASS
  < 2048 → WARN
```

#### 3. Interface Type Check
```
Supported: ethernet, infiniband, bond, vlan
Unsupported: loopback, virtual bridges (cni0, docker0, etc.)
```

#### 4. IP Address Check
```
Must have IP address in CIDR notation
/32 subnet mask → FAIL (host-only route)
```

#### 5. Bond Validation (if bond interface)
```
Mode must be: 802.3ad (LACP)
Slave count must be: 2
Missing either → FAIL
```

#### 6. DPDK/UDP Capability Check
```
Based on VendorModel (PCI ID):
  - Check NIC registry for capabilities
  - Determine DPDK vs UDP mode support
  - Identify single-NIC sharing support
```

### MTU Requirements

**Ethernet (Standard):**
- Minimum: 1500 bytes (standard Ethernet)
- Recommended: 9000 bytes (jumbo frames)
- Warning: Below 9000 bytes without good reason

**InfiniBand (Standard):**
- Default: 4096 bytes
- Minimum: 2048 bytes
- Typical: 4096 or 65520 bytes

## NIC Database

The NIC database (`pkg/device-support/network.go`) contains:

### Supported Vendors
- Mellanox (15b3) – Full support for ConnectX series
- Intel (8086) – Partial support for X540+ series
- Broadcom (14e4) – Limited support
- Hypervisor vendors – Virtio, VMware, Amazon, Google

### Capabilities Tracked

For each NIC model:
1. **Basic Info**
   - Vendor and Device ID (e.g., "15b3:1021")
   - Model name (e.g., "ConnectX-7")
   - Speed rating
   - Number of ports
   - Interface mode (Converged/Ethernet/InfiniBand)

2. **Weka Capabilities**
   - `SupportedByWekaDpdk` – Can use DPDK mode
   - `SupportedByWekaDpdkSingleNic` – Can share NIC across processes
   - `SupportedByWekaUdp` – Can use UDP fallback
   - `SupportedByWekaForLacpSameCard` – Can use LACP on same NIC
   - `SupportedByWekaForLacpDiffCards` – Can use LACP across NICs

### Adding New NICs to Database

To add a new NIC model to the database:

1. Obtain Vendor ID and Device ID (from `lspci -nn`)
2. Add entry to `NICRegistry` in network.go
3. Add capabilities to `NICCapabilityMap` in network.go
4. Test with hostcheck validation

**Example:**
```go
// In NICRegistry
"8086:1234": {
    VendorID: "8086",
    DeviceID: "1234",
    Vendor: "Intel",
    ShortModel: "NewModel",
    Model: "Full Model Name",
    Speed: "100Gb/s",
    NumPorts: 2,
    InterfaceMode: "Ethernet",
},

// In NICCapabilityMap
"8086:1234": {
    SupportedByWekaDpdk: true,
    SupportedByWekaDpdkSingleNic: false,
    SupportedByWekaUdp: true,
    SupportedByWekaForLacpSameCard: false,
    SupportedByWekaForLacpDiffCards: false,
},
```

## Hostcheck Data Collection

### Scripts and Utilities

The `cmd/resources/hostcheck.sh` script contains:

#### iface_speed()
```bash
iface_speed <interface_name>
# Returns: Ethernet speed in Mbps or 0 if non-Ethernet
# Usage: speed=$(iface_speed eth0)
```

**Implementation:**
- Reads from `/sys/class/net/<iface>/speed`
- Converts to Mbps (already in Mbps typically)
- Returns 0 for InfiniBand interfaces

#### iface_rate()
```bash
iface_rate <interface_name>
# Returns: InfiniBand rate in MB/s or 0 if non-InfiniBand
# Usage: rate=$(iface_rate ib0)
```

**Implementation:**
- Uses `find` to locate rate file under `/sys/class/net/<iface>/device/infiniband/`
- Parses rate file format: "400 Gb/sec (4X NDR)"
- Converts Gbps to MB/s using: `MB/s = Gbps * 1000 / 8`
- Returns 0 if no InfiniBand interface

### JSON Schema

Network interface data in healthcheck JSON includes:

```json
{
  "name": "eth0",
  "type": "ethernet",
  "max_speed": 100000,
  "effective_speed": 100000,
  "max_rate": 0,
  "effective_rate": 0,
  "mtu": 9000,
  "pci_address": "0000:3d:00.0",
  "vendor_model": "15b3:1021",
  "model": "ConnectX-7"
}
```

## Troubleshooting

### Common Issues

#### Issue: Speed Not Detected
**Symptoms:** Network interfaces show "unknown" for speed

**Causes:**
- Interface is virtual (no sysfs speed file)
- Interface type not recognized
- sysfs paths not accessible in container

**Resolution:**
- Check with: `cat /sys/class/net/<iface>/speed`
- For virtual interfaces, check parent interface
- Ensure container has `/host-sys` mounted

#### Issue: InfiniBand Rate Not Detected
**Symptoms:** InfiniBand interfaces show 0 for rate

**Causes:**
- `find` command not working with globbing
- InfiniBand sysfs not mounted
- Rate file in different location

**Resolution:**
- Check rate file manually: `find /sys/class/net/ib0/device/infiniband -name rate`
- Verify InfiniBand tools: `ibstat`
- Check container mounts for /sys access

#### Issue: MTU Mismatch
**Symptoms:** "Warning: MTU too small for optimal performance"

**Causes:**
- MTU set to standard 1500 instead of jumbo frames
- Configuration not applied to interface

**Resolution:**
```bash
# Check current MTU
ip link show eth0

# Temporarily change MTU
sudo ip link set eth0 mtu 9000

# Permanently configure in network config
# (varies by distribution)
```

#### Issue: Bond Not Recognized
**Symptoms:** Bond interface shows as regular Ethernet

**Causes:**
- Bond not properly configured
- Mode not set to LACP (802.3ad)
- Slaves not detected

**Resolution:**
```bash
# Check bond status
cat /sys/class/net/bond0/bonding/slaves
cat /sys/class/net/bond0/bonding/mode

# Verify LACP
ethtool -i bond0
```

### Debug Information

Enable debug output in hostcheck:
```bash
# In container/host running hostcheck.sh
set -euxv
./hostcheck.sh
```

This shows:
- All function calls
- Variable expansions
- Command exit codes
- File operations

### Getting More Help

1. **Check full hostcheck output**: `kubectl weka preflight nodes -v`
2. **Review hostcheck JSON**: Check the `/tmp/hostcheck-*.json` file on the node
3. **Verify NIC database**: Check `pkg/device-support/network.go` for your NIC model
4. **Network interface details**:
   ```bash
   ip link show
   ethtool <interface>
   ethtool -i <interface>
   ```

## References

- [Mellanox OFED Documentation](https://www.nvidia.com/en-us/networking/infiniband/)
- [Linux Bonding Documentation](https://www.kernel.org/doc/html/latest/networking/bonding.html)
- [InfiniBand Speed Generations](https://en.wikipedia.org/wiki/InfiniBand#Data_rates)
- [kubectl-weka README](../README.md)
- [Developer Guide](../DEVELOPER_GUIDE.md)

