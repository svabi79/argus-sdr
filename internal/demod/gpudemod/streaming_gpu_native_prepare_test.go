//go:build cufft && windows

package gpudemod

import (
	"os"
	"path/filepath"
	"testing"
)

func configureNativePreparedDLLPath(t *testing.T) {
	t.Helper()
	candidates := []string{
		filepath.Join("build", "gpudemod_kernels.dll"),
		filepath.Join("internal", "demod", "gpudemod", "build", "gpudemod_kernels.dll"),
		"gpudemod_kernels.dll",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				t.Fatalf("resolve native prepared DLL path: %v", err)
			}
			t.Setenv("GPUMOD_DLL", abs)
			return
		}
	}
}

func requireNativePreparedTestRunner(t *testing.T) *BatchRunner {
	t.Helper()
	configureNativePreparedDLLPath(t)
	if err := ensureDLLLoaded(); err != nil {
		t.Skipf("native prepared path unavailable: %v", err)
	}
	if !Available() {
		t.Skip("native prepared path unavailable: cuda device not available")
	}
	r, err := NewBatchRunner(32768, 4000000)
	if err != nil {
		t.Skipf("native prepared path unavailable: %v", err)
	}
	t.Cleanup(r.Close)
	return r
}

func TestStreamingGPUNativePreparedMatchesCPUOracleAcrossChunkPatterns(t *testing.T) {
	job := StreamingExtractJob{
		SignalID:   1,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 777,
	}
	exec := func(r *BatchRunner, invocations []StreamingGPUInvocation) ([]StreamingGPUExecutionResult, error) {
		return r.executeStreamingGPUNativePrepared(invocations)
	}
	t.Run("DeterministicIQ", func(t *testing.T) {
		r := requireNativePreparedTestRunner(t)
		steps := makeStreamingValidationSteps(
			makeDeterministicIQ(8192),
			[]int{0, 1, 2, 17, 63, 64, 65, 129, 511, 2048},
			[]StreamingExtractJob{job},
		)
		runPreparedSequenceAgainstOracle(t, r, exec, steps, 1e-4, 1e-8)
	})
	t.Run("ToneNoiseIQ", func(t *testing.T) {
		r := requireNativePreparedTestRunner(t)
		steps := makeStreamingValidationSteps(
			makeToneNoiseIQ(12288, 0.023),
			[]int{7, 20, 3, 63, 64, 65, 777, 2048, 4096},
			[]StreamingExtractJob{job},
		)
		runPreparedSequenceAgainstOracle(t, r, exec, steps, 1e-4, 1e-8)
	})
}

