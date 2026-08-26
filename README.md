# qrouton 🟫

```text
              __________
             /  ·  *   /|
            / *   ·   / |
           /_________/  |
           |  ·   *  |  |
           | *     · |  /
           |  ·   *  | /
           |_________|/

              qrouton
```

qrouton turns a handful of GitHub repositories into one ready-to-use coding-agent workspace. Pick the repos, decide which are editing work and which are reference material, choose an agent, and qrouton handles mirrors, worktrees, branches, instructions, and the desktop workbench it all runs in. 🧊✨

## What it does 🥖

- Creates and resumes named multi-repo sessions.
- Reuses local bare mirrors instead of cloning everything repeatedly.
- Checks editing repos out on `<prefix>/<session-slug>` branches.
- Pins reference repos as detached, read-only context.
- Adds repositories to a live session from the workbench, on the branch it is already on.
- Discovers repositories across several GitHub organizations or user accounts.
- Starts from cached GitHub data, then refreshes owners concurrently.
- Opens a desktop window and runs Claude Code, Codex CLI, or OpenCode in it, with a shell tab alongside.
- Resumes the agent conversation when the workspace is resumed.
- Gives agents session-aware instructions, skills, and MCP tools that open tabs — an editor, a live command, a diff — while the conversation keeps the keyboard.
- Starts sessions in **RPI** (orchestrated Research → Plan → Implement) or **Assistant** (open-ended) mode.

## Requirements 🧰

