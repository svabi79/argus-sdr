# SDR Wideband Suite - Persisted Roadmap

This file is the durable architectural roadmap and phase memory for the project.
It exists so a context reset, model switch, or new coding agent can continue from the real project state without reconstructing the entire plan from chat history.

## Current Project Status

The project has progressed through four major milestones:

- **Phase 1 complete**: architecture foundation
- **Phase 2 complete**: multi-resolution surveillance semantics
- **Phase 3 complete**: conservative runtime prioritization / admission / rebalance intelligence
- **Phase 4 complete**: monitor-window operating model

This roadmap reflects the actual code state as of the latest landed Phase-4 work.

---

## Phase 1 - Architecture Foundation (Complete)

### Intent
Turn the original SDR visualizer into a scalable, policy-driven wideband SDR engine.

### Delivered
- explicit config model for `pipeline`, `surveillance`, `refinement`, `resources`, `profiles`
- separation of surveillance / refinement / presentation concerns
- explicit candidate / refinement model
- centralized arbitration / admission surface for refinement / record / decode
- budget / hold / queue scaffolding
- profile and policy surface established

### Meaning
Phase 1 established the architecture so later scaling would not require another ground-up rewrite.

---

## Phase 2 - Multi-Resolution Surveillance Semantics (Complete)

### Intent
Make surveillance genuinely multi-level and connect those levels to candidate behavior.

### Delivered
- operational surveillance level sets
- derived / decimated surveillance spectra
- primary / derived / support / presentation level roles
- derived detection governance
- candidate evidence model across levels
- primary/derived fusion
- level-aware refinement scoring
- debug/API visibility for surveillance levels and evidence

### Meaning
Phase 2 moved the system from a single-spectrum mentality toward a real multi-resolution surveillance engine.

---

## Phase 3 - Runtime Prioritization / Admission / Rebalance (Complete)

### Intent
Move from architecture + heuristics to conservative runtime intelligence.

### Delivered
- priority tiers and admission classes
- richer reason taxonomy across refinement / record / decode
- budget preference and effective budget semantics
- pressure summaries
- hold / protection / opportunistic displacement semantics
- intent-aware hold behavior
- family-aware tier floors
- conservative adaptive cross-resource rebalance
- debug/API visibility for admission / pressure / rebalance state

### Meaning
Phase 3 created a real runtime decision layer rather than just a scoring layer.
It is intentionally conservative, explainable, and incremental rather than a full adaptive scheduler.

---

## Phase 4 - Monitor-Window Operating Model (Complete)

### Intent
Turn monitor windows from simple goal/gating objects into operational monitoring zones inside a capture span.

### Delivered
- `monitor_windows` in config/goals/policy/runtime
- monitor-window candidate gating
- per-window candidate attribution
- per-window stats
- monitor-window priority bias
- window-based record/decode actions
- lightweight zone hints:
  - `focus`
  - `record`
  - `decode`
  - `background`
- consolidated `window_summary`
- per-window pressure / outcome summaries
- overlap and zone/action tests
- debug/API visibility for monitor-window behavior

### Meaning
Phase 4 delivered a practical operating model for multiple monitoring zones within a single capture span, without requiring full hardware multi-span orchestration.

---

# Next Planned Phases

## Phase 5 - Span / Operating-Orchestration

### Core idea
Move from monitor windows inside one capture span to richer orchestration across operational spans / span groups / monitoring areas.

### Target outcomes
- explicit span / zone groups
- span-aware resource allocation
- stronger profile-driven operating modes
- retune / dwell / revisit semantics where hardware or mode requires it
- more operational behavior across multiple monitoring regions, not just one captured span

### Likely sub-areas
- span orchestration model
- span-aware resource allocation
- profile-driven operating modes
- retune / scan / dwell semantics

### Important note
Phase 5 should still avoid unnecessary premature hardware-specific complexity, but it is the logical next step after the monitor-window model.

---

## Future Architecture Note - Observation/Track Reconciliation

A likely later improvement is an explicit reconciliation layer between:
- raw surveillance observations / candidates
- stable tracked signals / identities

This is intentionally NOT the first fix for live-audio regressions.
For now, stable-ID-carrying signal sources should be used for stream/session-sensitive paths.
If needed later, a dedicated observation-to-track reconciliation layer can be introduced as its own architecture block.

## Phase 6 - Adaptive QoS / Scheduler Intelligence

### Core idea
Make the engine more adaptive under changing load and signal density.

### Target outcomes
- deeper QoS model
- adaptive runtime rebalance beyond the conservative Phase-3 layer
- richer priority ecology (decay, revisit, persistence, overload behavior)
- broader operating-state behavior under calm / dense / overload conditions

### Important note
This should build on the operational structures from Phase 5 rather than bypass them.

---

## Phase 7 - Performance / Scale / Hardware Depth

### Core idea
Turn the architecture into a genuinely high-bandwidth, high-throughput system.

### Target outcomes
- GPU spectral pyramid / multi-resolution acceleration
- better batching / buffering / async processing
- larger stable operating bandwidths
- profiling / benchmark / regression discipline for throughput

### Important note
Performance work should follow the behavioral model, not lead it.

---

## Phase 8 - Productization / Operator Workflow / Polish

### Core idea
Turn the powerful engine into a polished operator product.

### Target outcomes
- better UX and status visibility
- reusable monitoring plans / templates
- export / review / workflow polish
- safer and clearer autonomous behavior
- polished docs, defaults, and examples

---

# Practical Continuation Guidance

If a future agent resumes work, the recommended order is:

1. **Start with Phase 5**, not Phase 6/7/8.
2. Treat Phases 1-4 as completed milestones.
3. Avoid reopening already-closed phase work unless there is a concrete bug or mismatch.
4. Before starting a new major block, inspect the current repo state and confirm the roadmap still matches the code.
5. Prefer milestone-sized coherent blocks over broad speculative redesigns.

---

# Current Default Testing / Operator Context

A valid FM/UKW-oriented test config was intentionally introduced for easier local testing.
Current default direction:
- FM broadcast band (87.5-108 MHz)
- centered around 99.5 MHz
- `wideband-balanced`
- `broadcast-monitoring`
- WFM/RDS-friendly priorities

If that changes later, update this file when the default operator/testing posture changes materially.

---

# Why this file exists

This file is meant to survive:
- context resets
- model changes
- agent swaps
- long pauses in development

It should be kept updated whenever a phase is meaningfully completed or the roadmap changes materially.
