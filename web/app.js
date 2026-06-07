const qs = (id) => document.getElementById(id);

const navCanvas = qs('navCanvas');
const spectrumCanvas = qs('spectrum');
const waterfallCanvas = qs('waterfall');
const occupancyCanvas = qs('occupancy');
const timelineCanvas = qs('timeline');
const detailSpectrogram = qs('detailSpectrogram');
const signalPopover = qs('signalPopover');

const wsBadge = qs('wsBadge');
const metaLine = qs('metaLine');
const heroSubtitle = qs('heroSubtitle');
const configStatusEl = qs('configStatus');
const timelineRangeEl = qs('timelineRange');

const metricCenter = qs('metricCenter');
const metricSpan = qs('metricSpan');
const metricRes = qs('metricRes');
const metricSignals = qs('metricSignals');
const metricGpu = qs('metricGpu');
const metricSource = qs('metricSource');

const centerInput = qs('centerInput');
const spanInput = qs('spanInput');
const sampleRateSelect = qs('sampleRateSelect');
const bwSelect = qs('bwSelect');
const fftSelect = qs('fftSelect');
const gainRange = qs('gainRange');
const gainInput = qs('gainInput');
const thresholdRange = qs('thresholdRange');
const thresholdInput = qs('thresholdInput');
const cfarModeSelect = qs('cfarModeSelect');
const cfarWrapToggle = qs('cfarWrapToggle');
const cfarGuardHzInput = qs('cfarGuardHzInput');
const cfarTrainHzInput = qs('cfarTrainHzInput');
const cfarRankInput = qs('cfarRankInput');
const classifierModeSelect = qs('classifierModeSelect');
const edgeMarginInput = qs('edgeMarginInput');
const mergeGapInput = qs('mergeGapInput');
const classHistoryInput = qs('classHistoryInput');
const classSwitchInput = qs('classSwitchInput');
const cfarScaleInput = qs('cfarScaleInput');
const minDurationInput = qs('minDurationInput');
const holdInput = qs('holdInput');
const emaAlphaInput = qs('emaAlphaInput');
const hysteresisInput = qs('hysteresisInput');
const stableFramesInput = qs('stableFramesInput');
const gapToleranceInput = qs('gapToleranceInput');
const agcToggle = qs('agcToggle');
const dcToggle = qs('dcToggle');
const iqToggle = qs('iqToggle');
const avgSelect = qs('avgSelect');
const welchSegInput = qs('welchSegInput');
const maxHoldToggle = qs('maxHoldToggle');
const gpuToggle = qs('gpuToggle');
const recEnableToggle = qs('recEnableToggle');
const recIQToggle = qs('recIQToggle');
const recAudioToggle = qs('recAudioToggle');
const recDemodToggle = qs('recDemodToggle');
const recDecodeToggle = qs('recDecodeToggle');
const recMinSNR = qs('recMinSNR');
const recMaxDisk = qs('recMaxDisk');
const recClassFilter = qs('recClassFilter');

const refineAutoSpan = qs('refineAutoSpan');
const refineMinSpan = qs('refineMinSpan');
const refineMaxSpan = qs('refineMaxSpan');

const resMaxRefine = qs('resMaxRefine');
const resMaxRecord = qs('resMaxRecord');
const resMaxDecode = qs('resMaxDecode');
const resDecisionHold = qs('resDecisionHold');

const signalList = qs('signalList');
const signalDecisionSummary = qs('signalDecisionSummary');
const signalQueueSummary = qs('signalQueueSummary');
const eventList = qs('eventList');
const recordingList = qs('recordingList');
const signalCountBadge = qs('signalCountBadge');
const eventCountBadge = qs('eventCountBadge');
const recordingCountBadge = qs('recordingCountBadge');
const signalSummaryLine = qs('signalSummaryLine');

const healthBuffer = qs('healthBuffer');
const healthDropped = qs('healthDropped');
const healthResets = qs('healthResets');
const healthAge = qs('healthAge');
const healthGpu = qs('healthGpu');
const healthFps = qs('healthFps');
const healthRefinePlan = qs('healthRefinePlan');
const healthRefineWindows = qs('healthRefineWindows');
const healthWs = qs('healthWs');
const healthApi = qs('healthApi');
const healthConfig = qs('healthConfig');
const healthRefine = qs('healthRefine');
const healthTelemetry = qs('healthTelemetry');
const healthSource = qs('healthSource');
const refineDetails = qs('refineDetails');
const telemetryEventList = qs('telemetryEventList');
const policySummaryList = qs('policySummaryList');
const policyRecommendationList = qs('policyRecommendationList');

const drawerEl = qs('eventDrawer');
const drawerCloseBtn = qs('drawerClose');
const detailSubtitle = qs('detailSubtitle');
const detailCenterEl = qs('detailCenter');
const detailBwEl = qs('detailBw');
const detailStartEl = qs('detailStart');
const detailEndEl = qs('detailEnd');
const detailSnrEl = qs('detailSnr');
const detailDurEl = qs('detailDur');
const detailClassEl = qs('detailClass');
const jumpToEventBtn = qs('jumpToEventBtn');
const exportEventBtn = qs('exportEventBtn');
const liveListenEventBtn = qs('liveListenEventBtn');
const decodeEventBtn = qs('decodeEventBtn');
const decodeModeSelect = qs('decodeMode');
const recordingMetaEl = qs('recordingMeta');
const decodeResultEl = qs('decodeResult');
const classifierScoresEl = qs('classifierScores');
const classifierScoreBarsEl = qs('classifierScoreBars');
const recordingMetaLink = qs('recordingMetaLink');
const recordingIQLink = qs('recordingIQLink');
const recordingAudioLink = qs('recordingAudioLink');

const followBtn = qs('followBtn');
const fitBtn = qs('fitBtn');
const resetMaxBtn = qs('resetMaxBtn');
const debugOverlayToggle = qs('debugOverlayToggle');
const timelineFollowBtn = qs('timelineFollowBtn');
const timelineFreezeBtn = qs('timelineFreezeBtn');

const modeButtons = Array.from(document.querySelectorAll('.mode-btn'));
const railTabs = Array.from(document.querySelectorAll('.rail-tab'));
const tabPanels = Array.from(document.querySelectorAll('.tab-panel'));
const presetButtons = Array.from(document.querySelectorAll('.preset-btn'));
const liveListenBtn = qs('liveListenBtn');
const listenSecondsInput = qs('listenSeconds');
const listenModeSelect = qs('listenMode');
const listenMetaDemod = qs('listenMetaDemod');
const listenMetaPlayback = qs('listenMetaPlayback');
const listenMetaStereo = qs('listenMetaStereo');
const listenMetaStatus = qs('listenMetaStatus');
const listenMetaAudio = qs('listenMetaAudio');

let latest = null;
let currentConfig = null;
let liveAudio = null;
let liveListenWS = null; // WebSocket-based live listen
let spectrumWS = null;
let liveListenTarget = null; // { freq, bw, mode }
let liveListenInfo = null;
let stats = { buffer_samples: 0, dropped: 0, resets: 0, last_sample_ago_ms: -1 };
let refinementInfo = {};
let decisionIndex = new Map();
let telemetryLive = null;
let policyInfo = null;
let policyRecommendations = null;
let wsState = 'init';
let wsLastMessageTs = 0;
let wsCarriesSignals = false;
let wsCarriesDebug = false;
let apiState = { ok: false, latencyMs: null, lastOkTs: 0, lastError: '' };
const apiClient = window.SpectreApi?.createClient
  ? window.SpectreApi.createClient({ timeoutMs: 4500 })
  : null;
const operatorPanel = window.OperatorPanel?.create
  ? window.OperatorPanel.create({
    healthWs,
    healthApi,
    healthConfig,
    healthRefine,
    healthTelemetry,
    healthSource,
    refineDetails,
    telemetryEvents: telemetryEventList
  })
  : null;

// ---------------------------------------------------------------------------
// LiveListenWS — WebSocket-based gapless audio streaming via /ws/audio
// ---------------------------------------------------------------------------
// v5: AudioWorklet-first playback.
//
// - Audio is pushed into an AudioWorklet ring buffer when available, so
//   canvas/DOM jank on the main thread no longer directly starves playback.
// - Fallback remains scheduled BufferSource playback for environments where
//   AudioWorklet is unavailable.
// ---------------------------------------------------------------------------
class LiveListenWS {
  constructor(freq, bw, mode) {
    this.freq = freq;
    this.bw = bw;
    this.mode = mode;
    this.ws = null;
    this.audioCtx = null;
    this.sampleRate = 48000;
    this.channels = 1;
    this.playing = false;
    this.nextTime = 0;
    this.started = false;
    this._onStop = null;
    this._audioInitPromise = null;
    this._workletNode = null;
    this._workletReady = false;
    this._useWorklet = false;
    this._workletStats = { underruns: 0, overruns: 0, availableFrames: 0, lastTs: 0 };
    this._pendingWorkletChunks = [];
    this._pendingWorkletSamples = 0;
    // Fallback chunk coalescing buffer
    this._pendingSamples = [];
    this._pendingLen = 0;
    this._flushTimer = 0;
  }

