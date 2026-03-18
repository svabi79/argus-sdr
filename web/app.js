const qs = (id) => document.getElementById(id);

const navCanvas = qs('navCanvas');
const spectrumCanvas = qs('spectrum');
const waterfallCanvas = qs('waterfall');
const occupancyCanvas = qs('occupancy');
const timelineCanvas = qs('timeline');
const detailSpectrogram = qs('detailSpectrogram');

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
const recordingMetaLink = qs('recordingMetaLink');
const recordingIQLink = qs('recordingIQLink');
const recordingAudioLink = qs('recordingAudioLink');

const followBtn = qs('followBtn');
const fitBtn = qs('fitBtn');
const resetMaxBtn = qs('resetMaxBtn');
const timelineFollowBtn = qs('timelineFollowBtn');
const timelineFreezeBtn = qs('timelineFreezeBtn');

const modeButtons = Array.from(document.querySelectorAll('.mode-btn'));
const railTabs = Array.from(document.querySelectorAll('.rail-tab'));
const tabPanels = Array.from(document.querySelectorAll('.tab-panel'));
const presetButtons = Array.from(document.querySelectorAll('.preset-btn'));
const liveListenBtn = qs('liveListenBtn');

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
  metaLine.textContent = `${fmtMHz(latest.center_hz, 3)} · ${fmtHz(span)} span · ${gpuText}`;
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

  const display = processSpectrum(latest.spectrum_db);
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

function renderSpectrum() {
  if (!latest) return;
  const ctx = spectrumCanvas.getContext('2d');
  const w = spectrumCanvas.width;
  const h = spectrumCanvas.height;
  ctx.clearRect(0, 0, w, h);

  const display = processSpectrum(latest.spectrum_db);
  const n = display.length;
  const span = latest.sample_rate / zoom;
  const startHz = latest.center_hz - span / 2 + pan * span;
  const endHz = latest.center_hz + span / 2 + pan * span;
  spanInput.value = (span / 1e6).toFixed(3);

  drawSpectrumGrid(ctx, w, h, startHz, endHz);

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
      const label = `${(s.center_hz / 1e6).toFixed(4)} MHz`;
      ctx.fillText(label, Math.max(4, x1 + 4), 24 + (index % 3) * 16);

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

  const display = processSpectrum(latest.spectrum_db);
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

  const display = processSpectrum(latest.spectrum_db);
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

function renderLists() {
  const signals = Array.isArray(latest?.signals) ? [...latest.signals] : [];
  signals.sort((a, b) => (b.snr_db || 0) - (a.snr_db || 0));
  signalCountBadge.textContent = `${signals.length} live`;
  metricSignals.textContent = String(signals.length);

  if (signals.length === 0) {
    signalList.innerHTML = '<div class="empty-state">No live signals yet.</div>';
  } else {
    signalList.innerHTML = signals.slice(0, 24).map((s) => `
      <button class="list-item signal-item" type="button" data-center="${s.center_hz}" data-bw="${s.bw_hz || 0}" data-class="${s.class?.mod_type || ''}">
        <div class="item-top">
          <span class="item-title">${fmtMHz(s.center_hz, 6)}</span>
          <span class="item-badge" style="color:${snrColor(s.snr_db || 0)}">${(s.snr_db || 0).toFixed(1)} dB</span>
        </div>
        <div class="item-bottom">
          <span class="item-meta">BW ${fmtKHz(s.bw_hz || 0)}</span>
          <span class="item-meta">${s.class?.mod_type || 'live carrier'}</span>
        </div>
      </button>
    `).join('');
  }

  const recent = [...events].sort((a, b) => b.end_ms - a.end_ms);
  eventCountBadge.textContent = `${recent.length} stored`;
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
window.addEventListener('mousemove', (ev) => {
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

avgSelect.addEventListener('change', () => {
  avgAlpha = parseFloat(avgSelect.value) || 0;
  avgSpectrum = null;
});
maxHoldToggle.addEventListener('change', () => {
  maxHold = maxHoldToggle.checked;
  if (!maxHold) maxSpectrum = null;
});
resetMaxBtn.addEventListener('click', () => { maxSpectrum = null; });
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
  const center = parseFloat(target.dataset.center);
  if (Number.isFinite(center)) tuneToFrequency(center);
});

if (liveListenBtn) {
  liveListenBtn.addEventListener('click', async () => {
    const first = signalList.querySelector('.signal-item');
    if (!first) return;
    const freq = parseFloat(first.dataset.center);
    const bw = parseFloat(first.dataset.bw || '12000');
    const mode = first.dataset.class || 'NFM';
    const url = `/api/demod?freq=${freq}&bw=${bw}&mode=${mode}&sec=2`;
    const audio = new Audio(url);
    audio.play();
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
    } catch {}
  });
}

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

