package cmd

import (
	"k8s.io/api/core/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeHostChecksPod(ns, nodeName, podName, labelKey, labelVal string) *v1.Pod {

	script := `
set -eu

json_escape() { echo "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }

# ----- OS detection via /etc/os-release (host) -----
OSR=""
IS_RHCOS=0
if [ -f /host/etc/os-release ]; then
  OSR="$(cat /host/etc/os-release | tr '\n' ' ')"
  ID="$(grep -E '^ID=' /host/etc/os-release 2>/dev/null | head -n1 | cut -d= -f2 | tr -d '"')"
  NAME="$(grep -E '^NAME=' /host/etc/os-release 2>/dev/null | head -n1 | cut -d= -f2- | tr -d '"')"
  if [ "$ID" = "rhcos" ] || echo "$NAME" | grep -qi 'coreos'; then
    IS_RHCOS=1
  fi
fi

# ----- WEKA dir check (>= 300GB available) -----
WEKADIR="/host/opt/k8s-weka"
if [ "$IS_RHCOS" -eq 1 ]; then
  WEKADIR="/host/root/k8s-weka"
fi

WEKADIR_OK=0
WEKADIR_AVAIL_BYTES=0
WEKADIR_DETAIL=""
WEKADIR_PATH="${WEKADIR#/host}"

if [ -d "$WEKADIR" ]; then
  WEKADIR_AVAIL_BYTES="$(df -PB1 "$WEKADIR" 2>/dev/null | tail -n1 | awk '{print $4}' || echo 0)"
  MIN_PASS=$((300*1000*1000*1000))
  if [ "$WEKADIR_AVAIL_BYTES" -ge "$MIN_PASS" ]; then
    WEKADIR_OK=1
  fi
  WEKADIR_DETAIL="ok"
else
  WEKADIR_DETAIL="directory does not exist"
fi

# ----- XFS installed -----
XFS_OK=0
XFS_DETAIL=""
for p in /host/usr/sbin/mkfs.xfs /host/sbin/mkfs.xfs /host/usr/bin/mkfs.xfs /host/bin/mkfs.xfs; do
  if [ -x "$p" ]; then
    XFS_OK=1
    XFS_DETAIL="found $p"
    break
  fi
done
if [ "$XFS_OK" -eq 0 ]; then
  XFS_DETAIL="mkfs.xfs not found (xfsprogs likely missing)"
fi

# ----- WEKA agent service must not exist -----
WEKA_CLEAN=1
WEKA_DETAIL="clean"

for u in \
  /host/etc/systemd/system/weka-agent.service \
  /host/usr/lib/systemd/system/weka-agent.service \
  /host/lib/systemd/system/weka-agent.service; do
  if [ -f "$u" ]; then
    WEKA_CLEAN=0
    WEKA_DETAIL="weka-agent.service exists"
    break
  fi
done

# ----- Mellanox model mapping by PCI device ID (15b3:xxxx) -----
model_from_devid() {
  case "$1" in
    1021) echo "ConnectX-7" ;;
    101d) echo "CX-6 Dx" ;;
    1017) echo "CX-5" ;;
    1023) echo "ConnectX-8" ;;
    *)    echo "" ;;
  esac
}

# ----- Mellanox interface inventory -----
# We build:
#  - mlx_ifaces: [{name,bond,ip,model}, ...]
#  - mlx_bonds:  [{name,ip,slaves,mode}, ...] only bonds that have Mellanox slaves
#
MLX_IFACES_JSON=""
MLX_IFACES_COUNT=0

# helper: read IPv4 CIDR for iface from host netns (pod uses hostNetwork)
iface_ipv4() {
  # prints first IPv4 CIDR, or empty
  ip -o -4 addr show dev "$1" 2>/dev/null | awk '{print $4}' | head -n1 || true
}

# helper: bond master name if enslaved, else empty
iface_bond_master() {
  # use host sysfs to avoid dependence on iproute details
  # /sys/class/net/IFACE/master -> .../bond0
  if [ -L "/host-sys/class/net/$1/master" ]; then
    m="$(readlink -f "/host-sys/class/net/$1/master" 2>/dev/null || true)"
    b="$(basename "$m" 2>/dev/null || true)"
    echo "$b"
  fi
}

human_mbps() {
  # input: Mbps number, output: "100Gb/s" etc.
  n="$1"
  if [ -z "$n" ] || [ "$n" = "-1" ]; then
    echo ""
    return
  fi
  if [ "$n" -ge 1000 ]; then
    g=$((n/1000))
    echo "${g}Gb/s"
  else
    echo "${n}Mb/s"
  fi
}

iface_speed() {
  ifn="$1"
  # Ethernet-style
  if [ -f "/host-sys/class/net/$ifn/speed" ]; then
    s="$(cat "/host-sys/class/net/$ifn/speed" 2>/dev/null || true)"
    hs="$(human_mbps "$s")"
    if [ -n "$hs" ]; then
      echo "$hs"
      return
    fi
  fi

  # InfiniBand-style (rate string)
  for r in /host-sys/class/net/"$ifn"/device/infiniband/*/ports/*/rate; do
    [ -f "$r" ] || continue
    # example: "200 Gb/sec (4X HDR)"
    rate="$(cat "$r" 2>/dev/null || true)"
    gb="$(echo "$rate" | awk '{print $1}' 2>/dev/null || true)"
    unit="$(echo "$rate" | awk '{print $2}' 2>/dev/null || true)"
    if [ -n "$gb" ] && echo "$unit" | grep -qi '^gb'; then
      echo "${gb}Gb/s"
      return
    fi
  done

  echo "unknown"
}

# enumerate Mellanox NIC PCI functions, then their net ifaces
for d in /host-sys/bus/pci/devices/*; do
  [ -f "$d/vendor" ] || continue
  v="$(cat "$d/vendor" 2>/dev/null || true)"
  [ "$v" = "0x15b3" ] || continue

  # network class only
  c="$(cat "$d/class" 2>/dev/null || true)"
  case "$c" in 0x02*) ;; *) continue ;; esac

  pci="$(basename "$d")"
  devhex="$(cat "$d/device" 2>/dev/null || echo "")"  # e.g. 0x1021
  devid="${devhex#0x}"
  mdl="$(model_from_devid "$devid")"
  if [ -z "$mdl" ]; then
    mdl="unknown (15b3:${devid} on ${pci})"
  fi

  [ -d "$d/net" ] || continue
  for n in "$d"/net/*; do
    [ -e "$n" ] || continue
    ifn="$(basename "$n")"
    bond="$(iface_bond_master "$ifn")"
    ip4=""
    if [ -z "$bond" ]; then
      ip4="$(iface_ipv4 "$ifn")"
    fi
    spd="$(iface_speed "$ifn")"

    obj="{\"name\":\"$(json_escape "$ifn")\",\"bond\":\"$(json_escape "$bond")\",\"ip\":\"$(json_escape "$ip4")\",\"speed\":\"$(json_escape "$spd")\",\"model\":\"$(json_escape "$mdl")\"}"

    if [ "$MLX_IFACES_COUNT" -gt 0 ]; then
      MLX_IFACES_JSON="${MLX_IFACES_JSON},"
    fi
    MLX_IFACES_JSON="${MLX_IFACES_JSON}${obj}"
    MLX_IFACES_COUNT=$((MLX_IFACES_COUNT+1))
  done
done

MLX_PRESENT=false
if [ "$MLX_IFACES_COUNT" -gt 0 ]; then
  MLX_PRESENT=true
fi

# ----- Bonds that include Mellanox slaves + LACP validation -----
BOND_LACP_OK=1
BOND_LACP_DETAIL="no Mellanox bonds detected"
MLX_BONDS_JSON=""
MLX_BONDS_COUNT=0

# Build a space-separated set of Mellanox iface names (for quick membership checks)
MLX_NAMES=" "
if [ "$MLX_IFACES_COUNT" -gt 0 ]; then
  # Extract names from JSON: split on "name":"...".
  # (simple parser; good enough for our generated JSON)
  MLX_NAMES="$MLX_NAMES$(echo "$MLX_IFACES_JSON" | sed 's/{"name":"\([^"]*\)".*/\1\n/g' 2>/dev/null || true) "
fi

for b in /host-sys/class/net/bond*; do
  [ -d "$b" ] || continue
  bond="$(basename "$b")"
  slaves_file="$b/bonding/slaves"
  mode_file="$b/bonding/mode"
  [ -f "$slaves_file" ] || continue
  [ -f "$mode_file" ] || continue

  slaves="$(cat "$slaves_file" 2>/dev/null || true)"
  mode="$(cat "$mode_file" 2>/dev/null || true)"

  # check if this bond contains Mellanox slaves
  has_mlx=0
  for s in $slaves; do
    echo " $MLX_IFACES_JSON " | grep -q "\"name\":\"$s\"" && { has_mlx=1; break; } || true
  done

  if [ "$has_mlx" -eq 1 ]; then
    # validate LACP
    if echo "$mode" | grep -q "802\.3ad"; then
      BOND_LACP_DETAIL="ok"
    else
      BOND_LACP_OK=0
      BOND_LACP_DETAIL="bond=$bond mode='$mode' slaves='$slaves' (must be 802.3ad/LACP)"
    fi

    bip="$(iface_ipv4 "$bond")"

    # slaves list json
    sjson=""
    sc=0
    for s in $slaves; do
      [ -n "$s" ] || continue
      if [ "$sc" -gt 0 ]; then sjson="${sjson},"; fi
      sjson="${sjson}\"$(json_escape "$s")\""
      sc=$((sc+1))
    done
    obj="{\"name\":\"$(json_escape "$bond")\",\"ip\":\"$(json_escape "$bip")\",\"slaves\":[${sjson}],\"mode\":\"$(json_escape "$mode")\"}"
    if [ "$MLX_BONDS_COUNT" -gt 0 ]; then MLX_BONDS_JSON="${MLX_BONDS_JSON},"; fi
    MLX_BONDS_JSON="${MLX_BONDS_JSON}${obj}"
    MLX_BONDS_COUNT=$((MLX_BONDS_COUNT+1))
  fi

  # if bond_lacp already failed, we still continue collecting bonds
done

# ----- CPU and Memory detection -----
HT_ENABLED=0
PHYSICAL_CORES=0
LOGICAL_CORES=0
MEMORY_BYTES=0
FREE_MEMORY_BYTES=0
HUGEPAGES_FREE_BYTES=0
CPU_MODEL=""
CPU_FAMILY=""
CPU_ARCH=""
CPU_SOCKETS=0

# Get logical cores (number of processors)
LOGICAL_CORES="$(grep -c '^processor' /host/proc/cpuinfo 2>/dev/null || echo 1)"

if [ -f /host/proc/cpuinfo ]; then
  # Get CPU model
  CPU_MODEL="$(grep '^model name' /host/proc/cpuinfo 2>/dev/null | head -n1 | cut -d: -f2 | sed 's/^ //')"
  
  # Get CPU vendor (Intel, AMD, ARM, etc.)
  VENDOR_ID="$(grep '^vendor_id' /host/proc/cpuinfo 2>/dev/null | head -n1 | cut -d: -f2 | sed 's/^ //')"
  case "$VENDOR_ID" in
    GenuineIntel) CPU_FAMILY="Intel" ;;
    AuthenticAMD) CPU_FAMILY="AMD" ;;
    *)
      # For other CPUs, extract from model name
      if echo "$CPU_MODEL" | grep -qi 'intel'; then
        CPU_FAMILY="Intel"
      elif echo "$CPU_MODEL" | grep -qi 'amd'; then
        CPU_FAMILY="AMD"
      elif echo "$CPU_MODEL" | grep -qi 'grace'; then
        CPU_FAMILY="Grace"
      elif echo "$CPU_MODEL" | grep -qi 'arm'; then
        CPU_FAMILY="ARM"
      fi
      ;;
  esac
  
  # Get architecture from uname
  ARCH="$(uname -m 2>/dev/null || echo 'unknown')"
  case "$ARCH" in
    x86_64|amd64) CPU_ARCH="x86_64" ;;
    aarch64|arm64) CPU_ARCH="aarch64" ;;
    *) CPU_ARCH="$ARCH" ;;
  esac
  
  # Get physical cores from "cpu cores" field (most reliable method)
  CPU_CORES="$(grep '^cpu cores' /host/proc/cpuinfo 2>/dev/null | head -n1 | awk '{print $NF}')"
  if [ -n "$CPU_CORES" ] && [ "$CPU_CORES" -gt 0 ]; then
    # Calculate total physical cores: (cpu cores per socket) × (number of sockets)
    SIBLINGS="$(grep '^siblings' /host/proc/cpuinfo 2>/dev/null | head -n1 | awk '{print $NF}')"
    if [ -z "$SIBLINGS" ] || [ "$SIBLINGS" -eq 0 ]; then
      SIBLINGS="$CPU_CORES"
    fi
    CPU_SOCKETS=$((LOGICAL_CORES / SIBLINGS))
    if [ "$CPU_SOCKETS" -eq 0 ]; then
      CPU_SOCKETS=1
    fi
    PHYSICAL_CORES=$((CPU_CORES * CPU_SOCKETS))
  else
    # Fallback: assume physical_id field exists and count unique ones
    PHYSICAL_CORES="$(grep 'physical id' /host/proc/cpuinfo 2>/dev/null | sort -u | wc -l)"
    if [ "$PHYSICAL_CORES" -eq 0 ] || [ "$PHYSICAL_CORES" -eq 1 ]; then
      # Last resort: divide logical by siblings
      SIBLINGS="$(grep '^siblings' /host/proc/cpuinfo 2>/dev/null | head -n1 | awk '{print $NF}')"
      if [ -n "$SIBLINGS" ] && [ "$SIBLINGS" -gt 0 ]; then
        PHYSICAL_CORES=$((LOGICAL_CORES / SIBLINGS))
        CPU_SOCKETS=$((LOGICAL_CORES / SIBLINGS))
      else
        PHYSICAL_CORES="$LOGICAL_CORES"
        CPU_SOCKETS=1
      fi
    else
      # Count actual sockets from physical_id
      CPU_SOCKETS="$(grep 'physical id' /host/proc/cpuinfo 2>/dev/null | sort -u | wc -l)"
    fi
  fi
fi

# Ensure we have at least something
if [ "$PHYSICAL_CORES" -eq 0 ]; then
  PHYSICAL_CORES="$LOGICAL_CORES"
fi
if [ "$CPU_SOCKETS" -eq 0 ]; then
  CPU_SOCKETS=1
fi

# Check if HT is enabled (logical cores > physical cores)
if [ "$LOGICAL_CORES" -gt "$PHYSICAL_CORES" ]; then
  HT_ENABLED=1
fi

# Get memory info (in bytes)
if [ -f /host/proc/meminfo ]; then
  MEMORY_KB="$(grep '^MemTotal:' /host/proc/meminfo 2>/dev/null | awk '{print $2}')"
  MEMORY_BYTES=$((MEMORY_KB * 1024))
  
  FREE_KB="$(grep '^MemAvailable:' /host/proc/meminfo 2>/dev/null | awk '{print $2}')"
  FREE_MEMORY_BYTES=$((FREE_KB * 1024))
fi

# Get hugepages free (2MB hugepages by default)
if [ -d /host/sys/kernel/mm/hugepages ]; then
  for hp_dir in /host/sys/kernel/mm/hugepages/hugepages-*; do
    if [ -d "$hp_dir" ]; then
      free_hp="$(cat "$hp_dir/free_hugepages" 2>/dev/null || echo 0)"
      page_size_kb="$(basename "$hp_dir" | sed 's/hugepages-//;s/kB//')"
      page_size_bytes=$((page_size_kb * 1024))
      HUGEPAGES_FREE_BYTES=$((HUGEPAGES_FREE_BYTES + free_hp * page_size_bytes))
    fi
  done
fi

# ----- Kernel version -----
KERNEL_VERSION=""
if [ -f /host/proc/version ]; then
  KERNEL_VERSION="$(cat /host/proc/version 2>/dev/null | awk '{print $3}')"
fi

# ----- NVMe Drive Discovery -----
# Discover NVMe drives by examining /dev and /sys
NVME_DRIVES_JSON=""
NVME_DRIVES_COUNT=0
NVME_DETAIL="no NVMe drives found"

# Function to get drive serial number from sysfs
get_nvme_serial() {
  local dev="$1"
  local serial=""
  
  # Try /sys/block/*/device/serial
  if [ -f "/host-sys/block/$dev/device/serial" ]; then
    serial="$(cat "/host-sys/block/$dev/device/serial" 2>/dev/null | tr -d ' \t\n\r')"
  fi
  
  # Fallback: try /sys/class/block/*/device/serial
  if [ -z "$serial" ] && [ -f "/host-sys/class/block/$dev/device/serial" ]; then
    serial="$(cat "/host-sys/class/block/$dev/device/serial" 2>/dev/null | tr -d ' \t\n\r')"
  fi
  
  echo "$serial"
}

# Function to get drive model
get_nvme_model() {
  local dev="$1"
  local model=""
  
  # Try /sys/block/*/device/model
  if [ -f "/host-sys/block/$dev/device/model" ]; then
    model="$(cat "/host-sys/block/$dev/device/model" 2>/dev/null | tr -d '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  fi
  
  # Fallback: try /sys/class/block/*/device/model
  if [ -z "$model" ] && [ -f "/host-sys/class/block/$dev/device/model" ]; then
    model="$(cat "/host-sys/class/block/$dev/device/model" 2>/dev/null | tr -d '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  fi
  
  echo "$model"
}

# Function to get drive size in bytes
get_nvme_size() {
  local dev="$1"
  local size=0
  
  # Read size from /sys/block/*/size (in 512-byte sectors)
  if [ -f "/host-sys/block/$dev/size" ]; then
    local sectors="$(cat "/host-sys/block/$dev/size" 2>/dev/null || echo 0)"
    size=$((sectors * 512))
  fi
  
  echo "$size"
}

# Function to check if drive or its partitions are mounted
is_nvme_mounted() {
  local dev="$1"
  local mount_point=""
  
  # Check /proc/mounts for the device or any partitions
  if grep -q "^/host/dev/${dev}" /host/proc/mounts 2>/dev/null; then
    mount_point="$(grep "^/host/dev/${dev}" /host/proc/mounts 2>/dev/null | head -n1 | awk '{print $2}')"
    echo "true|$mount_point"
    return
  fi
  
  # Check for partitions (nvme0n1p1, nvme0n1p2, etc.)
  if grep -q "^/host/dev/${dev}p" /host/proc/mounts 2>/dev/null; then
    mount_point="$(grep "^/host/dev/${dev}p" /host/proc/mounts 2>/dev/null | head -n1 | awk '{print $2}')"
    echo "true|$mount_point"
    return
  fi
  
  echo "false|"
}

# Scan for NVMe drives in /dev
for nvme_dev in /host/dev/nvme[0-9]n[0-9]*; do
  [ -b "$nvme_dev" ] || continue
  
  dev_name="$(basename "$nvme_dev")"
  
  # Get drive information
  serial="$(get_nvme_serial "$dev_name")"
  model="$(get_nvme_model "$dev_name")"
  size="$(get_nvme_size "$dev_name")"
  mount_info="$(is_nvme_mounted "$dev_name")"
  
  is_mounted="${mount_info%%|*}"
  mount_point="${mount_info##*|}"
  
  # Create JSON object
  obj="{"
  obj="$obj\"device_name\":\"$(json_escape "$dev_name")\","
  obj="$obj\"device_path\":\"/dev/$(json_escape "$dev_name")\","
  obj="$obj\"serial\":\"$(json_escape "$serial")\","
  obj="$obj\"model\":\"$(json_escape "$model")\","
  obj="$obj\"size\":$size,"
  obj="$obj\"mounted\":$is_mounted,"
  obj="$obj\"mount_point\":\"$(json_escape "$mount_point")\""
  obj="$obj}"
  
  if [ "$NVME_DRIVES_COUNT" -gt 0 ]; then
    NVME_DRIVES_JSON="${NVME_DRIVES_JSON},"
  fi
  NVME_DRIVES_JSON="${NVME_DRIVES_JSON}${obj}"
  NVME_DRIVES_COUNT=$((NVME_DRIVES_COUNT+1))
done

if [ "$NVME_DRIVES_COUNT" -gt 0 ]; then
  NVME_DETAIL="found $NVME_DRIVES_COUNT NVMe drive(s)"
fi

# Output JSON (single line)
printf '{'
printf '"is_rhcos":%s,' "$([ "$IS_RHCOS" -eq 1 ] && echo true || echo false)"
printf '"os_release":"%s",' "$(json_escape "$OSR")"
printf '"weka_dir_ok":%s,' "$([ "$WEKADIR_OK" -eq 1 ] && echo true || echo false)"
printf '"weka_dir_path":"%s",' "$(json_escape "$WEKADIR_PATH")"
printf '"weka_dir_detail":"%s",' "$(json_escape "$WEKADIR_DETAIL")"
printf '"weka_dir_avail_bytes":%d,' "$WEKADIR_AVAIL_BYTES"
printf '"xfs_installed":%s,' "$([ "$XFS_OK" -eq 1 ] && echo true || echo false)"
printf '"xfs_detail":"%s",' "$(json_escape "$XFS_DETAIL")"
printf '"weka_client_clean":%s,' "$([ "$WEKA_CLEAN" -eq 1 ] && echo true || echo false)"
printf '"weka_client_detail":"%s",' "$(json_escape "$WEKA_DETAIL")"
printf '"mlx_ifaces":[%s],' "$MLX_IFACES_JSON"
printf '"mlx_bonds":[%s],' "$MLX_BONDS_JSON"
printf '"bond_lacp_ok":%s,' "$([ "$BOND_LACP_OK" -eq 1 ] && echo true || echo false)"
printf '"bond_lacp_detail":"%s",' "$(json_escape "$BOND_LACP_DETAIL")"
printf '"ht_enabled":%s,' "$([ "$HT_ENABLED" -eq 1 ] && echo true || echo false)"
printf '"physical_cores":%d,' "$PHYSICAL_CORES"
printf '"logical_cores":%d,' "$LOGICAL_CORES"
printf '"memory_bytes":%d,' "$MEMORY_BYTES"
printf '"free_memory_bytes":%d,' "$FREE_MEMORY_BYTES"
printf '"hugepages_free_bytes":%d,' "$HUGEPAGES_FREE_BYTES"
printf '"kernel_version":"%s",' "$(json_escape "$KERNEL_VERSION")"
printf '"cpu_model":"%s",' "$(json_escape "$CPU_MODEL")"
printf '"cpu_family":"%s",' "$(json_escape "$CPU_FAMILY")"
printf '"cpu_arch":"%s",' "$(json_escape "$CPU_ARCH")"
printf '"cpu_sockets":%d,' "$CPU_SOCKETS"
printf '"nvme_drives":[%s],' "$NVME_DRIVES_JSON"
printf '"nvme_drive_count":%d,' "$NVME_DRIVES_COUNT"
printf '"nvme_drive_detail":"%s"' "$(json_escape "$NVME_DETAIL")"
printf '}\n'
`
	return &v1.Pod{
		ObjectMeta: v2.ObjectMeta{
			Name:      podName,
			Namespace: ns,
			Labels: map[string]string{
				labelKey: labelVal,
			},
		},
		Spec: v1.PodSpec{
			NodeName:      nodeName,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			RestartPolicy: v1.RestartPolicyNever,
			Tolerations: []v1.Toleration{
				{
					Operator: v1.TolerationOpExists, // Tolerate all taints
				},
			},
			Containers: []v1.Container{
				{
					Name:    "hostchecks",
					Image:   "busybox:1.36",
					Command: []string{"sh", "-c", script},
					SecurityContext: &v1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(false),
						ReadOnlyRootFilesystem:   boolPtr(true),
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "host-root", MountPath: "/host", ReadOnly: true},
						{Name: "host-sys", MountPath: "/host-sys", ReadOnly: true},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "host-root",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/",
							Type: hostPathTypePtr(v1.HostPathDirectory),
						},
					},
				},
				{
					Name: "host-sys",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/sys",
							Type: hostPathTypePtr(v1.HostPathDirectory),
						},
					},
				},
			},
		},
	}
}
