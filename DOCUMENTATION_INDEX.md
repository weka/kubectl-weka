# kubectl-weka Documentation Index

Welcome to the `kubectl-weka` documentation! This index will help you find the right documentation for your needs.

## 📚 Documentation Files

### For Users

| Document | Purpose | When to Use |
|----------|---------|-------------|
| **[README.md](README.md)** | Complete command reference | Main documentation - start here |
| **[QUICK_REFERENCE.md](QUICK_REFERENCE.md)** | Command cheat sheet | Quick lookup of commands and flags |
| **[CHANGELOG.md](CHANGELOG.md)** | Release history | Check what's new in each version |

### For Developers

| Document | Purpose | When to Use |
|----------|---------|-------------|
| **[DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)** | Extension and development guide | Adding new features or checks |
| **[DOCUMENTATION_SUMMARY.md](DOCUMENTATION_SUMMARY.md)** | Documentation update log | Track documentation changes |

### Legal

| Document | Purpose |
|----------|---------|
| **[LICENSE.md](LICENSE.md)** | Apache 2.0 License |

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

#### Monitor CSI drivers
→ See [README.md - get csi-drivers](README.md#get-csi-drivers)

#### Collect diagnostic data
→ See [README.md - Support Bundle Commands](README.md#support-bundle-commands)

#### Troubleshoot issues
→ See [README.md - Troubleshooting](README.md#troubleshooting)

#### Add a new preflight check
→ See [DEVELOPER_GUIDE.md - Adding Preflight Checks](DEVELOPER_GUIDE.md#adding-preflight-checks)

#### Add a new support bundle collector
→ See [DEVELOPER_GUIDE.md - Adding Support Bundle Collectors](DEVELOPER_GUIDE.md#adding-support-bundle-collectors)

#### Add a new command
→ See [DEVELOPER_GUIDE.md - Adding New Commands](DEVELOPER_GUIDE.md#adding-new-commands)

#### Understand the architecture
→ See [DEVELOPER_GUIDE.md - Architecture Overview](DEVELOPER_GUIDE.md#architecture-overview)

#### Contribute to the project
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
**~850+ lines** | Developer documentation

Comprehensive guide for extending kubectl-weka:
- Architecture overview
- Adding preflight checks (node & cluster)
- Adding plan validations
- Adding support bundle collectors
- Adding new commands
- Testing guidelines
- Code organization best practices
- Common patterns and debugging tips

**Use this** when developing new features or extending functionality.

---

### DOCUMENTATION_SUMMARY.md (Update Log)
**~300 lines** | Documentation change tracking

Summary of documentation updates including:
- Files modified
- Key improvements
- Coverage statistics
- Recommendations for future updates

**Use this** to understand what documentation changed and why.

---

### CHANGELOG.md (Release History)
Automatically generated release notes using Release Please.

**Use this** to see what's new in each version.

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
| User documentation files | 3 |
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

