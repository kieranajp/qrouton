---
name: qrspi-questions
description: Internally frame the Research part of qrouton's RPI workflow and open the research document with its approved questions. Use when the work needs investigation but its motivating symptoms, boundaries, or questions are not yet clear.
---

# Frame research

Keep this conversational; the user experiences one Research step.

1. Use the session description and ticket, when present, to understand the motivation.
2. Ask only for missing information that materially changes the investigation: symptoms, trigger, suspected area, boundaries, and desired understanding. Do not grill a user who already supplied enough context.
3. Convert the framing into concrete questions about what exists, how it behaves, and where it connects. Avoid embedding a proposed solution. Each question is one self-contained interrogative line, short enough to read as a heading: no leading numbering (`1.`, `Q1:`), and no context trailing inside the question itself.
4. Add safe context pointers such as repositories, paths, systems, and public documentation. Exclude the ticket URL, ticket text, and solution intent because this entire document is passed to blind researchers.
5. Write the research document itself at `thoughts/shared/research/R<n>-<date>-<slug>.md`, with the questions as headings and nothing answered under them; the research lead fills the same file in. There is no separate questions artifact. Its shape and template live beside the research skill, at `../qrspi-research/references/research-shape.md`: read that and write to it. Keep the document under 80 lines at this stage: combine overlapping questions and include only context the researchers need.
6. Ask for review only when ambiguity remains or correction would substantially change the research. Otherwise continue into delegated research.
