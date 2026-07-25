# qrouton agent guide

qrouton is a Go terminal app that assembles multi-repo workspaces from shared bare mirrors, then launches a coding agent inside a generated Zellij session.

## Architecture

Dependency direction: `config ← github ← session ← tui`; `launch`, `agents`, and `mcpserver` are leaves, and `repos` reads the manifest via `session`. Nothing imports `tui`. `mux` is the deepest leaf: `launch` and `mcpserver` drive the terminal multiplexer only through its `Launcher`/`PaneHost` ports, and the Zellij adapter behind them is the sole backend today.

- `main.go`: urfave/cli app; root action runs the onboarding flow, subcommands come from `cmd/*`.
- `cmd/mcp/`, `cmd/agents/`, `cmd/repos/`: `*cli.Command` definitions (flags only) delegating to `internal/*`.
- `internal/tui/`: fullscreen Bubble Tea onboarding and async UI state.
- `internal/session/`: manifest schema, active/reference roles, mirrors, worktree lifecycle.
- `internal/github/`: authenticated owner discovery, cache, concurrent refresh.
- `prompts/`: canonical workflow, skill, and agent prompts plus loader/provider rendering.
- `internal/launch/`: runner launch/resume arguments, MCP injection, the backend-neutral workspace layout, editor resolution, and session asset stamping.
- `internal/mux/`: multiplexer ports (`Launcher`, `PaneHost`), the `Handle` that carries backend identity into the MCP child, and the Zellij adapter (KDL rendering, session lifecycle, pane actions).
- `internal/mcpserver/`, `internal/agents/`, `internal/repos/`: agent-driven file opening in the editor pane; subagent and repo status panes.
- `internal/config/`: config file, XDG paths, first-run wizard.
- `cmd/qrouton-eval/`, `internal/evalharness/`: standalone prompt-eval binary; deliberately decoupled from the packages above.

## Invariants

- Active repos use session branches; reference repos are detached at pinned commits.
- Mirrors live under `<root>/.mirrors`; session worktrees live under `<session>/src`.
- Write `qrouton.json` last. A directory without it must not appear resumable.
- Never silently overwrite user-owned agent files; only replace qrouton-marked assets.
- Preserve cache-first startup, cancellable refreshes, and runner conversation resume.

## Working agreement

- Keep changes small and match existing package-level style.
- Use `apply_patch` for edits and `gofmt` changed Go files.
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
