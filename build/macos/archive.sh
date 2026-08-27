#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
dist="$root/dist"
app="$dist/qrouton.app"
version=${VERSION:-0.1.0}
version=${version#v}

if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+([.][0-9]+){0,2}$'; then
	printf 'VERSION must be a numeric macOS version such as 1.2.3 (got %s)\n' "$version" >&2
	exit 1
fi
if [ ! -d "$app" ]; then
	printf 'Build %s first with make app\n' "$app" >&2
	exit 1
fi

archive="$dist/qrouton-$version-macos-universal.zip"
rm -f "$archive" "$dist/checksums.txt"
ditto -c -k --sequesterRsrc --keepParent "$app" "$archive"
(
	cd "$dist"
	shasum -a 256 "$(basename "$archive")" > checksums.txt
)
printf 'Archived %s\n' "$archive"
