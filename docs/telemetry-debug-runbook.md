# Debug Telemetry Runbook

This project now includes structured server-side telemetry for the audio/DSP pipeline.

## Endpoints

- `GET /api/debug/telemetry/live`
  - Current counters/gauges/distributions and recent events.
- `GET /api/debug/telemetry/history`
  - Historical metric samples.
  - Query params:
    - `since`, `until`: unix seconds/ms or RFC3339
    - `limit`
    - `name`, `prefix`
    - `signal_id`, `session_id`, `stage`, `trace_id`, `component`
    - `tag_<key>=<value>` for arbitrary tag filters
    - `include_persisted=true|false`
- `GET /api/debug/telemetry/events`
  - Historical events/anomalies.
  - Same filters as history plus `level`.
- `GET /api/debug/telemetry/config`
  - Active telemetry config from runtime + collector.
- `POST /api/debug/telemetry/config`
  - Runtime config update (also saved to autosave config).

## Config knobs

`debug.telemetry` in config:

- `enabled`
- `heavy_enabled`
- `heavy_sample_every`
- `metric_sample_every`
- `metric_history_max`
- `event_history_max`
- `retention_seconds`
- `persist_enabled`
- `persist_dir`
- `rotate_mb`
- `keep_files`

Persisted JSONL files rotate in `persist_dir` (default: `debug/telemetry`).

## 5-10 minute debug flow

1. Keep `enabled=true`, `heavy_enabled=false`, `persist_enabled=true`.
2. Run workload for 5-10 minutes.
3. Pull live state:
   - `GET /api/debug/telemetry/live`
4. Pull anomalies:
   - `GET /api/debug/telemetry/events?since=<start>&level=warn`
5. Pull pipeline timing and queue/backpressure:
   - `GET /api/debug/telemetry/history?since=<start>&prefix=stage.`
   - `GET /api/debug/telemetry/history?since=<start>&prefix=streamer.`
6. If IQ boundary issues persist, temporarily set `heavy_enabled=true` (keep sampling coarse with `heavy_sample_every` > 1), rerun, then inspect `iq.*` metrics and `audio.*` anomalies by `signal_id`/`session_id`.

## 2026-03-25 audio click incident — final resolved summary

Status: **SOLVED**

The March 2026 live-audio click investigation ultimately converged on a combination of three real root causes plus two secondary fixes:

### Root causes

1. **Shared `allIQ` corruption by `IQBalance` aliasing**
   - `cmd/sdrd/pipeline_runtime.go`
   - `survIQ` aliased the tail of `allIQ`
   - `dsp.IQBalance(survIQ)` modified `allIQ` in-place
   - extractor then saw a corrupted boundary inside the shared buffer
   - final fix: copy `survIQ` before `IQBalance`

2. **Per-frame extractor reset due to `StreamingConfigHash` jitter**
   - `internal/demod/gpudemod/streaming_types.go`
   - smoothed tuning values changed slightly every frame
   - offset/bandwidth in the hash caused repeated state resets
   - final fix: hash only structural parameters

3. **Streaming path batch rejection for non-WFM exact-decimation mismatch**
   - `cmd/sdrd/streaming_refactor.go`
   - one non-WFM signal could reject the whole batch and silently force fallback to the legacy path
   - final fix: choose nearest exact integer-divisor output rate and keep fallback logging visible

### Secondary fixes

- FM discriminator cross-block carry in `internal/recorder/streamer.go`
- WFM mono/plain-path 15 kHz audio lowpass in `internal/recorder/streamer.go`

### Verification notes

- major discontinuities dropped sharply after the config-hash fix
- remaining fine clicks were eliminated only after the `IQBalance` aliasing fix in `pipeline_runtime.go`
- final confirmation was by operator listening test, backed by prior telemetry and WAV analysis

### Practical lesson

When the same captured `allIQ` buffer feeds both:
- surveillance/detail analysis
- and extraction/streaming

then surveillance-side DSP helpers must not mutate a shared sub-slice in-place unless that mutation is intentionally part of the extraction contract.
