//go:build cufft && windows

package gpudemod

import "testing"

func TestStreamingGPUNativePreparedComparableToCPUOracle(t *testing.T) {
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
	gpuRes, err := r.StreamingExtractGPU(iq, []StreamingExtractJob{job})
	if err != nil {
		t.Fatalf("unexpected native prepared GPU error: %v", err)
	}
	oracleRunner := NewCPUOracleRunner(4000000)
	oracleRes, err := oracleRunner.StreamingExtract(iq, []StreamingExtractJob{job})
	if err != nil {
		t.Fatalf("unexpected oracle error: %v", err)
	}
	if len(gpuRes) != 1 || len(oracleRes) != 1 {
		t.Fatalf("unexpected result sizes: gpu=%d oracle=%d", len(gpuRes), len(oracleRes))
	}
	metrics, stats := CompareOracleAndGPUHostOracle(oracleRes[0], gpuRes[0])
	if stats.Count == 0 {
		t.Fatalf("expected compare count > 0")
	}
	if metrics.RefMaxAbsErr > 1e-4 {
		t.Fatalf("native prepared path diverges too much from oracle: max abs err=%f", metrics.RefMaxAbsErr)
	}
}
