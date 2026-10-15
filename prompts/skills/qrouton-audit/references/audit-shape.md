# The shape of an audit document

The workbench reads an audit as a set of findings the user triages in place.
It finds them by heading, so keep the headings exactly as below.

`create_artefact` writes the skeleton from `audit-template.md` beside this
file: the frontmatter with `target:` empty, then `## Summary`, one section per
tell, `## Checked and deliberate` and `## Tools`. Keep those headings exactly.
Set `target:` to the repository audited, followed by the path when the audit
was narrowed to one. Replace each section's blockquote with its content. A tell
section with no findings says `None.`.

The audit is complete when `## Summary` and the three tell sections each hold
content.

## Findings

A finding sits under the section for its tell and looks like this:

```markdown
### F3 — Two date formatters build the same display string

Tell: duplicated behaviour or facts
Locations: lib/reports/format.py:41, lib/export/rows.py:88
Evidence: <what the code shows, with the lines that show it>
Verdict: rot. Fails step 1, no tie: nothing fails if one copy drifts
Proposed fix: <the change, in a sentence or two>
Behaviour risk: safe
Confidence: high
Source: llm

Triage:
```

Each finding has:

- a `### F<n> — <title>` heading. Number findings once across the whole
  document. An id never changes once written, and a re-run never reuses the
  id of a finding it dropped;
- `Tell:` naming its section in lower case;
- `Locations:` as one or more `path:line`, comma-separated;
- `Evidence:` with what the code shows. It may run onto following lines, up to
  the next field;
- `Verdict:` with the step of the five-step check the code fails;
- `Proposed fix:`;
- `Behaviour risk:` as `safe`, `needs a test first` or `changes behaviour`;
- `Confidence:` as `high`, `medium` or `low`;
- `Source:` as the tool that found the candidate, or `llm`;
- an empty `Triage:` line, last in the block.

Write one finding per defect. When two candidates describe the same defect,
even under different tells, merge them into one finding that lists every
location. File it under the tell whose fix removes it, and name the other tell
in `Evidence:`. Never bundle unrelated defects into one finding.

A candidate you cannot settle goes among the findings with `Confidence: low`,
or nowhere. No other section holds candidates.

## The five-step check

1. Is there a tie: a type or a test that fails when the copies drift?
2. Is there a reason written at the site that still holds?
3. Can the guarded case occur?
4. Do the copies agree today?
5. Does a live caller in shipped code reach it?

How the steps decide:

- Copies that disagree today are rot, whatever the intent.
- Otherwise a copy with a tie, or with a written reason that still holds, is
  deliberate. A reason that no longer holds is rot.
- A guard is rot unless a live producer can bring about the case it guards.
  A guard on an I/O boundary is never rot.
- Code that no live caller in shipped code reaches, once the tool's traps are
  ruled out, is rot whatever the other steps say. Callers in tests, stories and
  fixtures do not count. Exported API that other modules consume counts as
  reached.

A reason counts only when it is written at the site: a comment, a doc, or the
commit message for those lines. A reason the auditor supplies is not a reason.
Check each runtime claim in a written reason against the layer that really
runs the code before it decides anything.

`## Checked and deliberate` holds one line per candidate, each with its
location and the tie or written reason that keeps it. Never dismiss candidates
as a group. A tool candidate that a known trap explains is not listed here:
`## Tools` counts it by trap.

## Triage

The reader triages on the `Triage:` line. A choice counts only when the line
holds one of `accept`, `reject` or `defer` and nothing else:

```markdown
Triage: reject
the export copy pads columns, so the two differ on purpose
```

That is a reject with a note, which runs on the following lines up to the next
heading. Anything else on the line is a note with no choice: `Triage: accept,
but after the release`, `Triage: maybe`. Never infer a choice from a note.

When the user triages in chat, write their words into that finding's `Triage:`
line in the same forms.
