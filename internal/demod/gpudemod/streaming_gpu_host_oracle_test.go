package gpudemod

import "testing"

func TestStreamingGPUHostOracleComparableToCPUOracle(t *testing.T) {
	r := &BatchRunner{eng: &Engine{sampleRate: 4000000}, streamState: make(map[int64]*ExtractStreamState)}
	job := StreamingExtractJob{
		SignalID:   1,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 777,
	}
	iq := makeDeterministicIQ(16000)
	gpuLike, err := r.StreamingExtractGPUHostOracle(iq, []StreamingExtractJob{job})
	if err != nil {
		t.Fatalf("unexpected host-oracle error: %v", err)
	}
	oracleRunner := NewCPUOracleRunner(4000000)
	oracle, err := oracleRunner.StreamingExtract(iq, []StreamingExtractJob{job})
	if err != nil {
		t.Fatalf("unexpected oracle error: %v", err)
	}
	if len(gpuLike) != 1 || len(oracle) != 1 {
		t.Fatalf("unexpected result lengths: gpuLike=%d oracle=%d", len(gpuLike), len(oracle))
	}
	metrics, stats := CompareOracleAndGPUHostOracle(oracle[0], gpuLike[0])
	if stats.Count == 0 {
		t.Fatalf("expected compare count > 0")
	}
	if metrics.RefMaxAbsErr > 1e-5 {
		t.Fatalf("expected host-oracle path to match cpu oracle closely, got max abs err %f", metrics.RefMaxAbsErr)
	}
}
