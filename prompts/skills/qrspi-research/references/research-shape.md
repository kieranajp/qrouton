# The shape of a research document

Every second-level heading is one item in the workbench accordion, labelled with
its heading text, so an approved question becomes a `##` heading carrying that
question verbatim and the findings that answer it sit under it. A first section
named `Summary` — that word exactly, case aside — is pinned above the accordion
and always visible, so write it to be read on its own. Anything else that earns a
place is a trailing item rather than a special case; a closing `## Open Questions`
is simply the last one.

```markdown
---
kind: research
title: <title>
---

# <title>

<what was investigated and against what, a few sentences>

## Summary

## <an approved question, verbatim>

## Open Questions
```

One item per approved question, in the order they were asked. Sections beyond
those are yours to choose; the ones above are the ones we expect.
