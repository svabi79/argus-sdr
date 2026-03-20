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
  signalPopover.innerHTML = `<div class="signal-popover__title">${signal.class?.mod_type || 'Signal'}${signal.class?.pll?.rds_station ? ' · ' + signal.class.pll.rds_station : ''}</div><div class="signal-popover__meta">${fmtMHz(signal.class?.pll?.exact_hz || signal.center_hz, 5)} · ${fmtKHz(signal.bw_hz || 0)} · ${(signal.snr_db || 0).toFixed(1)} dB SNR${signal.class?.pll?.locked ? ` · PLL ${signal.class.pll.method} LOCK` : ''}${signal.class?.pll?.stereo ? ' · STEREO' : ''}</div><div class="signal-popover__scores">${rows || '<div class="signal-popover__meta">No classifier scores</div>'}</div>`;
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

async function loadSignals() {
  try {
    const res = await fetch('/api/signals');
    if (!res.ok) return;
    const sigs = await res.json();
    if (Array.isArray(sigs)) {
      latest = latest || {};
      latest.signals = sigs;
      renderLists();
    }
  } catch {}
}

async function loadDecoders() {
  if (!decodeModeSelect) return;
  try {
    const res = await fetch('/api/decoders');
    if (!res.ok) return;
    const list = await res.json();
    if (!Array.isArray(list)) return;
    const current = decodeModeSelect.value;
    decodeModeSelect.innerHTML = '';
    list.forEach((mode) => {
      const opt = document.createElement('option');
      opt.value = mode;
      opt.textContent = mode;
      decodeModeSelect.appendChild(opt);
    });
    if (current) decodeModeSelect.value = current;
  } catch {}
}

async function loadStats() {
  try {
    const res = await fetch('/api/stats');
    if (!res.ok) return;
    stats = await res.json();
  } catch {}
}

async function loadGPU() {
  try {
    const res = await fetch('/api/gpu');
    if (!res.ok) return;
    gpuInfo = await res.json();
  } catch {}
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
  try {
    const res = await fetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error('apply');
    currentConfig = await res.json();
    applyConfigToUI(currentConfig);
    setConfigStatus('Config applied');
  } catch {
    setConfigStatus('Config apply failed');
  }
}

