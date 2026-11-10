package api

import (
	"strings"
	"unicode/utf8"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
)

func document(d core.Document) *qroutonv1.Document {
	out := &qroutonv1.Document{
		Text:          validUTF8(d.Text),
		Hash:          d.Hash,
		Format:        d.Format,
		Source:        d.Source,
		Path:          d.Path,
		Deck:          d.Deck,
		Tells:         d.Tells,
		AssetToken:    d.AssetToken,
		Line:          int32(d.Line),
		To:            int32(d.To),
		ViewportEpoch: uint32(d.ViewportEpoch),
		CurrentIndex:  int32(d.CurrentIndex),
		Revision:      uint32(d.Revision),
	}
	if d.Kind != "" {
		out.Kind = &d.Kind
	}
	for _, asset := range d.Assets {
		out.Assets = append(out.Assets, &qroutonv1.DocumentAsset{Source: asset.Source, Path: asset.Path, Url: asset.URL, Kind: asset.Kind})
	}
	return out
}

// validUTF8 replaces each invalid byte as encoding/json does, where
// strings.ToValidUTF8 would fold a run of them into one.
func validUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var out strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			out.WriteRune(utf8.RuneError)
		} else {
			out.WriteString(s[i : i+size])
		}
		i += size
	}
	return out.String()
}

func renderedDiagram(d core.RenderedDiagram) *qroutonv1.RenderedDiagram {
	return &qroutonv1.RenderedDiagram{Request: uint32(d.Request), Line: int32(d.Line), Svg: d.SVG, Error: d.Error}
}
