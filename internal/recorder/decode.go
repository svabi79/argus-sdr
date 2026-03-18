package recorder

import (
	"encoding/json"
	"path/filepath"

	"sdr-visual-suite/internal/decoder"
)

func (m *Manager) runDecodeIfConfigured(mod string, iqPath string, sampleRate int, files map[string]any, dir string) {
	if !m.policy.AutoDecode || mod == "" {
		return
	}
	cmd := ""
	if m.decodeCommands != nil {
		cmd = m.decodeCommands[mod]
	}
	if cmd == "" {
		return
	}
	res, err := decoder.Run(cmd, iqPath, sampleRate)
	if err != nil {
		return
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	path := filepath.Join(dir, "decode.json")
	_ = writeFile(path, b)
	files["decode"] = "decode.json"
}
