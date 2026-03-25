# Audio Click Debug Notes — 2026-03-24

## Context

This note captures the intermediate findings from the live/recording audio click investigation on `sdr-wideband-suite`.

Goal: preserve the reasoning, experiments, false leads, and current best understanding so future work does not restart from scratch.

---

## High-level outcome so far

We do **not** yet have the final root cause.

But we now know substantially more about what the clicks are **not**, and we identified at least one real bug plus several strong behavioral constraints in the pipeline.

---

## What was tested

### 1. Session/context recovery
- Reconstructed prior debugging context from reset-session backup files.
- Confirmed the relevant investigation was the persistent audio clicking bug in live audio / recordings.

### 2. Codebase deep-read
Reviewed in detail:
- `cmd/sdrd/dsp_loop.go`
- `cmd/sdrd/pipeline_runtime.go`
- `cmd/sdrd/helpers.go`
- `internal/recorder/streamer.go`
- `internal/recorder/demod_live.go`
- `internal/dsp/fir.go`
- `internal/dsp/fir_stateful.go`
- `internal/dsp/resample.go`
- `internal/demod/fm.go`
- `internal/demod/gpudemod/*`
- `web/app.js`

Main conclusion from static reading: the pipeline contains several stateful continuity mechanisms, so clicks are likely to emerge at boundaries or from phase/timing inconsistencies rather than from one obvious isolated bug.

### 3. AM vs FM tests
Observed by ear:
- AM clicks too.
- Therefore this is **not** an FM-only issue.
- That shifted focus away from purely FM-specific explanations and toward shared-path / continuity / transport / demod-adjacent causes.

### 4. Recording vs live path comparison
Observed by ear:
- Recordings click too.
- Therefore browser/WebSocket/live playback is **not** the sole cause.
- The root problem exists in the server-side audio pipeline before browser playback.

### 5. Boundary instrumentation added
Temporary diagnostics were added to inspect:
- extract trimming
- snippet lengths
- demod path lengths
- boundary click / intra-click detector
- IQ continuity at various stages

### 6. Discriminator-overlap hypothesis
A test switch temporarily disabled the extra 1-sample discriminator overlap prepend in `streamer.go`.

Result:
- This extra overlap **was** a real problem.
- It caused the downstream decimation phase to flip between blocks.
- Removing it cleaned up the boundary model and was the correct change.

However:
- Removing it did **not** eliminate the audible clicks.
- Therefore it was a real bug, but **not the main remaining root cause**.

### 7. GPU vs CPU extraction test
Forced CPU-only stream extraction.

Result:
- CPU-only made things dramatically worse in real time.
- Large `feed_gap` values appeared.
- Huge backlogs built up.
- Therefore CPU-only is not a solution, and the GPU path is not the sole main problem.

### 8. Fixed read-size test
Forced a constant extraction read size (`389120`) instead of variable read sizing based on backlog.

Result:
- `allIQ`, `gpuIQ_len`, `raw_len`, and `out_len` became very stable.
- This reduced pipeline variability and made logs much cleaner.
- Subjectively, audio may have become slightly better, but clicks remained.
- Therefore variable block sizing is likely a contributing factor, but not the full explanation.

### 9. Multi-stage audio dump test
Added optional debug dumping for:
- demod audio (`*-demod.wav`)
- final audio after resampler (`*-final.wav`)

Observed by ear:
- Clicks are present in **both** dump types.
- Therefore the click is already present by the time demodulated audio exists.
- Resampler/final audio path is not the primary origin.

### 10. CPU monitoring
A process-level CSV monitor was added and used.

Result:
- Overall process CPU usage was modest (not near full machine saturation).
- This does **not** support “overall CPU is pegged” as the main explanation.
- Caveat: this does not fully exclude a hot thread or scheduler issue, but gross total CPU overload is not the main story.

---

## What we now know with reasonable confidence

