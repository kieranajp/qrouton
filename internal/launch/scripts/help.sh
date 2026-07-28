#!/bin/sh
# Colours mirror internal/tui/theme.go (Catppuccin Macchiato — the Zellij theme
# set in zellij-config.kdl) so this quick-reference panel matches the picker
# and the panes. True-colour escapes keep the hexes exact regardless of
# terminal theme.
#
# One global copy under the config dir, summonable three ways: the supervisor
# floats it at startup, Alt-? re-summons it, and the agent's help MCP tool
# opens it when a user sounds lost. So it reads ./qrouton.json itself for the
# mode tagline rather than having it baked in at stamp time, which is what
# keeps it correct immediately after an escalation. Every route sets this
# pane's cwd to the session directory. $1, when given, is the Codex depth
# warning; only the startup invocation passes it.
esc=$(printf '\033'); rst="${esc}[0m"; bold="${esc}[1m"
blue="${esc}[38;2;138;173;244m"
dim="${esc}[38;2;128;135;162m"
text="${esc}[38;2;202;211;245m"
green="${esc}[38;2;166;218;149m"
yellow="${esc}[38;2;238;212;159m"

# A grep/sed for the "mode" field is the right weight here: an unset,
# unknown, or unreadable mode all mean RPI, mirroring
# session.SessionMode.effective — manifests predating the field default to
# the RPI workflow, and a shell script has no business parsing JSON.
tagline="Coordinate here; delegate work to subagents."
mode=$(grep -o '"mode"[[:space:]]*:[[:space:]]*"[^"]*"' qrouton.json 2>/dev/null | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')
if [ "$mode" = "assistant" ]; then
    tagline="Open-ended session; ask to switch to RPI anytime."
fi

# key <name> <chord> [note] — one row of the reference. heading <text> — a
# section rule. Both exist so the rows below read as content, not as printf.
#
# printf pads to a width in *bytes*, so keep the two padded fields ASCII: one
# arrow glyph in a chord column silently eats three of its columns and knocks
# the rest of the row out of line. The note is last and unpadded, so anything
# goes there.
key() {
    printf '  %s%-16s%s %s%-13s%s %s%s%s\n' "$blue" "$1" "$rst" "$text" "$2" "$rst" "$dim" "$3" "$rst"
}
heading() {
    printf '\n  %s%s%s%s\n' "$bold" "$green" "$1" "$rst"
}

clear
printf '\n  %s%sqrouton%s   %s%s%s\n' "$bold" "$blue" "$rst" "$dim" "$tagline" "$rst"
if [ -n "$1" ]; then
    printf '\n  %sWARNING%s  %s%s%s\n' "$yellow" "$rst" "$dim" "$1" "$rst"
fi

heading "Moving around"
key "Move focus" "Alt-arrows" "← ↓ ↑ →, or Alt-h/j/k/l"
key "Cycle panes" "Alt-Tab"
key "Floating panes" "Alt-f" "show / hide the layer panes open on top"
key "Terminal" "Alt-g" "a floating shell in the session root"
key "Close pane" "Alt-x"
key "Resize" "Alt-+ / Alt--"
key "Scroll a pane" "Ctrl-g s" "then PageUp / PageDown; Esc when done"
key "This list" "Alt-?"
key "Quit" "Ctrl-g Ctrl-q"

heading "Switching workflow"
key "Escalate" "Alt-e" "hand the work to Research → Plan → Implement"
key "De-escalate" "Alt-n" "back to the open-ended assistant"

heading "The panes"
key "agent" "left" "the agent you are talking to"
key "shell" "top right" "a login shell, already in the session root"
key "repos / agents" "right" "checkout state and subagent activity"
key "status" "bottom row" "mode, phase, and the session's name"

heading "Ask the agent to"
printf '  %sshow me that file%s / %srun it in a pane%s / %sdiff what changed%s — it drives\n' "$text" "$rst" "$text" "$rst" "$text" "$rst"
printf '  %spanes of its own, so long-running output stays visible while you chat.%s\n' "$dim" "$rst"

heading "Where things live"
key "qrouton.json" "" "the manifest: repos, roles, branches, mode"
key "src/<repo>" "" "worktrees; active ones are yours to change"
key "thoughts/shared" "" "research, specs, and plans that outlive the chat"

printf '\n  %sPress any key to close%s\n' "$dim" "$rst"
# Read one raw keypress so Enter, Esc, or anything else dismisses the panel.
# A canonical-mode `read` only ever returned on Enter; every other key left
# this floating pane lingering over the workspace, swallowing input.
saved=$(stty -g 2>/dev/null)
stty -icanon -echo 2>/dev/null
dd bs=1 count=1 >/dev/null 2>&1
[ -n "$saved" ] && stty "$saved" 2>/dev/null
exit 0
