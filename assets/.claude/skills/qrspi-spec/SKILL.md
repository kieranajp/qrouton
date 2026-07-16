---
name: qrspi-spec
description: QRSPI phase S — turn a research doc plus user discussion into a spec that records design decisions, alternatives not chosen, scope, and open questions. First half of the SP macro-phase. Use when a session has a research doc but no spec.
---

# QRSPI — Spec

You are running **S**, the first half of the SP macro-phase. Output: a spec that fixes the *design decisions* — the binding "what and why", before any tactical "how". Keep it tight: **~200 lines max** (instruction-budget discipline — a spec no one finishes reading decides nothing).

## Steps

1. **Read the research doc** fully. It is the ground truth for what exists.
2. **Discuss the design with the user.** Surface the real decisions and their trade-offs; get calls made. Be skeptical — question vague requirements, name the alternatives, don't let decisions stay implicit.
3. **Write the spec** to `thoughts/shared/specs/S<n>-<YYYY-MM-DD>-<slug>.md`, `<n>` = (max existing `S` number) + 1. Match the shape of `thoughts/shared/specs/S002-*.md`:
   - Frontmatter: `date`, `author`, `topic`, `tags`, `status: draft`, `related_research` (path to the research doc).
   - **Desired end state** — concrete, verifiable.
   - **Resolved design decisions** — numbered; each states the choice, the why, and the alternative(s) *not* chosen with the reason.
   - **Scope** — In / Out, explicitly.
   - **Open questions** — anything unresolved, tagged with when it gets decided (defer freely; don't invent answers).
4. **Review gate.** Present the spec and **pause for human review**. Do not roll straight into the plan — the SP macro-phase has a deliberate checkpoint here. When the user approves, propose `qrspi-plan`.

## Guardrails

- Decisions, not implementation. File-by-file steps belong in the plan, not here.
- Every non-chosen path gets a one-line reason — a decision without its discarded alternative is an assertion.