async function sendSettingsUpdate() {
  if (!pendingSettingsUpdate) return;
  const payload = pendingSettingsUpdate;
  pendingSettingsUpdate = null;
  try {
    const res = await fetch('/api/sdr/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error('apply');
    currentConfig = await res.json();
    applyConfigToUI(currentConfig);
    setConfigStatus('Settings applied');
  } catch {
    setConfigStatus('Settings apply failed');
  }
}

function updateHeroMetrics() {
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
  metaLine.textContent = `${fmtMHz(latest.center_hz, 3)} · ${fmtHz(span)} span · ${thresholdInfo} · ${gpuText}`;
  heroSubtitle.textContent = `${latest.signals?.length || 0} live signals · ${events.length} recent events tracked`;

  healthBuffer.textContent = String(stats.buffer_samples ?? '-');
  healthDropped.textContent = String(stats.dropped ?? '-');
  healthResets.textContent = String(stats.resets ?? '-');
  healthAge.textContent = stats.last_sample_ago_ms >= 0 ? `${stats.last_sample_ago_ms} ms` : 'n/a';
  healthGpu.textContent = gpuInfo.error ? `${gpuInfo.active ? 'ON' : 'OFF'} · ${gpuInfo.error}` : (gpuInfo.active ? 'ON' : (gpuInfo.available ? 'Ready' : 'N/A'));
  healthFps.textContent = `${renderFps.toFixed(0)} fps`;
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
      const color = snrColor(s.snr_db || 0);

      ctx.fillStyle = color.replace('rgb', 'rgba').replace(')', ', 0.14)');
      ctx.strokeStyle = color;
      ctx.lineWidth = 1.5;
      ctx.fillRect(x1, 10, boxW, h - 28);
      ctx.strokeRect(x1, 10, boxW, h - 28);
      ctx.fillStyle = color;
      ctx.font = '12px Inter, sans-serif';
      const modLabel = s.class?.mod_type || '';
      const rdsName = s.class?.pll?.rds_station || '';
      const freqStr = `${(s.center_hz / 1e6).toFixed(4)} MHz`;
      const label = rdsName ? `${freqStr} · ${modLabel} · ${rdsName}` : (modLabel ? `${freqStr} · ${modLabel}` : freqStr);
      ctx.fillText(label, Math.max(4, x1 + 4), 24 + (index % 3) * 16);

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
  const ctx = waterfallCanvas.getContext('2d');
  const w = waterfallCanvas.width;
  const h = waterfallCanvas.height;

  const prev = ctx.getImageData(0, 0, w, h - 1);
  ctx.putImageData(prev, 0, 1);

  const display = getProcessedSpectrum();
  if (!display) return;
  const n = display.length;
  const span = latest.sample_rate / zoom;
  const startHz = latest.center_hz - span / 2 + pan * span;
  const endHz = latest.center_hz + span / 2 + pan * span;
  const minDb = -120;
  const maxDb = 0;

  const row = ctx.createImageData(w, 1);
  for (let x = 0; x < w; x++) {
    const f1 = startHz + (x / w) * (endHz - startHz);
    const f2 = startHz + ((x + 1) / w) * (endHz - startHz);
    const b0 = binForFreq(f1, latest.center_hz, latest.sample_rate, n);
    const b1 = binForFreq(f2, latest.center_hz, latest.sample_rate, n);
    const v = maxInBinRange(display, b0, b1);
    const norm = Math.max(0, Math.min(1, (v - minDb) / (maxDb - minDb)));
    const [r, g, b] = colorMap(norm);
    row.data[x * 4] = r;
    row.data[x * 4 + 1] = g;
    row.data[x * 4 + 2] = b;
    row.data[x * 4 + 3] = 255;
  }
  ctx.putImageData(row, 0, 0);
  drawCfarEdgeOverlay(ctx, w, h, startHz, endHz);
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
  const ctx = detailSpectrogram.getContext('2d');
  const w = detailSpectrogram.width;
  const h = detailSpectrogram.height;
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
  const minDb = -120;
  const maxDb = 0;

  const row = ctx.createImageData(w, 1);
  for (let x = 0; x < w; x++) {
    const f1 = startHz + (x / w) * (endHz - startHz);
    const f2 = startHz + ((x + 1) / w) * (endHz - startHz);
    const b0 = binForFreq(f1, latest.center_hz, latest.sample_rate, n);
    const b1 = binForFreq(f2, latest.center_hz, latest.sample_rate, n);
    const v = maxInBinRange(display, b0, b1);
    const norm = Math.max(0, Math.min(1, (v - minDb) / (maxDb - minDb)));
    const [r, g, b] = colorMap(norm);
    row.data[x * 4] = r;
    row.data[x * 4 + 1] = g;
    row.data[x * 4 + 2] = b;
    row.data[x * 4 + 3] = 255;
  }

  for (let y = 0; y < h; y++) ctx.putImageData(row, 0, y);

  const centerX = w / 2;
  ctx.strokeStyle = 'rgba(255,255,255,0.65)';
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(centerX, 0);
  ctx.lineTo(centerX, h);
  ctx.stroke();
}

let _lastEventListKey = '';

function _createSignalItem(s) {
  const btn = document.createElement('button');
  btn.className = 'list-item signal-item';
  btn.type = 'button';
  btn.dataset.center = s.center_hz;
  btn.dataset.bw = s.bw_hz || 0;
  btn.dataset.class = s.class?.mod_type || '';
  btn.dataset.id = s.id || 0;
  btn.innerHTML = `<div class="item-top"><span class="item-title" data-field="freq">${fmtMHz(s.center_hz, 6)}</span><span class="item-badge" data-field="snr" style="color:${snrColor(s.snr_db || 0)}">${(s.snr_db || 0).toFixed(1)} dB</span></div><div class="item-bottom"><span class="item-meta" data-field="bw">BW ${fmtKHz(s.bw_hz || 0)}</span><span class="item-meta" data-field="mod">${s.class?.mod_type || 'live carrier'}${s.class?.pll?.rds_station ? ' · ' + s.class.pll.rds_station : ''}</span></div>`;
  return btn;
}

function _patchSignalItem(el, s) {
  const freqEl = el.querySelector('[data-field="freq"]');
  const snrEl = el.querySelector('[data-field="snr"]');
  const bwEl = el.querySelector('[data-field="bw"]');
  const modEl = el.querySelector('[data-field="mod"]');
  if (freqEl) freqEl.textContent = fmtMHz(s.center_hz, 6);
  if (snrEl) { snrEl.textContent = `${(s.snr_db || 0).toFixed(1)} dB`; snrEl.style.color = snrColor(s.snr_db || 0); }
  if (bwEl) bwEl.textContent = `BW ${fmtKHz(s.bw_hz || 0)}`;
  if (modEl) modEl.textContent = (s.class?.mod_type || 'live carrier') + (s.class?.pll?.rds_station ? ' · ' + s.class.pll.rds_station : '');
  el.dataset.center = s.center_hz;
  el.dataset.bw = s.bw_hz || 0;
  el.dataset.class = s.class?.mod_type || '';
}

function renderLists() {
  const signals = Array.isArray(latest?.signals) ? [...latest.signals] : [];
  signals.sort((a, b) => (b.snr_db || 0) - (a.snr_db || 0));
  signalCountBadge.textContent = `${signals.length} live`;
  metricSignals.textContent = String(signals.length);

  const displaySigs = signals.slice(0, 24);
  const wantIds = new Set(displaySigs.map(s => String(s.id || 0)));

  // Remove empty-state placeholder if signals exist
  const emptyEl = signalList.querySelector('.empty-state');
  if (emptyEl && displaySigs.length > 0) emptyEl.remove();

  // Remove DOM items whose signal ID is no longer present
  signalList.querySelectorAll('.signal-item').forEach(el => {
    if (!wantIds.has(el.dataset.id)) el.remove();
  });

  if (displaySigs.length === 0) {
    if (!signalList.querySelector('.empty-state')) {
      signalList.innerHTML = '<div class="empty-state">No live signals yet.</div>';
    }
  } else {
    // Build map of existing DOM items
    const domById = new Map();
    signalList.querySelectorAll('.signal-item').forEach(el => domById.set(el.dataset.id, el));

    displaySigs.forEach(s => {
      const id = String(s.id || 0);
      const existing = domById.get(id);
      if (existing) {
        _patchSignalItem(existing, s);
      } else {
        const el = _createSignalItem(s);
        // Auto-select if it matches the user's last selection
        if (window._selectedSignal && Math.abs(s.center_hz - window._selectedSignal.freq) < 50000) {
          el.classList.add('active');
        }
        signalList.appendChild(el);
      }
    });
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
  renderLists();
}

async function fetchEvents(initial) {
  if (eventsFetchInFlight || timelineFrozen) return;
  eventsFetchInFlight = true;
  try {
    let url = '/api/events?limit=1000';
    if (!initial && lastEventEndMs > 0) url = `/api/events?since=${lastEventEndMs - 1}`;
    const res = await fetch(url);
    if (!res.ok) return;
    const data = await res.json();
    if (Array.isArray(data)) upsertEvents(data, initial);
  } finally {
    eventsFetchInFlight = false;
  }
}

async function fetchRecordings() {
  if (recordingsFetchInFlight || !recordingList) return;
  recordingsFetchInFlight = true;
  try {
    const res = await fetch('/api/recordings');
    if (!res.ok) return;
    const data = await res.json();
    if (Array.isArray(data)) {
      recordings = data;
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
  renderLists();
}

function closeDrawer() {
  drawerEl.classList.remove('open');
  drawerEl.setAttribute('aria-hidden', 'true');
  selectedEventId = null;
  renderLists();
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

function connect() {
  clearTimeout(wsReconnectTimer);
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(`${proto}://${location.host}/ws`);
  setWsBadge('Connecting', 'neutral');

  ws.onopen = () => setWsBadge('Live', 'ok');
  ws.onmessage = (ev) => {
    latest = JSON.parse(ev.data);
    markSpectrumDirty();
    if (followLive) pan = 0;
    updateHeroMetrics();
    renderLists();
  };
  ws.onclose = () => {
    setWsBadge('Retrying', 'bad');
    wsReconnectTimer = setTimeout(connect, 1000);
  };
  ws.onerror = () => ws.close();
}

function renderLoop() {
  renderFrames += 1;
  const now = performance.now();
  if (now - lastFpsTs >= 1000) {
    renderFps = (renderFrames * 1000) / (now - lastFpsTs);
    renderFrames = 0;
    lastFpsTs = now;
  }

  if (latest) {
    renderBandNavigator();
    renderSpectrum();
    renderWaterfall();
    renderOccupancy();
    renderTimeline();
    if (drawerEl.classList.contains('open')) renderDetailSpectrogram();
  }
  updateHeroMetrics();
  requestAnimationFrame(renderLoop);
}

function handleSpectrumClick(ev) {
  const rect = spectrumCanvas.getBoundingClientRect();
  const x = (ev.clientX - rect.left) * (spectrumCanvas.width / rect.width);
  const y = (ev.clientY - rect.top) * (spectrumCanvas.height / rect.height);

  for (let i = liveSignalRects.length - 1; i >= 0; i--) {
    const r = liveSignalRects[i];
    if (x >= r.x && x <= r.x + r.w && y >= r.y && y <= r.y + r.h) {
      tuneToFrequency(r.signal.center_hz);
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
    scheduleHideSignalPopover();
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
  localStorage.setItem('spectre.debugOverlay', showDebugOverlay ? '1' : '0');
  markSpectrumDirty();
  updateHeroMetrics();
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

railTabs.forEach((tab) => {
  tab.addEventListener('click', () => {
    railTabs.forEach(t => t.classList.toggle('active', t === tab));
    tabPanels.forEach(panel => panel.classList.toggle('active', panel.dataset.panel === tab.dataset.tab));
  });
});

modeButtons.forEach((btn) => {
  btn.addEventListener('click', () => {
    modeButtons.forEach(b => b.classList.toggle('active', b === btn));
    document.body.classList.remove('mode-live', 'mode-hunt', 'mode-review', 'mode-lab');
    document.body.classList.add(`mode-${btn.dataset.mode}`);
  });
});
document.body.classList.add('mode-live');

drawerCloseBtn.addEventListener('click', closeDrawer);
exportEventBtn.addEventListener('click', exportSelectedEvent);
if (liveListenEventBtn) {
  liveListenEventBtn.addEventListener('click', () => {
    const ev = eventsById.get(selectedEventId);
    if (!ev) return;
    const freq = ev.center_hz;
    const bw = ev.bandwidth_hz || 12000;
    const mode = (listenModeSelect?.value || ev.class?.mod_type || 'NFM');
    const sec = parseInt(listenSecondsInput?.value || '2', 10);
    const url = `/api/demod?freq=${freq}&bw=${bw}&mode=${mode}&sec=${sec}`;
    const audio = new Audio(url);
    audio.play();
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
    const res = await fetch(`/api/recordings/${rec.id}/decode?mode=${mode}`);
    if (!res.ok) {
      decodeResultEl.textContent = 'Decode: failed';
      return;
    }
    const data = await res.json();
    decodeResultEl.textContent = `Decode: ${String(data.stdout || '').slice(0, 80)}`;
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
  const allItems = signalList.querySelectorAll('.signal-item');
  allItems.forEach(el => el.classList.remove('active'));
  target.classList.add('active');
  // Store selected signal data for Live Listen button
  window._selectedSignal = {
    freq: parseFloat(target.dataset.center),
    bw: parseFloat(target.dataset.bw || '12000'),
    mode: target.dataset.class || ''
  };
});

if (liveListenBtn) {
  liveListenBtn.addEventListener('click', async () => {
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
    mode = (listenModeSelect?.value === 'Auto') ? (mode || 'NFM') : listenModeSelect.value;
    const sec = parseInt(listenSecondsInput?.value || '2', 10);
    const url = `/api/demod?freq=${freq}&bw=${bw}&mode=${mode}&sec=${sec}`;
    if (liveAudio) {
      liveAudio.pause();
    }
    liveAudio = new Audio(url);
    liveAudio.play().catch(() => {});
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
      const res = await fetch(`/api/recordings/${id}`);
      if (!res.ok) return;
      const meta = await res.json();
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

loadConfig();
loadStats();
loadGPU();
fetchEvents(true);
fetchRecordings();
loadDecoders();
connect();
requestAnimationFrame(renderLoop);
setInterval(loadStats, 1000);
setInterval(loadGPU, 1000);
setInterval(() => fetchEvents(false), 2000);
setInterval(fetchRecordings, 5000);
setInterval(loadSignals, 1500);
setInterval(loadDecoders, 10000);


