package main

import (
	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/sdr"
)

func (m *sourceManager) Restart(cfg config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.src
	_ = old.Stop()
	next, err := m.newSource(cfg)
	if err != nil {
		_ = old.Start()
		m.src = old
		return err
	}
	if err := next.Start(); err != nil {
		_ = next.Stop()
		_ = old.Start()
		m.src = old
		return err
	}
	m.src = next
	return nil
}

func (m *sourceManager) Stats() sdr.SourceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sp, ok := m.src.(sdr.StatsProvider); ok {
		return sp.Stats()
	}
	return sdr.SourceStats{}
}

func (m *sourceManager) Flush() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if fl, ok := m.src.(sdr.Flushable); ok {
		fl.Flush()
	}
}

func newSourceManager(src sdr.Source, newSource func(cfg config.Config) (sdr.Source, error)) *sourceManager {
	return &sourceManager{src: src, newSource: newSource}
}

func (m *sourceManager) Start() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.src.Start()
}

func (m *sourceManager) Stop() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.src.Stop()
}

func (m *sourceManager) ReadIQ(n int) ([]complex64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.src.ReadIQ(n)
}

func (m *sourceManager) ApplyConfig(cfg config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if updatable, ok := m.src.(sdr.ConfigurableSource); ok {
		if err := updatable.UpdateConfig(cfg.SampleRate, cfg.CenterHz, cfg.GainDb, cfg.AGC, cfg.TunerBwKHz); err == nil {
			return nil
		}
	}

	old := m.src
	_ = old.Stop()
	next, err := m.newSource(cfg)
	if err != nil {
		_ = old.Start()
		return err
	}
	if err := next.Start(); err != nil {
		_ = next.Stop()
		_ = old.Start()
		return err
	}
	m.src = next
	return nil
}

func pushDSPUpdate(ch chan dspUpdate, update dspUpdate) {
	select {
	case ch <- update:
	default:
		select {
		case <-ch:
		default:
		}
		ch <- update
	}
}
