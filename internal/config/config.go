// Package config loads the optional kube-waste configuration file.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// Config mirrors the YAML file. Every field is optional; the zero value keeps
// the built-in behaviour.
type Config struct {
	// NodeGroupLabels are node label keys tried, in order, before the
	// built-in node-pool label and node-role detection.
	NodeGroupLabels []string `json:"nodeGroupLabels"`
}

// DefaultPath is the config location used when none is given: normally
// ~/.config/kube-waste/config.yaml, honouring XDG_CONFIG_HOME.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "kube-waste", "config.yaml")
}

// Load reads the config file at path, or DefaultPath when path is empty. A
// missing default file is not an error; a missing explicit one is.
func Load(path string) (Config, error) {
	explicit := path != ""
	if !explicit {
		if path = DefaultPath(); path == "" {
			return Config{}, nil
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}
