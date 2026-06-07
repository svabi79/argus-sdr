# Claude Code Project Guide for Argus SDR

> **Claude Code: read [`AGENTS.md`](AGENTS.md) (operational guide) and
> [`CONSTITUTION.md`](CONSTITUTION.md) (the inviolable principles) before
> writing code or recommending changes.**
> This file exists only because Claude Code auto-loads `CLAUDE.md`. The
> project-wide source of truth lives in `AGENTS.md`, so every AI tool reads the
> same guide; the durable principles live in `CONSTITUTION.md`.

## Why this is a thin file

Argus SDR's contribution surface is multi-agent in practice (Claude for
spec/review, Codex for implementation, local models, plus the operator). Each
tool has its own auto-discovered file; to avoid N diverging copies, those files
are thin pointers and everything project-wide lives in `AGENTS.md`, with the
spine in `CONSTITUTION.md`.

| Role | File |
|---|---|
| Claude Code | `CLAUDE.md` (you are here) → points to `AGENTS.md` |
| Canonical operational guide | `AGENTS.md` |
| Inviolable principles | `CONSTITUTION.md` |
| Open engineering issues | `docs/known-issues.md` |

## The short version (read the real files for the rest)

- **GPU-first for DSP** — per-signal shift/filter/decimate/FFT/demod/RDS goes
  through `internal/demod/gpudemod`, not CPU loops (Constitution I, AGENTS §7).
- **Key per-signal state by stable tracker ID, never by frequency** (II).
- **Detected occupied bandwidth ≠ demod channel width** — WFM needs ~250 kHz (III).
- **Measure offline on the replay oracle; the live radio confirms, it does not
  diagnose** (IV).
- **Evidence over assertion** — run it and observe; a passing description is not
  a passing run (V).
- **Stop `sdrd.exe` and the browser UI before build/test sessions** (AGENTS §7).
- **Build/run with the project scripts** (`build-sdrplay.ps1`, `start-sdr.ps1`),
  not ad-hoc `go build`, for full-app validation (AGENTS §7).
- **Commit only intended source** — no `config.autosave.yaml`, no `debug/`
  dumps (IX, AGENTS §5/§6).
- **English for everything that lands in the tree** (X).

Read `AGENTS.md` end-to-end for build/test/branch/debug specifics, and
`CONSTITUTION.md` for the reasoning behind the rules above.
