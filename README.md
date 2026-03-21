# SDR Visual Suite

Go-based SDRplay RSP1b live spectrum + waterfall visualizer with event tracking, classifier, and demod/recording pipeline.

## Features
- Live spectrum + waterfall web UI (WebSocket streaming)
- Event timeline view (time vs frequency) + detail drawer
- Live signal list + classifier insights
- Runtime UI controls: center, span, sample rate, tuner bandwidth, FFT size, gain, AGC, DC block, IQ balance, detector settings
- Optional GPU FFT (cuFFT) + `/api/gpu`
- IQ/audio recording + recordings list
- Live demod endpoint + WebSocket live-listen audio
- WFM stereo + RDS baseband
- Mock mode for testing without hardware

---

## Requirements
**Core**
- Go **1.22** (see `go.mod`)

**Optional (real device)**
- SDRplay API (`sdrplay_api.h` + library)

**Optional (GPU)**
- CUDA toolkit (cuFFT)
- `nvcc` for kernel build (Linux) or `build-gpudemod-dll.ps1` (Windows)

**Windows build prerequisites (real device + GPU)**
- MSYS2 MinGW64 (`C:\msys64\mingw64\bin\gcc.exe` / `g++.exe`) for CGO
- SDRplay API installed (default path in scripts)
- CUDA toolkit (default paths in scripts)

---

## Quick Start (Mock Mode)
```bash
# From repo root

go run ./cmd/sdrd --mock
```
Open `http://localhost:8080`.

---

## Configuration
Edit `config.yaml` (autosave goes to `config.autosave.yaml`).

Common fields:
- `center_hz`, `sample_rate`, `fft_size`, `gain_db`, `tuner_bw_khz`
- `use_gpu_fft`, `agc`, `dc_block`, `iq_balance`
- `detector.*` (e.g. `threshold_db`, `cfar_mode`, `cfar_guard_hz`, `cfar_train_hz`, `min_duration_ms`, `hold_ms`, ...)
- `recorder.*` (enable IQ/audio recording, output path, ring buffer, etc.)
- `decoder.*` (external decoder commands)

**CFAR modes:** `OFF`, `CA`, `OS`, `GOSCA`, `CASO`

---

## Build & Run (Windows)
### Mock / No SDRplay
```powershell
go build ./cmd/sdrd
.\sdrd.exe --mock
```

### SDRplay (Real Device)
```powershell
$env:CGO_CFLAGS='-IC:\Program Files\SDRplay\API\inc'
$env:CGO_LDFLAGS='-LC:\Program Files\SDRplay\API\x64 -lsdrplay_api'

go build -tags sdrplay ./cmd/sdrd
.\sdrd.exe -config config.yaml
```

### Windows (GPU + SDRplay) — recommended path
```powershell
powershell -ExecutionPolicy Bypass -File .\build-cuda-windows.ps1
powershell -ExecutionPolicy Bypass -File .\build-sdrplay.ps1
```
This path:
- Builds `gpudemod_kernels.dll` (MSVC/nvcc)
- Builds Go app with MinGW64 CGO + tags `sdrplay,cufft`
- Copies CUDA runtime DLLs next to `sdrd.exe`

Notes:
- `build-sdrplay.ps1` expects MinGW at `C:\msys64\mingw64\bin`
- CUDA DLLs are copied if found (see script for exact paths)
- Override the GPU DLL path with `GPUMOD_DLL=C:\path\to\gpudemod_kernels.dll`

---

## Build & Run (Linux)
### SDRplay (Real Device)
```bash
export CGO_CFLAGS='-I/opt/sdrplay_api/include'
export CGO_LDFLAGS='-L/opt/sdrplay_api/lib -lsdrplay_api'

go build -tags sdrplay ./cmd/sdrd
./cmd/sdrd/sdrd -config config.yaml
```

### CUDA kernel build (GPU demod)
```bash
./build-cuda-linux.sh
```

---

## APIs
### Config
- `GET /api/config`
- `POST /api/config`
- `POST /api/sdr/settings`
- `GET /api/gpu`

### Signals / Events
- `GET /api/signals` → current live signals
- `GET /api/events?limit=&since=` → recent events

### Recordings
- `GET /api/recordings`
- `GET /api/recordings/:id` (meta.json)
- `GET /api/recordings/:id/iq`
- `GET /api/recordings/:id/audio`
- `GET /api/recordings/:id/decode?mode=FT8|WSPR|DMR|D-STAR|FSK|PSK`

### Live Demod / Listen
- `GET /api/demod?freq=...&bw=...&mode=...&sec=...` → audio/wav
- `WS /ws/audio?freq=...&bw=...&mode=...` → live PCM audio stream

---

## Decoder Tools
Put external decoder binaries/scripts under `tools/` and configure `decoder.*` in `config.yaml`.
Placeholders: `{iq}`, `{audio}`, `{sr}`.
See `tools/README.md` for examples.

---

## Tests
```bash
go test ./...
```

---

## Troubleshooting
- `sdrplay support not built` → rebuild with `-tags sdrplay`.
- SDRplay library not found → check `CGO_CFLAGS` / `CGO_LDFLAGS`.
- GPU demod not loading → verify `gpudemod_kernels.dll` / `cudart64_13.dll` next to `sdrd.exe` (Windows).
- Use `--mock` to run without hardware.
