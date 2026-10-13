# The shape of a plan document

Every second-level heading is one screen when the plan is read in the workbench,
so keep sections whole rather than trailing them off the end of a phase, and put
anything that earns a place beside another section rather than a screen of its own
under it as a third-level heading. A phase is a section whose heading carries the
phase prefix; its runnable checks go in a task list under `### Verify`, and manual
observations under `### See`, which is never the meter. Do not write a phase list
of your own — the reader generates one from the phases.

`create_artefact` writes the skeleton from `plan-template.md` beside this file.
Its `## Phase 1 — <name>` section shows a phase's shape: a prose lead, `### Verify`
holding checkbox commands, and `### See` holding checkbox observations. Add one
such section per phase, and keep `## Decisions`, `### Out of scope`, `## Closing checklist`
and `## Blockers`.

Sections other than the phases are yours to choose; the ones above are the ones
we expect.

## Where a phase ends

A phase ends where the user must act, such as relabelling data, signing off or
checking on a device, or at a clean code boundary someone could ship. Work one
implementation lead would do in one go stays one phase, however many files it
touches. Operational chores, such as moving files, promotion steps and cleanup,
go in the closing checklist, not in a phase of their own.

To add work mid-flight, insert a suffixed phase such as `Phase 5b`. Never
renumber existing phases.

## How a plan reads

Assume the lowest common denominator: that the plan will be read mid-task by a heavily-multitasking reader who slept badly and has ADHD. Write for them.
They skim, lose the thread, and re-enter in the middle of a section. So each
sentence has to land on one pass, and each section has to make sense read cold.

Put each phase's per-file work in a table, one row per file, stating what that
file ends up doing. A run of file edits written as prose is easy to lose your
place in. The prose around the table then carries only what a table cannot: why
an edit is not the obvious one, and what breaks if someone skips it.
