package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "storage.yaml")

	configContent := `
default: local
storages:
  local:
    driver: local
    options:
      root: ./uploads
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Default != "local" {
		t.Errorf("Expected default 'local', got %q", cfg.Default)
	}

	if _, ok := cfg.Storages["local"]; !ok {
		t.Error("Expected 'local' storage to be configured")
	}

	if cfg.Storages["local"].Driver != "local" {
		t.Errorf("Expected driver 'local', got %q", cfg.Storages["local"].Driver)
	}
}

func TestLoadConfig_EnvExpansion(t *testing.T) {
	// Set env var
	os.Setenv("TEST_BUCKET", "my-test-bucket")
	defer os.Unsetenv("TEST_BUCKET")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "storage.yaml")

	configContent := `
default: s3
storages:
  s3:
    driver: s3
    options:
      bucket: ${TEST_BUCKET}
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	bucket := cfg.Storages["s3"].Options["bucket"]
	if bucket != "my-test-bucket" {
		t.Errorf("Expected bucket 'my-test-bucket', got %q", bucket)
	}
}

func TestLoadConfigEmbedded(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
app:
  name: myapp
storage:
  default: local
  storages:
    local:
      driver: local
      options:
        root: ./data
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfigEmbedded(configPath)
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	if cfg.Default != "local" {
		t.Errorf("Expected default 'local', got %q", cfg.Default)
	}
}

func TestLoadConfigEmbeddedWithKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
app:
  name: myapp
oss:
  default: aliyun
  storages:
    aliyun:
      driver: aliyun
      options:
        bucket: test
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfigEmbeddedWithKey(configPath, "oss")
	if err != nil {
		t.Fatalf("LoadConfigEmbeddedWithKey failed: %v", err)
	}

	if cfg.Default != "aliyun" {
		t.Errorf("Expected default 'aliyun', got %q", cfg.Default)
	}
}

func TestNewConfigFromMap(t *testing.T) {
	m := map[string]any{
		"default": "local",
		"storages": map[string]any{
			"local": map[string]any{
				"driver": "local",
				"options": map[string]any{
					"root": "./uploads",
				},
			},
		},
	}

	cfg, err := NewConfigFromMap(m)
	if err != nil {
		t.Fatalf("NewConfigFromMap failed: %v", err)
	}

	if cfg.Default != "local" {
		t.Errorf("Expected default 'local', got %q", cfg.Default)
	}
}

func TestExpandEnvVars(t *testing.T) {
	os.Setenv("TEST_VAR1", "value1")
	os.Setenv("TEST_VAR2", "value2")
	defer os.Unsetenv("TEST_VAR1")
	defer os.Unsetenv("TEST_VAR2")

	tests := []struct {
		input    string
		expected string
	}{
		{"${TEST_VAR1}", "value1"},
		{"$TEST_VAR2", "value2"},
		{"prefix_${TEST_VAR1}_suffix", "prefix_value1_suffix"},
		{"${NONEXISTENT}", "${NONEXISTENT}"}, // Keep original if not set
	}

	for _, tt := range tests {
		got := string(expandEnvVars([]byte(tt.input)))
		if got != tt.expected {
			t.Errorf("expandEnvVars(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
