# Detection & Estimation Architecture

How Argus turns a wideband spectrum into a set of tracked, measured, classified
emissions — and the **contracts** that keep that pipeline universal across bands
and modulations as it grows.

This document is canonical *direction/architecture* (what we are building and
why). It is referenced from [`AGENTS.md`](../AGENTS.md) §3. The inviolable
invariants live in [`CONSTITUTION.md`](../CONSTITUTION.md) — especially
Principle I (GPU-first), XI (decode the whole band in real time) and XIII (phase
fidelity). The step-by-step plan lives in
[`detection-rework-plan-2026-06-06.md`](detection-rework-plan-2026-06-06.md); the
open issues in [`known-issues.md`](known-issues.md).

---

## 1. What Argus is (and is not)

Argus is a **universal wideband surveillance receiver**: it watches a whole RF
band at once and detects, separates, measures, classifies, tracks and decodes
**every** emission on it, in real time — from a ~100 Hz CW carrier to a ~256 kHz
WFM multiplex, in any band (broadcast FM, the ham bands, AM broadcast, utility,
and whatever sits between).

It is **not** a BC-FM channelizer. BC-FM is merely the easiest first target
(strong, well-understood, stereo/RDS structure). The design must generalize.

Two consequences fall out of this, and they are load-bearing:

- **No frequency-band prior may decide class or width.** A WFM-wide signal can
  appear in a ham band; BC-FM DX / overreach puts stations off the domestic
  raster; pirates and OIRT remnants sit off-grid entirely. "Center is in
  87.5–108 MHz ⇒ WFM ⇒ clamp to 256 kHz" is therefore **wrong** — it falsifies
  the occupied bandwidth of real off-raster, DX, and non-broadcast emissions.
- **No modulation-specific cue may be a prerequisite for detection.** A mono
  BC-FM station has no 19 kHz pilot; a CW carrier has no MPX. Class cues are
  *refinements* layered on top of a class-agnostic backbone, never gates.

The measurement reports **what is on the air**, not what a regulator's grid
expects.

---

## 2. The layered model

```
L1  Universal detection / resolution   — find & separate every emission, any
                                          band/modulation, by structure across SCALE
        │   (scale-aware candidates)
        ▼
L2  Generic estimation                 — occupied bandwidth / center / SNR via
                                          power-containment (modulation-agnostic)
        │   (refined signals; may SPLIT/MERGE the candidate set)
        ▼
L3  Per-class anchor refiners (plugins) — optional, modulation-specific cues that
                                          add information where energy is ambiguous
        │   (WFM 19 kHz pilot, AM carrier symmetry, SSB asymmetry, CW narrowband,
        │    digital cyclostationarity …)
        ▼
L4  Tracking / identity                 — stable tracker IDs across frames (Const. II)
```

- **L1 is the backbone and the root fix for "bridging".** Two adjacent strong
  stations merged into one over-wide candidate is a *resolution* failure, not an
  estimator failure (the estimator faithfully measures whatever blob it is
  handed). Resolving them belongs here.
- **L2 is already class-agnostic** (`internal/estimate`): power-containment
  occupied bandwidth + centroid + peak-over-noise. It is correct once L1 hands it
  a clean single-emission region.
- **L3 anchors are thin and incremental.** They are the *only* grid-free handle on
  cases energy detection cannot resolve (e.g. two equal WFM stations whose skirts
  overlap → two distinct 19 kHz pilots = two stations). They must be optional:
  the pipeline is correct with all of them disabled.
- **L4 already exists** and is governed by Constitution II.

---

## 3. The contracts (lock these once; implement behind them incrementally)

These interfaces are what prevent building the same thing twice. Backbone now,
anchors later, both fit without a rewrite.

### C1 — Scale-aware candidate
A candidate carries `(centerHz, bandwidthHz, scale, confidence, provenance)`,
where *scale* is the resolution at which the emission is a coherent blob and
*provenance* records the scale(s) that produced it. One bin width cannot resolve
a 100 Hz CW and a 256 kHz WFM; the candidate must say at what scale it was seen.

### C2 — Refinement may change the candidate set (SPLIT / MERGE, not 1:1)
The refine stage may turn one candidate into several (an L3 anchor finding two
pilots in a merged blob; a notch-to-noise split) or several into one. Downstream
must not assume a 1:1 candidate→signal mapping. This is the docking point that
lets anchors and segmentation correct the set *without* rebuilding the detector.

### C3 — Pluggable, optional per-class refiner
A per-class refiner is a stage with a common interface that may inspect a
candidate's region/IQ and adjust/split/confirm it. Every refiner must be
no-op-able; the pipeline is correct with none registered. WFM-pilot is the first
instance and reuses existing pilot code (`classifier.StereoPilotPresent`, the RDS
path).

