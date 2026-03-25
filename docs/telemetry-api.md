# Telemetry API Reference

This document describes the server-side telemetry collector, its runtime configuration, and the HTTP API exposed by `sdrd`.

The telemetry system is intended for debugging and performance analysis of the SDR pipeline, especially around source cadence, extraction, DSP timing, boundary artifacts, queue pressure, and other runtime anomalies.

## Goals

The telemetry layer gives you three different views of runtime state:

1. **Live snapshot**
   - Current counters, gauges, distributions, recent events, and collector status.
2. **Historical metrics**
   - Timestamped metric samples that can be filtered by name, prefix, or tags.
3. **Historical events**
   - Structured anomalies / warnings / debug events with optional fields.

It is designed to be lightweight in normal operation and more detailed when `heavy_enabled` is turned on.

---

## Base URLs

All telemetry endpoints live under:

- `/api/debug/telemetry/live`
- `/api/debug/telemetry/history`
- `/api/debug/telemetry/events`
- `/api/debug/telemetry/config`

Responses are JSON.

---

## Data model

### Metric types

Telemetry metrics are stored in three logical groups:

- **counter**
  - Accumulating values, usually incremented over time.
- **gauge**
  - Latest current value.
- **distribution**
  - Observed numeric samples with summary stats.

A historical metric sample is returned as:

```json
{
  "ts": "2026-03-25T12:00:00Z",
  "name": "stage.extract_stream.duration_ms",
  "type": "distribution",
  "value": 4.83,
  "tags": {
    "stage": "extract_stream",
    "signal_id": "1"
  }
}
```

### Events

Telemetry events are structured anomaly/debug records:

```json
{
  "id": 123,
  "ts": "2026-03-25T12:00:02Z",
  "name": "demod_boundary",
  "level": "warn",
  "message": "boundary discontinuity detected",
  "tags": {
    "signal_id": "1",
    "stage": "demod"
  },
  "fields": {
    "d2": 0.3358,
    "index": 25
  }
}
```

### Tags

Tags are string key/value metadata used for filtering and correlation.

Common tag keys already supported by the HTTP layer:

- `signal_id`
- `session_id`
- `stage`
- `trace_id`
- `component`

You can also filter on arbitrary tags via `tag_<key>=<value>` query parameters.

---

## Endpoint: `GET /api/debug/telemetry/live`

Returns a live snapshot of the in-memory collector state.

### Response shape

```json
{
  "now": "2026-03-25T12:00:05Z",
  "started_at": "2026-03-25T11:52:10Z",
  "uptime_ms": 472500,
  "config": {
    "enabled": true,
    "heavy_enabled": false,
    "heavy_sample_every": 12,
    "metric_sample_every": 2,
    "metric_history_max": 12000,
    "event_history_max": 4000,
    "retention": 900000000000,
    "persist_enabled": false,
    "persist_dir": "debug/telemetry",
    "rotate_mb": 16,
    "keep_files": 8
  },
  "counters": [
    {
      "name": "source.resets",
      "value": 1,
      "tags": {
        "component": "source"
      }
    }
  ],
  "gauges": [
    {
      "name": "source.buffer_samples",
      "value": 304128,
      "tags": {
        "component": "source"
      }
    }
  ],
  "distributions": [
    {
      "name": "dsp.frame.duration_ms",
      "count": 96,
      "min": 82.5,
      "max": 212.4,
      "mean": 104.8,
      "last": 98.3,
      "p95": 149.2,
      "tags": {
        "stage": "dsp"
      }
    }
  ],
  "recent_events": [],
  "status": {
    "source_state": "running"
  }
}
```

### Notes

- `counters`, `gauges`, and `distributions` are sorted by metric name.
- `recent_events` contains the most recent in-memory event slice.
- `status` is optional and contains arbitrary runtime status published by code using `SetStatus(...)`.
- If telemetry is unavailable, the server returns a small JSON object instead of a full snapshot.

### Typical uses

