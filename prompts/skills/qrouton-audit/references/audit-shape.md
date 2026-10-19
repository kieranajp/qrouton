# The shape of an audit document

The workbench reads an audit as a set of findings the user triages in place.
It finds them by heading, so keep the headings exactly as below.

`create_artefact` writes the skeleton from `audit-template.md` beside this
file: the frontmatter with `target:` empty and `tells:` listing every tell,
then `## Summary`, one section per tell, `## Checked and deliberate` and
`## Tools`. Keep those headings exactly. Set `target:` to the repository
audited, followed by the path when the audit was narrowed to one. Replace each
section's blockquote with its content. A tell section with no findings says
`None.`.

## The chosen tells

`tells:` lists the slugs of the tells the audit runs, in tell order, as a YAML
list:

| Slug | Section |
| --- | --- |
| `duplicated` | `## Duplicated behaviour or facts` |
| `dead` | `## Dead or needlessly defensive code` |
| `homegrown` | `## Homegrown library or platform behaviour` |
| `volkswagen` | `## Tests that pass or fail for the wrong reason`; an older audit's `## Tests that cannot fail or pass for the wrong reason` still reads as this tell |

The lead deletes the slug and the section of each tell not chosen, so an audit
of tells 2 and 3 carries `tells: [dead, homegrown]` and neither section of
tell 1. An audit with no `tells:` key was written before the key existed, and
it ran the three tells of that time: `duplicated`, `dead` and `homegrown`.

The audit is complete when `## Summary` and each chosen tell section hold
content.

An audit does the work of only the chosen tells. It runs their detectors, hunts
and checks, and merges findings among them. The job inventory runs
only when tell 1 or tell 3 is chosen, and the tell-3 category sweep only when
tell 3 is. A rule below that belongs to a tell the audit did not run does not
apply.

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
- `Verdict:` with the step the code fails: one of the five-step check for
  tells 1 to 3, or one of steps 6 to 8 of the test check for tell 4;
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
Copies that differ or disagree today are `high`, unless the finding states a
cosmetic difference in one clause.

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

A fix that derives several lists from one table keeps every gate the current
lists enforce, and a list that is deliberately a subset stays a subset, for
example through a filter on the table.

A tell-3 finding's cost line states the library's size and installed status,
and the install cost when it is not installed. The cost is weighed against the
whole family: every homegrown piece doing that library's job, including pieces
filed under other tells. The cost line lists every family location. A case goes
under `## Checked and deliberate` for its cost only when the cost outweighs the
whole family. A spare that contradicts another finding is wrong: never spare a
library while another finding proposes growing or copying the homegrown
version. A family member with a behaviour defect is a finding, never part of a
cost-spare. It still counts in the family the cost is weighed against, and the
spare covers only the defect-free rest.

The tell-3 sweep records an outcome per category for each common library
category: the finding ids, its line in `## Checked and deliberate` (a
cost-spare or a written reason), "uses the library" with where, or "none
hand-rolled" with the search that shows it. It also checks every comment that makes platform or runtime claims
against the layer that runs the code. A claim that no longer holds is a
finding, filed under the tell of the code the claim justifies.

## The five-step check

1. Is there a tie: a type, a test, a generator or a build step that fails or
   regenerates when the copies drift?
2. Is there a reason written at the site that still holds?
3. Can the guarded case occur?
4. Do the copies agree today, and does each do what its own comment says?
5. Does a live caller in shipped code reach it?

How the steps decide:

- Copies that disagree today are rot, whatever the intent. A copy that
  contradicts its own comment does not agree.
- Otherwise a copy with a tie, or with a written reason that still holds, is
  deliberate. A reason that no longer holds is rot.
- A guard is rot unless a live producer can bring about the case it guards.
  A guard on an I/O boundary is never rot.
- Code that no live caller in shipped code reaches, once the tool's traps are
  ruled out, is rot whatever the other steps say. Callers in tests, stories and
  fixtures do not count. Exported API that other modules consume counts as
  reached.

A reason counts only when it is written at the site: a comment, a doc, or a
commit message for those lines or beside them. A reason the auditor supplies is not a reason.
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

`## Tools` holds the tools, the job inventory and the tell-3 category outcomes
when they ran, and the trap counts. An audit split into areas by `areas.md`
opens `## Tools` with the areas table, and its inventory and category tables
carry an `Area` column, with `seam` for seam rows. Every file type in scope that no tool
covered gets a stated skip that names the file type and why. The job inventory is the full inventory: every import,
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

