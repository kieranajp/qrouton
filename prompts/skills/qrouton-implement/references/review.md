# Standard review inside Implement

Perform authorized standard review yourself in the orchestrator’s current harness. This is non-adversarial review. Do not delegate, call `start_review`, or launch a second model/process. Read binding repository instructions, the completed plan/spec and recorded verification. Implementation and reference files are read-only; preserve source files and unrelated dirty work. Findings authorize no fixes or publication.

Before inspecting any diff, read the session manifest and resolve every editing repository by owner/name (`org` plus `name`), using a bare name only for an ownerless legacy entry. Resolve its worktree inside the session and verify it is the repository root. Reject duplicate identities, unknown or ambiguous base keys, missing repositories, missing targets and ambiguous refs. Reference repositories provide read-only context.

Pin the committed scope with `git -C <worktree> rev-parse --verify --end-of-options 'HEAD^{commit}'`. Resolve an explicit base with `git -C <worktree> rev-parse --verify --end-of-options '<base>^{commit}'`. When no explicit base is supplied, resolve the manifest base branch (or default branch if absent) the same way, then run `git -C <worktree> merge-base --all <head> <resolved-base>`. Require exactly one merge base; reject none or multiple. Record each owner/name, worktree and resolved base/head before reading `git -C <worktree> diff <base> <head>`. Review the whole-plan committed scope, not only the last phase. A moving HEAD never extends the pinned scope.

Resolve the binding plan and optional spec to existing Markdown files inside the session or its admitted thoughts root. Create `thoughts/shared/reviews/` on demand. Reserve a unique report filename with exclusive creation (for example Python `open(path, "x")`); choose a fresh filename on collision and never overwrite another report. Compute a relative plan path from the report directory and verify it resolves to the binding plan, including thoughts symlinks. Use that same path for metadata and the visible Plan link before writing findings.

Write an explicit pending header immediately:

```yaml
---
kind: review
title: Standard implementation review
plan: ../plans/<binding-plan>.md
review_mode: standard
---
```

Include `[Plan](<relative-plan-path>)`, the pinned scope, and “No completed review has been recorded. This review is pending or interrupted.” Keep incomplete status if interrupted; neither a placeholder nor a process exit is a verdict.

On completion, preserve frontmatter `kind: review`, `title`, relative `plan`, `review_mode: standard`, and the visible Plan link. Lead with material findings or a clean result. State that the orchestrator performed standard non-adversarial review in its current harness. Record actual harness/model/reasoning only when known from reliable context; mark unknown values explicitly. Do not borrow configured independent settings, infer a provider default, or claim an independent perspective. Include pinned base/head commits, binding artifacts, verification evidence, commands/results, unrun checks and practical limits.

Read and open the finished report with `open_file` before communicating its verdict. Report verified implementation completion separately from review interruption. Deliver the local result and stop. Review grants no authority to modify implementation files, apply fixes, push, open a PR, publish, or monitor.

{{review-rubric}}

{{evidence-discipline}}
