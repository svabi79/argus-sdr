const spectrumCanvas = document.getElementById('spectrum');
const waterfallCanvas = document.getElementById('waterfall');
const timelineCanvas = document.getElementById('timeline');
const statusEl = document.getElementById('status');
const metaEl = document.getElementById('meta');
const timelineRangeEl = document.getElementById('timelineRange');
const drawerEl = document.getElementById('eventDrawer');
const drawerCloseBtn = document.getElementById('drawerClose');
const detailCenterEl = document.getElementById('detailCenter');
const detailBwEl = document.getElementById('detailBw');
const detailStartEl = document.getElementById('detailStart');
const detailEndEl = document.getElementById('detailEnd');
const detailSnrEl = document.getElementById('detailSnr');
const detailDurEl = document.getElementById('detailDur');
const detailSpectrogram = document.getElementById('detailSpectrogram');
const configStatusEl = document.getElementById('configStatus');
const centerInput = document.getElementById('centerInput');
const spanInput = document.getElementById('spanInput');
const sampleRateSelect = document.getElementById('sampleRateSelect');
const fftSelect = document.getElementById('fftSelect');
const bwSelect = document.getElementById('bwSelect');
const gainRange = document.getElementById('gainRange');
const gainInput = document.getElementById('gainInput');
const thresholdRange = document.getElementById('thresholdRange');
const thresholdInput = document.getElementById('thresholdInput');
const agcToggle = document.getElementById('agcToggle');
const dcToggle = document.getElementById('dcToggle');
const iqToggle = document.getElementById('iqToggle');
const avgSelect = document.getElementById('avgSelect');
const maxHoldToggle = document.getElementById('maxHoldToggle');
const maxHoldReset = document.getElementById('maxHoldReset');
const gpuToggle = document.getElementById('gpuToggle');
const presetButtons = Array.from(document.querySelectorAll('.preset-btn'));

let latest = null;
let zoom = 1.0;
let pan = 0.0;
let isDragging = false;
let dragStartX = 0;
let dragStartPan = 0;
let timelineDirty = true;
let detailDirty = false;
let currentConfig = null;
let isSyncingConfig = false;
let pendingConfigUpdate = null;
let pendingSettingsUpdate = null;
let configTimer = null;
let settingsTimer = null;
const GAIN_MAX = 60;
let avgAlpha = 0;
let avgSpectrum = null;
let maxHold = false;
let maxSpectrum = null;
let lastFFTSize = null;
let stats = { buffer_samples: 0, dropped: 0, resets: 0 };
let gpuInfo = { available: false, active: false, error: '' };

const events = [];
const eventsById = new Map();
let lastEventEndMs = 0;
let eventsFetchInFlight = false;
let timelineRects = [];
let selectedEventId = null;

function resize() {
  const dpr = window.devicePixelRatio || 1;
  const rect1 = spectrumCanvas.getBoundingClientRect();
  spectrumCanvas.width = rect1.width * dpr;
  spectrumCanvas.height = rect1.height * dpr;
  const rect2 = waterfallCanvas.getBoundingClientRect();
  waterfallCanvas.width = rect2.width * dpr;
  waterfallCanvas.height = rect2.height * dpr;
  const rect3 = timelineCanvas.getBoundingClientRect();
  timelineCanvas.width = rect3.width * dpr;
  timelineCanvas.height = rect3.height * dpr;
  const rect4 = detailSpectrogram.getBoundingClientRect();
  detailSpectrogram.width = rect4.width * dpr;
  detailSpectrogram.height = rect4.height * dpr;
  timelineDirty = true;
  detailDirty = true;
}

window.addEventListener('resize', resize);
resize();

function setConfigStatus(text) {
  if (configStatusEl) {
    configStatusEl.textContent = text;
  }
}

function toMHz(hz) {
  return hz / 1e6;
}

function fromMHz(mhz) {
  return mhz * 1e6;
}

