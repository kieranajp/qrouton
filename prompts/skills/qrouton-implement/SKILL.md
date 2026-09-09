---
name: qrouton-implement
description: Execute the Implement part of qrouton's workflow through a delegated implementation lead that owns plan execution, specialist workers, verification, and progress artifacts. Use when an approved plan or sufficiently concrete request is ready to build.
---

# Delegate implementation

Enter implementation only when the user explicitly asked to build, fix, finish, implement, or resume the work, or unambiguously accepted an identified offer to proceed with it. A plan's presence, incomplete checkboxes, approval-like wording attached to feedback, or completion of the prior stage is not authorization. Keep using authorization already given in the current conversation; do not request it again for ordinary continuation or resumed work.

Spawn one `qrouton-implementation-lead` when available, otherwise a capable general implementation agent. Give it:

- the approved plan path or a bounded concrete request;
- whether the user authorized the first incomplete phase or the whole plan;
- active and reference repository roles;
- user decisions not already captured in the artifact;
- the requirement to update durable progress and return a compact result.

The lead owns the implementation context. It should read the plan fully, resume at the first incomplete item, and delegate independent exploration, implementation, tests, or review to specialist subagents where useful. It must coordinate shared-file edits to avoid collisions.

An ordinary request to implement a multi-phase plan authorizes only its first incomplete phase. Requests to complete the whole plan, all phases, or run it in one shot authorize every phase. A bounded concrete request without a multi-phase plan is one authorized task. Preserve authorization for focused fixes and reverification within the current phase.

For each authorized plan phase, the lead must implement the vertical increment, run that phase's verification, update its checkboxes, and commit the verified work. A phase-only run returns after that phase. The orchestrator presents its outcome and waits for explicit authorization before spawning or resuming a lead for the next phase. If code contradicts a binding decision or the plan needs a materially different direction, the lead returns a blocker for the orchestrator and user instead of forcing the plan.

Require this of the lead's return:

{{return-contract}}

- what happened: the completed phases and the outcome of each;
- where the work lives: the changed repositories and files, and the plan path with its updated status;
- how you checked it: the verification commands and their results;
- what stays unresolved: the remaining risks, failures, or decisions.

Do not repeat the lead's investigation or ingest its raw logs. Resolve blockers, communicate the concise result, and delegate follow-up verification/review if warranted.

`qrouton-implement` is the last skill in the workflow. Say whether one phase or the whole plan completed, report the remaining phases, and stop. Do not resume the lead until the user authorizes more work.
