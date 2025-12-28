package storage

import (
	"fmt"
	"sync"
)

// Manager manages multiple storage backends based on configuration.
type Manager struct {
	config   *Config
	storages map[string]Storage
	mu       sync.RWMutex
}

// NewManager creates a new storage manager from configuration.
func NewManager(cfg *Config) *Manager {
	return &Manager{
		config:   cfg,
		storages: make(map[string]Storage),
	}
}

// NewManagerFromFile creates a new storage manager from a config file.
func NewManagerFromFile(path string) (*Manager, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return NewManager(cfg), nil
}

// NewManagerFromEnv creates a new storage manager from environment variable.
func NewManagerFromEnv(envVar string) (*Manager, error) {
	cfg, err := LoadConfigFromEnv(envVar)
	if err != nil {
		return nil, err
	}
	return NewManager(cfg), nil
}

// Disk returns a storage backend by name.
// If name is empty, returns the default storage.
func (m *Manager) Disk(name string) (Storage, error) {
	if name == "" {
		name = m.config.Default
	}
	if name == "" {
		return nil, fmt.Errorf("storage: no default storage configured")
	}

	// Check if already initialized
	m.mu.RLock()
	if s, ok := m.storages[name]; ok {
		m.mu.RUnlock()
		return s, nil
	}
	m.mu.RUnlock()

	// Initialize the storage
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if s, ok := m.storages[name]; ok {
		return s, nil
	}

	cfg, ok := m.config.Storages[name]
	if !ok {
		return nil, fmt.Errorf("storage: disk %q not configured", name)
	}

	s, err := Open(cfg.Driver, cfg.Options)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to open disk %q: %w", name, err)
	}

	m.storages[name] = s
	return s, nil
}

// Default returns the default storage backend.
func (m *Manager) Default() (Storage, error) {
	return m.Disk("")
}

// Close closes all initialized storage backends.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, s := range m.storages {
		if err := s.Close(); err != nil {
			lastErr = fmt.Errorf("storage: failed to close %q: %w", name, err)
		}
	}
	m.storages = make(map[string]Storage)
	return lastErr
}

// Global manager instance for convenience.
var (
	globalManager *Manager
	globalMu      sync.RWMutex
)

// Init initializes the global storage manager from a config file.
func Init(path string) error {
	mgr, err := NewManagerFromFile(path)
	if err != nil {
		return err
	}
	globalMu.Lock()
	globalManager = mgr
	globalMu.Unlock()
	return nil
}

// InitEmbedded initializes the global storage manager from an embedded config.
// The storage config should be under the "storage" key in the config file.
//
// Example config.yaml:
//
//	app:
//	  name: myapp
//	storage:
//	  default: local
//	  storages:
//	    local:
//	      driver: local
//	      options:
//	        root: ./uploads
func InitEmbedded(path string) error {
	cfg, err := LoadConfigEmbedded(path)
	if err != nil {
		return err
	}
	globalMu.Lock()
	globalManager = NewManager(cfg)
	globalMu.Unlock()
	return nil
}

// InitEmbeddedWithKey initializes from a custom key in the config file.
//
// Example:
//
//	storage.InitEmbeddedWithKey("config.yaml", "oss")
func InitEmbeddedWithKey(path, key string) error {
	cfg, err := LoadConfigEmbeddedWithKey(path, key)
	if err != nil {
		return err
	}
	globalMu.Lock()
	globalManager = NewManager(cfg)
	globalMu.Unlock()
	return nil
}

// InitFromConfig initializes the global storage manager from a Config struct.
// Useful when you've already loaded config via viper, koanf, or other libraries.
func InitFromConfig(cfg *Config) {
	globalMu.Lock()
	globalManager = NewManager(cfg)
	globalMu.Unlock()
}

// InitFromEnv initializes the global storage manager from environment variable.
func InitFromEnv(envVar string) error {
	mgr, err := NewManagerFromEnv(envVar)
	if err != nil {
		return err
	}
	globalMu.Lock()
	globalManager = mgr
	globalMu.Unlock()
	return nil
}
