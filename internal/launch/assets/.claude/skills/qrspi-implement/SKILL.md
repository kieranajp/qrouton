---
name: qrspi-implement
description: Execute the Implement part of qrouton's RPI workflow through a delegated implementation lead that owns plan execution, specialist workers, verification, and progress artifacts. Use when an approved plan or sufficiently concrete request is ready to build.
---

# Delegate implementation

Spawn one `qrspi-implementation-lead` when available, otherwise a capable general implementation agent. Give it:

- the approved plan path or a bounded concrete request;
- active and reference repository roles;
- user decisions not already captured in the artifact;
- the requirement to update durable progress and return a compact result.

The lead owns the implementation context. It should read the plan fully, resume at the first incomplete item, and delegate independent exploration, implementation, tests, or review to specialist subagents where useful. It must coordinate shared-file edits to avoid collisions.

For each plan phase, the lead must implement the vertical increment, run that phase's verification, and update its checkboxes. If code contradicts a binding decision or the plan needs a materially different direction, it returns a blocker for the orchestrator and user instead of forcing the plan.

Require the final return to contain only:

- completed phases and outcome;
- changed repositories/files;
- verification commands and results;
- remaining risks, failures, or decisions;
- plan artifact path and its updated status.

Do not repeat the lead's investigation or ingest its raw logs. Resolve blockers, communicate the concise result, and delegate follow-up verification/review if warranted.
