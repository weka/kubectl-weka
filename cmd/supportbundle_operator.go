package cmd

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"path/filepath"

	"github.com/spf13/cobra"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var supportBundleOperatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "Collect operator logs and resources",
	Long: `Collects diagnostic information for the Weka operator including:
  - Operator controller manager logs
  - Node-agent pod logs
  - WekaPolicy resources
  - Jobs created by policies`,
	RunE: runSupportBundleOperator,
}

func init() {
	supportBundleCmd.AddCommand(supportBundleOperatorCmd)

	supportBundleOperatorCmd.Flags().StringVar(&supportBundleCaseID, "case-id", "", "Case ID (Salesforce/Jira) to include in bundle name")
	supportBundleOperatorCmd.Flags().StringVarP(&supportBundleOutput, "output", "o", ".", "Output directory for the support bundle archive")
	supportBundleOperatorCmd.Flags().StringVarP(&supportBundleNamespace, "namespace", "n", "weka-operator-system", "Namespace where the operator is running")
	supportBundleOperatorCmd.Flags().BoolVar(&supportBundleIncludeSensitive, "include-sensitive-data", false, "Include sensitive data such as Secrets and credentials (⚠️  INSECURE - use with caution)")

	supportBundleOperatorCmd.SilenceUsage = true
}

func runSupportBundleOperator(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args
	return runSupportBundleByMode(ModeOperator, "", supportBundleNamespace, supportBundleAllNS)
}

// OperatorLogsCollector collects logs from operator controller manager pods
type OperatorLogsCollector struct{}

func (c *OperatorLogsCollector) Name() string {
	return "Operator Logs"
}

func (c *OperatorLogsCollector) Start(ctx context.Context) {
	logger := getLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "operator logs, pod descriptions")
}

func (c *OperatorLogsCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := getLogger(ctx)
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "warnings", len(result.Warnings))
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

