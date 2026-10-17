# Splitting a large target into areas

A large target is audited one area at a time, so each inventory row stays
narrow enough to check every site. A seam pass then checks the facts that
cross areas, which no single area owns. Everything merges into one audit
document.

## Pick the areas

Derive the areas from the repo itself. An area is one of:

- a language;
- a top-level folder;
- a workspace package or module with its own manifest.

There is no fixed list. Pick the cut that gives each area one build and one
set of conventions. Count only shipped source files, never tests, fixtures,
generated files or vendored code.

## Decide whether to split

Split only when the scope holds two or more areas and more than
300 shipped source files. Make at most four areas: while there are more,
merge the smallest into the area that imports it most. An area under 30
shipped source files joins the area that imports it most, or the smallest area
when none imports it.

Anything smaller, or a scope narrowed to one area, is one area. One area runs
as an unsplit audit, and nothing in this file applies to it.

## Per area

Run each detector once over the whole target, never once per area. Route each
candidate to the area that holds its file, and a test to the area of the code
it names. A clone pair whose copies sit in two areas, and a candidate in a
file no area holds, goes to the seam pass instead.

Each area then gets its own:

- job inventory. Count each entry across the whole target, so an entry over
  the threshold gets a row in every area that uses it;
- tell-3 category sweep;
- checkers, one or more for each chosen tell, briefed with that area's paths
  and candidates.

## The seam pass

The seam pass runs when the audit splits and tell 1 is chosen, after every
area checker has returned. It is one tell-1 checker, briefed with five inputs:

1. the areas table;
2. every inventory row, from every area, that holds a data table, a
   vocabulary, a constant set, a parser or a format, and every entry used
   from two or more areas;
3. the boundaries where one area calls another, such as routes, message or
   event names, generated bindings and the config that generates them;
4. the clone pairs whose copies cross areas;
5. the files outside the code in scope, such as docs, prompts, fixtures and
   config, that state facts the code holds.

Ask it to find each fact or behaviour kept on both sides of a boundary, such
as a constant set kept in two languages, and to judge each with the five-step
check.

## Merge

Merge the area and seam returns into one draft:

- One finding per defect holds across areas. A seam finding absorbs the area
  findings that describe the same fact, and lists all their locations.
- When an area verdict and the seam verdict on one fact differ, re-read the
  code and keep one.
- Number findings once, after the merge, across the whole document.
- `## Tools` opens with an areas table: area, paths, languages and shipped
  source files.
- The inventory and category tables gain an `Area` column. A row for an entry
  the seam pass judged carries `seam`.

Headings, frontmatter and finding fields do not change.

## Check the merged draft

The verifier runs once over the merged draft, never per area. Spares,
contradictions and families need the whole set. Split its checks the usual
way when one report cannot hold the failures.

## Fan-out

The checkers number the areas times the chosen tells, plus one seam checker
when tell 1 is chosen, plus any overflow splits when one report cannot hold
every candidate. The cap is 20 checker spawns. Above it, merge the two
smallest areas and count again. One area left runs unsplit.
