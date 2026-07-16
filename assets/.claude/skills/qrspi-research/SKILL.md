---
name: qrspi-research
description: QRSPI phase R — spawn ticket-blind subagents to document how the system works today, using only the approved questions doc, then synthesise a research document. Use when a qrouton session has an approved *-questions.md but no research doc yet.
---

# QRSPI — Research

You are running the **R** phase. Output: a research document capturing *what is* — no proposals, no critique, no "what we should build".

## Ticket-blind rule (load-bearing — do not violate)

Research subagents receive **only the content of the questions document**. Never pass them the `ticketUrl`, the ticket's contents, or any "what we're building" framing. Hiding is by construction: give them the questions, nothing else. If an agent asks what the work is for, it doesn't need to know.

## Steps

1. **Read the questions doc** fully (`thoughts/shared/research/R<n>-…-questions.md`). Reuse its `<n>` and `<slug>` for the research doc — do not allocate a new number.
2. **Spawn parallel subagents**, one focused brief per question or cluster of questions. Each brief contains only that question and the relevant Key Context Pointers.
   - Prefer onetech worker agents when loaded: `codebase-locator` (where things live), `codebase-analyzer` (how they work), `codebase-pattern-finder` (examples), `thoughts-locator`/`thoughts-analyzer` (prior docs). Prefer the ticket-blind `qrspi-researcher` agent for general read-only investigation.
   - If none are available, spawn plain general-purpose subagents with a read-only, documentarian brief.
3. **Wait for all**, then synthesise. Live code is the source of truth; prior thoughts docs are supplementary. Include concrete `path:line` references. Note contradictions rather than papering over them.
4. **Write the doc** to `thoughts/shared/research/R<n>-<YYYY-MM-DD>-<slug>.md` (same `<n>`/`<slug>` as the questions). Match the frontmatter + section shape of existing research docs (see `thoughts/shared/research/R1-*.md`): `date`, `researcher` (git user), `git_commit`/`branch`/`repository` (per repo, or N/A), `topic`, `tags`, `status`. Sections: Research Question, Summary, Detailed Findings, Code References, Open Questions.
5. **Present** a concise summary + key references. Propose moving to the spec (`qrspi-spec`).

## Guardrails

- Documentarian only: describe what exists and how it connects. No recommendations, no root-cause, no refactoring ideas.
- Grep your own subagent prompts before sending — if a ticket URL or intent snuck in, strip it.
