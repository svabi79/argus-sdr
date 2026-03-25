package main

import (
	"fmt"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/sdr"
	"sdr-wideband-suite/internal/telemetry"
)

func (m *sourceManager) Restart(cfg config.Config) error {
	start := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.src
	_ = old.Stop()
	next, err := m.newSource(cfg)
	if err != nil {
		_ = old.Start()
		m.src = old
		if m.telemetry != nil {
			m.telemetry.IncCounter("source.restart.error", 1, nil)
			m.telemetry.Event("source_restart_failed", "warn", "source restart failed", nil, map[string]any{"error": err.Error()})
		}
		return err
	}
	if err := next.Start(); err != nil {
		_ = next.Stop()
		_ = old.Start()
		m.src = old
		if m.telemetry != nil {
			m.telemetry.IncCounter("source.restart.error", 1, nil)
			m.telemetry.Event("source_restart_failed", "warn", "source restart failed", nil, map[string]any{"error": err.Error()})
		}
		return err
	}
	m.src = next
	if m.telemetry != nil {
		m.telemetry.IncCounter("source.restart.count", 1, nil)
		m.telemetry.Observe("source.restart.duration_ms", float64(time.Since(start).Milliseconds()), nil)
	}
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
	return newSourceManagerWithTelemetry(src, newSource, nil)
}

func newSourceManagerWithTelemetry(src sdr.Source, newSource func(cfg config.Config) (sdr.Source, error), coll *telemetry.Collector) *sourceManager {
	return &sourceManager{src: src, newSource: newSource, telemetry: coll}
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
	waitStart := time.Now()
	m.mu.RLock()
	wait := time.Since(waitStart)
	defer m.mu.RUnlock()
	if m.telemetry != nil {
		m.telemetry.Observe("source.lock_wait_ms", float64(wait.Microseconds())/1000.0, telemetry.TagsFromPairs("lock", "read"))
		if wait > 2*time.Millisecond {
			m.telemetry.IncCounter("source.lock_contention.count", 1, telemetry.TagsFromPairs("lock", "read"))
		}
	}
	readStart := time.Now()
	out, err := m.src.ReadIQ(n)
	if m.telemetry != nil {
		tags := telemetry.TagsFromPairs("requested", fmt.Sprintf("%d", n))
		m.telemetry.Observe("source.read.duration_ms", float64(time.Since(readStart).Microseconds())/1000.0, tags)
		m.telemetry.SetGauge("source.read.samples", float64(len(out)), nil)
		if err != nil {
			m.telemetry.IncCounter("source.read.error", 1, nil)
		}
	}
	return out, err
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