function applyConfigToUI(cfg) {
  if (!cfg) return;
  isSyncingConfig = true;
  centerInput.value = toMHz(cfg.center_hz).toFixed(6);
  if (sampleRateSelect) {
    sampleRateSelect.value = toMHz(cfg.sample_rate).toFixed(3).replace(/\.0+$/, '').replace(/\.$/, '');
  }
  const spanMHz = toMHz(cfg.sample_rate / zoom);
  spanInput.value = spanMHz.toFixed(3);
  fftSelect.value = String(cfg.fft_size);
  if (lastFFTSize !== cfg.fft_size) {
    avgSpectrum = null;
    maxSpectrum = null;
    lastFFTSize = cfg.fft_size;
  }
  if (bwSelect) {
    bwSelect.value = String(cfg.tuner_bw_khz || 1536);
  }
  const uiGain = Math.max(0, Math.min(GAIN_MAX, GAIN_MAX - cfg.gain_db));
  gainRange.value = uiGain;
  gainInput.value = uiGain;
  thresholdRange.value = cfg.detector.threshold_db;
  thresholdInput.value = cfg.detector.threshold_db;
  agcToggle.checked = !!cfg.agc;
  dcToggle.checked = !!cfg.dc_block;
  iqToggle.checked = !!cfg.iq_balance;
  if (gpuToggle) gpuToggle.checked = !!cfg.use_gpu_fft;
  isSyncingConfig = false;
}

async function loadConfig() {
  try {
    const res = await fetch('/api/config');
    if (!res.ok) {
      setConfigStatus('Failed to load');
      return;
    }
    const data = await res.json();
    currentConfig = data;
    applyConfigToUI(currentConfig);
    setConfigStatus('Synced');
  } catch (err) {
    setConfigStatus('Offline');
  }
}

async function loadStats() {
  try {
    const res = await fetch('/api/stats');
    if (!res.ok) return;
    const data = await res.json();
    stats = data || stats;
  } catch (err) {
    // ignore
  }
}

async function loadGPU() {
  try {
    const res = await fetch('/api/gpu');
    if (!res.ok) return;
    const data = await res.json();
    gpuInfo = data || gpuInfo;
  } catch (err) {
    // ignore
  }
}

function queueConfigUpdate(partial) {
  if (isSyncingConfig) return;
  pendingConfigUpdate = { ...(pendingConfigUpdate || {}), ...partial };
  setConfigStatus('Updating...');
  if (configTimer) clearTimeout(configTimer);
  configTimer = setTimeout(sendConfigUpdate, 200);
}

function queueSettingsUpdate(partial) {
  if (isSyncingConfig) return;
  pendingSettingsUpdate = { ...(pendingSettingsUpdate || {}), ...partial };
  setConfigStatus('Updating...');
  if (settingsTimer) clearTimeout(settingsTimer);
  settingsTimer = setTimeout(sendSettingsUpdate, 100);
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
    if (!res.ok) {
      setConfigStatus('Apply failed');
      return;
    }
    const data = await res.json();
    currentConfig = data;
    applyConfigToUI(currentConfig);
    setConfigStatus('Applied');
  } catch (err) {
    setConfigStatus('Offline');
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
    if (!res.ok) {
      setConfigStatus('Apply failed');
      return;
    }
    const data = await res.json();
    currentConfig = data;
    applyConfigToUI(currentConfig);
    setConfigStatus('Applied');
  } catch (err) {
    setConfigStatus('Offline');
  }
}

function colorMap(v) {
  // v in [0..1]
  const r = Math.min(255, Math.max(0, Math.floor(255 * Math.pow(v, 0.6))));
  const g = Math.min(255, Math.max(0, Math.floor(255 * Math.pow(v, 1.1))));
  const b = Math.min(255, Math.max(0, Math.floor(180 * Math.pow(1 - v, 1.2))));
  return [r, g, b];
}

function binForFreq(freq, centerHz, sampleRate, n) {
  return Math.floor((freq - (centerHz - sampleRate / 2)) / (sampleRate / n));
}

