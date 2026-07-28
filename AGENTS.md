# qrouton agent guide

qrouton is a Go terminal app that assembles multi-repo workspaces from shared bare mirrors, then launches a coding agent inside a generated Zellij session.

## Design directions

The invariants below are mechanical rules. These are the reasons they exist; when a change has to trade one thing off against another, trade in favour of these.

1. **Launcher, not harness.** qrouton's responsibility ends at `exec`. No pty, no daemon, no headless runner, no reimplemented chat UI. Everything past handoff belongs to the agent runner.

2. **The repository group is discovered, not declared.** You do not know which repos a piece of work needs until you know the work. Sessions therefore start empty and acquire repositories at escalation, rather than requiring a curated group up front. This is the thing no comparable tool does.

3. **Orchestration is the product.** The agent you talk to is an orchestrator that stays lean by delegating; leads own the read-heavy and write-heavy work; specialists answer bounded questions. The workspace exists to make that tree *legible* — panes are how you watch several things at once, which is why panes rather than tabs are the top-level primitive. Fan-out is concurrent, so the display must be concurrent.

   Two constraints keep the fan-out honest. **Depth is bounded, breadth is not:** three levels (orchestrator → lead → specialist), enforced by specialists declaring a read-only `tools:` set that excludes `Task`, so they cannot spawn. **Writes stay single-threaded:** extra agents add intelligence, not concurrent edits. More agents is not the goal; a wider fan-out that nobody can observe is how a run invents a datastore that does not exist.

4. **A collaborative workbench, not an autonomous one.** Human and agent share one surface and either may drive. Focus is borrowed, never taken: every grab announces itself and one key returns it. Leaning in and stepping back are both cheap.

5. **No prior knowledge required.** Discoverability is a requirement, not polish. One chord, no prefix, no modes to enter. Affordances live on the objects they operate, and the agent is expected to explain the workspace on request. The test is an engineer whose terminal use is `yarn dev`.

6. **Durable artifacts over live state.** Work survives context loss through documents and code, never through conversation. Phase is derived from what exists on disk. Prompts are re-stamped every launch. Clear beats compact.

## Architecture

Dependency direction: `config ← github ← session ← tui`; `launch`, `agents`, `repos`, and `mcpserver` sit above the shared leaves. Nothing imports `tui`. `mux` carries the multiplexer ports: `launch` and `mcpserver` drive the terminal only through `Launcher`/`PaneHost`, and the Zellij adapter behind them is the only backend there will be — the ports exist to quarantine Zellij's mechanics and to give the pane tools a fakeable seam, not to admit a second backend.

The shared leaves import nothing of qrouton's own, so anything may depend on them: `sessionpaths` (a session's on-disk layout), `codex` (the Codex CLI's own files), `paneui` (in-place terminal frames for the watch panes), and `prompts` (canonical prompts, provider rendering, and the discovery-tree stamper).

- `main.go`: urfave/cli app; root action runs the onboarding flow, subcommands come from `cmd/*`.
- `cmd/mcp/`, `cmd/agents/`, `cmd/repos/`: `*cli.Command` definitions (flags only) delegating to `internal/*`.
- `internal/tui/`: fullscreen Bubble Tea onboarding and async UI state.
- `internal/session/`: manifest schema, active/reference roles, mirrors, worktree lifecycle.
- `internal/github/`: authenticated owner discovery, cache, concurrent refresh.
- `prompts/`: canonical workflow, skill, and agent prompts, provider rendering, and `Stamp` — the one implementation of the runner discovery tree, shared by launches and evals.
- `internal/launch/`: runner launch/resume arguments, MCP injection, the backend-neutral workspace layout, and editor resolution. Asset stamping delegates to `prompts.Stamp`; only the mode-to-discovery-file decision lives here.
- `internal/mux/`: multiplexer ports (`Launcher`, `PaneHost`), the `Handle` that carries backend identity into the MCP child, and the Zellij adapter (KDL rendering, session lifecycle, pane actions).
- `internal/mcpserver/`, `internal/agents/`, `internal/repos/`: agent-driven file opening in the editor pane; subagent and repo status panes (both drawn via `paneui`).
- `internal/sessionpaths/`, `internal/codex/`, `internal/paneui/`: the shared leaves above.
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

Do not discard unrelated worktree changes or edit generated session assets directly; change prompt sources under `prompts/`, launch support scripts under `internal/launch/scripts/`, or multiplexer assets under `internal/mux/assets/`.
