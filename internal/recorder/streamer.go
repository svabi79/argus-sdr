package recorder

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/demod"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/dsp"
	"sdr-wideband-suite/internal/logging"
	"sdr-wideband-suite/internal/telemetry"
)

// ---------------------------------------------------------------------------
// streamSession — one open demod session for one signal
// ---------------------------------------------------------------------------

type streamSession struct {
	sessionID    string
	signalID     int64
	centerHz     float64
	bwHz         float64
	snrDb        float64
	peakDb       float64
	class        *classifier.Classification
	startTime    time.Time
	lastFeed     time.Time
	playbackMode string
	stereoState  string
	lastAudioTs  time.Time

	debugDumpStart time.Time
	debugDumpUntil time.Time
	debugDumpBase  string

	demodDump    []float32
	finalDump    []float32
	lastAudioL   float32
	lastAudioR   float32
	prevAudioL   float64 // second-to-last L sample for boundary transient detection
	lastAudioSet bool

	lastDecIQ    complex64
	prevDecIQ    complex64
	lastDecIQSet bool

	lastExtractIQ    complex64
	prevExtractIQ    complex64
	lastExtractIQSet bool

	// FM discriminator cross-block bridging: carry the last IQ sample so the
	// discriminator can compute the phase step across block boundaries.
	lastDiscrimIQ    complex64
	lastDiscrimIQSet bool

	lastDemodL   float32
	prevDemodL   float64
	lastDemodSet bool
	snippetSeq   uint64

	// listenOnly sessions have no WAV file and no disk I/O.
	// They exist solely to feed audio to live-listen subscribers.
	listenOnly bool

	// Recording state (nil/zero for listen-only sessions)
	dir        string
	wavFile    *os.File
	wavBuf     *bufio.Writer
	wavSamples int64
	segmentIdx int

	sampleRate int // actual output audio sample rate (always streamAudioRate)
	channels   int
	demodName  string

	// --- Persistent DSP state for click-free streaming ---

	// Overlap-save: tail of previous extracted IQ snippet.
	// Currently unused for live demod after removing the extra discriminator
	// overlap prepend, but kept in DSP snapshot state for compatibility.
	overlapIQ []complex64

	// De-emphasis IIR state (persists across frames)
	deemphL float64
	deemphR float64

	// Stereo lock state for live WFM streaming
	stereoEnabled  bool
	stereoOnCount  int
	stereoOffCount int
	// Pilot-locked stereo PLL state (19kHz pilot)
	pilotPhase   float64
	pilotFreq    float64
	pilotAlpha   float64
	pilotBeta    float64
	pilotErrAvg  float64
	pilotI       float64
	pilotQ       float64
	pilotLPAlpha float64

	// Polyphase resampler (replaces integer-decimate hack)
	monoResampler       *dsp.Resampler
	monoResamplerRate   int
	stereoResampler     *dsp.StereoResampler
	stereoResamplerRate int

	// AQ-4: Stateful FIR filters for click-free stereo decode
	stereoFilterRate int
	stereoLPF        *dsp.StatefulFIRReal // 15kHz lowpass for L+R
	stereoBPHi       *dsp.StatefulFIRReal // 53kHz LP for bandpass high
	stereoBPLo       *dsp.StatefulFIRReal // 23kHz LP for bandpass low
	stereoLRLPF      *dsp.StatefulFIRReal // 15kHz LP for demodulated L-R
	stereoAALPF      *dsp.StatefulFIRReal // Anti-alias LP for pre-decim (mono path)
	pilotLPFHi       *dsp.StatefulFIRReal // ~21kHz LP for pilot bandpass high
	pilotLPFLo       *dsp.StatefulFIRReal // ~17kHz LP for pilot bandpass low

	// WFM 15kHz audio LPF — removes pilot (19kHz), L-R subcarrier (23-53kHz),
	// and RDS (57kHz) from the FM discriminator output before resampling.
	// Without this, the pilot leaks into the audio as a 19kHz tone (+55dB above
	// noise floor) and L-R subcarrier energy causes audible click-like artifacts.
	wfmAudioLPF     *dsp.StatefulFIRReal
	wfmAudioLPFRate int

	// Stateful pre-demod anti-alias FIR (eliminates cold-start transients
	// and avoids per-frame FIR recomputation)
	preDemodFIR        *dsp.StatefulFIRComplex
	preDemodDecimator  *dsp.StatefulDecimatingFIRComplex
	preDemodDecim      int     // cached decimation factor
	preDemodRate       int     // cached snipRate this FIR was built for
	preDemodCutoff     float64 // cached cutoff
	preDemodDecimPhase int     // retained for backward compatibility in snapshots/debug

	// AQ-2: De-emphasis config (µs, 0 = disabled)
	deemphasisUs float64

	// Scratch buffers — reused across frames to avoid GC pressure.
	// Grown as needed, never shrunk.
	scratchIQ    []complex64 // for pre-demod FIR output + decimate input
	scratchAudio []float32   // for stereo decode intermediates
	scratchPCM   []byte      // for PCM encoding

	// live-listen subscribers
	audioSubs []audioSub
}

type audioSub struct {
	id int64
	ch chan []byte
}

type RuntimeSignalInfo struct {
	DemodName    string
	PlaybackMode string
	StereoState  string
	Channels     int
	SampleRate   int
}

// AudioInfo describes the audio format of a live-listen subscription.
// Sent to the WebSocket client as the first message.
type AudioInfo struct {
	SampleRate   int    `json:"sample_rate"`
	Channels     int    `json:"channels"`
	Format       string `json:"format"` // always "s16le"
	DemodName    string `json:"demod"`
	PlaybackMode string `json:"playback_mode,omitempty"`
	StereoState  string `json:"stereo_state,omitempty"`
}

const (
	streamAudioRate = 48000
	resamplerTaps   = 32 // taps per polyphase arm — good quality
)

var debugDumpDelay = func() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SDR_DEBUG_DUMP_DELAY_SECONDS"))
	if raw == "" {
		return 5 * time.Second
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 5 * time.Second
	}
	return time.Duration(v) * time.Second
}()

var debugDumpDuration = func() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SDR_DEBUG_DUMP_DURATION_SECONDS"))
	if raw == "" {
		return 15 * time.Second
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 15 * time.Second
	}
	return time.Duration(v) * time.Second
}()

var audioDumpEnabled = func() bool {
	raw := strings.TrimSpace(os.Getenv("SDR_DEBUG_AUDIO_DUMP_ENABLED"))
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return v
}()

var decHeadTrimSamples = func() int {
	raw := strings.TrimSpace(os.Getenv("SDR_DEC_HEAD_TRIM"))
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0
	}
	return v
}()

// ---------------------------------------------------------------------------
// Streamer — manages all active streaming sessions
// ---------------------------------------------------------------------------

type streamFeedItem struct {
	signal   detector.Signal
	snippet  []complex64
	snipRate int
}

type streamFeedMsg struct {
	traceID    uint64
	items      []streamFeedItem
	enqueuedAt time.Time
}

type Streamer struct {
	mu       sync.Mutex
	sessions map[int64]*streamSession
	policy   Policy
	centerHz float64
	nextSub  int64
	feedCh   chan streamFeedMsg
	done     chan struct{}

	droppedFeed uint64
	droppedPCM  uint64

	lastFeedTS time.Time
	lastProcTS time.Time

	// pendingListens are subscribers waiting for a matching session.
	pendingListens map[int64]*pendingListen
	telemetry      *telemetry.Collector

	debugSummary *audioStutterDebugLogger
	summaryStop  chan struct{}
	summaryWG    sync.WaitGroup

	// Stream summary counters (cheap to maintain, sampled every ~5s)
	producedPCMFrames uint64
	processLoopCount  uint64
	processLoopSumMs  float64
	processLoopMaxMs  float64
}

type pendingListen struct {
	freq float64
	bw   float64
	mode string
	ch   chan []byte
}

func newStreamer(policy Policy, centerHz float64, coll *telemetry.Collector) *Streamer {
	st := &Streamer{
		sessions:       make(map[int64]*streamSession),
		policy:         policy,
		centerHz:       centerHz,
		feedCh:         make(chan streamFeedMsg, 2),
		done:           make(chan struct{}),
		pendingListens: make(map[int64]*pendingListen),
		telemetry:      coll,
		debugSummary:   newAudioStutterDebugLogger(),
		summaryStop:    make(chan struct{}),
	}
	go st.worker()
	st.summaryWG.Add(1)
	go st.summaryWorker()
	return st
}

func (st *Streamer) worker() {
	for msg := range st.feedCh {
		st.processFeed(msg)
	}
	close(st.done)
}

func (st *Streamer) updatePolicy(policy Policy, centerHz float64) {
	st.mu.Lock()
	defer st.mu.Unlock()
	wasEnabled := st.policy.Enabled
	st.policy = policy
	st.centerHz = centerHz

	// If recording was just disabled, close recording sessions
	// but keep listen-only sessions alive.
	if wasEnabled && !policy.Enabled {
		for id, sess := range st.sessions {
			if sess.listenOnly {
				continue
			}
			if len(sess.audioSubs) > 0 {
				// Convert to listen-only: close WAV but keep session
				convertToListenOnly(sess)
			} else {
				closeSession(sess, &st.policy)
				delete(st.sessions, id)
			}
		}
	}
}

// HasListeners returns true if any sessions have audio subscribers
// or there are pending listen requests. Used by the DSP loop to
// decide whether to feed snippets even when recording is disabled.
func (st *Streamer) HasListeners() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.hasListenersLocked()
}

func (st *Streamer) hasListenersLocked() bool {
	if len(st.pendingListens) > 0 {
		return true
	}
	for _, sess := range st.sessions {
		if len(sess.audioSubs) > 0 {
			return true
		}
	}
	return false
}

