if command -v tree >/dev/null 2>&1; then tree -L 2; else find . -maxdepth 2 -print; fi; exec "${SHELL:-/bin/sh}" -l
