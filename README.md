# SDR Visual Suite

Go-based SDRplay RSP1b live spectrum + waterfall visualizer with an event recorder, classifier, and demod/recording pipeline.

## Features
- Live spectrum + waterfall web UI (WebSocket streaming)
- Event timeline view (time vs frequency) with detail drawer
- Event JSONL output (`data/events.jsonl`)
- Runtime UI controls for center frequency, span, sample rate, tuner bandwidth, FFT size, gain, AGC, DC block, IQ balance, detector threshold
- Optional GPU FFT (cuFFT) with toggle + `/api/gpu`
- IQ/audio recording + recordings list
- Live demod endpoint
- WFM stereo + RDS baseband
- Mock mode for testing without hardware

## Quick Start (Mock)
```bash
# From repo root

go run ./cmd/sdrd --mock
```
Open `http://localhost:8080`.

## SDRplay Build/Run (Real Device)
This project uses the SDRplay API via cgo (`sdrplay_api.h`). Ensure the SDRplay API is installed.

### Windows
```powershell
$env:CGO_CFLAGS='-IC:\Program Files\SDRplay\API\inc'
$env:CGO_LDFLAGS='-LC:\Program Files\SDRplay\API\x64 -lsdrplay_api'

go build -tags sdrplay ./cmd/sdrd
.\sdrd.exe -config config.yaml
```

#### Windows (GPU / CUDA + SDRplay)
The only supported Windows build path in this repository is:

```powershell
powershell -ExecutionPolicy Bypass -File .\build-cuda-windows.ps1
powershell -ExecutionPolicy Bypass -File .\build-sdrplay.ps1
```

This path uses:
- `nvcc` + MSVC to build `gpudemod_kernels.dll`
- MinGW GCC/G++ for the Go/CGO application build
- runtime DLL loading for the Windows `gpudemod` path

Important:
- `gpudemod_kernels.dll` is copied next to `sdrd.exe` by `build-sdrplay.ps1`
- `build-sdrplay.ps1` prepares the runtime DLL placement and PATH setup for SDRplay + CUDA DLLs
- the gpudemod DLL is built with `-cudart=hybrid`
- GPU validation is disabled by default for performance; enable it with `SDR_GPU_VALIDATE=1` when debugging
- you can override DLL lookup with `GPUMOD_DLL=C:\path\to\gpudemod_kernels.dll`
- Windows builds may still show a harmless `__cdecl redefined` warning from CUDA headers
- older experimental Windows build scripts were removed to avoid confusion

### Linux
```bash
export CGO_CFLAGS='-I/opt/sdrplay_api/include'
export CGO_LDFLAGS='-L/opt/sdrplay_api/lib -lsdrplay_api'

go build -tags sdrplay ./cmd/sdrd
./cmd/sdrd/sdrd -config config.yaml
```

## Configuration
Edit `config.yaml`:
- `bands`: list of band ranges
- `center_hz`: center frequency
- `sample_rate`: sample rate
- `fft_size`: FFT size
- `gain_db`: device gain (gain reduction)
- `tuner_bw_khz`: tuner bandwidth (200/300/600/1536/5000/6000/7000/8000)
- `use_gpu_fft`: enable GPU FFT (requires CUDA + cufft build tag)
- `agc`: enable automatic gain control
- `dc_block`: enable DC blocking filter
- `iq_balance`: enable basic IQ imbalance correction
- `detector.threshold_db`: power threshold in dB (fallback if CFAR disabled)
- `detector.cfar_enabled`: enable OS-CFAR detection
- `detector.cfar_guard_cells`, `detector.cfar_train_cells`, `detector.cfar_rank`, `detector.cfar_scale_db`: OS-CFAR window + ordered-statistic parameters
- `detector.min_duration_ms`, `detector.hold_ms`: debounce/merge
- `recorder.*`: enable IQ/audio recording, preroll, output_dir, max_disk_mb
- `decoder.*`: external decode commands (use `{iq}`, `{audio}`, `{sr}` placeholders)

## APIs
### Config API
- `GET /api/config`
- `POST /api/config`
- `POST /api/sdr/settings`
- `GET /api/gpu`

### Events API
`/api/events` reads from the JSONL event log:
- `limit` (optional): max number of events (default 200, max 2000)
- `since` (optional): Unix milliseconds or RFC3339 timestamp

### Signals API
- `GET /api/signals` → current live signals (latest snapshot)

### Recordings API
- `GET /api/recordings`
- `GET /api/recordings/:id` (meta.json)
- `GET /api/recordings/:id/iq`
- `GET /api/recordings/:id/audio`
- `GET /api/recordings/:id/decode?mode=FT8|WSPR|DMR|D-STAR|FSK|PSK`

### Live Demod API
- `GET /api/demod?freq=...&bw=...&mode=...&sec=...` → audio/wav

## Decoder Tools
Put external decoder binaries/scripts under `tools/` and configure `decoder.*` in `config.yaml`.
See `tools/README.md` for examples.

## Tests
```bash

go test ./...
```

## Troubleshooting
- If you see `sdrplay support not built`, rebuild with `-tags sdrplay`.
- If the SDRplay library is not found, ensure `CGO_CFLAGS` and `CGO_LDFLAGS` point to the API headers and library.
- Use `--mock` to run without hardware.