  start() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${location.host}/ws/audio?freq=${this.freq}&bw=${this.bw}&mode=${this.mode || ''}`;
    this.ws = new WebSocket(url);
    this.ws.binaryType = 'arraybuffer';
    this.playing = true;

    this.ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') {
        try {
          const info = JSON.parse(ev.data);
          handleLiveListenAudioInfo(info);
          const hasRate = Number.isFinite(info.sample_rate) && info.sample_rate > 0;
          const hasCh = Number.isFinite(info.channels) && info.channels > 0;
          if (hasRate || hasCh) {
            const newRate = hasRate ? info.sample_rate : this.sampleRate;
            const newCh = hasCh ? info.channels : this.channels;
            if (newRate !== this.sampleRate || newCh !== this.channels) {
              this.sampleRate = newRate;
              this.channels = newCh;
              this._teardownAudio();
            }
            this._initAudio();
          }
        } catch (e) { /* ignore */ }
        return;
      }
      if (!this.playing) return;
      this._onPCM(ev.data);
    };

    this.ws.onclose = () => {
      this.playing = false;
      if (this._onStop) this._onStop();
    };
    this.ws.onerror = () => {
      this.playing = false;
      if (this._onStop) this._onStop();
    };

    setTimeout(() => {
      if (!this.audioCtx && this.playing) this._initAudio();
    }, 500);
  }

  stop() {
    this.playing = false;
    if (this.ws) { this.ws.close(); this.ws = null; }
    this._teardownAudio();
  }

  onStop(fn) { this._onStop = fn; }

  _teardownAudio() {
    if (this._flushTimer) { clearTimeout(this._flushTimer); this._flushTimer = 0; }
    if (this.audioCtx) { this.audioCtx.close().catch(() => {}); this.audioCtx = null; }
    this._audioInitPromise = null;
    this._workletNode = null;
    this._workletReady = false;
    this._useWorklet = false;
    this._pendingWorkletChunks = [];
    this._pendingWorkletSamples = 0;
    this.nextTime = 0;
    this.started = false;
    this._pendingSamples = [];
    this._pendingLen = 0;
  }

  _canUseWorklet() {
    return !!(window.AudioWorkletNode && window.AudioContext && (window.isSecureContext || location.hostname === 'localhost' || location.hostname === '127.0.0.1'));
  }

  _initAudio() {
    if (this.audioCtx) return this._audioInitPromise || Promise.resolve();
    this.audioCtx = new (window.AudioContext || window.webkitAudioContext)({
      sampleRate: this.sampleRate,
      latencyHint: 'interactive'
    });
    this.audioCtx.resume().catch(() => {});
    this.nextTime = 0;
    this.started = false;
    this._useWorklet = this._canUseWorklet();

    if (!this._useWorklet) {
      this._audioInitPromise = Promise.resolve();
      return this._audioInitPromise;
    }

    this._audioInitPromise = this.audioCtx.audioWorklet.addModule('ring-player-processor.js')
      .then(() => {
        if (!this.audioCtx) return;
        this._workletNode = new AudioWorkletNode(this.audioCtx, 'ring-player-processor', {
          numberOfInputs: 0,
          numberOfOutputs: 1,
          outputChannelCount: [this.channels],
          processorOptions: {
            channels: this.channels,
            startThresholdSeconds: 0.22,
            ringSeconds: 1.0
          }
        });
        this._workletNode.connect(this.audioCtx.destination);
        this._workletNode.port.onmessage = (ev) => {
          if (!ev?.data || ev.data.type !== 'stats') return;
          this._workletStats = {
            underruns: ev.data.underruns || 0,
            overruns: ev.data.overruns || 0,
            availableFrames: ev.data.availableFrames || 0,
            lastTs: performance.now()
          };
        };
        this._workletReady = true;
        if (this._pendingLen > 0) this._flushPending();
      })
      .catch((err) => {
        console.warn('audio_worklet_init_failed', err);
        this._useWorklet = false;
        if (this._pendingLen > 0) this._flushPending();
      });

    return this._audioInitPromise;
  }

  _onPCM(buf) {
    this._initAudio();
    if (!this.audioCtx) return;

    const chunk = new Int16Array(buf);
    const maxPendingFrames = Math.ceil(this.sampleRate * 0.25);
    const maxPendingSamples = maxPendingFrames * Math.max(1, this.channels);

    if (this._pendingLen + chunk.length > maxPendingSamples) {
      this._pendingSamples = [chunk];
      this._pendingLen = chunk.length;
      if (this._flushTimer) {
        clearTimeout(this._flushTimer);
        this._flushTimer = 0;
      }
    } else {
      this._pendingSamples.push(chunk);
      this._pendingLen += chunk.length;
    }

    const minFrames = Math.ceil(this.sampleRate * 0.04);
    const haveFrames = Math.floor(this._pendingLen / Math.max(1, this.channels));

    if (haveFrames >= minFrames) {
      this._flushPending();
    } else if (!this._flushTimer) {
      this._flushTimer = setTimeout(() => {
        this._flushTimer = 0;
        if (this._pendingLen > 0) this._flushPending();
      }, 40);
    }
  }

  _flushPending() {
    if (this._flushTimer) { clearTimeout(this._flushTimer); this._flushTimer = 0; }
    if (this._pendingSamples.length === 0 || !this.audioCtx) return;
    if (this._useWorklet && !this._workletReady) return;

    const total = this._pendingLen;
    const merged = new Int16Array(total);
    let off = 0;
    for (const chunk of this._pendingSamples) {
      merged.set(chunk, off);
      off += chunk.length;
    }
    this._pendingSamples = [];
    this._pendingLen = 0;

    if (this._useWorklet && this._workletNode) {
      const floatSamples = new Float32Array(merged.length);
      for (let i = 0; i < merged.length; i++) floatSamples[i] = merged[i] / 32768;
      this._workletNode.port.postMessage({ type: 'pcm', samples: floatSamples.buffer }, [floatSamples.buffer]);
      return;
    }

    this._scheduleChunkFallback(merged);
  }

  _scheduleChunkFallback(samples) {
    const ctx = this.audioCtx;
    if (!ctx) return;
    if (ctx.state === 'suspended') ctx.resume().catch(() => {});

    const nFrames = Math.floor(samples.length / this.channels);
    if (nFrames === 0) return;

    const audioBuffer = ctx.createBuffer(this.channels, nFrames, this.sampleRate);
    for (let ch = 0; ch < this.channels; ch++) {
      const data = audioBuffer.getChannelData(ch);
      for (let i = 0; i < nFrames; i++) {
        data[i] = samples[i * this.channels + ch] / 32768;
      }
    }

    const now = ctx.currentTime;
    const targetLatency = 0.4;
    const maxBuffered = 0.9;

    if (!this.started || this.nextTime < now) {
      const fadeIn = Math.min(64, nFrames);
      for (let ch = 0; ch < this.channels; ch++) {
        const data = audioBuffer.getChannelData(ch);
        for (let i = 0; i < fadeIn; i++) data[i] *= i / fadeIn;
      }
      this.nextTime = now + targetLatency;
      this.started = true;
    }

    if (this.nextTime > now + maxBuffered) {
      return;
    }

    const source = ctx.createBufferSource();
    source.buffer = audioBuffer;
    source.connect(ctx.destination);
    source.start(this.nextTime);
    this.nextTime += audioBuffer.duration;
  }
}

const liveListenDefaults = {
  status: 'Idle',
  demod: '-',
  playback_mode: '-',
  stereo_state: '-',
  sample_rate: null,
  channels: null
};

function formatListenMetaValue(value, fallback = '-') {
  if (value === undefined || value === null || value === '') return fallback;
  return String(value);
}

function isListeningSignal(signal) {
  return !!(signal && liveListenTarget && matchesListenTarget(signal));
}

function getSignalPrimaryMode(signal) {
  if (isListeningSignal(signal) && liveListenInfo?.playback_mode && liveListenInfo.playback_mode !== '-') {
    return liveListenInfo.playback_mode;
  }
  if (signal?.playback_mode) return signal.playback_mode;
  if (signal?.demod) return signal.demod;
  if (signal?.class?.mod_type) return signal.class.mod_type;
  return 'carrier';
}

function getSignalRuntimeSummary(signal) {
  const bits = [];
  if (isListeningSignal(signal)) {
    if (liveListenInfo?.stereo_state && liveListenInfo.stereo_state !== '-') bits.push(liveListenInfo.stereo_state);
    if (liveListenInfo?.demod && liveListenInfo.demod !== getSignalPrimaryMode(signal)) bits.push(liveListenInfo.demod);
    if (liveListenInfo?.status && !['Idle', '-', 'Live'].includes(liveListenInfo.status)) bits.push(liveListenInfo.status);
    if (bits.length) return bits.join(' · ');
  }
  if (signal?.stereo_state) bits.push(signal.stereo_state);
  if (signal?.demod && signal.demod !== getSignalPrimaryMode(signal)) bits.push(signal.demod);
  return bits.join(' · ');
}

function getSignalAudioSummary(signal) {
  if (!isListeningSignal(signal)) return '';
  const rate = Number.isFinite(liveListenInfo?.sample_rate) && liveListenInfo.sample_rate > 0 ? fmtHz(liveListenInfo.sample_rate) : '';
  const ch = Number.isFinite(liveListenInfo?.channels) && liveListenInfo.channels > 0 ? `${liveListenInfo.channels} ch` : '';
  return [rate, ch].filter(Boolean).join(' · ');
}

function renderLiveListenMeta(info) {
  if (!listenMetaDemod) return;
  const status = formatListenMetaValue(info?.status, 'Idle');
  const demod = formatListenMetaValue(info?.demod);
  const playback = formatListenMetaValue(info?.playback_mode);
  const stereo = formatListenMetaValue(info?.stereo_state);
  const sampleRate = Number.isFinite(info?.sample_rate) && info.sample_rate > 0
    ? fmtHz(info.sample_rate)
    : '-';
  const channels = Number.isFinite(info?.channels) && info.channels > 0
    ? `${info.channels} ch`
    : '-';

  if (listenMetaStatus) listenMetaStatus.textContent = status;
  listenMetaPlayback.textContent = playback;
  listenMetaStereo.textContent = stereo;
  listenMetaDemod.textContent = `Demod ${demod}`;
  if (listenMetaAudio) listenMetaAudio.textContent = `Audio ${sampleRate}${channels !== '-' ? ` · ${channels}` : ''}`;
}

function resetLiveListenMeta() {
  liveListenInfo = { ...liveListenDefaults };
  renderLiveListenMeta(liveListenInfo);
}

function updateLiveListenMeta(partial) {
  liveListenInfo = { ...(liveListenInfo || liveListenDefaults), ...partial };
  renderLiveListenMeta(liveListenInfo);
}

function handleLiveListenAudioInfo(info) {
  if (!info || typeof info !== 'object') return;
  const partial = { status: 'Live' };
  if (info.demod) partial.demod = info.demod;
  if (info.playback_mode) partial.playback_mode = info.playback_mode;
  if (info.stereo_state) partial.stereo_state = info.stereo_state;
  if (Number.isFinite(info.sample_rate)) partial.sample_rate = info.sample_rate;
  if (Number.isFinite(info.channels)) partial.channels = info.channels;
  if (Object.keys(partial).length > 0) {
    updateLiveListenMeta(partial);
  }
}

function resolveListenMode(detectedMode) {
  const manual = listenModeSelect?.value || '';
  if (manual) return manual;
  return detectedMode || 'NFM';
}

function syncListeningVisuals() {
  signalList?.querySelectorAll('.signal-item').forEach(el => {
    const center = parseFloat(el.dataset.center || '0');
    const bw = parseFloat(el.dataset.bw || '0');
    const fakeSignal = { center_hz: center, bw_hz: bw };
    el.classList.toggle('listening', matchesListenTarget(fakeSignal));
  });
}

function setLiveListenUI(active) {
  if (liveListenBtn) {
    liveListenBtn.textContent = active ? '■ Stop' : 'Live Listen';
    liveListenBtn.classList.toggle('active', active);
  }
  if (liveListenEventBtn) {
    liveListenEventBtn.textContent = active ? '■ Stop' : 'Listen';
    liveListenEventBtn.classList.toggle('active', active);
  }
}

function stopLiveListen() {
  if (liveListenWS) {
    liveListenWS.onStop(() => {});
    liveListenWS.stop();
    liveListenWS = null;
  }
  liveListenTarget = null;
  setLiveListenUI(false);
  resetLiveListenMeta();
  syncListeningVisuals();
}

function startLiveListen(freq, bw, detectedMode) {
  if (!Number.isFinite(freq)) return;
  const mode = resolveListenMode(detectedMode);
  const width = Number.isFinite(bw) && bw > 0 ? bw : 12000;

  // Stop any old HTTP audio
  if (liveAudio) { liveAudio.pause(); liveAudio = null; }

  // Switch on the fly if already listening
  if (liveListenWS) {
    liveListenWS.onStop(() => {});
    liveListenWS.stop();
    liveListenWS = null;
  }

  liveListenTarget = { freq, bw: width, mode };

  liveListenWS = new LiveListenWS(freq, width, mode);
  liveListenWS.onStop(() => {
    liveListenWS = null;
    liveListenTarget = null;
    setLiveListenUI(false);
    resetLiveListenMeta();
  });
  liveListenWS.start();
  setLiveListenUI(true);
  syncListeningVisuals();

  const startingInfo = {
    status: 'Connecting',
    demod: mode || '-',
    playback_mode: mode || '-',
    stereo_state: mode === 'WFM_STEREO' ? 'searching' : 'mono',
    sample_rate: 48000,
    channels: mode === 'WFM_STEREO' ? 2 : 1
  };
  updateLiveListenMeta(startingInfo);
}

function matchesListenTarget(signal) {
  if (!liveListenTarget || !signal) return false;
  const bw = signal.bw_hz || liveListenTarget.bw || 0;
  const tol = Math.max(500, bw * 0.5);
  return Math.abs((signal.center_hz || 0) - liveListenTarget.freq) <= tol;
}

let gpuInfo = { available: false, active: false, error: '' };

let zoom = 1;
let pan = 0;
let followLive = true;
let maxHold = false;
let avgAlpha = 0;
let avgSpectrum = null;
let maxSpectrum = null;
let lastFFTSize = null;
let processedSpectrum = null;
let processedSpectrumSource = null;
let processingDirty = true;

let pendingConfigUpdate = null;
let pendingSettingsUpdate = null;
let configTimer = null;
let settingsTimer = null;
let isSyncingConfig = false;

let isDraggingSpectrum = false;
let dragStartX = 0;
let dragStartPan = 0;
let navDrag = false;
let timelineFrozen = false;

// Keep the browser path best-effort under load so audio work wins over paint churn.
const TARGET_VISUAL_FPS = 24;
const VISUAL_FRAME_INTERVAL_MS = 1000 / TARGET_VISUAL_FPS;
const WATERFALL_FRAME_INTERVAL_MS = 1000 / 10;
const DETAIL_RENDER_INTERVAL_MS = 1000 / 6;
const LIST_RENDER_INTERVAL_MS = 250;
const HERO_RENDER_INTERVAL_MS = 200;
const STATUS_RENDER_INTERVAL_MS = 250;
const MAX_RENDER_DPR = 1.25;
const WATERFALL_MIN_INTERNAL_WIDTH = 640;
const DETAIL_MIN_INTERNAL_WIDTH = 480;
const COLOR_LUT_SIZE = 1024;
const COLOR_LUT = new Uint8ClampedArray(COLOR_LUT_SIZE * 3);
for (let i = 0; i < COLOR_LUT_SIZE; i++) {
  const x = i / (COLOR_LUT_SIZE - 1);
  COLOR_LUT[i * 3] = Math.floor(255 * Math.pow(x, 0.55));
  COLOR_LUT[i * 3 + 1] = Math.floor(255 * Math.pow(x, 1.08));
  COLOR_LUT[i * 3 + 2] = Math.floor(220 * Math.pow(1 - x, 1.15));
}

let renderFrames = 0;
let renderFps = 0;
let lastFpsTs = performance.now();
let lastVisualRenderTs = 0;
let lastWaterfallRenderTs = 0;
let lastListRenderTs = 0;
let lastHeroRenderTs = 0;
let lastStatusRenderTs = 0;
let pendingWaterfallRender = true;
let pendingListRender = true;
let pendingHeroRender = true;
let pendingStatusRender = true;
let lastDetailRenderTs = 0;
let waterfallRowImageData = null;
// Double-buffer (ping-pong) for the waterfall scroll. Drawing a canvas onto
// itself under globalCompositeOperation='copy' is browser-dependent and could
// leave the shifted region blank (OI-19: only the newest top row was visible).
// Cross-canvas blits between two offscreen buffers avoid that entirely.
let waterfallBufA = null;
let waterfallBufB = null;
let waterfallFront = null; // buffer currently holding the displayed history
let detailRowImageData = null;
let detailRowCanvas = null;
let detailRowCtx = null;
let waterfallRangeCache = null;
let detailRangeCache = null;

let wsReconnectTimer = null;
let eventsFetchInFlight = false;
const events = [];
const eventsById = new Map();
let lastEventEndMs = 0;
let selectedEventId = null;
let timelineRects = [];
let liveSignalRects = [];
let recordings = [];
let recordingsFetchInFlight = false;
let showDebugOverlay = localStorage.getItem('spectre.debugOverlay') === '1';
let hoveredSignal = null;
let popoverHideTimer = null;

const GAIN_MAX = 60;
const timelineWindowMs = 5 * 60 * 1000;

function setConfigStatus(text) {
  configStatusEl.textContent = text;
  updateOperatorStatus();
}

function setWsBadge(text, kind = 'neutral') {
  wsState = kind === 'ok' ? 'live' : (kind === 'bad' ? 'retrying' : 'connecting');
  wsBadge.textContent = text;
  wsBadge.style.borderColor = kind === 'ok'
    ? 'rgba(124, 251, 131, 0.35)'
    : kind === 'bad'
      ? 'rgba(255, 107, 129, 0.35)'
      : 'rgba(112, 150, 207, 0.18)';
}

function updateApiState(result) {
  if (!result) return;
  const latency = result.meta?.duration_ms;
  if (Number.isFinite(latency)) apiState.latencyMs = latency;
  if (result.ok) {
    apiState.ok = true;
    apiState.lastOkTs = Date.now();
    apiState.lastError = '';
  } else {
    apiState.ok = false;
    apiState.lastError = result.error || (result.status ? `HTTP ${result.status}` : 'request failed');
  }
}

function fmtAgeShort(ms) {
  if (!Number.isFinite(ms) || ms < 0) return '-';
  if (ms < 1000) return `${Math.round(ms)} ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)} s`;
  return `${Math.round(ms / 60000)} min`;
}

function renderOperatorStatusNow() {
  if (operatorPanel) {
    operatorPanel.updateStatus({
      wsState,
      wsLastMessageTs,
      apiState,
      configStatusText: configStatusEl?.textContent || '-',
      refinementInfo,
      telemetryLive,
      sourceAgeMs: stats?.last_sample_ago_ms
    });
    return;
  }
  if (healthWs) {
    const age = wsLastMessageTs > 0 ? fmtAgeShort(Date.now() - wsLastMessageTs) : '-';
    healthWs.textContent = `${wsState} | last ${age}`;
  }
  if (healthApi) {
    const latency = Number.isFinite(apiState.latencyMs) ? `${apiState.latencyMs} ms` : 'n/a';
    healthApi.textContent = apiState.ok ? `ok | ${latency}` : `degraded | ${apiState.lastError || 'n/a'}`;
  }
  if (healthConfig) {
    healthConfig.textContent = configStatusEl?.textContent || '-';
  }
  if (healthRefine) {
    const plan = refinementInfo.plan || {};
    const queue = refinementInfo.arbitration?.queue || {};
    healthRefine.textContent = `${plan.selected?.length || 0}/${plan.budget || 0} | q ${queue.record_queued || 0}/${queue.decode_queued || 0}`;
  }
  if (healthTelemetry) {
    if (!telemetryLive) {
      healthTelemetry.textContent = 'unavailable';
    } else {
      const enabled = telemetryLive.enabled === false ? 'off' : 'on';
      const collector = telemetryLive.collector || {};
      const recent = Array.isArray(telemetryLive.recent_events) ? telemetryLive.recent_events.length : 0;
      const heavy = collector.heavy_enabled ? 'heavy' : 'light';
      healthTelemetry.textContent = `${enabled} | ${heavy} | events ${recent}`;
    }
  }
}

function flushOperatorStatus(now, force = false) {
  if (!pendingStatusRender) return;
  if (!force && now - lastStatusRenderTs < STATUS_RENDER_INTERVAL_MS) return;
  pendingStatusRender = false;
  lastStatusRenderTs = now;
  renderOperatorStatusNow();
}

function updateOperatorStatus(force = false) {
  pendingStatusRender = true;
  if (force) flushOperatorStatus(performance.now(), true);
}
function toMHz(hz) { return hz / 1e6; }
function fromMHz(mhz) { return mhz * 1e6; }
function fmtMHz(hz, digits = 3) { return `${(hz / 1e6).toFixed(digits)} MHz`; }
function fmtKHz(hz, digits = 2) { return `${(hz / 1e3).toFixed(digits)} kHz`; }
function fmtHz(hz) {
  if (hz >= 1e6) return `${(hz / 1e6).toFixed(3)} MHz`;
  if (hz >= 1e3) return `${(hz / 1e3).toFixed(2)} kHz`;
  return `${hz.toFixed(0)} Hz`;
}
function fmtMs(ms) {
  if (ms < 1000) return `${Math.max(0, Math.round(ms))} ms`;
  return `${(ms / 1000).toFixed(2)} s`;
}

function scoreEntries(scores, limit = 6) {
  if (!scores || typeof scores !== 'object') return [];
  return Object.entries(scores)
    .filter(([, v]) => Number.isFinite(Number(v)))
    .sort((a, b) => Number(b[1]) - Number(a[1]))
    .slice(0, limit);
}

function renderScoreBars(scores) {
  if (!classifierScoreBarsEl) return;
  const entries = scoreEntries(scores);
  if (!entries.length) {
    classifierScoreBarsEl.innerHTML = '';
    return;
  }
  const maxVal = Math.max(...entries.map(([, v]) => Number(v)), 1e-6);
  classifierScoreBarsEl.innerHTML = entries.map(([label, value]) => {
    const width = Math.max(4, (Number(value) / maxVal) * 100);
    return `<div class="score-bar"><span class="score-bar-label">${label}</span><span class="score-bar-track"><span class="score-bar-fill" style="width:${width}%"></span></span><span class="score-bar-value">${Number(value).toFixed(2)}</span></div>`;
  }).join('');
}

function hideSignalPopover() {
  hoveredSignal = null;
  if (!signalPopover) return;
  signalPopover.classList.remove('open');
  signalPopover.setAttribute('aria-hidden', 'true');
}

function renderSignalPopover(rect, signal) {
  if (!signalPopover || !signal) return;
  const entries = scoreEntries(signal.class?.scores || signal.debug_scores || {}, 4);
  const maxVal = Math.max(...entries.map(([, v]) => Number(v)), 1e-6);
  const rows = entries.map(([label, value]) => {
    const width = Math.max(4, (Number(value) / maxVal) * 100);
    return `<div class="signal-popover__row"><span>${label}</span><span class="signal-popover__bar"><span class="signal-popover__fill" style="width:${width}%"></span></span><span>${Number(value).toFixed(2)}</span></div>`;
  }).join('');
  const primaryMode = getSignalPrimaryMode(signal);
  const runtimeInfo = getSignalRuntimeSummary(signal);
  signalPopover.innerHTML = `<div class="signal-popover__title">${primaryMode}${signal.class?.pll?.rds_station ? ' · ' + signal.class.pll.rds_station : ''}</div><div class="signal-popover__meta">${fmtMHz(signal.class?.pll?.exact_hz || signal.center_hz, 5)} · ${fmtKHz(signal.bw_hz || 0)} · ${(signal.snr_db || 0).toFixed(1)} dB SNR${runtimeInfo ? ` · ${runtimeInfo}` : ''}${signal.class?.pll?.locked ? ` · PLL ${signal.class.pll.method} LOCK` : ''}${signal.class?.pll?.stereo ? ' · STEREO' : ''}</div><div class="signal-popover__scores">${rows || '<div class="signal-popover__meta">No classifier scores</div>'}</div>`;
  const popW = 220;
  const top = rect.y + 8;
  const left = rect.x + (rect.w / 2) - (popW / 2);
  const maxLeft = Math.max(8, spectrumCanvas.width - popW - 8);
  signalPopover.style.left = `${Math.max(8, Math.min(maxLeft, left))}px`;
  signalPopover.style.top = `${Math.max(8, top)}px`;
  signalPopover.classList.add('open');
  signalPopover.setAttribute('aria-hidden', 'false');
}


function getLutColor(v) {
  const idx = Math.max(0, Math.min(COLOR_LUT_SIZE - 1, Math.round(v * (COLOR_LUT_SIZE - 1))));
  const base = idx * 3;
  return [COLOR_LUT[base], COLOR_LUT[base + 1], COLOR_LUT[base + 2]];
}

function fillSpectrumRowRGBA(target, width, rangeCache, display, centerHz, sampleRate, minDb = -120, maxDb = 0) {
  if (!target || !display || width <= 0) return;
  const spanMap = rangeCache?.map;
  const bins = display.length;
  const dbSpan = Math.max(1e-6, maxDb - minDb);
  for (let x = 0; x < width; x++) {
    let start = 0;
    let end = bins - 1;
    if (spanMap) {
      start = spanMap[x * 2];
      end = spanMap[x * 2 + 1];
    }
    let v = -1e9;
    for (let i = start; i <= end; i++) {
      const cur = display[i];
      if (cur > v) v = cur;
    }
    const norm = Math.max(0, Math.min(1, (v - minDb) / dbSpan));
    const lutBase = Math.max(0, Math.min(COLOR_LUT_SIZE - 1, Math.round(norm * (COLOR_LUT_SIZE - 1)))) * 3;
    const di = x * 4;
    target[di] = COLOR_LUT[lutBase];
    target[di + 1] = COLOR_LUT[lutBase + 1];
    target[di + 2] = COLOR_LUT[lutBase + 2];
    target[di + 3] = 255;
  }
}

function getRangeCache(prevCache, width, startHz, endHz, centerHz, sampleRate, n) {
  const key = `${width}|${startHz.toFixed(3)}|${endHz.toFixed(3)}|${centerHz.toFixed(3)}|${sampleRate}|${n}`;
  if (prevCache && prevCache.key === key) return prevCache;
  const map = new Int32Array(width * 2);
  for (let x = 0; x < width; x++) {
    const f1 = startHz + (x / width) * (endHz - startHz);
    const f2 = startHz + ((x + 1) / width) * (endHz - startHz);
    let b0 = binForFreq(f1, centerHz, sampleRate, n);
    let b1 = binForFreq(f2, centerHz, sampleRate, n);
    if (b1 < b0) [b0, b1] = [b1, b0];
    map[x * 2] = Math.max(0, Math.min(n - 1, b0));
    map[x * 2 + 1] = Math.max(0, Math.min(n - 1, b1));
  }
  return { key, map };
}

function getCanvas2DContext(canvas, opts = {}) {
  if (!canvas) return null;
  if (!canvas.__ctx2d) {
    canvas.__ctx2d = canvas.getContext('2d', {
      alpha: false,
      desynchronized: true,
      willReadFrequently: !!opts.willReadFrequently
    }) || canvas.getContext('2d');
  }
  return canvas.__ctx2d;
}

function ensureDetailRowCanvas(width) {
  if (!detailRowCanvas) {
    detailRowCanvas = document.createElement('canvas');
    detailRowCanvas.width = Math.max(1, width);
    detailRowCanvas.height = 1;
    detailRowCtx = detailRowCanvas.getContext('2d', { alpha: false, desynchronized: true }) || detailRowCanvas.getContext('2d');
  }
  if (detailRowCanvas.width !== width) {
    detailRowCanvas.width = Math.max(1, width);
    detailRowCanvas.height = 1;
    detailRowCtx = detailRowCanvas.getContext('2d', { alpha: false, desynchronized: true }) || detailRowCanvas.getContext('2d');
    detailRowImageData = null;
  }
}

function getWaterfallInternalWidth(canvas) {
  return Math.max(WATERFALL_MIN_INTERNAL_WIDTH, Math.min(canvas.width, Math.floor(canvas.width * 0.85)));
}

function getDetailInternalWidth(canvas) {
  return Math.max(DETAIL_MIN_INTERNAL_WIDTH, Math.min(canvas.width, Math.floor(canvas.width * 0.9)));
}

function colorMap(v) {
  return getLutColor(Math.max(0, Math.min(1, v)));
}

function snrColor(snr) {
  const norm = Math.max(0, Math.min(1, (snr + 5) / 35));
  const [r, g, b] = colorMap(norm);
  return `rgb(${r}, ${g}, ${b})`;
}

// Modulation-type color map for signal boxes and badges
function modColor(modType) {
  switch (modType) {
    case 'WFM':        return { r: 72, g: 210, b: 120, label: '#48d278' }; // green
    case 'WFM_STEREO': return { r: 72, g: 230, b: 160, label: '#48e6a0' }; // bright green
    case 'NFM':        return { r: 255, g: 170, b: 60, label: '#ffaa3c' };  // orange
    case 'AM':         return { r: 90, g: 160, b: 255, label: '#5aa0ff' };  // blue
    case 'USB': case 'LSB':
                       return { r: 160, g: 120, b: 255, label: '#a078ff' }; // purple
    case 'CW':         return { r: 255, g: 100, b: 100, label: '#ff6464' }; // red
    case 'FT8': case 'WSPR': case 'FSK': case 'PSK':
                       return { r: 255, g: 220, b: 80, label: '#ffdc50' };  // yellow
    default:           return { r: 160, g: 190, b: 220, label: '#a0bedc' }; // grey-blue
  }
}
function modColorStr(modType, alpha) {
  const c = modColor(modType);
  return `rgba(${c.r}, ${c.g}, ${c.b}, ${alpha})`;
}

function binForFreq(freq, centerHz, sampleRate, n) {
  return Math.floor((freq - (centerHz - sampleRate / 2)) / (sampleRate / n));
}

function maxInBinRange(spectrum, b0, b1) {
  const n = spectrum.length;
  let start = Math.max(0, Math.min(n - 1, b0));
  let end = Math.max(0, Math.min(n - 1, b1));
  if (end < start) [start, end] = [end, start];
  let max = -1e9;
  for (let i = start; i <= end; i++) {
    if (spectrum[i] > max) max = spectrum[i];
  }
  return max;
}

function sampleOverlayAtX(overlay, x, width, centerHz, sampleRate) {
  if (!Array.isArray(overlay) || overlay.length === 0 || width <= 0) return null;
  const n = overlay.length;
  const span = sampleRate / zoom;
  const startHz = centerHz - span / 2 + pan * span;
  const endHz = centerHz + span / 2 + pan * span;
  const f1 = startHz + (x / width) * (endHz - startHz);
  const f2 = startHz + ((x + 1) / width) * (endHz - startHz);
  const b0 = binForFreq(f1, centerHz, sampleRate, n);
  const b1 = binForFreq(f2, centerHz, sampleRate, n);
  return maxInBinRange(overlay, b0, b1);
}

function drawThresholdOverlay(ctx, w, h, minDb, maxDb) {
  if (!showDebugOverlay) return;
  const thresholds = latest?.debug?.thresholds;
  if (!Array.isArray(thresholds) || thresholds.length === 0) return;
  ctx.save();
  ctx.strokeStyle = 'rgba(255, 196, 92, 0.9)';
  ctx.lineWidth = 1.25;
  if (ctx.setLineDash) ctx.setLineDash([6, 4]);
  ctx.beginPath();
  for (let x = 0; x < w; x++) {
    const v = sampleOverlayAtX(thresholds, x, w, latest.center_hz, latest.sample_rate);
    if (v == null || Number.isNaN(v)) continue;
    const y = h - ((v - minDb) / (maxDb - minDb)) * (h - 18) - 6;
    if (x === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.stroke();
  if (ctx.setLineDash) ctx.setLineDash([]);
  ctx.fillStyle = 'rgba(255, 196, 92, 0.95)';
  ctx.font = '11px Inter, sans-serif';
  ctx.fillText('CFAR', 8, 14);
  ctx.restore();
}

function markSpectrumDirty() {
  processingDirty = true;
  pendingWaterfallRender = true;
}

function processSpectrum(spectrum) {
  if (!spectrum) return spectrum;
  let base = spectrum;
  if (avgAlpha > 0) {
    if (!avgSpectrum || avgSpectrum.length !== spectrum.length) {
      avgSpectrum = spectrum.slice();
    } else {
      for (let i = 0; i < spectrum.length; i++) {
        avgSpectrum[i] = avgAlpha * spectrum[i] + (1 - avgAlpha) * avgSpectrum[i];
      }
    }
    base = avgSpectrum;
  }
  if (maxHold) {
    if (!maxSpectrum || maxSpectrum.length !== base.length) {
      maxSpectrum = base.slice();
    } else {
      for (let i = 0; i < base.length; i++) {
        if (base[i] > maxSpectrum[i]) maxSpectrum[i] = base[i];
      }
    }
    base = maxSpectrum;
  }
  return base;
}

function resetProcessingCaches() {
  avgSpectrum = null;
  maxSpectrum = null;
  processedSpectrum = null;
  processedSpectrumSource = null;
  processingDirty = true;
}

function getProcessedSpectrum() {
  if (!latest?.spectrum_db) return null;
  if (!processingDirty && processedSpectrumSource === latest.spectrum_db) return processedSpectrum;
  processedSpectrum = processSpectrum(latest.spectrum_db);
  processedSpectrumSource = latest.spectrum_db;
  processingDirty = false;
  return processedSpectrum;
}

function resizeCanvas(canvas) {
  if (!canvas) return;
  const rect = canvas.getBoundingClientRect();
  const dpr = Math.min(window.devicePixelRatio || 1, MAX_RENDER_DPR);
  let width = Math.max(1, Math.floor(rect.width * dpr));
  const height = Math.max(1, Math.floor(rect.height * dpr));
  if (canvas === waterfallCanvas) width = getWaterfallInternalWidth({ width });
  if (canvas === detailSpectrogram) width = getDetailInternalWidth({ width });
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width;
    canvas.height = height;
    if (canvas === waterfallCanvas) {
      waterfallRowImageData = null;
      waterfallRangeCache = null;
      waterfallBufA = null;
      waterfallBufB = null;
      waterfallFront = null;
      pendingWaterfallRender = true;
    }
    if (canvas === detailSpectrogram) {
      detailRowImageData = null;
      detailRangeCache = null;
    }
  }
}

function resizeAll() {
  [navCanvas, spectrumCanvas, waterfallCanvas, occupancyCanvas, timelineCanvas, detailSpectrogram].forEach(resizeCanvas);
  pendingWaterfallRender = true;
}
window.addEventListener('resize', resizeAll);
resizeAll();


function setSelectValueOrNearest(selectEl, numericValue) {
  if (!selectEl) return;
  const options = Array.from(selectEl.options || []);
  const exact = options.find(o => Number.parseFloat(o.value) === numericValue);
  if (exact) {
    selectEl.value = exact.value;
    return;
  }
  let best = options[0];
  let bestDist = Infinity;
  for (const opt of options) {
    const dist = Math.abs(Number.parseFloat(opt.value) - numericValue);
    if (dist < bestDist) {
      best = opt;
      bestDist = dist;
    }
  }
  if (best) selectEl.value = best.value;
}

function applyConfigToUI(cfg) {
  if (!cfg) return;
  isSyncingConfig = true;
  centerInput.value = toMHz(cfg.center_hz).toFixed(6);
  setSelectValueOrNearest(sampleRateSelect, cfg.sample_rate / 1e6);
  setSelectValueOrNearest(bwSelect, cfg.tuner_bw_khz || 1536);
  setSelectValueOrNearest(fftSelect, cfg.fft_size);
  if (welchSegInput && cfg.surveillance) welchSegInput.value = cfg.surveillance.welch_segments ?? 0;
  if (lastFFTSize !== cfg.fft_size) {
    resetProcessingCaches();
    lastFFTSize = cfg.fft_size;
  }
  const uiGain = Math.max(0, Math.min(GAIN_MAX, GAIN_MAX - cfg.gain_db));
  gainRange.value = uiGain;
  gainInput.value = uiGain;
  thresholdRange.value = cfg.detector.threshold_db;
  thresholdInput.value = cfg.detector.threshold_db;
  if (cfarModeSelect) cfarModeSelect.value = cfg.detector.cfar_mode || 'OFF';
  if (cfarWrapToggle) cfarWrapToggle.checked = cfg.detector.cfar_wrap_around !== false;
  if (cfarGuardHzInput) cfarGuardHzInput.value = cfg.detector.cfar_guard_hz ?? 500;
  if (cfarTrainHzInput) cfarTrainHzInput.value = cfg.detector.cfar_train_hz ?? 5000;
  if (cfarRankInput) cfarRankInput.value = cfg.detector.cfar_rank ?? 24;
  if (cfarScaleInput) cfarScaleInput.value = cfg.detector.cfar_scale_db ?? 6;
  const rankRow = cfarRankInput?.closest('.field');
  if (rankRow) rankRow.style.display = (cfg.detector.cfar_mode === 'OS') ? '' : 'none';
  if (minDurationInput) minDurationInput.value = cfg.detector.min_duration_ms;
  if (holdInput) holdInput.value = cfg.detector.hold_ms;
  if (emaAlphaInput) emaAlphaInput.value = cfg.detector.ema_alpha ?? 0.2;
  if (hysteresisInput) hysteresisInput.value = cfg.detector.hysteresis_db ?? 3;
  if (stableFramesInput) stableFramesInput.value = cfg.detector.min_stable_frames ?? 3;
  if (gapToleranceInput) gapToleranceInput.value = cfg.detector.gap_tolerance_ms ?? cfg.detector.hold_ms;
  if (classifierModeSelect) classifierModeSelect.value = cfg.classifier_mode || 'combined';
  if (edgeMarginInput) edgeMarginInput.value = cfg.detector.edge_margin_db ?? 3.0;
  if (mergeGapInput) mergeGapInput.value = cfg.detector.merge_gap_hz ?? 5000;
  if (classHistoryInput) classHistoryInput.value = cfg.detector.class_history_size ?? 10;
  if (classSwitchInput) classSwitchInput.value = cfg.detector.class_switch_ratio ?? 0.6;
  agcToggle.checked = !!cfg.agc;
  dcToggle.checked = !!cfg.dc_block;
  iqToggle.checked = !!cfg.iq_balance;
  gpuToggle.checked = !!cfg.use_gpu_fft;
  maxHoldToggle.checked = maxHold;
  if (cfg.recorder) {
    if (recEnableToggle) recEnableToggle.checked = !!cfg.recorder.enabled;
    if (recIQToggle) recIQToggle.checked = !!cfg.recorder.record_iq;
    if (recAudioToggle) recAudioToggle.checked = !!cfg.recorder.record_audio;
    if (recDemodToggle) recDemodToggle.checked = !!cfg.recorder.auto_demod;
    if (recDecodeToggle) recDecodeToggle.checked = !!cfg.recorder.auto_decode;
    if (recMinSNR) recMinSNR.value = cfg.recorder.min_snr_db ?? 10;
    if (recMaxDisk) recMaxDisk.value = cfg.recorder.max_disk_mb ?? 0;
    if (recClassFilter) recClassFilter.value = (cfg.recorder.class_filter || []).join(', ');
  }
  if (refineAutoSpan) refineAutoSpan.value = String(cfg.refinement?.auto_span ?? true);
  if (refineMinSpan) refineMinSpan.value = cfg.refinement?.min_span_hz ?? 0;
  if (refineMaxSpan) refineMaxSpan.value = cfg.refinement?.max_span_hz ?? 0;
  if (resMaxRefine) resMaxRefine.value = cfg.resources?.max_refinement_jobs ?? 0;
  if (resMaxRecord) resMaxRecord.value = cfg.resources?.max_recording_streams ?? 0;
  if (resMaxDecode) resMaxDecode.value = cfg.resources?.max_decode_jobs ?? 0;
  spanInput.value = (cfg.sample_rate / zoom / 1e6).toFixed(3);
  isSyncingConfig = false;
}

async function loadConfig() {
  if (!apiClient) return;
  const res = await apiClient.getConfig();
  updateApiState(res);
  if (res.ok && res.data) {
    currentConfig = res.data;
    applyConfigToUI(currentConfig);
    setConfigStatus('Config synced');
  } else {
    setConfigStatus('Config offline');
  }
  updateOperatorStatus();
}

async function loadSignals() {
  if (!apiClient) return;
  const res = await apiClient.getSignals();
  updateApiState(res);
  if (!res.ok || !Array.isArray(res.data)) return;
  latest = latest || {};
  latest.signals = res.data;
  updateHeroMetrics();
  renderLists();
}

async function loadDecoders() {
  if (!decodeModeSelect || !apiClient) return;
  const res = await apiClient.getDecoders();
  updateApiState(res);
  if (!res.ok || !Array.isArray(res.data)) return;
  const current = decodeModeSelect.value;
  decodeModeSelect.innerHTML = '';
  res.data.forEach((mode) => {
    const opt = document.createElement('option');
    opt.value = mode;
    opt.textContent = mode;
    decodeModeSelect.appendChild(opt);
  });
  if (current) decodeModeSelect.value = current;
}

async function loadStats() {
  if (!apiClient) return;
  const res = await apiClient.getStats();
  updateApiState(res);
  if (res.ok && res.data) stats = res.data;
  updateHeroMetrics();
  updateOperatorStatus();
}

async function loadGPU() {
  if (!apiClient) return;
  const res = await apiClient.getGPU();
  updateApiState(res);
  if (res.ok && res.data) gpuInfo = res.data;
  updateHeroMetrics();
  updateOperatorStatus();
}

async function loadRefinement() {
  if (!apiClient) return;
  const res = await apiClient.getRefinement();
  updateApiState(res);
  if (!res.ok || !res.data) return;
  refinementInfo = res.data;
  decisionIndex = new Map();
  const items = Array.isArray(refinementInfo.arbitration?.decision_items) ? refinementInfo.arbitration.decision_items : [];
  items.forEach(item => {
    if (item && item.id != null) decisionIndex.set(String(item.id), item);
  });
  updateSignalDecisionSummary(window._selectedSignal?.id);
  updateSignalQueueSummary();
  updateHeroMetrics();
  updateOperatorStatus();
}

async function loadTelemetryLive() {
  if (!apiClient) return;
  const res = await apiClient.getTelemetryLive();
  updateApiState(res);
  if (!res.ok) {
    telemetryLive = null;
    updateOperatorStatus();
    return;
  }
  telemetryLive = res.data;
  updateOperatorStatus();
}

function renderKvList(root, rows, emptyText) {
  if (!root) return;
  if (!rows || !rows.length) {
    root.innerHTML = `<div class="ops-line ops-line--muted">${emptyText}</div>`;
    return;
  }
  root.innerHTML = rows.map(({ key, value }) => `<div class="kv-row"><div class="kv-key">${key}</div><div class="kv-val">${value}</div></div>`).join('');
}

function renderPolicyLists() {
  if (policySummaryList) {
    if (!policyInfo) {
      renderKvList(policySummaryList, [], 'Policy unavailable.');
    } else {
      renderKvList(policySummaryList, [
        { key: 'Profile', value: policyInfo.profile || 'n/a' },
        { key: 'Mode', value: policyInfo.mode || 'n/a' },
        { key: 'Intent', value: policyInfo.intent || 'n/a' },
        { key: 'Surveillance', value: policyInfo.surveillance_strategy || 'n/a' },
        { key: 'Refinement', value: policyInfo.refinement_strategy || 'n/a' }
      ], 'Policy unavailable.');
    }
  }
  if (policyRecommendationList) {
    if (!policyRecommendations) {
      renderKvList(policyRecommendationList, [], 'Recommendations unavailable.');
    } else {
      renderKvList(policyRecommendationList, [
        { key: 'Monitor span', value: Number.isFinite(policyRecommendations.monitor_span_hz) ? fmtHz(policyRecommendations.monitor_span_hz) : 'n/a' },
        { key: 'Refine jobs', value: policyRecommendations.refinement_jobs ?? 'n/a' },
        { key: 'Detail FFT', value: policyRecommendations.refinement_detail_fft ?? 'n/a' },
        { key: 'Auto span', value: policyRecommendations.refinement_auto_span ?? 'n/a' },
        { key: 'Auto record', value: Array.isArray(policyRecommendations.auto_record_classes) && policyRecommendations.auto_record_classes.length ? policyRecommendations.auto_record_classes.join(', ') : 'n/a' },
        { key: 'Auto decode', value: Array.isArray(policyRecommendations.auto_decode_classes) && policyRecommendations.auto_decode_classes.length ? policyRecommendations.auto_decode_classes.join(', ') : 'n/a' }
      ], 'Recommendations unavailable.');
    }
  }
}

async function loadPolicy() {
  if (!apiClient) return;
  const [policyRes, recRes] = await Promise.all([
    apiClient.getPolicy(),
    apiClient.getRecommendations()
  ]);
  updateApiState(policyRes.ok ? policyRes : recRes);
  policyInfo = policyRes.ok ? policyRes.data : null;
  policyRecommendations = recRes.ok ? recRes.data : null;
  renderPolicyLists();
}

function formatLevelSummary(level) {
  if (!level) return 'n/a';
  const name = level.name || 'level';
  const fft = level.fft_size ? `${level.fft_size} bins` : 'bins n/a';
  const span = level.span_hz ? fmtHz(level.span_hz) : 'span n/a';
  const binHz = level.bin_hz || ((level.sample_rate && level.fft_size) ? (level.sample_rate / level.fft_size) : 0);
  const binText = binHz ? `${binHz.toFixed(1)} Hz/bin` : 'bin n/a';
  const decim = level.decimation && level.decimation > 1 ? `decim ${level.decimation}` : '';
  const source = level.source ? `src ${level.source}` : '';
  return [name, fft, span, binText, decim, source].filter(Boolean).join(' · ');
}

function queueConfigUpdate(partial) {
  if (isSyncingConfig) return;
  pendingConfigUpdate = { ...(pendingConfigUpdate || {}), ...partial };
  setConfigStatus('Applying…');
  clearTimeout(configTimer);
  configTimer = setTimeout(sendConfigUpdate, 180);
}

function queueSettingsUpdate(partial) {
  if (isSyncingConfig) return;
  pendingSettingsUpdate = { ...(pendingSettingsUpdate || {}), ...partial };
  setConfigStatus('Applying…');
  clearTimeout(settingsTimer);
  settingsTimer = setTimeout(sendSettingsUpdate, 120);
}

async function sendConfigUpdate() {
  if (!pendingConfigUpdate) return;
  const payload = pendingConfigUpdate;
  pendingConfigUpdate = null;
  if (!apiClient) return;
  const res = await apiClient.postConfig(payload);
  updateApiState(res);
  if (res.ok && res.data) {
    currentConfig = res.data;
    applyConfigToUI(currentConfig);
    setConfigStatus('Config applied');
  } else {
    setConfigStatus('Config apply failed');
  }
}

async function sendSettingsUpdate() {
  if (!pendingSettingsUpdate) return;
  const payload = pendingSettingsUpdate;
  pendingSettingsUpdate = null;
  if (!apiClient) return;
  const res = await apiClient.postSettings(payload);
  updateApiState(res);
  if (res.ok && res.data) {
    currentConfig = res.data;
    applyConfigToUI(currentConfig);
    setConfigStatus('Settings applied');
  } else {
    setConfigStatus('Settings apply failed');
  }
}

function renderHeroMetricsNow() {
  if (!latest) return;
  const span = latest.sample_rate / zoom;
  const binHz = latest.sample_rate / Math.max(1, latest.spectrum_db?.length || latest.fft_size || 1);
  metricCenter.textContent = fmtMHz(latest.center_hz, 6);
  metricSpan.textContent = fmtHz(span);
  metricRes.textContent = `${binHz.toFixed(1)} Hz/bin`;
  metricSignals.textContent = String(latest.signals?.length || 0);
  metricGpu.textContent = gpuInfo.active ? 'ON' : (gpuInfo.available ? 'OFF' : 'N/A');
  metricSource.textContent = stats.last_sample_ago_ms >= 0 ? `${stats.last_sample_ago_ms} ms` : 'n/a';

  const gpuText = gpuInfo.active ? 'GPU active' : (gpuInfo.available ? 'GPU ready' : 'GPU n/a');
  const debug = latest.debug || {};
  const thresholdInfo = Array.isArray(debug.thresholds) && debug.thresholds.length
    ? `CFAR ${showDebugOverlay ? 'on' : 'hidden'} · noise ${(Number.isFinite(debug.noise_floor) ? debug.noise_floor.toFixed(1) : 'n/a')} dB`
    : `CFAR off · noise ${(Number.isFinite(debug.noise_floor) ? debug.noise_floor.toFixed(1) : 'n/a')} dB`;
  const plan = debug.refinement_plan || null;
  const windowSummary = debug.window_summary || null;
  const windows = (windowSummary && windowSummary.refinement) || debug.refinement_windows || null;
  const refineInfo = plan && showDebugOverlay
    ? `refine ${plan.selected?.length || 0}/${plan.budget || 0} drop ${plan.dropped_by_snr || 0}/${plan.dropped_by_budget || 0}`
    : '';
  const windowInfo = windows && showDebugOverlay
    ? `win ${windows.count || 0} span ${fmtHz(windows.min_span_hz || 0)}–${fmtHz(windows.max_span_hz || 0)}`
    : '';
  const extras = [refineInfo, windowInfo].filter(Boolean).join(' · ');
  metaLine.textContent = `${fmtMHz(latest.center_hz, 3)} · ${fmtHz(span)} span · ${thresholdInfo}${extras ? ' · ' + extras : ''} · ${gpuText}`;
  heroSubtitle.textContent = `${latest.signals?.length || 0} live signals · ${events.length} recent events tracked`;

  healthBuffer.textContent = String(stats.buffer_samples ?? '-');
  healthDropped.textContent = String(stats.dropped ?? '-');
  healthResets.textContent = String(stats.resets ?? '-');
  healthAge.textContent = stats.last_sample_ago_ms >= 0 ? `${stats.last_sample_ago_ms} ms` : 'n/a';
  healthGpu.textContent = gpuInfo.error ? `${gpuInfo.active ? 'ON' : 'OFF'} · ${gpuInfo.error}` : (gpuInfo.active ? 'ON' : (gpuInfo.available ? 'Ready' : 'N/A'));
  healthFps.textContent = `${renderFps.toFixed(0)} fps`;
  if (healthRefinePlan) {
    const plan = refinementInfo.plan || {};
    const decisionSummary = refinementInfo.arbitration?.decision_summary || {};
    const recOn = decisionSummary.record_enabled ?? 0;
    const decOn = decisionSummary.decode_enabled ?? 0;
    const reasonCounts = decisionSummary.reasons || {};
    const topReason = Object.entries(reasonCounts).sort((a, b) => b[1] - a[1])[0];
    const reasonText = topReason ? `· ${topReason[0]}` : '';
    const queueStats = refinementInfo.arbitration?.queue || {};
    const queueText = (queueStats.record_queued || queueStats.decode_queued)
      ? `· q ${queueStats.record_queued || 0}/${queueStats.decode_queued || 0}`
      : '';
    healthRefinePlan.textContent = `${plan.selected?.length || 0}/${plan.budget || 0} · drop ${plan.dropped_by_snr || 0}/${plan.dropped_by_budget || 0} · rec ${recOn} dec ${decOn} ${queueText} ${reasonText}`;
  }
  if (healthRefineWindows) {
    const stats = refinementInfo.window_summary?.refinement || refinementInfo.window_stats || null;
    if (stats && stats.count) {
      const levelSet = refinementInfo.surveillance_level_set || {};
      const primary = levelSet.primary || refinementInfo.surveillance_level;
      const presentation = levelSet.presentation || refinementInfo.display_level || null;
      const primaryText = primary ? ` · primary ${formatLevelSummary(primary)}` : '';
      const presentationText = presentation ? ` · display ${formatLevelSummary(presentation)}` : '';
      healthRefineWindows.textContent = `${fmtHz(stats.min_span_hz || 0)}–${fmtHz(stats.max_span_hz || 0)}${primaryText}${presentationText}`;
    } else {
      const windows = refinementInfo.windows || [];
      if (!Array.isArray(windows) || windows.length === 0) {
        healthRefineWindows.textContent = 'n/a';
      } else {
        const spans = windows.map(w => w.span_hz || 0).filter(v => v > 0);
        const minSpan = spans.length ? Math.min(...spans) : 0;
        const maxSpan = spans.length ? Math.max(...spans) : 0;
        healthRefineWindows.textContent = spans.length ? `${fmtHz(minSpan)}–${fmtHz(maxSpan)}` : 'n/a';
      }
    }
  }
}

function flushHeroMetrics(now, force = false) {
  if (!pendingHeroRender) return;
  if (!force && now - lastHeroRenderTs < HERO_RENDER_INTERVAL_MS) return;
  pendingHeroRender = false;
  lastHeroRenderTs = now;
  renderHeroMetricsNow();
}

function updateHeroMetrics(force = false) {
  pendingHeroRender = true;
  if (force) flushHeroMetrics(performance.now(), true);
}

function renderBandNavigator() {
  if (!latest) return;
  const ctx = navCanvas.getContext('2d');
  const w = navCanvas.width;
  const h = navCanvas.height;
  ctx.clearRect(0, 0, w, h);

  const display = getProcessedSpectrum();
  if (!display) return;
  const minDb = -120;
  const maxDb = 0;

  ctx.fillStyle = '#071018';
  ctx.fillRect(0, 0, w, h);

  ctx.strokeStyle = 'rgba(102, 169, 255, 0.25)';
  ctx.lineWidth = 1;
  ctx.beginPath();
  for (let x = 0; x < w; x++) {
    const idx = Math.min(display.length - 1, Math.floor((x / w) * display.length));
    const v = display[idx];
    const y = h - ((v - minDb) / (maxDb - minDb)) * (h - 10) - 5;
    if (x === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.stroke();

  const span = latest.sample_rate / zoom;
  const fullStart = latest.center_hz - latest.sample_rate / 2;
  const viewStart = latest.center_hz - span / 2 + pan * span;
  const viewEnd = latest.center_hz + span / 2 + pan * span;
  const x1 = ((viewStart - fullStart) / latest.sample_rate) * w;
  const x2 = ((viewEnd - fullStart) / latest.sample_rate) * w;

  ctx.fillStyle = 'rgba(102, 240, 209, 0.10)';
  ctx.strokeStyle = 'rgba(102, 240, 209, 0.85)';
  ctx.lineWidth = 2;
  ctx.fillRect(x1, 4, Math.max(2, x2 - x1), h - 8);
  ctx.strokeRect(x1, 4, Math.max(2, x2 - x1), h - 8);
}

function drawSpectrumGrid(ctx, w, h, startHz, endHz) {
  ctx.strokeStyle = 'rgba(86, 109, 148, 0.18)';
  ctx.lineWidth = 1;
  for (let i = 1; i < 6; i++) {
    const y = (h / 6) * i;
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(w, y);
    ctx.stroke();
  }
  for (let i = 1; i < 8; i++) {
    const x = (w / 8) * i;
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, h);
    ctx.stroke();
    const hz = startHz + (i / 8) * (endHz - startHz);
    ctx.fillStyle = 'rgba(173, 192, 220, 0.72)';
    ctx.font = `${Math.max(11, Math.floor(h / 26))}px Inter, sans-serif`;
    ctx.fillText((hz / 1e6).toFixed(3), x + 4, h - 8);
  }
}

function drawCfarEdgeOverlay(ctx, w, h, startHz, endHz) {
  if (!latest) return;
  const mode = currentConfig?.detector?.cfar_mode || 'OFF';
  if (mode === 'OFF') return;
  if (currentConfig?.detector?.cfar_wrap_around) return;
  const guardHz = currentConfig.detector.cfar_guard_hz ?? 500;
  const trainHz = currentConfig.detector.cfar_train_hz ?? 5000;
  const fftSize = latest.fft_size || latest.spectrum_db?.length;
  if (!fftSize || fftSize <= 0) return;
  const binW = (latest.sample_rate || 2048000) / fftSize;
  const bins = Math.ceil(guardHz / binW) + Math.ceil(trainHz / binW);
  if (bins <= 0) return;
  const binHz = latest.sample_rate / fftSize;
  const edgeHz = bins * binHz;
  const bandStart = latest.center_hz - latest.sample_rate / 2;
  const bandEnd = latest.center_hz + latest.sample_rate / 2;
  const leftEdgeEnd = bandStart + edgeHz;
  const rightEdgeStart = bandEnd - edgeHz;

  ctx.fillStyle = 'rgba(255, 204, 102, 0.08)';
  ctx.strokeStyle = 'rgba(255, 204, 102, 0.18)';
  ctx.lineWidth = 1;

  const leftStart = Math.max(startHz, bandStart);
  const leftEnd = Math.min(endHz, leftEdgeEnd);
  if (leftEnd > leftStart) {
    const x1 = ((leftStart - startHz) / (endHz - startHz)) * w;
    const x2 = ((leftEnd - startHz) / (endHz - startHz)) * w;
    ctx.fillRect(x1, 0, Math.max(2, x2 - x1), h);
    ctx.strokeRect(x1, 0, Math.max(2, x2 - x1), h);
  }

  const rightStart = Math.max(startHz, rightEdgeStart);
  const rightEnd = Math.min(endHz, bandEnd);
  if (rightEnd > rightStart) {
    const x1 = ((rightStart - startHz) / (endHz - startHz)) * w;
    const x2 = ((rightEnd - startHz) / (endHz - startHz)) * w;
    ctx.fillRect(x1, 0, Math.max(2, x2 - x1), h);
    ctx.strokeRect(x1, 0, Math.max(2, x2 - x1), h);
  }
}

function renderSpectrum() {
  if (!latest) return;
  const ctx = spectrumCanvas.getContext('2d');
  const w = spectrumCanvas.width;
  const h = spectrumCanvas.height;
  ctx.clearRect(0, 0, w, h);

  const display = getProcessedSpectrum();
  if (!display) return;
  const n = display.length;
  const span = latest.sample_rate / zoom;
  const startHz = latest.center_hz - span / 2 + pan * span;
  const endHz = latest.center_hz + span / 2 + pan * span;
  spanInput.value = (span / 1e6).toFixed(3);

  drawSpectrumGrid(ctx, w, h, startHz, endHz);
  drawCfarEdgeOverlay(ctx, w, h, startHz, endHz);

  const minDb = -120;
  const maxDb = 0;

  const fill = ctx.createLinearGradient(0, 0, 0, h);
  fill.addColorStop(0, 'rgba(102, 240, 209, 0.20)');
  fill.addColorStop(1, 'rgba(102, 240, 209, 0.02)');

  ctx.beginPath();
  for (let x = 0; x < w; x++) {
    const f1 = startHz + (x / w) * (endHz - startHz);
    const f2 = startHz + ((x + 1) / w) * (endHz - startHz);
    const b0 = binForFreq(f1, latest.center_hz, latest.sample_rate, n);
    const b1 = binForFreq(f2, latest.center_hz, latest.sample_rate, n);
    const v = maxInBinRange(display, b0, b1);
    const y = h - ((v - minDb) / (maxDb - minDb)) * (h - 18) - 6;
    if (x === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.lineTo(w, h);
  ctx.lineTo(0, h);
  ctx.closePath();
  ctx.fillStyle = fill;
  ctx.fill();

  ctx.strokeStyle = '#66f0d1';
  ctx.lineWidth = 2;
  ctx.beginPath();
  liveSignalRects = [];
  for (let x = 0; x < w; x++) {
    const f1 = startHz + (x / w) * (endHz - startHz);
    const f2 = startHz + ((x + 1) / w) * (endHz - startHz);
    const b0 = binForFreq(f1, latest.center_hz, latest.sample_rate, n);
    const b1 = binForFreq(f2, latest.center_hz, latest.sample_rate, n);
    const v = maxInBinRange(display, b0, b1);
    const y = h - ((v - minDb) / (maxDb - minDb)) * (h - 18) - 6;
    if (x === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.stroke();
  drawThresholdOverlay(ctx, w, h, minDb, maxDb);

  if (Array.isArray(latest.signals)) {
    latest.signals.forEach((s, index) => {
      const left = s.center_hz - s.bw_hz / 2;
      const right = s.center_hz + s.bw_hz / 2;
      if (right < startHz || left > endHz) return;
      const x1 = ((left - startHz) / (endHz - startHz)) * w;
      const x2 = ((right - startHz) / (endHz - startHz)) * w;
      const boxW = Math.max(2, x2 - x1);
      const primaryMode = getSignalPrimaryMode(s);
      const mc = modColor(primaryMode);
      const rdsName = s.class?.pll?.rds_station || '';
      const runtimeInfo = getSignalRuntimeSummary(s);

      // Signal box with modulation-based color
      ctx.fillStyle = modColorStr(primaryMode, 0.10);
      ctx.strokeStyle = modColorStr(primaryMode, 0.75);
      ctx.lineWidth = 1.5;
      ctx.fillRect(x1, 10, boxW, h - 28);
      ctx.strokeRect(x1, 10, boxW, h - 28);

      if (matchesListenTarget(s)) {
        ctx.strokeStyle = 'rgba(255, 92, 92, 0.95)';
        ctx.lineWidth = 2.5;
        ctx.strokeRect(x1 - 1, 9, boxW + 2, h - 26);
      }

      // Label badges with dark background for readability
      const labelX = Math.max(4, x1 + 4);
      const baseY = 14;
      const freqStr = `${(s.center_hz / 1e6).toFixed(4)} MHz`;

      // Badge background
      const badgeH = rdsName ? 42 : ((primaryMode || runtimeInfo) ? 30 : 16);
      const freqW = ctx.measureText ? 0 : 0; // will measure below
      ctx.font = '11px Inter, sans-serif';
      const line2 = runtimeInfo || primaryMode;
      const textW = Math.max(ctx.measureText(freqStr).width, line2 ? ctx.measureText(line2).width : 0, rdsName ? ctx.measureText(rdsName).width : 0) + 8;
      ctx.fillStyle = 'rgba(7, 16, 24, 0.82)';
      ctx.fillRect(labelX - 3, baseY, textW, badgeH);

      // Line 1: Frequency (teal)
      ctx.fillStyle = 'rgba(102, 240, 209, 0.95)';
      ctx.font = '11px Inter, sans-serif';
      ctx.fillText(freqStr, labelX, baseY + 11);

      // Line 2: runtime info first, then primary mode
      if (runtimeInfo || primaryMode) {
        ctx.fillStyle = mc.label;
        ctx.font = 'bold 10px Inter, sans-serif';
        ctx.fillText(runtimeInfo || primaryMode, labelX, baseY + 23);
      }

      // Line 3: RDS station name (white bold)
      if (rdsName) {
        ctx.fillStyle = 'rgba(255, 255, 255, 0.95)';
        ctx.font = 'bold 12px Inter, sans-serif';
        ctx.fillText(rdsName, labelX, baseY + 36);
      }

      const debugMatch = (latest?.debug?.scores || []).find((d) => Math.abs((d.center_hz || 0) - (s.center_hz || 0)) < Math.max(500, s.bw_hz || 0));
      if (debugMatch?.scores && (!s.class || !s.class.scores)) {
        s.debug_scores = debugMatch.scores;
      }
      liveSignalRects.push({
        x: x1,
        y: 10,
        w: boxW,
        h: h - 28,
        signal: s,
      });
    });
  }
}

function renderWaterfall() {
  if (!latest) return;
  const ctx = getCanvas2DContext(waterfallCanvas);
  const w = waterfallCanvas.width;
  const h = waterfallCanvas.height;
  if (!ctx || w <= 0 || h <= 0) return;

  const display = getProcessedSpectrum();
  if (!display) return;
  const n = display.length;
  const span = latest.sample_rate / zoom;
  const startHz = latest.center_hz - span / 2 + pan * span;
  const endHz = latest.center_hz + span / 2 + pan * span;

  waterfallRangeCache = getRangeCache(waterfallRangeCache, w, startHz, endHz, latest.center_hz, latest.sample_rate, n);
  if (!waterfallRowImageData || waterfallRowImageData.width !== w) {
    waterfallRowImageData = ctx.createImageData(w, 1);
  }

  fillSpectrumRowRGBA(waterfallRowImageData.data, w, waterfallRangeCache, display, latest.center_hz, latest.sample_rate);

  // Ping-pong scroll: shift the front buffer's history down by one row into the
  // back buffer (cross-canvas, no self-draw), stamp the new row at the top, then
  // blit to the visible canvas and swap. See OI-19.
  if (!waterfallBufA || waterfallBufA.width !== w || waterfallBufA.height !== h) {
    waterfallBufA = document.createElement('canvas');
    waterfallBufB = document.createElement('canvas');
    waterfallBufA.width = waterfallBufB.width = w;
    waterfallBufA.height = waterfallBufB.height = h;
    waterfallFront = waterfallBufA;
  }
  const back = (waterfallFront === waterfallBufA) ? waterfallBufB : waterfallBufA;
  const backCtx = back.getContext('2d');
  backCtx.globalCompositeOperation = 'copy'; // replace dest; safe cross-canvas
  if (h > 1) {
    backCtx.drawImage(waterfallFront, 0, 1); // history shifted down one row
  } else {
    backCtx.clearRect(0, 0, w, h);
  }
  backCtx.globalCompositeOperation = 'source-over';
  backCtx.putImageData(waterfallRowImageData, 0, 0); // newest row at top
  waterfallFront = back;

  ctx.save();
  ctx.globalCompositeOperation = 'copy';
  ctx.drawImage(back, 0, 0);
  ctx.restore();

  drawCfarEdgeOverlay(ctx, w, h, startHz, endHz);

  if (Array.isArray(latest.signals)) {
    for (const sig of latest.signals) {
      if (!sig.center_hz) continue;
      const xc = ((sig.center_hz - startHz) / (endHz - startHz)) * w;
      if (xc < 0 || xc > w) continue;
      const mod = sig.class?.mod_type || '';
      ctx.strokeStyle = modColorStr(mod, 0.35);
      ctx.lineWidth = 1;
      ctx.setLineDash([2, 3]);
      ctx.beginPath();
      ctx.moveTo(xc, 0);
      ctx.lineTo(xc, h);
      ctx.stroke();
    }
    ctx.setLineDash([]);
  }
}

function renderOccupancy() {
  const ctx = occupancyCanvas.getContext('2d');
  const w = occupancyCanvas.width;
  const h = occupancyCanvas.height;
  ctx.clearRect(0, 0, w, h);
  ctx.fillStyle = '#071018';
  ctx.fillRect(0, 0, w, h);

  if (!latest || events.length === 0) return;

  const bins = new Array(Math.max(32, Math.min(160, Math.floor(w / 8)))).fill(0);
  const bandStart = latest.center_hz - latest.sample_rate / 2;
  const bandEnd = latest.center_hz + latest.sample_rate / 2;
  const now = Date.now();
  const windowStart = now - timelineWindowMs;

  for (const ev of events) {
    if (ev.end_ms < windowStart || ev.start_ms > now) continue;
    const left = ev.center_hz - ev.bandwidth_hz / 2;
    const right = ev.center_hz + ev.bandwidth_hz / 2;
    const normL = Math.max(0, Math.min(1, (left - bandStart) / (bandEnd - bandStart)));
    const normR = Math.max(0, Math.min(1, (right - bandStart) / (bandEnd - bandStart)));
    let b0 = Math.floor(normL * bins.length);
    let b1 = Math.floor(normR * bins.length);
    if (b1 < b0) [b0, b1] = [b1, b0];
    for (let i = Math.max(0, b0); i <= Math.min(bins.length - 1, b1); i++) {
      bins[i] += Math.max(0.3, (ev.snr_db || 0) / 12 + 1);
    }
  }

  const maxBin = Math.max(1, ...bins);
  bins.forEach((v, i) => {
    const norm = v / maxBin;
    const [r, g, b] = colorMap(norm);
    ctx.fillStyle = `rgb(${r}, ${g}, ${b})`;
    const x = (i / bins.length) * w;
    const bw = Math.ceil(w / bins.length) + 1;
    ctx.fillRect(x, 0, bw, h);
  });
}

function renderTimeline() {
  const ctx = timelineCanvas.getContext('2d');
  const w = timelineCanvas.width;
  const h = timelineCanvas.height;
  ctx.clearRect(0, 0, w, h);
  ctx.fillStyle = '#071018';
  ctx.fillRect(0, 0, w, h);

  if (events.length === 0) {
    timelineRangeEl.textContent = 'No events yet';
    return;
  }

  const endMs = Date.now();
  const startMs = endMs - timelineWindowMs;
  timelineRangeEl.textContent = `${new Date(startMs).toLocaleTimeString()} - ${new Date(endMs).toLocaleTimeString()}`;

  let minHz = Infinity;
  let maxHz = -Infinity;
  if (latest) {
    minHz = latest.center_hz - latest.sample_rate / 2;
    maxHz = latest.center_hz + latest.sample_rate / 2;
  } else {
    for (const ev of events) {
      minHz = Math.min(minHz, ev.center_hz - ev.bandwidth_hz / 2);
      maxHz = Math.max(maxHz, ev.center_hz + ev.bandwidth_hz / 2);
    }
  }
  if (!isFinite(minHz) || !isFinite(maxHz) || minHz === maxHz) {
    minHz = 0;
    maxHz = 1;
  }

  ctx.strokeStyle = 'rgba(86, 109, 148, 0.18)';
  ctx.lineWidth = 1;
  for (let i = 1; i < 6; i++) {
    const y = (h / 6) * i;
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(w, y);
    ctx.stroke();
  }
  for (let i = 1; i < 8; i++) {
    const x = (w / 8) * i;
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, h);
    ctx.stroke();
  }

  timelineRects = [];
  for (const ev of events) {
    if (ev.end_ms < startMs || ev.start_ms > endMs) continue;

    const x1 = ((Math.max(ev.start_ms, startMs) - startMs) / (endMs - startMs)) * w;
    const x2 = ((Math.min(ev.end_ms, endMs) - startMs) / (endMs - startMs)) * w;
    const topHz = ev.center_hz + ev.bandwidth_hz / 2;
    const bottomHz = ev.center_hz - ev.bandwidth_hz / 2;
    const y1 = ((maxHz - topHz) / (maxHz - minHz)) * h;
    const y2 = ((maxHz - bottomHz) / (maxHz - minHz)) * h;

    const rect = { x: x1, y: y1, w: Math.max(2, x2 - x1), h: Math.max(3, y2 - y1), id: ev.id };
    timelineRects.push(rect);

    ctx.fillStyle = snrColor(ev.snr_db || 0).replace('rgb', 'rgba').replace(')', ', 0.85)');
    ctx.fillRect(rect.x, rect.y, rect.w, rect.h);
  }

  if (selectedEventId) {
    const hit = timelineRects.find(r => r.id === selectedEventId);
    if (hit) {
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 2;
      ctx.strokeRect(hit.x - 1, hit.y - 1, hit.w + 2, hit.h + 2);
    }
  }
}

function renderDetailSpectrogram() {
  const ev = eventsById.get(selectedEventId);
  const ctx = getCanvas2DContext(detailSpectrogram);
  const w = detailSpectrogram.width;
  const h = detailSpectrogram.height;
  if (!ctx || w <= 0 || h <= 0) return;
  ctx.clearRect(0, 0, w, h);
  ctx.fillStyle = '#071018';
  ctx.fillRect(0, 0, w, h);
  if (!latest || !ev) return;

  const display = getProcessedSpectrum();
  if (!display) return;
  const n = display.length;
  const localSpan = Math.min(latest.sample_rate, Math.max(ev.bandwidth_hz * 4, latest.sample_rate / 10));
  const startHz = ev.center_hz - localSpan / 2;
  const endHz = ev.center_hz + localSpan / 2;

  detailRangeCache = getRangeCache(detailRangeCache, w, startHz, endHz, latest.center_hz, latest.sample_rate, n);
  ensureDetailRowCanvas(w);
  if (!detailRowImageData || detailRowImageData.width !== w) {
    detailRowImageData = detailRowCtx.createImageData(w, 1);
  }
  fillSpectrumRowRGBA(detailRowImageData.data, w, detailRangeCache, display, latest.center_hz, latest.sample_rate);
  detailRowCtx.putImageData(detailRowImageData, 0, 0);
  ctx.imageSmoothingEnabled = false;
  ctx.drawImage(detailRowCanvas, 0, 0, w, 1, 0, 0, w, h);

  const centerX = w / 2;
  ctx.strokeStyle = 'rgba(255,255,255,0.65)';
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(centerX, 0);
  ctx.lineTo(centerX, h);
  ctx.stroke();
}

let _lastEventListKey = '';

function updateSignalDecisionSummary(id) {
  if (!signalDecisionSummary) return;
  if (!id) {
    signalDecisionSummary.textContent = 'Decision: -';
    return;
  }
  const dec = decisionIndex.get(String(id));
  if (!dec) {
    signalDecisionSummary.textContent = 'Decision: -';
    return;
  }
  const flags = `${dec.record ? 'REC' : ''}${dec.decode ? (dec.record ? '+DEC' : 'DEC') : ''}` || 'none';
  const reason = dec.reason ? ` · ${dec.reason}` : '';
  signalDecisionSummary.textContent = `Decision: ${flags}${reason}`;
}

function updateSignalQueueSummary() {
  if (!signalQueueSummary) return;
  const queue = refinementInfo?.arbitration?.queue;
  if (!queue) {
    signalQueueSummary.textContent = 'Queue: -';
    return;
  }
  signalQueueSummary.textContent = `Queue: rec ${queue.record_queued || 0} / dec ${queue.decode_queued || 0}`;
}

function setSelectedSignal(sel) {
  window._selectedSignal = sel || null;
  signalList.querySelectorAll('.signal-item').forEach((el) => {
    const active = !!sel && ((sel.key && el.dataset.key === sel.key) || (!sel.key && sel.id && el.dataset.id === String(sel.id)));
    el.classList.toggle('active', active);
  });
  updateSignalDecisionSummary(window._selectedSignal?.id);
  updateSignalQueueSummary();
}

function getSignalDomKey(s) {
  if (s?.id != null && s.id !== '') return `id:${s.id}`;
  const center = Math.round(Number(s?.center_hz || 0));
  const bw = Math.round(Number(s?.bw_hz || 0));
  const mode = getSignalPrimaryMode(s) || s?.class?.mod_type || 'UNK';
  const rds = s?.class?.pll?.rds_station || '';
  return `sig:${center}:${bw}:${mode}:${rds}`;
}

function _createSignalItem(s) {
  const btn = document.createElement('button');
  btn.className = 'list-item signal-item';
  btn.type = 'button';
  btn.dataset.center = s.center_hz;
  btn.dataset.bw = s.bw_hz || 0;
  btn.dataset.class = s.class?.mod_type || '';
  btn.dataset.id = s.id ?? '';
  btn.dataset.key = getSignalDomKey(s);
  const primaryMode = getSignalPrimaryMode(s);
  const runtimeInfo = getSignalRuntimeSummary(s);
  const mc = modColor(primaryMode);
  const rds = s.class?.pll?.rds_station || '';
  const dec = decisionIndex.get(String(s.id || 0));
  const decText = dec?.reason ? `${dec.reason}` : '';
  const decFlags = dec ? `${dec.record ? 'REC' : ''}${dec.decode ? (dec.record ? '+DEC' : 'DEC') : ''}` : '';
  const metaBits = [];
  if (decFlags) metaBits.push(decFlags);
  if (decText) metaBits.push(decText);
  if (runtimeInfo) metaBits.push(runtimeInfo);
  btn.title = metaBits.join(' · ');
  btn.innerHTML = `<div class="item-top"><span class="item-title" data-field="freq">${fmtMHz(s.center_hz, 6)}</span><span class="item-badge" data-field="snr" style="color:${snrColor(s.snr_db || 0)}">${(s.snr_db || 0).toFixed(1)} dB</span></div><div class="item-bottom"><span class="item-meta item-meta--runtime" data-field="mode" style="color:${mc.label}">${primaryMode}</span>${rds ? `<span class="item-meta item-meta--rds" data-field="rds">${rds}</span>` : ''}</div>`;
  btn.style.borderLeftColor = mc.label;
  btn.style.borderLeftWidth = '3px';
  btn.style.borderLeftStyle = 'solid';
  if (matchesListenTarget(s)) btn.classList.add('listening');
  return btn;
}

function _patchSignalItem(el, s) {
  const freqEl = el.querySelector('[data-field="freq"]');
  const snrEl = el.querySelector('[data-field="snr"]');
  const modeEl = el.querySelector('[data-field="mode"]');
  const mod = s.class?.mod_type || '';
  const primaryMode = getSignalPrimaryMode(s);
  const runtimeInfo = getSignalRuntimeSummary(s);
  const mc = modColor(primaryMode);
  const rds = s.class?.pll?.rds_station || '';
  const rdsEl = el.querySelector('[data-field="rds"]');
  const dec = decisionIndex.get(String(s.id || 0));
  const decText = dec?.reason ? `${dec.reason}` : '';
  const decFlags = dec ? `${dec.record ? 'REC' : ''}${dec.decode ? (dec.record ? '+DEC' : 'DEC') : ''}` : '';
  const metaBits = [];
  if (decFlags) metaBits.push(decFlags);
  if (decText) metaBits.push(decText);
  if (runtimeInfo) metaBits.push(runtimeInfo);
  el.title = metaBits.join(' · ');
  if (freqEl) freqEl.textContent = fmtMHz(s.center_hz, 6);
  if (snrEl) { snrEl.textContent = `${(s.snr_db || 0).toFixed(1)} dB`; snrEl.style.color = snrColor(s.snr_db || 0); }
  if (modeEl) { modeEl.textContent = primaryMode; modeEl.style.color = mc.label; }
  if (rdsEl) {
    rdsEl.textContent = rds;
    rdsEl.style.display = rds ? '' : 'none';
  } else if (rds) {
    const span = document.createElement('span');
    span.className = 'item-meta item-meta--rds';
    span.dataset.field = 'rds';
    span.textContent = rds;
    el.querySelector('.item-bottom')?.appendChild(span);
  }
  el.dataset.center = s.center_hz;
  el.dataset.bw = s.bw_hz || 0;
  el.dataset.class = mod;
  el.dataset.id = s.id ?? '';
  el.dataset.key = getSignalDomKey(s);
  el.style.borderLeftColor = mc.label;
  el.classList.toggle('listening', matchesListenTarget(s));
}

function renderListsNow() {
  const signals = Array.isArray(latest?.signals) ? [...latest.signals] : [];
  signals.sort((a, b) => (b.snr_db || 0) - (a.snr_db || 0));
  signalCountBadge.textContent = `${signals.length} live`;
  metricSignals.textContent = String(signals.length);

  const displaySigs = signals.slice(0, 24);
  const strongest = displaySigs[0] || null;
  const selectedSignal = window._selectedSignal || null;
  if (signalSummaryLine) {
    const strongestText = strongest
      ? `${fmtMHz(strongest.center_hz, 4)} · ${(strongest.snr_db || 0).toFixed(1)} dB`
      : '-';
    const selectedText = selectedSignal && Number.isFinite(selectedSignal.freq)
      ? `${fmtMHz(selectedSignal.freq, 4)}${selectedSignal.mode ? ` · ${selectedSignal.mode}` : ''}`
      : '-';
    signalSummaryLine.textContent = `Visible: ${displaySigs.length} · Strongest: ${strongestText} · Selected: ${selectedText}`;
  }

  if (displaySigs.length === 0) {
    signalList.innerHTML = '<div class="empty-state">No live signals yet.</div>';
  } else {
    const existing = new Map();
    signalList.querySelectorAll('.signal-item').forEach((el) => existing.set(el.dataset.key, el));
    const frag = document.createDocumentFragment();

    displaySigs.forEach((s) => {
      const key = getSignalDomKey(s);
      let el = existing.get(key);
      if (el) {
        _patchSignalItem(el, s);
      } else {
        el = _createSignalItem(s);
      }

      if (window._selectedSignal) {
        const selectedKey = window._selectedSignal.key;
        const selectedId = window._selectedSignal.id;
        const sameKey = selectedKey && key === selectedKey;
        const sameId = !selectedKey && selectedId && s.id != null && String(s.id) === String(selectedId);
        const nearFreq = !selectedKey && !selectedId && Number.isFinite(window._selectedSignal.freq) && Math.abs(s.center_hz - window._selectedSignal.freq) < 2500;
        el.classList.toggle('active', !!(sameKey || sameId || nearFreq));
      } else {
        el.classList.remove('active');
      }

      frag.appendChild(el);
      existing.delete(key);
    });

    signalList.innerHTML = '';
    signalList.appendChild(frag);
  }

  const recent = [...events].sort((a, b) => b.end_ms - a.end_ms);
  eventCountBadge.textContent = `${recent.length} stored`;

  const evtKey = `${recent.length}:${selectedEventId}:${recent.slice(0, 5).map(e => e.id).join(',')}`;
  if (evtKey !== _lastEventListKey) {
    _lastEventListKey = evtKey;
    if (recent.length === 0) {
      eventList.innerHTML = '<div class="empty-state">No events yet.</div>';
    } else {
      eventList.innerHTML = recent.slice(0, 40).map((ev) => `
      <button class="list-item event-item ${selectedEventId === ev.id ? 'active' : ''}" type="button" data-event-id="${ev.id}">
        <div class="item-top">
          <span class="item-title">${fmtMHz(ev.center_hz, 6)}</span>
          <span class="item-badge" style="color:${snrColor(ev.snr_db || 0)}">${(ev.snr_db || 0).toFixed(1)} dB</span>
        </div>
        <div class="item-bottom">
          <span class="item-meta">${fmtKHz(ev.bandwidth_hz || 0)} · ${fmtMs(ev.duration_ms || 0)}</span>
          <span class="item-meta">${new Date(ev.end_ms).toLocaleTimeString()}</span>
        </div>
      </button>
    `).join('');
    }
  }

  if (recordingList && recordingCountBadge) {
    recordingCountBadge.textContent = `${recordings.length}`;
    if (recordings.length === 0) {
      recordingList.innerHTML = '<div class="empty-state">No recordings yet.</div>';
    } else {
      recordingList.innerHTML = recordings.slice(0, 50).map((rec) => `
        <button class="list-item recording-item" type="button" data-id="${rec.id}">
          <div class="item-top">
            <span class="item-title">${new Date(rec.start).toLocaleString()}</span>
            <span class="item-badge">${fmtMHz(rec.center_hz || 0, 6)}</span>
          </div>
          <div class="item-bottom">
            <span class="item-meta">${rec.id}</span>
            <span class="item-meta">recording</span>
          </div>
        </button>
      `).join('');
    }
  }

  updateSignalDecisionSummary(window._selectedSignal?.id);
}

function flushLists(now, force = false) {
  if (!pendingListRender) return;
  if (!force && now - lastListRenderTs < LIST_RENDER_INTERVAL_MS) return;
  pendingListRender = false;
  lastListRenderTs = now;
  renderListsNow();
}

function renderLists(force = false) {
  pendingListRender = true;
  if (force) flushLists(performance.now(), true);
}

function normalizeEvent(ev) {
  const startMs = new Date(ev.start).getTime();
  const endMs = new Date(ev.end).getTime();
  return {
    ...ev,
    start_ms: startMs,
    end_ms: endMs,
    duration_ms: Math.max(0, endMs - startMs),
  };
}

function upsertEvents(list, replace = false) {
  if (replace) {
    events.length = 0;
    eventsById.clear();
  }
  for (const raw of list) {
    if (!raw || !raw.id || eventsById.has(raw.id)) continue;
    const ev = normalizeEvent(raw);
    eventsById.set(ev.id, ev);
    events.push(ev);
  }
  events.sort((a, b) => a.end_ms - b.end_ms);
  const maxEvents = 1500;
  if (events.length > maxEvents) {
    const drop = events.length - maxEvents;
    for (let i = 0; i < drop; i++) eventsById.delete(events[i].id);
    events.splice(0, drop);
  }
  if (events.length > 0) lastEventEndMs = events[events.length - 1].end_ms;
  updateHeroMetrics();
  renderLists();
}

async function fetchEvents(initial) {
  if (eventsFetchInFlight || timelineFrozen || !apiClient) return;
  eventsFetchInFlight = true;
  try {
    const query = initial
      ? { limit: 1000 }
      : (lastEventEndMs > 0 ? { since: lastEventEndMs - 1 } : { limit: 200 });
    const res = await apiClient.getEvents(query);
    updateApiState(res);
    if (res.ok && Array.isArray(res.data)) upsertEvents(res.data, initial);
  } finally {
    eventsFetchInFlight = false;
  }
}

async function fetchRecordings() {
  if (recordingsFetchInFlight || !recordingList || !apiClient) return;
  recordingsFetchInFlight = true;
  try {
    const res = await apiClient.getRecordings();
    updateApiState(res);
    if (res.ok && Array.isArray(res.data)) {
      recordings = res.data;
      renderLists();
    }
  } finally {
    recordingsFetchInFlight = false;
  }
}

function openDrawer(ev) {
  if (!ev) return;
  selectedEventId = ev.id;
  detailSubtitle.textContent = `Event ${ev.id}`;
  detailCenterEl.textContent = fmtMHz(ev.center_hz, 6);
  detailBwEl.textContent = fmtKHz(ev.bandwidth_hz || 0);
  detailStartEl.textContent = new Date(ev.start_ms).toLocaleString();
  detailEndEl.textContent = new Date(ev.end_ms).toLocaleString();
  detailSnrEl.textContent = `${(ev.snr_db || 0).toFixed(1)} dB`;
  detailDurEl.textContent = fmtMs(ev.duration_ms || 0);
  detailClassEl.textContent = ev.class?.mod_type || '-';
  if (classifierScoresEl) {
    const scores = ev.class?.scores;
    if (scores && typeof scores === 'object') {
      const rows = Object.entries(scores)
        .sort((a, b) => b[1] - a[1])
        .slice(0, 6)
        .map(([k, v]) => `${k}:${v.toFixed(2)}`)
        .join(' · ');
      classifierScoresEl.textContent = rows ? `Classifier scores: ${rows}` : 'Classifier scores: -';
      renderScoreBars(scores);
    } else {
      const liveScores = (latest?.debug?.scores || []).find((s) => Math.abs((s.center_hz || 0) - (ev.center_hz || 0)) < Math.max(500, (ev.bandwidth_hz || 0)));
      if (liveScores?.scores) {
        const rows = Object.entries(liveScores.scores)
          .sort((a, b) => b[1] - a[1])
          .slice(0, 6)
          .map(([k, v]) => `${k}:${Number(v).toFixed(2)}`)
          .join(' · ');
        classifierScoresEl.textContent = rows ? `Classifier scores: ${rows}` : 'Classifier scores: -';
        renderScoreBars(liveScores.scores);
      } else {
        classifierScoresEl.textContent = 'Classifier scores: -';
        renderScoreBars(null);
      }
    }
  }
  if (recordingMetaEl) {
    recordingMetaEl.textContent = 'Recording: -';
  }
  if (recordingMetaLink) {
    recordingMetaLink.href = '#';
    recordingIQLink.href = '#';
    recordingAudioLink.href = '#';
  }
  drawerEl.classList.add('open');
  drawerEl.setAttribute('aria-hidden', 'false');
  renderDetailSpectrogram();
  renderLists(true);
}

function closeDrawer() {
  drawerEl.classList.remove('open');
  drawerEl.setAttribute('aria-hidden', 'true');
  selectedEventId = null;
  renderLists(true);
}

function fitView() {
  zoom = 1;
  pan = 0;
  followLive = true;
}

function tuneToFrequency(centerHz) {
  if (!Number.isFinite(centerHz)) return;
  followLive = true;
  centerInput.value = (centerHz / 1e6).toFixed(6);
  queueConfigUpdate({ center_hz: centerHz });
}

function applyLiveFrame(frame) {
  if (!frame) return;
  const next = frame;
  if (!wsCarriesSignals && Array.isArray(latest?.signals)) {
    next.signals = latest.signals;
  }
  if (!wsCarriesDebug) {
    next.debug = null;
  }
  latest = next;
  pendingWaterfallRender = true;
  updateHeroMetrics();
}

function sendSpectrumWSConfig(update) {
  if (!spectrumWS || spectrumWS.readyState !== WebSocket.OPEN) return;
  try {
    spectrumWS.send(JSON.stringify(update));
  } catch (err) {
    console.warn('ws config update failed:', err);
  }
}

function connect() {
  clearTimeout(wsReconnectTimer);
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';

  // Remote optimization: detect non-localhost and opt into binary + decimation
  const hn = location.hostname;
  const isLocal = ['localhost', '127.0.0.1', '::1'].includes(hn)
    || hn.startsWith('192.168.')
    || hn.startsWith('10.')
    || /^172\.(1[6-9]|2\d|3[01])\./.test(hn)
    || hn.endsWith('.local')
    || hn.endsWith('.lan');
  const params = new URLSearchParams(location.search);
  // Default to the binary spectrum protocol everywhere (incl. localhost): it sends
  // the spectrum as compact int16 instead of JSON-encoding the float64 array every
  // frame, which was the dominant per-frame broadcast allocation (#24). Opt out with
  // ?binary=0 to get the legacy JSON frames for debugging.
  const wantBinary = params.get('binary') !== '0';
  const bins = parseInt(params.get('bins') || (isLocal ? '0' : '2048'), 10);
  const fps = parseInt(params.get('fps') || (isLocal ? '0' : '10'), 10);
  const wantSignals = params.get('signals') === '1';
  const wantDebug = params.get('debug') === '1' || showDebugOverlay;
  wsCarriesSignals = wantSignals;
  wsCarriesDebug = wantDebug;

  let wsUrl = `${proto}://${location.host}/ws`;
  if (wantBinary || bins > 0 || fps > 0 || wantDebug || !wantSignals) {
    const qp = [];
    if (wantBinary) qp.push('binary=1');
    if (bins > 0) qp.push(`bins=${bins}`);
    if (fps > 0) qp.push(`fps=${fps}`);
    if (wantDebug) qp.push('debug=1');
    if (!wantSignals) qp.push('signals=0');
    wsUrl += '?' + qp.join('&');
  }

  const ws = new WebSocket(wsUrl);
  spectrumWS = ws;
  ws.binaryType = 'arraybuffer';
  setWsBadge('Connecting', 'neutral');

  ws.onopen = () => {
    setWsBadge('Live', 'ok');
    wsLastMessageTs = Date.now();
    updateOperatorStatus(true);
  };
  ws.onmessage = (ev) => {
    if (ev.data instanceof ArrayBuffer) {
      try {
        const decoded = decodeBinaryFrame(ev.data);
        applyLiveFrame(decoded);
      } catch (e) {
        console.warn('binary frame decode error:', e);
        return;
      }
    } else {
      applyLiveFrame(JSON.parse(ev.data));
    }
    wsLastMessageTs = Date.now();
    updateOperatorStatus();
    markSpectrumDirty();
    if (followLive) pan = 0;
    renderLists();
  };
  ws.onclose = () => {
    if (spectrumWS === ws) spectrumWS = null;
    setWsBadge('Retrying', 'bad');
    updateOperatorStatus(true);
    wsReconnectTimer = setTimeout(connect, 1000);
  };
  ws.onerror = () => ws.close();
}