function maxInBinRange(spectrum, b0, b1) {
  const n = spectrum.length;
  let start = Math.max(0, Math.min(n - 1, b0));
  let end = Math.max(0, Math.min(n - 1, b1));
  if (end < start) {
    const tmp = start;
    start = end;
    end = tmp;
  }
  let max = -1e9;
  for (let i = start; i <= end; i++) {
    const v = spectrum[i];
    if (v > max) max = v;
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

function snrColor(snr) {
  const norm = Math.max(0, Math.min(1, (snr + 5) / 30));
  const [r, g, b] = colorMap(norm);
  return `rgb(${r}, ${g}, ${b})`;
}

function renderSpectrum() {
  if (!latest) return;
  const ctx = spectrumCanvas.getContext('2d');
  const w = spectrumCanvas.width;
  const h = spectrumCanvas.height;
  ctx.clearRect(0, 0, w, h);

  // Grid
  ctx.strokeStyle = '#13263b';
  ctx.lineWidth = 1;
  for (let i = 1; i < 10; i++) {
    const y = (h / 10) * i;
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(w, y);
    ctx.stroke();
  }

  const { spectrum_db, sample_rate, center_hz } = latest;
  const display = processSpectrum(spectrum_db);
  const n = display.length;
  const span = sample_rate / zoom;
  const startHz = center_hz - span / 2 + pan * span;
  const endHz = center_hz + span / 2 + pan * span;
  if (!isSyncingConfig && spanInput) {
    spanInput.value = (span / 1e6).toFixed(3);
  }

  const minDb = -120;
  const maxDb = 0;

  ctx.strokeStyle = '#48d1b8';
  ctx.lineWidth = 2;
  ctx.beginPath();
  for (let x = 0; x < w; x++) {
    const f1 = startHz + (x / w) * (endHz - startHz);
    const f2 = startHz + ((x + 1) / w) * (endHz - startHz);
    const b0 = binForFreq(f1, center_hz, sample_rate, n);
    const b1 = binForFreq(f2, center_hz, sample_rate, n);
    const v = maxInBinRange(display, b0, b1);
    const y = h - ((v - minDb) / (maxDb - minDb)) * h;
    if (x === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.stroke();

  // Signals overlay
  ctx.strokeStyle = '#ffb454';
  ctx.lineWidth = 2;
  if (latest.signals) {
    for (const s of latest.signals) {
      const left = s.center_hz - s.bw_hz / 2;
      const right = s.center_hz + s.bw_hz / 2;
      if (right < startHz || left > endHz) continue;
      const x1 = ((left - startHz) / (endHz - startHz)) * w;
      const x2 = ((right - startHz) / (endHz - startHz)) * w;
      ctx.beginPath();
      ctx.moveTo(x1, h - 4);
      ctx.lineTo(x2, h - 4);
      ctx.stroke();
    }
  }

  const binHz = sample_rate / n;
  const gpuState = gpuInfo.active ? 'GPU:ON' : (gpuInfo.available ? 'GPU:OFF' : 'GPU:N/A');
  metaEl.textContent = `Center ${(center_hz/1e6).toFixed(3)} MHz | Span ${(span/1e6).toFixed(3)} MHz | Res ${binHz.toFixed(1)} Hz/bin | Buf ${stats.buffer_samples} Drop ${stats.dropped} Reset ${stats.resets} | ${gpuState}`;
}

function renderWaterfall() {
  if (!latest) return;
  const ctx = waterfallCanvas.getContext('2d');
  const w = waterfallCanvas.width;
  const h = waterfallCanvas.height;

  // Scroll down
  const image = ctx.getImageData(0, 0, w, h);
  ctx.putImageData(image, 0, 1);

  const { spectrum_db, sample_rate, center_hz } = latest;
  const display = processSpectrum(spectrum_db);
  const n = display.length;
  const span = sample_rate / zoom;
  const startHz = center_hz - span / 2 + pan * span;
  const endHz = center_hz + span / 2 + pan * span;
  const minDb = -120;
  const maxDb = 0;

  const row = ctx.createImageData(w, 1);
  for (let x = 0; x < w; x++) {
    const f1 = startHz + (x / w) * (endHz - startHz);
    const f2 = startHz + ((x + 1) / w) * (endHz - startHz);
    const b0 = binForFreq(f1, center_hz, sample_rate, n);
    const b1 = binForFreq(f2, center_hz, sample_rate, n);
    if (b0 < n && b1 >= 0) {
      const v = maxInBinRange(display, b0, b1);
      const norm = Math.max(0, Math.min(1, (v - minDb) / (maxDb - minDb)));
      const [r, g, b] = colorMap(norm);
      row.data[x * 4 + 0] = r;
      row.data[x * 4 + 1] = g;
      row.data[x * 4 + 2] = b;
      row.data[x * 4 + 3] = 255;
    } else {
      row.data[x * 4 + 3] = 255;
    }
  }
  ctx.putImageData(row, 0, 0);
}

function renderTimeline() {
  const ctx = timelineCanvas.getContext('2d');
  const w = timelineCanvas.width;
  const h = timelineCanvas.height;
  ctx.clearRect(0, 0, w, h);

  if (events.length === 0) {
    timelineRangeEl.textContent = 'No events yet';
    return;
  }

  const now = Date.now();
  const windowMs = 5 * 60 * 1000;
  const endMs = now;
  const startMs = endMs - windowMs;

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

  ctx.strokeStyle = '#13263b';
  ctx.lineWidth = 1;
  for (let i = 1; i < 6; i++) {
    const y = (h / 6) * i;
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(w, y);
    ctx.stroke();
  }

  timelineRects = [];
  for (const ev of events) {
    if (ev.end_ms < startMs || ev.start_ms > endMs) continue;
    const x1 = ((Math.max(ev.start_ms, startMs) - startMs) / (endMs - startMs)) * w;
    const x2 = ((Math.min(ev.end_ms, endMs) - startMs) / (endMs - startMs)) * w;
    const bw = Math.max(ev.bandwidth_hz, 1);
    const topHz = ev.center_hz + bw / 2;
    const bottomHz = ev.center_hz - bw / 2;
    const y1 = ((maxHz - topHz) / (maxHz - minHz)) * h;
    const y2 = ((maxHz - bottomHz) / (maxHz - minHz)) * h;
    const rectH = Math.max(2, y2 - y1);

    ctx.fillStyle = snrColor(ev.snr_db || 0);
    ctx.fillRect(x1, y1, Math.max(2, x2 - x1), rectH);

    const rect = { x: x1, y: y1, w: Math.max(2, x2 - x1), h: rectH, id: ev.id };
    timelineRects.push(rect);
  }

  if (selectedEventId) {
    const hit = timelineRects.find((r) => r.id === selectedEventId);
    if (hit) {
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 2;
      ctx.strokeRect(hit.x - 1, hit.y - 1, hit.w + 2, hit.h + 2);
    }
  }

  const startLabel = new Date(startMs).toLocaleTimeString();
  const endLabel = new Date(endMs).toLocaleTimeString();
  timelineRangeEl.textContent = `${startLabel} - ${endLabel}`;
}

function renderDetailSpectrogram(ev) {
  const ctx = detailSpectrogram.getContext('2d');
  const w = detailSpectrogram.width;
  const h = detailSpectrogram.height;
  ctx.clearRect(0, 0, w, h);
  if (!latest || !ev) return;

  const span = Math.min(latest.sample_rate, Math.max(ev.bandwidth_hz * 3, latest.sample_rate / 8));
  const startHz = ev.center_hz - span / 2;
  const endHz = ev.center_hz + span / 2;

  const { spectrum_db, sample_rate, center_hz } = latest;
  const display = processSpectrum(spectrum_db);
  const n = display.length;
  const minDb = -120;
  const maxDb = 0;

  const row = ctx.createImageData(w, 1);
  for (let x = 0; x < w; x++) {
    const f1 = startHz + (x / w) * (endHz - startHz);
    const f2 = startHz + ((x + 1) / w) * (endHz - startHz);
    const b0 = binForFreq(f1, center_hz, sample_rate, n);
    const b1 = binForFreq(f2, center_hz, sample_rate, n);
    if (b0 < n && b1 >= 0) {
      const v = maxInBinRange(display, b0, b1);
      const norm = Math.max(0, Math.min(1, (v - minDb) / (maxDb - minDb)));
      const [r, g, b] = colorMap(norm);
      row.data[x * 4 + 0] = r;
      row.data[x * 4 + 1] = g;
      row.data[x * 4 + 2] = b;
      row.data[x * 4 + 3] = 255;
    } else {
      row.data[x * 4 + 3] = 255;
    }
  }
  for (let y = 0; y < h; y++) {
    ctx.putImageData(row, 0, y);
  }
}

function tick() {
  renderSpectrum();
  renderWaterfall();
  if (timelineDirty) {
    renderTimeline();
    timelineDirty = false;
  }
  if (detailDirty && drawerEl.classList.contains('open')) {
    const ev = eventsById.get(selectedEventId);
    renderDetailSpectrogram(ev);
    detailDirty = false;
  }
  requestAnimationFrame(tick);
}

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onopen = () => {
    statusEl.textContent = 'Connected';
  };
  ws.onmessage = (ev) => {
    latest = JSON.parse(ev.data);
    detailDirty = true;
    timelineDirty = true;
  };
  ws.onclose = () => {
    statusEl.textContent = 'Disconnected - retrying...';
    setTimeout(connect, 1000);
  };
  ws.onerror = () => {
    ws.close();
  };
}

spectrumCanvas.addEventListener('wheel', (ev) => {
  ev.preventDefault();
  const delta = Math.sign(ev.deltaY);
  zoom = Math.max(0.5, Math.min(10, zoom * (delta > 0 ? 1.1 : 0.9)));
});

spectrumCanvas.addEventListener('mousedown', (ev) => {
  isDragging = true;
  dragStartX = ev.clientX;
  dragStartPan = pan;
});

window.addEventListener('mouseup', () => { isDragging = false; });
window.addEventListener('mousemove', (ev) => {
  if (!isDragging) return;
  const dx = ev.clientX - dragStartX;
  pan = dragStartPan - dx / spectrumCanvas.clientWidth;
  pan = Math.max(-0.5, Math.min(0.5, pan));
});

centerInput.addEventListener('change', () => {
  const mhz = parseFloat(centerInput.value);
  if (Number.isFinite(mhz)) {
    queueConfigUpdate({ center_hz: fromMHz(mhz) });
  }
});

spanInput.addEventListener('change', () => {
  const mhz = parseFloat(spanInput.value);
  if (!Number.isFinite(mhz) || mhz <= 0) return;
  const baseRate = currentConfig ? currentConfig.sample_rate : (latest ? latest.sample_rate : 0);
  if (!baseRate) return;
  zoom = Math.max(0.25, Math.min(20, baseRate / fromMHz(mhz)));
  timelineDirty = true;
});

if (sampleRateSelect) {
  sampleRateSelect.addEventListener('change', () => {
    const mhz = parseFloat(sampleRateSelect.value);
    if (Number.isFinite(mhz) && mhz > 0) {
      queueConfigUpdate({ sample_rate: Math.round(fromMHz(mhz)) });
    }
  });
}

if (bwSelect) {
  bwSelect.addEventListener('change', () => {
    const bw = parseInt(bwSelect.value, 10);
    if (Number.isFinite(bw)) {
      queueConfigUpdate({ tuner_bw_khz: bw });
    }
  });
}

if (avgSelect) {
  avgSelect.addEventListener('change', () => {
    avgAlpha = parseFloat(avgSelect.value) || 0;
    avgSpectrum = null;
  });
}

if (maxHoldToggle) {
  maxHoldToggle.addEventListener('change', () => {
    maxHold = maxHoldToggle.checked;
    if (!maxHold) {
      maxSpectrum = null;
    }
  });
}

if (maxHoldReset) {
  maxHoldReset.addEventListener('click', () => {
    maxSpectrum = null;
  });
}

if (gpuToggle) {
  gpuToggle.addEventListener('change', () => {
    queueConfigUpdate({ use_gpu_fft: gpuToggle.checked });
  });
}

fftSelect.addEventListener('change', () => {
  const size = parseInt(fftSelect.value, 10);
  if (Number.isFinite(size)) {
    queueConfigUpdate({ fft_size: size });
  }
});

gainRange.addEventListener('input', () => {
  gainInput.value = gainRange.value;
  const uiVal = parseFloat(gainRange.value);
  if (Number.isFinite(uiVal)) {
    const gr = Math.max(0, Math.min(GAIN_MAX, GAIN_MAX - uiVal));
    queueConfigUpdate({ gain_db: gr });
  }
});

gainInput.addEventListener('change', () => {
  const v = parseFloat(gainInput.value);
  if (Number.isFinite(v)) {
    gainRange.value = v;
    const gr = Math.max(0, Math.min(GAIN_MAX, GAIN_MAX - v));
    queueConfigUpdate({ gain_db: gr });
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

agcToggle.addEventListener('change', () => {
  queueSettingsUpdate({ agc: agcToggle.checked });
});

dcToggle.addEventListener('change', () => {
  queueSettingsUpdate({ dc_block: dcToggle.checked });
});

iqToggle.addEventListener('change', () => {
  queueSettingsUpdate({ iq_balance: iqToggle.checked });
});

for (const btn of presetButtons) {
  btn.addEventListener('click', () => {
    const mhz = parseFloat(btn.dataset.center);
    if (Number.isFinite(mhz)) {
      centerInput.value = mhz.toFixed(3);
      queueConfigUpdate({ center_hz: fromMHz(mhz) });
    }
  });
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

function upsertEvents(list, replace) {
  if (replace) {
    events.length = 0;
    eventsById.clear();
  }
  for (const raw of list) {
    if (eventsById.has(raw.id)) continue;
    const ev = normalizeEvent(raw);
    eventsById.set(ev.id, ev);
    events.push(ev);
  }
  events.sort((a, b) => a.end_ms - b.end_ms);
  const maxEvents = 1500;
  if (events.length > maxEvents) {
    const drop = events.length - maxEvents;
    for (let i = 0; i < drop; i++) {
      eventsById.delete(events[i].id);
    }
    events.splice(0, drop);
  }
  if (events.length > 0) {
    lastEventEndMs = events[events.length - 1].end_ms;
  }
  timelineDirty = true;
}

async function fetchEvents(initial) {
  if (eventsFetchInFlight) return;
  eventsFetchInFlight = true;
  try {
    let url = '/api/events?limit=1000';
    if (!initial && lastEventEndMs > 0) {
      url = `/api/events?since=${lastEventEndMs - 1}`;
    }
    const res = await fetch(url);
    if (!res.ok) return;
    const data = await res.json();
    if (Array.isArray(data)) {
      upsertEvents(data, initial);
    }
  } finally {
    eventsFetchInFlight = false;
  }
}

function openDrawer(ev) {
  if (!ev) return;
  selectedEventId = ev.id;
  detailCenterEl.textContent = `${(ev.center_hz / 1e6).toFixed(6)} MHz`;
  detailBwEl.textContent = `${(ev.bandwidth_hz / 1e3).toFixed(2)} kHz`;
  detailStartEl.textContent = new Date(ev.start_ms).toLocaleString();
  detailEndEl.textContent = new Date(ev.end_ms).toLocaleString();
  detailSnrEl.textContent = `${(ev.snr_db || 0).toFixed(1)} dB`;
  detailDurEl.textContent = `${(ev.duration_ms / 1000).toFixed(2)} s`;
  drawerEl.classList.add('open');
  drawerEl.setAttribute('aria-hidden', 'false');
  resize();
  detailDirty = true;
  timelineDirty = true;
}

function closeDrawer() {
  drawerEl.classList.remove('open');
  drawerEl.setAttribute('aria-hidden', 'true');
  selectedEventId = null;
  timelineDirty = true;
}

drawerCloseBtn.addEventListener('click', closeDrawer);

timelineCanvas.addEventListener('click', (ev) => {
  const rect = timelineCanvas.getBoundingClientRect();
  const scaleX = timelineCanvas.width / rect.width;
  const scaleY = timelineCanvas.height / rect.height;
  const x = (ev.clientX - rect.left) * scaleX;
  const y = (ev.clientY - rect.top) * scaleY;

  for (let i = timelineRects.length - 1; i >= 0; i--) {
    const r = timelineRects[i];
    if (x >= r.x && x <= r.x + r.w && y >= r.y && y <= r.y + r.h) {
      const hit = eventsById.get(r.id);
      openDrawer(hit);
      return;
    }
  }
});

loadConfig();
connect();
requestAnimationFrame(tick);
fetchEvents(true);
setInterval(() => fetchEvents(false), 2000);
setInterval(loadStats, 1000);
setInterval(loadGPU, 1000);
