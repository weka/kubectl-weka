package cmd

import (
	"fmt"
	"sync"
	"time"
)

type ModuleName string

const (
	ModuleNameOs                 ModuleName = "os"
	ModuleNameKernel             ModuleName = "kernel"
	ModuleNameCpuMemory          ModuleName = "cpu_memory"
	ModuleNameWekaDirectory      ModuleName = "weka_dir"
	ModuleNameXfs                ModuleName = "xfs"
	ModuleNameWekaAgentService   ModuleName = "weka_agent_service"
	ModuleNameNetworkInterfaces  ModuleName = "network_interfaces"
	ModuleNameSourceBasedRouting ModuleName = "source_based_routing"
	ModuleNameNVMeDrives         ModuleName = "nvme_drives"
)

// HostCheckModuleRegistry manages all available hostcheck modules
// and command-specific validation configurations with caching
type HostCheckModuleRegistry struct {
	// Modules: available validation modules
	modules map[ModuleName]HostCheckModule
	order   []ModuleName // Preserve module registration order

	// Command configs: which modules each command validates against
	commands map[string]*CommandHostCheckConfig

	// Cache: cached hostcheck results to avoid re-running
	cache struct {
		mu          sync.RWMutex
		results     HostChecksMap
		nodes       []string // Node names that were checked
		lastUpdated time.Time
	}
}

// NewHostCheckModuleRegistry creates a new registry
func NewHostCheckModuleRegistry() *HostCheckModuleRegistry {
	registry := &HostCheckModuleRegistry{
		modules:  make(map[ModuleName]HostCheckModule),
		order:    []ModuleName{},
		commands: make(map[string]*CommandHostCheckConfig),
	}
	registry.cache.results = make(HostChecksMap)
	return registry
}

// NewStandardModuleRegistry creates a registry with all standard modules and command configs
func NewStandardModuleRegistry() *HostCheckModuleRegistry {
	registry := NewHostCheckModuleRegistry()

	// Register all standard validation modules
	_ = registry.RegisterModule(&OSModule{})
	_ = registry.RegisterModule(&WekaDirModule{})
	_ = registry.RegisterModule(&XFSModule{})
	_ = registry.RegisterModule(&WekaAgentServiceModuleModule{})
	_ = registry.RegisterModule(&CPUModule{})
	_ = registry.RegisterModule(&KernelModule{})
	_ = registry.RegisterModule(&NVMeDrivesModule{})
	_ = registry.RegisterModule(&NetworkInterfacesModule{})
	_ = registry.RegisterModule(&SourceBasedRoutingModule{})

	// Register command validation configurations
	_ = registry.RegisterCommand(&CommandHostCheckConfig{
		CommandName: "preflight_nodes",
		ModuleNames: []ModuleName{
			ModuleNameOs, ModuleNameKernel, ModuleNameCpuMemory, ModuleNameWekaDirectory,
			ModuleNameXfs, ModuleNameWekaAgentService, ModuleNameNetworkInterfaces, ModuleNameSourceBasedRouting, ModuleNameNVMeDrives,
		},
	})

	_ = registry.RegisterCommand(&CommandHostCheckConfig{
		CommandName: "plan_cluster",
		ModuleNames: []ModuleName{ModuleNameCpuMemory, ModuleNameNetworkInterfaces, ModuleNameSourceBasedRouting, ModuleNameNVMeDrives},
	})

	_ = registry.RegisterCommand(&CommandHostCheckConfig{
		CommandName: "plan_client",
		ModuleNames: []ModuleName{ModuleNameCpuMemory, ModuleNameNetworkInterfaces, ModuleNameSourceBasedRouting, ModuleNameNVMeDrives},
	})

	_ = registry.RegisterCommand(&CommandHostCheckConfig{
		CommandName: "plan_converged",
		ModuleNames: []ModuleName{ModuleNameCpuMemory, ModuleNameNetworkInterfaces, ModuleNameSourceBasedRouting, ModuleNameNVMeDrives},
	})

	return registry
}

// Global registry instance (modules + commands + cache)
var GlobalHostCheckRegistry *HostCheckModuleRegistry

// InitializeHostCheckRegistry sets up the global registry
func InitializeHostCheckRegistry() {
	GlobalHostCheckRegistry = NewStandardModuleRegistry()
}

// RegisterModule adds a validation module to the registry
func (r *HostCheckModuleRegistry) RegisterModule(module HostCheckModule) error {
	name := module.Name()
	if _, exists := r.modules[name]; exists {
		return fmt.Errorf("hostcheck module '%s' already registered", name)
	}
	r.modules[name] = module
	r.order = append(r.order, name)
	return nil
}

// RegisterCommand adds a command's validation configuration
func (r *HostCheckModuleRegistry) RegisterCommand(config *CommandHostCheckConfig) error {
	if config.CommandName == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	if _, exists := r.commands[config.CommandName]; exists {
		return fmt.Errorf("command '%s' already registered", config.CommandName)
	}

	r.commands[config.CommandName] = config
	return nil
}

// GetCommand retrieves a command's validation configuration
func (r *HostCheckModuleRegistry) GetCommand(commandName string) (*CommandHostCheckConfig, bool) {
	config, exists := r.commands[commandName]
	return config, exists
}

// GetRequiredModules returns the list of validation modules a command needs
func (r *HostCheckModuleRegistry) GetRequiredModules(commandName string) []ModuleName {
	config, exists := r.commands[commandName]
	if !exists {
		return nil
	}
	return config.ModuleNames
}

// ============================================================================
// Cache Management
// ============================================================================

// ClearCache clears the hostcheck results cache
func (r *HostCheckModuleRegistry) ClearCache() {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()

	r.cache.results = make(HostChecksMap)
	r.cache.nodes = nil
	r.cache.lastUpdated = time.Time{}
}

// GetCacheInfo returns information about the cache state
func (r *HostCheckModuleRegistry) GetCacheInfo() (nodeCount int, lastUpdated time.Time) {
	r.cache.mu.RLock()
	defer r.cache.mu.RUnlock()

	return len(r.cache.results), r.cache.lastUpdated
}

// ============================================================================
// Module Access
// ============================================================================

// Get retrieves a module by name
func (r *HostCheckModuleRegistry) Get(name ModuleName) (HostCheckModule, error) {
	module, exists := r.modules[name]
	if !exists {
		return nil, fmt.Errorf("hostcheck module '%s' not found", name)
	}
	return module, nil
}

// ListModules returns all registered module names in registration order
func (r *HostCheckModuleRegistry) ListModules() []ModuleName {
	result := make([]ModuleName, len(r.order))
	copy(result, r.order)
	return result
}
