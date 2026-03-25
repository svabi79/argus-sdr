package gpudemod

import (
	"math"
	"testing"
)

type streamingValidationStep struct {
	name string
	iq   []complex64
	jobs []StreamingExtractJob
}

type streamingPreparedExecutor func(*BatchRunner, []StreamingGPUInvocation) ([]StreamingGPUExecutionResult, error)

func makeToneNoiseIQ(n int, phaseInc float64) []complex64 {
	out := make([]complex64, n)
	phase := 0.0
	for i := 0; i < n; i++ {
		tone := complex(math.Cos(phase), math.Sin(phase))
		noiseI := 0.17*math.Cos(0.113*float64(i)+0.31) + 0.07*math.Sin(0.071*float64(i))
		noiseQ := 0.13*math.Sin(0.097*float64(i)+0.11) - 0.05*math.Cos(0.043*float64(i))
		out[i] = complex64(0.85*tone + 0.15*complex(noiseI, noiseQ))
		phase += phaseInc
	}
	return out
}

func makeStreamingValidationSteps(iq []complex64, chunkSizes []int, jobs []StreamingExtractJob) []streamingValidationStep {
	steps := make([]streamingValidationStep, 0, len(chunkSizes)+1)
	pos := 0
	for idx, n := range chunkSizes {
		if n < 0 {
			n = 0
		}
		end := pos + n
		if end > len(iq) {
			end = len(iq)
		}
		steps = append(steps, streamingValidationStep{
			name: "chunk",
			iq:   append([]complex64(nil), iq[pos:end]...),
			jobs: append([]StreamingExtractJob(nil), jobs...),
		})
		_ = idx
		pos = end
	}
	if pos < len(iq) {
		steps = append(steps, streamingValidationStep{
			name: "remainder",
			iq:   append([]complex64(nil), iq[pos:]...),
			jobs: append([]StreamingExtractJob(nil), jobs...),
		})
	}
	return steps
}

func requirePhaseClose(t *testing.T, got float64, want float64, tol float64) {
	t.Helper()
	diff := got - want
	for diff > math.Pi {
		diff -= 2 * math.Pi
	}
	for diff < -math.Pi {
		diff += 2 * math.Pi
	}
	if math.Abs(diff) > tol {
		t.Fatalf("phase mismatch: got=%0.12f want=%0.12f diff=%0.12f tol=%0.12f", got, want, diff, tol)
	}
}

func requireStreamingExtractResultMatchesOracle(t *testing.T, got StreamingExtractResult, want StreamingExtractResult) {
	t.Helper()
	if got.SignalID != want.SignalID {
		t.Fatalf("signal id mismatch: got=%d want=%d", got.SignalID, want.SignalID)
	}
	if got.Rate != want.Rate {
		t.Fatalf("rate mismatch for signal %d: got=%d want=%d", got.SignalID, got.Rate, want.Rate)
	}
	if got.NOut != want.NOut {
		t.Fatalf("n_out mismatch for signal %d: got=%d want=%d", got.SignalID, got.NOut, want.NOut)
	}
	if got.PhaseCount != want.PhaseCount {
		t.Fatalf("phase count mismatch for signal %d: got=%d want=%d", got.SignalID, got.PhaseCount, want.PhaseCount)
	}
	if got.HistoryLen != want.HistoryLen {
		t.Fatalf("history len mismatch for signal %d: got=%d want=%d", got.SignalID, got.HistoryLen, want.HistoryLen)
	}
}

func requirePreparedExecutionResultMatchesOracle(t *testing.T, got StreamingGPUExecutionResult, want StreamingExtractResult, oracleState *CPUOracleState, sampleTol float64, phaseTol float64) {
	t.Helper()
	if oracleState == nil {
		t.Fatalf("missing oracle state for signal %d", got.SignalID)
	}
	if got.SignalID != want.SignalID {
		t.Fatalf("signal id mismatch: got=%d want=%d", got.SignalID, want.SignalID)
	}
	if got.Rate != want.Rate {
		t.Fatalf("rate mismatch for signal %d: got=%d want=%d", got.SignalID, got.Rate, want.Rate)
	}
	if got.NOut != want.NOut {
		t.Fatalf("n_out mismatch for signal %d: got=%d want=%d", got.SignalID, got.NOut, want.NOut)
	}
	if got.PhaseCountOut != oracleState.PhaseCount {
		t.Fatalf("phase count mismatch for signal %d: got=%d want=%d", got.SignalID, got.PhaseCountOut, oracleState.PhaseCount)
	}
	requirePhaseClose(t, got.NCOPhaseOut, oracleState.NCOPhase, phaseTol)
	if got.HistoryLenOut != len(oracleState.ShiftedHistory) {
		t.Fatalf("history len mismatch for signal %d: got=%d want=%d", got.SignalID, got.HistoryLenOut, len(oracleState.ShiftedHistory))
	}
	requireComplexSlicesClose(t, got.IQ, want.IQ, sampleTol)
	requireComplexSlicesClose(t, got.HistoryOut, oracleState.ShiftedHistory, sampleTol)
}

