# Known Issues and Limitations

This document lists known limitations and issues with kubectl-weka.

## Current Limitations

### Log Streaming

#### Time-Window Buffering

**Issue**: Log timestamps must be within 2 seconds for correct ordering guarantee.

**Details**:
- Real-time streaming uses a 2-second time-window buffer for safety
- Logs appearing >2 seconds apart are guaranteed correct ordering
- Logs within 2-second window may be slightly reordered depending on collection speed

**Workaround**: Logs typically appear within milliseconds, so this is rarely an issue in practice

**Impact**: VERY LOW - logs are correct in >99% of deployments

---

### Air-Gapped Deployments

#### Registry Authentication

**Issue**: Registry authentication supports only limited authentication methods.

**Supported Methods**:
- Username/Password via environment variables (`REG_<REGISTRY_USER`, `REG_<REGISTRY>_PASSWORD`)
- Docker configuration file (`~/.docker/config.json`)

**Not Supported**:
- SSH key authentication
- Mutual TLS (mTLS) certificates
- Bearer token authentication
- Kubernetes secrets
- Other authentication mechanisms

**Details**:
- Registry must be accessible and credentials must be valid
- Uses standard Docker authentication (.docker/config.json or explicit credentials)
- Private registries require explicit credentials setup
- Only basic auth and config file methods are implemented

**Workaround**:
1. Use username/password credentials for registry access
2. Configure credentials in .docker/config.json
3. For SSH/cert-based auth, configure credentials in .docker/config.json first

**Impact**: MEDIUM - affects air-gapped deployments with specialized authentication

---

#### Bundle Size

**Issue**: Large bundles may take significant time to download and upload.

**Details**:
- Multi-architecture bundles (amd64 + arm64) are larger than single-arch
- Bundle includes both component images and Helm charts
- Network bandwidth affects download/upload speed
- Both AMD64 and ARM64 architectures are downloaded by default

**Example Sizes**:
- Single architecture: ~1-6 GB
- Multi-architecture: ~1-12 GB

**Workaround**: Specify only needed architectures via `--architectures` flag
- You may specify also to upload only a particular architecture
- The bundle can be created for both architectures, but later on you can decide which architecture to upload
- Full upload of all architectures might be required if you decide to add it later

**Impact**: LOW - expected behavior, not a defect.

---

#### Multi-Arch Images Show UNKNOWN

**Issue**: In certain registries, multi-architecture images display as UNKNOWN instead of showing supported architectures.

**Root Cause**:
- Some registries don't properly support OCI multi-architecture manifest indexes
- Registry API may not expose architecture information in standard way
- Image manifest format may not include proper architecture metadata
- Docker tar format doesn't preserve architecture index information
- When images are re-uploaded, architecture information from original manifest may be lost

**Details**:
- Multi-architecture images download successfully despite UNKNOWN display
- Images function correctly when loaded and used
- Display issue is cosmetic - functionality is unaffected
- Occurs with certain private registries or older registry versions

**Workaround**:
1. Verify images work despite UNKNOWN display (they should)
2. Use registries with full OCI manifest support
3. Update registry if using older version
4. Document architecture separately if needed

**Impact**: LOW - Images work correctly despite UNKNOWN display

---

#### Progress Display in Pipes/Redirects

**Issue**: Progress bars may not display correctly when output is piped or redirected.

**Details**:
- Progress rendering uses carriage return (\r) for in-place updates
- Piped output (|) or file redirection (>) disables TTY features
- Progress will appear as multiple lines instead of single updating bar

**Workaround**: Run commands directly in terminal, not in pipes

**Impact**: LOW - progress is still accurate, just displayed differently

---

## Helm Chart Updates

### Comments Lost in values.yaml

**Issue**: When downloading Helm charts and storing them in the bundle, YAML comments in values.yaml are lost.

**Details**:
- Charts are extracted and re-packaged during bundle creation
- YAML parsing and re-serialization removes comments
- Comments are not preserved in the bundled charts
- When charts are extracted from bundle for upload, comments are not restored

**Impact**:
- Bundled chart values files contain no inline documentation
- Users must refer to original chart documentation for parameter explanations
- Custom override values files generated during upload also lack comments
- Severity: LOW - functionality is not affected, only documentation

**Workaround**:
1. Keep original chart documentation available
2. Use helm-docs or similar tools to generate documentation separately
3. Refer to original WEKA chart repository for parameter documentation
4. Comments can be manually re-added if modifying bundled charts