### A. The issue is not primarily caused by:
- Browser playback
- WebSocket transport
- Final PCM fanout only
- Resampler alone
- CPU-only vs GPU-only as the core dichotomy
- The old extra discriminator overlap prepend (that was a bug, but not the remaining dominant one)
- Purely variable block sizes alone
- Gross whole-process CPU saturation

### B. The issue is server-side and exists before final playback
Because:
- recordings click
- demod dump clicks
- final dump clicks

### C. The issue is present by the demodulated audio stage
This is one of the strongest current findings.

### D. The WFM/FM-demod-adjacent path remains highly suspicious
Current best area of suspicion:
- decimated IQ may still contain subtle corruption/instability not fully captured by current metrics
- OR the FM discriminator (`fmDiscrim`) is producing pathological output from otherwise “boundary-clean-looking” IQ

---

## Important runtime/pathology observations

### 1. Backlog amplification is real
Several debug runs showed severe buffer growth and drops:
- large `buf=` values
- growing `drop=` counts
- repeated `audio_gap`

This means some debug configurations can easily become self-distorting and produce additional artifacts that are not representative of the original bug.

### 2. Too much debug output causes self-inflicted load
At one point:
- rate limiter was disabled (`rate_limit_ms: 0`)
- aggressive boundary logging was enabled
- many short WAV files were generated

This clearly increased overhead and likely polluted some runs.

### 3. Many short WAVs were a bad debug design
That was replaced with a design intended to write one continuous window file instead of many micro-files.

### 4. Total process CPU saturation does not appear to be the main cause
A process-level CSV monitor was collected and showed only modest total CPU utilisation during the relevant tests.
This does **not** support a simple “the machine is pegged” explanation.
A hot thread / scheduling issue is still theoretically possible, but gross overall CPU overload is not the main signal.

---

## Current debug state in repo

### Branch
All current work is on:
- `debug/audio-clicks`

### Commits so far
- `94c132d` — `debug: instrument audio click investigation`
- `ffbc45d` — `debug: add advanced boundary metering`

### Current config/logging state
The active debug logging was trimmed down to:
- `demod`
- `discrim`
- `gap`
- `boundary`

Rate limit is currently back to a nonzero value to avoid self-induced spam.

### Dump/CPU debug state
A `debug:` config section was added with:
- `audio_dump_enabled: false`
- `cpu_monitoring: false`

Meaning:
- heavy WAV dumping is now OFF by default
- CPU monitoring is conceptually OFF by default (script still exists, but must be explicitly used)

---

## Most important code changes/findings to remember

### 1. Removed the extra discriminator overlap prepend in `streamer.go`
This was a correct fix.

Reason:
- it introduced a blockwise extra IQ sample
- this shifted decimation phase between blocks
- it created real boundary artifacts

This should **not** be reintroduced casually.

### 2. Fixed read-size test exists and is useful for investigation
A temporary mechanism exists to force stable extraction block sizes.
This is useful diagnostically because it removes one source of pipeline variability.

**IMPORTANT DECISION / DO NOT LOSE:**
- The fixed read-size path currently lives behind the environment variable `SDR_FORCE_FIXED_STREAM_READ_SAMPLES`.
- The tested value `389120` clearly helps by making `allIQ`, `gpuIQ_len`, `raw_len`, and `out_len` much more stable and by reducing one major source of pipeline variability.
- Current plan: **once the remaining click root cause is solved, promote this behavior into the normal code path instead of leaving it as an env-var-only debug switch.**
- In other words: treat fixed read sizing as a likely permanent stabilization improvement, but do not bake it in blindly until the click investigation is complete.

### 3. FM discriminator metering exists
`internal/demod/fm.go` now emits targeted discriminator stats under `discrim` logging, including:
- min/max IQ magnitude
- maximum absolute phase step
- count of large phase steps

This was useful to establish that large discriminator steps correlate with low IQ magnitude, but discriminator logging was later disabled from the active category list to reduce log spam.

