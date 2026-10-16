---
name: qrouton-audit
description: Internally run an audit of a repository for four tells (one behaviour or fact expressed more than one way, dead or needlessly defensive code, homegrown code that a well-known library or platform API covers, installed or not, and tests that cannot fail or pass for the wrong reason) through a delegated audit lead. Use when the user asks to audit a repository for these tells.
---

# Run an audit

An audit is Research to the user. It reports and never edits: fixes go through Plan and Implement.

Judge the live code only. The audit forms every finding from the target's code, config and docs. Do not read research, specs, plans or audits under `thoughts/` for it, and do not search them with `search_thoughts` or `read_thoughts`. A stale document must not plant or suppress a finding.

1. Settle the target and the tells with the user before starting, even when the request names a repository. Ask one structured question with two parts:
   - the target: one repository from `qrouton.json`, and optionally paths inside it to include or leave out. Offer the whole repository as the default, because the tells are cross-file, and name its main top-level areas so the user can narrow it in one reply;
   - the tells to run, all of them by default, as a multi-select: `duplicated` for one behaviour or fact expressed more than one way, `dead` for dead or needlessly defensive code, `homegrown` for homegrown code a library or the platform covers, and `volkswagen` for tests that cannot fail or pass for the wrong reason.

   Use the runner's structured question tool when it has one, such as `AskUserQuestion` in Claude Code. Otherwise ask in chat.
2. Call `create_artefact` with `kind: audit` and a short title naming the target. Do not name or write an audit path by hand.

   {{session-summary}} Pass `stage` as `research`.
3. Spawn a `qrouton-audit-lead` when available. Otherwise spawn a general subagent and tell it to read the `qrouton-audit-lead` agent definition stamped in this session and follow it as its own instructions. Brief either one with the audit document's path, the target repository and optional path, the chosen tells by slug, and the `editing`/`reference` repo roles. Pass the absolute path of this skill's `references/` folder: `audit-shape.md` is the document shape, and `typescript.md` and `go.md` are worked examples of choosing, running and distrusting a detector. Add no other path under `thoughts/`, and no plan, spec or research.
4. Accept a compact return: the audit path, the finding count per chosen tell, the tools used, and anything the lead could not judge.
5. Open the audit with `open_file`. Tell the user what it found in a few sentences, and ask them to triage each finding in the pane: Accept, Reject or Defer, each with an optional note. Triage written in chat goes onto the finding's `Triage:` line, in the forms the shape gives.

   {{session-digest}}

Triage does not start Plan. When the user asks to plan the accepted findings, run `qrouton-plan` with the audit's path.

Pass this to the lead, and hold the returned audit to it:

{{artifact-discipline}}
