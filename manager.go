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
