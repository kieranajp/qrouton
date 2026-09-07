#!/bin/sh
set -eu
export LC_ALL=C

fail() {
	printf 'Package verification failed: %s\n' "$*" >&2
	exit 1
}

[ "$#" -eq 1 ] || fail 'usage: verify.sh path/to/qrouton_<version>_amd64.deb'
[ -f "$1" ] || fail "package not found: $1"
directory=$(CDPATH= cd -- "$(dirname -- "$1")" && pwd)
archive=$(basename -- "$1")
package="$directory/$archive"
for tool in dpkg-deb desktop-file-validate readelf sha256sum docker; do
	command -v "$tool" >/dev/null 2>&1 || fail "required tool is missing: $tool"
done
docker info >/dev/null 2>&1 || fail 'Docker is unavailable; installation checks require a running Docker daemon.'

[ "$(dpkg-deb --field "$package" Package)" = qrouton ] || fail 'unexpected package name'
[ "$(dpkg-deb --field "$package" Architecture)" = amd64 ] || fail 'expected amd64 architecture'
version=$(dpkg-deb --field "$package" Version)
printf '%s\n' "$version" | grep -Eq '^[0-9]+([.][0-9]+){0,2}$' || fail 'invalid package version'
[ "$archive" = "qrouton_${version}_amd64.deb" ] || fail 'filename does not match package version'
[ "$(dpkg-deb --field "$package" Section)" = devel ] || fail 'unexpected package section'
[ "$(dpkg-deb --field "$package" Priority)" = optional ] || fail 'unexpected package priority'
[ "$(dpkg-deb --field "$package" Homepage)" = https://github.com/kieranajp/qrouton ] || fail 'unexpected homepage'
for field in Description Depends; do
	[ -n "$(dpkg-deb --field "$package" "$field")" ] || fail "empty metadata field: $field"
done
[ "$(dpkg-deb --field "$package" Maintainer)" = 'Kieran Patel <me@kieranajp.co.uk>' ] || fail 'unexpected maintainer'
installed_size=$(dpkg-deb --field "$package" Installed-Size)
printf '%s\n' "$installed_size" | grep -Eq '^[1-9][0-9]*$' || fail 'invalid installed size'
depends=$(dpkg-deb --field "$package" Depends)
printf '%s\n' "$depends" | grep -Eq '(^|, )git(,|$)' || fail 'git dependency is missing'
printf '%s\n' "$depends" | grep -Eq '(^|, )ca-certificates(,|$)' || fail 'ca-certificates dependency is missing'
checksum="$directory/checksums-linux-amd64.txt"
[ -f "$checksum" ] || fail 'Linux checksum file is missing'
expected=$(cd "$directory" && sha256sum "$archive")
[ "$(cat "$checksum")" = "$expected" ] || fail 'checksum must cover exactly the supplied package'
(cd "$directory" && sha256sum -c checksums-linux-amd64.txt)

work=$(mktemp -d "${TMPDIR:-/tmp}/qrouton-deb-verify.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
dpkg-deb --fsys-tarfile "$package" > "$work/payload.tar"
tar -tf "$work/payload.tar" | sort > "$work/actual"
printf '%s\n' ./ ./usr/ ./usr/bin/ ./usr/bin/qrouton \
	./usr/share/ ./usr/share/applications/ ./usr/share/applications/qrouton.desktop \
	./usr/share/icons/ ./usr/share/icons/hicolor/ ./usr/share/icons/hicolor/scalable/ \
	./usr/share/icons/hicolor/scalable/apps/ ./usr/share/icons/hicolor/scalable/apps/qrouton.svg \
	./usr/share/doc/ ./usr/share/doc/qrouton/ ./usr/share/doc/qrouton/licenses/ \
	./usr/share/doc/qrouton/licenses/OFL.txt ./usr/share/doc/qrouton/licenses/LICENSE-NerdFonts \
	./usr/share/doc/qrouton/licenses/README.md | sort > "$work/expected"
diff -u "$work/expected" "$work/actual" || fail 'unexpected package contents'
tar --numeric-owner -tvf "$work/payload.tar" > "$work/modes"
awk '$2 != "0/0" { exit 1 }
	$NF ~ /\/$/ { if ($1 != "drwxr-xr-x") exit 1; next }
	$NF == "./usr/bin/qrouton" { if ($1 != "-rwxr-xr-x") exit 1; next }
	$1 != "-rw-r--r--" { exit 1 }' \
	"$work/modes" || fail 'payload must have root ownership and standard file modes'
dpkg-deb --ctrl-tarfile "$package" > "$work/control.tar"
tar -tf "$work/control.tar" | sort > "$work/actual"
printf '%s\n' ./ ./control > "$work/expected"
diff -u "$work/expected" "$work/actual" || fail 'unexpected package control files'
dpkg-deb --extract "$package" "$work/root"
binary="$work/root/usr/bin/qrouton"
[ -x "$binary" ] || fail 'binary is not executable'
readelf -h "$binary" > "$work/elf"
grep -Eq 'Class: +ELF64' "$work/elf" || fail 'binary is not ELF64'
grep -Eq 'Machine: +Advanced Micro Devices X86-64' "$work/elf" || fail 'binary is not amd64'
desktop="$work/root/usr/share/applications/qrouton.desktop"
desktop-file-validate "$desktop"
for entry in Exec=qrouton Icon=qrouton Terminal=false; do
	grep -Fx "$entry" "$desktop" >/dev/null || fail "desktop entry is missing $entry"
done
for notice in OFL.txt LICENSE-NerdFonts README.md; do
	[ -s "$work/root/usr/share/doc/qrouton/licenses/$notice" ] || fail "font notice is empty: $notice"
done

docker run --rm --platform linux/amd64 \
	--mount "type=bind,source=$package,target=/package.deb,readonly" \
	-e DEBIAN_FRONTEND=noninteractive ubuntu:24.04 sh -ec '
		apt-get update
		depends=$(dpkg-deb --field /package.deb Depends)
		apt-get satisfy -y --no-install-recommends "$depends"
		dpkg -i /package.deb
		test "$(dpkg-query -W -f="\${Status}" qrouton)" = "install ok installed"
		/usr/bin/qrouton --help
		dpkg -i /package.deb
		test "$(dpkg-query -W -f="\${Status}" qrouton)" = "install ok installed"
		/usr/bin/qrouton --help
		dpkg --remove qrouton
		test ! -e /usr/bin/qrouton
	' || fail 'isolated dependency resolution or install/reinstall/remove check failed'
printf 'Verified %s, including isolated install, reinstall and removal.\n' "$archive"
