# Known Issues

This file tracks durable open engineering issues that remain after the 2026-03-25 audio-click fix.

> Open items are mirrored as **GitHub Issues** (linked per item below). This file is
> the curated narrative; the GitHub issues are the executable units of work — see
> [`agent-workflow.md`](agent-workflow.md).

Primary source:
- `docs/open-issues-report-2026-03-25.json`

Status values used here:
- `open`
- `deferred`
- `info`

---

## High Priority

### OI-26 — RDS decode runs its DSP chain on the CPU (root of OI-25)
- Status: `resolved` (RDS shift/filter/decimate moved to the GPU batch runner;
  CPU ~14 -> ~3.5 cores, RSS ~3.6 GB -> ~1.2 GB, and WFM stereo now locks — OI-24)
- Severity: High
- Category: performance / gpu
- File: `cmd/sdrd/pipeline_runtime.go` (`updateRDS`)
- Summary: `updateRDS` extracts the RDS baseband per WFM signal by doing
  `dsp.FreqShift` + `dsp.ApplyFIR` (51 then 101 taps) + `dsp.Decimate` over ~4 s
  of full-rate IQ (~16 M samples) on the CPU, in a goroutine per signal. A CPU
  profile attributes ~52% of total CPU (150 s / 290 s, ApplyFIR) to
  `updateRDS.func1`. With many BC-FM stations this saturates ~8 cores and is the
  primary driver of OI-25, and likely starves the DSP loop (OI-24 no stereo lock).
- Recommended fix (per the GPU-first rule, AGENTS §7): route the RDS
  shift/filter/decimate through the `gpudemod.BatchRunner` like the audio
  extraction path, with the CPU chain kept only as fallback when the GPU is
  unavailable.
- Source: pprof CPU profile, 2026-06-07.

### OI-25 — Very high CPU (~13-16 cores) and memory (~3-4 GB) at runtime
- Status: `resolved` — two root causes, both per-signal and scaling with signal count:
  1. RDS DSP ran on the CPU (OI-26) → moved to GPU: CPU ~14 -> ~3.5 cores.
  2. RDS sliced ~128 MB of full-rate IQ per station per decode and keyed state by
     the jittering center frequency, so the buffer re-allocated every frame
     (Ring.Slice = ~85% of all allocations) → GC stutter that grew with the number
     of signals. Fixed by a stable tracker-ID key + reused buffer (commit 516f0db):
     CPU -> ~0.1 cores, RSS ~3.6 -> ~1 GB, allocation profile now only necessary work.
- Severity: High
- Category: performance
- File: TBD (profile first — recorder/decode, extraction, surveillance levels, GPU demod)
- Summary: live runtime uses ~13-16 CPU cores and 3-4 GB RSS (soft mem limit is
  1 GB). Measured pre-existing: toggling the Phase-R features makes no difference
  (R1 on = 12.9 cores, R1 off = 15.7 cores), so it is NOT caused by R1/Welch.
  Likely sinks: recorder `auto_demod`/`auto_decode` per signal on the dense FM
  band, per-signal streaming extraction, multi-resolution surveillance, or a
  GPU-demod fallback. Suspected to also starve the DSP loop and contribute to
  OI-24 (no stereo lock).
- Recommended fix: profile (net/http/pprof CPU profile, or bisect by toggling
  recorder/auto-decode and surveillance levels) to find the hot path, then fix.
- Source: live testing 2026-06-07.

### OI-24 — WFM stereo lock unreliable (PLL integration window too short)
- Status: `resolved` — confirmed live (DLF 100.6 + Radio 7 102.5: stereo=locked,
  RDS PS "Dlf"/"RADIO 7"). Three independent root causes, all required:
  1. **Long-window pilot (the documented OI-24 blocker).** The stereo pilot test no
     longer runs on the ~1 ms per-frame snippet. `classifier.StereoPilotPresent`
     runs the 19 kHz pilot test on the SAME multi-second ring slice (~250 kHz
     baseband) that `updateRDS` already pulls for RDS, where the pilot is 43-47 dB
     over the floor (live ratio ~1000-1975 vs the 4.0 threshold). The short-snippet
     PLL is kept only for the exact-frequency estimate; its `.Stereo` is ignored.
  2. **Detected signals carried ID=0.** `detector.matchSignals` matched raw
     per-frame detections to tracked events but never wrote the stable event ID
     back into the returned `[]Signal`. So candidates -> refinement signals were all
     ID=0, and `updateRDS` early-returned on `key == 0` — the stereo/RDS long-window
     path NEVER ran live. Fixed: write `signals[i].ID = id` on both match and
     new-event branches.
  3. **`maintenance()` deleted every rdsState every frame.** `rt.rdsMap` is keyed by
     tracker ID (OI-25) but `maintenance()` still pruned it with a stale
     frequency-quantized key (`keyHz/25000`), which never matched the ID keys. So
     the async pilot/RDS decode wrote to an orphaned state, the next frame recreated
     a fresh one (stereo=false, lastDecode=0 → relaunch every frame), so stereo
     never stuck and the RDS decoder state never accumulated. Fixed to prune by
     `s.ID`. This also fixed RDS (PS names now decode) and the per-frame GPU RDS
     re-extraction churn.