// Decode binary spectrum frame v4 (hybrid: binary spectrum + JSON signals)
function decodeBinaryFrame(buf) {
  const view = new DataView(buf);
  if (buf.byteLength < 32) return null;

  // Header: 32 bytes
  const magic0 = view.getUint8(0);
  const magic1 = view.getUint8(1);
  if (magic0 !== 0x53 || magic1 !== 0x50) return null; // not "SP"

  const version = view.getUint16(2, true);
  const ts = Number(view.getBigInt64(4, true));
  const centerHz = view.getFloat64(12, true);
  const binCount = view.getUint32(20, true);
  const sampleRateHz = view.getUint32(24, true);
  const jsonOffset = view.getUint32(28, true);

  if (buf.byteLength < 32 + binCount * 2) return null;

  // Spectrum: binCount × int16 at offset 32
  const spectrum = new Float64Array(binCount);
  let off = 32;
  for (let i = 0; i < binCount; i++) {
    spectrum[i] = view.getInt16(off, true) / 100;
    off += 2;
  }

  // JSON signals + debug after the spectrum data
  let signals = [];
  let debug = null;
  if (jsonOffset > 0 && jsonOffset < buf.byteLength) {
    try {
      const jsonBytes = new Uint8Array(buf, jsonOffset);
      const jsonStr = new TextDecoder().decode(jsonBytes);
      const parsed = JSON.parse(jsonStr);
      signals = parsed.signals || [];
      debug = parsed.debug || null;
    } catch (e) {
      // JSON parse failed — continue with empty signals
    }
  }

  return {
    ts: ts,
    center_hz: centerHz,
    sample_rate: sampleRateHz,
    fft_size: binCount,
    spectrum_db: spectrum,
    signals: signals,
    debug: debug
  };
}

