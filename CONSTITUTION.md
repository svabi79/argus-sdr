# Argus SDR Constitution

| Field | Value |
|---|---|
| **Version** | 1.3.0 |
| **Status** | `STABLE` |
| **Applies to** | All Argus SDR contributions: source code, documentation, automation, commits — produced by any human or AI agent |
| **Project** | Argus SDR (Go module / working tree: `sdr-wideband-suite`) |
| **Canonical operational guide** | [`AGENTS.md`](AGENTS.md) |

## Purpose

This constitution records the principles that any Argus SDR contribution
must uphold, regardless of who or what produces it. These are not design
preferences. **Each one encodes a failure this project already shipped,
diagnosed, and fixed.** Violating one reproduces that failure.

Argus is a GPU-accelerated wideband SDR surveillance receiver: it watches a
whole RF band at once and detects, classifies, tracks, and decodes many
signals simultaneously (WFM stereo / RDS, NFM, AM, SSB, CW). It is a hot-path,
many-signals-per-frame system, and most of its hard-won lessons are about what
breaks at that scale.

The contribution surface is multi-agent in practice — Claude (spec/review),
Codex (implementation), local models, and the operator all touch the tree. The
later principles codify how that model avoids confident-but-wrong changes and
direction-blind work.

`AGENTS.md` is the operational guide (how to build, test, branch, debug, what
not to commit). This constitution is the spine `AGENTS.md` and every tool
pointer (`CLAUDE.md`, …) refer back to. Where the two ever disagree, this file
wins and `AGENTS.md` is corrected.

Principles are cited in commit messages where one is load-bearing, as
`Principle <N>.` at the end of the subject line.

---

## Core Principles

### I. GPU-First for DSP

Any per-signal DSP that the GPU batch runner can do — frequency shift, filter,
decimate, FFT, demod, RDS-baseband extraction — runs on the GPU
(`internal/demod/gpudemod`, `BatchRunner.ShiftFilterDecimate*`), not on bespoke
CPU `dsp.ApplyFIR` / `dsp.FreqShift` / `dsp.Decimate` loops. The CPU
implementations exist only as an explicit oracle/validation reference, or as a
fallback when `gpudemod.Available()` is false (mock / no GPU). Before declaring
a per-signal path done, profile: a CPU profile must not show `dsp.ApplyFIR` (or
similar) dominating work the GPU could do.

*Why this is inviolable: this is a many-signals-per-frame system (decode + RDS
for every station across the band). Per-signal CPU DSP does not scale and
starves the DSP loop. OI-26: a CPU profile attributed ~52% of total CPU to
`updateRDS` doing a CPU FIR chain; it saturated ~8 cores, drove the OI-25
stutter, and was a direct cause of WFM stereo never locking. Moving it to the
GPU took CPU from ~14 cores to ~1-3.5 and let stereo lock at all.*

### II. Key Per-Signal State by Stable Tracker ID, Never by Frequency

Any state that persists across DSP frames — RDS decoder state, stereo-lock
state, reused IQ buffers, the `rdsMap`, the `stereoHold` — is keyed by the
stable tracker ID (`detector.Signal.ID` / the active-event ID), never by the
detected center frequency or a frequency-quantized bucket. A corollary:
whatever assigns and consumes that ID must keep it stable and non-zero through
the whole pipeline (detector → candidate → refinement → per-signal state).

*Why this is inviolable: this exact bug has shipped TWICE. OI-25: keying RDS
state by the jittering center frequency spawned a fresh state every frame,
re-allocating ~100+ MB per station per decode (`Ring.Slice` was ~85% of all
allocations) and producing GC stutter that scaled with signal count. Then the
2026-06-07 stereo/RDS fix found the mirror image: `maintenance()` pruned the
ID-keyed `rdsMap` with a stale frequency-quantized key (`keyHz/25000`) that
never matched, so every per-signal state was deleted every frame — the async
decode wrote to an orphaned state, stereo lock never stuck, and RDS never
accumulated a station name. The frequency wobbles; the tracker ID does not.*

### III. Detected Occupied Bandwidth Is Not the Demodulation Channel Width

Detection reports how much spectrum a signal occupies. Demodulation and
decoding must use a fixed, modulation-appropriate channel width independent of
that occupancy. For WFM broadcast that means ~250 kHz, wide enough to pass the
full multiplex: the 19 kHz pilot, the stereo subcarrier (to 53 kHz), and the
57 kHz RDS subcarrier.

