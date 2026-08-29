# The shape of a plan document

Every second-level heading is one screen when the plan is read in the workbench,
so keep sections whole rather than trailing them off the end of a phase. A phase
is a section whose heading carries the phase prefix; its runnable checks go in a
task list under `### Verify`, and manual observations under `### See`, which is
never the meter. Do not write a phase list of your own — the reader generates one
from the phases.

```markdown
---
kind: plan
title: <title>
---

# <title>

<what this changes and why, a few sentences>

## Decisions
## Out of scope

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
