package recorder

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
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
// streamSession — one open recording for one signal
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

	dir        string
	wavFile    *os.File
	wavBuf     *bufio.Writer
	wavSamples int64
	sampleRate int // actual output audio sample rate
	channels   int
	demodName  string
	segmentIdx int

	// --- Persistent DSP state for click-free streaming ---

	// Overlap-save: tail of previous extracted IQ snippet.
	// Prepended to the next snippet so FIR filters and FM discriminator
	// have history — eliminates transient clicks at frame boundaries.
	overlapIQ []complex64

	// De-emphasis IIR state (persists across frames)
	deemphL float64
	deemphR float64

	// Stereo decode: phase-continuous 38kHz oscillator
	stereoPhase float64

	// live-listen subscribers
	audioSubs []audioSub
}

type audioSub struct {
	id int64
	ch chan []byte
}

const (
	streamAudioRate = 48000
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
}

func newStreamer(policy Policy, centerHz float64) *Streamer {
	st := &Streamer{
		sessions: make(map[int64]*streamSession),
		policy:   policy,
		centerHz: centerHz,
		feedCh:   make(chan streamFeedMsg, 2),
		done:     make(chan struct{}),
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

	// If recording was just disabled, close all active sessions
	// so WAV headers get fixed and meta.json gets written.
	if wasEnabled && !policy.Enabled {
		for id, sess := range st.sessions {
			for _, sub := range sess.audioSubs {
				close(sub.ch)
			}
			sess.audioSubs = nil
			closeSession(sess, &st.policy)
			delete(st.sessions, id)
		}
		log.Printf("STREAM: recording disabled — closed %d sessions", len(st.sessions))
	}
}

// FeedSnippets is called from the DSP loop with pre-extracted IQ snippets
// (GPU-accelerated FreqShift+FIR+Decimate already done). It copies the snippets
// and enqueues them for async demod in the worker goroutine.
func (st *Streamer) FeedSnippets(items []streamFeedItem) {
	st.mu.Lock()
	enabled := st.policy.Enabled && (st.policy.RecordAudio || st.policy.RecordIQ)
	st.mu.Unlock()
	if !enabled || len(items) == 0 {
		return
	}

	// Copy snippets (GPU buffers may be reused)
	copied := make([]streamFeedItem, len(items))
	for i, item := range items {
		snipCopy := make([]complex64, len(item.snippet))
		copy(snipCopy, item.snippet)
		copied[i] = streamFeedItem{
			signal:   item.signal,
			snippet:  snipCopy,
			snipRate: item.snipRate,
		}
	}

	select {
	case st.feedCh <- streamFeedMsg{items: copied}:
	default:
		// Worker busy — drop frame rather than blocking DSP loop
	}
}

// processFeed runs in the worker goroutine. Receives pre-extracted snippets
// and does the lightweight demod + stereo + de-emphasis with persistent state.
func (st *Streamer) processFeed(msg streamFeedMsg) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if !st.policy.Enabled || (!st.policy.RecordAudio && !st.policy.RecordIQ) {
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
		if sig.SNRDb < st.policy.MinSNRDb {
			continue
		}
		if !st.classAllowed(sig.Class) {
			continue
		}
		if len(item.snippet) == 0 || item.snipRate <= 0 {
			continue
		}

		sess, exists := st.sessions[sig.ID]
		if !exists {
			s, err := st.openSession(sig, now)
			if err != nil {
				log.Printf("STREAM: open failed signal=%d %.1fMHz: %v",
					sig.ID, sig.CenterHz/1e6, err)
				continue
			}
			st.sessions[sig.ID] = s
			sess = s
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

		// Demod with persistent state (overlap-save, stereo, de-emphasis)
		audio, audioRate := sess.processSnippet(item.snippet, item.snipRate)
		if len(audio) > 0 {
			if sess.wavSamples == 0 && audioRate > 0 {
				sess.sampleRate = audioRate
			}
			appendAudio(sess, audio)
			st.fanoutAudio(sess, audio)
		}

		// Segment split
		if st.policy.MaxDuration > 0 && now.Sub(sess.startTime) >= st.policy.MaxDuration {
			segIdx := sess.segmentIdx + 1
			oldSubs := sess.audioSubs
			oldOverlap := sess.overlapIQ
			oldDeemphL := sess.deemphL
			oldDeemphR := sess.deemphR
			oldStereo := sess.stereoPhase
			sess.audioSubs = nil
			closeSession(sess, &st.policy)
			s, err := st.openSession(sig, now)
			if err != nil {
				delete(st.sessions, sig.ID)
				continue
			}
			s.segmentIdx = segIdx
			s.audioSubs = oldSubs
			s.overlapIQ = oldOverlap
			s.deemphL = oldDeemphL
			s.deemphR = oldDeemphR
			s.stereoPhase = oldStereo
			st.sessions[sig.ID] = s
		}
	}

	// Close sessions for disappeared signals (with grace period)
	for id, sess := range st.sessions {
		if seen[id] {
			continue
		}
		if now.Sub(sess.lastFeed) > 3*time.Second {
			closeSession(sess, &st.policy)
			delete(st.sessions, id)
		}
	}
}

// CloseAll finalises all sessions and stops the worker goroutine.
func (st *Streamer) CloseAll() {
	// Stop accepting new feeds and wait for worker to finish
	close(st.feedCh)
	<-st.done

	st.mu.Lock()
	defer st.mu.Unlock()
	for id, sess := range st.sessions {
		for _, sub := range sess.audioSubs {
			close(sub.ch)
		}
		sess.audioSubs = nil
		closeSession(sess, &st.policy)
		delete(st.sessions, id)
	}
}

// ActiveSessions returns the number of open streaming sessions.
func (st *Streamer) ActiveSessions() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.sessions)
}