**Future Improvement**:
- Store comments separately during bundle creation
- Restore comments when extracting charts from bundle
- Preserve YAML structure during chart repackaging

---

### Nested Value Handling

**Issue**: Complex nested Helm values may not update correctly if structure is non-standard.

**Details**:
- Value updating assumes standard Helm values structure
- Highly customized charts with unusual nesting may not work
- YAML anchors and aliases are preserved as-is

**Workaround**:
1. Manually update Helm values for complex charts
2. Use override values files instead of modifying chart archives
3. Validate updated values with `helm lint`

**Impact**: LOW - affects non-standard Helm charts only

---

### Chart Compatibility

**Issue**: Chart updates only support specific image path patterns.

**Details**:
- Pre-defined image paths are updated (csi.image, taskmon.defaultImage, etc.)
- Custom image paths not in the predefined list are not updated
- Charts with non-standard image specifications may need manual updates

**Workaround**:
1. Check if custom paths need manual update
2. Use override values files for custom paths
3. Update charts manually for non-standard image locations

**Impact**: LOW - most WEKA charts follow standard patterns

---

## Kubernetes Compatibility

### RBAC Requirements

**Issue**: Plugin requires appropriate Kubernetes RBAC permissions.

**Details**:
- Viewing resources requires list/get permissions
- Creating pods (for preflight checks) requires pod creation permission
- Some checks require cluster-admin or elevated permissions

**Workaround**:
1. Grant appropriate ClusterRole to plugin service account
2. For air-gapped operations, may need special RBAC

**Impact**: MEDIUM - expected, properly documented

---

### Network Policy Impact

**Issue**: Strict network policies may block preflight check pods.

**Details**:
- Preflight nodes creates temporary pods for validation
- Network policies may block these pods from needed network access
- Pod-to-host communication may be restricted

**Workaround**:
1. Whitelist preflight check pods in network policy
2. Use network policy exemptions for validation
3. Temporarily relax policies during validation

**Impact**: MEDIUM - affects clusters with strict security

---

## Unsupported Configurations

### Not Supported: Kubernetes < 1.24

- Plugin uses features from Kubernetes 1.24+
- May not work on older clusters
- Not tested on Kubernetes < 1.24

### Not Supported: Windows Control Planes

- Plugin is designed for Linux nodes
- Windows nodes cannot run WEKA components
- Validation checks skip Windows nodes

### Not Supported: Custom CSI Driver Paths

- CSI driver detection assumes standard naming conventions
- Custom CSI drivers with non-standard paths may not be detected
- Use manual inspection for custom setups

---

## Troubleshooting Common Issues

### "No resources found" when running get commands

**Solution**:
1. Check you're looking in the right namespace: `kubectl weka get cluster-instances -A`
2. Verify WEKA CRDs are installed: `kubectl get crds | grep weka`
3. Verify resources actually exist: `kubectl get wekacluster -A`

### Preflight checks timeout

**Solution**:
1. Check if nodes are NotReady: `kubectl get nodes`
2. Use node selector to limit checks: `kubectl weka preflight nodes --node-selector role=storage`
3. Ensure pod can reach nodes (network/firewall issues)

### Air-gapped download fails

**Solution**:
1. Verify internet connectivity: `curl -I https://quay.io`
2. Check registry credentials are valid
3. Verify you have permission to pull WEKA images
4. Check available disk space for bundle

### Air-gapped upload fails

**Solution**:
1. Verify registry is accessible: `curl -I https://registry.internal.com:5000`
2. Check credentials have push permission
3. Verify bundle file is intact: `sha256sum -c bundle.tar.gz.sha256`
4. Check registry storage space

### Logs show as "out of order"

**Solution**:
1. This typically indicates clock skew between nodes
2. Verify NTP is working: `ntpstat` or `chronyc tracking`
3. Sync node clocks
4. Note: 2-second time-window handles minor skew automatically

---

## Getting Help

If you encounter an issue not listed here:

1. Check if you're using the latest version: `kubectl weka version`
2. Review your Kubernetes version: `kubectl version`
3. Check your RBAC permissions
4. Review network connectivity
5. Collect a support bundle: `kubectl weka support-bundle all --case-id YOUR_CASE`
6. Open an issue on GitHub with:
   - kubectl-weka version
   - Kubernetes version
   - Error message or unexpected behavior
   - Steps to reproduce
   - Support bundle (if available)

---

**Last Updated**: April 2026

**Compatibility**: kubectl-weka v0.2.0+