### 4. Strong `dec`-IQ findings before demod
Additional metering in `streamer.go` showed:
- repeated `dec_iq_head_dip`
- repeated low magnitude near `min_idx ~= 25`
- repeated early large local phase step near `max_step_idx ~= 24`
- repeated `demod_boundary` and audible clicks shortly afterward

This is the strongest currently known mechanism in the chain.

### 5. Group delay observation
For the current pre-demod FIR:
- taps = `101`
- FIR group delay = `(101 - 1) / 2 = 50` input samples
- with `decim1 = 2`, this projects to about `25` output samples

This matches the repeatedly observed problematic `dec` indices (~24-25) remarkably well.
That strongly suggests the audible issue is connected to the FIR/decimation settling region at the beginning of the `dec` block.

### 6. Pre-FIR vs post-FIR comparison
A dedicated pre-FIR probe was added on `fullSnip` (the input to the pre-demod FIR) and compared against the existing `dec`-side probes.

Observed pattern:
- pre-FIR head probe usually looked relatively normal
- no equally strong or equally reproducible hot spot appeared there
- after FIR + decimation, the problematic dip/step repeatedly appeared near `dec` indices ~24-25

Interpretation:
- the strongest currently observed defect is **not already present in the same form before the FIR**
- it is much more likely to emerge in the FIR/decimation section (or its settling behavior) than in the raw pre-FIR input

### 7. Head-trim test results
A debug head-trim on `dec` was tested.
Subjective result:
- `trim=32` sounded best among the tested values (`16/32/48/64`)
- but it did **not** remove the clicks entirely

Interpretation:
- the early `dec` settling region is a real contributor
- but it is probably not the only contributor, or trimming alone is not the final correct fix

### 8. Current architectural conclusion
The likely clean fix is **not** to keep trimming samples away.
The FIR/decimation section is still suspicious, but later tests showed it is likely not the sole origin.

Important nuance:
- the currently suspicious FIR + decimation section is already running in **Go/CPU** (`processSnippet`), not in CUDA
- therefore the next correctness fix should be developed and validated in Go first

Later update:
- a stateful decimating FIR / polyphase-style replacement was implemented in Go and tested
- it was architecturally cleaner than the old separated FIR->decimate handoff
- but it did **not** remove the recurring hot spot / clicks
- therefore the old handoff was not the whole root cause, even if the newer path is still cleaner

---

## Best current hypothesis

The remaining audible clicks are most likely generated **at or immediately before FM demodulation**.

Most plausible interpretations:
1. The decimated IQ stream still contains subtle corruption/instability not fully captured by the earliest boundary metrics.
2. The FM discriminator is reacting violently to short abnormal IQ behavior inside blocks, not just at block boundaries.
3. The problematic region is likely a **very specific early decimated-IQ settling zone**, not broad corruption across the whole block.

At this point, the most valuable next data is low-overhead IQ telemetry right before demod, plus carefully controlled demod-vs-final audio comparison.

### Stronger updated working theory (later findings, same day)

After discriminator-focused metering and targeted `dec`-IQ probes, the strongest current theory is:

> A reproducible early defect in the `dec` IQ block appears around sample index **24-25**, where IQ magnitude dips sharply and the effective FM phase step becomes abnormally large. This then shows up as `demod_boundary` and audible clicks.

Crucially:
- this issue appears in `demod.wav`, so it exists before the final resampler/playback path
- it is **not** spread uniformly across the whole `dec` block
- it repeatedly appears near the same index
- trimming the first ~32 samples subjectively reduces the click, but does not eliminate it entirely

This strongly suggests a **settling/transient zone at the beginning of the decimated IQ block**.

Later refinements to this theory:
- pre-FIR probing originally looked cleaner than post-FIR probing, which made FIR/decimation look like the main culprit
- however, a temporary FIR bypass showed the clicks were still present, only somewhat quieter / less aggressive
- this indicates the pre-demod FIR likely amplifies or sharpens an upstream issue, but is not the sole origin
- a cleaner stateful decimating FIR implementation also failed to eliminate the recurring hot spot, further weakening the idea that the old FIR->decimate handoff alone caused the bug