// SubscribeAudio registers a live-listen subscriber for a given frequency.
func (st *Streamer) SubscribeAudio(freq float64, bw float64, mode string) (int64, <-chan []byte) {
	ch := make(chan []byte, 64)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.nextSub++
	subID := st.nextSub

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
	} else {
		log.Printf("STREAM: audio subscriber %d has no matching session (freq=%.1fMHz)", subID, freq/1e6)
		close(ch)
	}
	return subID, ch
}

// UnsubscribeAudio removes a live-listen subscriber.
func (st *Streamer) UnsubscribeAudio(subID int64) {
	st.mu.Lock()
	defer st.mu.Unlock()
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

// processSnippet takes a pre-extracted IQ snippet (from GPU or CPU
// extractSignalIQBatch) and demodulates it with persistent state.
//
// The overlap-save operates on the EXTRACTED snippet level: we prepend
// the tail of the previous snippet so that:
//   - FM discriminator has iq[i-1] for the first sample
//   - The ~50-sample transient from FreqShift phase reset and FIR startup
//     falls into the overlap region and gets trimmed from the output
//
// Stateful components (across frames):
//   - overlapIQ: tail of previous extracted snippet
//   - stereoPhase: 38kHz oscillator for L-R decode
//   - deemphL/R: de-emphasis IIR accumulators
func (sess *streamSession) processSnippet(snippet []complex64, snipRate int) ([]float32, int) {
	if len(snippet) == 0 || snipRate <= 0 {
		return nil, 0
	}

	isWFMStereo := sess.demodName == "WFM_STEREO"
	isWFM := sess.demodName == "WFM" || isWFMStereo

	demodName := sess.demodName
	if isWFMStereo {
		demodName = "WFM" // mono FM demod, then stateful stereo post-process
	}
	d := demod.Get(demodName)
	if d == nil {
		d = demod.Get("NFM")
	}
	if d == nil {
		return nil, 0
	}

	// --- Minimal overlap: prepend last sample from previous snippet ---
	// The FM discriminator computes atan2(iq[i] * conj(iq[i-1])), so the
	// first output sample needs iq[-1] from the previous frame.
	// FIR halo is already handled by extractForStreaming's IQ-level overlap,
	// so we only need 1 sample here.
	var fullSnip []complex64
	trimSamples := 0
	if len(sess.overlapIQ) > 0 {
		fullSnip = make([]complex64, len(sess.overlapIQ)+len(snippet))
		copy(fullSnip, sess.overlapIQ)
		copy(fullSnip[len(sess.overlapIQ):], snippet)
		trimSamples = len(sess.overlapIQ)
	} else {
		fullSnip = snippet
	}

	// Save last sample for next frame's FM discriminator
	if len(snippet) > 0 {
		sess.overlapIQ = []complex64{snippet[len(snippet)-1]}
	}

	// --- Decimate to demod-preferred rate with anti-alias ---
	demodRate := d.OutputSampleRate()
	decim1 := int(math.Round(float64(snipRate) / float64(demodRate)))
	if decim1 < 1 {
		decim1 = 1
	}
	actualDemodRate := snipRate / decim1

	var dec []complex64
	if decim1 > 1 {
		cutoff := float64(actualDemodRate) / 2.0 * 0.8
		aaTaps := dsp.LowpassFIR(cutoff, snipRate, 101)
		filtered := dsp.ApplyFIR(fullSnip, aaTaps)
		dec = dsp.Decimate(filtered, decim1)
	} else {
		dec = fullSnip
	}

	// --- FM Demod ---
	audio := d.Demod(dec, actualDemodRate)
	if len(audio) == 0 {
		return nil, 0
	}

	// --- Trim the overlap sample(s) from audio ---
	audioTrim := trimSamples / decim1
	if decim1 <= 1 {
		audioTrim = trimSamples
	}
	if audioTrim > 0 && audioTrim < len(audio) {
		audio = audio[audioTrim:]
	}

	// --- Stateful stereo decode ---
	channels := 1
	if isWFMStereo {
		channels = 2
		audio = sess.stereoDecodeStateful(audio, actualDemodRate)
	}

	// --- Resample towards 48kHz ---
	outputRate := actualDemodRate
	if actualDemodRate > streamAudioRate {
		decim2 := actualDemodRate / streamAudioRate
		if decim2 < 1 {
			decim2 = 1
		}
		outputRate = actualDemodRate / decim2

		aaTaps := dsp.LowpassFIR(float64(outputRate)/2.0*0.9, actualDemodRate, 63)

		if channels > 1 {
			nFrames := len(audio) / channels
			left := make([]float32, nFrames)
			right := make([]float32, nFrames)
			for i := 0; i < nFrames; i++ {
				left[i] = audio[i*2]
				if i*2+1 < len(audio) {
					right[i] = audio[i*2+1]
				}
			}
			left = dsp.ApplyFIRReal(left, aaTaps)
			right = dsp.ApplyFIRReal(right, aaTaps)
			outFrames := nFrames / decim2
			if outFrames < 1 {
				return nil, 0
			}
			resampled := make([]float32, outFrames*2)
			for i := 0; i < outFrames; i++ {
				resampled[i*2] = left[i*decim2]
				resampled[i*2+1] = right[i*decim2]
			}
			audio = resampled
		} else {
			audio = dsp.ApplyFIRReal(audio, aaTaps)
			resampled := make([]float32, 0, len(audio)/decim2+1)
			for i := 0; i < len(audio); i += decim2 {
				resampled = append(resampled, audio[i])
			}
			audio = resampled
		}
	}

	// --- De-emphasis (50µs Europe) ---
	if isWFM && outputRate > 0 {
		const tau = 50e-6
		alpha := math.Exp(-1.0 / (float64(outputRate) * tau))
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

	return audio, outputRate
}

// stereoDecodeStateful: phase-continuous 38kHz oscillator for L-R extraction.
func (sess *streamSession) stereoDecodeStateful(mono []float32, sampleRate int) []float32 {
	if len(mono) == 0 || sampleRate <= 0 {
		return nil
	}

	lp := dsp.LowpassFIR(15000, sampleRate, 101)
	lpr := dsp.ApplyFIRReal(mono, lp)

	bpHi := dsp.LowpassFIR(53000, sampleRate, 101)
	bpLo := dsp.LowpassFIR(23000, sampleRate, 101)
	hi := dsp.ApplyFIRReal(mono, bpHi)
	lo := dsp.ApplyFIRReal(mono, bpLo)
	bpf := make([]float32, len(mono))
	for i := range mono {
		bpf[i] = hi[i] - lo[i]
	}

	lr := make([]float32, len(mono))
	phase := sess.stereoPhase
	inc := 2 * math.Pi * 38000 / float64(sampleRate)
	for i := range bpf {
		phase += inc
		lr[i] = bpf[i] * float32(2*math.Cos(phase))
	}
	sess.stereoPhase = math.Mod(phase, 2*math.Pi)

	lr = dsp.ApplyFIRReal(lr, lp)

	out := make([]float32, len(lpr)*2)
	for i := range lpr {
		out[i*2] = 0.5 * (lpr[i] + lr[i])
		out[i*2+1] = 0.5 * (lpr[i] - lr[i])
	}
	return out
}

// ---------------------------------------------------------------------------
// Session management helpers
// ---------------------------------------------------------------------------

func (st *Streamer) openSession(sig *detector.Signal, now time.Time) (*streamSession, error) {
	outputDir := st.policy.OutputDir
	if outputDir == "" {
		outputDir = "data/recordings"
	}

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
		signalID:   sig.ID,
		centerHz:   sig.CenterHz,
		bwHz:       sig.BWHz,
		snrDb:      sig.SNRDb,
		peakDb:     sig.PeakDb,
		class:      sig.Class,
		startTime:  now,
		lastFeed:   now,
		dir:        dir,
		wavFile:    f,
		wavBuf:     bufio.NewWriterSize(f, 64*1024),
		sampleRate: streamAudioRate,
		channels:   channels,
		demodName:  demodName,
	}

	log.Printf("STREAM: opened signal=%d %.1fMHz %s dir=%s",
		sig.ID, sig.CenterHz/1e6, demodName, dirName)
	return sess, nil
}

func closeSession(sess *streamSession, policy *Policy) {
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

func appendAudio(sess *streamSession, audio []float32) {
	if sess.wavBuf == nil || len(audio) == 0 {
		return
	}
	buf := make([]byte, len(audio)*2)
	for i, s := range audio {
		v := int16(clip(s * 32767))
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	n, err := sess.wavBuf.Write(buf)
	if err != nil {
		log.Printf("STREAM: write error signal=%d: %v", sess.signalID, err)
		return
	}
	sess.wavSamples += int64(n / 2)
}

func (st *Streamer) fanoutAudio(sess *streamSession, audio []float32) {
	if len(sess.audioSubs) == 0 {
		return
	}
	pcm := make([]byte, len(audio)*2)
	for i, s := range audio {
		v := int16(clip(s * 32767))
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(v))
	}
	alive := sess.audioSubs[:0]
	for _, sub := range sess.audioSubs {
		select {
		case sub.ch <- pcm:
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
