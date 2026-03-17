# SDR Visual Suite

Go-based SDRplay RSP1b live spectrum + waterfall visualizer with a minimal event recorder.

## Features
- Live spectrum + waterfall web UI (WebSocket streaming)
- Event timeline view (time vs frequency) with detail drawer
- In-browser spectrogram slice for selected events
- Basic detector with event JSONL output (`data/events.jsonl`)
- Events API (`/api/events?limit=...&since=...`)
- Runtime UI controls for center frequency, span, sample rate, tuner bandwidth, FFT size, gain, AGC, DC block, IQ balance, detector threshold
- Display controls: averaging + max-hold
- Recorded clips list placeholder (metadata only for now)
- Windows + Linux support
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
- `agc`: enable automatic gain control
- `dc_block`: enable DC blocking filter
- `iq_balance`: enable basic IQ imbalance correction
- `detector.threshold_db`: power threshold in dB
- `detector.min_duration_ms`, `detector.hold_ms`: debounce/merge

## Web UI
The UI is served from `web/` and connects to `/ws` for spectrum frames.

### Controls Panel
Use the right-side controls to adjust center frequency, span (zoom), sample rate, tuner bandwidth, FFT size, gain, AGC, DC block, IQ balance, and detector threshold. Preset buttons provide quick jumps to 40m/20m/17m.

### Event Timeline
- The timeline panel displays recent events (time vs frequency).
- Click any event block to open the detail drawer with event stats and a mini spectrogram slice from the latest frame.

### Config API
- `GET /api/config`: returns the current runtime configuration.
- `POST /api/config`: updates `center_hz`, `sample_rate`, `fft_size`, `gain_db`, and `detector.threshold_db` at runtime.
- `POST /api/sdr/settings`: updates `agc`, `dc_block`, and `iq_balance` at runtime.

### Events API
`/api/events` reads from the JSONL event log and returns the most recent events:
- `limit` (optional): max number of events (default 200, max 2000)
- `since` (optional): Unix milliseconds or RFC3339 timestamp

Example:
```bash
curl "http://localhost:8080/api/events?limit=100&since=1700000000000"
```

## Tests
```bash

go test ./...
```

## Troubleshooting
- If you see `sdrplay support not built`, rebuild with `-tags sdrplay`.
- If the SDRplay library is not found, ensure `CGO_CFLAGS` and `CGO_LDFLAGS` point to the API headers and library.
- Use `--mock` to run without hardware.
