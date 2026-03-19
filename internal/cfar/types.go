package cfar

// Mode selects the CFAR algorithm variant.
type Mode string

const (
	ModeOff   Mode = "OFF"
	ModeCA    Mode = "CA"
	ModeOS    Mode = "OS"
	ModeGOSCA Mode = "GOSCA"
	ModeCASO  Mode = "CASO"
)

// Config holds all CFAR parameters.
type Config struct {
	Mode        Mode
	GuardCells  int
	TrainCells  int
	Rank        int
	ScaleDb     float64
	WrapAround  bool
}

// CFAR computes adaptive thresholds for a spectrum.
type CFAR interface {
	// Thresholds returns per-bin detection thresholds in dB.
	// spectrum is power in dB, length n.
	// Returned slice has length n. No NaN values.
	Thresholds(spectrum []float64) []float64
}
