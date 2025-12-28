package storage

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
