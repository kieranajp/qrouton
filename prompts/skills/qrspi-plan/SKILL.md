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
- leave no unresolved decision that blocks implementation.

Write `thoughts/shared/plans/P<n>-<date>-<slug>.md`. Ask the lead to return only the artifact path, phase outline, verification strategy, and unresolved blockers. Present the phase outline for review when sequencing or scope is consequential; otherwise offer to Implement.