function renderLoop(now) {
  if (document.hidden) {
    requestAnimationFrame(renderLoop);
    return;
  }
  flushOperatorStatus(now);
  flushHeroMetrics(now);
  flushLists(now);

  if (latest && (lastVisualRenderTs === 0 || now - lastVisualRenderTs >= VISUAL_FRAME_INTERVAL_MS)) {
    lastVisualRenderTs = now;
    renderFrames += 1;
    if (now - lastFpsTs >= 1000) {
      renderFps = (renderFrames * 1000) / (now - lastFpsTs);
      renderFrames = 0;
      lastFpsTs = now;
      updateHeroMetrics();
    }

    renderBandNavigator();
    renderSpectrum();
    if (pendingWaterfallRender && (lastWaterfallRenderTs === 0 || now - lastWaterfallRenderTs >= WATERFALL_FRAME_INTERVAL_MS)) {
      renderWaterfall();
      pendingWaterfallRender = false;
      lastWaterfallRenderTs = now;
    }
    renderOccupancy();
    renderTimeline();
    if (drawerEl.classList.contains('open') && (lastDetailRenderTs === 0 || now - lastDetailRenderTs >= DETAIL_RENDER_INTERVAL_MS)) {
      renderDetailSpectrogram();
      lastDetailRenderTs = now;
    }
  }
  requestAnimationFrame(renderLoop);
}

