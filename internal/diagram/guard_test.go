package diagram

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

const onePixelPNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

// corpus is the d2 the allow-lists were derived from: every feature that puts a
// new element or attribute in the output. A guard that refuses one of these has
// broken diagrams nobody wrote to attack it.
func corpus() map[string]string {
	return map[string]string{
		"shapes and an edge": "a -> b: edge\nc\n",

		"every stock shape": `rectangle: {shape: rectangle}
square: {shape: square}
page: {shape: page}
parallelogram: {shape: parallelogram}
document: {shape: document}
cylinder: {shape: cylinder}
queue: {shape: queue}
package: {shape: package}
step: {shape: step}
callout: {shape: callout}
stored_data: {shape: stored_data}
person: {shape: person}
diamond: {shape: diamond}
oval: {shape: oval}
circle: {shape: circle}
hexagon: {shape: hexagon}
cloud: {shape: cloud}
text: {shape: text}
`,

		"a class shape": `Parser: {
  shape: class
  +reader: io.Reader
  -lexer: Lexer
  +parse(source): (Node, error)
  #depth: int
}
`,

		"a sql table": `users: {
  shape: sql_table
  id: int {constraint: primary_key}
  email: varchar {constraint: unique}
  org: int {constraint: foreign_key}
}
orgs: {
  shape: sql_table
  id: int {constraint: primary_key}
}
users.org -> orgs.id
`,

		"an inline image shape": `img: {
  shape: image
  icon: "` + onePixelPNG + `"
}
`,

		"an icon beside a label": `node: Labelled {
  icon: "` + onePixelPNG + `"
  icon.near: top-left
}
`,

		"containers four deep": `a: Outer {
  b: {
    c: {
      d: {
        e: Leaf
      }
    }
  }
}
z -> a.b.c.d.e
a.b -> a.b.c
`,

		"edge styles and arrowheads": `a -> b: dashed {style.stroke-dash: 4}
c -> d: bold {style.bold: true; style.stroke-width: 4}
e -> f: animated {style.animated: true}
g <-> h: both {
  source-arrowhead.shape: diamond
  target-arrowhead: {shape: arrow; style.filled: true}
}
i -> j: {target-arrowhead.shape: cf-many-required}
k -> l: {source-arrowhead: {shape: cf-one; label: 1}}
m -- n: undirected
`,

		"every arrowhead": `a1 -> b1: {target-arrowhead.shape: arrow}
a2 -> b2: {target-arrowhead.shape: triangle}
a4 -> b4: {target-arrowhead.shape: diamond}
a5 -> b5: {target-arrowhead: {shape: diamond; style.filled: true}}
a6 -> b6: {target-arrowhead.shape: circle}
a7 -> b7: {target-arrowhead: {shape: circle; style.filled: true}}
a8 -> b8: {target-arrowhead.shape: cross}
a9 -> b9: {target-arrowhead.shape: box}
a10 -> b10: {target-arrowhead: {shape: box; style.filled: true}}
a12 -> b12: {target-arrowhead.shape: none}
a13 -> b13: {source-arrowhead.shape: cf-one; target-arrowhead.shape: cf-many}
a14 -> b14: {source-arrowhead.shape: cf-one-required; target-arrowhead.shape: cf-many-required}
`,

		"gradient fills": `linear: {style.fill: "linear-gradient(#8aadf4, #a6da95)"}
radial: {style.fill: "radial-gradient(circle, #8aadf4 0%, #a6da95 100%)"}
linear -> radial
`,

		"shape styles": `three: {style: {3d: true; fill: "#8aadf4"}}
many: {style.multiple: true}
shady: {style.shadow: true}
double: {style.double-border: true}
dots: {style.fill-pattern: dots}
lines: {style.fill-pattern: lines}
grain: {style.fill-pattern: grain}
paper: {style.fill-pattern: paper}
round: {style.border-radius: 8}
faint: {style.opacity: 0.4}
dashed: {style.stroke-dash: 3}
typed: Fancy {style: {italic: true; underline: true; bold: true; font-size: 28}}
`,

		"classes and class": `classes: {
  balancer: {
    shape: circle
    style.multiple: true
  }
  unhealthy: {
    style: {
      fill: "#ed8796"
      stroke-dash: 5
    }
  }
}
web: {class: balancer}
db: {class: [balancer; unhealthy]}
web -> db
`,

		"a tooltip and a link": `x: {tooltip: What this box is for}
y: {link: https://example.com}
x -> y: {tooltip: and on an edge}
`,

		"a code block": `impl: |go
  package main

  func main() {
    println("hello")
  }
|
impl -> caller
`,

		"latex": `plus: |latex
  \sum_{i=0}^{n} i^2
|
fraction: |latex
  \frac{\partial f}{\partial x} = \alpha_i x^i + \sqrt{\beta_{ij}}
|
matrix: |latex
  \begin{pmatrix} a & b \\ c & d \end{pmatrix} \times \int_0^\infty e^{-x} dx
|
plus -> fraction -> matrix
`,

		"a sequence diagram": `shape: sequence_diagram
alice: Alice
bob: Bob
service: Service
alice -> bob: request
bob -> service: forward {style.stroke-dash: 3}
service -> bob: ok
bob -> alice: response
alice."a note about the exchange"
group: retries {
  alice -> bob: again
}
`,

		"a grid": `grid-rows: 2
grid-columns: 2
a
b
c
d: {
  e -> f
}
`,

		"a legend": `vars: {
  d2-legend: {
    compute: Compute {style.fill: "#8aadf4"}
    storage: Storage {style.fill: "#a6da95"; style.stroke-dash: 4}
  }
}
web: {style.fill: "#8aadf4"}
disk: {style.fill: "#a6da95"}
web -> disk
`,

		"near": `title: A standalone title {
  near: top-center
  shape: text
  style.font-size: 28
}
legend: bottom right {near: bottom-right; shape: text}
a -> b
`,

		"direction and a long label": "direction: right\nalpha -> beta -> gamma: a considerably longer edge label than usual\n",
	}
}

