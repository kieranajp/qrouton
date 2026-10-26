`## Tools` holds a run table with one row per attempted run, numbered from 1. Baselines, runs that timed out and runs that failed to start each get a row. Each row gives the test, the change, the command and the result. A baseline's change is `none`, and a baseline that passes is `survived`.

- The command runs one named test, anchored so no other test matches, with output that names each test it runs, and a timeout of exactly 120 seconds.
- The result is `killed`, `survived` or `did not run`, and no other word. A run whose output does not show that exactly one test ran is `did not run`. The test's own subtests count as part of it, and tests the output marks as skipped do not count.
- Each run's full output is saved as `run-<n>.txt` in the history directory, where `<n>` is the row number.
- The last line under the table reads `Checkout status: unchanged`, or `Checkout status: changed` followed by the first and last reads.
