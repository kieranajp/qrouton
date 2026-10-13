# The shape of a research document

A research workstream has one document. Framing creates it with `create_artefact`
and carries the approved questions as headings with nothing answered under
them; the research lead then fills it in where it stands. A questions brief is
simply this document before anyone has answered it.

Every second-level heading is one item in the workbench accordion, labelled with
its heading text, so an approved question becomes a `##` heading carrying that
question verbatim and the findings that answer it sit under it. A first section
named `Summary` — that word exactly, case aside — is pinned above the accordion
and always visible, so write it to be read on its own. Anything else that earns a
place is a trailing item rather than a special case; a closing `## Open Questions`
is simply the last one.

`create_artefact` writes the skeleton from `research-template.md` beside this
file: the frontmatter, a `Summary`, one placeholder question and a closing
`## Open Questions`. Replace the placeholder with one `##` heading per approved
question, each carrying a blockquote of framing.


One item per approved question, in the order they were asked. Sections beyond
those are yours to choose; the ones above are the ones we expect.

Answering consumes the framing rather than sitting beside it: each question's
blockquote is replaced by the finding, and the Summary is rewritten to summarize
what was found. A heading still holding only its blockquote is a question nobody
answered.
