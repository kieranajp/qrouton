---
name: qrspi-questions
description: QRSPI phase Q — interrogate the user about what's prompting the work, then draft numbered research questions for the ticket-blind research phase. Use when a qrouton session has no docs yet, or the user wants to (re)frame what to research.
---

# QRSPI — Questions

You are running the **Q** phase. Output: a research-questions document that the **R** phase will hand to blind subagents. The questions are the *only* thing research sees, so they must carry all the framing the ticket would have.

## Steps

1. **Read the ticket if there is one.** `qrouton.json` may have a `ticketUrl` — you (the main LLM) may read it now to inform the questions. It does **not** go into the questions doc verbatim and never reaches research.
2. **Grill the user.** Ask what's actually prompting this: the symptom, the trigger, what they suspect, what "done" looks like, which repos/areas are in frame. Push past the first answer — vague framing here produces useless research. Keep going until you could brief a stranger.
3. **Draft the questions.** Turn the conversation into a numbered list of concrete, answerable research questions about *how the system works today* — never "should we…" or "how do we build…". Each should point research at real files/repos/areas. Add a **Key Context Pointers** section: repos, filepaths, libraries, links worth knowing. (Shape: `thoughts/shared/humanlayer/01-research-questions-*.md`.)
4. **Write the doc.** `thoughts/shared/research/R<n>-<YYYY-MM-DD>-<slug>-questions.md`, where `<n>` = (max existing `R` number) + 1 and `<slug>` is a short kebab-case topic. Remember this `<n>` and `<slug>` — the research doc reuses them. Frontmatter: `type: research-questions`.
5. **Sign-off.** Show the questions, ask the user to correct/approve, iterate. When they approve, propose moving to research (`qrspi-research`).

## Guardrails

- Questions describe intent-blind investigation. If a question smells like a solution or leaks "what we're building", rewrite it as "how does X work / where is Y / what happens when Z".
- Don't spawn subagents here — this phase is you and the user.
