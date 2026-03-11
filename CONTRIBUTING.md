# Contributing to kubectl-weka

Thank you for your interest in contributing to `kubectl-weka`! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Making Changes](#making-changes)
- [Commit Guidelines](#commit-guidelines)
- [Pull Request Process](#pull-request-process)
- [Testing](#testing)
- [Documentation](#documentation)
- [Code Style](#code-style)
- [Reporting Issues](#reporting-issues)
- [Questions or Need Help?](#questions-or-need-help)

---

## Code of Conduct

We are committed to providing a welcoming and inclusive environment for all contributors. Please:

- Be respectful and constructive in all interactions
- Welcome people of all backgrounds and experience levels
- Focus on what is best for the community
- Show empathy towards other community members
- Report unacceptable behavior to the maintainers

---

## Getting Started

### Prerequisites

- **Go 1.25.0 or later** – [Install Go](https://golang.org/doc/install)
- **Git** – For version control
- **Make** – Standard Unix utility
- **kubectl** – Kubernetes command-line tool
- **Access to a Kubernetes cluster** – For testing (optional but recommended)

### Setting Up Your Development Environment

1. **Fork the repository** on GitHub

2. **Clone your fork**
   ```bash
   git clone https://github.com/YOUR_USERNAME/kubectl-weka.git
   cd kubectl-weka
   ```

3. **Add upstream remote**
   ```bash
   git remote add upstream https://github.com/weka/kubectl-weka.git
   ```

4. **Verify the setup**
   ```bash
   make help
   make build
   ./kubectl-weka version
   ```

---

## Development Workflow

### Creating a Feature Branch

Always create a new branch for your changes:

```bash
# Update main branch
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branch
git checkout -b feature/your-feature-name
```

**Branch naming conventions:**
- `feature/` – New features
- `fix/` – Bug fixes
- `docs/` – Documentation updates
- `refactor/` – Code refactoring
- `test/` – Test additions
- `chore/` – Maintenance tasks

### Building and Testing

```bash
# Build the project
make build

# Install to GOPATH/bin
make install

# Test the binary
./kubectl-weka version
./kubectl-weka preflight cluster
./kubectl-weka get cluster-instances
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestName ./...
```

---

## Making Changes

### Code Structure

The project is organized as follows:

```
cmd/
├── root.go                    # Root command definition
├── preflight*.go              # Preflight validation commands
├── plan*.go                   # Plan deployment commands
├── get*.go                    # Get resource commands
├── logs*.go                   # Logs streaming commands
├── supportbundle*.go          # Support bundle commands
├── version.go                 # Version command
└── utils.go                   # Shared utilities
```

### Adding a New Command

1. **Create a new file** in `cmd/` directory
   ```bash
   touch cmd/newcommand.go
   ```

2. **Implement the command** following existing patterns:
   ```go
   package cmd

   import (
       "github.com/spf13/cobra"
   )

   var newCmd = &cobra.Command{
       Use:   "newcommand",
       Short: "Brief description",
       Long:  `Longer description...`,
       RunE: func(cmd *cobra.Command, args []string) error {
           // Implementation
           return nil
       },
   }

   func init() {
       rootCmd.AddCommand(newCmd)
       newCmd.SilenceUsage = true
   }
   ```

3. **Register the command** in `cmd/root.go` or parent command
4. **Add tests** for your command
5. **Update documentation** in README.md

See [Developer Guide](DEVELOPER_GUIDE.md) for detailed instructions on:
- Adding preflight checks
- Adding plan validations
- Adding support bundle collectors

### Adding a Preflight Check

1. Create a check module in `cmd/preflight_*.go`
2. Implement the check interface
3. Register with the appropriate registry
4. Document the check

See the [Developer Guide](DEVELOPER_GUIDE.md#adding-preflight-checks) for examples.

### Adding a Support Bundle Collector

1. Create a collector in `cmd/supportbundle_*.go`
2. Implement the `Collector` interface
3. Register in the mode-based collector selection
4. Add logging and error handling

See the [Developer Guide](DEVELOPER_GUIDE.md#adding-support-bundle-collectors) for examples.

---

## Commit Guidelines

We use [Conventional Commits](https://www.conventionalcommits.org/) for clear, descriptive commit messages.

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- **feat** – A new feature
- **fix** – A bug fix
- **docs** – Documentation only
- **style** – Changes that don't affect functionality (formatting, missing semicolons, etc.)
- **refactor** – Code change that neither fixes a bug nor adds a feature
- **perf** – Code change that improves performance
- **test** – Add or update tests
- **chore** – Changes to build process, dependencies, etc.

### Scope

Optional but recommended. Examples:
- `feat(csi): add get csi-secrets command`
- `fix(preflight): handle missing kubelet config`
- `docs(readme): update installation instructions`

### Subject

- Use imperative mood: "add feature" not "added feature"
- Don't capitalize first letter
- No period (.) at the end
- Limit to 50 characters

### Body

- Explain what and why, not how
- Wrap at 72 characters
- Separate from subject with blank line

### Footer

Reference issues and breaking changes:

```
Fixes #123
Closes #456
BREAKING CHANGE: description of breaking change
```

### Examples

```
feat(csi): add get csi-secrets command

Add a new command to list and validate CSI secrets referenced
by storage classes. Validates required parameters and detects
configuration issues.

Fixes #42
```

```
fix(logs): handle pod with no logs gracefully

Previously would fail if pod had no logs. Now returns empty
log with informative message instead.

Closes #99
```

---

## Pull Request Process

### Before Submitting

1. **Update your branch** with latest upstream
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run tests locally**
   ```bash
   go test ./...
   ```

3. **Build and verify**
   ```bash
   make clean
   make build
   ./kubectl-weka version
   ```

4. **Check code style**
   ```bash
   go fmt ./...
   go vet ./...
   ```

5. **Update documentation** if needed

### Submitting a PR

1. **Push your branch**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create Pull Request** on GitHub with:
   - Clear title describing the change
   - Reference to related issues (e.g., "Fixes #123")
   - Description of what changed and why
   - Screenshots or examples if applicable

3. **PR Title Format** (follows Conventional Commits):
   ```
   feat(csi): add get csi-secrets command
   fix(logs): handle empty logs gracefully
   docs: update README installation section
   ```

### PR Requirements

- ✅ All CI checks must pass (multi-arch builds, tests)
- ✅ Code must follow project style guidelines
- ✅ Changes must have appropriate tests
- ✅ Documentation must be updated
- ✅ Commits must follow Conventional Commits format
- ✅ No merge conflicts with main branch

### Review Process

1. Maintainers will review your PR
2. Requested changes should be addressed in new commits
3. Don't force-push after review starts (keeps conversation visible)
4. Once approved, maintainers will merge

---

## Testing

### Test Requirements

- **New features** must include tests
- **Bug fixes** should include regression tests
- **Aim for >80% code coverage**

### Writing Tests

```go
package cmd

import (
    "testing"
)

func TestYourFeature(t *testing.T) {
    // Arrange
    input := "test"
    
    // Act
    result := YourFunction(input)
    
    // Assert
    if result != "expected" {
        t.Errorf("Expected 'expected', got '%s'", result)
    }
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Run specific test file
go test -run TestName ./cmd
```

---

## Documentation

### What to Document

- **New commands** – Add to README.md
- **New flags** – Document in command help text
- **New features** – Add to appropriate guide
- **Architecture changes** – Update DEVELOPER_GUIDE.md

### Documentation Guidelines

- Keep documentation close to code
- Use clear, simple language
- Include examples
- Update CHANGELOG.md for user-facing changes
- Update table of contents if adding new sections

### README.md Sections

When adding a new command, include:
- Purpose (what it does)
- Usage (syntax and examples)
- Flags (all available options)
- Output format (what it displays)
- Use cases (when to use it)

---

## Code Style

### Go Best Practices

- **Naming** – Use idiomatic Go names
  ```go
  // Good
  var serverPort int
  var isReady bool
  
  // Avoid
  var server_port int
  var is_ready bool
  ```

- **Error handling** – Always handle errors
  ```go
  // Good
  if err != nil {
      return fmt.Errorf("failed to do something: %w", err)
  }
  
  // Avoid
  if err != nil {
      panic(err)
  }
  ```

- **Comments** – Use clear, concise comments
  ```go
  // Good
  // validateSecret checks if secret has required parameters
  func validateSecret(s *v1.Secret) error {
  
  // Avoid
  // This function validates the secret
  func validateSecret(s *v1.Secret) error {
  ```

### Formatting

```bash
# Format code
go fmt ./...

# Check for issues
go vet ./...

# Run linter (if installed)
golint ./...
```

### Import Organization

```go
import (
    // Standard library
    "context"
    "fmt"
    
    // Third-party packages
    "github.com/spf13/cobra"
    "k8s.io/api/core/v1"
)
```

---

## Reporting Issues

### Bug Reports

When reporting a bug, include:

1. **Description** – What's the issue?
2. **Steps to Reproduce** – How to trigger it?
3. **Expected Behavior** – What should happen?
4. **Actual Behavior** – What actually happened?
5. **Environment** – OS, Go version, Kubernetes version
6. **Support Bundle** – Output of `kubectl weka support-bundle`

### Feature Requests

When requesting a feature, include:

1. **Description** – What do you want to do?
2. **Use Case** – Why is this needed?
3. **Proposed Solution** – How should it work?
4. **Alternatives** – Are there other ways?

### Security Issues

**Do NOT** open a public issue for security vulnerabilities.

Instead, email security concerns to the maintainers directly.

---

## Questions or Need Help?

### Getting Help

- **Documentation** – See [README.md](README.md) and [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)
- **Issues** – Search [existing issues](https://github.com/weka/kubectl-weka/issues)
- **Discussions** – Start a discussion if you have questions
- **Slack/Email** – Contact maintainers directly

### Reviewing Others' PRs

You don't need to be a maintainer to help! Reviewing PRs is valuable:

- Test the changes locally
- Provide constructive feedback
- Suggest improvements
- Ask clarifying questions

---

## Release Process

The project uses [release-please](https://github.com/googleapis/release-please) for automated versioning and releases.

### How It Works

1. **Commits** are analyzed for breaking changes, features, and fixes
2. **Release PR** is automatically created with:
   - Updated CHANGELOG.md
   - Version bump (major/minor/patch)
3. **PR is merged** → automatic Git tag creation
4. **Release build** is triggered automatically
5. **Binaries** are built for all platforms and attached to release

Your conventional commits automatically determine version bumps:
- `feat:` → minor version bump
- `fix:` → patch version bump
- `BREAKING CHANGE:` → major version bump

---

## Project Governance

### Maintainers

- Have merge permission
- Review and approve PRs
- Manage releases
- Make architectural decisions

### Contributors

- Submit PRs with fixes and features
- Review and test others' PRs
- Help with documentation
- Report issues and suggest improvements

---

## Recognition

Contributors are recognized in:
- Git commit history
- GitHub contributors page
- Release notes for significant contributions
- CHANGELOG.md for major features

Thank you for contributing to kubectl-weka! 🎉

