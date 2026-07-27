#!/bin/sh
# Colours mirror internal/tui/theme.go (Catppuccin Macchiato — the Zellij theme
# set in zellij-config.kdl) so this quick-reference panel matches the picker
# and the panes. True-colour escapes keep the hexes exact regardless of
# terminal theme.
#
# One global copy under the config dir, re-summonable via Alt-? as well as the
# startup floating pane — so it reads ./qrouton.json itself for the mode
# tagline rather than having it baked in at stamp time, which is what keeps it
# correct immediately after an escalation. Both Run blocks set this pane's cwd
# to the session directory. $1, when given, is the Codex depth warning; only
# the startup invocation passes it.
esc=$(printf '\033'); rst="${esc}[0m"; bold="${esc}[1m"
blue="${esc}[38;2;138;173;244m"
dim="${esc}[38;2;128;135;162m"
text="${esc}[38;2;202;211;245m"
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

clear
printf '\n  %s%sqrouton%s\n\n' "$bold" "$blue" "$rst"
printf '  %s%s%s\n\n' "$dim" "$tagline" "$rst"
if [ -n "$1" ]; then
    printf '  %sWARNING%s  %s%s%s\n\n' "$yellow" "$rst" "$dim" "$1" "$rst"
fi
printf '  %sMove focus%s       %sAlt-← ↓ ↑ →%s  (or Alt-h/j/k/l)\n' "$blue" "$rst" "$text" "$rst"
printf '  %sCycle panes%s      %sAlt-Tab%s\n' "$blue" "$rst" "$text" "$rst"
printf '  %sEscalate%s         %sAlt-e%s        open the picker\n' "$blue" "$rst" "$text" "$rst"
printf '  %sDe-escalate%s      %sAlt-n%s        back to assistant\n' "$blue" "$rst" "$text" "$rst"
printf '  %sTerminal%s         %sAlt-g%s        floating shell\n' "$blue" "$rst" "$text" "$rst"
printf '  %sFloating panes%s   %sAlt-f%s        show / hide\n' "$blue" "$rst" "$text" "$rst"
printf '  %sClose pane%s       %sAlt-x%s\n' "$blue" "$rst" "$text" "$rst"
printf '  %sResize%s           %sAlt-+ / Alt--%s\n' "$blue" "$rst" "$text" "$rst"
printf '  %sThis list%s        %sAlt-?%s\n' "$blue" "$rst" "$text" "$rst"
printf '  %sQuit%s             %sCtrl-g, then Ctrl-q%s\n\n' "$blue" "$rst" "$text" "$rst"
printf '  %sPress any key to close%s\n' "$dim" "$rst"
# Read one raw keypress so Enter, Esc, or anything else dismisses the panel.
# A canonical-mode `read` only ever returned on Enter; every other key left
# this floating pane lingering over the workspace, swallowing input.
saved=$(stty -g 2>/dev/null)
stty -icanon -echo 2>/dev/null
dd bs=1 count=1 >/dev/null 2>&1
[ -n "$saved" ] && stty "$saved" 2>/dev/null
exit 0