- Validated offline: `TestStereoPilotLongWindow` locks 100.6 + 102.5 MHz and rejects
  an empty slot (90.0 MHz); `pilotLockRatio` 4.0 separates them cleanly.
- Update 2026-06-07 (offline replay oracle): the chain was diagnosed end to end.
  1. CPU starvation removed (OI-26) — necessary but not sufficient.
  2. Center jitter reduced 3x (Slice A, alpha-beta + peak anchor) — necessary but
     not sufficient.
  3. **Real bug found & fixed (e6e7798):** the refinement upgrades WFM -> WFM_STEREO
     *before* calling EstimateExactFrequency, whose switch only handled ClassWFM, so
     every stereo station fell through to default and the 19 kHz pilot was never
     looked for. Added ClassWFMStereo to the pilot path. Stereo then locks.
  4. **Remaining root cause (the real blocker):** the PLL snippet is only ~512
     samples (~1 ms). Measured on the replay capture (TestRealTargetPilot proves the
     pilot is 43 dB present over 1.5 s), the live per-frame pilot ratio is ~2 for a
     real station vs noise spikes up to ~5 — i.e. at 512 samples the pilot is
     indistinguishable from noise and NO threshold separates them. The lock and the
     false-locks on weak/noise detections both stem from this.
- Fix (next): give the stereo PLL a longer integration window (e.g. a ~100 ms+ ring
  slice like the RDS path) so the 19 kHz pilot integrates reliably; then recalibrate
  pilotLockRatio against the replay. Interim: pilotLockRatio set high (4.0) to avoid
  false-lock spam (so real stations under-lock for now). A pilot-search window
  (±1 kHz, robust band-median threshold) and a sticky-lock hold (rt.stereoHold) are
  already in place for when the window is fixed.
- Tools: sdrd -replay (reproducible oracle), SDRD_PLL_DEBUG / SDRD_PILOT_RATIO,
  TestRealTargetPilot, TestRealJitter.

