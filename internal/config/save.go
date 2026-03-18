package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Save writes the current config to an autosave file to preserve the original YAML formatting/comments.
// Autosave path: <path without ext>.autosave<ext>
func Save(path string, cfg Config) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(autosavePath(path), b, 0o644)
}

func autosavePath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path + ".autosave"
	}
	base := strings.TrimSuffix(path, ext)
	return base + ".autosave" + ext
}
