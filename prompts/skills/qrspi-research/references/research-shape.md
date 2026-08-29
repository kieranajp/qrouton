# The shape of a research document

A research workstream has one document. Framing writes it from the template
below, carrying the approved questions as headings with nothing answered under
them; the research lead then fills it in where it stands. A questions brief is
simply this document before anyone has answered it.

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

## Summary

<framing: what is being investigated, and the repositories, paths and systems
to look at. Rewritten as the summary of findings.>

## <an approved question, verbatim>

> <framing: the context a researcher needs for this question. Replaced by the
> finding that answers it.>

## Open Questions
```

One item per approved question, in the order they were asked. Sections beyond
those are yours to choose; the ones above are the ones we expect.

Answering consumes the framing rather than sitting beside it: each question's
blockquote is replaced by the finding, and the Summary is rewritten to summarize
what was found. Every section in the finished document carries something — a
heading holding only its blockquote, or nothing at all, is a question nobody
answered, so say `None.` under `## Open Questions` rather than leaving it blank.
