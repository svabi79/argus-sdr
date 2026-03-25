package gpudemod

import "testing"

func TestStreamingExtractGPUExecUsesSafeDefaultMode(t *testing.T) {
	r := &BatchRunner{eng: &Engine{sampleRate: 4000000}, streamState: make(map[int64]*ExtractStreamState)}
	job := StreamingExtractJob{
		SignalID:   1,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 777,
	}
	res, err := r.StreamingExtractGPUExec(makeDeterministicIQ(2048), []StreamingExtractJob{job})
	if err != nil {
		t.Fatalf("expected safe default execution path, got error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Rate != job.OutRate {
		t.Fatalf("expected output rate %d, got %d", job.OutRate, res[0].Rate)
	}
	if res[0].NOut <= 0 {
		t.Fatalf("expected streaming output samples")
	}
}

func TestStreamingGPUExecMatchesCPUOracleAcrossChunkPatterns(t *testing.T) {
	job := StreamingExtractJob{
		SignalID:   1,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 777,
	}
	t.Run("DeterministicIQ", func(t *testing.T) {
		r := &BatchRunner{eng: &Engine{sampleRate: 4000000}, streamState: make(map[int64]*ExtractStreamState)}
		steps := makeStreamingValidationSteps(
			makeDeterministicIQ(1500),
			[]int{0, 1, 2, 17, 63, 64, 65, 129, 511},
			[]StreamingExtractJob{job},
		)
		runStreamingExecSequenceAgainstOracle(t, r, steps, 1e-5, 1e-9)
	})
	t.Run("ToneNoiseIQ", func(t *testing.T) {
		r := &BatchRunner{eng: &Engine{sampleRate: 4000000}, streamState: make(map[int64]*ExtractStreamState)}
		steps := makeStreamingValidationSteps(
			makeToneNoiseIQ(4096, 0.023),
			[]int{7, 20, 3, 63, 64, 65, 777},
			[]StreamingExtractJob{job},
		)
		runStreamingExecSequenceAgainstOracle(t, r, steps, 1e-5, 1e-9)
	})
}

func TestStreamingGPUExecLifecycleMatchesCPUOracle(t *testing.T) {
	r := &BatchRunner{
		eng:         &Engine{sampleRate: 4000000},
		streamState: make(map[int64]*ExtractStreamState),
		nativeState: make(map[int64]*nativeStreamingSignalState),
	}
	baseA := StreamingExtractJob{
		SignalID:   11,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 1001,
	}
	baseB := StreamingExtractJob{
		SignalID:   22,
		OffsetHz:   -18750,
		Bandwidth:  16000,
		OutRate:    100000,
		NumTaps:    33,
		ConfigHash: 2002,
	}
	steps := []streamingValidationStep{
		{
			name: "prime_both_signals",
			iq:   makeDeterministicIQ(512),
			jobs: []StreamingExtractJob{baseA, baseB},
		},
		{
			name: "config_reset_with_zero_new",
			iq:   nil,
			jobs: []StreamingExtractJob{{SignalID: baseA.SignalID, OffsetHz: baseA.OffsetHz, Bandwidth: baseA.Bandwidth, OutRate: baseA.OutRate, NumTaps: baseA.NumTaps, ConfigHash: baseA.ConfigHash + 1}, baseB},
		},
		{
			name: "signal_b_disappears",
			iq:   makeToneNoiseIQ(96, 0.041),
			jobs: []StreamingExtractJob{baseA},
		},
		{
			name: "signal_b_reappears_fresh",
			iq:   makeDeterministicIQ(160),
			jobs: []StreamingExtractJob{baseA, baseB},
		},
		{
			name: "small_history_boundary_chunk",
			iq:   makeToneNoiseIQ(65, 0.017),
			jobs: []StreamingExtractJob{baseA, baseB},
		},
	}
	runStreamingExecSequenceAgainstOracle(t, r, steps, 1e-5, 1e-9)
	if _, ok := r.nativeState[baseB.SignalID]; ok {
		t.Fatalf("expected safe host-oracle path to keep native state inactive while gate is off")
	}
}
