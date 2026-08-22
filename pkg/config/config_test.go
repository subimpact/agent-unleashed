package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadAndSave(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfgPath := filepath.Join(tempDir, "config.yaml")

	cfg := &AppConfig{
		System: SystemConfig{
			AgentName: "Test-Agent",
		},
		Model: ModelConfig{
			Driver: "claude",
		},
	}

	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.System.AgentName != "Test-Agent" {
		t.Fatalf("Expected AgentName 'Test-Agent', got '%s'", loaded.System.AgentName)
	}
	if loaded.Model.Driver != "claude" {
		t.Fatalf("Expected Driver 'claude', got '%s'", loaded.Model.Driver)
	}
}