function handleSpectrumClick(ev) {
  const rect = spectrumCanvas.getBoundingClientRect();
  const x = (ev.clientX - rect.left) * (spectrumCanvas.width / rect.width);
  const y = (ev.clientY - rect.top) * (spectrumCanvas.height / rect.height);

  for (let i = liveSignalRects.length - 1; i >= 0; i--) {
    const r = liveSignalRects[i];
    if (x >= r.x && x <= r.x + r.w && y >= r.y && y <= r.y + r.h) {
      const sig = r.signal;
      const freq = sig.center_hz;
      const bw = sig.bw_hz || 12000;
      const mode = sig.class?.mod_type || '';
      startLiveListen(freq, bw, mode);
      setSelectedSignal({
        key: getSignalDomKey(sig),
        id: sig.id ?? null,
        freq,
        bw,
        mode
      });
      return;
    }
  }

  if (!latest) return;
  const span = latest.sample_rate / zoom;
  const startHz = latest.center_hz - span / 2 + pan * span;
  const clickedHz = startHz + (x / spectrumCanvas.width) * span;
  tuneToFrequency(clickedHz);
}

function handleNavPosition(ev) {
  if (!latest) return;
  const rect = navCanvas.getBoundingClientRect();
  const x = Math.max(0, Math.min(rect.width, ev.clientX - rect.left));
  const norm = x / rect.width;
  const fullStart = latest.center_hz - latest.sample_rate / 2;
  const newViewCenter = fullStart + norm * latest.sample_rate;
  const span = latest.sample_rate / zoom;
  const desiredPan = (newViewCenter - latest.center_hz) / span;
  pan = Math.max(-0.5, Math.min(0.5, desiredPan));
  followLive = false;
}

