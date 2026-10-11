## Review scope and finding bar

Read binding repository instructions and the supplied plan/spec. Establish the supplied diff boundary, including staged or unstaged changes when requested. When the request pins base/head commits, review committed changes at those commits; a moved live HEAD does not extend that scope. Preserve dirty files. Reference repositories provide context.

Report concrete failing scenarios or credible maintenance harm with a proportionate remedy. Separate introduced defects from existing debt. Give each material finding a severity, precise location, impact, evidence, and remedy. State uncertainty; verify a suspicious behavior with a focused check when feasible. Combine findings with the same root cause. Do not invent a quota or reopen a settled decision without contrary evidence.

Use P0 for critical failures requiring an immediate stop; P1 for high-impact merge blockers; P2 for material actionable defects, stating whether they block or may be deferred; P3 for minor issues only when they materially affect the verdict. Accept reasoned deferrals and say when no material findings remain.

## Correctness

Trace behavior across boundaries, error paths, concurrency, cancellation, loading, retries, and lifecycle teardown. Check data loss, security, compatibility, and actual caller assumptions. Explain the trigger and consequence of a defect.

## Architecture and maintenance

Check responsibilities, dependency direction, state ownership, and the cost of adding the next similar behavior. Apply SOLID and DRY to concrete coupling or repeated facts. Avoid abstractions justified only by superficial similarity. Check stale comments, docs, dependencies, exports, and generated consumers.

## Refactor completeness

Follow callers and all renamed or moved consumers. Look for dead paths, orphaned configuration, outdated generators, tests, and documentation. Explain any remaining contract that no longer matches its implementation.

## Plan alignment

Check intended behavior, completed authorized scope, and binding decisions against the pinned changes. Preserve settled choices. Name unfinished scope and contradictions; checked plan boxes alone are not verification or a review verdict.

## UI and platform behavior

Check the supported platform's layout, input, scrolling, accessibility, large text, reduced motion, and RTL where relevant. Distinguish code reasoning, fixture tests, screenshots, and observed native behavior. Never claim an interaction was observed from a unit test.

## Evidence and delivery

Use recorded verification for the pinned head and current-head CI when a PR exists. Run targeted checks for suspected defects or stale/missing evidence. Explicit repository verification requirements take precedence. Record commands, results, unrun checks, and practical limits. Do not infer a verdict from a process exit.

A re-review needs a new explicit request. Record the prior report and review its pinned-head delta plus affected surrounding behavior. Do not reopen settled findings or begin another monitoring loop.

Review findings authorize no fixes, pushes, PR creation, publication, or monitoring. Deliver the local result and stop.