---

## Recommended next steps

1. Run with reduced logging only and keep heavy dump features OFF unless explicitly needed.
2. Continue investigating the extractor path and its immediate surroundings (`extractForStreaming`, signal parameter source, offset/BW stability, overlap/trim behavior).
3. Treat FIR/decimation as a possible amplifier/focuser of the issue, but not the only suspect.
4. When testing fixes, prefer low-overhead, theory-driven experiments over broad logging/dump spam.
5. Only re-enable audio dump windows selectively and briefly.

### Debug TODO / operational reminders

- The current telemetry collector is **not** using a true ring buffer for metric/event history.
- Internally it keeps append-only history slices (`metricsHistory`, `events`) and periodically trims them by copying tail slices.
- Under heavy per-block telemetry this can add enough mutex/copy overhead to make the live stream start stuttering after a short run.
- Therefore: keep telemetry sampling conservative during live reproduction runs; do **not** leave full heavy telemetry enabled longer than needed.
- Follow-up engineering task: replace or redesign telemetry history storage to use a proper low-overhead ring-buffer style structure (or equivalent bounded lock-light design) if live telemetry is to remain a standard debugging tool.

---

## 2026-03-25 update — extractor-focused live telemetry findings

### Where the investigation moved

The investigation was deliberately refocused away from browser/feed/demod-only suspicions and toward:
- shared upstream IQ cadence / block boundaries
- extractor input/output continuity
- raw vs trimmed extractor-head behaviour

This was driven by two observations:
1. all signals still click
2. the newly added live telemetry made it possible to inspect the shared path while the system was running

### Telemetry infrastructure / config notes

Two config files matter for debug telemetry defaults:
- `config.yaml`
- `config.autosave.yaml`

The autosave file can overwrite intended telemetry defaults after restart, so both must be updated together.

Current conservative live-debug defaults that worked better:
- `heavy_enabled: false`
- `heavy_sample_every: 12`
- `metric_sample_every: 8`
- `metric_history_max: 6000`
- `event_history_max: 1500`

Important operational lesson:
- runtime `POST /api/debug/telemetry/config` changes only affect the current `sdrd` process
- after restart, the process reloads config defaults again
- if autosave still contains older values (for example `heavy_enabled: true` or very large history limits), the debug run can accidentally become self-distorting again

### Telemetry endpoints

The live debug work used these HTTP endpoints on the `sdrd` web server (typically `http://127.0.0.1:8080`):

#### `GET /api/debug/telemetry/config`
Returns the current effective telemetry configuration.
Useful for verifying:
- whether heavy telemetry is enabled
- history sizes
- persistence settings
- sample rates actually active in the running process

Typical fields:
- `enabled`
- `heavy_enabled`
- `heavy_sample_every`
- `metric_sample_every`
- `metric_history_max`
- `event_history_max`
- `retention_seconds`
- `persist_enabled`
- `persist_dir`

#### `POST /api/debug/telemetry/config`
Applies runtime telemetry config changes to the current process.
Used during investigation to temporarily reduce telemetry load without editing files.

Example body used during investigation:
```json
{
  "heavy_enabled": true,
  "heavy_sample_every": 12,
  "metric_sample_every": 8
}
```

#### `GET /api/debug/telemetry/live`
Returns the current live metric snapshot (gauges/counters/distributions).
Useful for:
- quick sanity checks
- verifying that a metric family exists
- confirming whether a new metric name is actually being emitted

#### `GET /api/debug/telemetry/history?prefix=<prefix>&limit=<n>`
Returns stored metric history entries filtered by metric-name prefix.
This is the main endpoint for time-series debugging during live runs.

Useful examples:
- `prefix=stage.`
- `prefix=source.`
- `prefix=iq.boundary.all`
- `prefix=iq.extract.input`
- `prefix=iq.extract.output`
- `prefix=iq.extract.raw.`
- `prefix=iq.extract.trimmed.`
- `prefix=iq.pre_demod`
- `prefix=audio.demod`

