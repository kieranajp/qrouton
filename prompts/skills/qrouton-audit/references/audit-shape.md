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
| `high` | It causes wrong behaviour or data loss today, or most changes in its area must work around it. |
| `medium` | It will cause drift or extra work the next time someone changes that area. |
| `low` | Cosmetic or local; fixing it saves little. |

A structural split that most changes in its area must work around is `high`.

`## Summary` gives the findings by impact: one sentence per high and medium
finding, high first, saying what is wrong and why it matters. It never gives a
bare list of ids. Low findings are counted per tell.

Cosmetic nits that come from one shared root cause become one finding, and
that root cause is its defect. A nit with no shared root cause is
not filed. Never pad the audit with nits.

A root cause is a defect in the code. Candidates found by the same detector,
tool or trap have that in common, but it is not a shared root cause, and they
are never grouped into one finding for it.

A proposed fix that deletes code keeps every test that would also serve
another purpose, such as checking exhaustiveness or a contract, or it says what
replaces that test.

A tell-3 finding's cost line states the library's size and installed status,
and the install cost when it is not installed. The cost is weighed against the
whole family: every homegrown piece doing that library's job, including pieces
filed under other tells. The cost line lists every family location. A case goes
under `## Checked and deliberate` for its cost only when the cost outweighs the
whole family. A spare that contradicts another finding is wrong: never spare a
library while another finding proposes growing or copying the homegrown
version.

The tell-3 sweep records an outcome per category for each common library
category: the finding ids, its line in `## Checked and deliberate` (a
cost-spare or a written reason), or "none hand-rolled" with the search that
shows it. It also checks every comment that makes platform or runtime claims
against the layer that runs the code. A claim that no longer holds is a
finding, filed under the tell of the code the claim justifies.

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

`## Tools` holds the tools, the job inventory, the tell-3 category outcomes
and the trap counts. The job inventory is the full inventory: every import,
built-in, global and internal module used from three or more files in two or
more directories, and every kind of shared machinery that reaches as many, such
as lifecycle subscriptions, error types or routing APIs. Count shipped files
only. The inventory is a table of entry, file count and outcome, one short line
per row, never only the jobs that were kept. Each row carries one outcome:

| Outcome | What the row shows |
| --- | --- |
| one route | the routes searched: the patterns grepped and where, as evidence that no other exists |
| findings | the finding ids |
| deliberate | its line in `## Checked and deliberate` |
| stated skip | a reason the reader can check: a `path:line` or a search result |

A skip because the routes cannot differ in behaviour must
name what was compared. Pure entries that do no I/O and hold no config or
state may share one stated-skip row that lists them. "Low value" and "out of
scope" are not reasons. No section holds open questions or follow-ups.

## Before returning

The lead checks its finished draft in two passes. First a fresh read-only
agent gives it an independent re-read against live code. The re-read verifies
every claim that copies agree or differ today, every one route outcome, every
cost-spare against the other findings (no spare whose family another finding
proposes to grow), every fix that deletes code or tests (no deleted test that
would serve another purpose), and every runtime claim. The lead fixes or drops
each failed finding, and corrects a failed inventory row's outcome without
removing the row.

Then the lead runs a conformance pass, last, and fixes every violation of these
rules:

- one finding per defect, with no bundled or repeated defects;
- every field present in order, `Impact:` included, with `Triage:` last;
- no open questions, follow-ups or loose candidates in any section;
- one line per candidate under `## Checked and deliberate`, each with its own
  tie, written reason or weighed install cost;
- `## Tools` holds tools, the full inventory, category outcomes and trap
  counts only, and every row has a file count and one outcome;
- every one route row lists the routes searched, and every stated skip gives a
  reason the reader can check;
- every library category has an outcome;
- each tell-3 cost line states size and installed status and lists the whole
  family, and no spare contradicts another finding;
- every runtime claim cites the runtime-layer file or package that settles it;
- cosmetic nits grouped by one shared root cause, or left out, and nothing
  grouped only because one detector, tool or trap found it;
- every deleting fix keeps the tests that serve another purpose;
- `## Summary` gives one sentence per high and medium finding, high first,
  and no bare list of ids.

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
