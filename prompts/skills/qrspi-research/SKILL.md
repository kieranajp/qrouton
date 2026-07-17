---
name: qrspi-research
description: Internally execute the Research part of qrouton's RPI workflow through a delegated research lead and ticket-blind specialists. Use after research questions are sufficiently framed.
---

# Run research

Delegate the investigation; keep its exploratory output out of the orchestrator context.

1. Read the approved `*-questions.md`. Do not include `qrouton.json`, a ticket, or solution framing in the delegated brief.
2. Spawn a `qrspi-research-lead` when available, otherwise a general subagent. Give it only the questions artifact path/content, safe context pointers, the active/reference repo rules, and the required output path.
3. Instruct the lead to split independent questions among ticket-blind research specialists, wait for them, verify important claims against live code, and synthesize one artifact. It may delegate recursively; it must not ask the orchestrator to carry worker details.
4. Require `path:line` evidence, explicit separation of verified behavior from inference, and contradictions left visible. Research describes what is; it does not recommend a solution.
5. Write `thoughts/shared/research/R<n>-<date>-<slug>.md` using its questions pair's number and slug. Include: Research Question, Summary, Detailed Findings, Code References, and Open Questions, plus existing research frontmatter conventions.
6. Accept a compact return containing the outcome, artifact path, major findings, and unresolved questions. Present the useful conclusions naturally, then offer to Plan.

Before spawning, inspect the exact brief for ticket or intended-solution leakage.
Sparse, contradictory, or unexpectedly minimal code is still a valid research finding. Complete the delegated investigation and artifacts so the evidence gap is durable; do not replace Research with an informal direct inspection or an implementation proposal.
