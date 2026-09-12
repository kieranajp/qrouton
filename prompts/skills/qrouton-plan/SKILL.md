---
name: qrouton-plan
description: Internally produce the tactical artifact for the Plan part of qrouton's workflow. Use when the design and scope are sufficiently decided for an implementation lead to execute.
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
- leave no unresolved decision that blocks implementation.

The shape is in `references/plan-shape.md` beside this file: the heading convention the workbench reads progress from, and the template to write to. Read it and pass it, or its absolute path, to the lead — the lead starts in a fresh context and cannot resolve a path relative to this file.

Write `thoughts/shared/plans/P<n>-<date>-<slug>.md`.

Pass this to the lead, and hold the returned plan to it:

{{artifact-discipline}}

Require this of the lead's return:

{{return-contract}}

- what happened: the phase outline;
- where the work lives: the plan artifact path;
- how you checked it: the verification each phase carries;
- what stays unresolved: the blockers the lead could not resolve.

Present the phase outline for review when sequencing or scope is consequential; otherwise offer to Implement. Plan feedback authorizes revisions to the plan, not implementation. Positive wording such as “looks good” does not change that when the same request asks for a revision. After revising the plan, wait for an explicit implementation request.

Run `qrouton-implement` next when the user explicitly asks to implement the plan or unambiguously accepts your offer to proceed with it.
