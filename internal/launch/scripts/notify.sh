#!/bin/sh
# qrouton: cross-platform attention sound (generated; regenerated at every launch).
# Tries macOS, PipeWire/PulseAudio, then Windows interop, only using a player
# when its sound file exists, and falls back to the terminal bell.
play() { command -v "$1" >/dev/null 2>&1 && [ -r "$2" ] && { "$1" "$2" >/dev/null 2>&1 & exit 0; }; }
play afplay /System/Library/Sounds/Glass.aiff
for f in /usr/share/sounds/freedesktop/stereo/complete.oga /usr/share/sounds/freedesktop/stereo/bell.oga; do
  play pw-play "$f"
  play paplay "$f"
done
if command -v powershell.exe >/dev/null 2>&1; then
  powershell.exe -c "[console]::beep(880,200)" >/dev/null 2>&1 & exit 0
fi
printf '\a' > /dev/tty 2>/dev/null || printf '\a'