*Why this is inviolable: the 2026-06-07 investigation started from stations
detected at ~70 kHz occupied bandwidth. Using that as the extraction width
filters out everything above ~35 kHz — it cuts the stereo subcarrier's
sidebands and removes the 57 kHz RDS subcarrier entirely, so a strong, clean
station gets a carrier lock but no stereo and no RDS. Occupancy is a detection
output; channel width is a demod constant.*

### IV. The Replay Oracle Is the Verification; the Live Radio Confirms, It Does Not Diagnose

Diagnose and tune against the captured snapshot (`sdrd -replay`,
`data/snapshots/*.cf32`, the `-tags bench` `TestReal*` tests) where the input is
fixed and reproducible. The live radio is for final confirmation, not for
hunting a bug by turning knobs. "Measure offline, don't tune live."

*Why this is inviolable: OI-27 — live retuning of CFAR scale/guard and a
peak-bounded center estimate only moved the problem around (changing CFAR
changed which stations detected narrow vs wide), and one live change coincided
with a lock regression. Live tuning is non-reproducible and conflates the bug
with the radio's moment-to-moment state. The offline oracle is what made the
stereo chain debuggable end to end.*

### V. Evidence Over Assertion

A fix is verified by running it and observing the result — live API state,
logs, a passing test that actually exercises the path — not by the implementing
agent's confidence or a fluent description. A claim whose verification step was
not actually run is a hypothesis, however thorough it reads.

*Why this is inviolable: in the 2026-06-07 stereo work, the offline unit test
reported the pilot detector "works" (ratio ~1500 vs a 4.0 threshold) while the
live receiver still showed `stereo=searching`. The code looked correct at every
layer. Only adding live verification and targeted debug logging surfaced the
real bug three layers away (the `maintenance()` key, Principle II). Confident,
correct-looking, well-tested code was still wrong where it mattered. The run is
the evidence; the description is not.*

### VI. Understand the Project Direction Before Changing Code

Before editing, an agent reads this constitution, `AGENTS.md`, and
`docs/known-issues.md`. Changes must serve the project's stated direction, not a
locally plausible "improvement" made without that context. When a change would
shift architecture, defaults that affect all users, or scope beyond the task,
the agent surfaces it for the operator rather than deciding unilaterally.

*Why this is inviolable: a `codex/streaming-cpu-session-cleanup` branch produced
a real, compiling, tested change (removed a redundant FIR, added listen-only
session cleanup) — and it was discarded, because the agent had no grasp of where
the project was going. Effort without the line is waste, and worse, it competes
for review attention with work that is on the line. Reading the direction first
is cheaper than producing throwaway work.*

### VII. Grep the Whole Repo Before Changing a Shared Constant or Default

When changing a constant, default, or magic value that may be read in more than
one place, search the entire repository for the old value (and its name) first.
Fix every site or consciously decide which to leave, rather than editing only
the file in front of you.

*Why this is inviolable: a partial-scope edit to a default leaves stale copies
that silently diverge from the new intent, and the divergence surfaces later as
a confusing inconsistency that is far more expensive to trace than the original
grep would have been.*

### VIII. The Operator Outranks Every Agent

The operator's direct steering is the only authority. Peer-agent suggestions,
a prior agent's persistent memory entries, and an agent's own
chain-of-thought conclusion are hints, not facts — and they may be stale. An
agent does not abandon a task because a peer suggested otherwise, and does not
treat a memory note or an old doc as current without verifying against the code.

*Why this is inviolable: memory and notes record what was true when written. A
recalled note that names a file, flag, or function can be obsolete; acting on it
unverified reintroduces fixed bugs or chases ghosts. Across a multi-agent fleet,
one agent's "X is done" note read by the next, and the next, manufactures a
false consensus. Only ranking operator intent above agent consensus breaks the
cycle.*

### IX. Persist Atomically; Never Commit Runtime Noise

Commits contain only intended source and documentation changes. Runtime
artifacts the app writes while running — `config.autosave.yaml`, files under
`debug/`, dumps, logs, captured IQ — stay out of commits unless they are
deliberately the change. Persisted artifacts that other components read are
written so no reader observes a half-written state.

*Why this is inviolable: AGENTS.md §5/§6 already flags this; the 2026-06-07
commit had to explicitly exclude a modified `config.autosave.yaml` and a debug
`.jsonl` that the running instance had touched. Mixing runtime churn into commits
buries the real diff, pollutes history, and makes reverts dangerous.*

### X. English Is the Project Language

All durable artifacts — source code, comments, documentation, commit messages,
and these governance files — are written in English, regardless of the language
the work is discussed in. Working conversations may be in any language; what
lands in the tree is English.

