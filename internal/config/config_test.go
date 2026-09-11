package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsNodeGroupLabels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "nodeGroupLabels:\n  - node.danskespil.dk/workload\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NodeGroupLabels) != 1 || cfg.NodeGroupLabels[0] != "node.danskespil.dk/workload" {
		t.Fatalf("got %v", cfg.NodeGroupLabels)
	}
}

func TestLoadMissingDefaultFileIsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NodeGroupLabels != nil {
		t.Fatalf("got %v, want nil", cfg.NodeGroupLabels)
	}
}

func TestLoadMissingExplicitFileFails(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("expected an error for a missing explicit config")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("nodeGroupLabel: typo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected an error for an unknown field")
	}
}
