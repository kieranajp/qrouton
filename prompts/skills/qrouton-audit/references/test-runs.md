# Targeted test runs

A targeted run confirms one tell-4 verdict by running one test against one
change. It never replaces reading: candidates still come from reading the
tests, the runner's config and the code.

## When to run

Run only to confirm a step 7 or step 9 verdict you mean to keep. Read every
other step's verdict from the code.

## How to run

Make one scratch copy per audit, by the scratch-copy rules you were given, and
run everything in it. Every edit below is to a file in the copy.

1. Find the ecosystem's command that runs one test alone. The worked examples
   give one for each of theirs.
2. Run the test alone, unbroken, with a timeout of 120 seconds. When it
   fails, or does not finish, the baseline is broken: record a stated skip with the
   output, and judge by reading instead.
3. Make one change in the shipped code the test names. For step 7, make one
   break, such as a wrong return value, a skipped branch or a dropped call. For
   step 9, make one harmless change that keeps behaviour, such as rewording a
   message no shipped code checks or reordering a list whose order nothing
   reads.
4. Run the test alone again, with the same timeout.
5. Before the next run, restore the changed file in the copy from the
   checkout you were given: `cp <checkout>/<path> <copy>/<path>`. The
   checkout is only ever read.

## Results

Each run ends in one of three results:

| Result | Meaning |
| --- | --- |
| `killed` | the test failed after the change |
| `survived` | the test passed after the change |
| `did not run` | the baseline failed, the run timed out, or the command could not start |

A step 7 verdict that the test cannot fail needs `survived`. A step 9 verdict
that a harmless change breaks the test needs `killed`. `did not run` confirms
nothing: judge by reading instead, and say so in the finding.

## Limits

- At most 20 runs per audit, the baseline runs included. Spend them on the
  verdicts with the most impact.
- Never run a whole suite, a coverage tool, a watch mode, or a flag that
  updates snapshots or golden files.
- Never run a test that needs a network service, a database, a browser or a
  device.
- With no runnable single-test command, or dependencies the copy lacks, run
  nothing for that test: record a stated skip that names the reason, and judge
  by reading instead.

## The run table

`## Tools` holds a run table, one row per run: the test, the change, the
command and the result. A finding whose verdict rests on a run names the row's
result in `Evidence:`. Record the checkout's status comparison as one more line
under the table.
