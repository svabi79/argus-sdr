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
- Phase-1 architecture foundation complete: explicit pipeline/surveillance/refinement/resources config model plus candidate/refinement/admission scaffolding

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

### Phase-1 pipeline fields
- `pipeline.mode` -- operating mode label (`legacy`, `wideband-balanced`, ...)
- `pipeline.profile` -- last applied operating profile name (if any)
- `pipeline.goals.*` -- declarative target/intent layer for future autonomous operation
  - `intent`
  - `monitor_start_hz` / `monitor_end_hz` / `monitor_span_hz`
  - `signal_priorities`
  - `auto_record_classes`
  - `auto_decode_classes`
- `surveillance.analysis_fft_size` -- analysis FFT size used by the surveillance layer
- `surveillance.frame_rate` -- surveillance cadence target
- `surveillance.strategy` -- `single-resolution` or `multi-resolution`
- `surveillance.display_bins` -- preferred presentation density for clients/UI
- `surveillance.display_fps` -- preferred presentation cadence for clients/UI
- `refinement.enabled` -- enables explicit candidate refinement stage
- `refinement.max_concurrent` -- refinement budget hint
- `refinement.detail_fft_size` -- FFT size for refinement/detail path (defaults to surveillance analysis FFT)
- `refinement.min_candidate_snr_db` -- floor for future scheduling decisions
- `refinement.min_span_hz` / `refinement.max_span_hz` -- clamp refinement window span (0 = no clamp)
- `refinement.auto_span` -- use mod-type heuristics when candidate bandwidth is missing/odd
- `resources.prefer_gpu` -- GPU preference hint

**Operating profiles (wideband)**
- `wideband-balanced`: multi-resolution, 4096 surveillance/detail FFT, refinement span 4000-200000 Hz
- `wideband-aggressive`: multi-resolution, 8192 surveillance/detail FFT, refinement span 6000-250000 Hz
- `archive`: record-forward bias, higher record/decode budgets, 4096 detail FFT
- `digital-hunting`: digital-first priorities and decode bias
- `resources.max_refinement_jobs` -- processing budget hint
- `resources.max_recording_streams` -- recording/streaming budget hint
- `resources.max_decode_jobs` -- decode budget hint
- `resources.decision_hold_ms` -- baseline hold time for queue slots before churn (arbitration scales per profile/strategy and tags hold reasons in debug snapshots)
- `profiles[]` -- named operating profiles/intent metadata

Phase 1 stays backward compatible, but the config model now reflects the intended separation between:
- acquisition
- surveillance analysis
- local refinement
- resource policy
- presentation
- operator goals / future autonomous intent

Refinement plans now rank candidates, while a shared arbitration step admits refinement/record/decode work based on budgets and hold policy. Arbitration reasons are normalized:
- `refinement:*` for work item lifecycle (planned/admitted/running/completed/drop/skip)
- `admission:*` for refinement admission outcomes
- `decision:*` for record/decode decisions
- `queue:*` when record/decode is deferred by budget
Hold policy reasons are surfaced as `profile:*` / `strategy:*` tokens in `hold_source`.

Phase-1 scope stops at consistent policy surfaces, ranking/admission scaffolding, and debug visibility. Phase 2+ adds a true multi-resolution surveillance engine and scheduler/intent overrides that can re-balance budgets automatically.

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

### Windows (GPU + SDRplay) -- recommended path
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
- `GET /api/refinement` -> latest refinement plan/windows snapshot (includes `window_stats`, levels, request/context/work_items, plus `arbitration` with budgets/hold policy/refinement admission/queue/decision summary)

### Signals / Events
- `GET /api/signals` -> current live signals
- `GET /api/events?limit=&since=` -> recent events

### Recordings
- `GET /api/recordings`
- `GET /api/recordings/:id` (meta.json)
- `GET /api/recordings/:id/iq`
- `GET /api/recordings/:id/audio`
- `GET /api/recordings/:id/decode?mode=FT8|WSPR|DMR|D-STAR|FSK|PSK`

### Live Demod / Listen
- `GET /api/demod?freq=...&bw=...&mode=...&sec=...` -> audio/wav
- `WS /ws/audio?freq=...&bw=...&mode=...` -> live PCM audio stream

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
- `sdrplay support not built` -> rebuild with `-tags sdrplay`.
- SDRplay library not found -> check `CGO_CFLAGS` / `CGO_LDFLAGS`.
- GPU demod not loading -> verify `gpudemod_kernels.dll` / `cudart64_13.dll` next to `sdrd.exe` (Windows).
- Use `--mock` to run without hardware.
