package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	getCSIDriversOnlyHelm     bool
	getCSIDriversOnlyOperator bool
	getCSIDriversWide         bool
	flagCSIDriversOutput      string
	flagCSIDriversNoHeaders   bool
)

var getCSIDriversCmd = &cobra.Command{
	Use:   "csi-drivers [DRIVER_NAME]",
	Short: "Display CSI driver deployment information (controller and node components)",
	Long: `Lists CSI driver deployments with their controller and node components.

Arguments:
  DRIVER_NAME      Optional: Show only a specific CSI driver by name

Filters:
  --only-helm        Show only CSI drivers installed via Helm chart
  --only-operator    Show only CSI drivers installed by Weka operator
  --wide, -w         Show additional columns (PVs, PVCs, Bound PVs)

Columns (default):
  CSI DRIVER       - CSI driver name
  MANAGED BY       - Installation method (Helm or weka-operator)
  NAMESPACE        - Namespace where CSI components are deployed
  CONTROLLER       - Controller component deployment name
  NODE DAEMONSET   - Node component daemonset name
  STORAGECLASSES   - Number of StorageClasses that refer to this driver
  AGE              - Time since CSI driver was installed

Wide columns (--wide):
  PVS              - Total number of PersistentVolumes using this driver
  PVCS             - Total number of PersistentVolumeClaims using this driver
  BOUND PVS        - Number of PersistentVolumes in Bound state
`,
	RunE: runGetCSIDrivers,
}

func init() {
	getCmd.AddCommand(getCSIDriversCmd)

	getCSIDriversCmd.Flags().BoolVar(&getCSIDriversOnlyHelm, "only-helm", false, "Only show CSI drivers installed via Helm chart")
	getCSIDriversCmd.Flags().BoolVar(&getCSIDriversOnlyOperator, "only-operator", false, "Only show CSI drivers installed by Weka operator")
	getCSIDriversCmd.Flags().StringVarP(&flagCSIDriversOutput, "output", "o", "", "Output format. Supported: json, yaml, wide, custom-columns=<COLS...>")
	getCSIDriversCmd.Flags().BoolVar(&flagCSIDriversNoHeaders, "no-headers", false, "Don't print headers")
	getCSIDriversCmd.SilenceUsage = true
}

func runGetCSIDrivers(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if strings.Contains(flagCSIDriversOutput, "wide") {
		getCSIDriversWide = true
	}

	// Extract optional driver name argument
	var driverName string
	if len(args) > 0 {
		driverName = args[0]
	}

	driverOutput, err := generateCSIDriversOutput(ctx, KubeClients, getCSIDriversOnlyHelm, getCSIDriversOnlyOperator, getCSIDriversWide, driverName)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), driverOutput)
	_, _ = fmt.Fprintln(cmd.OutOrStdout()) // Add newline after table
	return nil
}

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
	CreationTime       metav1.Time
}

