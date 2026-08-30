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

minimum=${MINIMUM_VERSION:-0.0.0}
if ! printf '%s\n' "$minimum" | grep -Eq '^[0-9]+([.][0-9]+){0,2}$'; then
	printf 'MINIMUM_VERSION must be a numeric version such as 1.2.3 (got %s)\n' "$minimum" >&2
	exit 1
fi

archive="$dist/qrouton-$version-macos-universal.zip"
rm -f "$archive" "$dist/checksums.txt" "$dist/minimum-version.txt"
ditto -c -k --sequesterRsrc --keepParent "$app" "$archive"
(
	cd "$dist"
	shasum -a 256 "$(basename "$archive")" > checksums.txt
)
# The oldest version this release will talk to. Installs below it are held at
# the update gate rather than left to assemble a session against a build this
# release can no longer speak to.
printf '%s\n' "$minimum" > "$dist/minimum-version.txt"
printf 'Archived %s (floor %s)\n' "$archive" "$minimum"
