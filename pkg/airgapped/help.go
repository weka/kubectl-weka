package airgapped

import (
	"fmt"
)

// HelpAirGapped provides guidance for air-gapped deployments
func HelpAirGapped() error {
	help := `
WEKA Air-Gapped Deployment Guide
==================================

This guide walks through deploying WEKA in an air-gapped (offline) environment.

PREREQUISITES
-----------
1. Two environments:
   - Internet-connected host for downloading images and charts
   - Air-gapped cluster for deployment

2. Access to:
   - Custom Docker registry accessible from air-gapped environment
   - kubectl configured for the target cluster

STEP-BY-STEP PROCESS
-------------------

STEP 1: Download Images and Helm Charts (on internet-connected host)
------
   kubectl weka air-gapped download \
     --weka-version 5.3.0 \
     --operator-version 1.10.0 \
     --csi-version 2.8.2

   This creates a single tar.gz bundle containing:
   - WEKA Software images (amd64 and arm64)
   - WEKA Operator images (amd64 and arm64)
   - WEKA CSI Driver images (amd64 and arm64)
   - Operator Helm chart (OCI format from Quay.io)
   - CSI Driver Helm chart (from GitHub releases)
   - manifest.json describing all contents

   Output: weka-5.3.0-operator-1.10.0-csi-2.8.2-offline-bundle.tar.gz

STEP 2: Transfer Bundle to Air-Gapped Environment
------
   Transfer the bundle file to air-gapped environment via:
   - Secure copy: scp weka-*.tar.gz user@air-gapped-host:/tmp/
   - Portable media (USB drive, etc.)
   - Approved data transfer mechanism

STEP 3: Upload Images to Custom Registry and Extract Updated Helm Charts
------
   In the air-gapped environment, extract bundle and upload:

   kubectl weka air-gapped upload \
     --bundle /path/to/weka-5.3.0-operator-1.10.0-csi-2.8.2-offline-bundle.tar.gz \
     --registry my-registry.local:5000 \
     --username admin \
     --password <registry-password>

   This performs THREE operations:
   1. Uploads all images to your internal registry
   2. Extracts Helm charts from the bundle
   3. Updates Helm charts with custom registry URLs
   4. Creates local archives with updated charts

   Output:
   - Images uploaded to: my-registry.local:5000/weka/...
   - Updated Operator chart: ./weka-operator-1.10.0.tgz (with updated image URLs)
   - Updated CSI chart: ./weka-csi-2.8.2.tgz (with updated image URLs)
   - WEKA software image URL that needs to be set inside WEKA Custom Resources (CRs) will be printed on screen.

STEP 4: Deploy WEKA Components
------
   Use the updated Helm charts (already containing correct image URLs):

   A. Create namespace:
      kubectl create namespace weka-system

   B. Deploy WEKA Operator:
      helm upgrade --install weka-operator ./weka-operator-1.10.0.tgz \
        -n weka-operator-system [additional_flags_if_required...]

   C. Deploy WEKA software (using WekaCluster, WekaClient etc. custom resources):
      kubectl apply -f ./weka-cluster.yaml -n weka-system

   D. Create CSI namespace:
      kubectl create namespace weka-csi-system

   E. Deploy WEKA CSI Driver:
      helm install weka-csi ./weka-csi-2.8.2.tgz \
        -n weka-csi-system

WORKFLOW SUMMARY
---------

Online Environment:
   kubectl weka air-gapped download [versions] 
   → weka-*-offline-bundle.tar.gz

Transfer:
   [Copy bundle to air-gapped environment]

Air-Gapped Environment:
   kubectl weka air-gapped upload --bundle [bundle] --registry [url]
   → Images uploaded to registry
   → Updated Helm charts created locally
   
   helm install [updated-charts]
   → Deploy WEKA to cluster

TROUBLESHOOTING
---------------
1. Download fails:
   - Ensure internet connectivity
   - Verify version numbers are correct
   - Check available disk space
   - For Operator: Uses OCI format from quay.io
   - For CSI: Uses GitHub releases

2. Upload fails:
   - Verify custom registry is accessible
   - Check registry credentials
   - Ensure registry has sufficient storage
   - For specific architecture only:
     kubectl weka air-gapped upload --bundle [...] --registry [...] --architecture arm64

3. Deployment fails:
   - Verify custom registry URL in charts matches actual registry
   - Check cluster has network access to custom registry
   - Verify images are present: kubectl get images -o wide
   - Check pod logs: kubectl logs -f [pod-name]
   - Check pod status: kubectl describe pod [pod-name]

4. Image pull errors:
   - Verify image names in cluster match uploaded names
   - Check registry credentials are configured
   - Confirm image architecture matches node architecture

ADVANCED SCENARIOS
---------

Download Specific Architecture Only:
   kubectl weka air-gapped download \
     --weka-version 5.3.0 \
     --operator-version 1.10.0 \
     --architecture arm64
   
   (Creates bundle with only arm64 images)

Upload Specific Architecture:
   kubectl weka air-gapped upload \
     --bundle [...] \
     --registry my-registry:5000 \
     --architecture arm64
   
   (Uploads only arm64 images to registry)

Mix Version and Chart Sources:
   kubectl weka air-gapped download \
     --weka-version 5.3.0 \
     --operator-version 1.10.0 \
     --csi-helm-path ./my-local-csi-chart.tgz
   
   (Downloads WEKA and Operator, uses provided CSI chart)

Multiple Air-Gapped Clusters:
   1. Download once (Step 1)
   2. Transfer bundle to each cluster (Step 2)
   3. Each cluster uploads to its local registry (Step 3)
      - Each registry gets correct custom URL in charts
   4. Deploy each cluster (Step 4)

Updating Versions:
   1. Download new versions (Step 1)
   2. Transfer to air-gapped environment (Step 2)
   3. Upload to registry (Step 3)
   4. Helm upgrade deployments:
      helm upgrade weka-operator ./new-operator-chart.tgz
      helm upgrade weka-csi ./new-csi-chart.tgz

BUNDLE MANIFEST
---------

Each bundle includes manifest.json describing:
   - Operator version and Helm chart
   - CSI version and Helm chart
   - WEKA software version
   - All included images (with references)
   - Supported architectures
   - Checksums (SHA256)

The upload command uses this manifest to:
   - Extract correct charts
   - Identify images to upload
   - Update charts with registry URLs
   - Verify image integrity

ADDITIONAL RESOURCES
-------------------
- WEKA Documentation: https://docs.weka.io/
- OCI Charts: oci://quay.io/weka.io/helm/weka-operator
- GitHub CSI: https://github.com/weka/csi-wekafs/releases
- Support: support@weka.io

`
	fmt.Print(help)
	return nil
}
