---
name: qrspi-plan
description: Internally produce the tactical artifact for the Plan part of qrouton's RPI workflow. Use when the design and scope are sufficiently decided for an implementation lead to execute.
---

# Produce the implementation plan

Delegate plan construction to a `qrouton-planning-lead` when available.

Give the lead the relevant research/spec paths, user decisions, active/reference repo roles, and the required output path. The lead should inspect the live code and may spawn bounded specialists for unfamiliar areas.

The plan must:

- link to its binding research/spec artifacts and identify the workstream;
- state current and desired end states, scope exclusions, and approach;
- use vertical phases that each deliver a coherent increment;
- name concrete files and commands without pretending uncertain line numbers are stable;
- give every phase its own runnable verification;
- follow the plan document shape, so progress is readable from the file itself;
- leave no unresolved decision that blocks implementation;
- stay at or below 400 lines unless the user explicitly asks for an exhaustive runbook; link to research/spec context instead of repeating it, and omit routine mechanics an implementation lead can recover from named files and commands.

The shape is in `references/plan-shape.md` beside this file: the heading convention the workbench reads progress from, and the template to write to. Read it and pass it, or its absolute path, to the lead — the lead starts in a fresh context and cannot resolve a path relative to this file.

Write `thoughts/shared/plans/P<n>-<date>-<slug>.md`. Ask the lead to check the finished artifact's length and compress it before returning only the artifact path, phase outline, verification strategy, and unresolved blockers. Present the phase outline for review when sequencing or scope is consequential; otherwise offer to Implement.
