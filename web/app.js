const spectrumCanvas = document.getElementById('spectrum');
const waterfallCanvas = document.getElementById('waterfall');
const statusEl = document.getElementById('status');
const metaEl = document.getElementById('meta');

let latest = null;
let zoom = 1.0;
let pan = 0.0;
let isDragging = false;
let dragStartX = 0;
let dragStartPan = 0;

function resize() {
  const dpr = window.devicePixelRatio || 1;
  const rect1 = spectrumCanvas.getBoundingClientRect();
  spectrumCanvas.width = rect1.width * dpr;
  spectrumCanvas.height = rect1.height * dpr;
  const rect2 = waterfallCanvas.getBoundingClientRect();
  waterfallCanvas.width = rect2.width * dpr;
  waterfallCanvas.height = rect2.height * dpr;
}

window.addEventListener('resize', resize);
resize();

function colorMap(v) {
  // v in [0..1]
  const r = Math.min(255, Math.max(0, Math.floor(255 * Math.pow(v, 0.6))));
  const g = Math.min(255, Math.max(0, Math.floor(255 * Math.pow(v, 1.1))));
  const b = Math.min(255, Math.max(0, Math.floor(180 * Math.pow(1 - v, 1.2))));
  return [r, g, b];
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
  const n = spectrum_db.length;
  const span = sample_rate / zoom;
  const startHz = center_hz - span / 2 + pan * span;
  const endHz = center_hz + span / 2 + pan * span;

  const minDb = -120;
  const maxDb = 0;

  ctx.strokeStyle = '#48d1b8';
  ctx.lineWidth = 2;
  ctx.beginPath();
  for (let i = 0; i < n; i++) {
    const freq = center_hz + (i - n / 2) * (sample_rate / n);
    if (freq < startHz || freq > endHz) continue;
    const x = ((freq - startHz) / (endHz - startHz)) * w;
    const v = spectrum_db[i];
    const y = h - ((v - minDb) / (maxDb - minDb)) * h;
    if (i === 0) ctx.moveTo(x, y);
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

  metaEl.textContent = `Center ${(center_hz/1e6).toFixed(3)} MHz | Span ${(span/1e6).toFixed(3)} MHz`;
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
  const n = spectrum_db.length;
  const span = sample_rate / zoom;
  const startHz = center_hz - span / 2 + pan * span;
  const endHz = center_hz + span / 2 + pan * span;
  const minDb = -120;
  const maxDb = 0;

  const row = ctx.createImageData(w, 1);
  for (let x = 0; x < w; x++) {
    const freq = startHz + (x / (w - 1)) * (endHz - startHz);
    const bin = Math.floor((freq - (center_hz - sample_rate / 2)) / (sample_rate / n));
    if (bin >= 0 && bin < n) {
      const v = spectrum_db[bin];
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

function tick() {
  renderSpectrum();
  renderWaterfall();
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

connect();
requestAnimationFrame(tick);