func TestStreamingGPUNativePreparedLifecycleResetAndCapacity(t *testing.T) {
	r := requireNativePreparedTestRunner(t)
	exec := func(invocations []StreamingGPUInvocation) ([]StreamingGPUExecutionResult, error) {
		return r.executeStreamingGPUNativePrepared(invocations)
	}
	jobA := StreamingExtractJob{
		SignalID:   11,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 3001,
	}
	jobB := StreamingExtractJob{
		SignalID:   22,
		OffsetHz:   -18750,
		Bandwidth:  16000,
		OutRate:    100000,
		NumTaps:    33,
		ConfigHash: 4002,
	}

	steps := []streamingValidationStep{
		{
			name: "prime_both_signals",
			iq:   makeDeterministicIQ(256),
			jobs: []StreamingExtractJob{jobA, jobB},
		},
		{
			name: "grow_capacity",
			iq:   makeToneNoiseIQ(4096, 0.037),
			jobs: []StreamingExtractJob{jobA, jobB},
		},
		{
			name: "config_reset_zero_new",
			iq:   nil,
			jobs: []StreamingExtractJob{{SignalID: jobA.SignalID, OffsetHz: jobA.OffsetHz, Bandwidth: jobA.Bandwidth, OutRate: jobA.OutRate, NumTaps: jobA.NumTaps, ConfigHash: jobA.ConfigHash + 1}, jobB},
		},
		{
			name: "signal_b_disappears",
			iq:   makeDeterministicIQ(64),
			jobs: []StreamingExtractJob{jobA},
		},
		{
			name: "signal_b_reappears",
			iq:   makeToneNoiseIQ(96, 0.017),
			jobs: []StreamingExtractJob{jobA, jobB},
		},
		{
			name: "history_boundary",
			iq:   makeDeterministicIQ(65),
			jobs: []StreamingExtractJob{jobA, jobB},
		},
	}

	oracle := NewCPUOracleRunner(r.eng.sampleRate)
	var grownCap int
	for idx, step := range steps {
		invocations, err := r.buildStreamingGPUInvocations(step.iq, step.jobs)
		if err != nil {
			t.Fatalf("step %d (%s): build invocations failed: %v", idx, step.name, err)
		}
		got, err := exec(invocations)
		if err != nil {
			t.Fatalf("step %d (%s): native prepared exec failed: %v", idx, step.name, err)
		}
		want, err := oracle.StreamingExtract(step.iq, step.jobs)
		if err != nil {
			t.Fatalf("step %d (%s): oracle failed: %v", idx, step.name, err)
		}
		if len(got) != len(want) {
			t.Fatalf("step %d (%s): result count mismatch: got=%d want=%d", idx, step.name, len(got), len(want))
		}
		applied := r.applyStreamingGPUExecutionResults(got)
		for i, job := range step.jobs {
			oracleState := oracle.States[job.SignalID]
			requirePreparedExecutionResultMatchesOracle(t, got[i], want[i], oracleState, 1e-4, 1e-8)
			requireStreamingExtractResultMatchesOracle(t, applied[i], want[i])
			requireExtractStateMatchesOracle(t, r.streamState[job.SignalID], oracleState, 1e-8, 1e-4)

			state := r.nativeState[job.SignalID]
			if state == nil {
				t.Fatalf("step %d (%s): missing native state for signal %d", idx, step.name, job.SignalID)
			}
			if state.configHash != job.ConfigHash {
				t.Fatalf("step %d (%s): native config hash mismatch for signal %d: got=%d want=%d", idx, step.name, job.SignalID, state.configHash, job.ConfigHash)
			}
			if state.decim != oracleState.Decim {
				t.Fatalf("step %d (%s): native decim mismatch for signal %d: got=%d want=%d", idx, step.name, job.SignalID, state.decim, oracleState.Decim)
			}
			if state.numTaps != oracleState.NumTaps {
				t.Fatalf("step %d (%s): native num taps mismatch for signal %d: got=%d want=%d", idx, step.name, job.SignalID, state.numTaps, oracleState.NumTaps)
			}
			if state.historyCap != maxInt(0, oracleState.NumTaps-1) {
				t.Fatalf("step %d (%s): native history cap mismatch for signal %d: got=%d want=%d", idx, step.name, job.SignalID, state.historyCap, maxInt(0, oracleState.NumTaps-1))
			}
			if state.historyLen != len(oracleState.ShiftedHistory) {
				t.Fatalf("step %d (%s): native history len mismatch for signal %d: got=%d want=%d", idx, step.name, job.SignalID, state.historyLen, len(oracleState.ShiftedHistory))
			}
			if len(step.iq) > 0 && state.shiftedCap < len(step.iq) {
				t.Fatalf("step %d (%s): native shifted capacity too small for signal %d: got=%d need>=%d", idx, step.name, job.SignalID, state.shiftedCap, len(step.iq))
			}
			if state.outCap < got[i].NOut {
				t.Fatalf("step %d (%s): native out capacity too small for signal %d: got=%d need>=%d", idx, step.name, job.SignalID, state.outCap, got[i].NOut)
			}
			if job.SignalID == jobA.SignalID && state.shiftedCap > grownCap {
				grownCap = state.shiftedCap
			}
		}
		if step.name == "grow_capacity" && grownCap < len(step.iq) {
			t.Fatalf("expected capacity growth for signal %d, got=%d want>=%d", jobA.SignalID, grownCap, len(step.iq))
		}
		if step.name == "config_reset_zero_new" {
			state := r.nativeState[jobA.SignalID]
			if state == nil {
				t.Fatalf("missing native state for signal %d after config reset", jobA.SignalID)
			}
			if state.historyLen != 0 {
				t.Fatalf("expected cleared native history after config reset, got=%d", state.historyLen)
			}
		}
		if step.name == "signal_b_disappears" {
			if _, ok := r.nativeState[jobB.SignalID]; ok {
				t.Fatalf("expected native state for signal %d to be removed on disappearance", jobB.SignalID)
			}
		}
	}
}
