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
	"strings"
	"sync"
	"time"

	"sdr-visual-suite/internal/classifier"
	"sdr-visual-suite/internal/demod"
	"sdr-visual-suite/internal/detector"
	"sdr-visual-suite/internal/dsp"
)

// ---------------------------------------------------------------------------
// streamSession — one open demod session for one signal
// ---------------------------------------------------------------------------

type streamSession struct {
	signalID  int64
	centerHz  float64
	bwHz      float64
	snrDb     float64
	peakDb    float64
	class     *classifier.Classification
	startTime time.Time
	lastFeed  time.Time

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
	overlapIQ []complex64

	// De-emphasis IIR state (persists across frames)
	deemphL float64
	deemphR float64

	// Stereo decode: phase-continuous 38kHz oscillator
	stereoPhase float64

	// Polyphase resampler (replaces integer-decimate hack)
	monoResampler   *dsp.Resampler
	stereoResampler *dsp.StereoResampler

	// AQ-4: Stateful FIR filters for click-free stereo decode
	stereoLPF    *dsp.StatefulFIRReal // 15kHz lowpass for L+R
	stereoBPHi   *dsp.StatefulFIRReal // 53kHz LP for bandpass high
	stereoBPLo   *dsp.StatefulFIRReal // 23kHz LP for bandpass low
	stereoLRLPF  *dsp.StatefulFIRReal // 15kHz LP for demodulated L-R
	stereoAALPF  *dsp.StatefulFIRReal // Anti-alias LP for pre-decim (mono path)

	// Stateful pre-demod anti-alias FIR (eliminates cold-start transients
	// and avoids per-frame FIR recomputation)
	preDemodFIR     *dsp.StatefulFIRComplex
	preDemodDecim   int     // cached decimation factor
	preDemodRate    int     // cached snipRate this FIR was built for
	preDemodCutoff  float64 // cached cutoff

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

// AudioInfo describes the audio format of a live-listen subscription.
// Sent to the WebSocket client as the first message.
type AudioInfo struct {
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
	Format     string `json:"format"` // always "s16le"
	DemodName  string `json:"demod"`
}

const (
	streamAudioRate = 48000
	resamplerTaps   = 32 // taps per polyphase arm — good quality
)

// ---------------------------------------------------------------------------
// Streamer — manages all active streaming sessions
// ---------------------------------------------------------------------------

type streamFeedItem struct {
	signal   detector.Signal
	snippet  []complex64
	snipRate int
}

type streamFeedMsg struct {
	items []streamFeedItem
}

type Streamer struct {
	mu       sync.Mutex
	sessions map[int64]*streamSession
	policy   Policy
	centerHz float64
	nextSub  int64
	feedCh   chan streamFeedMsg
	done     chan struct{}

	// pendingListens are subscribers waiting for a matching session.
	pendingListens map[int64]*pendingListen
}

type pendingListen struct {
	freq float64
	bw   float64
	mode string
	ch   chan []byte
}

func newStreamer(policy Policy, centerHz float64) *Streamer {
	st := &Streamer{
		sessions:       make(map[int64]*streamSession),
		policy:         policy,
		centerHz:       centerHz,
		feedCh:         make(chan streamFeedMsg, 2),
		done:           make(chan struct{}),
		pendingListens: make(map[int64]*pendingListen),
	}
	go st.worker()
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
func (st *Streamer) FeedSnippets(items []streamFeedItem) {
	st.mu.Lock()
	recEnabled := st.policy.Enabled && (st.policy.RecordAudio || st.policy.RecordIQ)
	hasListeners := st.hasListenersLocked()
	st.mu.Unlock()

	if (!recEnabled && !hasListeners) || len(items) == 0 {
		return
	}

	select {
	case st.feedCh <- streamFeedMsg{items: items}:
	default:
	}
}

// processFeed runs in the worker goroutine.
func (st *Streamer) processFeed(msg streamFeedMsg) {
	st.mu.Lock()
	defer st.mu.Unlock()

	recEnabled := st.policy.Enabled && (st.policy.RecordAudio || st.policy.RecordIQ)
	hasListeners := st.hasListenersLocked()

	if !recEnabled && !hasListeners {
		return
	}

	now := time.Now()
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

		if !needsRecording && !needsListen {
			continue
		}

		sess, exists := st.sessions[sig.ID]
		if !exists {
			if needsRecording {
				s, err := st.openRecordingSession(sig, now)
				if err != nil {
					log.Printf("STREAM: open failed signal=%d %.1fMHz: %v",
						sig.ID, sig.CenterHz/1e6, err)
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
		audio, audioRate := sess.processSnippet(item.snippet, item.snipRate)
		if len(audio) > 0 {
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
			delete(st.sessions, id)
		}
	}
}

func (st *Streamer) signalHasListenerLocked(sig *detector.Signal) bool {
	if sess, ok := st.sessions[sig.ID]; ok && len(sess.audioSubs) > 0 {
		return true
	}
	for _, pl := range st.pendingListens {
		if math.Abs(sig.CenterHz-pl.freq) < 200000 {
			return true
		}
	}
	return false
}

func (st *Streamer) attachPendingListeners(sess *streamSession) {
	for subID, pl := range st.pendingListens {
		if math.Abs(sess.centerHz-pl.freq) < 200000 {
			sess.audioSubs = append(sess.audioSubs, audioSub{id: subID, ch: pl.ch})
			delete(st.pendingListens, subID)

			// Send updated audio_info now that we know the real session params.
			// Prefix with 0x00 tag byte so ws/audio handler sends as TextMessage.
			infoJSON, _ := json.Marshal(AudioInfo{
				SampleRate: sess.sampleRate,
				Channels:   sess.channels,
				Format:     "s16le",
				DemodName:  sess.demodName,
			})
			tagged := make([]byte, 1+len(infoJSON))
			tagged[0] = 0x00 // tag: audio_info
			copy(tagged[1:], infoJSON)
			select {
			case pl.ch <- tagged:
			default:
			}

			log.Printf("STREAM: attached pending listener %d to signal %d (%.1fMHz %s ch=%d)",
				subID, sess.signalID, sess.centerHz/1e6, sess.demodName, sess.channels)
		}
	}
}

// CloseAll finalises all sessions and stops the worker goroutine.
func (st *Streamer) CloseAll() {
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
		delete(st.sessions, id)
	}
	for _, pl := range st.pendingListens {
		close(pl.ch)
	}
	st.pendingListens = nil
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

	// Try to find a matching session
	var bestSess *streamSession
	bestDist := math.MaxFloat64
	for _, sess := range st.sessions {
		d := math.Abs(sess.centerHz - freq)
		if d < bestDist {
			bestDist = d
			bestSess = sess
		}
	}

	if bestSess != nil && bestDist < 200000 {
		bestSess.audioSubs = append(bestSess.audioSubs, audioSub{id: subID, ch: ch})
		info := AudioInfo{
			SampleRate: bestSess.sampleRate,
			Channels:   bestSess.channels,
			Format:     "s16le",
			DemodName:  bestSess.demodName,
		}
		log.Printf("STREAM: subscriber %d attached to signal %d (%.1fMHz %s)",
			subID, bestSess.signalID, bestSess.centerHz/1e6, bestSess.demodName)
		return subID, ch, info, nil
	}

	// No matching session yet — add as pending listener
	st.pendingListens[subID] = &pendingListen{
		freq: freq,
		bw:   bw,
		mode: mode,
		ch:   ch,
	}
	info := AudioInfo{
		SampleRate: streamAudioRate,
		Channels:   1,
		Format:     "s16le",
		DemodName:  "NFM",
	}
	log.Printf("STREAM: subscriber %d pending (freq=%.1fMHz)", subID, freq/1e6)
	return subID, ch, info, nil
}

// UnsubscribeAudio removes a live-listen subscriber.
func (st *Streamer) UnsubscribeAudio(subID int64) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if pl, ok := st.pendingListens[subID]; ok {
		close(pl.ch)
		delete(st.pendingListens, subID)
		return
	}

	for _, sess := range st.sessions {
		for i, sub := range sess.audioSubs {
			if sub.id == subID {
				close(sub.ch)
				sess.audioSubs = append(sess.audioSubs[:i], sess.audioSubs[i+1:]...)
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
func (sess *streamSession) processSnippet(snippet []complex64, snipRate int) ([]float32, int) {
	if len(snippet) == 0 || snipRate <= 0 {
		return nil, 0
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

	// --- FM discriminator overlap: prepend 1 sample from previous frame ---
	// The FM discriminator needs iq[i-1] to compute the first output.
	// All FIR filtering is now stateful, so no additional overlap is needed.
	var fullSnip []complex64
	trimSamples := 0
	if len(sess.overlapIQ) == 1 {
		fullSnip = make([]complex64, 1+len(snippet))
		fullSnip[0] = sess.overlapIQ[0]
		copy(fullSnip[1:], snippet)
		trimSamples = 1
	} else {
		fullSnip = snippet
	}

	// Save last sample for next frame's FM discriminator
	if len(snippet) > 0 {
		sess.overlapIQ = []complex64{snippet[len(snippet)-1]}
	}

	// --- Stateful anti-alias FIR + decimation to demod rate ---
	demodRate := d.OutputSampleRate()
	decim1 := int(math.Round(float64(snipRate) / float64(demodRate)))
	if decim1 < 1 {
		decim1 = 1
	}
	actualDemodRate := snipRate / decim1

	var dec []complex64
	if decim1 > 1 {
		cutoff := float64(actualDemodRate) / 2.0 * 0.8

		// Lazy-init or reinit stateful FIR if parameters changed
		if sess.preDemodFIR == nil || sess.preDemodRate != snipRate || sess.preDemodCutoff != cutoff {
			taps := dsp.LowpassFIR(cutoff, snipRate, 101)
			sess.preDemodFIR = dsp.NewStatefulFIRComplex(taps)
			sess.preDemodRate = snipRate
			sess.preDemodCutoff = cutoff
			sess.preDemodDecim = decim1
		}

		filtered := sess.preDemodFIR.ProcessInto(fullSnip, sess.growIQ(len(fullSnip)))
		dec = dsp.Decimate(filtered, decim1)
	} else {
		dec = fullSnip
	}

	// --- FM Demod ---
	audio := d.Demod(dec, actualDemodRate)
	if len(audio) == 0 {
		return nil, 0
	}

	// --- Trim the 1-sample FM discriminator overlap ---
	if trimSamples > 0 {
		audioTrim := trimSamples / decim1
		if audioTrim < 1 {
			audioTrim = 1 // at minimum trim 1 audio sample
		}
		if audioTrim > 0 && audioTrim < len(audio) {
			audio = audio[audioTrim:]
		}
	}

	// --- Stateful stereo decode ---
	channels := 1
	if isWFMStereo {
		channels = 2
		audio = sess.stereoDecodeStateful(audio, actualDemodRate)
	}

	// --- Polyphase resample to exact 48kHz ---
	if actualDemodRate != streamAudioRate {
		if channels > 1 {
			if sess.stereoResampler == nil {
				sess.stereoResampler = dsp.NewStereoResampler(actualDemodRate, streamAudioRate, resamplerTaps)
			}
			audio = sess.stereoResampler.Process(audio)
		} else {
			if sess.monoResampler == nil {
				sess.monoResampler = dsp.NewResampler(actualDemodRate, streamAudioRate, resamplerTaps)
			}
			audio = sess.monoResampler.Process(audio)
		}
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

	return audio, streamAudioRate
}

// stereoDecodeStateful: phase-continuous 38kHz oscillator for L-R extraction.
// AQ-4: Uses persistent FIR filter state across frames for click-free stereo.
// Reuses session scratch buffers to minimize allocations.
func (sess *streamSession) stereoDecodeStateful(mono []float32, sampleRate int) []float32 {
	if len(mono) == 0 || sampleRate <= 0 {
		return nil
	}
	n := len(mono)

	// Lazy-init stateful filters on first call
	if sess.stereoLPF == nil {
		lp := dsp.LowpassFIR(15000, sampleRate, 101)
		sess.stereoLPF = dsp.NewStatefulFIRReal(lp)
		sess.stereoBPHi = dsp.NewStatefulFIRReal(dsp.LowpassFIR(53000, sampleRate, 101))
		sess.stereoBPLo = dsp.NewStatefulFIRReal(dsp.LowpassFIR(23000, sampleRate, 101))
		sess.stereoLRLPF = dsp.NewStatefulFIRReal(lp)
	}

	// Reuse scratch for intermediates: need 4*n float32 for bpf, lr, hi, lo
	// plus 2*n for output. We'll use scratchAudio for bpf+lr (2*n) and
	// allocate hi/lo from the stateful FIR ProcessInto.
	scratch := sess.growAudio(n * 4)
	bpf := scratch[:n]
	lr := scratch[n : 2*n]
	hiBuf := scratch[2*n : 3*n]
	loBuf := scratch[3*n : 4*n]

	lpr := sess.stereoLPF.Process(mono) // allocates — but could use ProcessInto too

	sess.stereoBPHi.ProcessInto(mono, hiBuf)
	sess.stereoBPLo.ProcessInto(mono, loBuf)
	for i := 0; i < n; i++ {
		bpf[i] = hiBuf[i] - loBuf[i]
	}

	phase := sess.stereoPhase
	inc := 2 * math.Pi * 38000 / float64(sampleRate)
	for i := range bpf {
		phase += inc
		lr[i] = bpf[i] * float32(2*math.Cos(phase))
	}
	sess.stereoPhase = math.Mod(phase, 2*math.Pi)

	lr = sess.stereoLRLPF.Process(lr)

	out := make([]float32, n*2)
	for i := 0; i < n; i++ {
		out[i*2] = 0.5 * (lpr[i] + lr[i])
		out[i*2+1] = 0.5 * (lpr[i] - lr[i])
	}
	return out
}

// dspStateSnapshot captures persistent DSP state for segment splits.
type dspStateSnapshot struct {
	overlapIQ       []complex64
	deemphL         float64
	deemphR         float64
	stereoPhase     float64
	monoResampler   *dsp.Resampler
	stereoResampler *dsp.StereoResampler
	stereoLPF       *dsp.StatefulFIRReal
	stereoBPHi      *dsp.StatefulFIRReal
	stereoBPLo      *dsp.StatefulFIRReal
	stereoLRLPF     *dsp.StatefulFIRReal
	stereoAALPF     *dsp.StatefulFIRReal
	preDemodFIR     *dsp.StatefulFIRComplex
	preDemodDecim   int
	preDemodRate    int
	preDemodCutoff  float64
}

func (sess *streamSession) captureDSPState() dspStateSnapshot {
	return dspStateSnapshot{
		overlapIQ:       sess.overlapIQ,
		deemphL:         sess.deemphL,
		deemphR:         sess.deemphR,
		stereoPhase:     sess.stereoPhase,
		monoResampler:   sess.monoResampler,
		stereoResampler: sess.stereoResampler,
		stereoLPF:       sess.stereoLPF,
		stereoBPHi:      sess.stereoBPHi,
		stereoBPLo:      sess.stereoBPLo,
		stereoLRLPF:     sess.stereoLRLPF,
		stereoAALPF:     sess.stereoAALPF,
		preDemodFIR:     sess.preDemodFIR,
		preDemodDecim:   sess.preDemodDecim,
		preDemodRate:    sess.preDemodRate,
		preDemodCutoff:  sess.preDemodCutoff,
	}
}

func (sess *streamSession) restoreDSPState(s dspStateSnapshot) {
	sess.overlapIQ = s.overlapIQ
	sess.deemphL = s.deemphL
	sess.deemphR = s.deemphR
	sess.stereoPhase = s.stereoPhase
	sess.monoResampler = s.monoResampler
	sess.stereoResampler = s.stereoResampler
	sess.stereoLPF = s.stereoLPF
	sess.stereoBPHi = s.stereoBPHi
	sess.stereoBPLo = s.stereoBPLo
	sess.stereoLRLPF = s.stereoLRLPF
	sess.stereoAALPF = s.stereoAALPF
	sess.preDemodFIR = s.preDemodFIR
	sess.preDemodDecim = s.preDemodDecim
	sess.preDemodRate = s.preDemodRate
	sess.preDemodCutoff = s.preDemodCutoff
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

	sess := &streamSession{
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
		deemphasisUs: st.policy.DeemphasisUs,
	}

	log.Printf("STREAM: opened recording signal=%d %.1fMHz %s dir=%s",
		sig.ID, sig.CenterHz/1e6, demodName, dirName)
	return sess, nil
}

func (st *Streamer) openListenSession(sig *detector.Signal, now time.Time) *streamSession {
	demodName, channels := resolveDemod(sig)

	sess := &streamSession{
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
		default:
		}
		alive = append(alive, sub)
	}
	sess.audioSubs = alive
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