A one route or stated-skip row whose entry is used from ten or more files also
names the call sites searched.

A skip because the routes cannot differ in behaviour must
name what was compared. Pure entries that do no I/O and hold no config or
state may share one stated-skip row that lists them. A data table is not
pure: a module holding a display-string table, a vocabulary or constants
another module mirrors gets its own row. "Low value" and "out of scope" are
not reasons. No section holds open questions or follow-ups.

## The test check

Tell 4 uses three more steps in place of the five:

6. Does the runner collect and run the test?
7. Would the test fail if the code it names were broken?
8. Does shipped code behave the same under test as in production?

How the steps decide:

- A test that fails step 7 is rot, whatever its reason.
- A skip, or shipped code that behaves differently under test, is deliberate
  only when a reason written at the site still holds and no test relies on it
  to pass. Otherwise it is rot.
- A test weakened to pass, such as an expected value changed to match the
  output or an assertion removed, needs commit evidence: the commit and the
  lines it loosened. A checker cannot read history, so it returns such a
  candidate as `needs history`, and the lead reads the commits.

The audit never runs the target's tests, and runs no coverage tool. What the
runner collects comes from reading its config and test-file patterns. Nothing
runs in a mode that updates snapshots or golden files. Code that only tests
reach comes from the reachability run that leaves tests out as entry points.
When tell 2 runs, that code is filed under tell 2, because deleting it removes
the finding. When only tell 4 runs, it is filed under tell 4, and its verdict
says it fails step 5.

## Before returning

The lead checks its finished draft in two passes. First a fresh read-only
agent gives it an independent re-read against live code. The re-read verifies
every claim that copies agree or differ today, every one route outcome, every
cost-spare against the other findings (no spare whose family another finding
proposes to grow), every fix that deletes code or tests (no deleted test that
would serve another purpose), every runtime claim, every claim that a test
never runs or cannot fail, and every claim that shipped code behaves
differently under test. It also checks every "no tie" and "no written reason"
claim, the latter against history the lead collects into a history directory,
every behaviour-risk label, every fix to persisted data for a migration, and
every impact rating against behaviour that is wrong today. It also reads the
comments at each copy, samples call sites behind widely used one route and
stated-skip rows, verifies every category outcome, re-checks every tell-4 line
under `## Checked and deliberate` and every fix that derives lists from one
table. It skips the one route check when no inventory ran,
the cost-spare check when tell 3 did not run, and the test claims when tell 4
did not run. The lead fixes or drops each failed finding, and corrects a failed
inventory row's outcome without removing the row.

Then the lead runs a conformance pass, last, and fixes every violation of these
rules, skipping those for a tell or an inventory that did not run:

- one finding per defect, with no bundled or repeated defects;
- every field present in order, `Impact:` included, with `Triage:` last;
- no open questions, follow-ups or loose candidates in any section;
- one line per candidate under `## Checked and deliberate`, each with its own
  location and its own tie, written reason or weighed install cost; only a
  cost line lists several locations, and they are the family it was weighed
  against;
- `## Tools` holds tools, the full inventory, category outcomes and trap
  counts only, and every row has a file count and one outcome;
- a split audit opens `## Tools` with the areas table, and every inventory and
  category row carries its `Area`;
- every one route row lists the routes searched, and every stated skip gives a
  reason the reader can check; a row whose entry is used from ten or more files
  names the call sites searched;
- every file type in scope that no tool covered has a stated skip that names
  the file type;
- every library category has an outcome: finding ids, a line in
  `## Checked and deliberate`, "uses the library", or "none hand-rolled" with
  its search;
- each tell-3 cost line states size and installed status and lists the whole
  family, no spare contradicts another finding, and no spare covers a
  behaviour defect;
- every runtime claim cites the runtime-layer file or package that settles it;
- cosmetic nits grouped by one shared root cause, or left out, and nothing
  grouped only because one detector, tool or trap found it;
- every deleting fix keeps the tests that serve another purpose, or names what
  replaces them;
- every fix that derives lists from one table keeps every gate, and a
  deliberate subset stays a subset;
- every finding whose verdict is that copies differ or disagree today is
  `high`, or states a cosmetic difference in one clause;
- every tell-4 finding names the test and the code it names, and its verdict
  names the step from 6 to 8 it fails, or step 5 for code only tests reach;
- every weakened-test finding cites the commit and the lines it loosened;
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
