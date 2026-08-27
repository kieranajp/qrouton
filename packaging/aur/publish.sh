#!/bin/sh
set -eu

# Points packaging/aur at a released tag and, with --push, sends it to the AUR.
# Arch-only: makepkg owns the .SRCINFO format and updpkgsums owns the source
# checksum, so neither is reimplemented here.

usage() {
	printf 'usage: %s [--push] <version>\n' "$0" >&2
	exit 2
}

push=no
version=
while [ $# -gt 0 ]; do
	case $1 in
	--push) push=yes ;;
	-*) usage ;;
	*)
		[ -z "$version" ] || usage
		version=$1
		;;
	esac
	shift
done
[ -n "$version" ] || usage

version=${version#v}
if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+([.][0-9]+){0,2}$'; then
	printf 'version must be numeric such as 1.2.3 (got %s)\n' "$version" >&2
	exit 1
fi

# makepkg refuses to run as root, so the release workflow builds a user first.
if [ "$(id -u)" = 0 ]; then
	printf 'run this as an unprivileged user: makepkg refuses root\n' >&2
	exit 1
fi
for tool in makepkg updpkgsums git; do
	command -v "$tool" >/dev/null 2>&1 || {
		printf '%s is required; run this on Arch (base-devel, pacman-contrib)\n' "$tool" >&2
		exit 1
	}
done

cd -- "$(dirname -- "$0")"

sed -i "s/^pkgver=.*/pkgver=$version/; s/^pkgrel=.*/pkgrel=1/" PKGBUILD
updpkgsums
makepkg --printsrcinfo > .SRCINFO
printf 'packaging/aur now describes v%s\n' "$version"

[ "$push" = yes ] || exit 0

: "${AUR_SSH_PRIVATE_KEY:?set AUR_SSH_PRIVATE_KEY to the key registered with your AUR account}"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
key="$work/aur.key"
printf '%s\n' "$AUR_SSH_PRIVATE_KEY" > "$key"
chmod 600 "$key"
export GIT_SSH_COMMAND="ssh -i $key -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=$work/known_hosts"

git clone --quiet ssh://aur@aur.archlinux.org/qrouton.git "$work/aur"
cp PKGBUILD .SRCINFO "$work/aur/"
cd "$work/aur"
# The first push claims the package name, and clones an empty repository to do
# it — so untracked files count as a change, and the branch is named explicitly.
git add -A
if [ -z "$(git status --porcelain)" ]; then
	printf 'the AUR already carries this PKGBUILD; nothing to push\n'
	exit 0
fi
git -c "user.name=${AUR_MAINTAINER_NAME:-qrouton release}" \
	-c "user.email=${AUR_MAINTAINER_EMAIL:-noreply@users.noreply.github.com}" \
	commit --quiet -m "qrouton $version"
git push --quiet origin HEAD:master
printf 'pushed qrouton %s to the AUR\n' "$version"