function exportSelectedEvent() {
  const ev = eventsById.get(selectedEventId);
  if (!ev) return;
  const blob = new Blob([JSON.stringify(ev, null, 2)], { type: 'application/json' });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = `event-${ev.id}.json`;
  a.click();
  URL.revokeObjectURL(a.href);
}

spectrumCanvas.addEventListener('wheel', (ev) => {
  ev.preventDefault();
  const direction = Math.sign(ev.deltaY);
  zoom = Math.max(0.25, Math.min(24, zoom * (direction > 0 ? 1.12 : 0.89)));
  followLive = false;
});

spectrumCanvas.addEventListener('mousedown', (ev) => {
  isDraggingSpectrum = true;
  dragStartX = ev.clientX;
  dragStartPan = pan;
});
window.addEventListener('mouseup', () => {
  isDraggingSpectrum = false;
  navDrag = false;
});
spectrumCanvas.addEventListener('mouseleave', hideSignalPopover);
window.addEventListener('mousemove', (ev) => {
  const rect = spectrumCanvas.getBoundingClientRect();
  const x = (ev.clientX - rect.left) * (spectrumCanvas.width / rect.width);
  const y = (ev.clientY - rect.top) * (spectrumCanvas.height / rect.height);
  let hoverHit = null;
  for (let i = liveSignalRects.length - 1; i >= 0; i--) {
    const r = liveSignalRects[i];
    if (x >= r.x && x <= r.x + r.w && y >= r.y && y <= r.y + r.h) {
      hoverHit = r;
      break;
    }
  }
  if (hoverHit) {
    hoveredSignal = hoverHit.signal;
    renderSignalPopover(hoverHit, hoverHit.signal);
  } else {
    hideSignalPopover();
  }
  if (isDraggingSpectrum) {
    const dx = ev.clientX - dragStartX;
    pan = Math.max(-0.5, Math.min(0.5, dragStartPan - dx / spectrumCanvas.clientWidth));
    followLive = false;
  }
  if (navDrag) handleNavPosition(ev);
});

