# Known Issues

This file tracks durable open engineering issues that remain after the 2026-03-25 audio-click fix.

Primary source:
- `docs/open-issues-report-2026-03-25.json`

Status values used here:
- `open`
- `deferred`
- `info`

---

## High Priority

### OI-24 — WFM stereo never locks on live broadcast (pilot not detected)
- Status: `open`
- Severity: High
- Category: demod / stereo
- File: `internal/pipeline/refiner.go` (PLL), `internal/recorder/streamer.go`
- Summary: live WFM broadcast signals stay `stereo=searching` (pll pilot not
  detected) on all stations. Predates the Phase-R bandwidth work — it was already
  "searching" when R1 was effectively skipped (OI-23), so it is not caused by the
  bandwidth re-estimation. Needs its own investigation: verify the 19 kHz pilot is
  present in the extracted snippet at the snippet rate, the FM discriminator
  output, and the PLL lock thresholds.
- Note: extraction bandwidth (sig.BWHz) must be wide enough to pass the pilot;
  unstable bandwidth (OI-23) can make this worse.

### OI-23 — Occupied-bandwidth estimate unstable on the dense, strong FM band
- Status: `open`
- Severity: High
- Category: estimation-quality
- File: `internal/estimate/bandwidth.go`, `internal/pipeline/refiner.go`
- Summary: R1 occupied-bandwidth re-estimation is validated on the synthetic
  benchmark (isolated ~30 dB signals) but is unreliable on the real broadcast FM
  band: very strong (55–58 dB) carriers over a low (Welch) noise floor, with
  closely spaced neighbours, make the blob chase far skirts / bridge to adjacent
  stations (observed bandwidths swinging 47 k…504 k for WFM). Mitigations added
  (peak-relative dynamic-range bound, tightened sane-factor guard) reduce but do
  not eliminate it.
- Also: a coordinate bug was fixed here — refinement was indexing the detail
  spectrum (4096) with candidate bins from the surveillance spectrum (16384), so
  R1 was silently skipped live whenever detail_fft != fft_size. Now uses the
  surveillance spectrum.
- Recommended fix: add a dense/strong-FM scene to the synth benchmark (multiple
  strong WFM stations at realistic spacing and 50+ dB SNR), reproduce the
  instability, and harden the estimator there (peak-relative containment, smarter
  inter-station gap handling) instead of tuning constants on live signals.
- Source: live testing 2026-06-07.

### OI-20 — Refinement does not re-estimate bandwidth/SNR (copies coarse value)
- Status: `resolved` (Phase R / R1: bandwidth, center and SNR re-estimation landed)
- Severity: High
- Category: estimation-quality
- File: `internal/pipeline/refiner.go`, `internal/estimate/`
- Resolution: refinement now re-estimates occupied bandwidth + center + SNR via
  `estimate.RefineFromSpectrum` (power containment + peak-over-noise), gated by
  `refinement.occupied_bw_fraction`. Benchmark: refined median bw error ~24% vs
  geometric ~49%; refined SNR tracks configured SNR ~1:1.
- Summary: the refinement layer is documented (PLAN §4) to "stabilize center/bw/snr",
  but it copies the coarse detector bandwidth unchanged (`sig.BWHz = c.BandwidthHz`)
  and only attaches classification/PLL. The classifier's dominant feature (bandwidth)
  is therefore the noisy single-resolution geometric estimate, never refined.
- Recommended fix: implement occupied-bandwidth (power-containment) + SNR
  re-estimation per candidate from the local PSD/IQ snippet (Phase R, step R1).
- Source: `docs/detection-rework-plan-2026-06-06.md`

### OI-21 — Single-resolution detection is not universal (over/under detection)
- Status: `open`
- Severity: High
- Category: detection-architecture
- File: `internal/detector/detector.go`, `internal/cfar/`
- Summary: detection runs on one FFT resolution with threshold-crossing + heuristic
  edge expansion + a fixed `MergeGapHz`. One bin width cannot resolve both narrowband
  (CW) and wideband (WFM) signals, so it over-/under-detects unless tuned for one band
  (BC-FM). The Phase-2 multi-resolution surveillance scaffolding is not actually
  consumed by the detector.
- Recommended fix: multi-resolution detection consuming multiple levels + scale-aware
  merge + candidate fusion (Phase R, step R3).
- Source: `docs/detection-rework-plan-2026-06-06.md`

### OI-22 — No ground-truth benchmark for detection/estimation/classification
- Status: `open`
- Severity: High
- Category: test-coverage / measurability
- File: `internal/mock/`, (new) benchmark/eval target
- Summary: there is no measurement with known ground truth, so detection P/R,
  bandwidth/center error, and classification accuracy are unquantified. Any tuning is
  blind ("tinkering"). This blocks doing the rework correctly.
- Recommended fix: parametric synthetic scene generator (extend mock) + ground-truth
  benchmark harness as a tagged `go test` (Phase R, step R0). Also satisfies B-5 and
  the classifier-ml-plan Phase 0.
- Source: `docs/detection-rework-plan-2026-06-06.md`, `docs/architecture-review-2026-06-06.md`

