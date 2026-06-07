# AGENTS.md

This file is the repo-level working guide for humans, coding agents, and LLMs.
Read it before making changes.

---

## 1. Purpose of this file

Use this file as the canonical "how to work in this repo" guide.
It is intentionally practical and operational.

Use it to answer questions like:
- Where should changes go?
- What must not be committed?
- How should builds/tests be run?
- Which docs are canonical?
- How should debugging work be documented?
- How should agents behave when touching this repo?

---

## 2. Repo intent

`sd r-wideband-suite` is a Go-based SDR analysis and streaming system with:
- live spectrum/waterfall UI
- signal detection/classification
- extraction / demodulation / recording
- GPU-assisted paths
- streaming audio paths
- extensive telemetry/debugging support

This repo has gone through active streaming-path and audio-click debugging.
Do not assume older comments, notes, or experimental code paths are still authoritative.
Prefer current code, current docs in `docs/`, and current branch state over historical assumptions.

---

## 3. Canonical documentation

### Keep as primary references
- `README.md`
  - high-level project overview
  - build/run basics
  - feature summary
- `ROADMAP.md`
  - longer-lived architectural direction
- `docs/known-issues.md`
  - curated open engineering issues
- `docs/telemetry-api.md`
  - telemetry endpoint documentation
- `docs/telemetry-debug-runbook.md`
  - telemetry/debug operating guide
- `docs/audio-click-debug-notes-2026-03-24.md`
  - historical incident record and final resolution notes for the audio-click investigation
- `docs/architecture-review-2026-06-06.md`
  - architecture/implementation review: strengths, ausbau-schieflage (arbitration > perception),
    prioritized findings (B-1..B-6) and recommended execution order
- `docs/classifier-ml-plan-2026-06-06.md`
  - detailed plan for adding ML to the classifier (data+benchmark first → Stage A trees → Stage B CNN),
    including automated data collection strategy
- `docs/detection-rework-plan-2026-06-06.md`
  - step-wise plan to make the perception layer universal and measurable
    (R0 ground-truth benchmark → R1 occupied-bandwidth/SNR re-estimation →
    R2 Welch PSD/noise → R3 multi-resolution detection → R4 classification → R5 real)
  - this is the current IMMEDIATE NEXT PRIORITY (Phase R), ahead of ROADMAP Phase 5

### Treat as historical / contextual docs
Anything in `docs/` that reads like an incident log, deep debug note, or one-off investigation should be treated as supporting context, not automatic source of truth.

### Do not create multiple competing issue lists
If new open problems are found:
- update `docs/known-issues.md`
- keep raw reviewer/ad-hoc reports out of the main repo flow unless they are converted into curated docs

---

## 4. Branching and workflow rules

### Current working model
- `master` is the canonical active branch.
- Use focused short-lived branches for real feature/fix work when needed.
- Do not keep long-lived junk/debug branches alive once the useful work has been transferred.
- Prefer short-lived cleanup branches for docs/config cleanup.

### Branch hygiene
- Do not pile unrelated work onto one branch if it can be split cleanly.
- Keep bugfixes, config cleanup, and large refactors logically separable when possible.
- Before deleting an old branch, ensure all useful work is already present in the active branch or merged into the main line.
- After merge, prefer deleting obsolete local branches so `master` stays the obvious default.

### Mainline policy
- Do not merge to `master` blindly.
- Before merge, prefer at least a short sanity pass on:
  - live playback
  - recording
  - WFM / WFM_STEREO / at least one non-WFM mode if relevant
  - restart behavior if the change affects runtime state

---

## 5. Commit policy

### Commit what matters
Good commits are:
- real code fixes
- clear docs improvements
- deliberate config-default changes
- cleanup that reduces confusion

### Do not commit accidental noise
Do **not** commit unless explicitly intended:
- local debug dumps
- ad-hoc telemetry exports
- generated WAV debug windows
- temporary patch files
- throwaway reviewer JSON snapshots
- local-only runtime artifacts

### Prefer small, readable commit scopes
Examples of good separate commit scopes:
- code fix
- config default cleanup
- doc cleanup
- known-issues update

---

## 6. Files and paths that need extra care

### Config files
- `config.yaml`
- `config.autosave.yaml`

Rules:
- These can drift during debugging.
- Do not commit config changes accidentally.
- Only commit them when the intent is to change repo defaults.
- `config.autosave.yaml` may be intentionally kept locally modified and uncommitted.
- Keep in mind that `config.autosave.yaml` can override expected runtime behavior after restart.

### Debug / dump artifacts
Examples:
- `debug/`
- `tele-*.json`
- ad-hoc patch/report scratch files
- generated WAV capture windows

Rules:
- Treat these as local investigation material unless intentionally promoted into docs.
- Do not leave them hanging around as tracked repo clutter.

### Root docs
The repo root should stay relatively clean.
Keep only genuinely canonical top-level docs there.
One-off investigation output belongs in `docs/` or should be deleted.

---

## 7. Build and test rules

