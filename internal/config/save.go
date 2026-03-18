package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Save writes the current config back to disk.
func Save(path string, cfg Config) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