// FeedSnippets is called from the DSP loop with pre-extracted IQ snippets.
// Feeds are accepted if:
//   - Recording is enabled (policy.Enabled && RecordAudio/RecordIQ), OR
//   - Any live-listen subscribers exist (listen-only mode)
//
// IMPORTANT: The caller (Manager.FeedSnippets) already copies the snippet
// data, so items can be passed directly without another copy.
func (st *Streamer) FeedSnippets(items []streamFeedItem, traceID uint64) {
	st.mu.Lock()
	recEnabled := st.policy.Enabled && (st.policy.RecordAudio || st.policy.RecordIQ)
	hasListeners := st.hasListenersLocked()
	pending := len(st.pendingListens)
	debugLiveAudio := st.policy.DebugLiveAudio
	now := time.Now()
	if !st.lastFeedTS.IsZero() {
		gap := now.Sub(st.lastFeedTS)
		if gap > 150*time.Millisecond {
			logging.Warn("gap", "feed_gap", "gap_ms", gap.Milliseconds())
		}
	}
	st.lastFeedTS = now
	st.mu.Unlock()

	if debugLiveAudio {
		log.Printf("LIVEAUDIO STREAM: feedSnippets items=%d recEnabled=%v hasListeners=%v pending=%d", len(items), recEnabled, hasListeners, pending)
	}
	if (!recEnabled && !hasListeners) || len(items) == 0 {
		return
	}
	if st.telemetry != nil {
		st.telemetry.SetGauge("streamer.feed.queue_len", float64(len(st.feedCh)), nil)
		st.telemetry.SetGauge("streamer.pending_listeners", float64(pending), nil)
		st.telemetry.Observe("streamer.feed.batch_size", float64(len(items)), nil)
	}

	select {
	case st.feedCh <- streamFeedMsg{traceID: traceID, items: items, enqueuedAt: time.Now()}:
	default:
		st.droppedFeed++
		logging.Warn("drop", "feed_drop", "count", st.droppedFeed)
		if st.telemetry != nil {
			st.telemetry.IncCounter("streamer.feed.drop", 1, nil)
			st.telemetry.Event("stream_feed_drop", "warn", "feed queue full", nil, map[string]any{
				"trace_id":  traceID,
				"queue_len": len(st.feedCh),
			})
		}
	}
}

// processFeed runs in the worker goroutine.
func (st *Streamer) processFeed(msg streamFeedMsg) {
	procStart := time.Now()
	lockStart := time.Now()
	st.mu.Lock()
	lockWait := time.Since(lockStart)
	recEnabled := st.policy.Enabled && (st.policy.RecordAudio || st.policy.RecordIQ)
	hasListeners := st.hasListenersLocked()
	now := time.Now()
	if !st.lastProcTS.IsZero() {
		gap := now.Sub(st.lastProcTS)
		if gap > 150*time.Millisecond {
			logging.Warn("gap", "process_gap", "gap_ms", gap.Milliseconds(), "trace", msg.traceID)
			if st.telemetry != nil {
				st.telemetry.IncCounter("streamer.process.gap.count", 1, nil)
				st.telemetry.Observe("streamer.process.gap_ms", float64(gap.Milliseconds()), nil)
			}
		}
	}
	st.lastProcTS = now
	defer st.mu.Unlock()
	defer func() {
		procMs := float64(time.Since(procStart).Microseconds()) / 1000.0
		st.processLoopCount++
		st.processLoopSumMs += procMs
		if procMs > st.processLoopMaxMs {
			st.processLoopMaxMs = procMs
		}
		if st.telemetry != nil {
			st.telemetry.Observe("streamer.process.total_ms", procMs, nil)
			st.telemetry.Observe("streamer.lock_wait_ms", float64(lockWait.Microseconds())/1000.0, telemetry.TagsFromPairs("lock", "process"))
		}
	}()
	if st.telemetry != nil {
		st.telemetry.Observe("streamer.feed.enqueue_delay_ms", float64(now.Sub(msg.enqueuedAt).Microseconds())/1000.0, nil)
		st.telemetry.SetGauge("streamer.sessions.active", float64(len(st.sessions)), nil)
	}

	logging.Debug("trace", "process_feed", "trace", msg.traceID, "items", len(msg.items))

	if !recEnabled && !hasListeners {
		return
	}

	seen := make(map[int64]bool, len(msg.items))

	for i := range msg.items {
		item := &msg.items[i]
		sig := &item.signal
		seen[sig.ID] = true

		if sig.ID == 0 || sig.Class == nil {
			continue
		}
		if len(item.snippet) == 0 || item.snipRate <= 0 {
			continue
		}

		// Decide whether this signal needs a session
		needsRecording := recEnabled && sig.SNRDb >= st.policy.MinSNRDb && st.classAllowed(sig.Class)
		needsListen := st.signalHasListenerLocked(sig)
		className := "<nil>"
		demodName := ""
		if sig.Class != nil {
			className = string(sig.Class.ModType)
			demodName, _ = resolveDemod(sig)
		}
		if st.policy.DebugLiveAudio {
			log.Printf("LIVEAUDIO STREAM: signal id=%d center=%.3fMHz bw=%.0f snr=%.1f class=%s demod=%s needsRecord=%v needsListen=%v", sig.ID, sig.CenterHz/1e6, sig.BWHz, sig.SNRDb, className, demodName, needsRecording, needsListen)
		}

		if !needsRecording && !needsListen {
			continue
		}

		sess, exists := st.sessions[sig.ID]
		requestedMode := ""
		for _, pl := range st.pendingListens {
			if math.Abs(sig.CenterHz-pl.freq) < 200000 {
				if m := normalizeRequestedMode(pl.mode); m != "" {
					requestedMode = m
					break
				}
			}
		}
		if exists && sess.listenOnly && requestedMode != "" && sess.demodName != requestedMode {
			for _, sub := range sess.audioSubs {
				st.pendingListens[sub.id] = &pendingListen{freq: sig.CenterHz, bw: sig.BWHz, mode: requestedMode, ch: sub.ch}
			}
			delete(st.sessions, sig.ID)
			sess = nil
			exists = false
		}
		if !exists {
			if needsRecording {
				s, err := st.openRecordingSession(sig, now)
				if err != nil {
					log.Printf("STREAM: open failed signal=%d %.1fMHz: %v",
						sig.ID, sig.CenterHz/1e6, err)
					if st.telemetry != nil {
						st.telemetry.IncCounter("streamer.session.open_error", 1, telemetry.TagsFromPairs("kind", "recording"))
					}
					continue
				}
				st.sessions[sig.ID] = s
				sess = s
			} else {
				s := st.openListenSession(sig, now)
				st.sessions[sig.ID] = s
				sess = s
			}
			// Attach any pending listeners
			st.attachPendingListeners(sess)
			if st.telemetry != nil {
				st.telemetry.IncCounter("streamer.session.open", 1, telemetry.TagsFromPairs("session_id", sess.sessionID, "signal_id", fmt.Sprintf("%d", sig.ID)))
				st.telemetry.Event("session_open", "info", "stream session opened", telemetry.TagsFromPairs("session_id", sess.sessionID, "signal_id", fmt.Sprintf("%d", sig.ID)), map[string]any{
					"listen_only": sess.listenOnly,
					"demod":       sess.demodName,
				})
			}
		}

		// Update metadata
		sess.lastFeed = now
		sess.centerHz = sig.CenterHz
		sess.bwHz = sig.BWHz
		if sig.SNRDb > sess.snrDb {
			sess.snrDb = sig.SNRDb
		}
		if sig.PeakDb > sess.peakDb {
			sess.peakDb = sig.PeakDb
		}
		if sig.Class != nil {
			sess.class = sig.Class
		}

		// Demod with persistent state
		logging.Debug("trace", "demod_start", "trace", msg.traceID, "signal", sess.signalID, "snip_len", len(item.snippet), "snip_rate", item.snipRate)
		audioStart := time.Now()
		audio, audioRate := sess.processSnippet(item.snippet, item.snipRate, st.telemetry)
		if st.telemetry != nil {
			st.telemetry.Observe("streamer.process_snippet_ms", float64(time.Since(audioStart).Microseconds())/1000.0, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
		}
		logging.Debug("trace", "demod_done", "trace", msg.traceID, "signal", sess.signalID, "audio_len", len(audio), "audio_rate", audioRate)
		if len(audio) == 0 {
			logging.Warn("gap", "audio_empty", "signal", sess.signalID, "snip_len", len(item.snippet), "snip_rate", item.snipRate)
			if st.telemetry != nil {
				st.telemetry.IncCounter("streamer.audio.empty", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID)))
			}
		}
		if len(audio) > 0 {
			ch := sess.channels
			if ch <= 0 {
				ch = 1
			}
			st.producedPCMFrames += uint64(len(audio) / ch)
			if sess.wavSamples == 0 && audioRate > 0 {
				sess.sampleRate = audioRate
			}
			// Encode PCM once into scratch buffer, reuse for both WAV and fanout
			pcmLen := len(audio) * 2
			pcm := sess.growPCM(pcmLen)
			for k, s := range audio {
				v := int16(clip(s * 32767))
				binary.LittleEndian.PutUint16(pcm[k*2:], uint16(v))
			}
			if !sess.listenOnly && sess.wavBuf != nil {
				n, err := sess.wavBuf.Write(pcm)
				if err != nil {
					log.Printf("STREAM: write error signal=%d: %v", sess.signalID, err)
				} else {
					sess.wavSamples += int64(n / 2)
				}
			}
			// Gap logging for live-audio sessions + transient click detector
			if len(sess.audioSubs) > 0 {
				if !sess.lastAudioTs.IsZero() {
					gap := time.Since(sess.lastAudioTs)
					if gap > 150*time.Millisecond {
						logging.Warn("gap", "audio_gap", "signal", sess.signalID, "gap_ms", gap.Milliseconds())
						if st.telemetry != nil {
							st.telemetry.IncCounter("streamer.audio.gap.count", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID)))
							st.telemetry.Observe("streamer.audio.gap_ms", float64(gap.Milliseconds()), telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID)))
						}
					}
				}
				// Transient click detector: finds short impulses (1-3 samples)
				// that deviate sharply from the local signal trend.
				// A click looks like: ...smooth... SPIKE ...smooth...
				// Normal FM audio has large deltas too, but they follow
				// a continuous curve. A click has high |d2/dt2| (acceleration).
				//
				// Method: second-derivative detector. For each sample triplet
				// (a, b, c), compute |2b - a - c| which is the discrete
				// second derivative magnitude. High values = transient spike.
				// Threshold: 0.15 (tuned to reject normal FM content <15kHz).
				if logging.EnabledCategory("boundary") && len(audio) > 0 {
					stride := sess.channels
					if stride < 1 {
						stride = 1
					}
					nFrames := len(audio) / stride

					// Boundary transient: use last 2 samples of prev frame + first sample of this frame
					if sess.lastAudioSet && nFrames >= 1 {
						// second derivative across boundary: |2*last - prevLast - first|
						first := float64(audio[0])
						d2 := math.Abs(2*float64(sess.lastAudioL) - sess.prevAudioL - first)
						if d2 > 0.15 {
							logging.Warn("boundary", "boundary_click", "signal", sess.signalID, "d2", d2)
							if st.telemetry != nil {
								st.telemetry.IncCounter("audio.boundary_click.count", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
								st.telemetry.Observe("audio.boundary_click.d2", d2, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID)))
							}
						}
					}

					// Intra-frame transient scan (L channel only for performance)
					nClicks := 0
					maxD2 := float64(0)
					maxD2Pos := 0
					for k := 1; k < nFrames-1; k++ {
						a := float64(audio[(k-1)*stride])
						b := float64(audio[k*stride])
						c := float64(audio[(k+1)*stride])
						d2 := math.Abs(2*b - a - c)
						if d2 > maxD2 {
							maxD2 = d2
							maxD2Pos = k
						}
						if d2 > 0.15 {
							nClicks++
						}
					}
					if nClicks > 0 {
						logging.Warn("boundary", "intra_click", "signal", sess.signalID, "clicks", nClicks, "maxD2", maxD2, "pos", maxD2Pos, "len", nFrames)
						if st.telemetry != nil {
							st.telemetry.IncCounter("audio.intra_click.count", float64(nClicks), telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
							st.telemetry.Observe("audio.intra_click.max_d2", maxD2, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID)))
						}
					}

					// Store last two samples for next frame's boundary check
					if nFrames >= 2 {
						sess.prevAudioL = float64(audio[(nFrames-2)*stride])
						sess.lastAudioL = audio[(nFrames-1)*stride]
						if stride > 1 {
							sess.lastAudioR = audio[(nFrames-1)*stride+1]
						}
					} else if nFrames == 1 {
						sess.prevAudioL = float64(sess.lastAudioL)
						sess.lastAudioL = audio[0]
						if stride > 1 && len(audio) >= 2 {
							sess.lastAudioR = audio[1]
						}
					}
					sess.lastAudioSet = true
				}
				sess.lastAudioTs = time.Now()
			}
			st.fanoutPCM(sess, pcm, pcmLen)
		}

		// Segment split (recording sessions only)
		if !sess.listenOnly && st.policy.MaxDuration > 0 && now.Sub(sess.startTime) >= st.policy.MaxDuration {
			segIdx := sess.segmentIdx + 1
			oldSubs := sess.audioSubs
			oldState := sess.captureDSPState()
			sess.audioSubs = nil
			closeSession(sess, &st.policy)
			s, err := st.openRecordingSession(sig, now)
			if err != nil {
				delete(st.sessions, sig.ID)
				continue
			}
			s.segmentIdx = segIdx
			s.audioSubs = oldSubs
			s.restoreDSPState(oldState)
			st.sessions[sig.ID] = s
			if st.telemetry != nil {
				st.telemetry.IncCounter("streamer.session.reopen", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sig.ID)))
				st.telemetry.Event("session_reopen", "info", "stream session rotated by max duration", telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sig.ID)), map[string]any{
					"old_session": sess.sessionID,
					"new_session": s.sessionID,
				})
			}
		}
	}

	// Close sessions for disappeared signals (with grace period)
	for id, sess := range st.sessions {
		if seen[id] {
			continue
		}
		gracePeriod := 3 * time.Second
		if sess.listenOnly {
			gracePeriod = 5 * time.Second
		}
		if now.Sub(sess.lastFeed) > gracePeriod {
			for _, sub := range sess.audioSubs {
				close(sub.ch)
			}
			sess.audioSubs = nil
			if !sess.listenOnly {
				closeSession(sess, &st.policy)
			}
			if st.telemetry != nil {
				st.telemetry.IncCounter("streamer.session.close", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", id), "session_id", sess.sessionID))
				st.telemetry.Event("session_close", "info", "stream session closed", telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", id), "session_id", sess.sessionID), map[string]any{
					"reason":      "signal_missing",
					"listen_only": sess.listenOnly,
				})
			}
			delete(st.sessions, id)
		}
	}
}

