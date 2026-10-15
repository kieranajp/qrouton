---
name: qrouton-audit
description: Internally run an audit of a repository for three tells (one behaviour or fact expressed more than one way, dead or needlessly defensive code, and homegrown code that a well-known library or platform API covers, installed or not) through a delegated audit lead. Use when the user asks to audit a repository for these tells.
---

# Run an audit

An audit is Research to the user. It reports and never edits: fixes go through Plan and Implement.

Judge the live code only. The audit forms every finding from the target's code, config and docs. Do not read research, specs, plans or audits under `thoughts/` for it, and do not search them with `search_thoughts` or `read_thoughts`. A stale document must not plant or suppress a finding.

1. Settle the target with the user if it is unclear: one repository from `qrouton.json`, and optionally a path inside it. The default is the whole repository, because the tells are cross-file.
2. Call `create_artefact` with `kind: audit` and a short title naming the target. Do not name or write an audit path by hand.

   {{session-summary}} Pass `stage` as `research`.
3. Spawn a `qrouton-audit-lead` when available, otherwise a general subagent. Tell 3 covers code that a well-known library or platform API already provides, installed or not. A general subagent also gets the lead's rules: judge the live code only, never pass `--fix`, and install nothing into the target during the audit run, though a proposed fix may add a dependency. Run reachability in a production mode as well, and record every tool under `## Tools`. Build a job inventory from the repo's own imports, built-ins and call sites, including concerns handled in many places without a shared module, and record each job under `## Tools` with its outcome. Weigh a library's cost against the whole family of homegrown code doing its job. Before returning, run a conformance pass that checks the draft against the rules listed under Before returning in `audit-shape.md`. A deliberate reason counts only when it is written at the site: a comment, a doc, or the commit message for those lines. Report one line per candidate under `## Checked and deliberate`, never a grouped dismissal. Brief it with the audit document's path, the target repository and optional path, and the `editing`/`reference` repo roles. Pass the absolute path of this skill's `references/` folder: `audit-shape.md` is the document shape, and `typescript.md` and `go.md` are worked examples of choosing, running and distrusting a detector. Add no other path under `thoughts/`, and no plan, spec or research.
4. Accept a compact return: the audit path, the finding count per tell, the tools used, and anything the lead could not judge.
5. Open the audit with `open_file`. Tell the user what it found in a few sentences, and ask them to triage each finding in the pane: Accept, Reject or Defer, each with an optional note. Triage written in chat goes onto the finding's `Triage:` line, in the forms the shape gives.

   {{session-digest}}

Triage does not start Plan. When the user asks to plan the accepted findings, run `qrouton-plan` with the audit's path.

Pass this to the lead, and hold the returned audit to it:

{{artifact-discipline}}
