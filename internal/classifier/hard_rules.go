package classifier

import (
	_ "embed"
	"encoding/json"
	"log"
)

//go:embed hard_rules.json
var hardRulesJSON []byte

type hardRule struct {
	Name   string     `json:"name"`
	Match  hardMatch  `json:"match"`
	Result hardResult `json:"result"`
	Note   string     `json:"note"`
}

type hardMatch struct {
	MinMHz  float64 `json:"min_mhz,omitempty"`
	MaxMHz  float64 `json:"max_mhz,omitempty"`
	MinBWHz float64 `json:"min_bw_hz,omitempty"`
	MaxBWHz float64 `json:"max_bw_hz,omitempty"`
}

type hardResult struct {
	ModType    string  `json:"mod_type"`
	Confidence float64 `json:"confidence"`
}

type hardRulesFile struct {
	Rules []hardRule `json:"rules"`
}

var loadedHardRules []hardRule

func init() {
	var f hardRulesFile
	if err := json.Unmarshal(hardRulesJSON, &f); err != nil {
		log.Printf("classifier: failed to load hard rules: %v", err)
		return
	}
	loadedHardRules = f.Rules
}

func TryHardRule(centerHz float64, bwHz float64) *Classification {
	mhz := centerHz / 1e6
	for _, r := range loadedHardRules {
		if r.Match.MinMHz > 0 && mhz < r.Match.MinMHz {
			continue
		}
		if r.Match.MaxMHz > 0 && mhz > r.Match.MaxMHz {
			continue
		}
		if r.Match.MinBWHz > 0 && bwHz < r.Match.MinBWHz {
			continue
		}
		if r.Match.MaxBWHz > 0 && bwHz > r.Match.MaxBWHz {
			continue
		}
		mod := SignalClass(r.Result.ModType)
		return &Classification{
			ModType:    mod,
			Confidence: r.Result.Confidence,
			BW3dB:      bwHz,
			Scores:     map[SignalClass]float64{mod: 10.0},
		}
	}
	return nil
}
