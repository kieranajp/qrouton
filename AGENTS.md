# qrouton agent guide

qrouton is a Go desktop app that assembles multi-repo workspaces from shared bare mirrors, then runs a coding agent in a terminal window it owns.

## Design directions

The invariants below are mechanical rules. These are the reasons they exist; when a change has to trade one thing off against another, trade in favour of these.

1. **qrouton owns the surface, not the conversation.** It may own a desktop process, PTYs and windows; it may not own the exchange between the user and the model. The runner is an opaque interactive child: no parsing model output, no deciding when work is done, no headless runs, no agent loop, no reimplemented chat UI. qrouton is a harness for the agent's harness, never a replacement for it.

2. **The repository group is discovered, not declared.** You do not know which repos a piece of work needs until you know the work. Sessions therefore start empty and acquire repositories at escalation, rather than requiring a curated group up front. This is the thing no comparable tool does.

3. **Orchestration is the product.** The agent you talk to is an orchestrator that stays lean by delegating; leads own the read-heavy and write-heavy work; specialists answer bounded questions. The workspace exists to make that tree *legible* — a surface per thing is how you watch several at once. Fan-out is concurrent, so the display must be concurrent. That surface may be a real OS window or a docked tab, provided an unfocused tab still reports its state; what this forbids is work stacked behind something that cannot say what it is doing.

   Two constraints keep the fan-out honest. **Depth is bounded, breadth is not:** three levels (orchestrator → lead → specialist), enforced by specialists declaring a read-only `tools:` set that excludes `Task`, so they cannot spawn. **Writes stay single-threaded:** extra agents add intelligence, not concurrent edits. More agents is not the goal; a wider fan-out that nobody can observe is how a run invents a datastore that does not exist.

4. **A collaborative workbench, not an autonomous one.** Human and agent share one surface and either may drive. Focus is borrowed, never taken: a window qrouton opens leaves the keyboard where it was, and the one case that needs it announces itself. Leaning in and stepping back are both cheap.

5. **No prior knowledge required.** Discoverability is a requirement, not polish. No chords, no prefixes, no modes to enter: a window with a title bar is driven by reflexes the user already has. Affordances live on the objects they operate, and the agent is expected to explain the workspace on request. The test is an engineer whose terminal use is `yarn dev`.

6. **Durable artifacts over live state.** Work survives context loss through documents and code, never through conversation. Phase is derived from what exists on disk. Prompts are re-stamped every launch. Clear beats compact.

## Architecture

Dependency direction: `config ← github ← ticket ← assembly` and `config ← github ← session ← assembly`; `launch`, `agents`, and `mcpserver` sit above the shared leaves. `assembly` sits where the old TUI did — above `session`, below `desktop` — and imports no display and no `launch`: it reaches the supervisor signal through a function field, because everything it imports is linked into the workbench with it. `workbench` carries the window port: `launch` and `mcpserver` reach the running workbench only through `WindowHost` and the `Handle` naming its control socket, so neither links a webview and the window tools keep a fakeable seam. `desktop` is the workbench itself — the Wails application, the conversation PTY, the window registry and the socket server — and it is the only package that imports Wails, imported only by `main.go`. That split is load-bearing: put the two together and every test that touches a window tool links WebKit through cgo.

