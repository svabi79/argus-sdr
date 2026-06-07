package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof on the default mux
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/iqfile"
	"sdr-wideband-suite/internal/fft/gpufft"
	"sdr-wideband-suite/internal/logging"
	"sdr-wideband-suite/internal/mock"
	"sdr-wideband-suite/internal/recorder"
	"sdr-wideband-suite/internal/runtime"
	"sdr-wideband-suite/internal/sdr"
	"sdr-wideband-suite/internal/sdrplay"
	"sdr-wideband-suite/internal/telemetry"
)

func main() {
	// Reduce GC target to limit peak memory. Default GOGC=100 lets heap
	// grow to 2× live set before collecting. GOGC=50 triggers GC at 1.5×,
	// halving the memory swings at a small CPU cost.
	debug.SetGCPercent(50)
	// Soft memory limit — GC will be more aggressive near this limit.
	// 1 GB is generous for 5 WFM-stereo signals + FFT + recordings.
	debug.SetMemoryLimit(1024 * 1024 * 1024)

	var cfgPath string
	var mockFlag bool
	var pprofAddr string
	var capturePath string
	var captureSeconds float64
	var captureCenterMHz float64
	var replayPath string
	flag.StringVar(&cfgPath, "config", "config.yaml", "path to config YAML")
	flag.BoolVar(&mockFlag, "mock", false, "use synthetic IQ source")
	flag.StringVar(&pprofAddr, "pprof", "localhost:6060", "pprof/debug HTTP address (empty to disable)")
	flag.StringVar(&capturePath, "capture", "", "capture IQ to this file and exit")
	flag.Float64Var(&captureSeconds, "capture-seconds", 20, "seconds of IQ to capture")
	flag.Float64Var(&captureCenterMHz, "capture-center", 0, "override center (MHz) for capture")
	flag.StringVar(&replayPath, "replay", "", "replay IQ from this file instead of a live source")
	flag.Parse()

	if pprofAddr != "" {
		go func() {
			log.Printf("pprof listening on http://%s/debug/pprof/", pprofAddr)
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				log.Printf("pprof server: %v", err)
			}
		}()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if replayPath != "" {
		_, meta, err := iqfile.Read(replayPath)
		if err != nil {
			log.Fatalf("replay: %v", err)
		}
		cfg.SampleRate = meta.SampleRate
		cfg.CenterHz = meta.CenterHz
		log.Printf("replay: %s (%.3f MHz, %.3f MS/s, %d samples)", replayPath, meta.CenterHz/1e6, float64(meta.SampleRate)/1e6, meta.Samples)
	}
	if err := logging.Init(logging.Config(cfg.Logging)); err != nil {
		log.Fatalf("logging init: %v", err)
	}
	defer logging.Close()

	cfgManager := runtime.New(cfg)
	gpuState := &gpuStatus{Available: gpufft.Available()}
	telemetryCfg := telemetry.Config{
		Enabled:           cfg.Debug.Telemetry.Enabled,
		HeavyEnabled:      cfg.Debug.Telemetry.HeavyEnabled,
		HeavySampleEvery:  cfg.Debug.Telemetry.HeavySampleEvery,
		MetricSampleEvery: cfg.Debug.Telemetry.MetricSampleEvery,
		MetricHistoryMax:  cfg.Debug.Telemetry.MetricHistoryMax,
		EventHistoryMax:   cfg.Debug.Telemetry.EventHistoryMax,
		Retention:         time.Duration(cfg.Debug.Telemetry.RetentionSeconds) * time.Second,
		PersistEnabled:    cfg.Debug.Telemetry.PersistEnabled,
		PersistDir:        cfg.Debug.Telemetry.PersistDir,
		RotateMB:          cfg.Debug.Telemetry.RotateMB,
		KeepFiles:         cfg.Debug.Telemetry.KeepFiles,
	}
	telemetryCollector, err := telemetry.New(telemetryCfg)
	if err != nil {
		log.Fatalf("telemetry init failed: %v", err)
	}
	defer telemetryCollector.Close()
	telemetryCollector.SetStatus("build", "sdrd")

	newSource := func(cfg config.Config) (sdr.Source, error) {
		if replayPath != "" {
			src, _, err := iqfile.NewSource(replayPath)
			return src, err
		}
		if mockFlag {
			src := mock.New(cfg.SampleRate)
			if updatable, ok := interface{}(src).(sdr.ConfigurableSource); ok {
				_ = updatable.UpdateConfig(cfg.SampleRate, cfg.CenterHz, cfg.GainDb, cfg.AGC, cfg.TunerBwKHz)
			}
			return src, nil
		}
		src, err := sdrplay.New(cfg.SampleRate, cfg.CenterHz, cfg.GainDb, cfg.TunerBwKHz)
		if err != nil {
			return nil, err
		}
		if updatable, ok := src.(sdr.ConfigurableSource); ok {
			_ = updatable.UpdateConfig(cfg.SampleRate, cfg.CenterHz, cfg.GainDb, cfg.AGC, cfg.TunerBwKHz)
		}
		return src, nil
	}

	if capturePath != "" {
		capCfg := cfg
		if captureCenterMHz > 0 {
			capCfg.CenterHz = captureCenterMHz * 1e6
		}
		runCapture(newSource, capCfg, capturePath, captureSeconds)
		return
	}

	src, err := newSource(cfg)
	if err != nil {
		log.Fatalf("sdrplay init failed: %v (try --mock or build with -tags sdrplay)", err)
	}
	srcMgr := newSourceManagerWithTelemetry(src, newSource, telemetryCollector)
	if err := srcMgr.Start(); err != nil {
		log.Fatalf("source start: %v", err)
	}
	defer srcMgr.Stop()

	if err := os.MkdirAll(filepath.Dir(cfg.EventPath), 0o755); err != nil {
		log.Fatalf("event path: %v", err)
	}

	eventFile, err := os.OpenFile(cfg.EventPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("open events: %v", err)
	}
	defer eventFile.Close()
	eventMu := &sync.RWMutex{}

	det := detector.New(cfg.Detector, cfg.SampleRate, cfg.FFTSize)

	window := fftutil.Hann(cfg.FFTSize)
	h := newHub()
	dspUpdates := make(chan dspUpdate, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	decodeMap := buildDecoderMap(cfg)
	recMgr := recorder.New(cfg.SampleRate, cfg.FFTSize, recorder.Policy{
		Enabled:          cfg.Recorder.Enabled,
		MinSNRDb:         cfg.Recorder.MinSNRDb,
		MinDuration:      mustParseDuration(cfg.Recorder.MinDuration, 1*time.Second),
		MaxDuration:      mustParseDuration(cfg.Recorder.MaxDuration, 300*time.Second),
		PrerollMs:        cfg.Recorder.PrerollMs,
		RecordIQ:         cfg.Recorder.RecordIQ,
		RecordAudio:      cfg.Recorder.RecordAudio,
		AutoDemod:        cfg.Recorder.AutoDemod,
		AutoDecode:       cfg.Recorder.AutoDecode,
		MaxDiskMB:        cfg.Recorder.MaxDiskMB,
		OutputDir:        cfg.Recorder.OutputDir,
		ClassFilter:      cfg.Recorder.ClassFilter,
		RingSeconds:      cfg.Recorder.RingSeconds,
		DeemphasisUs:     cfg.Recorder.DeemphasisUs,
		ExtractionTaps:   cfg.Recorder.ExtractionTaps,
		ExtractionBwMult: cfg.Recorder.ExtractionBwMult,
	}, cfg.CenterHz, decodeMap, telemetryCollector)
	defer recMgr.Close()

	sigSnap := &signalSnapshot{}
	extractMgr := &extractionManager{}
	defer extractMgr.reset()

	phaseSnap := &phaseSnapshot{}
	go runDSP(ctx, srcMgr, cfg, det, window, h, eventFile, eventMu, dspUpdates, gpuState, recMgr, sigSnap, extractMgr, phaseSnap, telemetryCollector)

	server := newHTTPServer(cfg.WebAddr, cfg.WebRoot, h, cfgPath, cfgManager, srcMgr, dspUpdates, gpuState, recMgr, sigSnap, eventMu, phaseSnap, telemetryCollector)
	go func() {
		log.Printf("web listening on %s", cfg.WebAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	shutdownServer(server)
}

// runCapture reads `seconds` of IQ from a fresh source and writes it to path.
func runCapture(newSource func(config.Config) (sdr.Source, error), cfg config.Config, path string, seconds float64) {
	src, err := newSource(cfg)
	if err != nil {
		log.Fatalf("capture: source init: %v", err)
	}
	if err := src.Start(); err != nil {
		log.Fatalf("capture: start: %v", err)
	}
	defer src.Stop()
	chunk := cfg.SampleRate / 10 // ~100 ms
	if chunk < 1 {
		chunk = 65536
	}
	// Discard ~0.5 s so the tuner/AGC settles before we keep samples.
	for warm := 0; warm < cfg.SampleRate/2; {
		buf, err := src.ReadIQ(chunk)
		if err != nil {
			break
		}
		warm += len(buf)
	}
	total := int(float64(cfg.SampleRate) * seconds)
	iq := make([]complex64, 0, total)
	log.Printf("capture: %.1fs @ %.3f MHz / %.3f MS/s -> %s", seconds, cfg.CenterHz/1e6, float64(cfg.SampleRate)/1e6, path)
	for len(iq) < total {
		n := chunk
		if len(iq)+n > total {
			n = total - len(iq)
		}
		buf, err := src.ReadIQ(n)
		if err != nil {
			log.Printf("capture: read: %v", err)
			break
		}
		if len(buf) == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		iq = append(iq, buf...)
	}
	if err := iqfile.Write(path, iq, cfg.SampleRate, cfg.CenterHz); err != nil {
		log.Fatalf("capture: write: %v", err)
	}
	log.Printf("capture: wrote %d samples (%.1f MB)", len(iq), float64(len(iq))*8/1e6)
}
