package getters

import (
	"context"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/kubernetes"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/utils"
	v2 "k8s.io/api/apps/v1"
	v3 "k8s.io/api/core/v1"
	"k8s.io/api/storage/v1"
	v4 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"
	"strings"
)

// CSIDriverInfo holds information about a CSI driver deployment
type CSIDriverInfo struct {
	DriverName         string
	ManagedBy          string
	Namespace          string
	ControllerName     string
	ControllerReplicas int32
	NodeDaemonsetName  string
	NodeInstances      int32
	StorageClassCount  int
	PVCount            int
	PVCCount           int
	BoundPVCount       int
	CreationTime       v4.Time
}

// GenerateCSIDriversOutput generates the CSI driver output for printing
func GenerateCSIDriversOutput(ctx context.Context, clients *kubernetes.K8sClients, onlyHelm, onlyOperator bool, targetName string, printerObj printer.ResourcePrinter) (string, error) {
	crClient := clients.CRClient

	// List all CSIDriver resources (cluster-wide, non-namespaced)
	var csiDriverList v1.CSIDriverList
	if err := crClient.List(ctx, &csiDriverList); err != nil {
		return "", fmt.Errorf("failed to list CSIDrivers: %w", err)
	}

	// Filter for weka.io CSI drivers
	var wekaCsiDrivers []v1.CSIDriver
	for _, driver := range csiDriverList.Items {
		if kubernetes.IsWekaCSI(driver.Name) {
			// If targetName is specified, only include matching driver
			if targetName != "" && driver.Name != targetName {
				continue
			}
			wekaCsiDrivers = append(wekaCsiDrivers, driver)
		}
	}

	// List all deployments across all namespaces
	var deploymentList v2.DeploymentList
	if err := crClient.List(ctx, &deploymentList); err != nil {
		return "", fmt.Errorf("failed to list deployments: %w", err)
	}

	// List all daemonsets across all namespaces
	var daemonsetList v2.DaemonSetList
	if err := crClient.List(ctx, &daemonsetList); err != nil {
		return "", fmt.Errorf("failed to list daemonsets: %w", err)
	}

	// List all StorageClasses
	var storageClassList v1.StorageClassList
	if err := crClient.List(ctx, &storageClassList); err != nil {
		return "", fmt.Errorf("failed to list StorageClasses: %w", err)
	}

	// Build a map of provisioner -> StorageClass count
	storageClassCountByProvisioner := make(map[string]int)
	for _, sc := range storageClassList.Items {
		if kubernetes.IsWekaCSI(sc.Provisioner) {
			storageClassCountByProvisioner[sc.Provisioner]++
		}
	}

	// List all PersistentVolumes and PersistentVolumeClaims if wide format requested
	var pvCountByDriver map[string]int
	var pvcCountByDriver map[string]int
	var boundPvCountByDriver map[string]int

	if printerObj.GetOptions().WideOutput {
		pvCountByDriver = make(map[string]int)
		pvcCountByDriver = make(map[string]int)
		boundPvCountByDriver = make(map[string]int)

		// List all PersistentVolumes
		var pvList v3.PersistentVolumeList
		if err := crClient.List(ctx, &pvList); err != nil {
			return "", fmt.Errorf("failed to list PersistentVolumes: %w", err)
		}

		// List all PersistentVolumeClaims
		var pvcList v3.PersistentVolumeClaimList
		if err := crClient.List(ctx, &pvcList); err != nil {
			return "", fmt.Errorf("failed to list PersistentVolumeClaims: %w", err)
		}

		// Count PVs by CSI driver
		for _, pv := range pvList.Items {
			if pv.Spec.CSI != nil && kubernetes.IsWekaCSI(pv.Spec.CSI.Driver) {
				pvCountByDriver[pv.Spec.CSI.Driver]++
				if pv.Status.Phase == v3.VolumeBound {
					boundPvCountByDriver[pv.Spec.CSI.Driver]++
				}
			}
		}

		// Count PVCs by CSI driver (via StorageClass)
		storageClassToProv := make(map[string]string)
		for _, sc := range storageClassList.Items {
			if kubernetes.IsWekaCSI(sc.Provisioner) {
				storageClassToProv[sc.Name] = sc.Provisioner
			}
		}
		for _, pvc := range pvcList.Items {
			if pvc.Spec.StorageClassName != nil {
				if prov, ok := storageClassToProv[*pvc.Spec.StorageClassName]; ok {
					pvcCountByDriver[prov]++
				}
			}
		}
	}

	// Build maps of deployments and daemonsets by CSI driver name
	controllersByDriver := make(map[string]*v2.Deployment)
	nodesByDriver := make(map[string]*v2.DaemonSet)

	// Index deployments by CSI driver name (using CSI_DRIVER_NAME env var)
	for i := range deploymentList.Items {
		deploy := &deploymentList.Items[i]
		driverName := getCSIDriverNameFromDeployment(deploy)
		if driverName != "" && kubernetes.IsWekaCSI(driverName) {
			controllersByDriver[driverName] = deploy
		}
	}

	// Index daemonsets by CSI driver name (using CSI_DRIVER_NAME env var)
	for i := range daemonsetList.Items {
		ds := &daemonsetList.Items[i]
		driverName := getCSIDriverNameFromDaemonset(ds)
		if driverName != "" && kubernetes.IsWekaCSI(driverName) {
			nodesByDriver[driverName] = ds
		}
	}

	// Build deployment info from filtered CSIDriver resources
	var deployments []CSIDriverInfo

	for i := range wekaCsiDrivers {
		driver := &wekaCsiDrivers[i]
		driverName := driver.Name

		// Find controller and node components
		controller, hasController := controllersByDriver[driverName]
		daemonset, hasNode := nodesByDriver[driverName]

		// Determine managed-by and namespace from whichever component exists
		var managedBy string
		var namespace string
		var creationTime v4.Time

		if hasController {
			managedBy = getManagedBy(controller)
			namespace = controller.Namespace
			creationTime = controller.CreationTimestamp
		} else if hasNode {
			managedBy = getManagedBy(daemonset)
			namespace = daemonset.Namespace
			creationTime = daemonset.CreationTimestamp
		} else {
			// If no controller or node found, mark as Unknown
			managedBy = "Unknown"
			namespace = "Unknown"
		}

		// Apply filters
		if (onlyHelm && managedBy != "Helm") || (onlyOperator && managedBy != "weka-operator") {
			continue
		}

		// Build deployment info
		info := CSIDriverInfo{
			DriverName:        driverName,
			ManagedBy:         managedBy,
			Namespace:         namespace,
			StorageClassCount: storageClassCountByProvisioner[driverName],
			PVCount:           pvCountByDriver[driverName],
			PVCCount:          pvcCountByDriver[driverName],
			BoundPVCount:      boundPvCountByDriver[driverName],
		}

		if hasController {
			info.ControllerName = controller.Name
			info.ControllerReplicas = *controller.Spec.Replicas
		}

		if hasNode {
			info.NodeDaemonsetName = daemonset.Name
			info.NodeInstances = daemonset.Status.DesiredNumberScheduled
		}

		info.CreationTime = creationTime
		deployments = append(deployments, info)
	}

	// Sort by driver name (numerically aware)
	sort.Slice(deployments, func(i, j int) bool {
		return kubernetes.CompareNodeNames(deployments[i].DriverName, deployments[j].DriverName) < 0
	})

	// Build columns
	columns := []printer.TableColumn{
		{Name: "CSI DRIVER", VisibleInWide: false},
		{Name: "MANAGED BY", VisibleInWide: false},
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "CONTROLLER", VisibleInWide: false},
		{Name: "NODE DAEMONSET", VisibleInWide: false},
		{Name: "STORAGECLASSES", VisibleInWide: false},
		{Name: "PVS", VisibleInWide: true},
		{Name: "PVCS", VisibleInWide: true},
		{Name: "BOUND PVS", VisibleInWide: true},
		{Name: "AGE", VisibleInWide: false, FormatFuncs: printer.TableFormatFunctions{utils.HumanAge}},
	}

	// Build rows
	var rows []printer.TableRow
	for _, info := range deployments {
		row := printer.TableRow{Values: map[string]interface{}{}}
		row.Values["CSI DRIVER"] = info.DriverName
		row.Values["MANAGED BY"] = info.ManagedBy
		row.Values["NAMESPACE"] = info.Namespace
		row.Values["CONTROLLER"] = info.ControllerName
		row.Values["NODE DAEMONSET"] = info.NodeDaemonsetName
		row.Values["STORAGECLASSES"] = info.StorageClassCount
		row.Values["AGE"] = info.CreationTime.Time
		row.Values["PVS"] = info.PVCount
		row.Values["PVCS"] = info.PVCCount
		row.Values["BOUND PVS"] = info.BoundPVCount
		rows = append(rows, row)
	}
	var sb strings.Builder
	err := printerObj.Print(columns, rows, &sb)
	if err != nil {
		return "", err
	}
	return sb.String(), nil
}

