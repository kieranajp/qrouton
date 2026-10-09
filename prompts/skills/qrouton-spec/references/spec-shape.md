# The shape of a spec document

The workbench reads a spec as a set of questions the user answers in place.
It finds them by heading, so keep the headings exactly as below. Every
second-level heading is one screen in the reader.

```markdown
---
kind: spec
title: <title>
research: <relative path to the research document>
---

# <title>

## End state

<what is true when the work is done, a few sentences>

## Scope

**In:** <what this work covers>

**Out:** <what it leaves alone>

## Open questions

### Q1 — <the question>

<the context the user needs to answer it: what depends on it, what the evidence says>

- A. <option>
- B. <option> **(recommended: <why this one>)**
- C. <option>

Answer:

## Decisions

**<the decision, in one sentence>.** <why, with evidence>
Rejected: <the alternative and why it lost>

## <any other section the work needs>
```

## Questions

A question goes in only when a wrong guess means rework. Routine
implementation detail, anything the research already settles, and anything
cheap to change later are decisions you make and record under `## Decisions`.
Most specs carry few questions. Many carry none.

Keep `## Open questions` even when nothing is open, with `None.` under it. The
workbench uses the heading to show the spec as answerable.

Each open question has:

- a `### Q<n> — <question>` heading. The id never changes once written;
- a context paragraph;
- lettered options as list items, `- A. <text>`;
- exactly one option marked `**(recommended: <reason>)**` at its end;
- an empty `Answer:` line, last in the block.

An option letter counts as picked only when the `Answer:` line holds that one
letter and nothing else. Never infer a letter from prose.

```markdown
Answer: B
but only if the deadline is per caller
```

That is a pick of B with a note, which runs on the following lines up to the
next heading. Anything else on the `Answer:` line is free text with no letter:
`Answer: B, but only if X`, `Answer: B (I think so)`, `Answer: yes`. Free text
may continue onto later lines, and it counts as answered with no option
selected. A note with no letter goes on the `Answer:` line itself, or, when its
first line is a single letter, on the lines after an empty `Answer:`.

When the user answers in chat, write their answer into that question's
`Answer:` block in these forms: the letter alone on the line with any note
beneath it, or their words as given with no letter. If they say "B, but only if
X", record `Answer: B, but only if X`, not `Answer: B`.

## Folding answers

When the user says the answers are in, re-read the spec from disk first. The
user's pane writes answers into the file, so any copy you hold is stale. Then
move each answered question into `## Decisions`:

```markdown
### Q1 — <the question, unchanged>

<the note or free-text answer, if any>

Chosen: <the option taken, and why. Carry the note or free text verbatim>
Rejected: A. <option>: <why it lost>
Rejected: C. <option>: <why it lost>
```

Never reduce a free-text answer to a letter: "B, but only if X" is decided as
that, caveat included. Keep the id and the heading text. Remove the options and
the `Answer:` line, and leave unanswered questions where they are. When the
last one moves, put `None.` under `## Open questions`.

Answering questions does not start Plan. The user still asks for it.
