# Worked example: Go

This is how one ecosystem's detectors were chosen, run and distrusted. Read it
for the method. In any other ecosystem, pick that ecosystem's own tools the
same way and treat them as equal choices.

Run each command against the target and write its output to a scratch
directory outside it. Never pass a flag that rewrites code, never write a config file
into the target, and never add a dependency to the target during the audit
run: `go run pkg@version` fetches into the module cache, not the target's
`go.mod`. A proposed fix may add one.

## Repo checks first

Run the repo's own checks first. Read the Makefile's check and lint targets
and the CI config. Run the linter config the repo carries, such as
`.golangci.yml`, at the version the repo pins, and `go vet ./...`, with output
to scratch. Run a target that formats or fixes in place as its underlying
command without those flags. Non-Go file types in scope get their own
ecosystem's checks, or a stated skip that names the file type and why.

## Reachability: deadcode

`deadcode` reports functions unreachable from `main` by whole-program
analysis.

- From the target module: `go run golang.org/x/tools/cmd/deadcode@v0.52.0 ./...`
  for production reachability, and again with `-test` to count tests as
  entry points. A function only the second run reaches is test-only code.
- Record both runs in `## Tools` with their candidate counts.

Check every candidate against these traps:

- **Reflection and registration.** A method called through `reflect`, a
  plugin registry or a framework binding has no static caller.
- **Build tags and GOOS files.** `_darwin.go`, `_windows.go` and files behind
  `//go:build` tags are analysed for the host only.
- **Library packages.** Exported API in a module others import has no caller
  inside the module.
- **Generated code and `go:generate` tools.** A generator's `main` runs from
  a directive, not from the binary.

## Clone detection: dupl

`dupl` finds syntactically similar Go code.

- `go run github.com/mibk/dupl@v1.1.0 -t 50 <target>`, with the
  output redirected there.
- It matches syntax trees, so it misses two functions that do the same job in
  different shapes. Hunt those with the lead's tell-1 method.

## Needless conditions

Go has no direct counterpart to `no-unnecessary-condition`. `staticcheck`
and the `nilness` analyser find part of it:
`go run honnef.co/go/tools/cmd/staticcheck@2026.2.2 ./...`. Read the rest. A
guard on an error from I/O, a decoder or another process is never rot.

## Tests that pass or fail for the wrong reason

These are where tell-4 candidates hide in Go. Find them by reading; never
run a whole suite, and never pass a flag that rewrites snapshots or golden files.
Such update flags are defined by the target's own tests, so read the flag
definitions to know which ones to avoid.

- **Skips.** Grep test files for `t.Skip`, `t.Skipf` and `t.SkipNow`, and for
  skips behind `testing.Short()` or a missing environment variable. Check
  whether the CI config passes `-short` or sets the variable.
- **Build constraints.** A `_test.go` file behind a custom `//go:build` tag
  runs only when `-tags` names it; check the Makefile and CI config for that
  tag. One behind a GOOS or GOARCH constraint, or a suffix such as
  `_linux_test.go`, runs only on a matching CI host.
- **Files the go command ignores.** Directories named `testdata`, or starting
  with `_` or `.`, are never built, and a function named `Testfoo`, with a
  lower-case letter after `Test`, is never run.
- **Shipped code under test.** Grep non-test files for `testing.Testing()`,
  `flag.Lookup("test.v")` and checks on `os.Args[0]` ending in `.test`.
- **Assertions that cannot fail.** `t.Log` or `t.Logf` where the test should
  call `t.Error` or `t.Fatal`, an error assigned to `_`, and a test that only
  compares a fake's output with the value it was set up to return.
- **Test-only code.** Compare the two `deadcode` runs: a function only the
  `-test` run reaches is code only tests use. File it under tell 2 when that
  tell runs, else under tell 4.
- **One test alone.** For a targeted run under `test-runs.md`, run one test in
  one package from the copy: `go test -run '^TestName$' -count=1 -timeout 120s
  ./path/to/pkg`. Anchor the name so no sibling test runs, add `/SubName` for
  one subtest, and pass any build tag the CI config passes for that file.
