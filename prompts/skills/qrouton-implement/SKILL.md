---
name: qrouton-implement
description: Execute the Implement part of qrouton's workflow through a delegated implementation lead that owns plan execution, specialist workers, verification, and progress artifacts. Use when an approved plan is ready to build, or the user explicitly consents to implementing a bounded concrete request without a tactical plan.
---

# Delegate implementation

Enter implementation only when the user explicitly asked to build, fix, finish, implement, or resume the work, or unambiguously accepted an identified offer to proceed with it. A plan's presence, incomplete checkboxes, approval-like wording attached to feedback, or completion of the prior stage is not authorization. Keep using authorization already given in the current conversation; do not request it again for ordinary continuation or resumed work. Strongly discourage implementation without a tactical plan. A generic implementation request does not consent to that exception. Before delegating a request without a tactical plan, obtain explicit user consent to proceed without one unless the user already gave it in the current conversation. Recommend a tactical plan when asking for that consent.

Spawn one `qrouton-implementation-lead` when available, otherwise a capable general implementation agent.

{{session-summary}} Pass `stage` as `implement`.

Give the lead:

- the approved plan path, or a bounded concrete request with the user’s explicit consent to proceed without a tactical plan;
- whether the user authorized the first incomplete phase or the whole plan;
- `editing` and `reference` repository roles;
- user decisions not already captured in the artifact;
- the requirement to update durable progress and return a compact result.

The lead owns the implementation context. For a supplied plan, it reads the plan fully and resumes at the first incomplete item. For a bounded concrete request without a tactical plan, the brief must record explicit user consent to that exception; otherwise the lead returns missing consent to the orchestrator without asking the user itself. With that consent, it inspects the files needed to implement and verify the task. It delegates independent exploration, implementation, tests, or review to specialist subagents where useful. It must coordinate shared-file edits to avoid collisions.

An ordinary request to implement a multi-phase plan authorizes only its first incomplete phase. Requests to complete the whole plan, all phases, or run it in one shot authorize every phase. A bounded concrete request without a multi-phase plan is one task; if it has no tactical plan, require explicit user consent to that exception. After consent, complete the bounded task without invented phase gates. Preserve authorization for focused fixes and reverification within the current phase.

For each authorized plan phase, the lead must implement the vertical increment, run that phase's verification, update its checkboxes, and commit the verified work. A phase-only run returns after that phase. The orchestrator presents its outcome and waits for explicit authorization before spawning or resuming a lead for the next phase. If code contradicts a binding decision or the plan needs a materially different direction, the lead returns a blocker for the orchestrator and user instead of forcing the plan.

Require this of the lead's return:

{{return-contract}}

- what happened: the completed phases and the outcome of each;
- where the work lives: the changed repositories and files, and the plan path with its updated status;
- how you checked it: the verification commands and their results;
- what stays unresolved: the remaining risks, failures, or decisions.

Do not repeat the lead's investigation or ingest its raw logs. Resolve blockers, communicate the concise result, and delegate follow-up verification/review if warranted.

`qrouton-implement` is the last skill in the workflow. Say whether one phase or the whole plan completed and report any remaining phases. After a phase-only or partial completion, stop without a whole-plan independent-review offer. Do not resume the lead until the user authorizes more work.

Only after verified whole-plan completion, call `reviewer_settings`. None produces no independent-review offer or launch; end the workflow. If the user explicitly requested review while disabled, explain the Settings requirement; select no harness and create no request, report, or tab. An enabled unavailable selection produces its explicit availability error without substitution.

For an enabled available reviewer, offer optional independent review. Whole-plan implementation alone authorizes the offer, not the launch. Declining ends without creating anything. A prior explicit request to implement the whole plan and review it authorizes `start_review` without another permission question. Otherwise launch only after explicit acceptance. Supply the completed plan, relevant spec, editing repositories' base refs when explicit, completed scope, pinned commits, and recorded verification evidence. The capability rechecks Settings, so disabling after an offer prevents launch.

Keep the implementation lead's independent final pass regardless of reviewer configuration. The fresh reviewer writes local `kind: review` thoughts linked to the plan and delivers them through `open_file`. Read the report before describing a verdict. A request acknowledgement, process exit, or status mark never proves review completion. After an uncertain open, inspect its stable tab/report paths without retrying. Report completed implementation separately from failed or interrupted review. Findings authorize no fixes, pushes, PR creation, publication, or monitoring; those require a separate explicit request.