### C4 — Resource budget & profiling gate (see §4)
Multi-resolution is built as a **single-FFT scale-space**, allocation-free, with
the K-fold cost confined to detection. No detection change ships without a
delta-heap + within-run A/B profile on the replay oracle showing `gcDrain` share
and `capture.stream_reset` / `streamer.audio.gap` did not regress.

### C5 — Phase fidelity (Constitution XIII)
Any per-signal path that L3/decode adds must preserve phase continuity end to end
(shift phase carried across frames; no per-frame discontinuity; invalidate on
stream reset rather than splice). This is why detection works on magnitude
spectra (phase-agnostic) while the per-signal coherent paths (pilot PLL, RDS
BPSK, FM discriminator) are held to Constitution XIII.

---

## 4. Resource model (how it stays affordable)

The system already fought a GC-saturation / audio-dropout battle (#15 and the
#21–#32 allocation work). Multi-resolution must not reintroduce it.

- **Scale-space = multi-σ smoothing of ONE high-resolution PSD, never K FFTs.**
  Compute one FFT (sized for the narrowest target signal — e.g. 65536 at
  2.5 MS/s ≈ 38 Hz bins for CW), then derive coarser scales by box/Gaussian
  smoothing (O(N) per level, GPU-trivial). The only structural GPU increase is
  one larger FFT per frame.
- **K-fold cost is confined to detection.** CFAR + cross-scale fusion is
  O(N·K) on a few 10⁴–10⁵ bins → microseconds. Per-signal extraction / demod /
  RDS stays **once per resolved signal** — never per scale.
- **Allocation-free by construction.** Scale-space buffers are fixed-size (they
  do *not* scale with signal count) → preallocate once, reuse every frame.
  Candidate / estimator scratch is pooled. (CGO: no Go-slice field on a struct
  whose `&field` reaches `cudaMalloc` — keep reuse buffers off such structs.)
- **The real scaling lever is the resolved signal count**, feeding the existing
  (already allocation-free) per-signal path. Better resolution correctly yields
  *more* signals on a dense band — that is the point (Constitution XI), and it is
  the number to watch against the dropout budget, not the FFT cost.
- **Profiling gate (C4):** measure on `sdrd -replay data/snapshots/*.cf32` with
  delta-heap (two `/debug/pprof/allocs` 20 s apart, `pprof -base`) and within-run
  A/B via a live toggle; compare *shares*, not absolutes (replay density varies
  along the loop).

---

## 5. Universal vs per-class (what to build once vs incrementally)

| Build once (universal) | Add incrementally (per-class, thin) |
|---|---|
| L1 multi-resolution scale-space detection + scale-aware fusion | L3 anchors: WFM pilot, AM carrier, SSB asymmetry, CW, digital cyclostationarity |
| L2 occupied-bw / center / SNR estimation (`internal/estimate`) | Classifier features per kind |
| L4 tracking by stable ID | |
| The synthetic benchmark — already covers CW/AM/SSB/NFM/WFM/FSK/PSK/DIGITAL (`internal/synth`) | Per-kind benchmark scenes / ground truth as kinds are hardened |

The benchmark being already multi-modulation is what makes this efficient: every
layer is scored against known truth across kinds, offline (Constitution IV/V).

---

## 6. Sequencing

1. **Backbone first (L1, OI-21).** Multi-resolution scale-space detection +
   scale-aware merge/fusion against contracts C1–C4. This is the root fix for
   over-/under-detection and for bridging across *all* bands, not just WFM.
2. **L2 stays** (already shipped); confirm it behaves once L1 hands clean
   regions; keep its peak-relative skirt bound (`dynamicRangeDb`).
3. **L3 anchors, thinnest-first.** WFM 19 kHz pilot (reuses existing code) to
   resolve heavy WFM overlap the backbone cannot; then AM/SSB/CW/digital as
   needed. Each behind C3, each optional.

Why this order: anchors built before the backbone would partly duplicate work the
backbone later does (it already separates the common cases); the backbone shrinks
how often anchors are needed, so anchors come second and stay thin. The
split/merge contract (C2) means adding them later costs no rewrite.

Anti-goals (explicitly out): frequency-band priors; regulatory channel-grid
clamping of the *measured* occupied bandwidth; any per-class cue as a detection
prerequisite. (Demod channel width as a *demod* constant remains correct —
Constitution III — that is a different thing from the reported occupancy.)

---

## 7. Relationship to the open issues

- **OI-21** (#5) single-resolution detection → this is L1; the backbone.
- **OI-23** (#4) occupied-bw instability / bridging on dense strong FM → a
  *resolution* problem (L1), reproduced offline by the breathing/bridging
  benchmark; the estimator (L2) faithfully measures what it is handed.
- **OI-27** (#3) wandering phantom / center jitter → L1 stability + L4.
- **OI-22** (#2, done) the ground-truth benchmark that scores all of the above.
