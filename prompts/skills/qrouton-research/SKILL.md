---
name: qrouton-research
description: Internally execute the Research part of qrouton's workflow through a delegated research lead and ticket-blind specialists. Use after research questions are sufficiently framed.
---

# Run research

Delegate the investigation; keep its exploratory output out of the orchestrator context.

1. Read the approved research document: it carries the questions as headings with nothing answered under them. Do not include `qrouton.json`, a ticket, or solution framing in the delegated brief.
2. When `search_vault` is available, search the vault for each approved question before delegating, always with `kinds: ["research"]`. Only research artifacts may reach the lead: specs, plans and notes carry intended solutions, whatever their workstream or ticket. The scores are rank evidence, not relevance, so read a hit with `read_vault` and keep it only when it bears on the question. Pass a reference, not content: the artifact ID, heading breadcrumb, line range, and the approved question it bears on. Add no summary of your own and no reason why the hit matters to the ticket. No hits, an unavailable channel and a list of irrelevant neighbours all mean the same thing: pass nothing and let the lead investigate fresh.
3. Spawn a `qrouton-research-lead` when available, otherwise a general subagent. Give it only that document's path/content, the vault references you kept, safe context pointers, the active/reference repo rules, and the instruction to fill it in where it stands.
4. Instruct the lead to split independent questions among ticket-blind research specialists, wait for them, verify important claims against live code, and synthesize one artifact. It may delegate recursively; it must not ask the orchestrator to carry worker details or paste worker reports into the artifact.
5. Research describes what is; it does not recommend a solution.
6. Fill in `thoughts/shared/research/R<n>-<date>-<slug>.md` in place, answering each question under the heading that already carries it, with inline evidence instead of a duplicate Code References catalogue. No second artifact is written. The shape is in `references/research-shape.md` beside this file: the heading convention the workbench builds its accordion from, and how a framed question becomes an answered one. Read it and pass it, or its absolute path, to the lead — the lead starts in a fresh context and cannot resolve a path relative to this file.
7. Accept a compact return containing the outcome, artifact path, major findings, and unresolved questions. Present the useful conclusions naturally, then offer to Plan.

Before spawning, inspect the exact brief for ticket or intended-solution leakage.
Sparse, contradictory, or unexpectedly minimal code is still a valid research finding. Complete the delegated investigation and artifacts so the evidence gap is durable; do not replace Research with an informal direct inspection or an implementation proposal.

Continue only when the user asks for it. Run `qrouton-spec` next when a material choice stays open. Run `qrouton-plan` next when the choices are settled.

Pass this to the lead and its specialists:

{{evidence-discipline}}

A citation on every sentence is not the goal; a citation on every material claim is.

Pass this to the lead, and hold the returned artifact to it:

{{artifact-discipline}}
