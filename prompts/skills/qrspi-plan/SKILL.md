---
name: qrspi-plan
description: Internally produce the tactical artifact for the Plan part of qrouton's RPI workflow. Use when the design and scope are sufficiently decided for an implementation lead to execute.
---

# Produce the implementation plan

Delegate plan construction to a `qrspi-planning-lead` when available.

Give the lead the relevant research/spec paths, user decisions, active/reference repo roles, and the required output path. The lead should inspect the live code and may spawn bounded specialists for unfamiliar areas.

The plan must:

- link to its binding research/spec artifacts and identify the workstream;
- state current and desired end states, scope exclusions, and approach;
- use vertical phases that each deliver a coherent increment;
- name concrete files and commands without pretending uncertain line numbers are stable;
- give every phase its own runnable verification;
- follow the shape below, so progress is readable from the file itself;
- leave no unresolved decision that blocks implementation;
- stay at or below 400 lines unless the user explicitly asks for an exhaustive runbook; link to research/spec context instead of repeating it, and omit routine mechanics an implementation lead can recover from named files and commands.

Every second-level heading is one screen when the plan is read in the workbench,
so keep sections whole rather than trailing them off the end of a phase. A phase
is a section whose heading carries the phase prefix; its runnable checks go in a
task list under `### Verify`, and manual observations under `### See`, which is
never the meter. Do not write a phase list of your own — the reader generates one
from the phases.

```markdown
---
kind: plan
title: <title>
---

# <title>

<what this changes and why, a few sentences>

## Decisions
## Out of scope

## Phase 1 — <name>

<what this phase does>

### Verify
- [ ] <a command that passes or fails>

### See
- [ ] <something a human confirms; not the meter>

## Blockers
```

Sections other than the phases are yours to choose; the ones above are the ones
we expect. Write `thoughts/shared/plans/P<n>-<date>-<slug>.md`. Ask the lead to check the finished artifact's length and compress it before returning only the artifact path, phase outline, verification strategy, and unresolved blockers. Present the phase outline for review when sequencing or scope is consequential; otherwise offer to Implement.
