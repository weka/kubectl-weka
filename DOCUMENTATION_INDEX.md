# kubectl-weka Documentation Index

Welcome to the `kubectl-weka` documentation! This index will help you find the right documentation for your needs.

## 📚 Documentation Files

### For Users

| Document | Purpose | When to Use |
|----------|---------|-------------|
| **[README.md](README.md)** | Complete command reference | Main documentation - start here |
| **[QUICK_REFERENCE.md](QUICK_REFERENCE.md)** | Command cheat sheet | Quick lookup of commands and flags |
| **[CHANGELOG.md](CHANGELOG.md)** | Release history | Check what's new in each version |
| **[docs/network-configuration.md](docs/network-configuration.md)** | Network setup guide | Understanding Ethernet/InfiniBand, speed/rate metrics, validation |

### For Developers

| Document | Purpose | When to Use |
|----------|---------|-------------|
| **[DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)** | Extension and development guide | Adding new features, packages, or commands |
| **[CONTRIBUTING.md](CONTRIBUTING.md)** | Contribution guidelines | How to contribute to the project |

### Legal

| Document | Purpose |
|----------|---------|
| **[LICENSE](LICENSE)** | Apache 2.0 License |

---

## 🎯 Quick Navigation

### I want to...

#### Install kubectl-weka
→ See [README.md - Installation](README.md#installation)

#### Learn all commands
→ See [README.md - Commands Overview](README.md#commands-overview)

#### Get a quick command reference
→ See [QUICK_REFERENCE.md](QUICK_REFERENCE.md)

#### Run preflight checks
→ See [README.md - Preflight Commands](README.md#preflight-commands)

#### Plan a deployment
→ See [README.md - Plan Commands](README.md#plan-commands)

#### Inspect WEKA resources
→ See [README.md - Get Commands](README.md#get-commands)

#### Understand network configuration
→ See [README.md - Network Configuration](README.md#network-configuration) or [docs/network-configuration.md](docs/network-configuration.md)

#### Build from source
→ See [README.md - Building from Source](README.md#building-from-source) or [CONTRIBUTING.md - Building and Testing](CONTRIBUTING.md#building-and-testing)

#### Contribute to the project
→ See [CONTRIBUTING.md](CONTRIBUTING.md)

#### Understand Ethernet speed vs InfiniBand rate
→ See [docs/network-configuration.md - Speed and Rate Metrics](docs/network-configuration.md#speed-and-rate-metrics)

#### Monitor CSI drivers
→ See [README.md - get csi-drivers](README.md#get-csi-drivers)

#### View CSI pod instances
→ See [README.md - get csi-instances](README.md#get-csi-instances)

#### Collect diagnostic data
→ See [README.md - Support Bundle Commands](README.md#support-bundle-commands)

#### Stream cluster container logs
→ See [README.md - logs wekacluster](README.md#logs-wekacluster)

#### Stream client container logs
→ See [README.md - logs wekaclient](README.md#logs-wekaclient)

#### Stream arbitrary WekaContainer logs
→ See [README.md - logs wekacontainer](README.md#logs-wekacontainer)

#### Deploy WEKA in air-gapped environment
→ See [DEVELOPER_GUIDE.md - Air-Gapped Deployment](DEVELOPER_GUIDE.md#air-gapped-deployment)
→ See [README.md - Troubleshooting](README.md#troubleshooting)

#### Add a new preflight check
→ See [DEVELOPER_GUIDE.md - Adding Preflight Checks](DEVELOPER_GUIDE.md#adding-preflight-checks)

#### Add a new support bundle collector
→ See [DEVELOPER_GUIDE.md - Adding Support Bundle Collectors](DEVELOPER_GUIDE.md#adding-support-bundle-collectors)

#### Add a new command
→ See [DEVELOPER_GUIDE.md - Adding New Commands](DEVELOPER_GUIDE.md#adding-new-commands)

#### Understand the architecture
→ See [DEVELOPER_GUIDE.md - Architecture Overview](DEVELOPER_GUIDE.md#architecture-overview)

#### Learn about Docker image handling
→ See [DEVELOPER_GUIDE.md - Docker Package](DEVELOPER_GUIDE.md#docker-package)

#### Understand progress tracking
→ See [DEVELOPER_GUIDE.md - Progress Package](DEVELOPER_GUIDE.md#progress-package)

#### Work with tar.gz operations
→ See [DEVELOPER_GUIDE.md - Targzutils Package](DEVELOPER_GUIDE.md#targzutils-package)

#### Integrate with Helm charts
→ See [DEVELOPER_GUIDE.md - Helm Package](DEVELOPER_GUIDE.md#helm-package)

#### Build air-gapped deployment features
→ See [DEVELOPER_GUIDE.md - Air-Gapped Deployment](DEVELOPER_GUIDE.md#air-gapped-deployment)
→ See [README.md - Contributing](README.md#contributing)

---

## 📖 Document Summaries

### README.md (Main Documentation)
**~700+ lines** | Complete command reference

The main documentation file covering:
- Installation instructions
- All command categories (preflight, get, plan, logs, support-bundle)
- Detailed examples and output samples
- Resource calculation formulas
- Troubleshooting guide
- Contributing guidelines
- Development setup

**Start here** if you're new to kubectl-weka or need comprehensive command documentation.

---

### QUICK_REFERENCE.md (Cheat Sheet)
**~250 lines** | Quick command lookup

A condensed reference for:
- Common command patterns
- All flags at a glance
- Resource formulas
- Quick troubleshooting
- Common workflows

**Use this** when you need a quick reminder of command syntax.

---

### DEVELOPER_GUIDE.md (Extension Guide)
**~1000+ lines** | Developer documentation

Comprehensive guide for extending kubectl-weka:
- Building and versioning
- Architecture overview with new packages
- Docker package for image registry operations
- Logging package for structured logging
- Progress package for real-time progress display
- Targzutils package for tar.gz operations
- Helm package for chart manipulation
- Air-gapped deployment subsystem
- ResourcePrinter system for output formatting
- Adding preflight checks (node & cluster)
- Adding plan validations
- Adding support bundle collectors
- Adding new commands
- Testing guidelines
- Inline function documentation standards
- Code organization best practices
- Debugging tips

**Use this** when developing new features or extending functionality.

---

### KNOWN_ISSUES.md (Current Issues)

Known limitations and issues with kubectl-weka:
- Log streaming time-window buffering
- Air-gapped deployment limitations (authentication, bundle size, image format)
- Progress tracking in pipes/redirects
- Docker image handling (multi-layer, token expiry)
- Helm chart updates (nested values, custom paths)
- Kubernetes compatibility (RBAC, network policies)
- Unsupported configurations
- Troubleshooting guide

**Use this** to understand current limitations and find solutions.

---

### CHANGELOG.md (Release History)

Contains all feature additions, improvements, and changes per version.

**Use this** to see what's new in each version and track changes.

---

## 🔍 Command Reference Quick Links

### Preflight Commands
- [preflight cluster](README.md#preflight-cluster) - Cluster validation
- [preflight nodes](README.md#preflight-nodes) - Node validation

### Get Commands
- [get cluster-instances](README.md#get-cluster-instances) - List cluster containers
- [get client-instances](README.md#get-client-instances) - List client containers
- [get nodes](README.md#get-nodes) - List nodes
- [get policies](README.md#get-policies) - List policies

### Plan Commands
- [plan cluster](README.md#plan-cluster) - Plan cluster deployment
- [plan client](README.md#plan-client) - Plan client deployment
- [plan converged](README.md#plan-converged) - Plan converged deployment

### Logs Commands
- [logs operator](README.md#logs-operator) - Stream operator logs
- [logs wekacluster](README.md#logs-wekacluster) - Stream cluster container logs
- [logs wekaclient](README.md#logs-wekaclient) - Stream client container logs
- [logs wekacontainer](README.md#logs-wekacontainer) - Stream arbitrary container logs

### Support Bundle Commands
- [support-bundle operator](README.md#support-bundle-operator) - Operator diagnostics
- [support-bundle cluster](README.md#support-bundle-cluster) - Cluster diagnostics
- [support-bundle client](README.md#support-bundle-client) - Client diagnostics
- [support-bundle csi](README.md#support-bundle-csi) - CSI diagnostics
- [support-bundle k8s](README.md#support-bundle-k8s) - K8s preflight results
- [support-bundle all](README.md#support-bundle-all) - Complete diagnostics

---

## 🚀 Getting Started Paths

### For End Users

1. **Installation**
   - Read [Installation](README.md#installation)
   - Install via Krew or manual binary

2. **First Steps**
   - Run `kubectl weka help`
   - Try `kubectl weka preflight cluster`
   - Review [QUICK_REFERENCE.md](QUICK_REFERENCE.md)

3. **Common Tasks**
   - Preflight validation before deployment
   - Resource planning with `plan` commands
   - Monitoring with `get` commands
   - Troubleshooting with `support-bundle`

### For Developers

1. **Setup**
   - Read [Development](README.md#development)
   - Clone repository
   - Build locally: `go build -o kubectl-weka .`

2. **Learn Architecture**
   - Review [Architecture Overview](DEVELOPER_GUIDE.md#architecture-overview)
   - Understand registry pattern
   - Explore existing modules in `cmd/`

3. **Add Features**
   - Follow guides in [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)
   - Add tests
   - Update documentation
   - Submit PR

### For Contributors

1. **Understand Guidelines**
   - Read [Contributing](README.md#contributing)
   - Follow [Conventional Commits](https://www.conventionalcommits.org/)
   - Review code style

2. **Make Changes**
   - Fork repository
   - Create feature branch
   - Implement changes
   - Add/update documentation

3. **Submit**
   - Run tests
   - Update README if adding commands
   - Create PR with clear description

---

## 📝 Documentation Standards

All documentation follows these principles:

- ✅ **Clear Examples** - Every command has usage examples
- ✅ **Consistent Format** - Standard structure across all docs
- ✅ **Up-to-Date** - Synchronized with code implementation
- ✅ **Comprehensive** - All commands and features documented
- ✅ **Accessible** - Easy to navigate and find information

---

## 🆘 Need Help?

### Can't find what you need?

1. **Search the docs** - Use Ctrl+F in README.md
2. **Check QUICK_REFERENCE** - For command syntax
3. **Review examples** - Similar commands might have what you need
4. **Open an issue** - [GitHub Issues](https://github.com/weka/kubectl-weka/issues)

### Found an error?

- Documentation bugs → Open an issue
- Code bugs → Open an issue
- Feature requests → Open an issue with `enhancement` label

---

## 📊 Documentation Stats

| Metric | Count |
|--------|-------|
| Total markdown files | 8 |
| User documentation files | 4 |
| Developer documentation files | 2 |
| Total documented commands | 15+ |
| Code examples | 50+ |
| Total documentation lines | ~2000+ |

---

## 🔄 Keeping Up-to-Date

- **CHANGELOG.md** - Check for new releases
- **GitHub Releases** - Tagged versions with binaries
- **README.md** - Main documentation updates with each release
- **DEVELOPER_GUIDE.md** - Updated when architecture changes

---

**Last Updated:** March 2026

**Documentation Version:** 1.0

**Compatible with:** kubectl-weka v0.1.0+

---

**Ready to get started?** → [README.md](README.md)

**Need a quick reference?** → [QUICK_REFERENCE.md](QUICK_REFERENCE.md)

**Want to contribute?** → [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)

