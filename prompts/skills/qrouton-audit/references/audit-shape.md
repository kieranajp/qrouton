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
Impact: medium
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
- `Impact:` as `high`, `medium` or `low`, by the meanings below;
- `Confidence:` as `high`, `medium` or `low`;
- `Source:` as the tool that found the candidate, or `llm`;
- an empty `Triage:` line, last in the block.

Write one finding per defect. When two candidates describe the same defect,
even under different tells, merge them into one finding that lists every
location. File it under the tell whose fix removes it, and name the other tell
in `Evidence:`. Never bundle unrelated defects into one finding.

A candidate you cannot settle goes among the findings with `Confidence: low`,
or nowhere. No other section holds candidates.

Impact says how much fixing the finding is worth:

| Impact | Meaning |
| --- | --- |
| `high` | It causes wrong behaviour today, or most changes in its area must work around it. |
| `medium` | It will cause drift or extra work the next time someone changes that area. |
| `low` | Cosmetic or local; fixing it saves little. |

`## Summary` lists the findings by impact, high first.

Cosmetic nits that come from one shared root cause become one finding, and
that root cause is its defect. A nit with no shared root cause is
not filed. Never pad the audit with nits.

A tell-3 finding's cost line states the library's size and installed status,
and the install cost when it is not installed. The cost is weighed against the
whole family: every homegrown piece doing that library's job, including pieces
filed under other tells. The cost line lists every family location. A case goes
under `## Checked and deliberate` for its cost only when the cost outweighs the
whole family. A spare that contradicts another finding is wrong: never spare a
library while another finding proposes growing or copying the homegrown
version.

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
runs the code before it decides anything, and cite the file or package that
settles it. Inferring a claim from the target's own code
is not a runtime check. Verify it or drop it. A verdict that rested on a
dropped claim makes the candidate a `Confidence: low` finding, or nothing.

`## Checked and deliberate` holds one line per candidate, each with its
location and the tie or written reason that keeps it. A tell-3 case spared for
its cost is one line giving the cost and every family location. Never dismiss
candidates as a group. A tool candidate that a known trap explains is not listed here:
`## Tools` counts it by trap.

`## Tools` holds the tools, the job inventory and the trap counts. The job
inventory lists each job: a primitive, built-in or module used from three or
more files in two or more directories, or a concern handled that widely
without a shared module. Each job carries its outcome: one route, the finding
ids it gave, or deliberate with its line in `## Checked and deliberate`. No
section holds open questions or follow-ups.

## Before returning

The lead runs a conformance pass over its draft and fixes every violation of
these rules:

- one finding per defect, with no bundled or repeated defects;
- every field present in order, `Impact:` included, with `Triage:` last;
- no open questions, follow-ups or loose candidates in any section;
- one line per candidate under `## Checked and deliberate`, each with its own
  tie, written reason or weighed install cost;
- `## Tools` holds tools, the job inventory and trap counts only, and every job
  has an outcome;
- each tell-3 cost line states size and installed status and lists the whole
  family, and no spare contradicts another finding;
- every runtime claim cites the runtime-layer file or package that settles it;
- cosmetic nits grouped by one shared root cause, or left out;
- `## Summary` lists findings by impact, high first.

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
