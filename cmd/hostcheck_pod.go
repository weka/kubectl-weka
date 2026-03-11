package cmd

import (
	"k8s.io/api/core/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeHostChecksPod(ns, nodeName, podName, labelKey, labelVal string) *v1.Pod {

	script := `
set -eu

json_escape() { 
  local str="$1"
  # Use printf with %q and then process to create JSON string
  str="${str//\\/\\\\}"  # backslash
  str="${str//\"/\\\"}"  # quote
  str="${str//$'\t'/\\t}"  # tab
  str="${str//$'\n'/\\n}"  # newline
  str="${str//$'\r'/\\r}"  # carriage return
  echo "$str"
}

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

# helper: get PCI address for network interface
get_iface_pci_addr() {
  local ifname="$1"
  local pci_dir="/host-sys/class/net/$ifname/device"
  
  if [ -d "$pci_dir" ]; then
    # Resolve the symlink to get the actual device path
    local pci_path="$(readlink -f "$pci_dir" 2>/dev/null || true)"
    if [ -n "$pci_path" ]; then
      # Try to extract PCI address from path
      # Look for pattern like 0000:00:1d.0 in the device path
      local pci_addr=$(echo "$pci_path" | grep -o '[0-9a-f]\{4\}:[0-9a-f]\{2\}:[0-9a-f]\{2\}\.[0-9]' | tail -n1)
      
      # If regex didn't work, try alternative method using directory walking
      if [ -z "$pci_addr" ]; then
        # Walk up the sysfs tree to find a directory matching PCI address format
        local dev_dir="$(dirname "$pci_path")"
        while [ -n "$dev_dir" ] && [ "$dev_dir" != "/" ] && [ "$dev_dir" != "/host" ]; do
          local dir_name="$(basename "$dev_dir")"
          if echo "$dir_name" | grep -q '^[0-9a-f]\{4\}:[0-9a-f]\{2\}:[0-9a-f]\{2\}\.[0-9]'; then
            pci_addr="$dir_name"
            break
          fi
          dev_dir="$(dirname "$dev_dir")"
        done
      fi
      
      echo "$pci_addr"
    fi
  fi
}

# helper: get NUMA node for a PCI device by fetching from sysfs
get_pci_numa_node() {
    local pci="$1" 
    [[ -z "$pci" ]] && echo -1 && return
    [[ "$pci" =~ ^[0-9a-fA-F]{2}: ]] && pci="0000:$pci"
    local f="/sys/bus/pci/devices/$pci/numa_node"
    [[ -f "$f" ]] && cat "$f" && return
	echo -1
}

# helper: get interface type (ethernet or infiniband)
get_iface_type() {
  local ifname="$1"
  local sysfs="/host-sys/class/net/$ifname"
  
  # Check for InfiniBand
  if [ -d "$sysfs/device/infiniband" ]; then
    echo "infiniband"
    return
  fi
  
  # Check device type
  if [ -f "$sysfs/type" ]; then
    local type="$(cat "$sysfs/type" 2>/dev/null || true)"
    case "$type" in
      32) echo "infiniband" ;;
      1|*) echo "ethernet" ;;  # 1 is Ethernet, default to ethernet
    esac
  else
    echo "ethernet"
  fi
}

# helper: get interface MAC address
get_iface_mac() {
  local ifname="$1"
  [ -f "/host-sys/class/net/$ifname/address" ] && cat "/host-sys/class/net/$ifname/address" 2>/dev/null || echo ""
}

# helper: get interface MTU
get_iface_mtu() {
  local ifname="$1"
  [ -f "/host-sys/class/net/$ifname/mtu" ] && cat "/host-sys/class/net/$ifname/mtu" 2>/dev/null || echo "0"
}

# helper: get interface status (up/down)
get_iface_status() {
  local ifname="$1"
  if [ -f "/host-sys/class/net/$ifname/operstate" ]; then
    cat "/host-sys/class/net/$ifname/operstate" 2>/dev/null || echo "unknown"
  else
    echo "unknown"
  fi
}

# helper: get network interface metrics from /proc/net/dev
get_iface_metrics() {
  local ifname="$1"
  local metrics_line
  
  metrics_line="$(grep -E "^\s*$ifname:" /host/proc/net/dev 2>/dev/null | sed 's/.*:\s*//' || true)"
  
  if [ -z "$metrics_line" ]; then
    echo ""
    return
  fi
  
  # Format: bytes_rcvd packets_rcvd errs_rcvd drop_rcvd fifo_rcvd frame_rcvd compressed_rcvd multicast_rcvd
  #         bytes_sent packets_sent errs_sent drop_sent fifo_sent colls_sent carrier_sent compressed_sent
  
  local bytes_in="$(echo "$metrics_line" | awk '{print $1}' | sed 's/[^0-9]//g')"
  local packets_in="$(echo "$metrics_line" | awk '{print $2}' | sed 's/[^0-9]//g')"
  local errors_in="$(echo "$metrics_line" | awk '{print $3}' | sed 's/[^0-9]//g')"
  local dropped_in="$(echo "$metrics_line" | awk '{print $4}' | sed 's/[^0-9]//g')"
  local overruns_in="$(echo "$metrics_line" | awk '{print $5}' | sed 's/[^0-9]//g')"
  
  local bytes_out="$(echo "$metrics_line" | awk '{print $9}' | sed 's/[^0-9]//g')"
  local packets_out="$(echo "$metrics_line" | awk '{print $10}' | sed 's/[^0-9]//g')"
  local errors_out="$(echo "$metrics_line" | awk '{print $11}' | sed 's/[^0-9]//g')"
  local dropped_out="$(echo "$metrics_line" | awk '{print $12}' | sed 's/[^0-9]//g')"
  local overruns_out="$(echo "$metrics_line" | awk '{print $13}' | sed 's/[^0-9]//g')"
  local collisions="$(echo "$metrics_line" | awk '{print $14}' | sed 's/[^0-9]//g')"
  
  printf '{"bytes_in":%d,"bytes_out":%d,"packets_in":%d,"packets_out":%d,"errors_in":%d,"errors_out":%d,"dropped_in":%d,"dropped_out":%d,"overruns_in":%d,"overruns_out":%d,"collisions_in":%d,"crc_errors":0}' \
    "${bytes_in:-0}" "${bytes_out:-0}" "${packets_in:-0}" "${packets_out:-0}" \
    "${errors_in:-0}" "${errors_out:-0}" "${dropped_in:-0}" "${dropped_out:-0}" \
    "${overruns_in:-0}" "${overruns_out:-0}" "${collisions:-0}"
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

# helper: check if interface is used as default route
is_default_route_iface() {
  local ifname="$1"
  # Check ip route show for default route via this interface
  ip -o route show 2>/dev/null | grep -E "^default.*\s$ifname(\s|$)" >/dev/null 2>&1 && echo true || echo false
}

# helper: get all routes for a specific interface
get_routes_for_iface() {
  local ifname="$1"
  local route_json=""
  local route_count=0
  
  # Get all routes with this interface
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    
    # Parse route line from ip route show
    # Format: <destination> via <gateway> dev <device> metric <metric> ...
    local dest="$(echo "$line" | awk '{print $1}')"
    local via_idx=2
    local gateway=""
    local metric=""
    
    # Check if it contains "via"
    if echo "$line" | grep -q " via "; then
      gateway="$(echo "$line" | awk '{for(i=1;i<=NF;i++) if($i=="via") print $(i+1); exit}')"
    fi
    
    # Get metric if present
    if echo "$line" | grep -q " metric "; then
      metric="$(echo "$line" | awk '{for(i=1;i<=NF;i++) if($i=="metric") print $(i+1); exit}')"
    fi
    
    if [ -z "$dest" ]; then
      continue
    fi
    
    local route_obj="{\"destination\":\"$(json_escape "$dest")\""
    [ -n "$gateway" ] && route_obj="$route_obj,\"gateway\":\"$(json_escape "$gateway")\""
    route_obj="$route_obj,\"device\":\"$(json_escape "$ifname")\""
    [ -n "$metric" ] && route_obj="$route_obj,\"metric\":$metric"
    route_obj="$route_obj}"
    
    if [ "$route_count" -gt 0 ]; then
      route_json="${route_json},"
    fi
    route_json="${route_json}${route_obj}"
    route_count=$((route_count+1))
  done < <(ip -o route show 2>/dev/null | grep " dev $ifname " || true)
  
  echo "$route_json"
}

# helper: detect CNI Pod CIDR from kubelet configuration
get_cni_from_kubelet_config() {
  local kubelet_conf="/host/etc/kubernetes/kubelet.conf"
  
  # Try standard location first
  if [ -f "$kubelet_conf" ]; then
    grep -o '"podCIDR":"[^"]*"' "$kubelet_conf" 2>/dev/null | cut -d'"' -f4
    return 0
  fi
  
  # Try alternative location
  kubelet_conf="/host/var/lib/kubelet/kubeconfig"
  if [ -f "$kubelet_conf" ]; then
    grep -o '"podCIDR":"[^"]*"' "$kubelet_conf" 2>/dev/null | cut -d'"' -f4
    return 0
  fi
  
  return 1
}

# helper: detect CNI Pod CIDR from kubelet process arguments
get_cni_from_kubelet_args() {
  # Find kubelet process
  local kubelet_pid=""
  
  # Try pgrep first
  kubelet_pid=$(pgrep -f "[k]ubelet" 2>/dev/null | head -n1)
  
  if [ -n "$kubelet_pid" ] && [ -f "/proc/$kubelet_pid/cmdline" ]; then
    # Read cmdline (arguments are null-separated)
    tr '\0' ' ' < "/proc/$kubelet_pid/cmdline" 2>/dev/null | grep -o "\--pod-cidr=[^ ]*" | cut -d= -f2
    return 0
  fi
  
  return 1
}

# helper: detect CNI Pod CIDR from Flannel data file
get_cni_from_flannel_data() {
  local flannel_file="/host/var/lib/cni/flannel/subnet.env"
  
  if [ -f "$flannel_file" ]; then
    # Extract network from Flannel config
    grep "^FLANNEL_NETWORK" "$flannel_file" 2>/dev/null | cut -d= -f2
    return 0
  fi
  
  return 1
}

# helper: detect CNI type and settings from CNI config files
get_cni_from_config_files() {
  local cni_dir="/host/etc/cni/net.d"
  local cni_type=""
  local pod_cidr=""
  
  if [ ! -d "$cni_dir" ]; then
    return 1
  fi
  
  # Check for Flannel
  if [ -f "$cni_dir/10-flannel.conflist" ]; then
    cni_type="flannel"
    pod_cidr=$(grep -o '"Network":"[^"]*"' "$cni_dir/10-flannel.conflist" 2>/dev/null | cut -d'"' -f4)
    echo "$cni_type|$pod_cidr"
    return 0
  fi
  
  # Check for Calico
  if [ -f "$cni_dir/10-calico.conflist" ]; then
    cni_type="calico"
    pod_cidr=$(grep -o '"subnet":"[^"]*"' "$cni_dir/10-calico.conflist" 2>/dev/null | cut -d'"' -f4 | head -n1)
    echo "$cni_type|$pod_cidr"
    return 0
  fi
  
  # Check for Weave
  if [ -f "$cni_dir/10-weave.conf" ]; then
    cni_type="weave"
    pod_cidr=$(grep -o '"subnet":"[^"]*"' "$cni_dir/10-weave.conf" 2>/dev/null | cut -d'"' -f4 | head -n1)
    echo "$cni_type|$pod_cidr"
    return 0
  fi
  
  # Generic search for Network or subnet in any config
  for config in "$cni_dir"/*.conf*; do
    if [ -f "$config" ]; then
      # Try to extract Network (Flannel pattern)
      pod_cidr=$(grep -o '"Network":"[^"]*"' "$config" 2>/dev/null | cut -d'"' -f4)
      if [ -n "$pod_cidr" ]; then
        cni_type="unknown"
        echo "$cni_type|$pod_cidr"
        return 0
      fi
      
      # Try to extract subnet (Calico/Weave pattern)
      pod_cidr=$(grep -o '"subnet":"[^"]*"' "$config" 2>/dev/null | cut -d'"' -f4 | head -n1)
      if [ -n "$pod_cidr" ]; then
        cni_type="unknown"
        echo "$cni_type|$pod_cidr"
        return 0
      fi
    fi
  done
  
  return 1
}

# helper: comprehensive CNI Pod CIDR detection with fallback chain
detect_cni_pod_cidr() {
  local pod_cidr=""
  local source=""
  local cni_type=""
  
  # Priority 1: Kubelet config (most reliable)
  pod_cidr=$(get_cni_from_kubelet_config)
  if [ -n "$pod_cidr" ]; then
    echo "$pod_cidr|kubelet_config|unknown"
    return 0
  fi
  
  # Priority 2: Kubelet process arguments
  pod_cidr=$(get_cni_from_kubelet_args)
  if [ -n "$pod_cidr" ]; then
    echo "$pod_cidr|kubelet_args|unknown"
    return 0
  fi
  
  # Priority 3: Flannel data file
  pod_cidr=$(get_cni_from_flannel_data)
  if [ -n "$pod_cidr" ]; then
    echo "$pod_cidr|flannel_data|flannel"
    return 0
  fi
  
  # Priority 4: CNI config files
  local cni_info=""
  cni_info=$(get_cni_from_config_files)
  if [ -n "$cni_info" ]; then
    cni_type=$(echo "$cni_info" | cut -d'|' -f1)
    pod_cidr=$(echo "$cni_info" | cut -d'|' -f2)
    if [ -n "$pod_cidr" ]; then
      echo "$pod_cidr|config_files|$cni_type"
      return 0
    fi
  fi
  
  # No CNI detected
  return 1
}

# helper: convert IP and CIDR prefix to netmask
cidr_to_netmask() {
  local cidr="$1"
  local prefix="${cidr##*/}"
  
  if [ -z "$prefix" ] || [ "$prefix" = "$cidr" ]; then
    echo "255.255.255.255"
    return
  fi
  
  # Complete CIDR to netmask conversion for all sizes 0-32
  case "$prefix" in
    0) echo "0.0.0.0" ;;
    1) echo "128.0.0.0" ;;
    2) echo "192.0.0.0" ;;
    3) echo "224.0.0.0" ;;
    4) echo "240.0.0.0" ;;
    5) echo "248.0.0.0" ;;
    6) echo "252.0.0.0" ;;
    7) echo "254.0.0.0" ;;
    8) echo "255.0.0.0" ;;
    9) echo "255.128.0.0" ;;
    10) echo "255.192.0.0" ;;
    11) echo "255.224.0.0" ;;
    12) echo "255.240.0.0" ;;
    13) echo "255.248.0.0" ;;
    14) echo "255.252.0.0" ;;
    15) echo "255.254.0.0" ;;
    16) echo "255.255.0.0" ;;
    17) echo "255.255.128.0" ;;
    18) echo "255.255.192.0" ;;
    19) echo "255.255.224.0" ;;
    20) echo "255.255.240.0" ;;
    21) echo "255.255.248.0" ;;
    22) echo "255.255.252.0" ;;
    23) echo "255.255.254.0" ;;
    24) echo "255.255.255.0" ;;
    25) echo "255.255.255.128" ;;
    26) echo "255.255.255.192" ;;
    27) echo "255.255.255.224" ;;
    28) echo "255.255.255.240" ;;
    29) echo "255.255.255.248" ;;
    30) echo "255.255.255.252" ;;
    31) echo "255.255.255.254" ;;
    32) echo "255.255.255.255" ;;
    *) echo "255.255.255.0" ;; # fallback for invalid input
  esac
}


# helper: get network address from IP and CIDR prefix
get_network_address() {
  local ip="$1"
  local prefix="${2##*/}"
  
  # Simple extraction of network address for common CIDR sizes
  case "$prefix" in
    24)
      echo "$ip" | sed 's/\([^.]*\.[^.]*\.[^.]*\)\..*/\1.0/'
      ;;
    16)
      echo "$ip" | sed 's/\([^.]*\.[^.]*\)\..*/\1.0.0/'
      ;;
    8)
      echo "$ip" | sed 's/\([^.]*\)\..*/\1.0.0.0/'
      ;;
    *)
      # For other prefixes, return the IP itself as fallback
      echo "$ip"
      ;;
  esac
}

# helper: check if subnet is Kubernetes CNI subnet
is_cni_subnet() {
  local cidr="$1"
  
  # Kubernetes CNI Pod CIDRs typically use specific ranges:
  # - 10.0.0.0/8 to 10.255.255.255/32 is too broad, but common CNI patterns are:
  #   * Flannel: 10.x.0.0/24 per node (e.g., 10.1.0.0/24, 10.2.0.0/24)
  #   * Weave: 10.32.0.0/12 by default
  #   * Calico: 10.0.0.0/8 or 192.168.0.0/16
  #   * AWS VPC CNI: Uses VPC CIDR (e.g., 10.0.0.0/16)
  # - 172.16.0.0/12 to 172.31.255.255/32
  # - 192.168.0.0/16
  # 
  # The key insight: if it's a /8, /12, or /16 with a single interface, it's likely management network
  # If it's a /24 or smaller with multiple interfaces per node, it's likely CNI
  #
  # For now, we'll only mark as CNI if it matches known patterns or is clearly a Pod network
  case "$cidr" in
    # Known Kubernetes CNI defaults
    10.0.0.0/8|10.32.0.0/12|172.16.0.0/12|192.168.0.0/16)
      echo "false"  # These are too broad to be certain without more context
      ;;
    # Flannel per-node pattern (10.x.0.0/24)
    10.[0-9]*.0.0/24)
      echo "true"
      ;;
    # Weave pattern (10.32.0.0/12 - handled above, returns false)
    # Calico inline subnets (typically /24, /25, etc with smaller network)
    *)
      # If it's in 10.x.x.x range AND it's a /24 or smaller (/25, /26, /27, /28, /29, /30, /31, /32)
      # AND it has multiple interfaces, it might be CNI
      # But since we can't easily check interface count here, we'll be conservative
      # and only mark obvious CNI patterns
      if echo "$cidr" | grep -qE '^10\.'; then
        # Extract the prefix length
        local prefix="${cidr##*/}"
        # Only mark as CNI if it's /24 or smaller AND not a /16 or /8 (management networks)
        if [ "$prefix" -ge 24 ] && [ "$prefix" -le 32 ]; then
          echo "true"
        else
          echo "false"
        fi
      else
        echo "false"
      fi
      ;;
  esac
}


# helper: collect all subnets and their interfaces
collect_subnets_info() {
  local subnets_json=""
  local subnet_count=0
  local seen_subnets=""
  
  # Iterate through all interfaces and extract their subnets
  for iface_dir in /host-sys/class/net/*; do
    [ -d "$iface_dir" ] || continue
    local ifname="$(basename "$iface_dir")"
    [ "$ifname" = "lo" ] && continue
    
    # Get IP address for this interface
    local ip_info="$(ip -o -4 addr show dev "$ifname" 2>/dev/null | awk '{print $4}')"
    
    if [ -z "$ip_info" ]; then
      continue
    fi
    
    # ip_info format: x.x.x.x/y
    local ip_addr="${ip_info%/*}"
    local cidr_prefix="${ip_info##*/}"
    
    # Get network address and netmask
    local network="$(get_network_address "$ip_addr" "/$cidr_prefix")"
    local netmask="$(cidr_to_netmask "/$cidr_prefix")"
    local cidr="${network}/${cidr_prefix}"
    
    # Check if we've already processed this subnet
    if echo "$seen_subnets" | grep -q "^${cidr}\$"; then
      continue
    fi
    
    # Build subnet JSON
    local subnet_ifaces=""
    local subnet_iface_count=0
    
    # Collect all interfaces on this subnet
    for check_iface_dir in /host-sys/class/net/*; do
      [ -d "$check_iface_dir" ] || continue
      local check_ifname="$(basename "$check_iface_dir")"
      [ "$check_ifname" = "lo" ] && continue
      
      local check_ip_info="$(ip -o -4 addr show dev "$check_ifname" 2>/dev/null | awk '{print $4}')"
      if [ -z "$check_ip_info" ]; then
        continue
      fi
      
      local check_ip="${check_ip_info%/*}"
      local check_prefix="${check_ip_info##*/}"
      local check_network="$(get_network_address "$check_ip" "/$check_prefix")"
      
      # If this interface is on the same subnet
      if [ "$check_network/${check_prefix}" = "$cidr" ]; then
        local iface_obj="{\"name\":\"$(json_escape "$check_ifname")\",\"ip\":\"$(json_escape "$check_ip")\"}"
        if [ "$subnet_iface_count" -gt 0 ]; then
          subnet_ifaces="${subnet_ifaces},"
        fi
        subnet_ifaces="${subnet_ifaces}${iface_obj}"
        subnet_iface_count=$((subnet_iface_count+1))
      fi
    done
    
    # Check if it's a CNI subnet
    local is_cni="$(is_cni_subnet "$cidr")"
    
    local subnet_obj="{\"network_address\":\"$(json_escape "$network")\","
    subnet_obj="$subnet_obj\"netmask\":\"$(json_escape "$netmask")\","
    subnet_obj="$subnet_obj\"cidr\":\"$(json_escape "$cidr")\","
    subnet_obj="$subnet_obj\"interfaces\":[$subnet_ifaces],"
    subnet_obj="$subnet_obj\"interface_count\":$subnet_iface_count,"
    subnet_obj="$subnet_obj\"is_cni_subnet\":$is_cni"
    subnet_obj="$subnet_obj}"
    
    if [ "$subnet_count" -gt 0 ]; then
      subnets_json="${subnets_json},"
    fi
    subnets_json="${subnets_json}${subnet_obj}"
    subnet_count=$((subnet_count+1))
    
    seen_subnets="${seen_subnets}${cidr}"$'\n'
  done
  
  echo "$subnets_json|$subnet_count"
}

# helper: collect all routing tables and rules
collect_routing_info() {
  local routing_obj="{"
  
  # Collect routing rules
  local rules_json=""
  local rule_count=0
  
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    
    # Parse rule: <priority>: from <src> lookup <table>
    local priority="$(echo "$line" | awk '{print $1}' | sed 's/://')"
    local table="$(echo "$line" | awk '{for(i=1;i<=NF;i++) if($i=="lookup") print $(i+1); exit}')"
    
    # Extract condition (everything after "priority:" and trim spaces)
    local condition="$(echo "$line" | sed 's/^[0-9]*:[[:space:]]*//' | sed 's/[[:space:]]*$//')"
    
    if [ -z "$priority" ]; then
      continue
    fi
    
    local rule_obj="{\"priority\":$priority"
    [ -n "$table" ] && rule_obj="$rule_obj,\"table\":\"$(json_escape "$table")\""
    rule_obj="$rule_obj,\"condition\":\"$(json_escape "$condition")\""
    rule_obj="$rule_obj}"
    
    if [ "$rule_count" -gt 0 ]; then
      rules_json="${rules_json},"
    fi
    rules_json="${rules_json}${rule_obj}"
    rule_count=$((rule_count+1))
  done < <(ip rule show 2>/dev/null || true)
  
  # Collect routing tables (main and local)
  local tables_json=""
  local table_count=0
  
  for table_name in main local; do
    local table_routes=""
    local table_route_count=0
    
    while IFS= read -r line; do
      if [ -z "$line" ]; then
        continue
      fi
      
      local dest="$(echo "$line" | awk '{print $1}')"
      local gateway=""
      local device=""
      local metric=""
      
      if echo "$line" | grep -q " via "; then
        gateway="$(echo "$line" | awk '{for(i=1;i<=NF;i++) if($i=="via") print $(i+1); exit}')"
      fi
      
      if echo "$line" | grep -q " dev "; then
        device="$(echo "$line" | awk '{for(i=1;i<=NF;i++) if($i=="dev") print $(i+1); exit}')"
      fi
      
      if echo "$line" | grep -q " metric "; then
        metric="$(echo "$line" | awk '{for(i=1;i<=NF;i++) if($i=="metric") print $(i+1); exit}')"
      fi
      
      if [ -z "$dest" ]; then
        continue
      fi
      
      local route_obj="{\"destination\":\"$(json_escape "$dest")\""
      [ -n "$gateway" ] && route_obj="$route_obj,\"gateway\":\"$(json_escape "$gateway")\""
      [ -n "$device" ] && route_obj="$route_obj,\"device\":\"$(json_escape "$device")\""
      [ -n "$metric" ] && route_obj="$route_obj,\"metric\":$metric"
      route_obj="$route_obj,\"table\":\"$(json_escape "$table_name")\""
      route_obj="$route_obj}"
      
      if [ "$table_route_count" -gt 0 ]; then
        table_routes="${table_routes},"
      fi
      table_routes="${table_routes}${route_obj}"
      table_route_count=$((table_route_count+1))
    done < <(ip -o route show table "$table_name" 2>/dev/null || true)
    
    if [ "$table_route_count" -gt 0 ] || [ "$table_name" = "main" ]; then
      local table_obj="{\"table_name\":\"$(json_escape "$table_name")\",\"routes\":[$table_routes],\"route_count\":$table_route_count}"
      if [ "$table_count" -gt 0 ]; then
        tables_json="${tables_json},"
      fi
      tables_json="${tables_json}${table_obj}"
      table_count=$((table_count+1))
    fi
  done
  
  routing_obj="$routing_obj\"namespace\":\"\","
  routing_obj="$routing_obj\"routing_tables\":[$tables_json],"
  routing_obj="$routing_obj\"routing_rules\":[$rules_json],"
  routing_obj="$routing_obj\"rule_count\":$rule_count,"
  routing_obj="$routing_obj\"table_count\":$table_count"
  
  # Collect subnets information
  local subnets_info="$(collect_subnets_info)"
  local subnets_json="${subnets_info%%|*}"
  local subnet_count="${subnets_info##*|}"
  
  routing_obj="$routing_obj,\"subnets\":[$subnets_json],"
  routing_obj="$routing_obj\"subnet_count\":$subnet_count"
  routing_obj="$routing_obj}"
  
  echo "$routing_obj"
}

# ----- Generic Network Interfaces Collection (Ethernet + InfiniBand) -----
NETWORK_IFACES_JSON=""
NETWORK_IFACES_COUNT=0

# Scan all network interfaces
for iface_dir in /host-sys/class/net/*; do
  [ -d "$iface_dir" ] || continue
  ifname="$(basename "$iface_dir")"
  
  # Skip loopback
  [ "$ifname" = "lo" ] && continue
  
  # Get interface information
  iface_type="$(get_iface_type "$ifname")"
  ip4="$(iface_ipv4 "$ifname")"
  pci_addr="$(get_iface_pci_addr "$ifname")"
  numa_node="$(get_pci_numa_node "$pci_addr")" 
  mac="$(get_iface_mac "$ifname")"
  mtu="$(get_iface_mtu "$ifname")"
  status="$(get_iface_status "$ifname")"
  bond_master="$(iface_bond_master "$ifname")"
  is_bond_slave=false
  [ -n "$bond_master" ] && is_bond_slave=true
  max_speed="$(iface_speed "$ifname")"
  effective_speed="$max_speed"  # TODO: Could be extended to read negotiated speed separately
  metrics="$(get_iface_metrics "$ifname")"
  is_default_route="$(is_default_route_iface "$ifname")"
  associated_routes="$(get_routes_for_iface "$ifname")"
  route_count=0
  if [ -n "$associated_routes" ]; then
    route_count="$(echo "$associated_routes" | grep -o '"destination"' | wc -l)"
  fi
  
  # Build JSON object
  obj="{"
  obj="$obj\"name\":\"$(json_escape "$ifname")\","
  obj="$obj\"type\":\"$(json_escape "$iface_type")\","
  obj="$obj\"ip\":\"$(json_escape "$ip4")\","
  obj="$obj\"mtu\":$mtu,"
  obj="$obj\"mac\":\"$(json_escape "$mac")\","
  obj="$obj\"bond_master\":\"$(json_escape "$bond_master")\","
  obj="$obj\"bond_slave\":$is_bond_slave,"
  obj="$obj\"max_speed\":\"$(json_escape "$max_speed")\","
  obj="$obj\"effective_speed\":\"$(json_escape "$effective_speed")\","
  obj="$obj\"pci_address\":\"$(json_escape "$pci_addr")\","
  obj="$obj\"numa_node\":$numa_node,"
  obj="$obj\"status\":\"$(json_escape "$status")\","
  obj="$obj\"is_default_route\":$is_default_route,"
  obj="$obj\"associated_routes\":[$associated_routes],"
  obj="$obj\"route_count\":$route_count,"
  obj="$obj\"metrics\":$metrics"
  obj="$obj}"
  
  if [ "$NETWORK_IFACES_COUNT" -gt 0 ]; then
    NETWORK_IFACES_JSON="${NETWORK_IFACES_JSON},"
  fi
  NETWORK_IFACES_JSON="${NETWORK_IFACES_JSON}${obj}"
  NETWORK_IFACES_COUNT=$((NETWORK_IFACES_COUNT+1))
done

NETWORK_DETAIL=""
if [ "$NETWORK_IFACES_COUNT" -gt 0 ]; then
  NETWORK_DETAIL="found $NETWORK_IFACES_COUNT interface(s)"
else
  NETWORK_DETAIL="no network interfaces found"
fi

# ----- Mellanox interface inventory -----
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

# Function to get NVMe drive PCI address
get_nvme_pci_addr() {
  local dev="$1"
  local pci_addr=""
  
  # Try to get PCI address from sysfs
  # NVMe devices typically have path like: /sys/devices/pci0000:00/0000:00:1d.0/nvme/nvme0/nvme0n1
  # We need to extract the PCI address (0000:00:1d.0)
  
  if [ -L "/host-sys/class/block/$dev/device" ]; then
    local device_path="$(readlink -f "/host-sys/class/block/$dev/device" 2>/dev/null || true)"
    if [ -n "$device_path" ]; then
      # Try to extract PCI address from path
      # Look for pattern like 0000:00:1d.0 in the device path
      pci_addr=$(echo "$device_path" | grep -o '[0-9a-f]\{4\}:[0-9a-f]\{2\}:[0-9a-f]\{2\}\.[0-9]' | tail -n1)
      
      # If regex didn't work, try alternative method using ls
      if [ -z "$pci_addr" ]; then
        # Get the device directory and look for the pci address format
        local dev_dir="$(dirname "$device_path")"
        # Walk up the sysfs tree to find a directory matching PCI address format
        while [ -n "$dev_dir" ] && [ "$dev_dir" != "/" ] && [ "$dev_dir" != "/host" ]; do
          local dir_name="$(basename "$dev_dir")"
          if echo "$dir_name" | grep -q '^[0-9a-f]\{4\}:[0-9a-f]\{2\}:[0-9a-f]\{2\}\.[0-9]'; then
            pci_addr="$dir_name"
            break
          fi
          dev_dir="$(dirname "$dev_dir")"
        done
      fi
    fi
  fi
  
  echo "$pci_addr"
}

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

# Scan for NVMe drives in /dev (only base devices, not partitions)
for nvme_dev in /host/dev/nvme[0-9]n[0-9]; do
  [ -b "$nvme_dev" ] || continue
  
  dev_name="$(basename "$nvme_dev")"
  
  # Skip if this is a partition (contains 'p' followed by digits)
  # Valid: nvme0n1, nvme5n3
  # Invalid: nvme0n1p1, nvme5n1p2
  case "$dev_name" in
    *p[0-9]*) continue ;;  # Skip partitions
  esac
  
  # Get drive information
  serial="$(get_nvme_serial "$dev_name")"
  model="$(get_nvme_model "$dev_name")"
  size="$(get_nvme_size "$dev_name")"
  pci_addr="$(get_nvme_pci_addr "$dev_name")"
  numa_node="$(get_pci_numa_node "$pci_addr")"
  
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
  obj="$obj\"pci_address\":\"$(json_escape "$pci_addr")\","
  obj="$obj\"numa_node\":$numa_node,"
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

# ----- CNI Detection -----
CNI_DETECTED=false
CNI_POD_CIDR=""
CNI_SOURCE=""
CNI_TYPE=""

# Safe CNI detection with error handling
cni_info=""
if command -v detect_cni_pod_cidr >/dev/null 2>&1; then
  cni_info="$(detect_cni_pod_cidr 2>/dev/null || true)"
  if [ -n "$cni_info" ]; then
    CNI_DETECTED=true
    CNI_POD_CIDR=$(echo "$cni_info" | cut -d'|' -f1 2>/dev/null || true)
    CNI_SOURCE=$(echo "$cni_info" | cut -d'|' -f2 2>/dev/null || true)
    CNI_TYPE=$(echo "$cni_info" | cut -d'|' -f3 2>/dev/null || true)
  fi
fi

# ----- Routing Configuration Collection -----
ROUTING_JSON="$(collect_routing_info)"
# Count custom routing rules (exclude default rules: priority 0, 32766, 32767)
CUSTOM_RULES="$(echo "$ROUTING_JSON" | grep -o '"priority":[^,}]*' | grep -v '"priority":\(0\|32766\|32767\)' | wc -l)"
ROUTING_DETAIL="routing configured"
if [ "$CUSTOM_RULES" -eq 0 ]; then
  ROUTING_DETAIL="default routing rules only (no custom rules configured)"
else
  ROUTING_DETAIL="custom routing rules configured ($CUSTOM_RULES rule(s))"
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
printf '"network_interfaces":[%s],' "$NETWORK_IFACES_JSON"
printf '"network_interface_count":%d,' "$NETWORK_IFACES_COUNT"
printf '"network_interface_detail":"%s",' "$(json_escape "$NETWORK_DETAIL")"
printf '"mellanox":%s,' "$MLX_PRESENT"
printf '"mellanox_detail":"%s",' "$(json_escape "Mellanox interface count: $MLX_IFACES_COUNT")"
printf '"mlx_ifaces":[%s],' "$MLX_IFACES_JSON"
printf '"mlx_bonds":[%s],' "$MLX_BONDS_JSON"
printf '"bond_lacp_ok":%s,' "$([ "$BOND_LACP_OK" -eq 1 ] && echo true || echo false)"
printf '"bond_lacp_detail":"%s",' "$(json_escape "$BOND_LACP_DETAIL")"
printf '"network_namespace_routing":%s,' "$ROUTING_JSON"
printf '"routing_detail":"%s",' "$(json_escape "$ROUTING_DETAIL")"
printf '"cni_detection":{"pod_cidr":"%s","source":"%s","cni_type":"%s","detected":%s},' \
  "$(json_escape "$CNI_POD_CIDR")" \
  "$(json_escape "$CNI_SOURCE")" \
  "$(json_escape "$CNI_TYPE")" \
  "$([ "$CNI_DETECTED" = true ] && echo true || echo false)"
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
