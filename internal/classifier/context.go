package classifier

import (
	_ "embed"
	"encoding/json"
	"math"
)

//go:embed frequency_context.json
var frequencyContextJSON []byte

type frequencyRange struct {
	Name     string  `json:"name"`
	StartMHz float64 `json:"start_mhz"`
	EndMHz   float64 `json:"end_mhz"`
}

type frequencyContextConfig struct {
	FT8MHz  []float64        `json:"ft8_mhz"`
	WSPRMHz []float64        `json:"wspr_mhz"`
	Ranges  []frequencyRange `json:"ranges"`
}

var frequencyContext = loadFrequencyContext()

func loadFrequencyContext() frequencyContextConfig {
	var cfg frequencyContextConfig
	if err := json.Unmarshal(frequencyContextJSON, &cfg); err != nil {
		return frequencyContextConfig{}
	}
	return cfg
}

func addFrequencyContext(add func(SignalClass, float64), centerHz float64, bw float64) {
	mhz := centerHz / 1e6
	for _, r := range frequencyContext.Ranges {
		if mhz < r.StartMHz || mhz > r.EndMHz {
			continue
		}
		switch r.Name {
		case "hf":
			for _, f := range frequencyContext.FT8MHz {
				if math.Abs(mhz-f) < 0.003 && bw >= 1500 && bw <= 3500 {
					add(ClassFT8, 2.0)
					break
				}
			}
			for _, f := range frequencyContext.WSPRMHz {
				if math.Abs(mhz-f) < 0.001 && bw >= 100 && bw <= 500 {
					add(ClassWSPR, 2.0)
					break
				}
			}
			if bw < 500 {
				add(ClassCW, 0.5)
			}
			if bw >= 2000 && bw <= 4000 {
				if mhz < 10 {
					add(ClassSSBLSB, 0.8)
				} else {
					add(ClassSSBUSB, 0.8)
				}
			}
		case "vhf_2m":
			if bw >= 6000 && bw <= 16000 {
				add(ClassNFM, 0.5)
			}
			if bw >= 2000 && bw <= 4000 {
				add(ClassSSBUSB, 0.5)
			}
		case "uhf_70cm":
			if bw >= 6000 && bw <= 16000 {
				add(ClassNFM, 0.3)
				add(ClassDMR, 0.5)
				add(ClassDStar, 0.3)
			}
		case "pmr446":
			add(ClassNFM, 1.0)
		case "broadcast_fm":
			if bw >= 50000 {
				add(ClassWFM, 1.5)
			}
		case "airband":
			if bw >= 5000 && bw <= 10000 {
				add(ClassAM, 1.5)
			}
		}
	}
}