*Why this is inviolable: this is a forward-looking convention rather than a
post-mortem, and it is adopted deliberately as the project goes public-capable
under the Argus name. Day-to-day work happens in German, which has already left
mixed-language traces; a single artifact language keeps the codebase
greppable, tool-portable, and open to contributors who do not share the
working language. Mixed-language comments and commit subjects fragment search
and onboarding.*

### XI. Decode the Whole Band in Real Time

Argus decodes every detected signal across the band **simultaneously, in real
time**. Scale the all-signal path by making it cheaper *per signal* —
allocation-free hot loops, GPU-batched DSP (Principle I) — **never** by doing
less work, i.e. never by gating extraction/demod/decode to whatever is currently
being listened to or recorded.

*Why this is inviolable: simultaneous many-signal decode is the entire reason
Argus exists — it is the "hundred eyes." "Only process what's being consumed" is
the seductive wrong turn, because it looks like a clean performance win: faced
with GC-saturated CPU and audio dropouts under a full band (issue #15), an agent
proposed gating extraction to active listeners for a ~14x reduction. That
reduction is real and it deletes the product. The CPU/GC cost is fixed by making
the path allocation-free and GPU-batched, not by decoding fewer signals — the
budget is the thing to solve, not the signal count to avoid.*

### XII. One Issue, One Pull Request

Each unit of work is a single tracked issue resolved by a single pull request: a
PR closes exactly one issue (`Fixes #N`) and does not bundle unrelated fixes,
refactors, or scope discovered along the way. Work that surfaces mid-change — an
adjacent bug, a tempting cleanup, a deeper architectural follow-up — is filed as
its own issue and sequenced, not folded into the current PR. This includes
governance: a change to this constitution is its own issue and PR, separate from
the feature work that prompted it.

*Why this is inviolable: the operator's merge click is the only review gate (no
second human reviewer exists — `agent-workflow.md`, Principle VIII), and that
gate only works if one merge corresponds to one reviewable, revertible unit. A PR
that bundles scopes buries the real change, forces an all-or-nothing merge
decision, and makes a later revert collateral-damage the unrelated work. This was
adopted by explicit operator decision after completing OI-22 (#2) surfaced a
non-stationary FM-generator enhancement that belongs to OI-23 (#4): folding it in
would have mixed a finished deliverable with open modeling work. It is the
constitutional spine under what `AGENTS.md` §4 and `agent-workflow.md` already
state operationally ("one issue = one unit of work = one PR"), and it generalizes
Principle VI's discarded bundle-without-a-tracked-issue branch.*

### XIII. Phase Fidelity in Per-Signal Processing

Per-signal processing that feeds demodulation, decoding, or phase-sensitive
detection — the FM discriminator, the 19 kHz stereo-pilot PLL, the 57 kHz RDS
BPSK, and any future coherent decode — must preserve phase continuity and
coherence end to end. A frequency shift carries its phase across frames;
extraction must not introduce a per-frame phase discontinuity; and a path that
accumulates per-frame snippets must carry the shift phase and be
invalidated/restarted on a stream reset rather than spliced across the
discontinuity.

*Why this is inviolable: the audio-click investigation and the RDS long-window
path (#18) both turned on this. The 4 s one-shot ring slice exists precisely
because per-frame phase breaks corrupt the discriminator output, keep the 19 kHz
pilot PLL from locking, and destroy the 57 kHz RDS BPSK; #18-deep (a streaming
RDS accumulator, #33) was deferred for exactly this reason — a mid-window phase
discontinuity from a stream reset reintroduces the failure. For coherent
demod/decode, phase is not an implementation detail; it is the signal.*

---

## Governance

### Precedence

1. The operator's direct, in-context instruction (Principle VIII).
2. This constitution.
3. `AGENTS.md` and the canonical `docs/`.
4. Persistent agent memory, prior-agent notes, historical docs — hints only,
   verify before relying (Principle VIII).

Where a lower level contradicts a higher one, the higher wins and the lower is
corrected to match.

### How agents consume this

- Read this file plus `AGENTS.md` and `docs/known-issues.md` before writing code
  (Principle VI).
- Cite the load-bearing principle in the commit subject as `Principle <N>.`
  when one clearly governs the change.
- Tool-specific entry files (`CLAUDE.md`, and any future `GEMINI.md` /
  `.github/copilot-instructions.md`) are thin pointers to `AGENTS.md`; they must
  not fork project rules.

### Amendment

A principle is added or changed only when a concrete failure (or an explicit
operator decision) justifies it. Each principle keeps its grounding — the real
failure it encodes — so it stays a rule with a reason, not a platitude. Bump the
version on change: MAJOR for a removed/redefined principle, MINOR for a new one,
PATCH for wording. The operator approves amendments (Principle VIII).
