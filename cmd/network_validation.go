package cmd

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// NetworkValidationResult holds the result of network validation for a single node
type NetworkValidationResult struct {
	NodeName       string
	Status         string // "PASS" or "FAIL"
	Issues         []string
	HasLACP        bool
	InterfaceFound bool
}

// validateNetworkInterfaceOnNodes validates that a specified ethDevice exists on all eligible nodes
// and if it's a bond, validates that it's using LACP (802.3ad mode).
// If hostChecksMap is provided, reuses existing host check data; otherwise creates new pods.
// If failFast is true, returns immediately on first error; otherwise collects all errors.
func validateNetworkInterfaceOnNodes(ctx context.Context, clients *K8sClients, nodes []corev1.Node, ethDevice string, failFast bool, hostChecksMap ...HostChecksMap) error {
	if ethDevice == "" {
		return nil // No validation needed
	}

	fmt.Printf("\n=== Validating Network Interface '%s' ===\n", ethDevice)

	var nodeResults []NetworkValidationResult
	var allErrors []string

	// If hostChecksMap is provided, use it; otherwise create new host checks
	if len(hostChecksMap) > 0 && len(hostChecksMap[0]) > 0 {
		// Reuse existing host checks data
		existingChecks := hostChecksMap[0]
		for nodeName, hostCheck := range existingChecks {
			nodeResult := NetworkValidationResult{
				NodeName: nodeName,
				Status:   "PASS",
				Issues:   []string{},
			}

			found := false

			// Check if ethDevice is a regular Mellanox interface
			for _, iface := range hostCheck.MlxIfaces {
				if iface.Name == ethDevice {
					found = true
					nodeResult.InterfaceFound = true
					break
				}
			}

			// Check if ethDevice is a bond
			if !found {
				for _, bond := range hostCheck.MlxBonds {
					if bond.Name == ethDevice {
						found = true
						nodeResult.InterfaceFound = true

						// Validate LACP if this is a bond
						if !hostCheck.BondLACPOk {
							nodeResult.Status = "FAIL"
							nodeResult.Issues = append(nodeResult.Issues, fmt.Sprintf("Bond not using LACP: %s", hostCheck.BondLACPDetail))
						} else {
							nodeResult.Status = "PASS"
							nodeResult.HasLACP = true
						}
						break
					}
				}
			}

			if !found {
				nodeResult.Status = "FAIL"
				nodeResult.Issues = append(nodeResult.Issues, fmt.Sprintf("Interface '%s' not found", ethDevice))
			}

			nodeResults = append(nodeResults, nodeResult)
		}
	} else {
		// Fall back to creating host checks if no map provided
		fmt.Printf("Creating pods to verify network configuration...\n")

		// Run host checks via pods to get network interface information
		opts := HostCheckOptions{
			Verbose:             false, // Less verbose for network validation
			CleanupInBackground: false, // Wait for cleanup
			Timeout:             2 * time.Minute,
		}

		hostChecksResult, err := RunHostChecks(ctx, nodes, opts)
		if err != nil {
			return fmt.Errorf("failed to run hostchecks: %w", err)
		}

		// Process results from the hostChecksMap
		for nodeName, hostCheck := range hostChecksResult {
			nodeResult := NetworkValidationResult{
				NodeName: nodeName,
				Status:   "PASS",
				Issues:   []string{},
			}

			found := false

			// Check if ethDevice is a regular Mellanox interface
			for _, iface := range hostCheck.MlxIfaces {
				if iface.Name == ethDevice {
					found = true
					nodeResult.InterfaceFound = true
					break
				}
			}

			// Check if ethDevice is a bond
			if !found {
				for _, bond := range hostCheck.MlxBonds {
					if bond.Name == ethDevice {
						found = true
						nodeResult.InterfaceFound = true

						// Validate LACP if this is a bond
						if !hostCheck.BondLACPOk {
							nodeResult.Status = "FAIL"
							nodeResult.Issues = append(nodeResult.Issues, fmt.Sprintf("Bond not using LACP: %s", hostCheck.BondLACPDetail))
						} else {
							nodeResult.Status = "PASS"
							nodeResult.HasLACP = true
						}
						break
					}
				}
			}

			if !found {
				nodeResult.Status = "FAIL"
				nodeResult.Issues = append(nodeResult.Issues, fmt.Sprintf("Interface '%s' not found", ethDevice))
			}

			nodeResults = append(nodeResults, nodeResult)
		}
	}

	// Print detailed results for each node
	fmt.Printf("\nNode-by-Node Validation Results:\n")
	for _, nr := range nodeResults {
		if nr.Status == "PASS" {
			fmt.Printf("  ✓ %s - Interface OK", nr.NodeName)
			if isBond(ethDevice) && nr.HasLACP {
				fmt.Printf(" (LACP enabled)")
			}
			fmt.Printf("\n")
		} else {
			fmt.Printf("  ✗ %s\n", nr.NodeName)
			for _, issue := range nr.Issues {
				fmt.Printf("      - %s\n", issue)
				allErrors = append(allErrors, fmt.Sprintf("%s: %s", nr.NodeName, issue))
			}
			// If fail-fast is enabled, return immediately on first error
			if failFast {
				return fmt.Errorf("network validation failed on node %s: %s", nr.NodeName, nr.Issues[0])
			}
		}
	}

	// Print summary and task list if there are errors
	passCount := 0
	failCount := 0
	for _, nr := range nodeResults {
		if nr.Status == "PASS" {
			passCount++
		} else {
			failCount++
		}
	}

	fmt.Printf("\nValidation Summary: %d/%d nodes passed\n", passCount, len(nodeResults))

	if failCount > 0 {
		fmt.Printf("\n❌ Validation Failed - Task List:\n")
		fmt.Printf("Required Actions:\n")

		// Analyze error types and provide task list
		hasInterfaceErrors := false
		hasLACPErrors := false

		for _, nr := range nodeResults {
			if nr.Status == "FAIL" {
				for _, issue := range nr.Issues {
					if isInterfaceNotFoundError(issue) {
						hasInterfaceErrors = true
					}
					if isLACPError(issue) {
						hasLACPErrors = true
					}
				}
			}
		}

		if hasInterfaceErrors {
			fmt.Printf("\n1. Add Network Interface to Nodes:\n")
			for _, nr := range nodeResults {
				if nr.Status == "FAIL" && !nr.InterfaceFound {
					fmt.Printf("   - Configure '%s' on node: %s\n", ethDevice, nr.NodeName)
				}
			}
			fmt.Printf("   Verify interface is present: ip link show %s\n", ethDevice)
		}

		if hasLACPErrors {
			fmt.Printf("\n2. Configure Bond with LACP (802.3ad):\n")
			for _, nr := range nodeResults {
				if nr.Status == "FAIL" && nr.InterfaceFound && !nr.HasLACP {
					fmt.Printf("   - Update bond configuration on: %s\n", nr.NodeName)
				}
			}
			fmt.Printf("   Required: Set bond mode to '802.3ad' (LACP)\n")
			fmt.Printf("   Current bond configuration: cat /proc/net/bonding/%s\n", ethDevice)
		}

		fmt.Printf("\n3. Verify Configuration:\n")
		fmt.Printf("   - Run this command again to validate: kubectl weka plan cluster <yaml>\n")

		fmt.Printf("\nFailed Nodes Summary:\n")
		for _, err := range allErrors {
			fmt.Printf("   ✗ %s\n", err)
		}

		return fmt.Errorf("network validation failed on %d nodes", failCount)
	}

	fmt.Printf("\n✅ Network Interface Validation Passed\n")
	fmt.Printf("   ✓ ethDevice '%s' found on all %d nodes\n", ethDevice, passCount)
	if isBond(ethDevice) {
		fmt.Printf("   ✓ Bond '%s' uses LACP (802.3ad) mode on all nodes\n", ethDevice)
	}

	return nil
}

// isInterfaceNotFoundError checks if an error is about missing interface
func isInterfaceNotFoundError(issue string) bool {
	return contains(issue, "not found")
}

// isLACPError checks if an error is about LACP configuration
func isLACPError(issue string) bool {
	return contains(issue, "LACP") || contains(issue, "802.3ad")
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// isBond checks if the interface name looks like a bond interface
func isBond(ifname string) bool {
	if len(ifname) < 5 {
		return false
	}
	return ifname[:4] == "bond"
}
