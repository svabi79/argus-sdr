package classifier

// SignalClass is a coarse modulation label.
type SignalClass string

const (
	ClassAM      SignalClass = "AM"
	ClassNFM     SignalClass = "NFM"
	ClassWFM     SignalClass = "WFM"
	ClassSSBUSB  SignalClass = "USB"
	ClassSSBLSB  SignalClass = "LSB"
	ClassCW      SignalClass = "CW"
	ClassNoise   SignalClass = "NOISE"
	ClassUnknown SignalClass = "UNKNOWN"
)

// Features are lightweight spectral features derived from a signal slice.
type Features struct {
	// Spectral
	BW3dB        float64 `json:"bw_3db_hz"`
	BW90         float64 `json:"bw_90_hz"`
	SpectralFlat float64 `json:"spectral_flat"`
	PeakToAvg    float64 `json:"peak_to_avg_db"`
	Symmetry     float64 `json:"symmetry"`
	RolloffLeft  float64 `json:"rolloff_left_db_khz"`
	RolloffRight float64 `json:"rolloff_right_db_khz"`
}

// Classification is the classifier output attached to signals/events.
type Classification struct {
	ModType    SignalClass `json:"mod_type"`
	Confidence float64     `json:"confidence"`
	BW3dB      float64     `json:"bw_3db_hz"`
	Features   Features    `json:"features,omitempty"`
	SecondBest SignalClass `json:"second_best,omitempty"`
}

// SignalInput is the minimal input needed for classification.
type SignalInput struct {
	FirstBin int
	LastBin  int
	SNRDb    float64
}
