# Test Suite Documentation

## Overview

Comprehensive unit tests have been created for kubectl-weka components. All tests are non-interactive and can be run with standard Go testing tools.

## Test Files Created

### 1. `cmd/utils_test.go`
Tests utility functions used throughout the project.

**Tests Included:**
- `TestParseSelector` – Tests label selector parsing
  - Single labels
  - Multiple labels
  - Labels with spaces
  - Empty selectors
  - Complex label combinations

- `TestSortNodeNamesNumerically` – Tests natural sorting of node names
  - Simple numeric names (node1, node2, node3)
  - Mixed numeric names (node11, node2, node1)
  - Names with suffixes
  - Multiple numeric parts
  - Single item, already sorted, empty list

- `TestRandomString` – Tests random string generation
  - Various lengths
  - Character validation
  - Uniqueness verification

### 2. `cmd/csi_test.go`
Tests CSI (Container Storage Interface) functionality.

**Tests Included:**
- `TestIsWekaCSI` – Tests CSI driver detection
  - weka.io drivers
  - weka-csi.weka.io drivers
  - Non-Weka drivers
  - Case sensitivity
  - Empty strings

- `TestExtractSecretReferencesFromStorageClass` – Tests secret extraction from storage classes
  - No secret parameters
  - Single secret with namespace
  - Multiple secrets
  - Secrets without namespace
  - Empty parameters

- `TestValidateSecretContent` – Tests CSI secret validation
  - Valid secrets
  - Missing required parameters
  - Invalid schemes
  - Whitespace issues
  - Empty secrets

### 3. `cmd/version_test.go`
Tests version command functionality.

**Tests Included:**
- `TestSetVersion` – Tests version information setting
  - Release versions
  - Development versions
  - Dirty versions
  - Empty values

### 4. `cmd/pod_utils_test.go`
Tests pod-related utility functions.

**Tests Included:**
- `TestPodRestartCount` – Tests counting container restarts
  - No restarts
  - Single container restarts
  - Multiple container restarts
  - No container statuses
  - Aggregate restart counting

- `TestPodStatus` – Tests pod status reading
  - Running pods
  - Pending pods
  - Succeeded pods
  - Failed pods

### 5. `cmd/collector_test.go`
Tests collector framework functionality.

**Tests Included:**
- `TestCollectorResultStructure` – Tests CollectorResult struct
  - Successful collections
  - Partial success
  - Failed collections
  - File and warning tracking

- `TestSecretReference` – Tests SecretReference struct
  - Name field
  - Namespace field

### 6. `cmd/hostcheck_test.go`
Tests host check data structures.

**Tests Included:**
- `TestHostChecksResultFields` – Tests HostChecksResult struct
  - OS detection
  - Kernel version
  - WEKA directory status
  - File system info
  - Network detection
  - CPU info
  - Memory info
  - NVMe drives

- `TestMellanoxIfaceStructure` – Tests Mellanox interface struct
  - Name field
  - Model field
  - Speed field

- `TestBondInfoStructure` – Tests bond information struct
  - Name field
  - Slaves field
  - Mode field

- `TestNVMeDriveInfo` – Tests NVMe drive info struct
  - Device name
  - Device path
  - Serial number
  - Size
  - Mount status

## Running Tests

### Run All Tests
```bash
go test ./cmd -v
```

### Run Specific Test File
```bash
go test ./cmd -run TestParseSelector -v
```

### Run Specific Test
```bash
go test ./cmd -run TestParseSelector/single_label -v
```

### Run with Coverage
```bash
go test ./cmd -cover
go test ./cmd -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Run Tests with Verbose Output
```bash
go test ./cmd -v
```

## Test Coverage

The test suite covers:
- ✅ Utility functions (selector parsing, node sorting, random strings)
- ✅ CSI driver detection and validation
- ✅ Secret extraction and validation from storage classes
- ✅ Pod status and restart counting
- ✅ Collector framework structures
- ✅ Host check data structures
- ✅ Version information handling

## Test Organization

Tests follow Go best practices:
- One test file per component/package
- Table-driven test design for comprehensive coverage
- Clear test names describing what is being tested
- Both positive and negative test cases
- Edge cases (empty inputs, missing fields, etc.)

## Adding More Tests

To add new tests:

1. Create test file: `cmd/{component}_test.go`
2. Follow table-driven test pattern
3. Include both success and failure cases
4. Document test purpose with comments
5. Run `go test ./cmd -v` to verify

## Integration with CI/CD

Tests can be integrated into GitHub Actions:

```yaml
- name: Run tests
  run: go test ./cmd -v -coverprofile=coverage.out
```

## Future Test Additions

Recommended areas for additional tests:
- Integration tests with Kubernetes client
- End-to-end tests for commands
- Performance tests for large datasets
- Mock Kubernetes API tests
- Command execution tests with mocked kubectl

