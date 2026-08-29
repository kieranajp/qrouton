---
name: qrspi-questions
description: Internally frame the Research part of qrouton's RPI workflow and write an approved research-questions artifact. Use when the work needs investigation but its motivating symptoms, boundaries, or questions are not yet clear.
---

# Frame research

Keep this conversational; the user experiences one Research step.

1. Use the session description and ticket, when present, to understand the motivation.
2. Ask only for missing information that materially changes the investigation: symptoms, trigger, suspected area, boundaries, and desired understanding. Do not grill a user who already supplied enough context.
3. Convert the framing into concrete questions about what exists, how it behaves, and where it connects. Avoid embedding a proposed solution. Each question is one self-contained interrogative line, short enough to read as a heading: no leading numbering (`1.`, `Q1:`), and no context trailing inside the question itself.
4. Add safe context pointers such as repositories, paths, systems, and public documentation. Exclude the ticket URL, ticket text, and solution intent because this entire document is passed to blind researchers.
5. Write `thoughts/shared/research/R<n>-<date>-<slug>-questions.md` with `type: research-questions` frontmatter. Pair its number and slug with the eventual research artifact. Give each question its own `##` heading carrying the question and nothing else, with its context beneath, so the research document can carry those headings verbatim. The file sits under `thoughts/shared/research/`, so the workbench reads it as research and opens it in that accordion — one item per question, nothing pinned; that is intended. Keep the brief under 80 lines: combine overlapping questions and include only context the researchers need.
6. Ask for review only when ambiguity remains or correction would substantially change the research. Otherwise continue into delegated research.
