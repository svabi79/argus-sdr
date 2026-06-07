# Argus SDR

> A GPU-accelerated **wideband SDR surveillance receiver**: it watches a whole RF
> band at once and detects, classifies, tracks, and decodes *many* signals
> simultaneously — with a live spectrum/waterfall web UI.

Argus is a Go engine plus a browser UI. Instead of tuning one channel at a time,
it ingests a wide IQ stream, runs spectral surveillance over the whole span,
detects every signal it can, classifies and tracks each one across frames, and
demodulates/decodes them in parallel on the GPU (frequency shift, filtering,
decimation, FM/AM/SSB demod, and the WFM multiplex including stereo and RDS).

*(Go module / working tree: `sdr-wideband-suite`. The project is named **Argus**
after the hundred-eyed watcher — it is built to see the whole band at once.)*

---

## Status (honest)

Argus is **proven on the FM broadcast band**: it detects, classifies, and tracks
the stations across 87.5–108 MHz and decodes them live, including **WFM stereo
lock and RDS** (station name / PI / radiotext) on strong stations, GPU-accelerated
end to end.

It is **not yet a universal scanner.** Detection is still tuned for broadcast FM;
the general-purpose detection & estimation rework (occupied-bandwidth estimation,
a dense/strong-signal benchmark, robust channelization) is in progress —
"**Phase R**". See [`docs/known-issues.md`](docs/known-issues.md) and
[`ROADMAP.md`](ROADMAP.md) for exactly what is and isn't load-bearing.

This is an active, single-maintainer project developed with AI agents under an
explicit [Constitution](CONSTITUTION.md). Expect sharp edges.

---

## Features

- **Wideband, many-signal pipeline** — surveillance over the whole span, not a
  single VFO. Multi-resolution analysis, CFAR detection, cross-frame tracking.
- **Live web UI** — spectrum + waterfall (WebSocket streaming), event timeline
  (time × frequency), live signal list with classifier insight, runtime controls
  (center, span, sample rate, bandwidth, FFT size, gain, AGC, DC block, IQ
  balance, detector settings).
- **GPU-first DSP** — per-signal shift/filter/decimate/FFT/demod and RDS-baseband
  extraction run on a CUDA/cuFFT batch runner; a CPU path exists as a fallback and
  validation oracle.
- **Demod & decode** — WFM (incl. **stereo** + **RDS**), NFM, AM, SSB, CW; live
  demod endpoint and a WebSocket live-listen audio stream.
- **Recording & replay** — IQ/audio recording with a recordings API, plus an
  **IQ capture/replay oracle** (`-capture` / `-replay`) for reproducible,
  hardware-free debugging.
- **Telemetry** — extensive runtime telemetry (counters/gauges/distributions,
  event history, pprof) for performance and DSP debugging.
- **Mock mode** — synthetic IQ source; runs with no hardware.

---

## Requirements

| | |
|---|---|
| **Core** | Go **1.22** (see `go.mod`) |
| **Real device** (optional) | SDRplay API (RSP-series; reference hardware is an RSP1B) |
| **GPU** (optional) | CUDA toolkit (cuFFT); `nvcc` (Linux) or `build-gpudemod-dll.ps1` (Windows) |
| **Windows CGO** | MSYS2 MinGW64 (`gcc`/`g++`) for the real-device / GPU build |

Build tags select capability: **(none)** = pure-Go mock build, **`sdrplay`** =
real device, **`cufft`** = GPU demod. The full app uses `sdrplay,cufft`.

---

## Quick start (mock, no hardware)

```bash
go run ./cmd/sdrd --mock
# open http://localhost:8080
```

---

## Build & run

### Mock / no device (any platform)

```bash
go build ./cmd/sdrd
./sdrd --mock
```

### Windows — real device + GPU (recommended path)

```powershell
powershell -ExecutionPolicy Bypass -File .\build-cuda-windows.ps1   # builds gpudemod_kernels.dll
powershell -ExecutionPolicy Bypass -File .\build-sdrplay.ps1        # builds sdrd.exe (tags sdrplay,cufft)
powershell -ExecutionPolicy Bypass -File .\start-sdr.ps1            # sets PATH for CUDA/SDRplay DLLs and runs
```