spectrumCanvas.addEventListener('dblclick', fitView);
spectrumCanvas.addEventListener('click', handleSpectrumClick);

navCanvas.addEventListener('mousedown', (ev) => {
  navDrag = true;
  handleNavPosition(ev);
});
navCanvas.addEventListener('click', handleNavPosition);

centerInput.addEventListener('change', () => {
  const mhz = parseFloat(centerInput.value);
  if (Number.isFinite(mhz)) tuneToFrequency(fromMHz(mhz));
});

spanInput.addEventListener('change', () => {
  const mhz = parseFloat(spanInput.value);
  if (!Number.isFinite(mhz) || mhz <= 0) return;
  const baseRate = currentConfig?.sample_rate || latest?.sample_rate;
  if (!baseRate) return;
  zoom = Math.max(0.25, Math.min(24, baseRate / fromMHz(mhz)));
  followLive = false;
});

sampleRateSelect.addEventListener('change', () => {
  const mhz = parseFloat(sampleRateSelect.value);
  if (Number.isFinite(mhz)) queueConfigUpdate({ sample_rate: Math.round(fromMHz(mhz)) });
});

bwSelect.addEventListener('change', () => {
  const bw = parseInt(bwSelect.value, 10);
  if (Number.isFinite(bw)) queueConfigUpdate({ tuner_bw_khz: bw });
});

fftSelect.addEventListener('change', () => {
  const size = parseInt(fftSelect.value, 10);
  if (Number.isFinite(size)) queueConfigUpdate({ fft_size: size });
});

if (welchSegInput) welchSegInput.addEventListener('change', () => {
  const v = parseInt(welchSegInput.value, 10);
  if (Number.isFinite(v) && v >= 0) queueConfigUpdate({ surveillance: { welch_segments: v } });
});

gainRange.addEventListener('input', () => {
  gainInput.value = gainRange.value;
  const uiVal = parseFloat(gainRange.value);
  if (Number.isFinite(uiVal)) queueConfigUpdate({ gain_db: Math.max(0, Math.min(GAIN_MAX, GAIN_MAX - uiVal)) });
});

gainInput.addEventListener('change', () => {
  const uiVal = parseFloat(gainInput.value);
  if (Number.isFinite(uiVal)) {
    gainRange.value = uiVal;
    queueConfigUpdate({ gain_db: Math.max(0, Math.min(GAIN_MAX, GAIN_MAX - uiVal)) });
  }
});

thresholdRange.addEventListener('input', () => {
  thresholdInput.value = thresholdRange.value;
  queueConfigUpdate({ detector: { threshold_db: parseFloat(thresholdRange.value) } });
});
thresholdInput.addEventListener('change', () => {
  const v = parseFloat(thresholdInput.value);
  if (Number.isFinite(v)) {
    thresholdRange.value = v;
    queueConfigUpdate({ detector: { threshold_db: v } });
  }
});

