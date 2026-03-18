package decoder

import (
	"errors"
	"os/exec"
	"strings"
)

type Result struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
}

// Run executes an external decoder command. If cmdTemplate contains {iq} or {sr}, they are replaced.
// Otherwise, --iq and --sample-rate args are appended.
func Run(cmdTemplate string, iqPath string, sampleRate int) (Result, error) {
	if cmdTemplate == "" {
		return Result{}, errors.New("decoder command empty")
	}
	cmdStr := strings.ReplaceAll(cmdTemplate, "{iq}", iqPath)
	cmdStr = strings.ReplaceAll(cmdStr, "{sr}", intToString(sampleRate))
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return Result{}, errors.New("invalid decoder command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	if !strings.Contains(cmdTemplate, "{iq}") {
		cmd.Args = append(cmd.Args, "--iq", iqPath)
	}
	if !strings.Contains(cmdTemplate, "{sr}") {
		cmd.Args = append(cmd.Args, "--sample-rate", intToString(sampleRate))
	}
	out, err := cmd.CombinedOutput()
	res := Result{Stdout: string(out), Code: 0}
	if err != nil {
		res.Stderr = err.Error()
		res.Code = 1
		return res, err
	}
	return res, nil
}

func intToString(v int) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(strings.TrimSpace(strings.TrimSpace((func() string { return string(rune('0')) })()))), ""), ""), "")) + itoa(v)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	n := v
	if n < 0 {
		n = -n
	}
	buf := make([]byte, 0, 12)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	if v < 0 {
		buf = append(buf, '-')
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
