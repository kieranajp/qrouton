// Package share renders a session document as one self-contained page: the
// workbench's own prose renderer, its palette, its fonts and its diagrams laid
// out ahead of time, in a file that fetches nothing. Handing that page to
// anybody is the agent's job.
package share

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/diagram"
	md "github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/theme"
)

// Page assembles the page for one document. source names it for the reader and
// is relative to the session root.
func Page(source string, markdown []byte) ([]byte, error) {
	if len(bytes.TrimSpace(markdown)) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNoDocument, source)
	}
	style, err := assetFS.ReadFile(styleAsset)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoBundle, err)
	}
	script, err := assetFS.ReadFile(scriptAsset)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoBundle, err)
	}

	var page bytes.Buffer
	fmt.Fprintf(&page, titleFormat, html.EscapeString(Title(source, markdown)))
	fmt.Fprintf(&page, styleFormat, theme.CSS())
	fmt.Fprintf(&page, styleFormat, style)
	fmt.Fprintf(&page, payloadFormat, payload(source, markdown))
	if drawn := diagrams(markdown); drawn != "" {
		fmt.Fprintf(&page, diagramsFormat, drawn)
	}
	fmt.Fprintf(&page, scriptFormat, script)
	return page.Bytes(), nil
}

func Write(dir, source string, markdown []byte) (string, error) {
	page, err := Page(source, markdown)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return "", err
	}
	path := filepath.Join(dir, slug(source)+pageSuffix)
	if err := os.WriteFile(path, page, fileMode); err != nil {
		return "", err
	}
	return path, nil
}

// Title is the document's own name: the level-one heading it opens with, or the
// file's own name for a document that starts with anything else.
func Title(source string, markdown []byte) string {
	if title, ok := md.Title(string(markdown)); ok {
		return title
	}
	return filepath.Base(source)
}

// The document travels base64-encoded so that no markdown in it can close the
// script tag carrying it. Its first lines are the kind and source path, which
// the page uses for its identity and title treatment.
func payload(source string, markdown []byte) string {
	var document bytes.Buffer
	document.WriteString(status.DocumentKind(source))
	document.WriteString("\n")
	document.WriteString(source)
	document.WriteString("\n")
	document.Write(markdown)
	return base64.StdEncoding.EncodeToString(document.Bytes())
}

// rendered is one d2 fence as the page receives it, carrying the document line
// it was found on so the page can place it. Exactly one of SVG and Error is set.
type rendered struct {
	Line  int    `json:"line"`
	SVG   string `json:"svg,omitempty"`
	Error string `json:"error,omitempty"`
}

// diagrams lays the document's d2 fences out at publish time: a shared page
// runs no renderer, so every diagram has to travel with it. A fence that fails
// carries the reason instead, which the page states where the drawing would be.
func diagrams(markdown []byte) string {
	found := diagram.Scan(string(markdown))
	if len(found) == 0 {
		return ""
	}
	renderer := diagram.New(diagram.DefaultTimeout)
	defer renderer.Close()
	drawings := make([]rendered, 0, len(found))
	for _, fence := range found {
		out := renderer.Render(context.Background(), fence)
		if out.Err != nil {
			drawings = append(drawings, rendered{Line: out.Line, Error: out.Err.Error()})
			continue
		}
		drawings = append(drawings, rendered{Line: out.Line, SVG: out.SVG})
	}
	encoded, err := json.Marshal(drawings)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

// slug keeps the whole relative path in the filename, so two documents sharing
// a basename cannot overwrite one another.
func slug(source string) string {
	trimmed := strings.TrimSuffix(source, filepath.Ext(source))
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == filepath.Separator || r == '/' || r == '.' || r == ' '
	})
	if len(parts) == 0 {
		return strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	}
	return strings.Join(parts, slugSeparator)
}