#### `GET /api/debug/telemetry/events?limit=<n>`
Returns recent structured telemetry events.
Used heavily once compact per-block event probes were added, because events were often easier to inspect reliably than sparsely sampled distribution histories.

This ended up being especially useful for:
- raw extractor head probes
- trimmed extractor head probes
- boundary snapshots

### Important telemetry families added/used

#### Shared-path / global boundary metrics
- `iq.boundary.all.head_mean_mag`
- `iq.boundary.all.prev_tail_mean_mag`
- `iq.boundary.all.delta_mag`
- `iq.boundary.all.delta_phase`
- `iq.boundary.all.discontinuity_score`

Purpose:
- detect whether the shared `allIQ` block boundary was already obviously broken before signal-specific extraction

#### Extractor input/output metrics
- `iq.extract.input.length`
- `iq.extract.input.overlap_length`
- `iq.extract.input.head_mean_mag`
- `iq.extract.input.prev_tail_mean_mag`
- `iq.extract.input.discontinuity_score`
- `iq.extract.output.length`
- `iq.extract.output.head_mean_mag`
- `iq.extract.output.head_min_mag`
- `iq.extract.output.head_max_step`
- `iq.extract.output.head_p95_step`
- `iq.extract.output.head_tail_ratio`
- `iq.extract.output.head_low_magnitude_count`
- `iq.extract.output.boundary.delta_mag`
- `iq.extract.output.boundary.delta_phase`
- `iq.extract.output.boundary.d2`
- `iq.extract.output.boundary.discontinuity_score`

Purpose:
- isolate whether the final per-signal extractor output itself was discontinuous across blocks

#### Raw vs trimmed extractor-head telemetry
- `iq.extract.raw.length`
- `iq.extract.raw.head_mag`
- `iq.extract.raw.tail_mag`
- `iq.extract.raw.head_zero_count`
- `iq.extract.raw.first_nonzero_index`
- `iq.extract.raw.head_max_step`
- `iq.extract.trim.trim_samples`
- `iq.extract.trimmed.head_mag`
- `iq.extract.trimmed.tail_mag`
- `iq.extract.trimmed.head_zero_count`
- `iq.extract.trimmed.first_nonzero_index`
- `iq.extract.trimmed.head_max_step`
- event `extract_raw_head_probe`
- event `extract_trimmed_head_probe`

Purpose:
- answer the key question: is the corruption already present in the raw extractor output head, or created by trimming/overlap logic afterward?

#### Pre-demod / audio-stage metrics
- `iq.pre_demod.head_mean_mag`
- `iq.pre_demod.head_min_mag`
- `iq.pre_demod.head_max_step`
- `iq.pre_demod.head_p95_step`
- `iq.pre_demod.head_low_magnitude_count`
- `audio.demod.head_mean_abs`
- `audio.demod.tail_mean_abs`
- `audio.demod.edge_delta_abs`
- existing `audio.demod_boundary.*`

Purpose:
- verify where artifacts become visible/audible downstream

### What the 2026-03-25 telemetry actually showed

#### 1. Feed / enqueue remained relatively uninteresting
`stage.feed_enqueue.duration_ms` was usually effectively zero.

Representative values during live runs:
- mostly `0`
- occasional small spikes such as `0.5 ms` and `5.8 ms`

Interpretation:
- feed enqueue is not the main source of clicks

#### 2. Extract-stream time was usually modest
`stage.extract_stream.duration_ms` was usually small and stable compared with the main loop.

Representative values:
- often `1–5 ms`
- occasional spikes such as `10.7 ms` and `18.9 ms`

Interpretation:
- extraction is not free, but runtime cost alone does not explain the clicks

#### 3. Shared capture / source cadence still fluctuated heavily
Representative live values:
- `dsp.frame.duration_ms`: often around `90–100 ms`, but also `110–150 ms`, with one observed spike around `212.6 ms`
- `source.read.duration_ms`: roughly `80–90 ms` often, but also about `60 ms`, `47 ms`, `19 ms`, and even `0.677 ms`
- `source.buffer_samples`: ranged from very small to very large bursts, including examples like `512`, `4608`, `94720`, `179200`, `304544`
- a `source_reset` event was seen and `source.resets=1`