func (st *Streamer) signalHasListenerLocked(sig *detector.Signal) bool {
	if sess, ok := st.sessions[sig.ID]; ok && len(sess.audioSubs) > 0 {
		if st.policy.DebugLiveAudio {
			log.Printf("LIVEAUDIO MATCH: signal id=%d matched existing session listener center=%.3fMHz", sig.ID, sig.CenterHz/1e6)
		}
		return true
	}
	for subID, pl := range st.pendingListens {
		delta := math.Abs(sig.CenterHz - pl.freq)
		if delta < 200000 {
			if st.policy.DebugLiveAudio {
				log.Printf("LIVEAUDIO MATCH: signal id=%d matched pending subscriber=%d center=%.3fMHz req=%.3fMHz delta=%.0fHz", sig.ID, subID, sig.CenterHz/1e6, pl.freq/1e6, delta)
			}
			return true
		}
	}
	return false
}

func (st *Streamer) attachPendingListeners(sess *streamSession) {
	for subID, pl := range st.pendingListens {
		requestedMode := normalizeRequestedMode(pl.mode)
		if requestedMode != "" && sess.demodName != requestedMode {
			continue
		}
		if math.Abs(sess.centerHz-pl.freq) < 200000 {
			sess.audioSubs = append(sess.audioSubs, audioSub{id: subID, ch: pl.ch})
			delete(st.pendingListens, subID)

			// Send updated audio_info now that we know the real session params.
			// Prefix with 0x00 tag byte so ws/audio handler sends as TextMessage.
			infoJSON, _ := json.Marshal(sess.audioInfo())
			tagged := make([]byte, 1+len(infoJSON))
			tagged[0] = 0x00 // tag: audio_info
			copy(tagged[1:], infoJSON)
			select {
			case pl.ch <- tagged:
			default:
			}

			if audioDumpEnabled {
				now := time.Now()
				sess.debugDumpStart = now.Add(debugDumpDelay)
				sess.debugDumpUntil = sess.debugDumpStart.Add(debugDumpDuration)
				sess.debugDumpBase = filepath.Join("debug", fmt.Sprintf("signal-%d-window-%s", sess.signalID, now.Format("20060102-150405")))
				sess.demodDump = nil
				sess.finalDump = nil
			}
			log.Printf("STREAM: attached pending listener %d to signal %d (%.1fMHz %s ch=%d)",
				subID, sess.signalID, sess.centerHz/1e6, sess.demodName, sess.channels)
			if audioDumpEnabled {
				log.Printf("STREAM: debug dump armed signal=%d start=%s until=%s", sess.signalID, sess.debugDumpStart.Format(time.RFC3339), sess.debugDumpUntil.Format(time.RFC3339))
			}
		}
	}
}

// CloseAll finalises all sessions and stops the worker goroutine.
func (st *Streamer) RuntimeInfoBySignalID() map[int64]RuntimeSignalInfo {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make(map[int64]RuntimeSignalInfo, len(st.sessions))
	for _, sess := range st.sessions {
		out[sess.signalID] = RuntimeSignalInfo{
			DemodName:    sess.demodName,
			PlaybackMode: sess.playbackMode,
			StereoState:  sess.stereoState,
			Channels:     sess.channels,
			SampleRate:   sess.sampleRate,
		}
	}
	return out
}

func (st *Streamer) CloseAll() {
	close(st.summaryStop)
	st.summaryWG.Wait()
	close(st.feedCh)
	<-st.done

	st.mu.Lock()
	defer st.mu.Unlock()
	for id, sess := range st.sessions {
		for _, sub := range sess.audioSubs {
			close(sub.ch)
		}
		sess.audioSubs = nil
		if !sess.listenOnly {
			closeSession(sess, &st.policy)
		}
		if st.telemetry != nil {
			st.telemetry.IncCounter("streamer.session.close", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", id), "session_id", sess.sessionID))
		}
		delete(st.sessions, id)
	}
	for _, pl := range st.pendingListens {
		close(pl.ch)
	}
	st.pendingListens = nil
	if st.telemetry != nil {
		st.telemetry.Event("streamer_close_all", "info", "all stream sessions closed", nil, nil)
	}
	if st.debugSummary != nil {
		st.debugSummary.Close()
	}
}

// ActiveSessions returns the number of open streaming sessions.
func (st *Streamer) ActiveSessions() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.sessions)
}

