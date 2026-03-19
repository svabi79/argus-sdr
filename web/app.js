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
const classifierModeSelect = qs('classifierModeSelect');
const cfarModeSelect = qs('cfarModeSelect');
const cfarWrapToggle = qs('cfarWrapToggle');
const cfarGuardHzInput = qs('cfarGuardHzInput');
const cfarTrainHzInput = qs('cfarTrainHzInput');
const cfarRankInput = qs('cfarRankInput');
const cfarScaleInput = qs('cfarScaleInput');
const minDurationInput = qs('minDurationInput');
const holdInput = qs('holdInput');
const emaAlphaInput = qs('emaAlphaInput');
const hysteresisInput = qs('hysteresisInput');
const stableFramesInput = qs('stableFramesInput');
const gapToleranceInput = qs('gapToleranceInput');
const edgeMarginInput = qs('edgeMarginInput');
const mergeGapInput = qs('mergeGapInput');
const classHistoryInput = qs('classHistoryInput');
const classSwitchInput = qs('classSwitchInput');
const agcToggle = qs('agcToggle');
const dcToggle = qs('dcToggle');
const iqToggle = qs('iqToggle');
const avgSelect = qs('avgSelect');
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

const signalList = qs('signalList');
const eventList = qs('eventList');
const recordingList = qs('recordingList');
const signalCountBadge = qs('signalCountBadge');
const eventCountBadge = qs('eventCountBadge');
const recordingCountBadge = qs('recordingCountBadge');

const healthBuffer = qs('healthBuffer');
const healthDropped = qs('healthDropped');
const healthResets = qs('healthResets');
const healthAge = qs('healthAge');
const healthGpu = qs('healthGpu');
const healthFps = qs('healthFps');

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

let latest = null;
let currentConfig = null;
let liveAudio = null;
let stats = { buffer_samples: 0, dropped: 0, resets: 0, last_sample_ago_ms: -1 };
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

let renderFrames = 0;
let renderFps = 0;
let lastFpsTs = performance.now();

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
let showDebugOverlay = localStorage.getItem('spectre.debugOverlay') !== '0';
let hoveredSignal = null;
let popoverHideTimer = null;

const GAIN_MAX = 60;
const timelineWindowMs = 5 * 60 * 1000;

function setConfigStatus(text) {
  configStatusEl.textContent = text;
}

function setWsBadge(text, kind = 'neutral') {
  wsBadge.textContent = text;
  wsBadge.style.borderColor = kind === 'ok'
    ? 'rgba(124, 251, 131, 0.35)'
    : kind === 'bad'
      ? 'rgba(255, 107, 129, 0.35)'
      : 'rgba(112, 150, 207, 0.18)';
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
  const pllMeta = signal.class?.pll?.locked ? ` • PLL ${signal.class.pll.method} LOCK ±${signal.class.pll.precision_hz} Hz` : '';
  signalPopover.innerHTML = `<div class="signal-popover__title">${signal.class?.mod_type || 'Signal'}</div><div class="signal-popover__meta">${fmtMHz(signal.class?.pll?.exact_hz || signal.center_hz, 5)} • ${fmtKHz(signal.bw_hz || 0)} • ${(signal.snr_db || 0).toFixed(1)} dB SNR${pllMeta}</div><div class="signal-popover__scores">${rows || '<div class="signal-popover__meta">No classifier scores</div>'}</div>`;
  const popW = 220;
  const left = rect.x + rect.w + 8;
  const top = rect.y + 8;
  const maxLeft = Math.max(8, spectrumCanvas.width - popW - 8);
  signalPopover.style.left = `${Math.max(8, Math.min(maxLeft, left))}px`;
  signalPopover.style.top = `${Math.max(8, top)}px`;
  signalPopover.classList.add('open');
  signalPopover.setAttribute('aria-hidden', 'false');
}

function colorMap(v) {
  const x = Math.max(0, Math.min(1, v));
  const r = Math.floor(255 * Math.pow(x, 0.55));
  const g = Math.floor(255 * Math.pow(x, 1.08));
  const b = Math.floor(220 * Math.pow(1 - x, 1.15));
  return [r, g, b];
}

function snrColor(snr) {
  const norm = Math.max(0, Math.min(1, (snr + 5) / 35));
  const [r, g, b] = colorMap(norm);
  return `rgb(${r}, ${g}, ${b})`;
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
  const dpr = window.devicePixelRatio || 1;
  const width = Math.max(1, Math.floor(rect.width * dpr));
  const height = Math.max(1, Math.floor(rect.height * dpr));
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width;
    canvas.height = height;
  }
}

function resizeAll() {
  [navCanvas, spectrumCanvas, waterfallCanvas, occupancyCanvas, timelineCanvas, detailSpectrogram].forEach(resizeCanvas);
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
  if (lastFFTSize !== cfg.fft_size) {
    resetProcessingCaches();
    lastFFTSize = cfg.fft_size;
  }
  const uiGain = Math.max(0, Math.min(GAIN_MAX, GAIN_MAX - cfg.gain_db));
  gainRange.value = uiGain;
  gainInput.value = uiGain;
  thresholdRange.value = cfg.detector.threshold_db;
  thresholdInput.value = cfg.detector.threshold_db;
  if (classifierModeSelect) classifierModeSelect.value = cfg.classifier_mode || 'combined';
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
  spanInput.value = (cfg.sample_rate / zoom / 1e6).toFixed(3);
  isSyncingConfig = false;
}

async function loadConfig() {
  try {
    const res = await fetch('/api/config');
    if (!res.ok) throw new Error('config');
    currentConfig = await res.json();
    applyConfigToUI(currentConfig);
    setConfigStatus('Config synced');
  } catch {
    setConfigStatus('Config offline');
  }
}

thresholdInput.addEventListener('change', () => {
  const v = parseFloat(thresholdInput.value);
  if (Number.isFinite(v)) {
    thresholdRange.value = v;
    queueConfigUpdate({ detector: { threshold_db: v } });
  }
});

if (classifierModeSelect) classifierModeSelect.addEventListener('change', () => {
  queueConfigUpdate({ classifier_mode: classifierModeSelect.value });
});
if (cfarGuardHzInput) cfarGuardHzInput.addEventListener('change', () => {
  const v = parseFloat(cfarGuardHzInput.value);
  if (Number.isFinite(v) && v >= 0) queueConfigUpdate({ detector: { cfar_guard_hz: v } });
});
if (cfarTrainHzInput) cfarTrainHzInput.addEventListener('change', () => {
  const v = parseFloat(cfarTrainHzInput.value);
  if (Number.isFinite(v) && v > 0) queueConfigUpdate({ detector: { cfar_train_hz: v } });
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