- Check whether telemetry is enabled.
- Look for timing hotspots in `*.duration_ms` distributions.
- Inspect current queue or source gauges.
- See recent anomaly events without querying history.

---

## Endpoint: `GET /api/debug/telemetry/history`

Returns historical metric samples from in-memory history and, optionally, persisted JSONL files.

### Response shape

```json
{
  "items": [
    {
      "ts": "2026-03-25T12:00:01Z",
      "name": "stage.extract_stream.duration_ms",
      "type": "distribution",
      "value": 5.2,
      "tags": {
        "stage": "extract_stream",
        "signal_id": "2"
      }
    }
  ],
  "count": 1
}
```

### Supported query parameters

#### Time filters

- `since`
- `until`

Accepted formats:

- Unix seconds
- Unix milliseconds
- RFC3339
- RFC3339Nano

Examples:

- `?since=1711368000`
- `?since=1711368000123`
- `?since=2026-03-25T12:00:00Z`

#### Result shaping

- `limit`
  - Default normalization is 500.
  - Values above 5000 are clamped down by the collector query layer.

#### Name filters

- `name=<exact_metric_name>`
- `prefix=<metric_name_prefix>`

Examples:

- `?name=source.read.duration_ms`
- `?prefix=stage.`
- `?prefix=iq.extract.`

#### Tag filters

Special convenience query params map directly to tag filters:

- `signal_id`
- `session_id`
- `stage`
- `trace_id`
- `component`

Arbitrary tag filters:

- `tag_<key>=<value>`

Examples:

- `?signal_id=1`
- `?stage=extract_stream`
- `?tag_path=gpu`
- `?tag_zone=broadcast`

#### Persistence control

- `include_persisted=true|false`
  - Default: `true`

When enabled and persistence is active, the server reads matching data from rotated JSONL telemetry files in addition to in-memory history.

### Notes

- Results are sorted by timestamp ascending.
- If `limit` is hit, the most recent matching items are retained.
- Exact retention depends on both in-memory retention and persisted file availability.
- A small set of boundary-related IQ metrics is force-stored regardless of the normal metric sample cadence.

### Typical queries

Get all stage timing since a specific start:

```text
/api/debug/telemetry/history?since=2026-03-25T12:00:00Z&prefix=stage.
```

Get extraction metrics for a single signal:

```text
/api/debug/telemetry/history?since=2026-03-25T12:00:00Z&prefix=extract.&signal_id=2
```

Get source cadence metrics only from in-memory history:

```text
/api/debug/telemetry/history?prefix=source.&include_persisted=false
```

---

## Endpoint: `GET /api/debug/telemetry/events`

Returns historical telemetry events from memory and, optionally, persisted storage.

### Response shape

```json
{
  "items": [
    {
      "id": 991,
      "ts": "2026-03-25T12:00:03Z",
      "name": "source_reset",
      "level": "warn",
      "message": "source reader reset observed",
      "tags": {
        "component": "source"
      },
      "fields": {
        "reason": "short_read"
      }
    }
  ],
  "count": 1
}
```

### Supported query parameters

All `history` filters are also supported here, plus:

- `level=<debug|info|warn|error|...>`

Examples:

- `?since=2026-03-25T12:00:00Z&level=warn`
- `?prefix=audio.&signal_id=1`
- `?name=demod_boundary&signal_id=1`

### Notes

- Event matching supports `name`, `prefix`, `level`, time range, and tags.
- Event `level` matching is case-insensitive.
- Results are timestamp-sorted ascending.

### Typical queries

Get warnings during a reproduction run:

```text
/api/debug/telemetry/events?since=2026-03-25T12:00:00Z&level=warn
```

Get boundary-related events for one signal:

```text
/api/debug/telemetry/events?since=2026-03-25T12:00:00Z&signal_id=1&prefix=demod_
```

---

## Endpoint: `GET /api/debug/telemetry/config`

Returns both:

1. the active collector configuration, and
2. the current runtime config under `debug.telemetry`

### Response shape

