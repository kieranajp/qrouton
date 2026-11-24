# The shape of a todo document

The workbench reads a todo document as a checklist the user works through in
its pane. It counts open items and calls out blocking ones, so keep to the
shape below.

`create_artefact` writes the skeleton from `todo-template.md` beside this file.
Replace the example section and item with real ones.

## Sections and order

Group items under `##` sections, in the order the user meets them, such as
"Before the first deploy", "Ship" and "Clean up". Items within a section run
top to bottom. Order is the only dependency: when an item can run alongside
the ones above it, say so in prose under it.

## Items

Each item is a task-list item on one line, a single action the user takes:

- `- [ ]` open: still to do.
- `- [x]` done.
- `- [-]` not needed: it was dropped, such as a rollback that never had to run.

Only `[ ]` counts as open. Never delete an item that turned out unnecessary;
mark it `[-]` so the record stays.

## Fields

An item may carry up to four fields, each on its own indented `- Key: value`
line. Spell the keys exactly:

- `Where:` the target, such as a repo, a pull request link or an environment.
  Write a pull request as its full URL or `org/repo#123`, so the pane can show
  its state.
- `Check:` how to confirm it is done, in prose or a command in backticks.
- `Run:` a command that does it, in backticks. The pane offers a copy button
  and never runs it.
- `Blocks:` what waits on it, such as `Phase 3` or `the agent`. An open item
  with `Blocks:` is a blocking item, and the workbench calls it out.

Any other line under an item is prose.

## Done notes

When an item is ticked, add a short outcome after a spaced em dash on the item
line, such as `- [x] Merge the migration PR — live as v2.94.0`. The pane does
the same when the user adds a note. Avoid a spaced em dash in an item's own
text, because the last one starts the note.

## Example

```markdown
## Before the first deploy

- [x] Apply the tenant secrets — applied 2026-11-06
  - Where: enclave · production
- [ ] Merge the schema migration
  - Where: https://github.com/acme/api/pull/4789
  - Check: `select count(*) from tenants` returns rows
  - Blocks: Phase 3

## Clean up

- [-] Roll back the cache — not needed, the new reader shipped clean
- [ ] Remove the old queue after seven days with no publishes
  - Run: `make remove-queue ENV=production`
```
