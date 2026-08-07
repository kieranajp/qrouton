# qrouton agent guide

qrouton is a Go desktop app that assembles multi-repo workspaces from shared bare mirrors, then runs a coding agent in a terminal window it owns.

## Design directions

The invariants below are mechanical rules. These are the reasons they exist; when a change has to trade one thing off against another, trade in favour of these.

1. **qrouton owns the surface, not the conversation.** It may own a desktop process, PTYs and windows; it may not own the exchange between the user and the model. The runner is an opaque interactive child: no parsing model output, no deciding when work is done, no headless runs, no agent loop, no reimplemented chat UI. qrouton is a harness for the agent's harness, never a replacement for it.

2. **The repository group is discovered, not declared.** You do not know which repos a piece of work needs until you know the work. Sessions therefore start empty and acquire repositories at escalation, rather than requiring a curated group up front. This is the thing no comparable tool does.

3. **Orchestration is the product.** The agent you talk to is an orchestrator that stays lean by delegating; leads own the read-heavy and write-heavy work; specialists answer bounded questions. The workspace exists to make that tree *legible* — a window per thing is how you watch several at once, which is why the workbench opens real windows rather than stacking work behind tabs. Fan-out is concurrent, so the display must be concurrent.

   Two constraints keep the fan-out honest. **Depth is bounded, breadth is not:** three levels (orchestrator → lead → specialist), enforced by specialists declaring a read-only `tools:` set that excludes `Task`, so they cannot spawn. **Writes stay single-threaded:** extra agents add intelligence, not concurrent edits. More agents is not the goal; a wider fan-out that nobody can observe is how a run invents a datastore that does not exist.

4. **A collaborative workbench, not an autonomous one.** Human and agent share one surface and either may drive. Focus is borrowed, never taken: a window qrouton opens leaves the keyboard where it was, and the one case that needs it announces itself. Leaning in and stepping back are both cheap.

5. **No prior knowledge required.** Discoverability is a requirement, not polish. No chords, no prefixes, no modes to enter: a window with a title bar is driven by reflexes the user already has. Affordances live on the objects they operate, and the agent is expected to explain the workspace on request. The test is an engineer whose terminal use is `yarn dev`.

6. **Durable artifacts over live state.** Work survives context loss through documents and code, never through conversation. Phase is derived from what exists on disk. Prompts are re-stamped every launch. Clear beats compact.

## Architecture

Dependency direction: `config ← github ← session ← tui`; `launch`, `agents`, `repos`, and `mcpserver` sit above the shared leaves. Nothing imports `tui`. `workbench` carries the window port: `launch` and `mcpserver` reach the running workbench only through `WindowHost` and the `Handle` naming its control socket, so neither links a webview and the window tools keep a fakeable seam. `desktop` is the workbench itself — the Wails application, the conversation PTY, the window registry and the socket server — and it is the only package that imports Wails, imported only by `main.go`. That split is load-bearing: put the two together and every test that touches a window tool links WebKit through cgo.

The shared leaves import nothing of qrouton's own, so anything may depend on them: `sessionpaths` (a session's on-disk layout), `codex` (the Codex CLI's own files), and `prompts` (canonical prompts, provider rendering, and the discovery-tree stamper).

- `main.go`: urfave/cli app; the root action opens the workbench, subcommands come from `cmd/*`. `cmd/onboard` is hidden: it runs the onboarding TUI inside the conversation PTY and then execs the supervisor in place, so a session has one long-lived terminal.
- `cmd/mcp/`, `cmd/agents/`, `cmd/repos/`, `cmd/shell/`: `*cli.Command` definitions (flags only) delegating to `internal/*`.
- `internal/tui/`: fullscreen Bubble Tea onboarding and async UI state.
- `internal/session/`: manifest schema, active/reference roles, mirrors, worktree lifecycle.
- `internal/github/`: authenticated owner discovery, cache, concurrent refresh.
- `prompts/`: canonical workflow, skill, and agent prompts, provider rendering, and `Stamp` — the one implementation of the runner discovery tree, shared by launches and evals.
- `internal/launch/`: runner launch/resume arguments, MCP injection, the conversation and shell argv, the supervisor and its signal path, and editor resolution. Asset stamping delegates to `prompts.Stamp`; only the mode-to-discovery-file decision lives here.
- `internal/workbench/`: the window port (`WindowHost`, `WindowOptions`), the `Handle` that carries the control socket into the MCP child, and the socket client. No webview.
- `internal/desktop/`: the Wails application — the conversation PTY, the window registry and its lifecycle rules, the control-socket server, and the embedded xterm.js pages. Window construction sits behind a `renderer` seam so everything else is testable without a display.
- `internal/mcpserver/`, `internal/agents/`, `internal/repos/`: the agent's window tools; subagent event collection, whose own surface is deferred; legacy repo status.
- `internal/sessionpaths/`, `internal/codex/`: the shared leaves above.
- `internal/config/`: config file, XDG paths, and the on-demand owner prompt. `Load` never prompts and never fails for a missing value — a zero-repo session needs neither a root nor owners, so the root defaults and `EnsureOrgs` asks at the first repository search.
- `cmd/qrouton-eval/`, `internal/evalharness/`: standalone prompt-eval binary; deliberately decoupled from the packages above.

## Invariants

- Subagent depth stays bounded at three levels. Lead prompts (`qrspi-*-lead`) omit `tools:` deliberately, so they inherit `Task` and can delegate; every specialist declares a read-only `tools:` set without `Task`, so it cannot. Adding or removing one `tools:` line silently changes the topology — treat it as an architectural change, not prompt editing.
- Active repos use session branches; reference repos are detached at pinned commits.
- Mirrors live under `<root>/.mirrors`; session worktrees live under `<session>/src`.
- Write `qrouton.json` last. A directory without it must not appear resumable.
- Never silently overwrite user-owned agent files; only replace qrouton-marked assets.
- One owner per fact: a path convention, a helper, or a piece of copy lives in exactly one place. `sessionpaths` owns the session layout, `codex` owns Codex's, and each package keeps its literals in `strings.go` and its sentinel errors in `errors.go` rather than inline.
- Eval and launch must stamp identical trees. Both go through `prompts.Stamp`; fixtures carry real `session.Manifest` documents, and the fixture-schema test fails if they drift.
- Preserve cache-first startup, cancellable refreshes, and runner conversation resume.

## Working agreement

- Keep changes small and match existing package-level style.
- Comments default to absent. A comment earns its place by saying something the code cannot — a trap where the obvious reading is wrong, or why a non-obvious choice was made. One line where earned, two for a real trap, never a file-header paragraph: when every comment is big, they all cry wolf and none get read. State what *is*, not the debugging journey or what the code used to be. Never point at another file, symbol or line number — those move, comments do not.
- New user-facing text goes in the package's `strings.go`; new failure modes get a sentinel in its `errors.go`, wrapped with `%w` at the call site.
- `gofmt` changed Go files.
- Add focused tests for behavior changes.
- Before handoff run:

```sh
GOCACHE=/tmp/qrouton-go-cache go test ./...
GOCACHE=/tmp/qrouton-go-cache go test -race ./...
GOCACHE=/tmp/qrouton-go-cache go vet ./...
GOCACHE=/tmp/qrouton-go-cache go build -o qrouton .
git diff --check
```

Do not discard unrelated worktree changes or edit generated session assets directly; change prompt sources under `prompts/`, launch support scripts under `internal/launch/scripts/`, or the workbench's frontend under `internal/desktop/assets/`.