### Core DSP rule — GPU first (2026-06-07)
**Any DSP that can run on the GPU must run on the GPU.** This is a deliberately
hot-path, many-signals system (e.g. decode + RDS for every signal across the band),
so per-signal CPU DSP does not scale and starves the DSP loop.

Concretely:
- Shift / filter / decimate / FFT / demod / RDS-baseband extraction for per-signal
  work should route through the GPU batch path (`internal/demod/gpudemod`,
  `BatchRunner.ShiftFilterDecimate*`), not bespoke `dsp.ApplyFIR` / `dsp.FreqShift`
  / `dsp.Decimate` loops on the CPU.
- CPU implementations stay only as: an explicit oracle/validation reference, or a
  fallback when `gpudemod.Available()` is false (mock / no GPU).
- When adding a new per-signal processing path, ask first: "can the GPU batch
  runner do this shift/filter/decimate?" If yes, use it.
- Profile before declaring done: a CPU profile should not show `dsp.ApplyFIR`
  (or similar) dominating for work that the GPU could do. (How this rule was found:
  a CPU profile showed `updateRDS` doing CPU FIR at ~52% of total CPU — OI-26.)

### General rule
Prefer the repo's own scripts and established workflow over ad-hoc raw build commands.

### Important operational rule
Before coding/build/test sessions on this repo:
- stop the browser UI
- stop `sdrd.exe`

This avoids file locks, stale runtime state, and misleading live-test behavior.

### Build preference
Use the project scripts where applicable, especially for the real app flows.
Examples already used during this project include:
- `build-sdrplay.ps1`
- `start-sdr.ps1`

Do **not** default to random raw `go build` commands for full workflow validation unless the goal is a narrow compile-only sanity check.

### GPU / native-path caution
If working on GPU/native streaming code:
- do not assume the CPU oracle path is currently trustworthy unless you have just validated it
- do not assume old README notes inside subdirectories are current
- check the current code and current docs first

---

## 8. Debugging rules

### Telemetry-first, but disciplined
Telemetry is available and useful.
However:
- heavy telemetry can distort runtime behavior
- debug config can accidentally persist via autosave
- not every one-off probe belongs in permanent code

### When debugging
Prefer this order:
1. existing telemetry and current docs
2. focused additional instrumentation
3. short-lived dumps / captures
4. cleanup afterward

### If you add debugging support
Ask:
- Is this reusable for future incidents?
- Should it live in `docs/known-issues.md` or a runbook?
- Is it temporary and should be removed after use?

### If a reviewer provides a raw report
Do not blindly keep raw snapshots as canonical repo docs.
Instead:
- extract the durable findings
- update `docs/known-issues.md`
- keep only the cleaned/curated version in the main repo flow

---

## 9. Documentation rules

### Prefer curated docs over raw dumps
Good:
- `docs/known-issues.md`
- runbooks
- architectural notes
- incident summaries with clear final status

Bad:
- random JSON reviewer dumps as primary docs
- duplicate issue lists
- stale TODO/STATE files that nobody maintains

### If a doc becomes stale
Choose one:
- update it
- move it into `docs/` as historical context
- delete it

Do not keep stale docs in prominent locations if they compete with current truth.

---

## 10. Known lessons from recent work

These are important enough to keep visible:

### Audio-click investigation lessons
- The final click bug was not a single simple DSP bug.
- Real causes included:
  - shared-buffer mutation / aliasing
  - extractor reset churn from unstable config hashing
  - streaming-path batch rejection / fallback behavior
- Secondary contributing issues existed in discriminator bridging and WFM mono/plain-path filtering.

### Practical repo lessons
- Silent fallback paths are dangerous; keep important fallthrough/fallback visibility.
- Shared IQ buffers should be treated very carefully.
- Debug artifacts should not become permanent repo clutter.
- Curated issue tracking in Git is better than keeping raw review snapshots around.

---

## 11. Agent behavior expectations

If you are an AI coding agent / LLM working in this repo:

### Do
- read this file first
- prefer current code and current docs over old assumptions
- keep changes scoped and explainable
- separate config cleanup from code fixes when possible
- leave the repo cleaner than you found it
- promote durable findings into curated docs

### Do not
- commit local debug noise by default
- create duplicate status/todo/issue files without a strong reason
- assume experimental comments or old subdirectory READMEs are still correct
- leave raw reviewer output as the only source of truth
- hide fallback behavior or silently ignore critical path failures

---

## 12. Recommended doc update pattern after meaningful work

When a meaningful fix or investigation lands:
1. update code
2. update any relevant canonical docs
3. update `docs/known-issues.md` if open issues changed
4. remove or archive temporary debug artifacts
5. keep the repo root and branch state clean

---

## 13. Minimal pre-commit checklist

Before committing, quickly check:
- Am I committing only intended files?
- Are config changes intentional?
- Am I accidentally committing dumps/logs/debug exports?
- Should any reviewer findings be moved into `docs/known-issues.md`?
- Did I leave stale temporary files behind?

---

## 14. If unsure

If a file looks ambiguous:
- canonical + actively maintained -> keep/update
- historical but useful -> move or keep in `docs/`
- stale and confusing -> delete

Clarity beats nostalgia.
