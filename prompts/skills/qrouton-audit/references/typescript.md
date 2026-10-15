# Worked example: TypeScript and JavaScript

This is how one ecosystem's detectors were chosen, run and distrusted. Read it
for the method. In any other ecosystem, pick that ecosystem's own tools the
same way and treat them as equal choices.

Run each command against the target and write its output to a scratch
directory outside it. Never pass `--fix`, never write a config file into the target,
and never add a dependency to the target during the audit run. A proposed fix
may add one.

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