The shared leaves import nothing of qrouton's own, so anything may depend on them: `sessionpaths` (a session's on-disk layout), `codex` (the Codex CLI's home, its config, and the nesting depth launch raises), `theme` (the palette and what each accent is for), `atomicfile` (whole-file replace and the advisory lock every multi-writer file is guarded by), and `prompts` (canonical prompts, provider rendering, and the discovery-tree stamper).

- `main.go`: urfave/cli app; the root action opens the workbench on the session last shown — or on no session at all, whose window is the assembly overlay — and re-execs the binary behind a hidden marker flag, so the workbench owns a process of its own and the prompt comes back. It takes no arguments: sessions are assembled in the window. Subcommands come from `cmd/*`.
- `cmd/mcp/`, `cmd/agentevent/`, `cmd/mode/`, `cmd/agent/`, `cmd/shell/`: `*cli.Command` definitions (flags only) delegating to `internal/*`.
- `internal/assembly/`: the rules a session is assembled by — the draft, its validation, the branch derivation, and the single atomic manifest write that adds repositories to a live session.
- `internal/session/`: manifest schema, editing/reference roles, mirrors, worktree lifecycle.
- `internal/github/`: authenticated owner discovery, cache, concurrent refresh. `Token()` is also the credential the GitHub ticket provider fetches with.
- `internal/ticket/`: ticket links, one file per provider behind a `Provider` interface and a host registry — Linear, Asana and GitHub. A provider owns its hostnames, its URL shape, the canonical URL a manifest persists and dedupe compares, the fragment a slug and branch are seeded from, and its own credential. Adding a fourth is one file and one entry in `providers`.
- `prompts/`: canonical workflow, skill, and agent prompts, provider rendering, and `Stamp` — the one implementation of the runner discovery tree, shared by launches and evals.
- `internal/launch/`: runner launch/resume arguments, MCP injection, the conversation and shell argv, the supervisor and its signal path, the workbench detach and its readiness wait, and editor resolution. Asset stamping delegates to `prompts.Stamp`; only the mode-to-discovery-file decision lives here. Everything qrouton knows about one coding agent — its command, its resume flag, how it takes a first prompt, and its MCP and hook wiring — is one `runnerSpec` entry; `builtinRunners` is derived from that table, so adding an agent is one literal and never an edit in four places.
- `internal/workbench/`: the window port (`WindowHost`, `WindowOptions`), the `Handle` that carries the control socket into the MCP child, and the socket client. No webview.
- `internal/desktop/`: the Wails application — the conversation PTY, the tab registry and its lifecycle rules, the control-socket server, and the embedded page. `Windows` is the one service the window tools and the page both reach a tab through, and it is that registry plus the terminal processes, the document poll and the diagram worker, each owning what only it touches. Window construction sits behind a `renderer` seam so everything else is testable without a display. The main conversation is the one renderer window; agent terminals and documents live in the session's tab strip. The page is a Vite + Svelte project under `frontend/`. The names it calls Go by are generated: `bridge/` reads the Go source and writes `frontend/src/lib/bridge/generated.js` — one constant per bound method, one per event, and the chrome defaults `status.Fields` implies — so the page never re-spells a name Go owns. `make front` runs the generator and builds the project into `assets/`; that tree is generated and is not in git.
- `internal/mcpserver/`, `internal/agentevent/`: the agent's window tools; and the decode half of the runner subagent hook, which `cmd/agentevent` turns into a lifecycle event and an attention state and pushes over the control socket. Nothing is written to disk: an event either reaches a live workbench or is dropped.
- `internal/sessionpaths/`, `internal/codex/`, `internal/theme/`, `internal/atomicfile/`: the shared leaves above.
- `internal/config/`: config file and XDG paths. `Load` never prompts and never fails for a missing value — a zero-repo session needs neither a root nor owners, so the root defaults and an empty owner list is simply an empty repository list. A workbench opening on no session asks for the owners and the sessions root once, then marks the config `welcomed`; one opening on a session never asks, so an install that always resumes stays unasked.
- `internal/lineartools/`: Linear's `coding-tools.json` — the starter document generated when the file is absent, the user's own text carried verbatim when it is present, and a save that leaves an unchanged file alone so their formatting survives it.
- `cmd/qrouton-eval/`, `internal/evalharness/`: standalone prompt-eval binary. It owns its own run loop, but takes the runner's MCP wiring from `launch` rather than rebuilding it, so an eval grades the agent a launch would produce.

## Invariants

- Subagent depth stays bounded at three levels. Lead prompts (`qrouton-*-lead`) omit `tools:` deliberately, so they inherit `Task` and can delegate; every specialist declares a read-only `tools:` set without `Task`, so it cannot. Adding or removing one `tools:` line silently changes the topology — treat it as an architectural change, not prompt editing.
- The runner has to *allow* that depth as well as the prompts asking for it. Codex defaults to one level, so `launch` raises it to `codex.RequiredMaxDepth` on every Codex launch, leaving a user's deeper setting alone. A runner whose nesting qrouton cannot raise cannot host a lead.
- Editing repos use session branches; reference repos are detached at pinned commits.
- Focus is never taken *from* the user. An agent surface is a tab and leaves the keyboard where it was. The main conversation is the sole desktop window. A surface the user asked for may take the keyboard.
- Mirrors live under `<root>/.mirrors`; session worktrees live under `<session>/src`.
- Write `qrouton.json` last. A directory without it must not appear resumable.
- Three processes write the manifest, so every change to an existing one goes through `session.UpdateManifest`, which holds an advisory lock across load, mutate and replace. It is not re-entrant: the lock is per open file description, so calling it from inside its own mutate self-deadlocks, and slow work (cloning, checkout) belongs outside it. `WriteManifest` composes a manifest from nothing on disk and takes no lock, which is what keeps it reachable from inside one.
- Never silently overwrite user-owned agent files; only replace qrouton-marked assets.
- One owner per fact: a path convention, a helper, or a piece of copy lives in exactly one place. `sessionpaths` owns the session layout, `codex` owns Codex's, and each package keeps its literals in `strings.go` and its sentinel errors in `errors.go` rather than inline.
- Eval and launch must stamp identical trees. Both go through `prompts.Stamp`; fixtures carry real `session.Manifest` documents, and the fixture-schema test fails if they drift.
- Preserve cache-first startup, cancellable refreshes, and runner conversation resume.

## Working agreement

- Keep changes small and match existing package-level style.
- Comments default to absent, in every language here. One line where earned, two for a real trap, never a paragraph. A comment earns its place only by saying what the code cannot: where the obvious reading is wrong, or why a non-obvious choice was made. Too many big ones drift and cry wolf until none get read — if a change comments most of the files it touches, delete the ones restating the code, the design system, or the plan. State what *is*, not the debugging journey. Never point at another file, symbol or line number.
- `make comment-check` mechanically caps standalone prose runs at four lines and rejects narrow journey phrases and unstable file pointers. It covers authored Go plus JavaScript, Svelte template/script/style, and CSS beneath the frontend; generated Go, the generated bridge, machine directives, dependencies, and local eval results are excluded. The shared root policy is the sole ratchet. Passing checks comment shape only — whether a comment is earned, plain, and near the one-line/two-line standard remains review judgment.
- New user-facing text goes in the package's `strings.go`; new failure modes get a sentinel in its `errors.go`, wrapped with `%w` at the call site.
- `gofmt` changed Go files.
- Add focused tests for behavior changes.
- Before handoff run:

```sh
GOCACHE=/tmp/qrouton-go-cache make check
```

  One entry point rather than four bare `go` commands: the embedded asset tree is generated, so a build, a test, or a vet run before `make front` fails on an empty embed. `check` covers test, race, vet, build, `gofmt -l` and `git diff --check`.

Do not discard unrelated worktree changes or edit generated session assets directly; change prompt sources under `prompts/`, launch support scripts under `internal/launch/scripts/`, or the workbench's frontend under `internal/desktop/frontend/`.
