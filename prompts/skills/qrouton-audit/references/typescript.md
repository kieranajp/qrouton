# Worked example: TypeScript and JavaScript

This is how one ecosystem's detectors were chosen, run and distrusted. Read it
for the method. In any other ecosystem, pick that ecosystem's own tools the
same way and treat them as equal choices.

Run each command against the target and write its output to a scratch
directory outside it. Never pass `--fix`, never write a config file into the target,
and never add a dependency to the target during the audit run. A proposed fix
may add one.

## Repo checks first

Run the repo's own checks first. Read the `package.json` scripts and the
check target the CI config or Makefile runs. Run the repo's ESLint config and
its type-check, such as `tsc --noEmit` against its `tsconfig`, with output to
scratch. When the repo runs a framework's own component checker, run that too.
Never run its test or build scripts. Run a script that passes `--fix` or
writes files as its underlying command without that flag, and point ESLint's
cache and any `.tsbuildinfo` at scratch. Every file type in scope that none of these parses gets a
stated skip that names the file type and why.

## Reachability: knip

knip builds the import graph from the project's entry points and reports
unused files, exports, dependencies and types.

- Prefer the version the repo pins. Otherwise run a pinned release:
  `npx --yes knip@5.88.1 --reporter json`.
- Run it twice: once by default, and once with `--production`, which drops
  tests and dev tooling as entry points. Code that only tests import shows up
  only in the second run.
- Record both runs in `## Tools` with their candidate counts.

Check every candidate against these traps before it becomes a finding:

- **Platform-suffixed files.** `foo.ios.ts`, `foo.android.ts`, `foo.native.ts`
  and `foo.web.ts` are resolved by the bundler, not by import paths.
- **Config reached through a string.** Babel, Metro, Expo, Jest and ESLint
  configs name plugins and files as strings.
- **Dynamic or computed imports.** `import(name)`, `require(variable)` and
  glob imports hide edges from the graph.
- **Files read with `fs`.** A file read at build or run time has no importer.
- **Tests as entry points.** Default mode counts test-only code as live.

## Clone detection: jscpd

jscpd finds token-level copies.

- `npx --yes jscpd@4.3.0 --reporters json --output <scratch> --ignore "**/node_modules/**,**/dist/**" <target>`.
- It finds token copies only. Two helpers that do the same job in different
  words do not match, so its output must not make up most of tell 1. Hunt the
  semantic copies with the lead's tell-1 method.

## Type-aware lint: typescript-eslint

`@typescript-eslint/no-unnecessary-condition` flags conditions the types say
cannot vary.

- The rule needs typed linting, so it runs only where the repo's ESLint
  config points typescript-eslint at a `tsconfig`.
- Use the repo's own ESLint and typescript-eslint when it has them, run with
  that rule enabled from the command line and output to scratch.
- The rule trusts declared types. At an unparsed boundary, such as network
  input, storage or a bridge, the declared type is a claim, not a fact. Trace
  every flagged guard to the producer of its value before calling it dead.

## Runtime

The JavaScript engine alone does not decide which globals and APIs exist. A
framework can install polyfills before the app's own code runs; Expo, for
example, ships a runtime polyfill layer. Before anything in the audit says an
API is missing, read what the framework installs at startup.

## Tests that pass or fail for the wrong reason

These are where tell-4 candidates hide in TypeScript and JavaScript. Find them
by reading; never run a whole suite, and never pass a flag that rewrites
snapshots or golden files, such as `-u`, `--update`, `--updateSnapshot` or
`--update-snapshots`.

- **Markers.** Grep test files for `.skip`, `.only` and `.todo`, the `x` and
  `f` prefixes (`xit`, `xdescribe`, `fit`, `fdescribe`), and conditional forms
  such as `it.skipIf(...)`, `describe.runIf(...)` or a ternary that picks
  `it` or `it.skip`. A committed `.only` stops its siblings from running, and
  some runners skip every other file as well, unless the CI run sets
  `forbidOnly` or `--forbid-only`, which fails the run instead.
- **Test patterns.** Read the runner's config for the files it collects:
  Jest's `testMatch`, `testRegex`, `roots` and `testPathIgnorePatterns`,
  Vitest's `include` and `exclude`, and Playwright's `testDir`, `testMatch`
  and `testIgnore`. A test file outside them never runs, and neither does one
  the package's `test` script never reaches.
- **Shipped code under test.** Grep non-test files for
  `process.env.NODE_ENV === "test"`, `JEST_WORKER_ID`, `VITEST` and globals
  the test config defines. An in-source test block behind `import.meta.vitest`
  is a test, not shipped behaviour, when the build defines it away.
- **Mocking the module under test.** A `jest.mock` or `vi.mock` of the path
  the test imports its subject from tests the mock.
- **Snapshots.** Read `__snapshots__` files and inline snapshots for an error
  message, `null`, an empty array or a loading state recorded as expected.
- **Test-only code.** Compare knip's default and `--production` runs: code
  only the default run counts as used is reached only from tests, scripts,
  configs or stories. When tests are its only callers, file it under tell 2
  when that tell runs, else under tell 4.
- **One test alone.** For a targeted run under `test-runs.md`, use the
  runner's file and name filter from the copy, through the copy's own
  installed runner: `npx vitest run <file> -t '<name>'`, or
  `npx jest <file> -t '<name>' --watchAll=false`. Vitest without `run` starts
  a watch mode. A test that needs a browser, such as a Playwright test, is not
  run.