Interpretation:
- shared upstream cadence is clearly unstable enough to remain suspicious
- but this alone did not localize the final click mechanism

#### 4. Pre-demod stage showed repeated hard phase anomalies even when energy looked healthy
Representative live values for normal non-vanishing signals:
- `iq.pre_demod.head_mean_mag` around `0.25–0.31`
- `iq.pre_demod.head_low_magnitude_count = 0`
- `iq.pre_demod.head_max_step` repeatedly high, including roughly:
  - `1.5`
  - `2.0`
  - `2.4`
  - `2.8`
  - `3.08`

Interpretation:
- not primarily an amplitude collapse
- rather a strong phase/continuity defect reaching the pre-demod stage

#### 5. Audio stage still showed real block-edge artifacts
Representative values:
- `audio.demod.edge_delta_abs` repeatedly around `0.4–0.8`
- outliers up to roughly `1.21` and `1.26`
- `audio.demod_boundary.count` continued to fire repeatedly

Interpretation:
- demod is where the problem becomes audible, but the root cause still appeared to be earlier/shared

### Key extractor findings from the new telemetry

#### A. Per-signal extractor output boundary is genuinely broken
For a representative strong signal (`signal_id=2`), `iq.extract.output.boundary.delta_phase` repeatedly showed very large jumps such as:
- `2.60`
- `3.06`
- `2.14`
- `2.71`
- `3.09`
- `2.92`
- `2.63`
- `2.78`

Also observed for `iq.extract.output.boundary.discontinuity_score`:
- `2.86`
- `3.08`
- `2.92`
- `2.52`
- `2.40`
- `2.85`

Later runs using `d2` made the discontinuity even easier to see. Representative `iq.extract.output.boundary.d2` values for the same strong signal included:
- `0.347`
- `0.303`
- `0.362`
- `0.359`
- `0.382`
- `0.344`
- `0.337`
- `0.206`

At the same time, `iq.extract.output.boundary.delta_mag` was often comparatively small (examples around `0.0003–0.0038`).

Interpretation:
- the main boundary defect is not primarily amplitude mismatch
- it is much more consistent with complex/phase discontinuity across output blocks

#### B. The raw extractor head is systematically bad on all signals
The new `extract_raw_head_probe` events were the strongest finding of the day.

Representative repeated pattern for strong signals (`signal_id=1` and `signal_id=2`):
- `first_nonzero_index = 1`
- `zero_count = 1`
- first magnitude sample exactly `0`
- then a short ramp: e.g. for `signal_id=2`
  - `0`
  - `0.000388`
  - `0.002316`
  - `0.004152`
  - `0.019126`
  - `0.011418`
  - `0.124034`
  - `0.257569`
  - `0.317579`
- `head_max_step` often near π, e.g.:
  - `3.141592653589793`
  - `3.088773696463606`
  - `3.0106854446936318`
  - `2.9794833659932527`

The same qualitative pattern appeared for weaker signals too:
- raw head starts at `0`
- a brief near-zero ramp follows
- only after several samples does the magnitude look like a normal extracted band

Interpretation:
- the raw extractor output head is already damaged / settling / invalid before trimming
- this strongly supports an upstream/shared-start-condition problem rather than a trim-created artifact

#### C. The trimmed extractor head usually looks sane
Representative repeated pattern for the same signals after `trim_samples = 64`:
- `first_nonzero_index = 0`
- `zero_count = 0`
- magnitudes look immediately plausible and stable
- `head_max_step` is dramatically lower than raw, often around `0.15–0.9` for strong channels

Example trimmed head magnitudes for `signal_id=2`:
- `0.299350`
- `0.300954`
- `0.298032`
- `0.298738`
- `0.312258`
- `0.296932`
- `0.239010`
- `0.266881`
- `0.313193`