func requireExtractStateMatchesOracle(t *testing.T, got *ExtractStreamState, want *CPUOracleState, phaseTol float64, sampleTol float64) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("state mismatch: got nil=%t want nil=%t", got == nil, want == nil)
	}
	if got.SignalID != want.SignalID {
		t.Fatalf("signal id mismatch: got=%d want=%d", got.SignalID, want.SignalID)
	}
	if got.ConfigHash != want.ConfigHash {
		t.Fatalf("config hash mismatch for signal %d: got=%d want=%d", got.SignalID, got.ConfigHash, want.ConfigHash)
	}
	if got.Decim != want.Decim {
		t.Fatalf("decim mismatch for signal %d: got=%d want=%d", got.SignalID, got.Decim, want.Decim)
	}
	if got.NumTaps != want.NumTaps {
		t.Fatalf("num taps mismatch for signal %d: got=%d want=%d", got.SignalID, got.NumTaps, want.NumTaps)
	}
	if got.PhaseCount != want.PhaseCount {
		t.Fatalf("phase count mismatch for signal %d: got=%d want=%d", got.SignalID, got.PhaseCount, want.PhaseCount)
	}
	requirePhaseClose(t, got.NCOPhase, want.NCOPhase, phaseTol)
	requireComplexSlicesClose(t, got.ShiftedHistory, want.ShiftedHistory, sampleTol)
}

func requireStateKeysMatchOracle(t *testing.T, got map[int64]*ExtractStreamState, want map[int64]*CPUOracleState) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("active state count mismatch: got=%d want=%d", len(got), len(want))
	}
	for signalID := range want {
		if got[signalID] == nil {
			t.Fatalf("missing active state for signal %d", signalID)
		}
	}
	for signalID := range got {
		if want[signalID] == nil {
			t.Fatalf("unexpected active state for signal %d", signalID)
		}
	}
}

func runStreamingExecSequenceAgainstOracle(t *testing.T, runner *BatchRunner, steps []streamingValidationStep, sampleTol float64, phaseTol float64) {
	t.Helper()
	oracle := NewCPUOracleRunner(runner.eng.sampleRate)
	for idx, step := range steps {
		got, err := runner.StreamingExtractGPUExec(step.iq, step.jobs)
		if err != nil {
			t.Fatalf("step %d (%s): exec failed: %v", idx, step.name, err)
		}
		want, err := oracle.StreamingExtract(step.iq, step.jobs)
		if err != nil {
			t.Fatalf("step %d (%s): oracle failed: %v", idx, step.name, err)
		}
		if len(got) != len(want) {
			t.Fatalf("step %d (%s): result count mismatch: got=%d want=%d", idx, step.name, len(got), len(want))
		}
		for i, job := range step.jobs {
			requireStreamingExtractResultMatchesOracle(t, got[i], want[i])
			requireComplexSlicesClose(t, got[i].IQ, want[i].IQ, sampleTol)
			requireExtractStateMatchesOracle(t, runner.streamState[job.SignalID], oracle.States[job.SignalID], phaseTol, sampleTol)
		}
		requireStateKeysMatchOracle(t, runner.streamState, oracle.States)
	}
}

func runPreparedSequenceAgainstOracle(t *testing.T, runner *BatchRunner, exec streamingPreparedExecutor, steps []streamingValidationStep, sampleTol float64, phaseTol float64) {
	t.Helper()
	oracle := NewCPUOracleRunner(runner.eng.sampleRate)
	for idx, step := range steps {
		invocations, err := runner.buildStreamingGPUInvocations(step.iq, step.jobs)
		if err != nil {
			t.Fatalf("step %d (%s): build invocations failed: %v", idx, step.name, err)
		}
		got, err := exec(runner, invocations)
		if err != nil {
			t.Fatalf("step %d (%s): prepared exec failed: %v", idx, step.name, err)
		}
		want, err := oracle.StreamingExtract(step.iq, step.jobs)
		if err != nil {
			t.Fatalf("step %d (%s): oracle failed: %v", idx, step.name, err)
		}
		if len(got) != len(want) {
			t.Fatalf("step %d (%s): result count mismatch: got=%d want=%d", idx, step.name, len(got), len(want))
		}
		applied := runner.applyStreamingGPUExecutionResults(got)
		if len(applied) != len(want) {
			t.Fatalf("step %d (%s): applied result count mismatch: got=%d want=%d", idx, step.name, len(applied), len(want))
		}
		for i, job := range step.jobs {
			oracleState := oracle.States[job.SignalID]
			requirePreparedExecutionResultMatchesOracle(t, got[i], want[i], oracleState, sampleTol, phaseTol)
			requireStreamingExtractResultMatchesOracle(t, applied[i], want[i])
			requireComplexSlicesClose(t, applied[i].IQ, want[i].IQ, sampleTol)
			requireExtractStateMatchesOracle(t, runner.streamState[job.SignalID], oracleState, phaseTol, sampleTol)
		}
		requireStateKeysMatchOracle(t, runner.streamState, oracle.States)
	}
}
