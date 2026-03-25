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
