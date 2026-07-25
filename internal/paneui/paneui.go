// Package paneui draws qrouton's always-on watch panes — the repo and subagent
// status readouts. Those panes redraw forever, so they repaint in place rather
// than clearing the screen: a full clear makes the pane flash blank between
// frames. The escape sequences that achieve that, and the small vocabulary of
// styles the panes use, live here so neither watcher writes raw ANSI.
package paneui

import (
	"fmt"
	"strings"
)

const (
	cursorHome     = "\033[H"
	eraseToLineEnd = "\033[K"
	eraseBelow     = "\033[J"
	crlf           = "\r\n"

	reset = "\033[0m"

	boldOn = "\033[1m"
	dimOn  = "\033[2m"

	colorCyan  = "36"
	colorGreen = "32"
	colorRed   = "31"

	colorFormat = "\033[%sm"
)

// Frame renders lines as one in-place terminal frame: cursor home, erase to
// end-of-line per row, erase-to-end-of-screen at the bottom. Redrawing this way
// never flashes the pane blank.
func Frame(lines []string) string {
	var frame strings.Builder
	frame.WriteString(cursorHome)
	for _, line := range lines {
		frame.WriteString(line)
		frame.WriteString(eraseToLineEnd + crlf)
	}
	// Clear any rows a previous, longer frame left below this one.
	frame.WriteString(eraseBelow)
	return frame.String()
}

// Title styles a pane's heading.
func Title(text string) string { return boldOn + text + reset }

// Bold emphasises a value inside a line.
func Bold(text string) string { return boldOn + text + reset }

// Muted styles secondary text: absent state, counts, and error notes.
func Muted(text string) string { return dimOn + text + reset }

// Running, Done, and Failed style a status marker and its glyph.
func Running(text string) string { return colored(colorCyan, text) }
func Done(text string) string    { return colored(colorGreen, text) }
func Failed(text string) string  { return colored(colorRed, text) }

func colored(color, text string) string {
	return fmt.Sprintf(colorFormat, color) + text + reset
}
