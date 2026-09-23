package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// chunk is one heading section. BM25 scores it whole; dense scores its windows.
type chunk struct {
	breadcrumb string
	text       string
	start, end int
	windows    []window
}

type window struct {
	crumb      string
	input      string
	hash       string
	text       string
	start, end int
}

type piece struct {
	text       string
	start, end int
}

// chunkDocument splits body at Markdown headings outside fences. Each chunk's
// breadcrumb leads with the document title, as the embedded inputs do.
func chunkDocument(title, body string, firstLine int) []chunk {
	var chunks []chunk
	var headings [6]string
	crumb := title
	var section strings.Builder
	sectionLine, line := firstLine, firstLine
	var fence byte
	fenceSize := 0
	headed := false
	flush := func() {
		text, content := section.String(), section.String()
		if headed {
			_, content, _ = strings.Cut(text, "\n")
		}
		if strings.TrimSpace(content) != "" {
			chunks = append(chunks, newChunk(crumb, text, sectionLine))
		}
		section.Reset()
	}
	for _, raw := range strings.SplitAfter(body, "\n") {
		if raw == "" {
			continue
		}
		trimmed := strings.TrimLeft(raw, " ")
		if len(raw)-len(trimmed) <= 3 {
			mark, n := byte(0), 0
			if trimmed != "" && (trimmed[0] == '`' || trimmed[0] == '~') {
				mark = trimmed[0]
				for n < len(trimmed) && trimmed[n] == mark {
					n++
				}
			}
			switch {
			case fence != 0:
				if mark == fence && n >= fenceSize && strings.TrimSpace(trimmed[n:]) == "" {
					fence = 0
				}
			case n >= 3 && (mark != '`' || !strings.Contains(trimmed[n:], "`")):
				fence, fenceSize = mark, n
			default:
				if level, heading, ok := atxHeading(trimmed); ok {
					flush()
					headings[level-1] = heading
					for i := level; i < len(headings); i++ {
						headings[i] = ""
					}
					parts := []string{title}
					for _, h := range headings {
						if h != "" && h != title {
							parts = append(parts, h)
						}
					}
					crumb = strings.Join(parts, crumbSeparator)
					sectionLine = line
					headed = true
				}
			}
		}
		section.WriteString(raw)
		line++
	}
	flush()
	return chunks
}

func atxHeading(line string) (int, string, bool) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || (level < len(line) && !strings.ContainsRune(" \t\r\n", rune(line[level]))) {
		return 0, "", false
	}
	title := strings.TrimSpace(line[level:])
	if closing := strings.TrimRight(title, "#"); len(closing) < len(title) && (closing == "" || strings.HasSuffix(closing, " ") || strings.HasSuffix(closing, "\t")) {
		title = strings.TrimSpace(closing)
	}
	return level, title, true
}

func newChunk(crumb, text string, start int) chunk {
	c := chunk{breadcrumb: crumb, text: text, start: start, end: start + strings.Count(strings.TrimRight(text, "\n"), "\n")}
	for _, p := range splitWindows(text, start, windowChars) {
		w := newWindow(crumb, p.text)
		w.start, w.end = p.start, p.end
		c.windows = append(c.windows, w)
	}
	return c
}

// splitWindows packs paragraphs, then lines, then runes into windows of at most
// limit bytes, keeping the source lines each window covers.
func splitWindows(text string, start, limit int) []piece {
	var out []piece
	for _, p := range pack(paragraphs(text, start), limit, func(p piece) []piece {
		return pack(lines(p), limit, func(p piece) []piece { return hardSplit(p, limit) })
	}) {
		if trimmed := strings.TrimSpace(p.text); trimmed != "" {
			p.text = trimmed
			out = append(out, p)
		}
	}
	return out
}

func pack(pieces []piece, limit int, finer func(piece) []piece) []piece {
	var out []piece
	var current piece
	open := false
	for _, p := range pieces {
		if len(p.text) > limit {
			if open {
				out, open = append(out, current), false
			}
			out = append(out, finer(p)...)
			continue
		}
		if open && len(current.text)+len(p.text) > limit {
			out, open = append(out, current), false
		}
		if !open {
			current, open = p, true
			continue
		}
		current.text += p.text
		current.end = p.end
	}
	if open {
		out = append(out, current)
	}
	return out
}

func paragraphs(text string, start int) []piece {
	var out []piece
	ended := true
	for _, l := range lines(piece{text: text, start: start}) {
		if n := len(out); n > 0 && !ended {
			out[n-1].text += l.text
			out[n-1].end = l.end
		} else {
			out = append(out, l)
		}
		ended = strings.TrimSpace(l.text) == ""
	}
	return out
}

func lines(p piece) []piece {
	var out []piece
	line := p.start
	for _, raw := range strings.SplitAfter(p.text, "\n") {
		if raw != "" {
			out = append(out, piece{text: raw, start: line, end: line})
			line++
		}
	}
	return out
}

func hardSplit(p piece, limit int) []piece {
	var out []piece
	for rest := p.text; rest != ""; {
		cut := min(limit, len(rest))
		for cut < len(rest) && cut > 0 && !utf8.RuneStart(rest[cut]) {
			cut--
		}
		out = append(out, piece{text: rest[:cut], start: p.start, end: p.end})
		rest = rest[cut:]
	}
	return out
}

// halves splits an input that overflowed the model at the line nearest its
// middle, or at a rune boundary when it holds a single line.
func halves(input string) (string, string) {
	mid := len(input) / 2
	if at := strings.LastIndex(input[:mid], "\n"); at > 0 {
		return input[:at+1], input[at+1:]
	}
	for mid > 0 && !utf8.RuneStart(input[mid]) {
		mid--
	}
	return input[:mid], input[mid:]
}

func newWindow(crumb, text string) window {
	input := crumb + "\n\n" + text
	sum := sha256.Sum256([]byte(input))
	return window{crumb: crumb, input: input, hash: hex.EncodeToString(sum[:]), text: text}
}
