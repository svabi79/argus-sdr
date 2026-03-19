package cfar

// New creates a CFAR detector for the given mode.
// Returns nil if mode is ModeOff or empty.
func New(cfg Config) CFAR {
	if cfg.TrainCells <= 0 {
		return nil
	}
	if cfg.GuardCells < 0 {
		cfg.GuardCells = 0
	}
	if cfg.ScaleDb <= 0 {
		cfg.ScaleDb = 6
	}
	switch cfg.Mode {
	case ModeCA:
		return newCA(cfg)
	case ModeOS:
		return newOS(cfg)
	case ModeGOSCA:
		return newGOSCA(cfg)
	case ModeCASO:
		return newCASO(cfg)
	default:
		return nil
	}
}
