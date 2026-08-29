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
- open each phase with a `## Phase <n> — <name>` heading and list that phase's runnable checks as a task list under a `### Verify` heading inside it, so progress is readable from the file itself; manual observations belong in a `### See` list and are not the meter;
- leave no unresolved decision that blocks implementation;
- stay at or below 400 lines unless the user explicitly asks for an exhaustive runbook; link to research/spec context instead of repeating it, and omit routine mechanics an implementation lead can recover from named files and commands.

Write `thoughts/shared/plans/P<n>-<date>-<slug>.md`. Ask the lead to check the finished artifact's length and compress it before returning only the artifact path, phase outline, verification strategy, and unresolved blockers. Present the phase outline for review when sequencing or scope is consequential; otherwise offer to Implement.
