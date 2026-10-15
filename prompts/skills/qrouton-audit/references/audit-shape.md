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
### F3 — Two timeout helpers race a promise against a timer

Tell: duplicated behaviour or facts
Locations: src/api/http.ts:41, src/sync/queue.ts:88
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
- `Verdict:` with the step of the four-step check the code fails;
- `Proposed fix:`;
- `Behaviour risk:` as `safe`, `needs a test first` or `changes behaviour`;
- `Confidence:` as `high`, `medium` or `low`;
- `Source:` as the tool that found the candidate, or `llm`;
- an empty `Triage:` line, last in the block.

## The four-step check

1. Is there a tie: a type or a test that fails when the copies drift?
2. Is there a reason at the site that still holds?
3. Can the guarded case occur?
4. Do the copies agree today?

How the steps decide:

- Copies that disagree today are rot, whatever the intent.
- Otherwise a copy with a tie, or with a reason at the site that still holds,
  is deliberate. A reason that no longer holds is rot.
- A guard is rot unless a live producer can bring about the case it guards.
  A guard on an I/O boundary is never rot.

A deliberate candidate goes under `## Checked and deliberate` as one line
with its location and the reason, never among findings.

## Triage

The reader triages on the `Triage:` line. A choice counts only when the line
holds one of `accept`, `reject` or `defer` and nothing else:

```markdown
Triage: reject
the copy in sync/ is kept for the offline build
```

That is a reject with a note, which runs on the following lines up to the next
heading. Anything else on the line is a note with no choice: `Triage: accept,
but after the release`, `Triage: maybe`. Never infer a choice from a note.

When the user triages in chat, write their words into that finding's `Triage:`
line in the same forms.
