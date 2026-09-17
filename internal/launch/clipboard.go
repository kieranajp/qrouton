package launch

// AppleScript can name only a handful of image classes, so sips converts
// whatever the gallery accepts into the PNG it can name. The path arrives as an
// argument, so a name carrying a quote stays one name.
const copyImageScript = `set -e
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
sips -s format png "$1" --out "$dir/image.png" >/dev/null
osascript -e 'on run argv' -e 'set the clipboard to (read (POSIX file (item 1 of argv)) as «class PNGf»)' -e 'end run' "$dir/image.png"
`

// CopyImageArgv copies an image file to the clipboard as a picture, so it
// pastes into a chat or a ticket rather than as its own name.
func CopyImageArgv(path string) []string {
	return []string{defaultShell, shellCommandFlag, copyImageScript, shellArgv0, path}
}
