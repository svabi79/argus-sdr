package main

import (
	"testing"

	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/pipeline"
)

func TestDecisionCanEnableRecorderFlags(t *testing.T) {
	rt := &dspRuntime{}
	rt.cfg.Recorder.Enabled = false
	rt.cfg.Recorder.AutoDecode = false
	policy := pipeline.Policy{AutoRecordClasses: []string{"WFM"}, AutoDecodeClasses: []string{"WFM"}}
	decision := pipeline.DecideSignalAction(policy, pipeline.Candidate{ID: 1, Hint: "WFM"}, &classifier.Classification{ModType: classifier.ClassWFM})
	if decision.ShouldRecord {
		rt.cfg.Recorder.Enabled = true
	}
	if decision.ShouldAutoDecode {
		rt.cfg.Recorder.AutoDecode = true
	}
	if !rt.cfg.Recorder.Enabled || !rt.cfg.Recorder.AutoDecode {
		t.Fatalf("expected recorder flags to be enabled")
	}
}
