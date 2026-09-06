#!/bin/sh
set -eu
umask 022

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
version=${VERSION:-0.1.0}
version=${version#v}

if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+([.][0-9]+){0,2}$'; then
	printf 'VERSION must be numeric, such as 1.2.3 (got %s)\n' "$version" >&2
	exit 1
fi
if [ "$(uname -s)" != Linux ] || [ "$(uname -m)" != x86_64 ]; then
	printf 'Debian packaging requires Ubuntu 24.04 amd64.\n' >&2
	exit 1
fi
. /etc/os-release
if [ "$ID" != ubuntu ] || [ "$VERSION_ID" != 24.04 ]; then
	printf 'Debian packaging requires Ubuntu 24.04 amd64.\n' >&2
	exit 1
fi
if [ "${1:-}" = --check ]; then
	exit 0
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/qrouton-deb.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
staged="$work/debian/qrouton"
mkdir -p "$staged/usr/bin" "$staged/usr/share/applications" \
	"$staged/usr/share/icons/hicolor/scalable/apps" "$staged/usr/share/doc/qrouton/licenses"
(
	cd "$root"
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		go build -tags production -trimpath -ldflags='-w -s' -o "$staged/usr/bin/qrouton" .
)
chmod 755 "$staged/usr/bin/qrouton"
install -m 644 "$root/build/linux/qrouton.desktop" "$staged/usr/share/applications/qrouton.desktop"
install -m 644 "$root/docs/brand/logo-mark.svg" "$staged/usr/share/icons/hicolor/scalable/apps/qrouton.svg"
fonts="$root/internal/desktop/frontend/src/tokens/nerd-font"
for notice in OFL.txt LICENSE-NerdFonts README.md; do
	install -m 644 "$fonts/$notice" "$staged/usr/share/doc/qrouton/licenses/$notice"
done

mkdir -p "$work/debian"
maintainer='Kieran Patel <me@kieranajp.co.uk>'
printf 'Source: qrouton\nSection: devel\nPriority: optional\nMaintainer: %s\n\nPackage: qrouton\nArchitecture: amd64\nDescription: Multi-repo agent workspaces\n' "$maintainer" > "$work/debian/control"
depends=$(
	cd "$work"
	dpkg-shlibdeps -O -e"$staged/usr/bin/qrouton"
)
depends=${depends#shlibs:Depends=}
if [ -z "$depends" ]; then
	printf 'No shared-library dependencies found.\n' >&2
	exit 1
fi
installed_size=$(du -sk "$staged/usr" | cut -f1)
mkdir -p "$staged/DEBIAN"
printf 'Package: qrouton\nVersion: %s\nArchitecture: amd64\nSection: devel\nPriority: optional\nMaintainer: %s\nHomepage: https://github.com/kieranajp/qrouton\nInstalled-Size: %s\nDepends: %s, git, ca-certificates\nDescription: Multi-repo agent workspaces\n A desktop workbench for coding agents and shared repository worktrees.\n' \
	"$version" "$maintainer" "$installed_size" "$depends" > "$staged/DEBIAN/control"
chmod 755 "$staged/DEBIAN"
chmod 644 "$staged/DEBIAN/control"
archive="qrouton_${version}_amd64.deb"
dpkg-deb --root-owner-group --build "$staged" "$work/$archive"
(
	cd "$work"
	sha256sum "$archive" > checksums-linux-amd64.txt
)
mkdir -p "$root/dist"
mv "$work/$archive" "$root/dist/$archive"
mv "$work/checksums-linux-amd64.txt" "$root/dist/checksums-linux-amd64.txt"
printf 'Built %s\n' "$root/dist/$archive"
