#!/bin/sh
clear
printf '\n  qrouton\n\n'
printf '  @@TAGLINE@@\n\n'
@@WARNING@@
printf '  Move   Alt + arrow keys\n'
printf '  Term   Alt-g (floating terminal)\n'
printf '  Quit   Ctrl-g, then Ctrl-q\n\n'
printf '  Press any key to begin\n'
# Read one raw keypress so Enter, Esc, or anything else dismisses the panel.
# A canonical-mode `read` only ever returned on Enter; every other key left
# this floating pane lingering over the workspace, swallowing input.
saved=$(stty -g 2>/dev/null)
stty -icanon -echo 2>/dev/null
dd bs=1 count=1 >/dev/null 2>&1
[ -n "$saved" ] && stty "$saved" 2>/dev/null
exit 0