### OI-02 — `lastDiscrimIQ` missing from `dspStateSnapshot`
- Status: `open`
- Severity: High
- Category: state-continuity
- File: `internal/recorder/streamer.go`
- Summary: FM discriminator bridging state is not preserved across `captureDSPState()` / `restoreDSPState()`, so recording segment splits can lose the final IQ sample and create a micro-click at the segment boundary.
- Recommended fix: add `lastDiscrimIQ` and `lastDiscrimIQSet` to `dspStateSnapshot`.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-02)

### OI-03 — CPU oracle path not yet usable as validation baseline
- Status: `open`
- Severity: High
- Category: architecture
- File: `cmd/sdrd/streaming_refactor.go`, `internal/demod/gpudemod/cpu_oracle.go`
- Summary: the CPU oracle exists, but the production comparison/integration path is not trusted yet. That means GPU-path regressions still cannot be checked automatically with confidence.
- Recommended fix: repair oracle integration and restore GPU-vs-CPU validation flow.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-03)

### OI-18 — planned C2-C validation gate never completed
- Status: `open`
- Severity: Info
- Category: architecture
- File: `docs/audio-click-debug-notes-2026-03-24.md`
- Summary: the final native streaming path works in practice, but the planned formal GPU-vs-oracle validation gate was never completed.
- Recommended fix: complete this together with OI-03.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-18)

---

## Medium Priority

### OI-14 — no regression test for `allIQ` immutability through spectrum/detection pipeline
- Status: `open`
- Severity: Low
- Category: test-coverage
- File: `cmd/sdrd/pipeline_runtime.go`
- Summary: the `IQBalance` aliasing bug showed that shared-buffer mutation can slip in undetected. There is still no test asserting that `allIQ` remains unchanged after capture/detection-side processing.
- Recommended fix: add an integration test that compares `allIQ` before and after the relevant pipeline stage.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-14)

### OI-15 — very low test coverage for `processSnippet` audio pipeline
- Status: `open`
- Severity: Low
- Category: test-coverage
- File: `internal/recorder/streamer.go`
- Summary: the main live audio pipeline still lacks focused tests for boundary continuity, WFM mono/stereo behavior, resampling, and demod-path regressions.
- Recommended fix: add synthetic fixtures and continuity-oriented tests around repeated `processSnippet` calls.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-15)

### OI-07 — taps are recalculated every frame
- Status: `open`
- Severity: Medium
- Category: correctness
- File: `internal/demod/gpudemod/stream_state.go`
- Summary: FIR/polyphase taps are recomputed every frame even when parameters do not change, which is unnecessary work and makes it easier for host/GPU tap state to drift apart.
- Recommended fix: only rebuild taps when tap-relevant inputs actually change.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-07)

### OI-17 — bandwidth changes can change Go-side taps without GPU tap re-upload
- Status: `open`
- Severity: Low-Medium
- Category: correctness
- File: `internal/demod/gpudemod/streaming_gpu_native_prepare.go`, `internal/demod/gpudemod/stream_state.go`
- Summary: after the config-hash fix, a bandwidth change may rebuild taps on the Go side while the GPU still keeps older uploaded taps unless a reset happens.
- Recommended fix: add a separate tap-change detection/re-upload path without forcing full extractor reset.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-17)

### OI-09 — streaming feature flags are compile-time constants
- Status: `open`
- Severity: Medium
- Category: architecture
- File: `cmd/sdrd/streaming_refactor.go`, `internal/demod/gpudemod/streaming_gpu_modes.go`
- Summary: switching between production/oracle/native-host modes still requires code changes and rebuilds, which makes field debugging and A/B validation harder than necessary.
- Recommended fix: expose these as config or environment-driven switches.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-09)

### OI-05 — feed channel is shallow and can drop frames under pressure
- Status: `open`
- Severity: Medium
- Category: reliability
- File: `internal/recorder/streamer.go`
- Summary: `feedCh` has a buffer of only 2. Under heavier processing or debug load, dropped feed messages can create audible gaps.
- Recommended fix: increase channel depth or redesign backpressure behavior.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-05)

### OI-06 — legacy overlap/trim extractor path is now mostly legacy baggage
- Status: `deferred`
- Severity: Medium
- Category: dead-code
- File: `cmd/sdrd/helpers.go`
- Summary: the old overlap/trim path is now mainly fallback/legacy code and adds complexity plus old instrumentation noise.
- Recommended fix: isolate, simplify, or remove it once the production path and fallback strategy are formally settled.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-06)

### OI-04 — telemetry history storage still uses append+copy trim
- Status: `deferred`
- Severity: Medium
- Category: telemetry
- File: `internal/telemetry/telemetry.go`
- Summary: heavy telemetry can still create avoidable allocation/copy pressure because history trimming is O(n) and happens under lock.
- Recommended fix: replace with a ring-buffer design.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-04)

---