The build scripts expect MinGW at `C:\msys64\mingw64\bin` and copy the CUDA
runtime DLLs next to `sdrd.exe`. If the SDRplay device fails to initialize after
a restart, `Restart-Service SDRplayAPIService -Force` and relaunch.

### Linux — real device

```bash
export CGO_CFLAGS='-I/opt/sdrplay_api/include'
export CGO_LDFLAGS='-L/opt/sdrplay_api/lib -lsdrplay_api'
go build -tags sdrplay ./cmd/sdrd
./cmd/sdrd/sdrd -config config.yaml
# GPU kernels: ./build-cuda-linux.sh
```

> **Before a build/test session, stop a running `sdrd` and the browser UI** to
> avoid file locks and misleading live state (see [`AGENTS.md`](AGENTS.md)).

---

## Capture & replay (the debugging oracle)

```bash
sdrd -capture data/snapshots/fm_bc.cf32 -capture-seconds 20 -capture-center 101.5
sdrd -replay  data/snapshots/fm_bc.cf32     # real-time-paced replay of the capture
```

Diagnosing against a fixed capture (and the `-tags bench` `TestReal*` tests) is
how DSP/detection work is verified — see Constitution Principle IV. Captures are
large and git-ignored.

---

## Configuration

Runtime config is `config.yaml`; the running app autosaves to
`config.autosave.yaml` (git-ignored — never commit it). The model separates
acquisition, surveillance analysis, refinement, resource policy, presentation,
and operator goals. Operating profiles (`wideband-balanced`,
`wideband-aggressive`, `archive`, `digital-hunting`) bundle sensible defaults.
The full field reference lives in the config file's comments and `docs/`.

---

## Web API (selected)

| Group | Endpoints |
|---|---|
| Config | `GET/POST /api/config`, `POST /api/sdr/settings`, `GET /api/gpu` |
| Pipeline | `GET /api/pipeline/policy`, `/api/pipeline/recommendations`, `/api/refinement` |
| Signals | `GET /api/signals`, `GET /api/events?limit=&since=` |
| Recordings | `GET /api/recordings`, `/api/recordings/:id[/iq|/audio|/decode]` |
| Live audio | `GET /api/demod?freq=&bw=&mode=&sec=`, `WS /ws/audio?freq=&bw=&mode=` |
| Telemetry | `GET /api/debug/telemetry/{live,history,events,config}` |

Full telemetry reference: [`docs/telemetry-api.md`](docs/telemetry-api.md).

---

## Architecture (one screen)

```
IQ source (SDRplay | mock | replay)
        │
   acquisition ─► ring buffer (recording/replay)
        │
  surveillance  (multi-resolution spectra, noise floor, CFAR detection)
        │
   detection ─► cross-frame tracker (stable per-signal IDs)
        │
  refinement  (re-estimate, classify, per-signal IQ snippet)
        │
   demod/decode on GPU  (shift/filter/decimate · WFM·NFM·AM·SSB·CW · stereo · RDS)
        │
  streaming (web UI, live-listen) · recording · telemetry
```

Key packages: `cmd/sdrd` (daemon + HTTP/WS), `internal/detector`,
`internal/pipeline`, `internal/classifier`, `internal/demod` (+ `gpudemod`),
`internal/recorder`, `internal/dsp`, `internal/telemetry`, `web/`.

---

## Contributing & governance

This repo is built to be worked on by AI agents as well as humans, with the gates
enforced by process rather than trust:

- **[`CONSTITUTION.md`](CONSTITUTION.md)** — the inviolable principles (each one
  encodes a real failure this project shipped and fixed). Read first.
- **[`AGENTS.md`](AGENTS.md)** — the operational guide (build, test, branch,
  debug, what not to commit).
- **[`docs/agent-workflow.md`](docs/agent-workflow.md)** — how work flows: issues
  → claim → PR → CI → operator merge.

CI (GitHub Actions) runs `go vet` + tagless build + tagless tests on every PR; the
full GPU build and replay-oracle tests run on a self-hosted GPU runner.

```bash
go test ./...        # tagless sweep (GPU-only tests skip without cufft)
go vet ./...
```

---

## License

[MIT](LICENSE) © 2026 Jan Svabenik.