### OI-27 — Wandering phantom detection near 99.9 MHz
- Status: `open` — GitHub: [#3](https://github.com/svabi79/argus-sdr/issues/3)
- Severity: Medium
- Category: detection
- Summary: a detection near ~99.9 MHz wanders in frequency and (with the loose
  interim pilot threshold) flickers a stereo lock — reported as clearly incorrect.
  Likely a weak/spurious detection (image/intermod/band-edge) tracked as one ID.
  Investigate reproducibly on the replay capture.
- Source: live observation 2026-06-07.
- Update 2026-06-07: Two findings from live work.
  1. CPU starvation was a major cause — once RDS moved to the GPU (OI-26, CPU
     ~14 -> ~1-3.5 cores) stereo started locking (e.g. 102.5 MHz, RDS "RADIO").
  2. Locks are still flaky and correlate with detection quality, not signal
     strength: a *narrowly* detected station locks; the same/stronger station
     when *widely* detected (250-540 kHz, from the dense band + skirts) does not.
     The wide detection skews the carrier center and pulls adjacent-channel
     energy into the extraction, detuning the 19 kHz pilot PLL and corrupting the
     RDS baseband ("the problem is before the decoder"). The detected bandwidth
     for one station also jitters frame to frame (33k/73k/106k/138k), so the lock
     comes and goes.
- Tried and reverted: live CFAR retuning (scale/guard) and a peak-bounded center
  estimate. These only moved the problem around (changing CFAR changed which
  stations detect narrow vs wide) and one coincided with a lock regression — i.e.
  live tuning is the wrong tool here.
- Recommended fix: this is detection-stability work, not live tuning. Build a
  realistic dense/strong-FM benchmark scene (OI-23 / Phase R R3), reproduce the
  center/bandwidth jitter offline, and give WFM a stable carrier center (peak-
  locked) + fixed channel bandwidth there, validated on the benchmark before
  live. Also fixes R1 (OI-23).
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
- Status: `open` — GitHub: [#4](https://github.com/svabi79/argus-sdr/issues/4)
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
- Status: `open` — GitHub: [#5](https://github.com/svabi79/argus-sdr/issues/5)
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
- Status: `open` — GitHub: [#2](https://github.com/svabi79/argus-sdr/issues/2)
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
- Status: `open` — GitHub: [#7](https://github.com/svabi79/argus-sdr/issues/7)
- Severity: High
- Category: state-continuity
- File: `internal/recorder/streamer.go`
- Summary: FM discriminator bridging state is not preserved across `captureDSPState()` / `restoreDSPState()`, so recording segment splits can lose the final IQ sample and create a micro-click at the segment boundary.
- Recommended fix: add `lastDiscrimIQ` and `lastDiscrimIQSet` to `dspStateSnapshot`.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-02)

### OI-03 — CPU oracle path not yet usable as validation baseline
- Status: `open` — GitHub: [#8](https://github.com/svabi79/argus-sdr/issues/8)
- Severity: High
- Category: architecture
- File: `cmd/sdrd/streaming_refactor.go`, `internal/demod/gpudemod/cpu_oracle.go`
- Summary: the CPU oracle exists, but the production comparison/integration path is not trusted yet. That means GPU-path regressions still cannot be checked automatically with confidence.
- Recommended fix: repair oracle integration and restore GPU-vs-CPU validation flow.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-03)

### OI-18 — planned C2-C validation gate never completed
- Status: `open` — GitHub: [#9](https://github.com/svabi79/argus-sdr/issues/9)
- Severity: Info
- Category: architecture
- File: `docs/audio-click-debug-notes-2026-03-24.md`
- Summary: the final native streaming path works in practice, but the planned formal GPU-vs-oracle validation gate was never completed.
- Recommended fix: complete this together with OI-03.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-18)

---

## Medium Priority

### OI-14 — no regression test for `allIQ` immutability through spectrum/detection pipeline
- Status: `open` — GitHub: [#10](https://github.com/svabi79/argus-sdr/issues/10)
- Severity: Low
- Category: test-coverage
- File: `cmd/sdrd/pipeline_runtime.go`
- Summary: the `IQBalance` aliasing bug showed that shared-buffer mutation can slip in undetected. There is still no test asserting that `allIQ` remains unchanged after capture/detection-side processing.
- Recommended fix: add an integration test that compares `allIQ` before and after the relevant pipeline stage.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-14)

### OI-15 — very low test coverage for `processSnippet` audio pipeline
- Status: `open` — GitHub: [#11](https://github.com/svabi79/argus-sdr/issues/11)
- Severity: Low
- Category: test-coverage
- File: `internal/recorder/streamer.go`
- Summary: the main live audio pipeline still lacks focused tests for boundary continuity, WFM mono/stereo behavior, resampling, and demod-path regressions.
- Recommended fix: add synthetic fixtures and continuity-oriented tests around repeated `processSnippet` calls.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-15)

### OI-07 — taps are recalculated every frame
- Status: `open` — GitHub: [#12](https://github.com/svabi79/argus-sdr/issues/12)
- Severity: Medium
- Category: correctness
- File: `internal/demod/gpudemod/stream_state.go`
- Summary: FIR/polyphase taps are recomputed every frame even when parameters do not change, which is unnecessary work and makes it easier for host/GPU tap state to drift apart.
- Recommended fix: only rebuild taps when tap-relevant inputs actually change.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-07)

### OI-17 — bandwidth changes can change Go-side taps without GPU tap re-upload
- Status: `open` — GitHub: [#13](https://github.com/svabi79/argus-sdr/issues/13)
- Severity: Low-Medium
- Category: correctness
- File: `internal/demod/gpudemod/streaming_gpu_native_prepare.go`, `internal/demod/gpudemod/stream_state.go`
- Summary: after the config-hash fix, a bandwidth change may rebuild taps on the Go side while the GPU still keeps older uploaded taps unless a reset happens.
- Recommended fix: add a separate tap-change detection/re-upload path without forcing full extractor reset.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-17)

### OI-09 — streaming feature flags are compile-time constants
- Status: `open` — GitHub: [#14](https://github.com/svabi79/argus-sdr/issues/14)
- Severity: Medium
- Category: architecture
- File: `cmd/sdrd/streaming_refactor.go`, `internal/demod/gpudemod/streaming_gpu_modes.go`
- Summary: switching between production/oracle/native-host modes still requires code changes and rebuilds, which makes field debugging and A/B validation harder than necessary.
- Recommended fix: expose these as config or environment-driven switches.
- Source: `docs/open-issues-report-2026-03-25.json` (OI-09)

### OI-05 — feed channel is shallow and can drop frames under pressure
- Status: `open` — GitHub: [#6](https://github.com/svabi79/argus-sdr/issues/6)
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
