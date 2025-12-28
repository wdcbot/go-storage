package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// envVarRegex matches ${VAR} or $VAR patterns.
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

// Config represents the storage configuration structure.
type Config struct {
	Default  string                   `yaml:"default" json:"default"`
	Storages map[string]StorageConfig `yaml:"storages" json:"storages"`
}

// StorageConfig represents a single storage backend configuration.
type StorageConfig struct {
	Driver  string         `yaml:"driver" json:"driver"`
	Options map[string]any `yaml:"options" json:"options"`
}

// LoadConfig loads configuration from a dedicated storage config file.
// Supports .yaml, .yml, and .json files.
// Environment variables in the format ${VAR} or $VAR are automatically expanded.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to read config file: %w", err)
	}

	// Expand environment variables
	data = expandEnvVars(data)

	ext := strings.ToLower(filepath.Ext(path))
	var cfg Config

	switch ext {
	case ".yaml", ".yml", ".json":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("storage: failed to parse config: %w", err)
		}
	default:
		return nil, fmt.Errorf("storage: unsupported config format %q", ext)
	}

	return &cfg, nil
}

// expandEnvVars expands ${VAR} and $VAR patterns in the data.
func expandEnvVars(data []byte) []byte {
	return envVarRegex.ReplaceAllFunc(data, func(match []byte) []byte {
		s := string(match)
		var varName string
		if strings.HasPrefix(s, "${") {
			varName = s[2 : len(s)-1]
		} else {
			varName = s[1:]
		}
		if val := os.Getenv(varName); val != "" {
			return []byte(val)
		}
		return match // Keep original if env var not set
	})
}

// LoadConfigFromEnv loads configuration from a file path specified in an environment variable.
// If the env var is not set, it looks for storage.yaml in the current directory.
func LoadConfigFromEnv(envVar string) (*Config, error) {
	path := os.Getenv(envVar)
	if path == "" {
		// Default paths to try
		defaults := []string{"storage.yaml", "storage.yml", "config/storage.yaml"}
		for _, p := range defaults {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
		if path == "" {
			return nil, fmt.Errorf("storage: no config file found (set %s or create storage.yaml)", envVar)
		}
	}
	return LoadConfig(path)
}

// EmbeddedConfig represents a user's config file with embedded storage configuration.
// This allows storage config to live inside the user's existing config.yaml.
//
// Example config.yaml:
//
//	app:
//	  name: myapp
//	storage:              # <-- storage config embedded here
//	  default: local
//	  storages:
//	    local:
//	      driver: local
//	      options:
//	        root: ./uploads
type EmbeddedConfig struct {
	Storage Config `yaml:"storage" json:"storage"`
}

// LoadConfigEmbedded loads storage configuration from a user's config file
// where storage config is nested under a "storage" key.
// Environment variables in the format ${VAR} or $VAR are automatically expanded.
//
// Example:
//
//	cfg, err := storage.LoadConfigEmbedded("config.yaml")
func LoadConfigEmbedded(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to read config file: %w", err)
	}

	// Expand environment variables
	data = expandEnvVars(data)

	ext := strings.ToLower(filepath.Ext(path))
	var embedded EmbeddedConfig

	switch ext {
	case ".yaml", ".yml", ".json":
		if err := yaml.Unmarshal(data, &embedded); err != nil {
			return nil, fmt.Errorf("storage: failed to parse config: %w", err)
		}
	default:
		return nil, fmt.Errorf("storage: unsupported config format %q", ext)
	}

	if embedded.Storage.Storages == nil {
		return nil, fmt.Errorf("storage: no 'storage' section found in config file")
	}

	return &embedded.Storage, nil
}

// LoadConfigEmbeddedWithKey loads storage configuration from a custom key in user's config file.
// Useful when user wants to use a different key name like "filestorage" or "oss".
// Environment variables in the format ${VAR} or $VAR are automatically expanded.
//
// Example:
//
//	cfg, err := storage.LoadConfigEmbeddedWithKey("config.yaml", "oss")
func LoadConfigEmbeddedWithKey(path, key string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to read config file: %w", err)
	}

	// Expand environment variables
	data = expandEnvVars(data)

	ext := strings.ToLower(filepath.Ext(path))
	var raw map[string]any

	switch ext {
	case ".yaml", ".yml", ".json":
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("storage: failed to parse config: %w", err)
		}
	default:
		return nil, fmt.Errorf("storage: unsupported config format %q", ext)
	}

	storageRaw, ok := raw[key]
	if !ok {
		return nil, fmt.Errorf("storage: key %q not found in config file", key)
	}

	// Re-marshal and unmarshal to convert to Config struct
	storageData, err := yaml.Marshal(storageRaw)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to process config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(storageData, &cfg); err != nil {
		return nil, fmt.Errorf("storage: failed to parse storage config: %w", err)
	}

	return &cfg, nil
}

// NewConfigFromMap creates a Config from a map structure.
// Useful for integrating with existing config libraries like viper, koanf, etc.
//
// Example with viper:
//
//	viper.ReadInConfig()
//	cfg, err := storage.NewConfigFromMap(viper.GetStringMap("storage"))
func NewConfigFromMap(m map[string]any) (*Config, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to process config map: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("storage: failed to parse config: %w", err)
	}

	return &cfg, nil
}
