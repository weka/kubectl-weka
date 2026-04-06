package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config holds all user-configurable settings
// Add new fields here as needed
// Always update defaultConfig() when adding fields
// All fields should have yaml tags

type Config struct {
	Cache CacheConfig `yaml:"cache"`
	// DefaultNamespace string `yaml:"default_namespace"` // (not used for now)
	OutputFormat string `yaml:"output_format"`
}

type CacheConfig struct {
	UsePersistentCaching bool             `yaml:"use_persistent_caching"`
	Completion           CompletionConfig `yaml:"completion"`
	HostCheck            HostCheckConfig  `yaml:"host_check"`
}

type CompletionConfig struct {
	CompletionTTLs CompletionTTLs `yaml:"ttls"`
	Encrypt        bool           `yaml:"encrypt"`
}
type CompletionTTLs struct {
	Namespaces    int64 `yaml:"namespaces"`
	Nodes         int64 `yaml:"nodes"`
	NodeSelectors int64 `yaml:"nodeselectors"`
	WekaObjects   int64 `yaml:"wekaobjects"`
}

type HostCheckConfig struct {
	Ttl int64 `yaml:"ttl"`
}

var (
	configPath string
	configDir  string
	configOnce sync.Once
	config     *Config
)

// defaultConfig returns a Config with all default values
func defaultConfig() *Config {
	return &Config{
		Cache: CacheConfig{
			UsePersistentCaching: true,
			Completion: CompletionConfig{
				Encrypt: true,
				CompletionTTLs: CompletionTTLs{
					Namespaces:    600,
					Nodes:         600,
					NodeSelectors: 600,
					WekaObjects:   600,
				},
			},
			HostCheck: HostCheckConfig{
				Ttl: 3600,
			},
		},
	}
}

// getConfigPath returns the config file path, creating the directory if needed
func getConfigPath() string {
	configOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			panic("cannot determine home directory")
		}
		configDir = filepath.Join(home, ".weka", "kubectl-weka")
		_ = os.MkdirAll(configDir, 0o755)
		configPath = filepath.Join(configDir, "config.yaml")
	})
	return configPath
}

// Load loads the config from disk, creating or upgrading as needed
func Load() *Config {
	path := getConfigPath()
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if err == nil {
		_ = yaml.Unmarshal(data, cfg)
	}
	// Upgrade: ensure all fields are set
	defaults := defaultConfig()
	// Cache
	if cfg.Cache.Completion.CompletionTTLs.Namespaces == 0 {
		cfg.Cache.Completion.CompletionTTLs.Namespaces = defaults.Cache.Completion.CompletionTTLs.Namespaces
	}
	if cfg.Cache.Completion.CompletionTTLs.Nodes == 0 {
		cfg.Cache.Completion.CompletionTTLs.Nodes = defaults.Cache.Completion.CompletionTTLs.Nodes
	}
	if cfg.Cache.Completion.CompletionTTLs.NodeSelectors == 0 {
		cfg.Cache.Completion.CompletionTTLs.NodeSelectors = defaults.Cache.Completion.CompletionTTLs.NodeSelectors
	}
	if cfg.Cache.Completion.CompletionTTLs.WekaObjects == 0 {
		cfg.Cache.Completion.CompletionTTLs.WekaObjects = defaults.Cache.Completion.CompletionTTLs.WekaObjects
	}
	if cfg.Cache.UsePersistentCaching != true && cfg.Cache.UsePersistentCaching != false {
		cfg.Cache.UsePersistentCaching = defaults.Cache.UsePersistentCaching
	}
	if cfg.Cache.Completion.Encrypt != true && cfg.Cache.Completion.Encrypt != false {
		cfg.Cache.Completion.Encrypt = defaults.Cache.Completion.Encrypt
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = defaults.OutputFormat
	}
	config = cfg
	return cfg
}

// Save writes the config to disk
func Save() error {
	if config == nil {
		return nil
	}
	path := getConfigPath()
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Get returns the loaded config (must call Load first)
func Get() *Config {
	if config == nil {
		Load()
	}
	return config
}