```json
{
  "collector": {
    "enabled": true,
    "heavy_enabled": false,
    "heavy_sample_every": 12,
    "metric_sample_every": 2,
    "metric_history_max": 12000,
    "event_history_max": 4000,
    "retention": 900000000000,
    "persist_enabled": false,
    "persist_dir": "debug/telemetry",
    "rotate_mb": 16,
    "keep_files": 8
  },
  "config": {
    "enabled": true,
    "heavy_enabled": false,
    "heavy_sample_every": 12,
    "metric_sample_every": 2,
    "metric_history_max": 12000,
    "event_history_max": 4000,
    "retention_seconds": 900,
    "persist_enabled": false,
    "persist_dir": "debug/telemetry",
    "rotate_mb": 16,
    "keep_files": 8
  }
}
```

### Important distinction

- `collector.retention` is a Go duration serialized in nanoseconds.
- `config.retention_seconds` is the config-facing field used by YAML and the POST update API.

If you are writing tooling, prefer `config.retention_seconds` for human-facing config edits.

---

## Endpoint: `POST /api/debug/telemetry/config`

Updates telemetry settings at runtime and writes them back via the autosave config path.

### Request body

All fields are optional. Only provided fields are changed.

```json
{
  "enabled": true,
  "heavy_enabled": true,
  "heavy_sample_every": 8,
  "metric_sample_every": 1,
  "metric_history_max": 20000,
  "event_history_max": 6000,
  "retention_seconds": 1800,
  "persist_enabled": true,
  "persist_dir": "debug/telemetry",
  "rotate_mb": 32,
  "keep_files": 12
}
```

### Response shape

```json
{
  "ok": true,
  "collector": {
    "enabled": true,
    "heavy_enabled": true,
    "heavy_sample_every": 8,
    "metric_sample_every": 1,
    "metric_history_max": 20000,
    "event_history_max": 6000,
    "retention": 1800000000000,
    "persist_enabled": true,
    "persist_dir": "debug/telemetry",
    "rotate_mb": 32,
    "keep_files": 12
  },
  "config": {
    "enabled": true,
    "heavy_enabled": true,
    "heavy_sample_every": 8,
    "metric_sample_every": 1,
    "metric_history_max": 20000,
    "event_history_max": 6000,
    "retention_seconds": 1800,
    "persist_enabled": true,
    "persist_dir": "debug/telemetry",
    "rotate_mb": 32,
    "keep_files": 12
  }
}
```

### Persistence behavior

A POST updates:

- the runtime manager snapshot/config
- the in-process collector config
- the autosave config file via `config.Save(...)`

That means these updates are runtime-effective immediately and also survive restarts through autosave, unless manually reverted.

### Error cases

- Invalid JSON -> `400 Bad Request`
- Invalid collector reconfiguration -> `400 Bad Request`
- Telemetry unavailable -> `503 Service Unavailable`

---

## Configuration fields (`debug.telemetry`)

Telemetry config lives under:

```yaml
debug:
  telemetry:
    enabled: true
    heavy_enabled: false
    heavy_sample_every: 12
    metric_sample_every: 2
    metric_history_max: 12000
    event_history_max: 4000
    retention_seconds: 900
    persist_enabled: false
    persist_dir: debug/telemetry
    rotate_mb: 16
    keep_files: 8
```

### Field reference

#### `enabled`
Master on/off switch for telemetry collection.

If false:
- metrics are not recorded
- events are not recorded
- live snapshot remains effectively empty/minimal

#### `heavy_enabled`
Enables more expensive / more detailed telemetry paths that should not be left on permanently unless needed.

Use this for deep extractor/IQ/boundary debugging.

#### `heavy_sample_every`
Sampling cadence for heavy telemetry.

- `1` means every eligible heavy sample
- higher numbers reduce cost by sampling less often

#### `metric_sample_every`
Sampling cadence for normal historical metric point storage.

Collector summaries still update live, but historical storage becomes less dense when this value is greater than 1.

#### `metric_history_max`
Maximum number of in-memory historical metric samples retained.