// getCSIDriverNameFromDeployment extracts CSI_DRIVER_NAME from deployment's first container
func getCSIDriverNameFromDeployment(deploy *v2.Deployment) string {
	if deploy.Spec.Template.Spec.Containers == nil || len(deploy.Spec.Template.Spec.Containers) == 0 {
		return ""
	}

	container := &deploy.Spec.Template.Spec.Containers[0]
	if container.Env == nil {
		return ""
	}

	for _, envVar := range container.Env {
		if envVar.Name == "CSI_DRIVER_NAME" {
			return envVar.Value
		}
	}

	return ""
}

// getCSIDriverNameFromDaemonset extracts CSI_DRIVER_NAME from daemonset's first container
func getCSIDriverNameFromDaemonset(ds *v2.DaemonSet) string {
	if ds.Spec.Template.Spec.Containers == nil || len(ds.Spec.Template.Spec.Containers) == 0 {
		return ""
	}

	container := &ds.Spec.Template.Spec.Containers[0]
	if container.Env == nil {
		return ""
	}

	for _, envVar := range container.Env {
		if envVar.Name == "CSI_DRIVER_NAME" {
			return envVar.Value
		}
	}

	return ""
}

// getManagedBy determines how the CSI deployment was installed
func getManagedBy(obj interface{}) string {
	var labels map[string]string

	switch v := obj.(type) {
	case *v2.Deployment:
		labels = v.GetLabels()
	case *v2.DaemonSet:
		labels = v.GetLabels()
	}

	// Check for Helm managed label
	if managedBy, ok := labels["app.kubernetes.io/managed-by"]; ok {
		if managedBy == "Helm" {
			return "Helm"
		}
	}

	// Check for Weka operator label
	if createdBy, ok := labels["app.kubernetes.io/created-by"]; ok {
		if createdBy == "weka-operator" {
			return "weka-operator"
		}
	}

	return "Unknown"
}
