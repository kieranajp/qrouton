# Worked example: Go

This is how one ecosystem's detectors were chosen, run and distrusted. Read it
for the method. In any other ecosystem, pick that ecosystem's own tools the
same way and treat them as equal choices.

Run each command against the target and write its output to a scratch
directory outside it. Never pass a flag that rewrites code, never write a config file
into the target, and never add a dependency to the target during the audit
run: `go run pkg@version` fetches into the module cache, not the target's
`go.mod`. A proposed fix may add one.

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
