# SDR Wideband Suite - Current State

This file is the practical handoff / resume state for future work.
Use it together with `ROADMAP.md`.

- `ROADMAP.md` = long-term architecture and phase roadmap
- `STATE.md` = current repo state, working conventions, and next recommended entry point

## Current Milestone State

- **Phase 1 complete**
- **Phase 2 complete**
- **Phase 3 complete**
- **Phase 4 complete**

Current project state should be treated as:
- Phase 1 = architecture foundation landed
- Phase 2 = multi-resolution surveillance semantics landed
- Phase 3 = conservative runtime prioritization/admission/rebalance landed
- Phase 4 = monitor-window operating model landed

Do not reopen these phases unless there is a concrete bug, mismatch, or regression.

---

## Most Recent Relevant Commits

These are the most important recent milestone commits that define the current state:

### Phase 4 monitor-window operating model
- `efe137b` Add monitor window goals for multi-span gating
- `ac64d6b` Add monitor window matches and stats
- `d7e457d` Expose monitor window summaries in runtime debug
- `c520423` Add monitor window priority bias
- `838c941` Add window-based record/decode actions
- `962cf06` Add window zone biases for record/decode actions
- `402a772` Consolidate monitor window summary in debug outputs
- `8545b62` Add per-window outcome summaries for admission pressure
- `65b9845` test: cover overlapping monitor windows
- `efe3215` docs: capture Phase-4 monitor-window status

### Phase 3 runtime intelligence milestone
- `4ebd51d` Add priority tiers and admission classes to pipeline
- `18b179b` Expose admission metadata in debug output and tests
- `ba9adca` Add budget preference and pressure modeling
- `7a75367` Expose arbitration pressure summary
- `592fa03` pipeline: deepen hold/displacement semantics
- `30a5d11` pipeline: apply intent holds and family tier floors
- `1f5d4ab` pipeline: add intent and family priority tests
- `822829c` Add conservative budget rebalance layer
- `da5fa22` Update Phase-3 Wave 3E status

### Documentation / stable defaults
- `fd718d5` docs: finalize phase milestones and ukf test config

If resuming after a long pause, inspect the current `git log` around these commits first.

---

## Current Important Files / Subsystems

### Long-term guidance
- `ROADMAP.md` - durable roadmap across phases
- `STATE.md` - practical resume/handoff state
- `PLAN.md` - project plan / narrative (may be less pristine than ROADMAP.md)
- `README.md` - user-facing/current feature status

### Config / runtime surface
- `config.yaml` - current committed default config
- `config.autosave.yaml` - local autosave; intentionally not tracked in git
- `internal/config/config.go`
- `internal/runtime/runtime.go`

### Phase 3 core runtime intelligence
- `internal/pipeline/arbiter.go`
- `internal/pipeline/arbitration.go`
- `internal/pipeline/arbitration_state.go`
- `internal/pipeline/priority.go`
- `internal/pipeline/budget.go`
- `internal/pipeline/pressure.go`
- `internal/pipeline/rebalance.go`
- `internal/pipeline/decision_queue.go`

### Phase 2 surveillance/evidence model
- `internal/pipeline/types.go`
- `internal/pipeline/evidence.go`
- `internal/pipeline/candidate_fusion.go`
- `internal/pipeline/scheduler.go`
- `cmd/sdrd/pipeline_runtime.go`

### Phase 4 monitor-window model
- `internal/pipeline/monitor_rules.go`
- `cmd/sdrd/window_summary.go`
- `cmd/sdrd/level_summary.go`
- `cmd/sdrd/http_handlers.go`
- `cmd/sdrd/decision_compact.go`
- `cmd/sdrd/dsp_loop.go`

---

## Current Default Operator / Test Posture

The repo was intentionally switched to an FM/UKW-friendly default test posture.

### Current committed config defaults
- band: `87.5-108.0 MHz`
- center: `99.5 MHz`
- sample rate: `2.048 MHz`
- FFT: `4096`
- profile: `wideband-balanced`
- intent: `broadcast-monitoring`
- priorities include `wfm`, `rds`, `broadcast`, `digital`

### Important config note
- `config.yaml` is committed and intended as the stable default reference
- `config.autosave.yaml` is **not** git-tracked and may diverge locally
- if behavior seems odd, compare the active runtime config against `config.yaml`

---

## Working Conventions That Matter

### Codex invocation on Windows
Preferred stable flow:
1. write prompt to `codex_prompt.txt`
2. create/use `run_codex.ps1` containing:
   - read prompt file
   - pipe to `codex exec --yolo`
3. run with PTY/background from the repo root
4. remove `codex_prompt.txt` and `run_codex.ps1` after the run

This was adopted specifically to avoid PowerShell quoting failures.

### Expectations for coding runs
- before every commit: `go test ./...` and `go build ./cmd/sdrd`
- commit in coherent blocks with clear messages
- push after successful validation
- avoid reopening already-closed phase work without a concrete reason

---

## Known Practical Caveats

- `PLAN.md` has had encoding/character issues in some reads; treat `ROADMAP.md` + `STATE.md` as the cleaner authoritative continuity docs.
- README is generally useful, but `ROADMAP.md`/`STATE.md` are better for architectural continuity.
- `config.autosave.yaml` can become misleading because it is local/autosaved and not tracked.

---

## Recommended Next Entry Point

If resuming technical work after this checkpoint:

### Start with **Phase 5**
Do **not** reopen Phase 1-4 unless there is a concrete bug or regression.

### Recommended Phase 5 direction
Move from monitor windows inside a single capture span toward richer span / operating orchestration:
- span / zone groups
- span-aware resource allocation
- stronger profile-driven operating modes
- retune / scan / dwell semantics where needed

### Avoid jumping ahead prematurely to
- full adaptive QoS engine (Phase 6)
- major GPU/performance re-architecture (Phase 7)
- heavy UX/product polish (Phase 8)

Those should build on Phase 5, not bypass it.

---

## Resume Checklist For A Future Agent

1. Read `ROADMAP.md`
2. Read `STATE.md`
3. Check current `git log` near the commits listed above
4. Inspect `config.yaml`
5. Confirm current repo state with:
   - `go test ./...`
   - `go build ./cmd/sdrd`
6. Then start Phase 5 planning from the actual repo state

If these steps still match the repo, continuation should be seamless enough even after a hard context reset.