Example trimmed head magnitudes for `signal_id=1`:
- `0.277400`
- `0.275994`
- `0.273718`
- `0.272846`
- `0.277842`
- `0.278398`
- `0.268829`
- `0.273790`
- `0.279031`

Interpretation:
- trimming is removing a genuinely bad raw head region
- trimming is therefore **not** the main origin of the problem
- it acts more like cleanup of an already bad upstream/raw start region

### Input-vs-raw-vs-trimmed extractor result (important refinement)

A later, more targeted telemetry pass added a direct probe on the signal-specific extractor input head (`extract_input_head_probe`) and compared it against the raw and trimmed extractor output heads.

This materially refined the earlier conclusion.

#### Input-head result
Representative values from `iq.extract.input_head.*`:
- `iq.extract.input_head.zero_count = 0`
- `iq.extract.input_head.first_nonzero_index = 0`

Interpretation:
- the signal-specific input head going into the GPU extractor is **not** starting with a zero sample
- the head is not arriving already dead/null from the immediate input probe point

#### Raw-head result
Representative values from `iq.extract.raw.*`:
- `iq.extract.raw.head_mag = 0`
- `iq.extract.raw.head_zero_count = 1`
- `iq.extract.raw.head_max_step` frequently around `2.4–3.14`

These values repeated for strong channels such as `signal_id=2`, and similarly across other signals.

Interpretation:
- the first raw output sample is repeatedly exactly zero
- therefore the visibly bad raw head is being created **after** the probed input head and **before/during raw extractor output generation**

#### Trimmed-head result
Representative values from `iq.extract.trimmed.*`:
- `iq.extract.trimmed.head_zero_count = 0`
- `iq.extract.trimmed.head_mag` often looked healthy immediately after trimming, for example:
  - signal 1: about `0.275–0.300`
  - signal 2: about `0.311`
- `iq.extract.trimmed.head_max_step` was much lower than raw for strong channels, often around:
  - `0.11`
  - `0.14`
  - `0.19`
  - `0.30`
  - `0.75`

Interpretation:
- trimming cleans up the visibly bad raw head region
- trimming still does **not** explain the deeper output-boundary continuity issue

### Refined strongest current conclusion after the 2026-03-25 telemetry pass

The strongest current reading is now:

> The click root cause is very likely **not** that the signal-specific extractor input already starts dead/null. Instead, the bad raw head appears to be introduced **inside the GPU extractor path or at its immediate start/output semantics**, before final trimming.

More specifically:
- signal-specific extractor input head looks non-zero and sane at the probe point
- all signals still show a systematically bad raw extractor head
- the trimmed head usually looks healthier
- yet the final extractor output still exhibits significant complex boundary discontinuity from block to block

This points away from a simple "shared global input head is already zero" theory and toward one of these narrower causes:
1. GPU extractor start semantics / kernel warmup / first-output handling
2. phase-start or alignment handling at extractor block start
3. output assembly semantics inside the raw GPU extractor path

### What should not be forgotten from this stage

- The overlap-prepend bug was real and worth fixing, but was not sufficient.
- The fixed read-size path (`SDR_FORCE_FIXED_STREAM_READ_SAMPLES=389120`) remains useful and likely worth promoting later, but it is not the root-cause fix.
- The telemetry system itself can perturb runs if overused; conservative sampling matters.
- `config.autosave.yaml` must be kept in sync with `config.yaml` or telemetry defaults can silently revert after restart.
- The most promising root-cause area is now the shared upstream/extractor-start boundary path, not downstream playback.

---

## Meta note

This investigation already disproved several plausible explanations. That is progress.

The most important thing not to forget is:
- the overlap prepend bug was real, but not sufficient
- the click is already present in demod audio
- whole-process CPU saturation is not the main explanation
- excessive debug instrumentation can itself create misleading secondary problems
- the 2026-03-25 extractor telemetry strongly suggests the remaining root cause is upstream of the final trim stage