- macOS or Linux. The workbench is a WebKit window, so the binary links cgo; there is no Windows build. Linux needs WebKitGTK 6 — `pacman -S webkitgtk-6.0` on Arch, `apt install libgtk-4-dev libwebkitgtk-6.0-dev` on Debian or Ubuntu. GTK4 only, so a distro new enough to carry it.
- Git.
- [GitHub CLI](https://cli.github.com/) authenticated with `gh auth login`, or `GITHUB_TOKEN`.
- `LINEAR_API_KEY` or `ASANA_ACCESS_TOKEN` to populate a new session's name and description from a linked ticket.
- At least one supported coding agent: `claude`, `codex`, or `opencode`.
- Go 1.26+ and Node 22+ to build from source — the workbench's Svelte frontend is built by `make front` before anything Go embeds it.

## Build and run 🚀

```sh
make build
./qrouton
```

`make install` puts the binary in `~/.local/bin` (override with `BINDIR=`). Worth doing: you switch a running session's mode with `qrouton mode` from its shell tab, so it wants to be somewhere your shell can find it. `make check` runs the whole pre-handoff gate.

qrouton does not ask for anything on first run — a session with no repositories needs neither a root nor GitHub owners, so the root defaults to `~/work`. Set `orgs` by hand before assembling your first session: with none, the repository list is empty. Configuration is stored at:

```text
$XDG_CONFIG_HOME/qrouton/config.json
# fallback: ~/.config/qrouton/config.json
```

Example:

```json
{
  "orgs": ["lifesum", "vimeda", "kieranajp"],
  "root": "~/work/qrouton",
  "editor": ["nvim", "+{line}", "{path}"]
}
```

`orgs` accepts both organizations and personal GitHub accounts. `editor` is optional; qrouton otherwise uses `$VISUAL`, `$EDITOR`, or a detected editor.

`launch` is optional too, and overrides a runner's command — keyed by runner id, with the exact argv to run:

```json
"launch": { "claude": ["claude", "--dangerously-skip-permissions", "--verbose"] }
```

It **replaces** the built-in command rather than adding to it, so an override that omits a flag removes it. The argv need not name the runner, which is how you point at a different build: `"claude": ["/opt/claude-beta/claude", "--dangerously-skip-permissions"]`. Omit the key entirely to get the default.

Useful flags:

```sh
./qrouton --runner codex     # preselect a supported coding agent
./qrouton --linear-issue LIF-2841
```

## Open Linear issues in qrouton 🔗

Linear Desktop can send **Work on issue → Custom script** to qrouton's existing New session flow. Open qrouton's Settings and save the prefilled **Linear custom script** field. If `~/.linear/coding-tools.json` does not exist, qrouton creates it with the running executable's absolute path:

```json
{
  "openIssue": {
    "path": "/Users/you/.local/bin/qrouton",
    "args": ["--linear-issue", "{{issue.identifier}}"],
    "env": ["LINEAR_PROMPT"]
  }
}
```

The field remains a plain JSON editor: an existing file is loaded verbatim, and qrouton validates it as a JSON object before writing it back.

The flag accepts an identifier such as `LIF-2841`, `https://linear.app/issue/LIF-2841`, or a workspace URL such as `https://linear.app/lifesum/issue/LIF-2841/title`. It validates and persists the workspace-free form `https://linear.app/issue/LIF-2841`. Repository roles, agent, mode, branch and Create remain interactive.

The generated `env` entry lets Linear pass its composed `LINEAR_PROMPT` alongside the issue. On the session's first launch, qrouton sends its own Assistant or RPI opening message first, followed by `Linear request:` and Linear's prompt. The prompt is consumed once and is not repeated on agent resume. Ticket fetching and session creation still happen later in the window.

The command succeeds when a running workbench accepts the request, or when a newly launched workbench is ready with the request queued. `LINEAR_API_KEY` must be present in the environment of the workbench process: a running workbench keeps the environment it started with, and Linear does not guarantee that a cold custom-script launch inherits the environment from your usual shell. qrouton does not store the key.

Repeated actions for the same issue reveal its open draft or preferred existing session. A different issue is refused while any New session draft is open, without replacing that draft. If a workbench from an older qrouton version is running, quit it and retry so the installed version can start.

To remove the integration, delete `openIssue` from `~/.linear/coding-tools.json` or point it back to the previous coding tool. To roll back the binary, quit the workbench before installing the earlier version; pasted Linear and Asana ticket URLs continue to work independently of this integration.

## Session shape 🗂️

```text
<root>/
├── .mirrors/<owner>/<repo>.git
└── <session>/
    ├── qrouton.json
    ├── src/
    │   ├── editing-repo/
    │   └── reference-repo/
    ├── thoughts/shared/
    └── .qrouton/
```

Each repository in the picker carries a three-way control: **Off**, **Editing**, **Reference**. Editing repositories are implementation targets, checked out on the session branch. Reference repositories are available for inspection but are explicitly marked read-only for the coding agent, and pinned to a commit.

## Session mode 🎛️

Each session starts in one of two modes, chosen on the new-session form (`RPI` is the default):

- **RPI** — the orchestrated Research → Plan → Implement workflow, with research/planning/implementation leads, ticket-blind specialists, and durable specs and plans.
- **Assistant** — an open-ended coding session: help directly, no forced workflow or artifacts.

RPI is qrouton's take on [loop engineering](https://newsletter.pragmaticengineer.com/p/what-is-loop-engineering): the interesting part isn't the prompt, it's the loop around it — the topology (leads fanning out to specialists), the verifier (review and test gates), and the stop rules (phased plans). Assistant mode is the honest other half of that idea: loops have preconditions, and plenty of work doesn't clear them. So you pick the loop when it earns its keep, and skip it when you're the verifier.

The mode only swaps the runner's starting system prompt and opening message; the workbench, MCP tools, and skills are identical either way. The choice is stored in `qrouton.json` and preserved on resume. Both prompts are always stamped under `.qrouton/qrspi/`, so an Assistant session can **escalate to RPI mid-conversation** just by asking the agent — no relaunch needed.

## Prompt sources 🧠

The workflow prompts are first-class source files under [`prompts/`](./prompts):

```text
prompts/
├── orchestrator.md   # RPI mode
├── assistant.md      # Assistant mode
├── skills/
└── agents/
```

A `prompts.PromptLoader` supplies these sources: the embedded loader ships them inside the qrouton binary, and the filesystem loader reads them from a directory, for tests, eval snapshots, and alternate prompt sets. Provider discovery files — Claude Markdown, Codex TOML — are rendered from the same canonical agent prompts.

Laying those rendered assets out on disk is `prompts.Stamp`, and both a session launch and a prompt eval call it. That is deliberate: an eval that graded a different discovery tree than sessions actually get would be grading the wrong thing.

## Development 🤖

```sh
make check
```

One entry point rather than bare `go` commands: the embedded asset tree is generated by `make front`, so a build, test, or vet run before it fails on an empty embed. `check` covers test, race, vet, build, and the frontend's own checks.

Robot contributors should read [`AGENTS.md`](./AGENTS.md). Humans are also allowed, especially if adequately caffeinated. ☕
