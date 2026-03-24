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
Instead, the likely correct direction is:
- replace the current “stateful FIR, then separate decimation” handoff with a **stateful decimating FIR / polyphase decimator**
- preserve phase and delay state explicitly
- ensure the first emitted decimated samples are already truly valid for demodulation

Important nuance:
- the currently suspicious FIR + decimation section is already running in **Go/CPU** (`processSnippet`), not in CUDA
- therefore the next correctness fix should be developed and validated in Go first

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

---

## Recommended next steps

1. Run with reduced logging only (`demod`, `gap`, `boundary`) unless discriminator logging is specifically needed again.
2. Keep heavy dump features OFF unless explicitly needed.
3. Treat the beginning of the `dec` block as the highest-priority investigation zone.
4. Continue analysing whether the observed issue is:
   - an expected FIR/decimation settling region being handled incorrectly, or
   - evidence that corrupted IQ is already entering the pre-demod FIR
5. When testing fixes, prefer low-overhead, theory-driven experiments over broad logging/dump spam.
6. Only re-enable audio dump windows selectively and briefly.

---

## Meta note

This investigation already disproved several plausible explanations. That is progress.

The most important thing not to forget is:
- the overlap prepend bug was real, but not sufficient
- the click is already present in demod audio
- whole-process CPU saturation is not the main explanation
- excessive debug instrumentation can itself create misleading secondary problems
