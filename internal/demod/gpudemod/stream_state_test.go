package gpudemod

import "testing"

func TestGetOrInitExtractStateInitializesPolyphaseAndHistory(t *testing.T) {
	r := &BatchRunner{streamState: make(map[int64]*ExtractStreamState)}
	job := StreamingExtractJob{
		SignalID:   7,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 555,
	}
	state, err := r.getOrInitExtractState(job, 4000000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Decim != 20 {
		t.Fatalf("unexpected decim: %d", state.Decim)
	}
	if len(state.BaseTaps) != 65 {
		t.Fatalf("unexpected base taps len: %d", len(state.BaseTaps))
	}
	if len(state.PolyphaseTaps) == 0 {
		t.Fatalf("expected polyphase taps")
	}
	if cap(state.ShiftedHistory) < 64 {
		t.Fatalf("expected shifted history capacity >= 64, got %d", cap(state.ShiftedHistory))
	}
}