// d2 escapes labels, tooltips and links, but not the value of class:, so a
// quote there closes the attribute and the rest of the value is markup. An
// <img> put through that hole leaves the SVG subtree when the page assigns
// innerHTML, and its onerror runs.
func TestAClassValueThatBreaksOutOfItsAttributeIsRefused(t *testing.T) {
	cases := map[string]struct {
		source string
		want   error
	}{
		"an unquoted event handler leaves the output malformed": {
			source: "x: hello {\n  class: 'a\"><img src=q onerror=alert(1)>'\n}\n",
			want:   ErrMalformedSVG,
		},
		"a balanced payload parses, and is refused by name": {
			source: "x: hello {\n  class: 'a\"></g><img src=\"q\" onerror=\"alert(1)\"/><g class=\"b'\n}\n",
			want:   ErrUnknownConstruct,
		},
	}

	r := renderer(t, 0)
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out := r.Render(context.Background(), Fence{Line: 1, Source: tc.source})
			if !errors.Is(out.Err, tc.want) {
				t.Fatalf("err = %v, want %v", out.Err, tc.want)
			}
			if out.SVG != "" {
				t.Error("a refused diagram still carries SVG")
			}
		})
	}
}

// Every feature above must survive the guard: a legitimate diagram refused is
// worse than the pattern matching this replaced.
func TestTheGuardPassesEveryFeatureD2Draws(t *testing.T) {
	r := renderer(t, 0)
	for name, source := range corpus() {
		t.Run(name, func(t *testing.T) {
			out := r.Render(context.Background(), Fence{Line: 1, Source: source})
			if out.Err != nil {
				t.Fatalf("refused: %v", out.Err)
			}
			if !strings.HasPrefix(out.SVG, "<svg") {
				t.Fatalf("SVG starts with %q", out.SVG[:min(40, len(out.SVG))])
			}
		})
	}
}