func (c *OperatorLogsCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := getLogger(ctx)
	logger.Debug("=== OperatorLogsCollector Debug Mode", "enabled", supportBundleDebug)

	// Collect operator controller manager logs
	operatorNS := "weka-operator-system"
	ns := getNamespace(ctx)
	if ns != "" {
		operatorNS = ns
	}

	clients := getClients(ctx)

	// List operator pods using cached controller-runtime client
	var pods corev1.PodList
	listOpts := []crclient.ListOption{
		crclient.InNamespace(operatorNS),
		crclient.MatchingLabels{
			"app":                          "weka-operator",
			"app.kubernetes.io/component":  "weka-operator",
			"app.kubernetes.io/created-by": "weka-operator",
			"control-plane":                "controller-manager",
		},
	}

	err := clients.CRClient.List(ctx, &pods, listOpts...)
	if err != nil {
		return CollectorResult{Status: StatusFailure, Error: fmt.Errorf("failed to list operator pods: %w", err), Warnings: warnings}
	}

	bundlePath := getBundlePath(ctx)
	for _, pod := range pods.Items {
		logs, _, err := collectPodLogs(ctx, clients, operatorNS, pod.Name, "", nil)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to collect logs from pod %s: %v", pod.Name, err))
			logger.Debug("⚠️  Failed to collect logs from pod", "pod", pod.Name, "error", err)
			continue
		}

		for containerName, logContent := range logs {
			filePath := filepath.Join("operator", "logs", fmt.Sprintf("%s_%s.log", pod.Name, containerName))
			if err := writeToFile(bundlePath, filePath, logContent); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to write log file for %s/%s: %v", pod.Name, containerName, err))
				logger.Debug("⚠️  Failed to write log file", "pod", pod.Name, "container", containerName, "error", err)
				continue
			}
			filesCreated = append(filesCreated, filePath)
			logger.Debug("✓ Collected logs", "pod", pod.Name, "container", containerName, "bytes", len(logContent))
		}

		previousLogs, previousUnavailable, err := collectPodLogs(ctx, clients, operatorNS, pod.Name, "", &corev1.PodLogOptions{Previous: true})
		if err != nil {
			// This is a real error (not just "previous logs don't exist")
			warnings = append(warnings, fmt.Sprintf("failed to collect previous logs from pod %s: %v", pod.Name, err))
			logger.Debug("⚠️  Failed to collect previous logs", "pod", pod.Name, "error", err)
		} else if previousUnavailable {
			// Previous logs don't exist - this is normal for pods that haven't restarted
			logger.Debug("Previous logs unavailable (container hasn't restarted)", "pod", pod.Name)
		} else {
			// Successfully collected previous logs
			for containerName, logContent := range previousLogs {
				if logContent != "" {
					prevFilePath := filepath.Join("operator", "logs", fmt.Sprintf("%s_%s.previous.log", pod.Name, containerName))
					if err := writeToFile(bundlePath, prevFilePath, logContent); err != nil {
						warnings = append(warnings, fmt.Sprintf("failed to write previous log file for %s/%s: %v", pod.Name, containerName, err))
						logger.Debug("⚠️  Failed to write previous log file", "pod", pod.Name, "container", containerName, "error", err)
						continue
					}
					filesCreated = append(filesCreated, prevFilePath)
					logger.Debug("✓ Collected previous logs", "pod", pod.Name, "container", containerName, "bytes", len(logContent))
				}
			}
		}

		desc, err := collectPodDescription(ctx, clients, operatorNS, pod.Name)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to describe pod %s: %v", pod.Name, err))
			logger.Debug("⚠️  Failed to describe pod", "pod", pod.Name, "error", err)
		} else {
			filePath := filepath.Join("operator", "pods", fmt.Sprintf("%s_describe.yaml", pod.Name))
			if err := writeToFile(bundlePath, filePath, desc); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to write pod description for %s: %v", pod.Name, err))
				logger.Debug("⚠️  Failed to write pod description", "pod", pod.Name, "error", err)
				continue
			}
			filesCreated = append(filesCreated, filePath)
			logger.Debug("✓ Collected pod description", "pod", pod.Name)
		}
	}

	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{Status: status, FilesCreated: filesCreated, Warnings: warnings}
}

// OperatorNodeAgentLogsCollector collects logs from node-agent pods
type OperatorNodeAgentLogsCollector struct{}

func (c *OperatorNodeAgentLogsCollector) Name() string {
	return "Operator Node-Agent Logs"
}

func (c *OperatorNodeAgentLogsCollector) Start(ctx context.Context) {
	logger := getLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "node-agent pod logs")
}

func (c *OperatorNodeAgentLogsCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := getLogger(ctx)
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "warnings", len(result.Warnings))
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

func (c *OperatorNodeAgentLogsCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := getLogger(ctx)
	logger.Debug("=== OperatorNodeAgentLogsCollector Debug Mode", "enabled", supportBundleDebug)

	operatorNS := "weka-operator-system"
	ns := getNamespace(ctx)
	if ns != "" {
		operatorNS = ns
	}

	clients := getClients(ctx)

	// List node-agent pods using cached controller-runtime client
	var nodeAgentPods corev1.PodList
	listOpts := []crclient.ListOption{
		crclient.InNamespace(operatorNS),
		crclient.MatchingLabels{"app.kubernetes.io/component": "weka-node-agent"},
	}

	err := clients.CRClient.List(ctx, &nodeAgentPods, listOpts...)
	if err != nil {
		return CollectorResult{Status: StatusFailure, Error: fmt.Errorf("failed to list node-agent pods: %w", err), Warnings: warnings}
	}

	if len(nodeAgentPods.Items) == 0 {
		warnings = append(warnings, "no node-agent pods found")
		logger.Debug("⚠️  No node-agent pods found")
		return CollectorResult{Status: StatusSuccess, FilesCreated: filesCreated, Warnings: warnings}
	}

	// Extract pod names for parallel collection
	var podNames []string
	for _, pod := range nodeAgentPods.Items {
		podNames = append(podNames, pod.Name)
	}

	// Collect logs from all pods in parallel
	filesCreated, warnings = CollectPodLogsParallel(ctx, clients, operatorNS, podNames, "operator/node-agent/logs", 5)

	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{Status: status, FilesCreated: filesCreated, Warnings: warnings}
}

