# Autonomous Agent Workflow

How agents (and the operator) get work done in Argus SDR using GitHub
primitives — Issues, Labels, Pull Requests, Actions, branch protection. The
goal: an agent can go from a defined task to a merge-ready PR on its own, while
the [Constitution](../CONSTITUTION.md) gates (evidence, operator-outranks) are
enforced by infrastructure, not by prompt.

This document is canonical. It is the operational expansion of Constitution
Principles V, VI, and VIII; [`AGENTS.md`](../AGENTS.md) covers build/test/debug
specifics.

## The loop

```
Issue (spec) → Claim (assignee) → Branch → Implement + Verify → PR (Fixes #N)
   → CI gates (Actions) → Review → Operator merge → Issue closes → Doc sync
```

## Roles

| Role | Does | Does NOT | Constitution |
|---|---|---|---|
| **Claude** | triage, write specs, make issues `ready`, review PRs | merge | VI, V |
| **Codex** | implement `ready` issues, open PRs | change architecture/defaults, exceed scope | VI, IX |
| **Operator** | direction, final merge, hardware verification | — | VIII |

The split is **spec-driven**: an implementation agent only picks up issues
labeled `status:ready` — i.e. issues that already state a goal, acceptance
criteria, and a verification method. Writing that spec is the review/triage
agent's job and is what was missing the time a branch got built without
grasping project direction.

## Issues = the work queue

- One issue = one unit of work = one PR.
- Open engineering issues from [`docs/known-issues.md`](known-issues.md) are
  mirrored as GitHub Issues (OI-NN) with a back-link. `known-issues.md` stays
  the curated narrative; Issues are the executable units.
- File via the templates (`.github/ISSUE_TEMPLATE/`). A **Task** issue is not
  `ready` until goal + acceptance + verification are filled.

## Labels

| Prefix | Values |
|---|---|
| `area:` | dsp · detector · classifier · recorder · gpu · web · telemetry · build · docs |
| `kind:` | bug · feature · refactor · docs |
| `priority:` | p0 · p1 · p2 |
| `status:` | triage · ready · in-progress · blocked · needs-operator · needs-hardware |
| `agent:` | claude · codex |

- `status:needs-operator` — an autonomy boundary was hit (architecture, a
  default affecting all users, UX/visuals, scope beyond the issue). Waits for
  the operator (Constitution VI/VIII).
- `status:needs-hardware` — needs GPU/SDR verification the hosted CI cannot do;
  routes to the self-hosted runner / operator.

## Claim protocol (atomic, mortal)

Before working an issue:

```bash
gh issue edit <N> --add-assignee @me
gh issue edit <N> --add-label status:in-progress
```

- Already assigned to **another** agent? Leave a coordination comment; do not
  double-work.
- Interrupted (token limit, crash)? Unassign and comment "stepping away" — the
  claim dies with the agent that held it; the work must be reclaimable by a
  fresh process.

## Branches & PRs

- Branch names: `fix/oi-27-phantom-99_9`, `feat/slice-b-channelization`,
  `chore/...`, `docs/...`.
- One PR per issue. Open as **Draft** until CI is green, then mark ready.
- The PR template enforces the Constitution gates: **What / Why / Evidence /
  checklist / autonomy-boundary / principle cited**. `Fixes #N` closes the issue
  on merge.
- Squash-merge; branches auto-delete.
- `feature/phase-r-r0-synth` is the operator's free working branch (unprotected);
  durable work flows to `master` via PR.

## CI gates (Actions) — tiered, honest about hardware

The full build needs CUDA + SDRplay + MinGW (Windows); hosted runners have
none of that. So verification is tiered:

- **Tier 1 — hosted (`ubuntu-latest`), every push/PR** (`.github/workflows/ci.yml`):
  `go vet`, tagless `go build ./...`, tagless `go test ./...`. This is the
  **required** check for merging to `master`. (GPU-only tests skip when
  `gpudemod.Available()==false`.) `gofmt` runs as an informational job.
- **Tier 2 — self-hosted Windows + CUDA runner, on demand** (label
  `status:needs-hardware`, or on merge to `master`): full `build-sdrplay.ps1`
  plus the `-tags bench` replay-oracle tests (`TestReal*`,
  `TestStereoPilotLongWindow`) against `data/snapshots/fm_bc.cf32`. This is the
  *real* verification (Constitution IV/V). *(Runner not yet provisioned — see
  Roadmap.)*
- **Tier 3 — operator / `workflow_dispatch`:** live-hardware smoke.

## Branch protection on `master`

- No direct pushes; changes land via PR.
- Tier-1 CI must be green.
- Solo-operator tuning: **no required peer approval** (the operator is the only
  human and cannot approve their own PR), admin merge allowed. The operator's
  merge click *is* the review gate — Constitution VIII enforced by infra, not by
  a second reviewer that does not exist.

## Autonomy boundaries

Agents may act autonomously **up to opening a PR** (not merging) for: bugs with
a clear root cause, build/CI fixes, docs. Agents must **not** autonomously
change: architecture (threads, signal routing, new deps), defaults affecting all
users, UX/visuals, or scope beyond the issue. When in doubt: implement, label
`status:needs-operator`, and flag the decision in the PR.

## gh cheat-sheet

```bash
# Find work that's ready and unclaimed
gh issue list --label status:ready --search "no:assignee" --state open

# Claim, branch, work, ship
gh issue edit <N> --add-assignee @me --add-label status:in-progress
git switch -c fix/oi-<N>-<slug>
# ... implement + verify (run it!) ...
gh pr create --draft --fill --base master   # body follows the PR template
# CI green → mark ready:
gh pr ready <PR>

# Coordinate / hand off
gh issue view <N> --json assignees
gh issue edit <N> --remove-assignee @me   # on interruption
```

## Roadmap (not yet in place)

- Convert the remaining open OIs into Issues + Phase-R **Milestones**.
- Provision the self-hosted Windows+CUDA runner for Tier 2.
- A dedicated PR to make the tree `gofmt`-clean, then promote `gofmt` to a
  required check.
- Optional automation: a `gofmt`/lint auto-fixer, a stale-issue sweep, a
  scheduled "pick next ready issue" nudge.
