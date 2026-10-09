---
name: qrouton-spec
description: Internally capture design decisions during the Plan part of qrouton's workflow, in a spec document the user answers in place. Use when research is answered and the work is not small enough, or explicit enough, to plan directly.
---

# Align the design

The user experiences this as design questions within Plan, never a stage of its own.

1. Read the relevant research and follow its workstream links.
2. Have a `qrouton-planning-lead` draft `thoughts/shared/specs/S<n>-<date>-<slug>.md`. It inspects the live code, states the end state and scope, records the decisions it can make with their rejected alternatives, and writes each remaining open question into the file with lettered options and a recommendation. The shape is in `references/spec-shape.md` beside this file, including the bar a question must clear. Read it and pass its absolute path to the lead, which starts in a fresh context and cannot resolve a path relative to this file.
3. Open the spec with `open_file`. Tell the user what it decides and how many design questions wait in it, and that they answer them in the document. Do not walk through the options in chat.
4. If the user answers in chat instead, write each answer into that question's answer block.
5. When the user says the answers are in, or their pane types that line into the conversation, re-read the spec from disk and have the lead fold each answered question into `## Decisions` in the shape the reference gives. Open the spec again and summarise what changed.

A spec with no open questions is complete as written. Present it and offer to Plan.

The spec records what and why. File-by-file execution belongs in the tactical plan. Answering questions does not authorise Plan; run `qrouton-plan` only when the user asks for it.

Pass this to the lead, and hold the returned spec to it:

{{artifact-discipline}}
