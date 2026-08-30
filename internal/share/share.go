// Package share renders a session document as one self-contained page: the
// workbench's own prose renderer, its palette and its fonts, in a file that
// fetches nothing. Handing that page to anybody is the agent's job — qrouton
// draws the surface and stays out of the conversation.
package share

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

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
