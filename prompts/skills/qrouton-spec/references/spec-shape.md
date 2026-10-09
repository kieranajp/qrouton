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

The user answers on the line after `Answer:`, or after it on the same line.
An answer may be a bare letter, a letter followed by a note, or free text that
names no letter. Any non-empty answer counts. When the user answers in chat,
write their answer into that question's `Answer:` block, as they gave it.

## Folding answers

When the user says the answers are in, re-read the spec from disk first. The
user's pane writes answers into the file, so any copy you hold is stale. Then
move each answered question into `## Decisions`:

```markdown
### Q1 — <the question, unchanged>

<the note or free-text answer, if any>

Chosen: <the option taken, and why>
Rejected: A. <option>: <why it lost>
Rejected: C. <option>: <why it lost>
```

Keep the id and the heading text. Remove the options and the `Answer:` line,
and leave unanswered questions where they are. When the last one moves, put
`None.` under `## Open questions`.

Answering questions does not start Plan. The user still asks for it.