if (cfarModeSelect) cfarModeSelect.addEventListener('change', () => {
  queueConfigUpdate({ detector: { cfar_mode: cfarModeSelect.value } });
  const rankRow = cfarRankInput?.closest('.field');
  if (rankRow) rankRow.style.display = (cfarModeSelect.value === 'OS') ? '' : 'none';
});
if (cfarWrapToggle) cfarWrapToggle.addEventListener('change', () => {
  queueConfigUpdate({ detector: { cfar_wrap_around: cfarWrapToggle.checked } });
});
if (cfarGuardHzInput) cfarGuardHzInput.addEventListener('change', () => {
  const v = parseFloat(cfarGuardHzInput.value);
  if (Number.isFinite(v) && v >= 0) queueConfigUpdate({ detector: { cfar_guard_hz: v } });
});
if (cfarTrainHzInput) cfarTrainHzInput.addEventListener('change', () => {
  const v = parseFloat(cfarTrainHzInput.value);
  if (Number.isFinite(v) && v > 0) queueConfigUpdate({ detector: { cfar_train_hz: v } });
});
if (cfarRankInput) cfarRankInput.addEventListener('change', () => {
  const v = parseInt(cfarRankInput.value, 10);
  if (Number.isFinite(v)) queueConfigUpdate({ detector: { cfar_rank: v } });
});
if (cfarScaleInput) cfarScaleInput.addEventListener('change', () => {
  const v = parseFloat(cfarScaleInput.value);
  if (Number.isFinite(v)) queueConfigUpdate({ detector: { cfar_scale_db: v } });
});
if (minDurationInput) minDurationInput.addEventListener('change', () => {
  const v = parseInt(minDurationInput.value, 10);
  if (Number.isFinite(v)) queueConfigUpdate({ detector: { min_duration_ms: v } });
});
if (holdInput) holdInput.addEventListener('change', () => {
  const v = parseInt(holdInput.value, 10);
  if (Number.isFinite(v)) queueConfigUpdate({ detector: { hold_ms: v } });
});
if (emaAlphaInput) emaAlphaInput.addEventListener('change', () => {
  const v = parseFloat(emaAlphaInput.value);
  if (Number.isFinite(v)) queueConfigUpdate({ detector: { ema_alpha: v } });
});
if (hysteresisInput) hysteresisInput.addEventListener('change', () => {
  const v = parseFloat(hysteresisInput.value);
  if (Number.isFinite(v)) queueConfigUpdate({ detector: { hysteresis_db: v } });
});
if (stableFramesInput) stableFramesInput.addEventListener('change', () => {
  const v = parseInt(stableFramesInput.value, 10);
  if (Number.isFinite(v)) queueConfigUpdate({ detector: { min_stable_frames: v } });
});
if (gapToleranceInput) gapToleranceInput.addEventListener('change', () => {
  const v = parseInt(gapToleranceInput.value, 10);
  if (Number.isFinite(v)) queueConfigUpdate({ detector: { gap_tolerance_ms: v } });
});
if (classifierModeSelect) classifierModeSelect.addEventListener('change', () => {
  queueConfigUpdate({ classifier_mode: classifierModeSelect.value });
});
if (edgeMarginInput) edgeMarginInput.addEventListener('change', () => {
  const v = parseFloat(edgeMarginInput.value);
  if (Number.isFinite(v) && v >= 0) queueConfigUpdate({ detector: { edge_margin_db: v } });
});
if (mergeGapInput) mergeGapInput.addEventListener('change', () => {
  const v = parseFloat(mergeGapInput.value);
  if (Number.isFinite(v)) queueConfigUpdate({ detector: { merge_gap_hz: v } });
});
if (classHistoryInput) classHistoryInput.addEventListener('change', () => {
  const v = parseInt(classHistoryInput.value, 10);
  if (Number.isFinite(v) && v >= 1) queueConfigUpdate({ detector: { class_history_size: v } });
});
if (classSwitchInput) classSwitchInput.addEventListener('change', () => {
  const v = parseFloat(classSwitchInput.value);
  if (Number.isFinite(v) && v >= 0.1 && v <= 1.0) queueConfigUpdate({ detector: { class_switch_ratio: v } });
});

agcToggle.addEventListener('change', () => queueSettingsUpdate({ agc: agcToggle.checked }));
dcToggle.addEventListener('change', () => queueSettingsUpdate({ dc_block: dcToggle.checked }));
iqToggle.addEventListener('change', () => queueSettingsUpdate({ iq_balance: iqToggle.checked }));
gpuToggle.addEventListener('change', () => queueConfigUpdate({ use_gpu_fft: gpuToggle.checked }));
if (recEnableToggle) recEnableToggle.addEventListener('change', () => queueConfigUpdate({ recorder: { enabled: recEnableToggle.checked } }));
if (recIQToggle) recIQToggle.addEventListener('change', () => queueConfigUpdate({ recorder: { record_iq: recIQToggle.checked } }));
if (recAudioToggle) recAudioToggle.addEventListener('change', () => queueConfigUpdate({ recorder: { record_audio: recAudioToggle.checked } }));
if (recDemodToggle) recDemodToggle.addEventListener('change', () => queueConfigUpdate({ recorder: { auto_demod: recDemodToggle.checked } }));
if (recDecodeToggle) recDecodeToggle.addEventListener('change', () => queueConfigUpdate({ recorder: { auto_decode: recDecodeToggle.checked } }));
if (recMinSNR) recMinSNR.addEventListener('change', () => queueConfigUpdate({ recorder: { min_snr_db: parseFloat(recMinSNR.value) } }));
if (recMaxDisk) recMaxDisk.addEventListener('change', () => queueConfigUpdate({ recorder: { max_disk_mb: parseInt(recMaxDisk.value || '0', 10) } }));
if (recClassFilter) recClassFilter.addEventListener('change', () => {
  const list = (recClassFilter.value || '')
    .split(',')
    .map(s => s.trim())
    .filter(Boolean);
  queueConfigUpdate({ recorder: { class_filter: list } });
});
if (refineAutoSpan) refineAutoSpan.addEventListener('change', () => {
  queueConfigUpdate({ refinement: { auto_span: refineAutoSpan.value === 'true' } });
});
if (refineMinSpan) refineMinSpan.addEventListener('change', () => {
  const v = parseFloat(refineMinSpan.value);
  if (Number.isFinite(v)) queueConfigUpdate({ refinement: { min_span_hz: v } });
});
if (refineMaxSpan) refineMaxSpan.addEventListener('change', () => {
  const v = parseFloat(refineMaxSpan.value);
  if (Number.isFinite(v)) queueConfigUpdate({ refinement: { max_span_hz: v } });
});
if (resMaxRefine) resMaxRefine.addEventListener('change', () => {
  const v = parseInt(resMaxRefine.value || '0', 10);
  queueConfigUpdate({ resources: { max_refinement_jobs: v } });
});
if (resMaxRecord) resMaxRecord.addEventListener('change', () => {
  const v = parseInt(resMaxRecord.value || '0', 10);
  queueConfigUpdate({ resources: { max_recording_streams: v } });
});
if (resMaxDecode) resMaxDecode.addEventListener('change', () => {
  const v = parseInt(resMaxDecode.value || '0', 10);
  queueConfigUpdate({ resources: { max_decode_jobs: v } });
});
if (resDecisionHold) resDecisionHold.addEventListener('change', () => {
  const v = parseInt(resDecisionHold.value || '0', 10);
  queueConfigUpdate({ resources: { decision_hold_ms: v } });
});

avgSelect.addEventListener('change', () => {
  avgAlpha = parseFloat(avgSelect.value) || 0;
  resetProcessingCaches();
});
maxHoldToggle.addEventListener('change', () => {
  maxHold = maxHoldToggle.checked;
  maxSpectrum = null;
  markSpectrumDirty();
});
if (debugOverlayToggle) debugOverlayToggle.addEventListener('change', () => {
  showDebugOverlay = debugOverlayToggle.checked;
  wsCarriesDebug = showDebugOverlay;
  localStorage.setItem('spectre.debugOverlay', showDebugOverlay ? '1' : '0');
  if (!showDebugOverlay && latest) latest.debug = null;
  sendSpectrumWSConfig({ debug: showDebugOverlay });
  markSpectrumDirty();
  updateHeroMetrics(true);
});
resetMaxBtn.addEventListener('click', () => {
  maxSpectrum = null;
  markSpectrumDirty();
});
followBtn.addEventListener('click', () => { followLive = true; pan = 0; });
fitBtn.addEventListener('click', fitView);
timelineFollowBtn.addEventListener('click', () => { timelineFrozen = false; });
timelineFreezeBtn.addEventListener('click', () => {
  timelineFrozen = !timelineFrozen;
  timelineFreezeBtn.textContent = timelineFrozen ? 'Frozen' : 'Freeze';
});

presetButtons.forEach((btn) => {
  btn.addEventListener('click', () => {
    const mhz = parseFloat(btn.dataset.center);
    if (Number.isFinite(mhz)) tuneToFrequency(fromMHz(mhz));
  });
});

function activateRailTab(tabName) {
  railTabs.forEach((t) => {
    const active = t.dataset.tab === tabName;
    t.classList.toggle('active', active);
    t.setAttribute('aria-selected', active ? 'true' : 'false');
  });
  tabPanels.forEach((panel) => {
    const active = panel.dataset.panel === tabName;
    panel.classList.toggle('active', active);
    panel.hidden = !active;
    panel.setAttribute('aria-hidden', active ? 'false' : 'true');
  });
}

railTabs.forEach((tab) => {
  tab.addEventListener('click', () => activateRailTab(tab.dataset.tab));
});

activateRailTab((railTabs.find((t) => t.classList.contains('active')) || railTabs[0])?.dataset.tab || 'radio');


drawerCloseBtn.addEventListener('click', closeDrawer);
exportEventBtn.addEventListener('click', exportSelectedEvent);
if (liveListenEventBtn) {
  liveListenEventBtn.addEventListener('click', () => {
    const ev = eventsById.get(selectedEventId);
    if (!ev) return;

    // Toggle off if already listening
    if (liveListenWS && liveListenWS.playing) {
      stopLiveListen();
      return;
    }

    const freq = ev.center_hz;
    const bw = ev.bandwidth_hz || 12000;
    const mode = ev.class?.mod_type || 'NFM';
    startLiveListen(freq, bw, mode);
  });
}
if (decodeEventBtn) {
  decodeEventBtn.addEventListener('click', async () => {
    const ev = eventsById.get(selectedEventId);
    if (!ev) return;
    if (!recordingMetaEl) return;
    const rec = recordings.find(r => r.event_id === ev.id) || recordings.find(r => r.center_hz === ev.center_hz);
    if (!rec) {
      decodeResultEl.textContent = 'Decode: no recording';
      return;
    }
    const mode = decodeModeSelect?.value || ev.class?.mod_type || 'FT8';
    if (!apiClient) {
      decodeResultEl.textContent = 'Decode: failed';
      return;
    }
    const res = await apiClient.decodeRecording(rec.id, mode);
    updateApiState(res);
    if (!res.ok || !res.data) {
      decodeResultEl.textContent = 'Decode: failed';
      return;
    }
    decodeResultEl.textContent = `Decode: ${String(res.data.stdout || '').slice(0, 80)}`;
  });
}
jumpToEventBtn.addEventListener('click', () => {
  const ev = eventsById.get(selectedEventId);
  if (!ev) return;
  tuneToFrequency(ev.center_hz);
});

timelineCanvas.addEventListener('click', (ev) => {
  const rect = timelineCanvas.getBoundingClientRect();
  const x = (ev.clientX - rect.left) * (timelineCanvas.width / rect.width);
  const y = (ev.clientY - rect.top) * (timelineCanvas.height / rect.height);
  for (let i = timelineRects.length - 1; i >= 0; i--) {
    const r = timelineRects[i];
    if (x >= r.x && x <= r.x + r.w && y >= r.y && y <= r.y + r.h) {
      openDrawer(eventsById.get(r.id));
      return;
    }
  }
});

signalList.addEventListener('click', (ev) => {
  const target = ev.target.closest('.signal-item');
  if (!target) return;
  // Select this signal for live listening — don't retune the SDR
  setSelectedSignal({
    key: target.dataset.key || null,
    id: target.dataset.id || null,
    freq: parseFloat(target.dataset.center),
    bw: parseFloat(target.dataset.bw || '12000'),
    mode: target.dataset.class || ''
  });
});

if (liveListenBtn) {
  liveListenBtn.addEventListener('click', async () => {
    // Toggle: if already listening, stop
    if (liveListenWS && liveListenWS.playing) {
      stopLiveListen();
      return;
    }

    // Use selected signal if available, otherwise first in list
    let freq, bw, mode;
    if (window._selectedSignal) {
      freq = window._selectedSignal.freq;
      bw = window._selectedSignal.bw;
      mode = window._selectedSignal.mode;
    } else {
      const first = signalList.querySelector('.signal-item');
      if (!first) return;
      freq = parseFloat(first.dataset.center);
      bw = parseFloat(first.dataset.bw || '12000');
      mode = first.dataset.class || '';
    }
    if (!Number.isFinite(freq)) return;

    startLiveListen(freq, bw, mode);
  });
}

eventList.addEventListener('click', (ev) => {
  const target = ev.target.closest('.event-item');
  if (!target) return;
  const id = target.dataset.eventId;
  openDrawer(eventsById.get(id));
});

if (recordingList) {
  recordingList.addEventListener('click', async (ev) => {
    const target = ev.target.closest('.recording-item');
    if (!target) return;
    const id = target.dataset.id;
    const audio = new Audio(`/api/recordings/${id}/audio`);
    audio.play();
    if (recordingMetaEl) recordingMetaEl.textContent = `Recording: ${id}`;
    if (recordingMetaLink) {
      recordingMetaLink.href = `/api/recordings/${id}`;
      recordingIQLink.href = `/api/recordings/${id}/iq`;
      recordingAudioLink.href = `/api/recordings/${id}/audio`;
    }
    try {
      if (!apiClient) return;
      const res = await apiClient.getRecording(id);
      updateApiState(res);
      if (!res.ok || !res.data) return;
      const meta = res.data;
      if (decodeResultEl) {
        const rds = meta.rds_ps ? `RDS: ${meta.rds_ps}` : '';
        decodeResultEl.textContent = `Decode: ${rds}`;
      }
      if (classifierScoresEl) {
        const scores = meta.classification?.scores;
        if (scores && typeof scores === 'object') {
          const rows = Object.entries(scores)
            .sort((a, b) => b[1] - a[1])
            .slice(0, 6)
            .map(([k, v]) => `${k}:${v.toFixed(2)}`)
            .join(' · ');
          classifierScoresEl.textContent = rows ? `Classifier scores: ${rows}` : 'Classifier scores: -';
        } else {
          classifierScoresEl.textContent = 'Classifier scores: -';
        }
      }
    } catch {}
  });
}

if (debugOverlayToggle) debugOverlayToggle.checked = showDebugOverlay;

window.addEventListener('keydown', (ev) => {
  if (ev.target && ['INPUT', 'SELECT', 'TEXTAREA'].includes(ev.target.tagName)) return;
  if (ev.key === ' ') {
    ev.preventDefault();
    followLive = true;
    pan = 0;
  } else if (ev.key.toLowerCase() === 'f') {
    fitView();
  } else if (ev.key.toLowerCase() === 'm') {
    maxHold = !maxHold;
    maxHoldToggle.checked = maxHold;
    if (!maxHold) maxSpectrum = null;
    markSpectrumDirty();
  } else if (ev.key.toLowerCase() === 'g') {
    gpuToggle.checked = !gpuToggle.checked;
    queueConfigUpdate({ use_gpu_fft: gpuToggle.checked });
  } else if (ev.key === '[') {
    zoom = Math.max(0.25, zoom * 0.88);
  } else if (ev.key === ']') {
    zoom = Math.min(24, zoom * 1.12);
  } else if (ev.key === 'ArrowLeft') {
    pan = Math.max(-0.5, pan - 0.04);
    followLive = false;
  } else if (ev.key === 'ArrowRight') {
    pan = Math.min(0.5, pan + 0.04);
    followLive = false;
  }
});

updateOperatorStatus(true);
loadConfig();
resetLiveListenMeta();
loadStats();
loadGPU();
loadRefinement();
loadTelemetryLive();
loadPolicy();
fetchEvents(true);
fetchRecordings();
loadDecoders();
connect();
requestAnimationFrame(renderLoop);
setInterval(loadStats, 1000);
setInterval(loadGPU, 1000);
setInterval(loadRefinement, 1500);
setInterval(loadTelemetryLive, 3000);
setInterval(loadPolicy, 10000);
setInterval(() => fetchEvents(false), 2000);
setInterval(fetchRecordings, 5000);
setInterval(loadSignals, 1500);
setInterval(loadDecoders, 10000);
