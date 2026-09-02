---
name: qrspi-research
description: Internally execute the Research part of qrouton's RPI workflow through a delegated research lead and ticket-blind specialists. Use after research questions are sufficiently framed.
---

# Run research

Delegate the investigation; keep its exploratory output out of the orchestrator context.

1. Read the approved research document: it carries the questions as headings with nothing answered under them. Do not include `qrouton.json`, a ticket, or solution framing in the delegated brief.
2. Spawn a `qrouton-research-lead` when available, otherwise a general subagent. Give it only that document's path/content, safe context pointers, the editing/reference repo rules, and the instruction to fill it in where it stands.
3. Instruct the lead to split independent questions among ticket-blind research specialists, wait for them, verify important claims against live code, and synthesize one artifact. It may delegate recursively; it must not ask the orchestrator to carry worker details or paste worker reports into the artifact.
4. Require representative `path:line` evidence for material claims, explicit separation of verified behavior from inference, and contradictions left visible. Do not require a citation on every sentence or an inventory of every matching file or call site. Research describes what is; it does not recommend a solution.
5. Fill in `thoughts/shared/research/R<n>-<date>-<slug>.md` in place, answering each question under the heading that already carries it, with inline evidence instead of a duplicate Code References catalogue. No second artifact is written. The shape is in `references/research-shape.md` beside this file: the heading convention the workbench builds its accordion from, and how a framed question becomes an answered one. Read it and pass it, or its absolute path, to the lead — the lead starts in a fresh context and cannot resolve a path relative to this file.
6. Treat 200 lines as the default ceiling. Exceed it only when the approved questions explicitly request an exhaustive inventory; otherwise compress repeated examples, omit recoverable mechanics, and leave lower-value follow-ups as open questions. Before returning, inspect the finished artifact's length and edit it down if needed.
7. Accept a compact return containing the outcome, artifact path, major findings, and unresolved questions. Present the useful conclusions naturally, then offer to Plan.

Before spawning, inspect the exact brief for ticket or intended-solution leakage.
Sparse, contradictory, or unexpectedly minimal code is still a valid research finding. Complete the delegated investigation and artifacts so the evidence gap is durable; do not replace Research with an informal direct inspection or an implementation proposal.
