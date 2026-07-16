# qrouton agent guide

qrouton is a Go terminal app that assembles multi-repo workspaces from shared bare mirrors, then launches a coding agent inside a generated Zellij session.

## Architecture

Dependency direction: `config ← github ← session ← tui`; `launch`, `agents`, and `mcpserver` are leaves. Nothing imports `tui`.

- `main.go`: urfave/cli app; root action runs the onboarding flow, subcommands come from `cmd/*`.
- `cmd/mcp/`, `cmd/agents/`: `*cli.Command` definitions (flags only) delegating to `internal/*`.
- `internal/tui/`: fullscreen Bubble Tea onboarding and async UI state.
- `internal/session/`: manifest schema, active/reference roles, mirrors, worktree lifecycle.
- `internal/github/`: authenticated owner discovery, cache, concurrent refresh.
- `internal/launch/`: runner launch/resume arguments, MCP injection, generated Zellij layout, editor resolution, and embedded session assets (`assets/`).
- `internal/mcpserver/`, `internal/agents/`: agent-driven file opening in the editor pane; subagent status pane.
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

Do not discard unrelated worktree changes or edit generated session assets directly; change their source under `assets/`.