#### `event_history_max`
Maximum number of in-memory telemetry events retained.

#### `retention_seconds`
Time-based in-memory retention window.

Older in-memory metrics/events are trimmed once they fall outside this retention period.

#### `persist_enabled`
When enabled, telemetry metrics/events are also appended to rotated JSONL files.

#### `persist_dir`
Directory where rotated telemetry JSONL files are written.

Default:

- `debug/telemetry`

#### `rotate_mb`
Approximate JSONL file rotation threshold in megabytes.

#### `keep_files`
How many rotated telemetry files to retain in `persist_dir`.

Older files beyond this count are pruned.

---

## Collector behavior and caveats

### In-memory vs persisted data

The query endpoints can read from both:

- current in-memory collector state/history
- persisted JSONL files

This means a request may return data older than current in-memory retention if:

- `persist_enabled=true`, and
- `include_persisted=true`

### Sampling behavior

Not every observation necessarily becomes a historical metric point.

The collector:

- always updates live counters/gauges/distributions while enabled
- stores historical points according to `metric_sample_every`
- force-stores selected boundary IQ metrics even when sampling would normally skip them

So the live snapshot and historical series density are intentionally different.

### Distribution summaries

Distribution values in the live snapshot include:

- `count`
- `min`
- `max`
- `mean`
- `last`
- `p95`

The p95 estimate is based on the collector's bounded rolling sample buffer, not an unbounded full-history quantile computation.

### Config serialization detail

The collector's `retention` field is a Go duration. In JSON this appears as an integer nanosecond count.

This is expected.

---

## Recommended workflows

### Fast low-overhead runtime watch

Use:

- `enabled=true`
- `heavy_enabled=false`
- `persist_enabled=false` or `true` if you want an archive

Then query:

- `/api/debug/telemetry/live`
- `/api/debug/telemetry/history?prefix=stage.`
- `/api/debug/telemetry/events?level=warn`

### 5-10 minute anomaly capture

Suggested settings:

- `enabled=true`
- `heavy_enabled=false`
- `persist_enabled=true`
- moderate `metric_sample_every`

Then:

1. note start time
2. reproduce workload
3. fetch live snapshot
4. inspect warning events
5. inspect `stage.*`, `streamer.*`, and `source.*` history

### Deep extractor / boundary investigation

Temporarily enable:

- `heavy_enabled=true`
- `heavy_sample_every` > 1 unless you really need every sample
- `persist_enabled=true`

Then inspect:

- `iq.*`
- `extract.*`
- `audio.*`
- boundary/anomaly events for specific `signal_id` or `session_id`

Turn heavy telemetry back off once done.

---

## Example requests

### Fetch live snapshot

```bash
curl http://localhost:8080/api/debug/telemetry/live
```

### Fetch stage timings from the last 10 minutes

```bash
curl "http://localhost:8080/api/debug/telemetry/history?since=2026-03-25T12:00:00Z&prefix=stage."
```

### Fetch source metrics for one signal

```bash
curl "http://localhost:8080/api/debug/telemetry/history?prefix=source.&signal_id=1"
```

### Fetch warning events only

```bash
curl "http://localhost:8080/api/debug/telemetry/events?since=2026-03-25T12:00:00Z&level=warn"
```

### Fetch events with a custom tag filter

```bash
curl "http://localhost:8080/api/debug/telemetry/events?tag_zone=broadcast"
```

### Enable persistence and heavy telemetry temporarily

```bash
curl -X POST http://localhost:8080/api/debug/telemetry/config \
  -H "Content-Type: application/json" \
  -d '{
    "heavy_enabled": true,
    "heavy_sample_every": 8,
    "persist_enabled": true
  }'
```

---

## Related docs

- `README.md` - high-level project overview and endpoint summary
- `docs/telemetry-debug-runbook.md` - quick operational runbook for short debug sessions
- `internal/telemetry/telemetry.go` - collector implementation details
- `cmd/sdrd/http_handlers.go` - HTTP wiring for telemetry endpoints