func TestGuardReadsAnHrefTheWayABrowserWould(t *testing.T) {
	cases := map[string]struct {
		svg  string
		want error
	}{
		"an entity-encoded scheme":     {svg: `<svg><a href="&#106;avascript:alert(1)">x</a></svg>`, want: ErrUnsafeLink},
		"a scheme split by a tab":      {svg: "<svg><a href=\"java\tscript:alert(1)\">x</a></svg>", want: ErrUnsafeLink},
		"an uppercase scheme":          {svg: `<svg><a href="FILE:///etc/passwd">x</a></svg>`, want: ErrUnsafeLink},
		"a single-quoted attribute":    {svg: `<svg><a xlink:href='data:text/html,x'>x</a></svg>`, want: ErrUnsafeLink},
		"an angle bracket in the href": {svg: `<svg><a href="javascript:void(0)>x">label</a></svg>`, want: ErrUnsafeLink},
		"a remote image":               {svg: `<svg><image href="//evil.example/x.png" /></svg>`, want: ErrRemoteImage},
		"a use that leaves the page":   {svg: `<svg><use href="https://evil.example/x.svg#a" /></svg>`, want: ErrUnsafeLink},
		"an inline image is fine":      {svg: `<svg><image href="data:image/png;base64,AAAA" /></svg>`},
		"an https link is fine":        {svg: `<svg><a href="https://example.com">x</a></svg>`},
		"a same-document use is fine":  {svg: `<svg><use href="#shape-1" /></svg>`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := guard(tc.svg); !errors.Is(got, tc.want) {
				t.Fatalf("guard() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGuardRefusesWhatIsNotOnTheLists(t *testing.T) {
	cases := map[string]struct {
		svg  string
		want error
	}{
		"a script element":              {svg: `<svg><SCRIPT>alert(1)</SCRIPT></svg>`, want: ErrEmbeddedScript},
		"an event handler":              {svg: `<svg><rect ONLOAD = "alert(1)" /></svg>`, want: ErrEmbeddedScript},
		"a foreignObject":               {svg: `<svg><foreignObject><p>x</p></foreignObject></svg>`, want: ErrEmbeddedMarkup},
		"an img":                        {svg: `<svg><img src="q" /></svg>`, want: ErrUnknownConstruct},
		"an iframe":                     {svg: `<svg><iframe src="https://evil.example" /></svg>`, want: ErrUnknownConstruct},
		"an embed":                      {svg: `<svg><embed src="https://evil.example" /></svg>`, want: ErrUnknownConstruct},
		"an animation retargeting href": {svg: `<svg><animate attributeName="href" to="javascript:alert(1)" /></svg>`, want: ErrUnknownConstruct},
		"an attribute nobody emits":     {svg: `<svg><rect ping="https://evil.example" /></svg>`, want: ErrUnknownConstruct},
		"a document type declaration":   {svg: `<!DOCTYPE svg SYSTEM "x.dtd"><svg />`, want: ErrUnknownConstruct},
		"an unclosed element":           {svg: `<svg><rect></svg>`, want: ErrMalformedSVG},
		"an unquoted attribute value":   {svg: `<svg><rect width=4 /></svg>`, want: ErrMalformedSVG},
		"prose that begins with on":     {svg: `<svg><text>only once = twice</text></svg>`},
		"a data- attribute is inert":    {svg: `<svg><path data-mml-node="mi" d="M0,0" /></svg>`},
		"a stylesheet in CDATA":         {svg: `<svg><style><![CDATA[.shape{fill:red}]]></style></svg>`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := guard(tc.svg); !errors.Is(got, tc.want) {
				t.Fatalf("guard() = %v, want %v", got, tc.want)
			}
		})
	}
}

// If author text ever reached the stylesheet, an unescaped class value would be
// a CSS injection as well as a markup one.
func TestNoAuthorTextReachesTheStyleBlock(t *testing.T) {
	const planted = "dredgeme"
	svg := rendered(t, renderer(t, 0), "y: {class: "+planted+"}\n")
	if !strings.Contains(svg, planted) {
		t.Fatal("the class name never reached the output, so this proves nothing")
	}

	decoder := xml.NewDecoder(strings.NewReader(svg))
	styling := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			styling = token.Name.Local == "style"
		case xml.EndElement:
			styling = false
		case xml.CharData:
			if styling && strings.Contains(string(token), planted) {
				t.Fatal("a class name reached the stylesheet")
			}
		}
	}
}
