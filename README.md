# SDR Wideband Suite

Go-based SDR analysis engine and live spectrum/waterfall UI, evolved from the original `sdr-visual-suite` into a more scalable foundation for wideband monitoring, candidate-driven refinement, classification, and demod/recording.

## Features
- Live spectrum + waterfall web UI (WebSocket streaming)
- Event timeline view (time vs frequency) + detail drawer
- Live signal list + classifier insights
- Runtime UI controls: center, span, sample rate, tuner bandwidth, analysis FFT size, gain, AGC, DC block, IQ balance, detector settings
- Optional GPU FFT (cuFFT) + `/api/gpu`
- IQ/audio recording + recordings list
- Live demod endpoint + WebSocket live-listen audio
- WFM stereo + RDS baseband
- Mock mode for testing without hardware
- Phase-1 wideband architecture foundation: explicit pipeline/surveillance/refinement/resources config model and candidate/refinement pipeline scaffolding

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

### Legacy-compatible core fields
- `center_hz`, `sample_rate`, `fft_size`, `gain_db`, `tuner_bw_khz`
- `use_gpu_fft`, `agc`, `dc_block`, `iq_balance`
- `detector.*`
- `recorder.*`
- `decoder.*`

### New phase-1 pipeline fields
- `pipeline.mode` — operating mode label (`legacy`, `wideband-balanced`, ...)
- `pipeline.goals.*` — declarative target/intent layer for future autonomous operation
  - `intent`
  - `monitor_start_hz` / `monitor_end_hz` / `monitor_span_hz`
  - `signal_priorities`
  - `auto_record_classes`
  - `auto_decode_classes`
- `surveillance.analysis_fft_size` — analysis FFT size used by the surveillance layer
- `surveillance.frame_rate` — surveillance cadence target
- `surveillance.strategy` — currently `single-resolution`, reserved for future multi-resolution modes
- `surveillance.display_bins` — preferred presentation density for clients/UI
- `surveillance.display_fps` — preferred presentation cadence for clients/UI
- `refinement.enabled` — enables explicit candidate refinement stage
- `refinement.max_concurrent` — refinement budget hint
- `refinement.min_candidate_snr_db` — floor for future scheduling decisions
- `refinement.min_span_hz` / `refinement.max_span_hz` — clamp refinement window span (0 = no clamp)
- `refinement.auto_span` — use mod-type heuristics when candidate bandwidth is missing/odd
- `resources.prefer_gpu` — GPU preference hint

**Profile defaults (wideband)**
- `wideband-balanced`: min_span_hz=4000, max_span_hz=200000
- `wideband-aggressive`: min_span_hz=6000, max_span_hz=250000
- `resources.max_refinement_jobs` — processing budget hint
- `resources.max_recording_streams` — recording/streaming budget hint
- `resources.max_decode_jobs` — decode budget hint
- `resources.decision_hold_ms` — hold time for queue slots before churn
- `profiles[]` — named operating profiles/intent metadata

In phase 1, the engine stays backward compatible, but the config model now reflects the intended separation between:
- acquisition
- surveillance analysis
- local refinement
- resource policy
- presentation
- operator goals / future autonomous intent

The long-term target is that you describe *what the system should do* (for example broad-span monitoring intent, preferred signal families, auto-record/decode priorities), while the engine decides *how* to allocate surveillance, refinement and decoding budgets.

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
- `GET /api/pipeline/policy`
- `GET /api/pipeline/recommendations`
- `GET /api/refinement` → latest refinement plan/windows snapshot (includes `window_stats`, `queue_stats`, `decision_summary`, `decision_items`, levels)

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
