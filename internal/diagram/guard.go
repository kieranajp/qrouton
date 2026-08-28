package diagram

import (
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"strings"
)

// The SVG goes straight into a webview with no content policy. d2's output is
// well-formed XML and an injection through it is not, so the guard decodes.
func guard(svg string) error {
	decoder := xml.NewDecoder(strings.NewReader(svg))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return phrased{message: malformedError, err: fmt.Errorf("%w: %w", ErrMalformedSVG, err)}
		}
		switch token := token.(type) {
		case xml.StartElement:
			if err := element(token); err != nil {
				return err
			}
		case xml.ProcInst:
			return refuse(unknownElementError + token.Target)
		case xml.Directive:
			return refuse(unknownElementError + directive)
		}
	}
}

func element(start xml.StartElement) error {
	name := start.Name.Local
	switch strings.ToLower(name) {
	case scriptElement:
		return ErrEmbeddedScript
	case foreignObjectElement:
		return ErrEmbeddedMarkup
	}
	if !allowedElements[name] {
		return refuse(unknownElementError + name)
	}
	for _, attr := range start.Attr {
		if err := attribute(name, attr); err != nil {
			return err
		}
	}
	return nil
}

func attribute(element string, attr xml.Attr) error {
	name := attr.Name.Local
	if strings.HasPrefix(strings.ToLower(name), eventPrefix) {
		return ErrEmbeddedScript
	}
	if attr.Name.Space == xmlnsSpace {
		return nil
	}
	if !allowedAttributes[name] && !strings.HasPrefix(name, dataPrefix) {
		return refuse(unknownAttributeError + name)
	}
	if name != hrefAttribute {
		return nil
	}
	return link(element, normalise(attr.Value))
}

func link(element, href string) error {
	switch element {
	case imageElement:
		if !strings.HasPrefix(href, dataScheme) {
			return ErrRemoteImage
		}
	case anchorElement:
		if !strings.HasPrefix(href, httpScheme) && !strings.HasPrefix(href, httpsScheme) {
			return ErrUnsafeLink
		}
	case useElement:
		// A use may only clone something the document already carries.
		if !strings.HasPrefix(href, fragmentPrefix) {
			return ErrUnsafeLink
		}
	default:
		return ErrUnsafeLink
	}
	return nil
}

// normalise reads an href as a browser would. Anything less lets
// `&#106;ava&#9;script:` through.
func normalise(target string) string {
	target = html.UnescapeString(target)
	target = strings.Map(func(r rune) rune {
		if r <= ' ' {
			return -1
		}
		return r
	}, target)
	return strings.ToLower(target)
}

func refuse(message string) error {
	return phrased{message: message, err: ErrUnknownConstruct}
}

// Elements d2 emits, collected by rendering a corpus of every feature of it
// qrouton draws. Excluding the rest is what closes <img>, <iframe> and the
// foreign-content breakout family without anyone predicting the next one.
var allowedElements = set(
	"a", "circle", "clipPath", "defs", "ellipse",
	"feBlend", "feComposite", "feFlood", "feGaussianBlur", "feOffset", "filter",
	"g", "image", "line", "linearGradient", "marker", "mask", "path", "pattern",
	"polygon", "polyline", "radialGradient", "rect", "stop", "style", "svg",
	"text", "title", "tspan", "use",
)

// Attributes those elements carry, from the same corpus and from what d2 and
// its MathJax port can write; data- names are inert and allowed by prefix.
var allowedAttributes = set(
	"aria-label", "background-color", "class", "clip-path", "color", "cx", "cy",
	"d", "display", "displayAlign", "dx", "dy", "fill", "fill-opacity", "filter",
	"flood-color", "flood-opacity", "focusable", "font-style", "font-weight",
	"height", "href", "id", "in", "in2", "marker-end", "marker-start",
	"markerHeight", "markerUnits", "markerWidth", "mask", "maskUnits", "mode",
	"offset", "opacity", "operator", "orient", "patternContentUnits",
	"patternUnits", "points", "pointer-events", "preserveAspectRatio", "r",
	"refX", "refY", "requiredFeatures", "result", "role", "rx", "ry",
	"stdDeviation", "stop-color", "stop-opacity", "stroke", "stroke-dasharray",
	"stroke-linecap", "stroke-linejoin", "stroke-width", "style", "transform",
	"type", "viewBox", "viewbox", "width", "x", "x1", "x2", "xmlns", "y", "y1",
	"y2",
)

func set(names ...string) map[string]bool {
	made := make(map[string]bool, len(names))
	for _, name := range names {
		made[name] = true
	}
	return made
}
