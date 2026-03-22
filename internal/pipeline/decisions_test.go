package pipeline

import (
	"testing"

	"sdr-wideband-suite/internal/classifier"
)

func TestDecideSignalAction(t *testing.T) {
	policy := Policy{AutoRecordClasses: []string{"WFM"}, AutoDecodeClasses: []string{"RDS", "WFM"}}
	cls := &classifier.Classification{ModType: classifier.ClassWFM}
	decision := DecideSignalAction(policy, Candidate{ID: 1, Hint: "WFM"}, cls)
	if !decision.ShouldRecord {
		t.Fatalf("expected record decision")
	}
	if !decision.ShouldAutoDecode {
		t.Fatalf("expected auto decode decision")
	}
}

func TestDecideSignalActionUsesHintWithoutClass(t *testing.T) {
	policy := Policy{AutoRecordClasses: []string{"WFM"}, AutoDecodeClasses: []string{"FT8"}}
	decision := DecideSignalAction(policy, Candidate{ID: 2, Hint: "WFM"}, nil)
	if !decision.ShouldRecord {
		t.Fatalf("expected record decision from hint")
	}
	if decision.ShouldAutoDecode {
		t.Fatalf("unexpected auto decode decision from hint")
	}
	if decision.Reason == "" {
		t.Fatalf("expected reason for hint-based decision")
	}
}

func TestDecideSignalActionWindowAutoRecord(t *testing.T) {
	policy := Policy{
		MonitorWindows: finalizeMonitorWindows([]MonitorWindow{{
			Label:      "record-zone",
			StartHz:    100,
			EndHz:      200,
			CenterHz:   150,
			SpanHz:     100,
			AutoRecord: true,
		}}),
	}
	decision := DecideSignalAction(policy, Candidate{ID: 3, CenterHz: 150}, nil)
	if !decision.ShouldRecord {
		t.Fatalf("expected window auto record decision")
	}
	if decision.Reason != DecisionReasonRecordWindow {
		t.Fatalf("expected window record reason, got %q", decision.Reason)
	}
	if decision.RecordWindow == nil || decision.RecordWindow.Label != "record-zone" {
		t.Fatalf("expected record window match to be set")
	}
}

func TestDecideSignalActionWindowAutoDecode(t *testing.T) {
	policy := Policy{
		MonitorWindows: finalizeMonitorWindows([]MonitorWindow{{
			Label:      "decode-zone",
			StartHz:    300,
			EndHz:      350,
			CenterHz:   325,
			SpanHz:     50,
			AutoDecode: true,
		}}),
	}
	decision := DecideSignalAction(policy, Candidate{ID: 4, CenterHz: 325}, nil)
	if !decision.ShouldAutoDecode {
		t.Fatalf("expected window auto decode decision")
	}
	if decision.Reason != DecisionReasonDecodeWindow {
		t.Fatalf("expected window decode reason, got %q", decision.Reason)
	}
	if decision.DecodeWindow == nil || decision.DecodeWindow.Label != "decode-zone" {
		t.Fatalf("expected decode window match to be set")
	}
}

func TestDecideSignalActionOverlappingRecordDecodeWindows(t *testing.T) {
	policy := Policy{
		MonitorWindows: finalizeMonitorWindows([]MonitorWindow{
			{
				Label:      "record-zone",
				StartHz:    100,
				EndHz:      200,
				CenterHz:   150,
				SpanHz:     100,
				AutoRecord: true,
			},
			{
				Label:      "decode-zone",
				StartHz:    140,
				EndHz:      260,
				CenterHz:   200,
				SpanHz:     120,
				AutoDecode: true,
			},
		}),
	}
	decision := DecideSignalAction(policy, Candidate{ID: 5, CenterHz: 150}, nil)
	if !decision.ShouldRecord {
		t.Fatalf("expected record decision from overlapping window")
	}
	if !decision.ShouldAutoDecode {
		t.Fatalf("expected decode decision from overlapping window")
	}
	if decision.RecordWindow == nil || decision.RecordWindow.Label != "record-zone" {
		t.Fatalf("expected record window match to be record-zone")
	}
	if decision.DecodeWindow == nil || decision.DecodeWindow.Label != "decode-zone" {
		t.Fatalf("expected decode window match to be decode-zone")
	}
	if decision.MonitorDetail == nil || decision.MonitorDetail.Label != "record-zone" {
		t.Fatalf("expected record-zone to be preferred monitor detail, got %+v", decision.MonitorDetail)
	}
}
