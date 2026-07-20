#!/bin/sh
# Colours mirror internal/tui/theme.go (Catppuccin Macchiato — the Zellij theme
# set in zellij-config.kdl) so this quick-start panel matches the picker and the
# panes. True-colour escapes keep the hexes exact regardless of terminal theme.
esc=$(printf '\033'); rst="${esc}[0m"; bold="${esc}[1m"
blue="${esc}[38;2;138;173;244m"
dim="${esc}[38;2;128;135;162m"
text="${esc}[38;2;202;211;245m"
yellow="${esc}[38;2;238;212;159m"
clear
printf '\n  %s%sqrouton%s\n\n' "$bold" "$blue" "$rst"
printf '  %s@@TAGLINE@@%s\n\n' "$dim" "$rst"
@@WARNING@@
printf '  %sMove%s   %sAlt + arrow keys%s\n' "$blue" "$rst" "$text" "$rst"
printf '  %sTerm%s   %sAlt-g (floating terminal)%s\n' "$blue" "$rst" "$text" "$rst"
printf '  %sQuit%s   %sCtrl-g, then Ctrl-q%s\n\n' "$blue" "$rst" "$text" "$rst"
printf '  %sPress any key to begin%s\n' "$dim" "$rst"
# Read one raw keypress so Enter, Esc, or anything else dismisses the panel.
# A canonical-mode `read` only ever returned on Enter; every other key left
# this floating pane lingering over the workspace, swallowing input.
saved=$(stty -g 2>/dev/null)
stty -icanon -echo 2>/dev/null
dd bs=1 count=1 >/dev/null 2>&1
[ -n "$saved" ] && stty "$saved" 2>/dev/null
exit 0
