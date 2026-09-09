# The shape of a plan document

Every second-level heading is one screen when the plan is read in the workbench,
so keep sections whole rather than trailing them off the end of a phase, and put
anything that earns a place beside another section rather than a screen of its own
under it as a third-level heading. A phase is a section whose heading carries the
phase prefix; its runnable checks go in a task list under `### Verify`, and manual
observations under `### See`, which is never the meter. Do not write a phase list
of your own — the reader generates one from the phases.

```markdown
---
kind: plan
title: <title>
---

# <title>

<what this changes and why, a few sentences>

## Decisions

### Out of scope

## Phase 1 — <name>

<what this phase does>

### Verify
- [ ] <a command that passes or fails>

### See
- [ ] <something a human confirms; not the meter>

## Blockers
```

Sections other than the phases are yours to choose; the ones above are the ones
we expect.

## How a plan reads

Assume the lowest common denominator: that the plan will be read mid-task by a heavily-multitasking reader who slept badly and has ADHD. Write for them.
They skim, lose the thread, and re-enter in the middle of a section. So each
sentence has to land on one pass, and each section has to make sense read cold.

Put each phase's per-file work in a table, one row per file, stating what that
file ends up doing. A run of file edits written as prose is easy to lose your
place in. The prose around the table then carries only what a table cannot: why
an edit is not the obvious one, and what breaks if someone skips it.
