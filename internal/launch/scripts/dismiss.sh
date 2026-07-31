#!/bin/sh
# The one Esc-to-dismiss wait, shared by every pane qrouton floats over the
# workspace: the quick-reference panel, the notification toast, diffs, and the
# commands the agent runs on the user's behalf. It runs in the foreground while
# command/diff payloads run without terminal input, and each pane carries
# close_on_exit, so "this script returned" is precisely what closes a pane. One
# global copy under the config dir, beside help.sh.
#
# Why a script and not a Zellij keybinding: Zellij's bindings cannot express
# "close this pane, but only if it is one of qrouton's transient ones" — they
# are per-mode, never per-pane. qrouton used to emulate the missing predicate
# by flipping the whole session between input modes from a focus poller, which
# made Esc close the editor pane out from under nvim and gave up entirely with
# two clients attached. A pane that reads its own Esc needs no predicate: the
# key never reaches Zellij, permanent panes cannot be closed by it, and the
# number of attached clients stops mattering.
#
# $1, when given, is a timeout in whole seconds, after which the pane closes on
# its own — the toast's auto-dismiss. Without it the wait is indefinite. The
# deadline is whole-second granular, so the real wait is up to a second short of
# what was asked for; a toast is not worth sub-second bookkeeping.

seconds=${1:-}
deadline=
if [ -n "$seconds" ]; then
    deadline=$(($(date +%s) + seconds))
fi

saved=$(stty -g 2>/dev/null)

# arm sets the terminal up to read one byte. A timed wait polls in half-second
# ticks so the deadline below is checked regularly; an indefinite one blocks,
# costing nothing while the user reads the pane.
arm() {
    if [ -n "$deadline" ]; then
        stty -icanon -echo min 0 time 5 2>/dev/null
    else
        stty -icanon -echo min 1 time 0 2>/dev/null
    fi
}

# peek reads whatever is buffered already, without waiting for more.
peek() { stty -icanon -echo min 0 time 0 2>/dev/null; }

# byte prints the next input byte in octal, or nothing if none arrived. Octal
# rather than the raw byte so an empty read (a tick, or a closed stdin) stays
# distinguishable from a key that simply is not Esc.
byte() { dd bs=1 count=1 2>/dev/null | od -An -b | tr -d '[:space:]'; }

expired() {
    [ -n "$deadline" ] && [ "$(date +%s)" -ge "$deadline" ]
}

arm
while :; do
    case "$(byte)" in
    033)
        # Arrows and other special keys arrive as Esc followed by more bytes. A
        # byte already waiting means this was one of those sequences rather than
        # the user pressing Esc; its remainder is read and ignored by later
        # turns of this loop.
        peek
        rest=$(byte)
        arm
        [ -z "$rest" ] && break
        ;;
    "")
        # A timed wait's tick, or stdin has gone away — in which case no
        # keypress is ever coming and lingering would strand the pane.
        [ -z "$deadline" ] && break
        ;;
    esac
    expired && break
done

[ -n "$saved" ] && stty "$saved" 2>/dev/null
exit 0
