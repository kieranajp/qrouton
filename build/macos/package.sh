#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
dist="$root/dist"
app="$dist/qrouton.app"
version=${VERSION:-0.1.0}
version=${version#v}
build_number=${BUILD_NUMBER:-1}
deployment_target=${MACOSX_DEPLOYMENT_TARGET:-12.0}
sign_identity=${SIGN_IDENTITY:--}

# The updater compares this against the release feed, so an unstamped bundle
# would report itself a working tree and never update.
version_package=github.com/kieranajp/qrouton/internal/version

if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+([.][0-9]+){0,2}$'; then
	printf 'VERSION must be a numeric macOS version such as 1.2.3 (got %s)\n' "$version" >&2
	exit 1
fi
if ! printf '%s\n' "$build_number" | grep -Eq '^[0-9]+$'; then
	printf 'BUILD_NUMBER must be numeric (got %s)\n' "$build_number" >&2
	exit 1
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/qrouton-package.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
staged="$work/qrouton.app"
contents="$staged/Contents"
mkdir -p "$contents/MacOS" "$contents/Resources"

build_arch() {
	goarch=$1
	clang_arch=$2
	(
		cd "$root"
		CGO_ENABLED=1 GOOS=darwin GOARCH="$goarch" \
			MACOSX_DEPLOYMENT_TARGET="$deployment_target" \
			CGO_CFLAGS="-mmacosx-version-min=$deployment_target -arch $clang_arch" \
			CGO_LDFLAGS="-mmacosx-version-min=$deployment_target -arch $clang_arch" \
			go build -tags production -trimpath \
				-ldflags="-w -s -X $version_package.Current=$version" \
				-o "$work/qrouton-$goarch" .
	)
}

build_arch arm64 arm64
build_arch amd64 x86_64
lipo -create -output "$contents/MacOS/qrouton" "$work/qrouton-arm64" "$work/qrouton-amd64"

iconset="$work/qrouton.iconset"
mkdir -p "$iconset"
sips -s format png "$root/docs/brand/logo-mark.svg" --out "$work/appicon.png" >/dev/null
for size in 16 32 128 256 512; do
	double=$((size * 2))
	sips -z "$size" "$size" "$work/appicon.png" --out "$iconset/icon_${size}x${size}.png" >/dev/null
	sips -z "$double" "$double" "$work/appicon.png" --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$contents/Resources/qrouton.icns"

cp "$root/build/macos/Info.plist" "$contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $version" "$contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $build_number" "$contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :LSMinimumSystemVersion $deployment_target" "$contents/Info.plist"

if [ "$sign_identity" = "-" ]; then
	codesign --force --sign - "$staged"
else
	codesign --force --options runtime --timestamp --sign "$sign_identity" "$staged"
fi
codesign --verify --deep --strict --verbose=2 "$staged"

mkdir -p "$dist"
rm -rf "$root/dist/qrouton.app"
mv "$staged" "$app"
printf 'Built %s (%s, universal arm64 + x86_64)\n' "$app" "$version"
