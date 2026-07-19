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

qrouton turns a handful of GitHub repositories into one ready-to-use coding-agent workspace. Pick the repos, decide which are active work and which are reference material, choose an agent, and qrouton handles mirrors, worktrees, branches, instructions, and the terminal layout. 🧊✨

## Screenshots 📸

<!-- Add onboarding, workspace, and editor-pane screenshots here. -->

_Screenshots coming soon._

## What it does 🥖

- Creates and resumes named multi-repo sessions.
- Reuses local bare mirrors instead of cloning everything repeatedly.
- Checks active repos out on `<prefix>/<session-slug>` branches.
- Pins reference repos as detached, read-only context.
- Discovers repositories across several GitHub organizations or user accounts.
- Starts from cached GitHub data, then refreshes owners concurrently.
- Launches Claude Code, Codex CLI, or OpenCode in Zellij.
- Resumes the agent conversation when the workspace is resumed.
- Gives agents session-aware instructions, skills, and an MCP-powered editor pane.

## Requirements 🧰

- macOS or Linux.
- Git.
- [GitHub CLI](https://cli.github.com/) authenticated with `gh auth login`, or `GITHUB_TOKEN`.
- `LINEAR_API_KEY` or `ASANA_ACCESS_TOKEN` to populate a new session's name and description from a linked ticket.
- Zellij 0.44 or newer.
- At least one supported coding agent: `claude`, `codex`, or `opencode`.
- Go 1.26+ to build from source.

## Build and run 🚀

```sh
go build -o qrouton .
./qrouton
```

On first run, qrouton asks for a workspace root and GitHub owners. Configuration is stored at:

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

Useful flags:

```sh
./qrouton --refresh          # ignore the repo cache for this launch
./qrouton --runner codex     # preselect a supported coding agent
```

## Session shape 🗂️

```text
<root>/
├── .mirrors/<owner>/<repo>.git
└── <session>/
    ├── qrouton.json
    ├── src/
    │   ├── active-repo/
    │   └── reference-repo/
    ├── thoughts/shared/
    └── .qrouton/
```

Press `Space` in the repository picker to cycle:

```text
excluded → active → reference → excluded
```

Active repositories are implementation targets. Reference repositories are available for inspection but are explicitly marked read-only for the coding agent.

## Prompt sources 🧠

The workflow prompts are first-class source files under [`prompts/`](./prompts):

```text
prompts/
├── orchestrator.md
├── skills/
└── agents/
```

The `prompts.PromptLoader` interface supplies these sources to both session launching and prompt evaluations. The embedded loader ships them in the qrouton binary; the filesystem loader supports tests, eval snapshots, and alternate prompt directories. Provider discovery files such as Claude Markdown and Codex TOML are rendered from the same canonical agent prompts.

## Development 🤖

```sh
go test ./...
go test -race ./...
go vet ./...
```

Robot contributors should read [`AGENTS.md`](./AGENTS.md). Humans are also allowed, especially if adequately caffeinated. ☕
