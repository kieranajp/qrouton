package mux

// KDL layout rendering. Pure serialisation of a Workspace into Zellij's layout
// language — nothing here talks to a zellij process.

import (
	"fmt"
	"strconv"
	"strings"
)

// renderKDL serialises the workspace into a Zellij layout, topped with the
// status bar and named so the new session self-attaches. The bar goes on top
// because qrouton's own strip pane holds the bottom row.
func renderKDL(ws Workspace) string {
	var b strings.Builder
	b.WriteString(kdlLayoutOpen)
	b.WriteString(kdlBar)
	renderNode(&b, ws.Tiled, 1)
	b.WriteString(kdlBlockClose)
	b.WriteString(fmt.Sprintf(kdlSessionName, ws.Slug))
	return b.String()
}

func renderNode(b *strings.Builder, n Node, depth int) {
	pad := strings.Repeat(kdlIndent, depth)
	if n.Pane == nil {
		attrs := ""
		if n.Stacked {
			attrs = kdlStackedAttr
		} else if n.Split != "" {
			attrs = fmt.Sprintf(kdlSplitAttr, n.Split)
		}
		if s := kdlSize(n.Size); s != "" {
			attrs += kdlSizeAttr + s
		}
		b.WriteString(pad + kdlPaneKeyword + " " + attrs + " {\n")
		for _, child := range n.Children {
			renderNode(b, child, depth+1)
		}
		b.WriteString(pad + kdlBlockClose)
		return
	}
	attrs := ""
	if s := kdlSize(n.Size); s != "" {
		attrs += kdlSizeAttr + s
	}
	if n.Pane.Borderless {
		attrs += kdlBorderless
	}
	attrs += fmt.Sprintf(kdlNameAttr, n.Pane.Name)
	if n.Pane.CloseOnExit {
		attrs += kdlCloseOnExit
	}
	if n.Pane.Focus {
		attrs += kdlFocus
	}
	b.WriteString(pad + kdlPaneKeyword + attrs + " {\n")
	renderCommand(b, n.Pane.Command, depth+1)
	b.WriteString(pad + kdlBlockClose)
}

func renderCommand(b *strings.Builder, command []string, depth int) {
	if len(command) == 0 {
		return
	}
	pad := strings.Repeat(kdlIndent, depth)
	b.WriteString(pad + fmt.Sprintf(kdlCommandFormat, command[0]))
	if len(command) > 1 {
		quoted := make([]string, len(command)-1)
		for i, a := range command[1:] {
			quoted[i] = fmt.Sprintf("%q", a)
		}
		b.WriteString(pad + kdlArgsKeyword + strings.Join(quoted, " ") + "\n")
	}
}

// kdlSize renders a size hint: bare for row counts, quoted for percentages.
func kdlSize(size string) string {
	if size == "" {
		return ""
	}
	if _, err := strconv.Atoi(size); err == nil {
		return size
	}
	return fmt.Sprintf("%q", size)
}