### OI-19 — Waterfall shows only the newest (top) row, no history scroll
- Status: `resolved` (fix `f561688`, verified in live UI 2026-06-06: waterfall scrolls)
- Severity: Medium
- Category: ui-rendering
- File: `web/app.js` (`renderWaterfall`, ~L1702-1729), `web/style.css` (`.viz-stack`)
- Summary: the waterfall only displays the most recent row at the top; rows below
  stay blank/black, so there is no scrolling history. Each frame is expected to
  (1) shift existing canvas content down by one row, then (2) draw the new row at
  `y=0`. The downward shift does not persist, so only the `putImageData` top row
  survives.
- Suspected root cause (ranked):
  1. Self-canvas scroll via `ctx.drawImage(waterfallCanvas, 0,0,w,h-1, 0,1,w,h-1)`
     under `globalCompositeOperation='copy'`. Drawing a canvas onto itself with
     `copy` is browser-dependent and can yield a blank shifted region, leaving
     only the freshly written top row.
  2. (Less likely) waterfall canvas internal `height` collapsing to ~1px — but
     `.viz-stack` grid (`minmax(150px,0.72fr)`) gives the card a real height, so
     this should not happen; still worth confirming `h > 1` at runtime.
  3. `waterfallRowImageData` width mismatch forcing a per-frame reset
     (`waterfallRowImageData.width !== w`), which would also wipe accumulation.
- Diagnostics:
  - log `w`/`h` inside `renderWaterfall` and confirm `h > 1`
  - temporarily replace the `copy` self-blit with a double-buffer (offscreen
    canvas) or a plain `source-over` `ctx.drawImage(canvas, 0, 1)` shift and see
    if history accumulates
  - check `git log -- web/app.js` for a recent change to the scroll/composite path
    (possible regression)
- Recommended fix: double-buffer the scroll via an offscreen canvas (draw shifted
  history + new row there, then blit back), or drop `globalCompositeOperation='copy'`
  for the self-shift. The detail spectrogram path (`detailRowImageData`,
  `putImageData` at ~L1894) may share the same pattern — verify it too.
- Source: reported 2026-06-06 (live UI observation).

---

## Lower Priority / Nice-to-Have

### OI-01 — `DCBlocker.Apply(allIQ)` still mutates extraction input in-place
- Status: `deferred`
- Severity: High
- Category: data-integrity
- File: `cmd/sdrd/pipeline_runtime.go`
- Summary: unlike the old `IQBalance` bug this does not create a boundary artifact, but it does mean live extraction and recorded/replayed data are not semantically identical.
- Recommended fix: clarify the contract or move to immutable/copy-based handling.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-01)

### OI-08 — WFM audio LPF could reject pilot more strongly
- Status: `deferred`
- Severity: Medium
- Category: audio-quality
- File: `internal/recorder/streamer.go`
- Summary: the current 15 kHz LPF is good enough functionally, but a steeper filter could further improve pilot suppression.
- Recommended fix: more taps or a dedicated pilot notch.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-08)

### OI-10 — `demod.wav` debug dumps can clip and mislead analysis
- Status: `deferred`
- Severity: Medium
- Category: correctness
- File: `internal/recorder/streamer.go`, `internal/recorder/wavwriter.go`
- Summary: raw discriminator output can exceed the WAV writer's `[-1,+1]` clip range, so debug dumps can show artifacts that are not part of the real downstream audio path.
- Recommended fix: scale by `1/pi` before dumping or use float WAV output.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-10)

### OI-11 — browser AudioContext resync still causes audible micro-gaps
- Status: `deferred`
- Severity: Low
- Category: reliability
- File: `web/app.js`
- Summary: underrun recovery is softened with a fade-in, but repeated resyncs still create audible stutter on the browser side.
- Recommended fix: prefer the AudioWorklet/ring-player path wherever possible.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-11)

### OI-12 — tiny per-frame tail copy for boundary telemetry
- Status: `info`
- Severity: Low
- Category: performance
- File: `cmd/sdrd/pipeline_runtime.go`
- Summary: the last-32-sample copy is trivial and not urgent, but it is one more small allocation in a path that already has several.
- Recommended fix: none needed unless a broader allocation cleanup happens.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-12)

### OI-13 — temporary patch artifacts should not live in the repo long-term
- Status: `deferred`
- Severity: Low
- Category: dead-code
- File: `patches/*`
- Summary: reviewer/debug patch artifacts were useful during the investigation, but they should either be removed or archived under docs rather than kept as loose patch files.
- Recommended fix: delete or archive them once no longer needed.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-13)

### OI-16 — `config.autosave.yaml` can re-enable unwanted debug telemetry after restart
- Status: `deferred`
- Severity: Low
- Category: config
- File: `config.autosave.yaml`
- Summary: autosave can silently restore debug-heavy telemetry settings after restart and distort future runs.
- Recommended fix: stop persisting debug telemetry knobs to autosave or explicitly ignore them.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-16)

---

## Suggested next execution order

1. Fix OI-02 (`lastDiscrimIQ` snapshot/restore)
2. Repair OI-03 and close OI-18 (oracle + formal validation path)
3. Add OI-14 and OI-15 regression tests
4. Consolidate OI-07 and OI-17 (tap rebuild / tap upload logic)
5. Expose OI-09 feature flags via config or env
6. Revisit OI-05 / OI-06 / OI-04 when doing reliability/cleanup work
