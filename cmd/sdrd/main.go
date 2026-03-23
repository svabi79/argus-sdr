package main

import (
	"context"
	"flag"
	"log"
	"net/http"
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
	"sdr-wideband-suite/internal/fft/gpufft"
	"sdr-wideband-suite/internal/logging"
	"sdr-wideband-suite/internal/mock"
	"sdr-wideband-suite/internal/recorder"
	"sdr-wideband-suite/internal/runtime"
	"sdr-wideband-suite/internal/sdr"
	"sdr-wideband-suite/internal/sdrplay"
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
	flag.StringVar(&cfgPath, "config", "config.yaml", "path to config YAML")
	flag.BoolVar(&mockFlag, "mock", false, "use synthetic IQ source")
	flag.Parse()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := logging.Init(logging.Config(cfg.Logging)); err != nil {
		log.Fatalf("logging init: %v", err)
	}
	defer logging.Close()

	cfgManager := runtime.New(cfg)
	gpuState := &gpuStatus{Available: gpufft.Available()}

	newSource := func(cfg config.Config) (sdr.Source, error) {
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

	src, err := newSource(cfg)
	if err != nil {
		log.Fatalf("sdrplay init failed: %v (try --mock or build with -tags sdrplay)", err)
	}
	srcMgr := newSourceManager(src, newSource)
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
	}, cfg.CenterHz, decodeMap)
	defer recMgr.Close()

	sigSnap := &signalSnapshot{}
	extractMgr := &extractionManager{}
	defer extractMgr.reset()

	phaseSnap := &phaseSnapshot{}
	go runDSP(ctx, srcMgr, cfg, det, window, h, eventFile, eventMu, dspUpdates, gpuState, recMgr, sigSnap, extractMgr, phaseSnap)

	server := newHTTPServer(cfg.WebAddr, cfg.WebRoot, h, cfgPath, cfgManager, srcMgr, dspUpdates, gpuState, recMgr, sigSnap, eventMu, phaseSnap)
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