// SubscribeAudio registers a live-listen subscriber for a given frequency.
//
// LL-2: Returns AudioInfo with correct channels and sample rate.
// LL-3: Returns error only on hard failures (nil streamer etc).
//
// If a matching session exists, attaches immediately. Otherwise, the
// subscriber is held as "pending" and will be attached when a matching
// signal appears in the next DSP frame.
func (st *Streamer) SubscribeAudio(freq float64, bw float64, mode string) (int64, <-chan []byte, AudioInfo, error) {
	ch := make(chan []byte, 64)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.nextSub++
	subID := st.nextSub

	requestedMode := normalizeRequestedMode(mode)

	// Try to find a matching session
	var bestSess *streamSession
	bestDist := math.MaxFloat64
	for _, sess := range st.sessions {
		if requestedMode != "" && sess.demodName != requestedMode {
			continue
		}
		d := math.Abs(sess.centerHz - freq)
		if d < bestDist {
			bestDist = d
			bestSess = sess
		}
	}

	if bestSess != nil && bestDist < 200000 {
		bestSess.audioSubs = append(bestSess.audioSubs, audioSub{id: subID, ch: ch})
		if audioDumpEnabled {
			now := time.Now()
			bestSess.debugDumpStart = now.Add(debugDumpDelay)
			bestSess.debugDumpUntil = bestSess.debugDumpStart.Add(debugDumpDuration)
			bestSess.debugDumpBase = filepath.Join("debug", fmt.Sprintf("signal-%d-window-%s", bestSess.signalID, now.Format("20060102-150405")))
			bestSess.demodDump = nil
			bestSess.finalDump = nil
		}
		info := bestSess.audioInfo()
		log.Printf("STREAM: subscriber %d attached to signal %d (%.1fMHz %s)",
			subID, bestSess.signalID, bestSess.centerHz/1e6, bestSess.demodName)
		if audioDumpEnabled {
			log.Printf("STREAM: debug dump armed signal=%d start=%s until=%s", bestSess.signalID, bestSess.debugDumpStart.Format(time.RFC3339), bestSess.debugDumpUntil.Format(time.RFC3339))
		}
		if st.telemetry != nil {
			st.telemetry.IncCounter("streamer.listener.attach", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", bestSess.signalID), "session_id", bestSess.sessionID))
		}
		return subID, ch, info, nil
	}

	// No matching session yet — add as pending listener
	st.pendingListens[subID] = &pendingListen{
		freq: freq,
		bw:   bw,
		mode: mode,
		ch:   ch,
	}
	info := defaultAudioInfoForMode(mode)
	log.Printf("STREAM: subscriber %d pending (freq=%.1fMHz)", subID, freq/1e6)
	log.Printf("LIVEAUDIO MATCH: subscriber=%d pending req=%.3fMHz bw=%.0f mode=%s", subID, freq/1e6, bw, mode)
	if st.telemetry != nil {
		st.telemetry.IncCounter("streamer.listener.pending", 1, nil)
		st.telemetry.SetGauge("streamer.pending_listeners", float64(len(st.pendingListens)), nil)
	}
	return subID, ch, info, nil
}

// UnsubscribeAudio removes a live-listen subscriber.
func (st *Streamer) UnsubscribeAudio(subID int64) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if pl, ok := st.pendingListens[subID]; ok {
		close(pl.ch)
		delete(st.pendingListens, subID)
		if st.telemetry != nil {
			st.telemetry.IncCounter("streamer.listener.unsubscribe", 1, telemetry.TagsFromPairs("kind", "pending"))
			st.telemetry.SetGauge("streamer.pending_listeners", float64(len(st.pendingListens)), nil)
		}
		return
	}

	for _, sess := range st.sessions {
		for i, sub := range sess.audioSubs {
			if sub.id == subID {
				close(sub.ch)
				sess.audioSubs = append(sess.audioSubs[:i], sess.audioSubs[i+1:]...)
				if st.telemetry != nil {
					st.telemetry.IncCounter("streamer.listener.unsubscribe", 1, telemetry.TagsFromPairs("kind", "active", "session_id", sess.sessionID))
				}
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Session: stateful extraction + demod
// ---------------------------------------------------------------------------

// processSnippet takes a pre-extracted IQ snippet and demodulates it with
// persistent state. Uses stateful FIR + polyphase resampler for exact 48kHz
// output with zero transient artifacts.
type iqHeadProbeStats struct {
	meanMag float64
	minMag  float64
	maxStep float64
	p95Step float64
	lowMag  int
}

func probeIQHeadStats(iq []complex64, probeLen int) iqHeadProbeStats {
	if probeLen <= 0 || len(iq) == 0 {
		return iqHeadProbeStats{}
	}
	if len(iq) < probeLen {
		probeLen = len(iq)
	}
	stats := iqHeadProbeStats{minMag: math.MaxFloat64}
	steps := make([]float64, 0, probeLen)
	var sum float64
	for i := 0; i < probeLen; i++ {
		v := iq[i]
		mag := math.Hypot(float64(real(v)), float64(imag(v)))
		sum += mag
		if mag < stats.minMag {
			stats.minMag = mag
		}
		if mag < 0.02 {
			stats.lowMag++
		}
		if i > 0 {
			p := iq[i-1]
			num := float64(real(p))*float64(imag(v)) - float64(imag(p))*float64(real(v))
			den := float64(real(p))*float64(real(v)) + float64(imag(p))*float64(imag(v))
			step := math.Abs(math.Atan2(num, den))
			steps = append(steps, step)
			if step > stats.maxStep {
				stats.maxStep = step
			}
		}
	}
	stats.meanMag = sum / float64(probeLen)
	if len(steps) > 0 {
		sorted := append([]float64(nil), steps...)
		sort.Float64s(sorted)
		idx := int(math.Round(0.95 * float64(len(sorted)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		stats.p95Step = sorted[idx]
	}
	if stats.minMag == math.MaxFloat64 {
		stats.minMag = 0
	}
	return stats
}

func (sess *streamSession) processSnippet(snippet []complex64, snipRate int, coll *telemetry.Collector) ([]float32, int) {
	if len(snippet) == 0 || snipRate <= 0 {
		return nil, 0
	}
	baseTags := telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID)
	if coll != nil {
		coll.SetGauge("iq.stage.snippet.length", float64(len(snippet)), baseTags)
		stats := probeIQHeadStats(snippet, 64)
		coll.Observe("iq.snippet.head_mean_mag", stats.meanMag, baseTags)
		coll.Observe("iq.snippet.head_min_mag", stats.minMag, baseTags)
		coll.Observe("iq.snippet.head_max_step", stats.maxStep, baseTags)
		coll.Observe("iq.snippet.head_p95_step", stats.p95Step, baseTags)
		coll.SetGauge("iq.snippet.head_low_magnitude_count", float64(stats.lowMag), baseTags)
		if sess.lastExtractIQSet {
			prevMag := math.Hypot(float64(real(sess.lastExtractIQ)), float64(imag(sess.lastExtractIQ)))
			currMag := math.Hypot(float64(real(snippet[0])), float64(imag(snippet[0])))
			deltaMag := math.Abs(currMag - prevMag)
			num := float64(real(sess.lastExtractIQ))*float64(imag(snippet[0])) - float64(imag(sess.lastExtractIQ))*float64(real(snippet[0]))
			den := float64(real(sess.lastExtractIQ))*float64(real(snippet[0])) + float64(imag(sess.lastExtractIQ))*float64(imag(snippet[0]))
			deltaPhase := math.Abs(math.Atan2(num, den))
			d2 := float64(real(snippet[0]-sess.lastExtractIQ))*float64(real(snippet[0]-sess.lastExtractIQ)) + float64(imag(snippet[0]-sess.lastExtractIQ))*float64(imag(snippet[0]-sess.lastExtractIQ))
			coll.Observe("iq.extract.output.boundary.delta_mag", deltaMag, baseTags)
			coll.Observe("iq.extract.output.boundary.delta_phase", deltaPhase, baseTags)
			coll.Observe("iq.extract.output.boundary.d2", d2, baseTags)
			coll.Observe("iq.extract.output.boundary.discontinuity_score", deltaMag+deltaPhase, baseTags)
		}
	}
	if len(snippet) > 0 {
		sess.prevExtractIQ = sess.lastExtractIQ
		sess.lastExtractIQ = snippet[len(snippet)-1]
		sess.lastExtractIQSet = true
	}

	isWFMStereo := sess.demodName == "WFM_STEREO"
	isWFM := sess.demodName == "WFM" || isWFMStereo

	demodName := sess.demodName
	if isWFMStereo {
		demodName = "WFM"
	}
	d := demod.Get(demodName)
	if d == nil {
		d = demod.Get("NFM")
	}
	if d == nil {
		return nil, 0
	}

	// The extra 1-sample discriminator overlap prepend was removed after it was
	// shown to shift the downstream decimation phase and create heavy click
	// artifacts in steady-state streaming/recording. The upstream extraction path
	// and the stateful FIR/decimation stages already provide continuity.
	fullSnip := snippet
	overlapApplied := false
	prevTailValid := false

	if logging.EnabledCategory("prefir") && len(fullSnip) > 0 {
		probeN := 64
		if len(fullSnip) < probeN {
			probeN = len(fullSnip)
		}
		minPreMag := math.MaxFloat64
		minPreIdx := 0
		maxPreStep := 0.0
		maxPreStepIdx := 0
		for i := 0; i < probeN; i++ {
			v := fullSnip[i]
			mag := math.Hypot(float64(real(v)), float64(imag(v)))
			if mag < minPreMag {
				minPreMag = mag
				minPreIdx = i
			}
			if i > 0 {
				p := fullSnip[i-1]
				num := float64(real(p))*float64(imag(v)) - float64(imag(p))*float64(real(v))
				den := float64(real(p))*float64(real(v)) + float64(imag(p))*float64(imag(v))
				step := math.Abs(math.Atan2(num, den))
				if step > maxPreStep {
					maxPreStep = step
					maxPreStepIdx = i - 1
				}
			}
		}
		logging.Debug("prefir", "pre_fir_head_probe", "signal", sess.signalID, "probe_len", probeN, "min_mag", minPreMag, "min_idx", minPreIdx, "max_step", maxPreStep, "max_step_idx", maxPreStepIdx, "snip_len", len(fullSnip))
		if minPreMag < 0.18 {
			logging.Warn("prefir", "pre_fir_head_dip", "signal", sess.signalID, "probe_len", probeN, "min_mag", minPreMag, "min_idx", minPreIdx, "max_step", maxPreStep, "max_step_idx", maxPreStepIdx)
		}
		if maxPreStep > 1.5 {
			logging.Warn("prefir", "pre_fir_head_step", "signal", sess.signalID, "probe_len", probeN, "max_step", maxPreStep, "max_step_idx", maxPreStepIdx, "min_mag", minPreMag, "min_idx", minPreIdx)
		}
	}

	// --- Stateful anti-alias FIR + decimation to demod rate ---
	demodRate := d.OutputSampleRate()
	decim1 := int(math.Round(float64(snipRate) / float64(demodRate)))
	if decim1 < 1 {
		decim1 = 1
	}
	// WFM override: force decim1=2 (256kHz) instead of round(512k/192k)=3 (170kHz).
	// At decim1=3, Nyquist is 85kHz which clips FM broadcast ±75kHz deviation.
	// At decim1=2, Nyquist is 128kHz → full FM deviation + stereo pilot + guard band.
	// Bonus: 256000→48000 resampler ratio is L=3/M=16 (96 taps, 1kB) instead of
	// the pathological L=24000/M=85333 (768k taps, 6MB) from 170666→48000.
	if isWFM && decim1 > 2 && snipRate/2 >= 200000 {
		decim1 = 2
	}
	actualDemodRate := snipRate / decim1
	logging.Debug("demod", "rates", "snipRate", snipRate, "decim1", decim1, "actual", actualDemodRate)

	var dec []complex64
	if decim1 > 1 {
		// FIR cutoff: for WFM, use 90kHz (above ±75kHz FM deviation + guard).
		// For NFM/other: use standard Nyquist*0.8 cutoff.
		cutoff := float64(actualDemodRate) / 2.0 * 0.8
		if isWFM {
			cutoff = 90000
		}

		// Lazy-init or reinit stateful FIR if parameters changed
		if sess.preDemodDecimator == nil || sess.preDemodRate != snipRate || sess.preDemodCutoff != cutoff || sess.preDemodDecim != decim1 {
			taps := dsp.LowpassFIR(cutoff, snipRate, 101)
			sess.preDemodFIR = dsp.NewStatefulFIRComplex(taps)
			sess.preDemodDecimator = dsp.NewStatefulDecimatingFIRComplex(taps, decim1)
			sess.preDemodRate = snipRate
			sess.preDemodCutoff = cutoff
			sess.preDemodDecim = decim1
			sess.preDemodDecimPhase = 0
			if coll != nil {
				coll.IncCounter("dsp.pre_demod.init", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
				coll.Event("prefir_reinit", "info", "pre-demod decimator reinitialized", telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID), map[string]any{
					"snip_rate": snipRate,
					"cutoff_hz": cutoff,
					"decim":     decim1,
				})
			}
		}

		decimPhaseBefore := sess.preDemodDecimPhase
		filtered := sess.preDemodFIR.ProcessInto(fullSnip, sess.growIQ(len(fullSnip)))
		dec = sess.preDemodDecimator.Process(fullSnip)
		sess.preDemodDecimPhase = sess.preDemodDecimator.Phase()
		if coll != nil {
			coll.Observe("dsp.pre_demod.decimation_factor", float64(decim1), baseTags)
			coll.SetGauge("iq.stage.pre_demod.length", float64(len(dec)), baseTags)
			decStats := probeIQHeadStats(dec, 64)
			coll.Observe("iq.pre_demod.head_mean_mag", decStats.meanMag, baseTags)
			coll.Observe("iq.pre_demod.head_min_mag", decStats.minMag, baseTags)
			coll.Observe("iq.pre_demod.head_max_step", decStats.maxStep, baseTags)
			coll.Observe("iq.pre_demod.head_p95_step", decStats.p95Step, baseTags)
			coll.SetGauge("iq.pre_demod.head_low_magnitude_count", float64(decStats.lowMag), baseTags)
		}
		logging.Debug("boundary", "snippet_path", "signal", sess.signalID, "overlap_applied", overlapApplied, "snip_len", len(snippet), "full_len", len(fullSnip), "filtered_len", len(filtered), "dec_len", len(dec), "decim1", decim1, "phase_before", decimPhaseBefore, "phase_after", sess.preDemodDecimPhase)
	} else {
		logging.Debug("boundary", "snippet_path", "signal", sess.signalID, "overlap_applied", overlapApplied, "snip_len", len(snippet), "full_len", len(fullSnip), "filtered_len", len(fullSnip), "dec_len", len(fullSnip), "decim1", decim1, "phase_before", 0, "phase_after", 0)
		dec = fullSnip
	}

	if decHeadTrimSamples > 0 && decHeadTrimSamples < len(dec) {
		logging.Warn("boundary", "dec_head_trim_applied", "signal", sess.signalID, "trim", decHeadTrimSamples, "before_len", len(dec))
		dec = dec[decHeadTrimSamples:]
		if coll != nil {
			coll.IncCounter("dsp.pre_demod.head_trim", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID)))
		}
	}

	if logging.EnabledCategory("boundary") && len(dec) > 0 {
		first := dec[0]
		if sess.lastDecIQSet {
			d2Re := math.Abs(2*float64(real(sess.lastDecIQ)) - float64(real(sess.prevDecIQ)) - float64(real(first)))
			d2Im := math.Abs(2*float64(imag(sess.lastDecIQ)) - float64(imag(sess.prevDecIQ)) - float64(imag(first)))
			d2Mag := math.Hypot(d2Re, d2Im)
			if d2Mag > 0.15 {
				logging.Warn("boundary", "dec_iq_boundary", "signal", sess.signalID, "d2", d2Mag)
				if coll != nil {
					coll.IncCounter("iq.dec.boundary.count", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
					coll.Observe("iq.dec.boundary.d2", d2Mag, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID)))
				}
			}
		}

		headN := 16
		if len(dec) < headN {
			headN = len(dec)
		}
		tailN := 16
		if len(dec) < tailN {
			tailN = len(dec)
		}
		var headSum, tailSum, minMag, maxMag float64
		minMag = math.MaxFloat64
		for i, v := range dec {
			mag := math.Hypot(float64(real(v)), float64(imag(v)))
			if mag < minMag {
				minMag = mag
			}
			if mag > maxMag {
				maxMag = mag
			}
			if i < headN {
				headSum += mag
			}
		}
		for i := len(dec) - tailN; i < len(dec); i++ {
			if i >= 0 {
				v := dec[i]
				tailSum += math.Hypot(float64(real(v)), float64(imag(v)))
			}
		}
		headAvg := 0.0
		if headN > 0 {
			headAvg = headSum / float64(headN)
		}
		tailAvg := 0.0
		if tailN > 0 {
			tailAvg = tailSum / float64(tailN)
		}
		logging.Debug("boundary", "dec_iq_meter", "signal", sess.signalID, "len", len(dec), "head_avg", headAvg, "tail_avg", tailAvg, "min_mag", minMag, "max_mag", maxMag)
		if tailAvg > 0 {
			ratio := headAvg / tailAvg
			if ratio < 0.75 || ratio > 1.25 {
				logging.Warn("boundary", "dec_iq_head_tail_skew", "signal", sess.signalID, "head_avg", headAvg, "tail_avg", tailAvg, "ratio", ratio)
			}
			if coll != nil {
				coll.Observe("iq.dec.head_tail_ratio", ratio, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
			}
		}

		probeN := 64
		if len(dec) < probeN {
			probeN = len(dec)
		}
		minHeadMag := math.MaxFloat64
		minHeadIdx := 0
		maxHeadStep := 0.0
		maxHeadStepIdx := 0
		for i := 0; i < probeN; i++ {
			v := dec[i]
			mag := math.Hypot(float64(real(v)), float64(imag(v)))
			if mag < minHeadMag {
				minHeadMag = mag
				minHeadIdx = i
			}
			if i > 0 {
				p := dec[i-1]
				num := float64(real(p))*float64(imag(v)) - float64(imag(p))*float64(real(v))
				den := float64(real(p))*float64(real(v)) + float64(imag(p))*float64(imag(v))
				step := math.Abs(math.Atan2(num, den))
				if step > maxHeadStep {
					maxHeadStep = step
					maxHeadStepIdx = i - 1
				}
			}
		}
		logging.Debug("boundary", "dec_iq_head_probe", "signal", sess.signalID, "probe_len", probeN, "min_mag", minHeadMag, "min_idx", minHeadIdx, "max_step", maxHeadStep, "max_step_idx", maxHeadStepIdx)
		if minHeadMag < 0.18 {
			logging.Warn("boundary", "dec_iq_head_dip", "signal", sess.signalID, "probe_len", probeN, "min_mag", minHeadMag, "min_idx", minHeadIdx, "max_step", maxHeadStep, "max_step_idx", maxHeadStepIdx)
		}
		if maxHeadStep > 1.5 {
			logging.Warn("boundary", "dec_iq_head_step", "signal", sess.signalID, "probe_len", probeN, "max_step", maxHeadStep, "max_step_idx", maxHeadStepIdx, "min_mag", minHeadMag, "min_idx", minHeadIdx)
		}
		if coll != nil {
			coll.Observe("iq.dec.magnitude.min", minMag, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
			coll.Observe("iq.dec.magnitude.max", maxMag, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
			coll.Observe("iq.dec.phase_step.max", maxHeadStep, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
		}

		if len(dec) >= 2 {
			sess.prevDecIQ = dec[len(dec)-2]
			sess.lastDecIQ = dec[len(dec)-1]
		} else {
			sess.prevDecIQ = sess.lastDecIQ
			sess.lastDecIQ = dec[0]
		}
		sess.lastDecIQSet = true
	}

	// --- FM/AM/etc Demod ---
	// For FM demod (NFM/WFM): bridge the block boundary by prepending the
	// previous block's last IQ sample. Without this, the discriminator loses
	// the cross-boundary phase step (1 audio sample missing per block) and
	// any phase discontinuity at the seam becomes an unsmoothed audio transient.
	var audio []float32
	isFMDemod := demodName == "NFM" || demodName == "WFM"
	if isFMDemod && sess.lastDiscrimIQSet && len(dec) > 0 {
		bridged := make([]complex64, len(dec)+1)
		bridged[0] = sess.lastDiscrimIQ
		copy(bridged[1:], dec)
		audio = d.Demod(bridged, actualDemodRate)
		// bridged produced len(dec) audio samples (= len(bridged)-1)
		// which is exactly the correct count for the new data
	} else {
		audio = d.Demod(dec, actualDemodRate)
	}
	if len(dec) > 0 {
		sess.lastDiscrimIQ = dec[len(dec)-1]
		sess.lastDiscrimIQSet = true
	}
	if len(audio) == 0 {
		return nil, 0
	}
	if coll != nil {
		coll.SetGauge("audio.stage.demod.length", float64(len(audio)), baseTags)
		probe := 64
		if len(audio) < probe {
			probe = len(audio)
		}
		if probe > 0 {
			var headAbs, tailAbs float64
			for i := 0; i < probe; i++ {
				headAbs += math.Abs(float64(audio[i]))
				tailAbs += math.Abs(float64(audio[len(audio)-probe+i]))
			}
			coll.Observe("audio.demod.head_mean_abs", headAbs/float64(probe), baseTags)
			coll.Observe("audio.demod.tail_mean_abs", tailAbs/float64(probe), baseTags)
			coll.Observe("audio.demod.edge_delta_abs", math.Abs(float64(audio[0])-float64(audio[len(audio)-1])), baseTags)
		}
	}
	if logging.EnabledCategory("boundary") {
		stride := d.Channels()
		if stride < 1 {
			stride = 1
		}
		nFrames := len(audio) / stride
		if nFrames > 0 {
			first := float64(audio[0])
			if sess.lastDemodSet {
				d2 := math.Abs(2*float64(sess.lastDemodL) - sess.prevDemodL - first)
				if d2 > 0.15 {
					logging.Warn("boundary", "demod_boundary", "signal", sess.signalID, "d2", d2)
					if coll != nil {
						coll.IncCounter("audio.demod_boundary.count", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
						coll.Observe("audio.demod_boundary.d2", d2, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID)))
					}
				}
			}
			if nFrames >= 2 {
				sess.prevDemodL = float64(audio[(nFrames-2)*stride])
				sess.lastDemodL = audio[(nFrames-1)*stride]
			} else {
				sess.prevDemodL = float64(sess.lastDemodL)
				sess.lastDemodL = audio[0]
			}
			sess.lastDemodSet = true
		}
	}
	logging.Debug("boundary", "audio_path", "signal", sess.signalID, "demod", demodName, "actual_rate", actualDemodRate, "audio_len", len(audio), "channels", d.Channels(), "overlap_applied", overlapApplied, "prev_tail_valid", prevTailValid)

	shouldDump := !sess.debugDumpStart.IsZero() && !sess.debugDumpUntil.IsZero()
	if shouldDump {
		now := time.Now()
		shouldDump = !now.Before(sess.debugDumpStart) && now.Before(sess.debugDumpUntil)
	}
	if shouldDump {
		sess.demodDump = append(sess.demodDump, audio...)
	}

	// --- Stateful stereo decode with conservative lock/hysteresis ---
	channels := 1
	if isWFMStereo {
		sess.playbackMode = "WFM_STEREO"
		channels = 2 // keep transport format stable for live WFM_STEREO sessions
		stereoAudio, locked := sess.stereoDecodeStateful(audio, actualDemodRate)
		if locked {
			sess.stereoOnCount++
			sess.stereoOffCount = 0
			if sess.stereoOnCount >= 4 {
				sess.stereoEnabled = true
			}
		} else {
			sess.stereoOnCount = 0
			sess.stereoOffCount++
			if sess.stereoOffCount >= 10 {
				sess.stereoEnabled = false
			}
		}
		prevPlayback := sess.playbackMode
		prevStereo := sess.stereoState
		if sess.stereoEnabled && len(stereoAudio) > 0 {
			sess.stereoState = "locked"
			audio = stereoAudio
		} else {
			sess.stereoState = "mono-fallback"
			// Apply 15kHz LPF before output: the raw discriminator contains
			// the 19kHz pilot (+55dB), L-R subcarrier (23-53kHz), and RDS (57kHz).
			// Without filtering, the pilot leaks into audio and subcarrier
			// energy produces audible click-like artifacts.
			audio = sess.wfmAudioFilter(audio, actualDemodRate)
			dual := make([]float32, len(audio)*2)
			for i, s := range audio {
				dual[i*2] = s
				dual[i*2+1] = s
			}
			audio = dual
		}
		if (prevPlayback != sess.playbackMode || prevStereo != sess.stereoState) && len(sess.audioSubs) > 0 {
			sendAudioInfo(sess.audioSubs, sess.audioInfo())
		}
	} else if isWFM {
		// Plain WFM (not stereo): also needs 15kHz LPF on discriminator output
		audio = sess.wfmAudioFilter(audio, actualDemodRate)
	}

	// --- Polyphase resample to exact 48kHz ---
	if actualDemodRate != streamAudioRate {
		if channels > 1 {
			if sess.stereoResampler == nil || sess.stereoResamplerRate != actualDemodRate {
				logging.Info("resample", "reset", "mode", "stereo", "rate", actualDemodRate)
				sess.stereoResampler = dsp.NewStereoResampler(actualDemodRate, streamAudioRate, resamplerTaps)
				sess.stereoResamplerRate = actualDemodRate
				if coll != nil {
					coll.Event("resampler_reset", "info", "stereo resampler reset", telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID), map[string]any{
						"mode": "stereo",
						"rate": actualDemodRate,
					})
				}
			}
			audio = sess.stereoResampler.Process(audio)
		} else {
			if sess.monoResampler == nil || sess.monoResamplerRate != actualDemodRate {
				logging.Info("resample", "reset", "mode", "mono", "rate", actualDemodRate)
				sess.monoResampler = dsp.NewResampler(actualDemodRate, streamAudioRate, resamplerTaps)
				sess.monoResamplerRate = actualDemodRate
				if coll != nil {
					coll.Event("resampler_reset", "info", "mono resampler reset", telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID), map[string]any{
						"mode": "mono",
						"rate": actualDemodRate,
					})
				}
			}
			audio = sess.monoResampler.Process(audio)
		}
	}
	if coll != nil {
		coll.SetGauge("audio.stage.output.length", float64(len(audio)), telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
	}

	// --- De-emphasis (configurable: 50µs Europe, 75µs US/Japan, 0=disabled) ---
	if isWFM && sess.deemphasisUs > 0 && streamAudioRate > 0 {
		tau := sess.deemphasisUs * 1e-6
		alpha := math.Exp(-1.0 / (float64(streamAudioRate) * tau))
		if channels > 1 {
			nFrames := len(audio) / channels
			yL, yR := sess.deemphL, sess.deemphR
			for i := 0; i < nFrames; i++ {
				yL = alpha*yL + (1-alpha)*float64(audio[i*2])
				audio[i*2] = float32(yL)
				yR = alpha*yR + (1-alpha)*float64(audio[i*2+1])
				audio[i*2+1] = float32(yR)
			}
			sess.deemphL, sess.deemphR = yL, yR
		} else {
			y := sess.deemphL
			for i := range audio {
				y = alpha*y + (1-alpha)*float64(audio[i])
				audio[i] = float32(y)
			}
			sess.deemphL = y
		}
	}

	if isWFM {
		for i := range audio {
			audio[i] *= 0.35
		}
	}

	if shouldDump {
		sess.finalDump = append(sess.finalDump, audio...)
	} else if !sess.debugDumpUntil.IsZero() && time.Now().After(sess.debugDumpUntil) && sess.debugDumpBase != "" {
		_ = os.MkdirAll(filepath.Dir(sess.debugDumpBase), 0o755)
		if len(sess.demodDump) > 0 {
			_ = writeWAVFile(sess.debugDumpBase+"-demod.wav", sess.demodDump, actualDemodRate, d.Channels())
		}
		if len(sess.finalDump) > 0 {
			_ = writeWAVFile(sess.debugDumpBase+"-final.wav", sess.finalDump, streamAudioRate, channels)
		}
		logging.Warn("boundary", "debug_audio_dump_window", "signal", sess.signalID, "base", sess.debugDumpBase)
		sess.debugDumpBase = ""
		sess.demodDump = nil
		sess.finalDump = nil
		sess.debugDumpStart = time.Time{}
		sess.debugDumpUntil = time.Time{}
	}

	return audio, streamAudioRate
}

// pllCoefficients returns the proportional (alpha) and integral (beta) gains
// for a Type-II PLL using the specified loop bandwidth and damping factor.
// loopBW is in Hz, sampleRate in samples/sec.
func pllCoefficients(loopBW, damping float64, sampleRate int) (float64, float64) {
	if sampleRate <= 0 || loopBW <= 0 {
		return 0, 0
	}
	bl := loopBW / float64(sampleRate)
	theta := bl / (damping + 0.25/damping)
	d := 1 + 2*damping*theta + theta*theta
	alpha := (4 * damping * theta) / d
	beta := (4 * theta * theta) / d
	return alpha, beta
}

// wfmAudioFilter applies a stateful 15kHz lowpass to WFM discriminator output.
// Removes the 19kHz stereo pilot, L-R DSB-SC subcarrier (23-53kHz), and RDS (57kHz)
// that would otherwise leak into the audio output as clicks and tonal artifacts.
func (sess *streamSession) wfmAudioFilter(audio []float32, sampleRate int) []float32 {
	if len(audio) == 0 || sampleRate <= 0 {
		return audio
	}
	if sess.wfmAudioLPF == nil || sess.wfmAudioLPFRate != sampleRate {
		sess.wfmAudioLPF = dsp.NewStatefulFIRReal(dsp.LowpassFIR(15000, sampleRate, 101))
		sess.wfmAudioLPFRate = sampleRate
	}
	return sess.wfmAudioLPF.Process(audio)
}

// stereoDecodeStateful: pilot-locked 38kHz oscillator for L-R extraction.
// Uses persistent FIR filter state across frames for click-free stereo.
// Reuses session scratch buffers to minimize allocations.
func (sess *streamSession) stereoDecodeStateful(mono []float32, sampleRate int) ([]float32, bool) {
	if len(mono) == 0 || sampleRate <= 0 {
		return nil, false
	}
	n := len(mono)

	// Rebuild rate-dependent stereo filters when sampleRate changes
	if sess.stereoLPF == nil || sess.stereoFilterRate != sampleRate {
		lp := dsp.LowpassFIR(15000, sampleRate, 101)
		sess.stereoLPF = dsp.NewStatefulFIRReal(lp)
		sess.stereoBPHi = dsp.NewStatefulFIRReal(dsp.LowpassFIR(53000, sampleRate, 101))
		sess.stereoBPLo = dsp.NewStatefulFIRReal(dsp.LowpassFIR(23000, sampleRate, 101))
		sess.stereoLRLPF = dsp.NewStatefulFIRReal(lp)
		// Narrow pilot bandpass via LPF(21k)-LPF(17k).
		sess.pilotLPFHi = dsp.NewStatefulFIRReal(dsp.LowpassFIR(21000, sampleRate, 101))
		sess.pilotLPFLo = dsp.NewStatefulFIRReal(dsp.LowpassFIR(17000, sampleRate, 101))
		sess.stereoFilterRate = sampleRate
		// Initialize PLL for 19kHz pilot tracking.
		sess.pilotPhase = 0
		sess.pilotFreq = 2 * math.Pi * 19000 / float64(sampleRate)
		sess.pilotAlpha, sess.pilotBeta = pllCoefficients(50, 0.707, sampleRate)
		sess.pilotErrAvg = 0
		sess.pilotI = 0
		sess.pilotQ = 0
		sess.pilotLPAlpha = 1 - math.Exp(-2*math.Pi*200/float64(sampleRate))
	}

	// Reuse scratch for intermediates: lpr, bpfLR, lr, work1, work2.
	scratch := sess.growAudio(n * 5)
	lpr := scratch[:n]
	bpfLR := scratch[n : 2*n]
	lr := scratch[2*n : 3*n]
	work1 := scratch[3*n : 4*n]
	work2 := scratch[4*n : 5*n]

	sess.stereoLPF.ProcessInto(mono, lpr)

	// 23-53kHz bandpass for L-R DSB-SC.
	sess.stereoBPHi.ProcessInto(mono, work1)
	sess.stereoBPLo.ProcessInto(mono, work2)
	for i := 0; i < n; i++ {
		bpfLR[i] = work1[i] - work2[i]
	}

	// 19kHz pilot bandpass for PLL.
	sess.pilotLPFHi.ProcessInto(mono, work1)
	sess.pilotLPFLo.ProcessInto(mono, work2)
	for i := 0; i < n; i++ {
		work1[i] = work1[i] - work2[i]
	}
	pilot := work1

	phase := sess.pilotPhase
	freq := sess.pilotFreq
	alpha := sess.pilotAlpha
	beta := sess.pilotBeta
	iState := sess.pilotI
	qState := sess.pilotQ
	lpAlpha := sess.pilotLPAlpha
	minFreq := 2 * math.Pi * 17000 / float64(sampleRate)
	maxFreq := 2 * math.Pi * 21000 / float64(sampleRate)
	var pilotPower float64
	var totalPower float64
	var errSum float64
	for i := 0; i < n; i++ {
		p := float64(pilot[i])
		sinP, cosP := math.Sincos(phase)
		iMix := p * cosP
		qMix := p * -sinP
		iState += lpAlpha * (iMix - iState)
		qState += lpAlpha * (qMix - qState)
		err := math.Atan2(qState, iState)
		freq += beta * err
		if freq < minFreq {
			freq = minFreq
		} else if freq > maxFreq {
			freq = maxFreq
		}
		phase += freq + alpha*err
		if phase > 2*math.Pi {
			phase -= 2 * math.Pi
		} else if phase < 0 {
			phase += 2 * math.Pi
		}

		totalPower += float64(mono[i]) * float64(mono[i])
		pilotPower += p * p
		errSum += math.Abs(err)

		lr[i] = bpfLR[i] * float32(2*math.Sin(2*phase))
	}
	sess.pilotPhase = phase
	sess.pilotFreq = freq
	sess.pilotI = iState
	sess.pilotQ = qState
	blockErr := errSum / float64(n)
	sess.pilotErrAvg = 0.9*sess.pilotErrAvg + 0.1*blockErr

	lr = sess.stereoLRLPF.ProcessInto(lr, lr)

	pilotRatio := 0.0
	if totalPower > 0 {
		pilotRatio = pilotPower / totalPower
	}
	freqHz := sess.pilotFreq * float64(sampleRate) / (2 * math.Pi)
	// Lock heuristics: pilot power fraction and PLL phase error stability.
	// Pilot power is a small but stable fraction of composite energy; require
	// a modest floor plus PLL settling to avoid flapping in noise.
	locked := pilotRatio > 0.003 && math.Abs(freqHz-19000) < 250 && sess.pilotErrAvg < 0.35

	out := make([]float32, n*2)
	for i := 0; i < n; i++ {
		out[i*2] = 0.5 * (lpr[i] + lr[i])
		out[i*2+1] = 0.5 * (lpr[i] - lr[i])
	}
	return out, locked
}

// dspStateSnapshot captures persistent DSP state for segment splits.
type dspStateSnapshot struct {
	overlapIQ           []complex64
	deemphL             float64
	deemphR             float64
	pilotPhase          float64
	pilotFreq           float64
	pilotAlpha          float64
	pilotBeta           float64
	pilotErrAvg         float64
	pilotI              float64
	pilotQ              float64
	pilotLPAlpha        float64
	monoResampler       *dsp.Resampler
	monoResamplerRate   int
	stereoResampler     *dsp.StereoResampler
	stereoResamplerRate int
	stereoLPF           *dsp.StatefulFIRReal
	stereoFilterRate    int
	stereoBPHi          *dsp.StatefulFIRReal
	stereoBPLo          *dsp.StatefulFIRReal
	stereoLRLPF         *dsp.StatefulFIRReal
	stereoAALPF         *dsp.StatefulFIRReal
	pilotLPFHi          *dsp.StatefulFIRReal
	pilotLPFLo          *dsp.StatefulFIRReal
	preDemodFIR         *dsp.StatefulFIRComplex
	preDemodDecimator   *dsp.StatefulDecimatingFIRComplex
	preDemodDecim       int
	preDemodRate        int
	preDemodCutoff      float64
	preDemodDecimPhase  int
	wfmAudioLPF         *dsp.StatefulFIRReal
	wfmAudioLPFRate     int
}

func (sess *streamSession) captureDSPState() dspStateSnapshot {
	return dspStateSnapshot{
		overlapIQ:           sess.overlapIQ,
		deemphL:             sess.deemphL,
		deemphR:             sess.deemphR,
		pilotPhase:          sess.pilotPhase,
		pilotFreq:           sess.pilotFreq,
		pilotAlpha:          sess.pilotAlpha,
		pilotBeta:           sess.pilotBeta,
		pilotErrAvg:         sess.pilotErrAvg,
		pilotI:              sess.pilotI,
		pilotQ:              sess.pilotQ,
		pilotLPAlpha:        sess.pilotLPAlpha,
		monoResampler:       sess.monoResampler,
		monoResamplerRate:   sess.monoResamplerRate,
		stereoResampler:     sess.stereoResampler,
		stereoResamplerRate: sess.stereoResamplerRate,
		stereoLPF:           sess.stereoLPF,
		stereoFilterRate:    sess.stereoFilterRate,
		stereoBPHi:          sess.stereoBPHi,
		stereoBPLo:          sess.stereoBPLo,
		stereoLRLPF:         sess.stereoLRLPF,
		stereoAALPF:         sess.stereoAALPF,
		pilotLPFHi:          sess.pilotLPFHi,
		pilotLPFLo:          sess.pilotLPFLo,
		preDemodFIR:         sess.preDemodFIR,
		preDemodDecimator:   sess.preDemodDecimator,
		preDemodDecim:       sess.preDemodDecim,
		preDemodRate:        sess.preDemodRate,
		preDemodCutoff:      sess.preDemodCutoff,
		preDemodDecimPhase:  sess.preDemodDecimPhase,
		wfmAudioLPF:         sess.wfmAudioLPF,
		wfmAudioLPFRate:     sess.wfmAudioLPFRate,
	}
}

func (sess *streamSession) restoreDSPState(s dspStateSnapshot) {
	sess.overlapIQ = s.overlapIQ
	sess.deemphL = s.deemphL
	sess.deemphR = s.deemphR
	sess.pilotPhase = s.pilotPhase
	sess.pilotFreq = s.pilotFreq
	sess.pilotAlpha = s.pilotAlpha
	sess.pilotBeta = s.pilotBeta
	sess.pilotErrAvg = s.pilotErrAvg
	sess.pilotI = s.pilotI
	sess.pilotQ = s.pilotQ
	sess.pilotLPAlpha = s.pilotLPAlpha
	sess.monoResampler = s.monoResampler
	sess.monoResamplerRate = s.monoResamplerRate
	sess.stereoResampler = s.stereoResampler
	sess.stereoResamplerRate = s.stereoResamplerRate
	sess.stereoLPF = s.stereoLPF
	sess.stereoFilterRate = s.stereoFilterRate
	sess.stereoBPHi = s.stereoBPHi
	sess.stereoBPLo = s.stereoBPLo
	sess.stereoLRLPF = s.stereoLRLPF
	sess.stereoAALPF = s.stereoAALPF
	sess.pilotLPFHi = s.pilotLPFHi
	sess.pilotLPFLo = s.pilotLPFLo
	sess.preDemodFIR = s.preDemodFIR
	sess.preDemodDecimator = s.preDemodDecimator
	sess.preDemodDecim = s.preDemodDecim
	sess.preDemodRate = s.preDemodRate
	sess.preDemodCutoff = s.preDemodCutoff
	sess.preDemodDecimPhase = s.preDemodDecimPhase
	sess.wfmAudioLPF = s.wfmAudioLPF
	sess.wfmAudioLPFRate = s.wfmAudioLPFRate
}

// ---------------------------------------------------------------------------
// Session management helpers
// ---------------------------------------------------------------------------

func (st *Streamer) openRecordingSession(sig *detector.Signal, now time.Time) (*streamSession, error) {
	outputDir := st.policy.OutputDir
	if outputDir == "" {
		outputDir = "data/recordings"
	}

	demodName, channels := resolveDemod(sig)

	dirName := fmt.Sprintf("%s_%.0fHz_stream%d",
		now.Format("2006-01-02T15-04-05"), sig.CenterHz, sig.ID)
	dir := filepath.Join(outputDir, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	wavPath := filepath.Join(dir, "audio.wav")
	f, err := os.Create(wavPath)
	if err != nil {
		return nil, err
	}
	if err := writeStreamWAVHeader(f, streamAudioRate, channels); err != nil {
		f.Close()
		return nil, err
	}

	playbackMode, stereoState := initialPlaybackState(demodName)

	sess := &streamSession{
		sessionID:    fmt.Sprintf("%d-%d-r", sig.ID, now.UnixMilli()),
		signalID:     sig.ID,
		centerHz:     sig.CenterHz,
		bwHz:         sig.BWHz,
		snrDb:        sig.SNRDb,
		peakDb:       sig.PeakDb,
		class:        sig.Class,
		startTime:    now,
		lastFeed:     now,
		dir:          dir,
		wavFile:      f,
		wavBuf:       bufio.NewWriterSize(f, 64*1024),
		sampleRate:   streamAudioRate,
		channels:     channels,
		demodName:    demodName,
		playbackMode: playbackMode,
		stereoState:  stereoState,
		deemphasisUs: st.policy.DeemphasisUs,
	}

	log.Printf("STREAM: opened recording signal=%d %.1fMHz %s dir=%s",
		sig.ID, sig.CenterHz/1e6, demodName, dirName)
	return sess, nil
}

func (st *Streamer) openListenSession(sig *detector.Signal, now time.Time) *streamSession {
	demodName, channels := resolveDemod(sig)
	for _, pl := range st.pendingListens {
		if math.Abs(sig.CenterHz-pl.freq) < 200000 {
			if requested := normalizeRequestedMode(pl.mode); requested != "" {
				demodName = requested
				if demodName == "WFM_STEREO" {
					channels = 2
				} else if d := demod.Get(demodName); d != nil {
					channels = d.Channels()
				} else {
					channels = 1
				}
				break
			}
		}
	}
	playbackMode, stereoState := initialPlaybackState(demodName)

	sess := &streamSession{
		sessionID:    fmt.Sprintf("%d-%d-l", sig.ID, now.UnixMilli()),
		signalID:     sig.ID,
		centerHz:     sig.CenterHz,
		bwHz:         sig.BWHz,
		snrDb:        sig.SNRDb,
		peakDb:       sig.PeakDb,
		class:        sig.Class,
		startTime:    now,
		lastFeed:     now,
		listenOnly:   true,
		sampleRate:   streamAudioRate,
		channels:     channels,
		demodName:    demodName,
		playbackMode: playbackMode,
		stereoState:  stereoState,
		deemphasisUs: st.policy.DeemphasisUs,
	}

	log.Printf("STREAM: opened listen-only signal=%d %.1fMHz %s",
		sig.ID, sig.CenterHz/1e6, demodName)
	return sess
}

func resolveDemod(sig *detector.Signal) (string, int) {
	demodName := "NFM"
	if sig.Class != nil {
		if n := mapClassToDemod(sig.Class.ModType); n != "" {
			demodName = n
		}
	}
	channels := 1
	if demodName == "WFM_STEREO" {
		channels = 2
	} else if d := demod.Get(demodName); d != nil {
		channels = d.Channels()
	}
	return demodName, channels
}

func initialPlaybackState(demodName string) (string, string) {
	playbackMode := demodName
	stereoState := "mono"
	if demodName == "WFM_STEREO" {
		stereoState = "searching"
	}
	return playbackMode, stereoState
}

func (sess *streamSession) audioInfo() AudioInfo {
	return AudioInfo{
		SampleRate:   sess.sampleRate,
		Channels:     sess.channels,
		Format:       "s16le",
		DemodName:    sess.demodName,
		PlaybackMode: sess.playbackMode,
		StereoState:  sess.stereoState,
	}
}

func sendAudioInfo(subs []audioSub, info AudioInfo) {
	infoJSON, _ := json.Marshal(info)
	tagged := make([]byte, 1+len(infoJSON))
	tagged[0] = 0x00 // tag: audio_info
	copy(tagged[1:], infoJSON)
	for _, sub := range subs {
		select {
		case sub.ch <- tagged:
		default:
		}
	}
}

func defaultAudioInfoForMode(mode string) AudioInfo {
	demodName := "NFM"
	if requested := normalizeRequestedMode(mode); requested != "" {
		demodName = requested
	}
	channels := 1
	if demodName == "WFM_STEREO" {
		channels = 2
	} else if d := demod.Get(demodName); d != nil {
		channels = d.Channels()
	}
	playbackMode, stereoState := initialPlaybackState(demodName)
	return AudioInfo{
		SampleRate:   streamAudioRate,
		Channels:     channels,
		Format:       "s16le",
		DemodName:    demodName,
		PlaybackMode: playbackMode,
		StereoState:  stereoState,
	}
}

func normalizeRequestedMode(mode string) string {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "", "AUTO":
		return ""
	case "WFM", "WFM_STEREO", "NFM", "AM", "USB", "LSB", "CW":
		return strings.ToUpper(strings.TrimSpace(mode))
	default:
		return ""
	}
}

// growIQ returns a complex64 slice of at least n elements, reusing sess.scratchIQ.
func (sess *streamSession) growIQ(n int) []complex64 {
	if cap(sess.scratchIQ) >= n {
		return sess.scratchIQ[:n]
	}
	sess.scratchIQ = make([]complex64, n, n*5/4)
	return sess.scratchIQ
}

// growAudio returns a float32 slice of at least n elements, reusing sess.scratchAudio.
func (sess *streamSession) growAudio(n int) []float32 {
	if cap(sess.scratchAudio) >= n {
		return sess.scratchAudio[:n]
	}
	sess.scratchAudio = make([]float32, n, n*5/4)
	return sess.scratchAudio
}

// growPCM returns a byte slice of at least n bytes, reusing sess.scratchPCM.
func (sess *streamSession) growPCM(n int) []byte {
	if cap(sess.scratchPCM) >= n {
		return sess.scratchPCM[:n]
	}
	sess.scratchPCM = make([]byte, n, n*5/4)
	return sess.scratchPCM
}

func convertToListenOnly(sess *streamSession) {
	if sess.wavBuf != nil {
		_ = sess.wavBuf.Flush()
	}
	if sess.wavFile != nil {
		fixStreamWAVHeader(sess.wavFile, sess.wavSamples, sess.sampleRate, sess.channels)
		sess.wavFile.Close()
	}
	sess.wavFile = nil
	sess.wavBuf = nil
	sess.listenOnly = true
	log.Printf("STREAM: converted signal=%d to listen-only", sess.signalID)
}

func closeSession(sess *streamSession, policy *Policy) {
	if sess.listenOnly {
		return
	}
	if sess.wavBuf != nil {
		_ = sess.wavBuf.Flush()
	}
	if sess.wavFile != nil {
		fixStreamWAVHeader(sess.wavFile, sess.wavSamples, sess.sampleRate, sess.channels)
		sess.wavFile.Close()
		sess.wavFile = nil
		sess.wavBuf = nil
	}

	dur := sess.lastFeed.Sub(sess.startTime)
	files := map[string]any{
		"audio":             "audio.wav",
		"audio_sample_rate": sess.sampleRate,
		"audio_channels":    sess.channels,
		"audio_demod":       sess.demodName,
		"recording_mode":    "streaming",
	}
	meta := Meta{
		EventID:     sess.signalID,
		Start:       sess.startTime,
		End:         sess.lastFeed,
		CenterHz:    sess.centerHz,
		BandwidthHz: sess.bwHz,
		SampleRate:  sess.sampleRate,
		SNRDb:       sess.snrDb,
		PeakDb:      sess.peakDb,
		Class:       sess.class,
		DurationMs:  dur.Milliseconds(),
		Files:       files,
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Join(sess.dir, "meta.json"), b, 0o644)
	}
	if policy != nil {
		enforceQuota(policy.OutputDir, policy.MaxDiskMB)
	}
}

func (st *Streamer) fanoutPCM(sess *streamSession, pcm []byte, pcmLen int) {
	if len(sess.audioSubs) == 0 {
		return
	}
	// Tag + copy for all subscribers: 0x01 prefix = PCM audio
	tagged := make([]byte, 1+pcmLen)
	tagged[0] = 0x01
	copy(tagged[1:], pcm[:pcmLen])
	alive := sess.audioSubs[:0]
	for _, sub := range sess.audioSubs {
		select {
		case sub.ch <- tagged:
			alive = append(alive, sub)
		default:
			st.droppedPCM++
			logging.Warn("drop", "pcm_drop", "count", st.droppedPCM)
			if st.telemetry != nil {
				st.telemetry.IncCounter("streamer.pcm.drop", 1, telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
			}
		}
	}
	sess.audioSubs = alive
	if st.telemetry != nil {
		st.telemetry.SetGauge("streamer.subscribers.count", float64(len(alive)), telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", sess.signalID), "session_id", sess.sessionID))
	}
}

func (st *Streamer) summaryWorker() {
	defer st.summaryWG.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			st.writePeriodicSummary()
		case <-st.summaryStop:
			return
		}
	}
}

func (st *Streamer) writePeriodicSummary() {
	st.mu.Lock()
	activeSubscribers := 0
	for _, sess := range st.sessions {
		activeSubscribers += len(sess.audioSubs)
	}
	processAvg := 0.0
	if st.processLoopCount > 0 {
		processAvg = st.processLoopSumMs / float64(st.processLoopCount)
	}
	summary := map[string]any{
		"ts":                    time.Now().UTC().Format(time.RFC3339Nano),
		"feed_drop_total":       st.droppedFeed,
		"pcm_drop_total":        st.droppedPCM,
		"active_subscribers":    activeSubscribers,
		"pending_listeners":     len(st.pendingListens),
		"active_sessions":       len(st.sessions),
		"produced_pcm_frames":   st.producedPCMFrames,
		"process_loop_ms_avg":   processAvg,
		"process_loop_ms_max":   st.processLoopMaxMs,
		"feed_queue_len":        len(st.feedCh),
		"feed_queue_cap":        cap(st.feedCh),
		"feed_queue_fill_ratio": safeRatio(float64(len(st.feedCh)), float64(cap(st.feedCh))),
		"backpressure_hint":     st.backpressureHintLocked(),
	}
	st.processLoopCount = 0
	st.processLoopSumMs = 0
	st.processLoopMaxMs = 0
	st.mu.Unlock()

	if st.debugSummary != nil {
		_ = st.debugSummary.WriteServerSummary(summary)
	}
}

func (st *Streamer) backpressureHintLocked() string {
	queueLen := len(st.feedCh)
	queueCap := cap(st.feedCh)
	if queueCap > 0 && float64(queueLen)/float64(queueCap) >= 0.8 {
		return "feed_queue_high"
	}
	if st.droppedFeed > 0 || st.droppedPCM > 0 {
		return "drops_seen"
	}
	if len(st.pendingListens) > 0 {
		return "pending_listeners"
	}
	return "ok"
}

func safeRatio(a float64, b float64) float64 {
	if b <= 0 {
		return 0
	}
	return a / b
}

func (st *Streamer) AppendBrowserAudioSummary(v any) error {
	if st == nil || st.debugSummary == nil {
		return nil
	}
	return st.debugSummary.WriteBrowserSummary(v)
}

func (st *Streamer) classAllowed(cls *classifier.Classification) bool {
	if len(st.policy.ClassFilter) == 0 {
		return true
	}
	if cls == nil {
		return false
	}
	for _, f := range st.policy.ClassFilter {
		if strings.EqualFold(f, string(cls.ModType)) {
			return true
		}
	}
	return false
}

// ErrNoSession is returned when no matching signal session exists.
var ErrNoSession = errors.New("no active or pending session for this frequency")

// ---------------------------------------------------------------------------
// WAV header helpers
// ---------------------------------------------------------------------------

func writeWAVFile(path string, audio []float32, sampleRate int, channels int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeWAVTo(f, audio, sampleRate, channels)
}

func writeStreamWAVHeader(f *os.File, sampleRate int, channels int) error {
	if channels <= 0 {
		channels = 1
	}
	hdr := make([]byte, 44)
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], 36)
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)
	binary.LittleEndian.PutUint16(hdr[20:22], 1)
	binary.LittleEndian.PutUint16(hdr[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(hdr[28:32], uint32(sampleRate*channels*2))
	binary.LittleEndian.PutUint16(hdr[32:34], uint16(channels*2))
	binary.LittleEndian.PutUint16(hdr[34:36], 16)
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], 0)
	_, err := f.Write(hdr)
	return err
}

func fixStreamWAVHeader(f *os.File, totalSamples int64, sampleRate int, channels int) {
	dataSize := uint32(totalSamples * 2)
	var buf [4]byte

	binary.LittleEndian.PutUint32(buf[:], 36+dataSize)
	if _, err := f.Seek(4, 0); err != nil {
		return
	}
	_, _ = f.Write(buf[:])

	binary.LittleEndian.PutUint32(buf[:], uint32(sampleRate))
	if _, err := f.Seek(24, 0); err != nil {
		return
	}
	_, _ = f.Write(buf[:])

	binary.LittleEndian.PutUint32(buf[:], uint32(sampleRate*channels*2))
	if _, err := f.Seek(28, 0); err != nil {
		return
	}
	_, _ = f.Write(buf[:])

	binary.LittleEndian.PutUint32(buf[:], dataSize)
	if _, err := f.Seek(40, 0); err != nil {
		return
	}
	_, _ = f.Write(buf[:])
}

// ResetStreams forces all active streaming sessions to discard their FIR states and decimation phases.
// This is used when the upstream DSP drops samples, creating a hard break in phase continuity.
func (st *Streamer) ResetStreams() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.telemetry != nil {
		st.telemetry.IncCounter("streamer.reset.count", 1, nil)
		st.telemetry.Event("stream_reset", "warn", "stream DSP state reset", nil, map[string]any{"sessions": len(st.sessions)})
	}
	for _, sess := range st.sessions {
		sess.preDemodFIR = nil
		sess.preDemodDecimator = nil
		sess.preDemodDecimPhase = 0
		sess.stereoResampler = nil
		sess.monoResampler = nil
		sess.wfmAudioLPF = nil
	}
}
