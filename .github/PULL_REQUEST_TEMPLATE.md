<!--
Read CONSTITUTION.md and AGENTS.md before opening this PR.
Keep the PR to one issue / one logical change. Open as a Draft until CI is green.
-->

## What

<!-- One or two sentences: what this change does. -->

Fixes #

## Why

<!-- The problem this solves. Link the issue / known-issue (OI-NN) it addresses. -->

## Evidence (Constitution V — Evidence Over Assertion)

<!--
A fix is verified by RUNNING it, not by describing it. Paste the actual proof:
the command(s) you ran and their output / the live API state / a screenshot /
the new or existing test that exercises the path. A claim whose verification
step was not actually run is a hypothesis — say so if that's the case.
-->

```
# command(s) + output here
```

## Checklist

- [ ] Read CONSTITUTION.md + AGENTS.md; this change serves the project direction (Principle VI)
- [ ] Per-signal state is keyed by stable tracker ID, not frequency (Principle II) — or N/A
- [ ] DSP that can run on the GPU does (Principle I) — or N/A
- [ ] Verified by running it; evidence pasted above (Principle V)
- [ ] Only intended source committed — no `config.autosave.yaml`, no `debug/` dumps (Principle IX)
- [ ] English for everything that lands in the tree (Principle X)
- [ ] Updated `docs/known-issues.md` / docs if an open issue changed (AGENTS §12)

## Autonomy boundary (Constitution VI)

<!--
If this touches architecture, a default that affects all users, UX/visuals, or
scope beyond the issue: STOP and flag for the operator. Label the PR
`needs-operator` and describe the decision needed below. Otherwise write "none".
-->

## Principle cited

<!-- The most load-bearing principle, e.g. "Principle II." — also goes in the commit subject. -->
