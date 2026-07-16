#!/bin/sh
# qrouton: live per-repo branch + status (generated; regenerated at every launch)
cd "$(dirname "$0")/.." || exit 1
while :; do
  clear
  for g in src/*/.git */.git; do
    [ -e "$g" ] || continue
    r=${g%/.git}
    branch=$(git -C "$r" branch --show-current)
    dirty=$(git -C "$r" status --porcelain | wc -l | tr -d ' ')
    [ "$dirty" -eq 0 ] && state=clean || state="${dirty} changed"
    printf '\033[1m%s\033[0m  %s · %s\n' "$r" "$branch" "$state"
  done
  sleep 3
done