// generateCSIDriversOutput generates the CSI driver output for printing
func generateCSIDriversOutput(ctx context.Context, clients *K8sClients, onlyHelm, onlyOperator, wide bool, driverName string) (string, error) {
	crClient := clients.CRClient

	// List all CSIDriver resources (cluster-wide, non-namespaced)
	var csiDriverList storagev1.CSIDriverList
	if err := crClient.List(ctx, &csiDriverList); err != nil {
		return "", fmt.Errorf("failed to list CSIDrivers: %w", err)
	}

	// Filter for weka.io CSI drivers
	var wekaCsiDrivers []storagev1.CSIDriver
	for _, driver := range csiDriverList.Items {
		if isWekaCSI(driver.Name) {
			// If driverName is specified, only include matching driver
			if driverName != "" && driver.Name != driverName {
				continue
			}
			wekaCsiDrivers = append(wekaCsiDrivers, driver)
		}
	}

	// List all deployments across all namespaces
	var deploymentList appsv1.DeploymentList
	if err := crClient.List(ctx, &deploymentList); err != nil {
		return "", fmt.Errorf("failed to list deployments: %w", err)
	}

	// List all daemonsets across all namespaces
	var daemonsetList appsv1.DaemonSetList
	if err := crClient.List(ctx, &daemonsetList); err != nil {
		return "", fmt.Errorf("failed to list daemonsets: %w", err)
	}

	// List all StorageClasses
	var storageClassList storagev1.StorageClassList
	if err := crClient.List(ctx, &storageClassList); err != nil {
		return "", fmt.Errorf("failed to list StorageClasses: %w", err)
	}

	// Build a map of provisioner -> StorageClass count
	storageClassCountByProvisioner := make(map[string]int)
	for _, sc := range storageClassList.Items {
		if isWekaCSI(sc.Provisioner) {
			storageClassCountByProvisioner[sc.Provisioner]++
		}
	}

	// List all PersistentVolumes and PersistentVolumeClaims if wide format requested
	var pvCountByDriver map[string]int
	var pvcCountByDriver map[string]int
	var boundPvCountByDriver map[string]int

	if wide {
		pvCountByDriver = make(map[string]int)
		pvcCountByDriver = make(map[string]int)
		boundPvCountByDriver = make(map[string]int)

		// List all PersistentVolumes
		var pvList corev1.PersistentVolumeList
		if err := crClient.List(ctx, &pvList); err != nil {
			return "", fmt.Errorf("failed to list PersistentVolumes: %w", err)
		}

		// List all PersistentVolumeClaims
		var pvcList corev1.PersistentVolumeClaimList
		if err := crClient.List(ctx, &pvcList); err != nil {
			return "", fmt.Errorf("failed to list PersistentVolumeClaims: %w", err)
		}

		// Count PVs by CSI driver
		for _, pv := range pvList.Items {
			if pv.Spec.CSI != nil && isWekaCSI(pv.Spec.CSI.Driver) {
				pvCountByDriver[pv.Spec.CSI.Driver]++
				if pv.Status.Phase == corev1.VolumeBound {
					boundPvCountByDriver[pv.Spec.CSI.Driver]++
				}
			}
		}

		// Count PVCs by CSI driver (via StorageClass)
		storageClassToProv := make(map[string]string)
		for _, sc := range storageClassList.Items {
			if isWekaCSI(sc.Provisioner) {
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
	controllersByDriver := make(map[string]*appsv1.Deployment)
	nodesByDriver := make(map[string]*appsv1.DaemonSet)

	// Index deployments by CSI driver name (using CSI_DRIVER_NAME env var)
	for i := range deploymentList.Items {
		deploy := &deploymentList.Items[i]
		driverName := getCSIDriverNameFromDeployment(deploy)
		if driverName != "" && isWekaCSI(driverName) {
			controllersByDriver[driverName] = deploy
		}
	}

	// Index daemonsets by CSI driver name (using CSI_DRIVER_NAME env var)
	for i := range daemonsetList.Items {
		ds := &daemonsetList.Items[i]
		driverName := getCSIDriverNameFromDaemonset(ds)
		if driverName != "" && isWekaCSI(driverName) {
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
		var creationTime metav1.Time

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
		return compareNodeNames(deployments[i].DriverName, deployments[j].DriverName) < 0
	})

	// Build columns
	var columns []TableColumn
	columns = []TableColumn{
		{Name: "CSI DRIVER", VisibleInWide: false},
		{Name: "MANAGED BY", VisibleInWide: false},
		{Name: "NAMESPACE", VisibleInWide: false},
		{Name: "CONTROLLER", VisibleInWide: false},
		{Name: "NODE DAEMONSET", VisibleInWide: false},
		{Name: "STORAGECLASSES", VisibleInWide: false},
		{Name: "PVS", VisibleInWide: true},
		{Name: "PVCS", VisibleInWide: true},
		{Name: "BOUND PVS", VisibleInWide: true},
		{Name: "AGE", VisibleInWide: false, formatFuncs: TableFormatFunctions{humanAge}},
	}

	// Build rows
	var rows []TableRow
	for _, info := range deployments {
		row := TableRow{Values: map[string]interface{}{}}
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
	printer, _ := GetPrinterFromFlags(flagCSIDriversOutput, !flagCSIDriversNoHeaders, nil, wide, 0, TableStyleMinimal)
	var sb strings.Builder
	err := printer.Print(columns, rows, &sb)
	if err != nil {
		return "", err
	}
	return sb.String(), nil
}

// getCSIDriverNameFromDeployment extracts CSI_DRIVER_NAME from deployment's first container
func getCSIDriverNameFromDeployment(deploy *appsv1.Deployment) string {
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
func getCSIDriverNameFromDaemonset(ds *appsv1.DaemonSet) string {
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
	case *appsv1.Deployment:
		labels = v.GetLabels()
	case *appsv1.DaemonSet:
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
