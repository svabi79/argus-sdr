package gpudemod

import "testing"

func TestStreamingExtractGPUExecUnavailableByDefault(t *testing.T) {
	r := &BatchRunner{eng: &Engine{sampleRate: 4000000}, streamState: make(map[int64]*ExtractStreamState)}
	job := StreamingExtractJob{
		SignalID:   1,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 777,
	}
	_, err := r.StreamingExtractGPUExec(makeDeterministicIQ(2048), []StreamingExtractJob{job})
	if err == nil {
		t.Fatalf("expected unavailable/disabled execution path by default")
	}
}
