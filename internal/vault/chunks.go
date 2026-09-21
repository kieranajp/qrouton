package vault

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Chunk struct {
	Breadcrumb []string `json:"breadcrumb"`
	Text       string   `json:"text"`
	Input      string   `json:"input"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
}

func ChunkDocument(body string, firstLine int) ([]Chunk, error) {
	tokenizer := NewTokenizer()
	var chunks []Chunk
	var headings [6]string
	var breadcrumb []string
	var section strings.Builder
	sectionLine := firstLine
	line := firstLine
	fence := byte(0)
	fenceSize := 0
	flush := func() error {
		rest := section.String()
		section.Reset()
		prefix := strings.Join(breadcrumb, " > ")
		if prefix != "" {
			prefix += "\n\n"
		}
		if tokenizer.Count(prefix) >= EmbeddingMaxTokens {
			return fmt.Errorf("%w: heading breadcrumb", ErrInputBudget)
		}
		for rest != "" {
			end := len(rest)
			if tokenizer.Count(prefix+rest) > EmbeddingMaxTokens {
				end = 0
				preferred := 0
				for at, r := range rest {
					next := at + utf8.RuneLen(r)
					if tokenizer.Count(prefix+rest[:next]) > EmbeddingMaxTokens {
						break
					}
					end = next
					if r == '\n' || r == '.' || r == '!' || r == '?' {
						preferred = next
					}
				}
				if preferred > 0 {
					end = preferred
				}
				if end == 0 {
					return ErrInputBudget
				}
			}
			text := rest[:end]
			endLine := sectionLine + strings.Count(text, "\n")
			if strings.HasSuffix(text, "\n") {
				endLine--
			}
			chunks = append(chunks, Chunk{Breadcrumb: append([]string(nil), breadcrumb...), Text: text, Input: prefix + text, StartLine: sectionLine, EndLine: endLine})
			sectionLine += strings.Count(text, "\n")
			rest = rest[end:]
		}
		return nil
	}
	for _, raw := range strings.SplitAfter(body, "\n") {
		if raw == "" {
			continue
		}
		trimmed := strings.TrimLeft(raw, " ")
		indent := len(raw) - len(trimmed)
		if indent <= 3 {
			mark := byte(0)
			n := 0
			if len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
				mark = trimmed[0]
				for n < len(trimmed) && trimmed[n] == mark {
					n++
				}
			}
			if fence != 0 {
				if mark == fence && n >= fenceSize && strings.TrimSpace(trimmed[n:]) == "" {
					fence = 0
				}
			} else if n >= 3 && (mark != '`' || !strings.Contains(trimmed[n:], "`")) {
				fence = mark
				fenceSize = n
			} else {
				level := 0
				for level < len(trimmed) && trimmed[level] == '#' {
					level++
				}
				if level > 0 && level <= 6 && (level == len(trimmed) || trimmed[level] == ' ' || trimmed[level] == '\t' || trimmed[level] == '\n' || trimmed[level] == '\r') {
					if err := flush(); err != nil {
						return nil, err
					}
					title := strings.TrimSpace(trimmed[level:])
					closing := strings.TrimRight(title, "#")
					if len(closing) < len(title) && (closing == "" || strings.HasSuffix(closing, " ") || strings.HasSuffix(closing, "\t")) {
						title = strings.TrimSpace(closing)
					}
					headings[level-1] = title
					for i := level; i < 6; i++ {
						headings[i] = ""
					}
					breadcrumb = nil
					for _, h := range headings {
						if h != "" {
							breadcrumb = append(breadcrumb, h)
						}
					}
					sectionLine = line
				}
			}
		}
		section.WriteString(raw)
		line += strings.Count(raw, "\n")
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return chunks, nil
}
