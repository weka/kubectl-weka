# kubectl-weka Developer Guide

This guide explains how to extend `kubectl-weka` with new functionality.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Adding Preflight Checks](#adding-preflight-checks)
  - [Node Preflight Checks](#node-preflight-checks)
  - [Cluster Preflight Checks](#cluster-preflight-checks)
- [Adding Plan Validations](#adding-plan-validations)
  - [WekaCluster Validations](#wekacluster-validations)
  - [WekaClient Validations](#wekaclient-validations)
- [Adding Support Bundle Collectors](#adding-support-bundle-collectors)
- [Adding New Commands](#adding-new-commands)
- [Testing Guidelines](#testing-guidelines)

---

## Architecture Overview

### Project Structure

```
cmd/
├── root.go                          # Root command definition
├── preflight*.go                    # Preflight commands
├── plan*.go                         # Plan commands
├── get*.go                          # Get commands
├── logs*.go                         # Logs commands
├── supportbundle*.go                # Support bundle commands
├── hostcheck*.go                    # Host check modules & registry
├── clustercheck*.go                 # Cluster check modules & registry
├── wekaconfig*.go                   # WEKA config validation modules & registry
└── utils.go                         # Shared utilities
```

### Key Design Patterns

#### 1. **Registry Pattern**
Checks and collectors are registered with central registries:
- `GlobalHostCheckRegistry` – Node-level checks
- `GlobalClusterCheckRegistry` – Cluster-level checks
- `GlobalWekaConfigValidationRegistry` – WEKA resource validations

#### 2. **Module Interface**
All checks implement a standard interface for consistency.

#### 3. **Context-Based Execution**
Collectors receive context with clients, namespace, paths via `context.Context`.

#### 4. **Streaming Output**
Long-running operations use `PreflightOutput` for dual screen+file output.

---

## Adding Preflight Checks

### Node Preflight Checks

Node preflight checks validate individual Kubernetes nodes by deploying temporary pods to inspect node conditions.

#### Step 1: Create a New Module

Create a new file in `cmd/` (e.g., `hostcheck_module_example.go`):

```go
package cmd

import (
	"context"
	"fmt"
)

// ExampleHostCheckModule validates example node configuration
type ExampleHostCheckModule struct{}

func (m *ExampleHostCheckModule) Name() string {
	return "example_check"
}

func (m *ExampleHostCheckModule) FriendlyName() string {
	return "Example Check"
}

func (m *ExampleHostCheckModule) AppliesTo() []string {
	// Specify which commands this module runs in
	return []string{"preflight_nodes"}
}

func (m *ExampleHostCheckModule) Validate(
	ctx context.Context,
	node *corev1.Node,
	hostCheck *HostCheckFacts,
	params map[string]interface{},
) (*HostCheckResult, error) {
	// Access host check facts collected from the node
	// hostCheck contains: OSRelease, KernelVersion, NVMeDrives, etc.
	
	// Perform validation logic
	isValid := true // Your validation logic here
	detailValue := "some detail"
	
	if !isValid {
		return &HostCheckResult{
			ModuleName: m.Name(),
			Status:     "error",
			Data: map[string]interface{}{
				"Issue": "Example check failed",
				"Detail": detailValue,
			},
			ErrorTemplate:               "Example check failed: {{.Detail}}",
			SuggestedResolutionTemplate: "Fix the issue by doing X on node {{.NodeName}}",
		}, nil
	}
	
	// Success case
	return &HostCheckResult{
		ModuleName: m.Name(),
		Status:     "success",
		Data: map[string]interface{}{
			"Detail": detailValue,
		},
		SuccessTemplate: "Example check passed: {{.Detail}}",
	}, nil
}

func (m *ExampleHostCheckModule) SuccessTemplate() string {
	return "{{.FriendlyName}}: {{.Detail}}"
}

func (m *ExampleHostCheckModule) WarningTemplate() string {
	return "{{.FriendlyName}}: ⚠️  {{.Warning}}"
}

func (m *ExampleHostCheckModule) ErrorTemplate() string {
	return "{{.FriendlyName}}: ❌ {{.Issue}}"
}

func (m *ExampleHostCheckModule) SuggestedResolutionTemplate() string {
	return "Run: some command --fix"
}
```

#### Step 2: Register the Module

In `cmd/hostcheck_modules.go`, add to the `init()` function:

```go
func init() {
	// ...existing modules...
	
	// Register your new module
	GlobalHostCheckRegistry.Register(&ExampleHostCheckModule{})
}
```

#### Step 3: Test

```bash
go build -o kubectl-weka .
./kubectl-weka preflight nodes
```

Your new check will automatically run on all nodes.

---

### Template Interpolation

Templates use Go's `text/template` syntax with data from `HostCheckResult.Data`:

```go
Data: map[string]interface{}{
	"Issue": "Low disk space",
	"AvailableGB": 50,
	"RequiredGB": 100,
}

ErrorTemplate: "{{.FriendlyName}}: {{.Issue}} ({{.AvailableGB}}GB available, {{.RequiredGB}}GB required)"
```

**Output:** `Example Check: Low disk space (50GB available, 100GB required)`

---

### Cluster Preflight Checks

Cluster-level checks validate Kubernetes cluster configuration and permissions.

#### Step 1: Create a Cluster Check Module

Create `cmd/clustercheck_module_example.go`:

```go
package cmd

import (
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ExampleClusterCheckModule struct{}

func (m *ExampleClusterCheckModule) Name() string {
	return "example_cluster_check"
}

func (m *ExampleClusterCheckModule) FriendlyName() string {
	return "Example Cluster Check"
}

func (m *ExampleClusterCheckModule) Validate(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	crClient client.Client,
	params map[string]interface{},
) (*ClusterCheckResult, error) {
	// Access cluster information via clientset or CRClient
	
	// Perform validation
	isValid := true
	
	if !isValid {
		return &ClusterCheckResult{
			ModuleName: m.Name(),
			Status:     "error",
			Data: map[string]interface{}{
				"Issue": "Cluster check failed",
			},
			ErrorTemplate:               "{{.Issue}}",
			SuggestedResolutionTemplate: "Fix by running: kubectl apply -f fix.yaml",
		}, nil
	}
	
	return &ClusterCheckResult{
		ModuleName: m.Name(),
		Status:     "success",
		Data: map[string]interface{}{
			"Version": "1.30.0",
		},
		SuccessTemplate: "{{.FriendlyName}}: {{.Version}}",
	}, nil
}

func (m *ExampleClusterCheckModule) SuccessTemplate() string {
	return "✅ {{.FriendlyName}}: Success"
}

func (m *ExampleClusterCheckModule) WarningTemplate() string {
	return "⚠️  {{.FriendlyName}}: {{.Warning}}"
}

func (m *ExampleClusterCheckModule) ErrorTemplate() string {
	return "❌ {{.FriendlyName}}: {{.Issue}}"
}

func (m *ExampleClusterCheckModule) SuggestedResolutionTemplate() string {
	return "Fix the issue"
}
```

#### Step 2: Register the Module

In `cmd/clustercheck_modules.go`:

```go
func init() {
	// ...existing modules...
	GlobalClusterCheckRegistry.Register(&ExampleClusterCheckModule{})
}
```

---

## Adding Plan Validations

Plan validations analyze WEKA resource specifications (WekaCluster, WekaClient) before deployment.

### WekaCluster Validations

#### Step 1: Create a Validation Module

Create `cmd/wekaconfig_module_example.go`:

```go
package cmd

import (
	"context"
	wekaapi "github.com/weka/weka-k8s-api/api/v1alpha1"
)

type ExampleClusterValidationModule struct{}

func (m *ExampleClusterValidationModule) Name() string {
	return "example_cluster_validation"
}

func (m *ExampleClusterValidationModule) FriendlyName() string {
	return "Example Cluster Validation"
}

func (m *ExampleClusterValidationModule) AppliesTo() []WekaConfigType {
	return []WekaConfigType{WekaConfigTypeCluster}
}

func (m *ExampleClusterValidationModule) Validate(
	ctx context.Context,
	config *WekaConfigValidationContext,
) (interface{}, error) {
	cluster := config.Cluster
	
	// Validate cluster specification
	if cluster.Spec.Dynamic == nil {
		return map[string]interface{}{
			"Status": "error",
			"Issue":  "Dynamic template is required",
		}, nil
	}
	
	// Success
	return map[string]interface{}{
		"Status": "success",
		"Detail": "Validation passed",
	}, nil
}

func (m *ExampleClusterValidationModule) SuccessTemplate() string {
	return "✅ {{.FriendlyName}}: {{.Detail}}"
}

func (m *ExampleClusterValidationModule) WarningTemplate() string {
	return "⚠️  {{.FriendlyName}}: {{.Warning}}"
}

func (m *ExampleClusterValidationModule) ErrorTemplate() string {
	return "❌ {{.FriendlyName}}: {{.Issue}}"
}

func (m *ExampleClusterValidationModule) SuggestedResolutionTemplate() string {
	return "Set spec.dynamic in your WekaCluster YAML"
}
```

#### Step 2: Register the Module

In `cmd/wekaconfig_modules.go`:

```go
func init() {
	// ...existing modules...
	GlobalWekaConfigValidationRegistry.Register(&ExampleClusterValidationModule{})
}
```

### WekaClient Validations

Similar to cluster validations, but specify `WekaConfigTypeClient` in `AppliesTo()`:

```go
func (m *ExampleClientValidationModule) AppliesTo() []WekaConfigType {
	return []WekaConfigType{WekaConfigTypeClient}
}

func (m *ExampleClientValidationModule) Validate(
	ctx context.Context,
	config *WekaConfigValidationContext,
) (interface{}, error) {
	client := config.Client
	
	// Validate client specification
	// ...
}
```

---

## Adding Support Bundle Collectors

Support bundle collectors gather diagnostic data and save it to files.

### Step 1: Create a Collector

Create `cmd/supportbundle_example.go`:

```go
package cmd

import (
	"context"
	"fmt"
	"path/filepath"
)

// ExampleCollector collects example diagnostic data
type ExampleCollector struct {
	ResourceName string // Optional: filter by resource name
}

func (c *ExampleCollector) Name() string {
	return "Example Data"
}

func (c *ExampleCollector) Start(ctx context.Context) {
	logger := getLogger(ctx)
	logger.Info("Running collector", "name", c.Name())
	logger.Info("Will collect", "items", "example data files")
}

func (c *ExampleCollector) Collect(ctx context.Context) CollectorResult {
	var filesCreated []string
	var warnings []string
	
	logger := getLogger(ctx)
	clients := getClients(ctx)
	bundlePath := getBundlePath(ctx)
	namespace := getNamespace(ctx)
	allNamespaces := getAllNamespaces(ctx)
	
	// Perform data collection
	logger.Debug("Collecting example data", "namespace", namespace)
	
	// Example: List resources
	// var resourceList SomeResourceList
	// if err := clients.CRClient.List(ctx, &resourceList, options...); err != nil {
	//     return CollectorResult{
	//         Status: StatusFailure,
	//         Error: fmt.Errorf("failed to list resources: %w", err),
	//     }
	// }
	
	// Example: Collect and write data
	exampleData := "collected data content"
	filePath := filepath.Join("example", "data.txt")
	if err := writeToFile(bundlePath, filePath, exampleData); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to write data: %v", err))
		logger.Debug("⚠️  Failed to write file", "error", err)
	} else {
		filesCreated = append(filesCreated, filePath)
		logger.Debug("✓ Collected data", "file", filePath)
	}
	
	// Determine status
	status := StatusSuccess
	if len(warnings) > 0 {
		if len(filesCreated) > 0 {
			status = StatusPartial
		} else {
			status = StatusFailure
		}
	}
	
	return CollectorResult{
		Status:       status,
		FilesCreated: filesCreated,
		Warnings:     warnings,
	}
}

func (c *ExampleCollector) Finish(ctx context.Context, result CollectorResult) {
	logger := getLogger(ctx)
	
	switch result.Status {
	case StatusSuccess:
		logger.Info("Collector finished", "name", c.Name(), "status", "success", "files", len(result.FilesCreated))
	case StatusPartial:
		logger.Warn("Collector finished", "name", c.Name(), "status", "partial", "files", len(result.FilesCreated))
		if len(result.Warnings) > 0 {
			logger.Info("Non-fatal warnings found", "count", len(result.Warnings))
			for _, warning := range result.Warnings {
				logger.Info("Warning", "message", warning)
			}
		}
	case StatusFailure:
		logger.Error("Collector failed", "name", c.Name(), "error", result.Error)
	}
}
```

### Step 2: Register the Collector

In `cmd/supportbundle_common.go`, add to `collectorsByMode()`:

```go
func collectorsByMode(mode CollectionMode, resourceName string) []Collector {
	switch mode {
	case ModeAll:
		return []Collector{
			// ...existing collectors...
			&ExampleCollector{ResourceName: resourceName},
		}
	// ...
	}
}
```

### Collector Best Practices

1. **Use Context Helpers**
   ```go
   logger := getLogger(ctx)
   clients := getClients(ctx)
   bundlePath := getBundlePath(ctx)
   namespace := getNamespace(ctx)
   ```

2. **Handle Errors Gracefully**
   - Non-critical errors → Add to `warnings`, return `StatusPartial`
   - Critical errors → Return `StatusFailure` with `Error` set

3. **Log Progress**
   ```go
   logger.Debug("✓ Collected resource", "namespace", ns, "name", name)
   logger.Debug("⚠️  Failed to collect", "error", err)
   ```

4. **Organize Files**
   ```go
   filepath.Join("category", "subcategory", "filename.ext")
   ```

5. **Use Parallel Collection for Scalability**
   ```go
   logFiles, warnings := CollectPodLogsParallel(ctx, clients, namespace, pods, "logs", 10)
   ```

---

## Adding New Commands

### Step 1: Define the Command

Create `cmd/newcommand.go`:

```go
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "newcommand [args]",
	Short: "Brief description of the command",
	Long:  `Detailed description of what the command does.`,
	Args:  cobra.ExactArgs(1), // or cobra.NoArgs, cobra.MinimumNArgs(1), etc.
	RunE:  runNewCommand,
}

func init() {
	// Add to root command
	rootCmd.AddCommand(newCmd)
	
	// Add flags
	newCmd.Flags().StringVarP(&someFlag, "flag", "f", "default", "Flag description")
	newCmd.SilenceUsage = true
}

var someFlag string

func runNewCommand(cmd *cobra.Command, args []string) error {
	// Command implementation
	fmt.Println("Running new command with arg:", args[0])
	return nil
}
```

### Step 2: Add Tests

Create `cmd/newcommand_test.go`:

```go
package cmd

import (
	"testing"
)

func TestNewCommand(t *testing.T) {
	// Test implementation
}
```

### Command Design Guidelines

1. **Follow kubectl Conventions**
   - Use standard flags: `-n/--namespace`, `-A/--all-namespaces`, `--no-headers`
   - Support `--output` for different formats (table, json, yaml)

2. **Error Handling**
   ```go
   if err != nil {
       return fmt.Errorf("failed to do something: %w", err)
   }
   ```

3. **Silent Usage on Errors**
   ```go
   newCmd.SilenceUsage = true
   ```

4. **Use Color Output**
   ```go
   import "github.com/fatih/color"
   
   green := color.New(color.FgGreen).SprintFunc()
   red := color.New(color.FgRed).SprintFunc()
   fmt.Printf("%s Success\n", green("✓"))
   ```

---

## Testing Guidelines

### Unit Tests

```go
func TestExampleModule(t *testing.T) {
	module := &ExampleModule{}
	
	// Test Name()
	if module.Name() != "example" {
		t.Errorf("Expected name 'example', got '%s'", module.Name())
	}
	
	// Test Validate()
	result, err := module.Validate(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("Expected success, got %s", result.Status)
	}
}
```

### Integration Tests

Require access to a Kubernetes cluster:

```go
func TestPreflight(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// Test against real cluster
}
```

Run with: `go test -v ./...` (skip integration: `go test -short ./...`)

---

## Code Organization Best Practices

### File Naming Conventions

- `<command>_<subcommand>.go` – Command implementations
- `<feature>_module.go` – Individual modules
- `<feature>_modules.go` – Module registration
- `<feature>_registry.go` – Registry implementation
- `<feature>_common.go` – Shared utilities

### Module Registration Pattern

All modules follow this pattern:

1. **Define the interface** (e.g., `HostCheckModule`)
2. **Implement modules** (e.g., `OSHostCheckModule`)
3. **Register in `init()`** (e.g., `GlobalHostCheckRegistry.Register()`)
4. **Registry validates all** (e.g., `ValidateAll()`)

### Context-Based Passing

Use `context.Context` with helper functions:

```go
// Set values
ctx = withClients(ctx, clients)
ctx = withBundlePath(ctx, path)
ctx = withLogger(ctx, logger)

// Get values
clients := getClients(ctx)
bundlePath := getBundlePath(ctx)
logger := getLogger(ctx)
```

---

## Debugging Tips

### Enable Debug Logging

```bash
kubectl weka support-bundle all --debug
```

### Check Module Registration

Add debug output in `init()`:

```go
func init() {
	fmt.Println("Registering ExampleModule")
	GlobalHostCheckRegistry.Register(&ExampleModule{})
}
```

### Test Individual Modules

Create a test command:

```go
func runTestModule(cmd *cobra.Command, args []string) error {
	module := &ExampleModule{}
	result, err := module.Validate(ctx, params)
	fmt.Printf("Result: %+v\nError: %v\n", result, err)
	return nil
}
```

---

## Common Patterns

### Parallel Data Collection

```go
// Collect logs in parallel
logFiles, warnings := CollectPodLogsParallel(
	ctx,
	clients,
	namespace,
	podNames,
	"logs",      // subdirectory
	10,          // max concurrency
)
```

### Resource Collection

```go
var resourceList SomeResourceList
opts := []crclient.ListOption{
	crclient.InNamespace(namespace),
	crclient.MatchingLabels{"app": "weka"},
}
if err := clients.CRClient.List(ctx, &resourceList, opts...); err != nil {
	return err
}

for _, resource := range resourceList.Items {
	// Process each resource
}
```

### YAML Serialization

```go
import "sigs.k8s.io/yaml"

yamlData, err := yaml.Marshal(object)
if err != nil {
	return err
}

// Optionally redact sensitive data
if !includeSensitive {
	yamlString = redactSensitiveData(string(yamlData))
}
```

---

## Release Checklist

Before creating a pull request:

1. ✅ **Tests Pass**: `go test ./...`
2. ✅ **Build Succeeds**: `go build -o kubectl-weka .`
3. ✅ **Conventional Commits**: Use `feat:`, `fix:`, `docs:`, `refactor:` prefixes
4. ✅ **Documentation Updated**: Update README.md if adding commands
5. ✅ **Module Registered**: Ensure `init()` registers new modules
6. ✅ **Code Formatted**: Run `go fmt ./...`
7. ✅ **Linter Clean**: Run `golangci-lint run` (if available)

---

## Additional Resources

- [Cobra Documentation](https://github.com/spf13/cobra)
- [Controller-Runtime Client](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client)
- [Kubernetes Client-Go](https://github.com/kubernetes/client-go)
- [WEKA K8s API CRDs](https://github.com/weka/weka-k8s-api)

---

## Getting Help

- Open an issue on GitHub
- Review existing modules for examples
- Check the `cmd/` directory for similar implementations

---

**Happy coding!** 🚀

