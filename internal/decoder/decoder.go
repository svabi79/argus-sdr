package decoder

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

type Result struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
}

// Run executes an external decoder command. If cmdTemplate contains {iq}/{sr}/{audio}, they are replaced.
// Otherwise, --iq and --sample-rate args are appended.
func Run(cmdTemplate string, iqPath string, sampleRate int, audioPath string) (Result, error) {
	if cmdTemplate == "" {
		return Result{}, errors.New("decoder command empty")
	}
	cmdStr := strings.ReplaceAll(cmdTemplate, "{iq}", iqPath)
	cmdStr = strings.ReplaceAll(cmdStr, "{sr}", strconv.Itoa(sampleRate))
	if strings.Contains(cmdTemplate, "{audio}") {
		if audioPath == "" {
			return Result{}, errors.New("audio path required for decoder")
		}
		cmdStr = strings.ReplaceAll(cmdStr, "{audio}", audioPath)
	}
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return Result{}, errors.New("invalid decoder command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	if !strings.Contains(cmdTemplate, "{iq}") {
		cmd.Args = append(cmd.Args, "--iq", iqPath)
	}
	if !strings.Contains(cmdTemplate, "{sr}") {
		cmd.Args = append(cmd.Args, "--sample-rate", strconv.Itoa(sampleRate))
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