// OperatorResourcesCollector collects WekaPolicy and related Job resources
type OperatorResourcesCollector struct{}

func (c *OperatorResourcesCollector) Name() string {
	return "Operator Resources"
}

func (c *OperatorResourcesCollector) Start(ctx context.Context) {
	logger := getLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "WekaPolicy resources, jobs")
}

func (c *OperatorResourcesCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := getLogger(ctx)
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "warnings", len(result.Warnings))
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}

func (c *OperatorResourcesCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string

	logger := getLogger(ctx)
	logger.Debug("=== OperatorResourcesCollector Debug Mode", "enabled", supportBundleDebug)

	var policies wekaapi.WekaPolicyList
	listOpts := []crclient.ListOption{}
	ns := getNamespace(ctx)
	if !getAllNamespaces(ctx) && ns != "" {
		listOpts = append(listOpts, crclient.InNamespace(ns))
	}

	clients := getClients(ctx)
	if err := clients.CRClient.List(ctx, &policies, listOpts...); err != nil {
		return CollectorResult{Status: StatusFailure, Error: fmt.Errorf("failed to list WekaPolicy resources: %w", err), Warnings: warnings}
	}

	bundlePath := getBundlePath(ctx)
	collectSensitive := getCollectSensitiveData(ctx)
	for _, policy := range policies.Items {
		logger.Debug("✓ Processing WekaPolicy", "namespace", policy.Namespace, "name", policy.Name)
		yaml, err := collectObjectAsYAMLWithSensitiveData(&policy, collectSensitive)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to marshal policy %s/%s: %v", policy.Namespace, policy.Name, err))
			logger.Debug("⚠️  Failed to marshal policy", "namespace", policy.Namespace, "name", policy.Name, "error", err)
			continue
		}

		filePath := filepath.Join("operator", "policies", generateSafeFileName("WekaPolicy", policy.Namespace, policy.Name, "yaml"))
		if err := writeToFile(bundlePath, filePath, yaml); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to write policy file for %s/%s: %v", policy.Namespace, policy.Name, err))
			logger.Debug("⚠️  Failed to write policy file", "namespace", policy.Namespace, "name", policy.Name, "error", err)
			continue
		}
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected WekaPolicy", "namespace", policy.Namespace, "name", policy.Name)

		jobs, err := clients.Clientset.BatchV1().Jobs(policy.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("weka-policy=%s", policy.Name),
		})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to list jobs for policy %s/%s: %v", policy.Namespace, policy.Name, err))
			logger.Debug("⚠️  Failed to list jobs", "namespace", policy.Namespace, "name", policy.Name, "error", err)
			continue
		}

		for _, job := range jobs.Items {
			jobYaml, err := collectObjectAsYAMLWithSensitiveData(&job, collectSensitive)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to marshal job %s/%s: %v", job.Namespace, job.Name, err))
				logger.Debug("⚠️  Failed to marshal job", "namespace", job.Namespace, "name", job.Name, "error", err)
				continue
			}
			jobPath := filepath.Join("operator", "policy-jobs", generateSafeFileName("Job", job.Namespace, job.Name, "yaml"))
			if err := writeToFile(bundlePath, jobPath, jobYaml); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to write job file for %s/%s: %v", job.Namespace, job.Name, err))
				logger.Debug("⚠️  Failed to write job file", "namespace", job.Namespace, "name", job.Name, "error", err)
				continue
			}
			filesCreated = append(filesCreated, jobPath)
			logger.Debug("✓ Collected job", "namespace", job.Namespace, "name", job.Name)
		}
	}

	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}

	return CollectorResult{Status: status, FilesCreated: filesCreated, Warnings: warnings}
}
