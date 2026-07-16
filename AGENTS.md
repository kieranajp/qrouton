# qrouton agent guide

qrouton is a Go terminal app that assembles multi-repo workspaces from shared bare mirrors, then launches a coding agent inside a generated Zellij session.

## Architecture

- `tui.go`: fullscreen Bubble Tea onboarding and async UI state.
- `session.go`: manifest schema, active/reference roles, worktree lifecycle.
- `github.go`: authenticated owner discovery, cache, concurrent refresh.
- `runner.go`: Claude/Codex/OpenCode launch and resume arguments, MCP injection.
- `layout.go`: generated Zellij layout, status pane, shell bootstrap.
- `mcp.go`, `editor.go`: agent-driven file opening in the editor pane.
- `assets/`, `assets.go`: canonical session instructions/skills and generated symlinks.

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
