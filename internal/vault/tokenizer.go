package vault

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

//go:embed assets/*
var tokenizerAssets embed.FS

var tokenizerOnce = sync.OnceValue(func() *Tokenizer {
	vocabulary, _ := tokenizerAssets.ReadFile("assets/minilm-vocab.txt")
	settings, _ := tokenizerAssets.ReadFile("assets/minilm-tokenizer.json")
	config, _ := tokenizerAssets.ReadFile("assets/tokenizer_config.json")
	h := sha256.New()
	h.Write(vocabulary)
	h.Write(settings)
	h.Write(config)
	t := &Tokenizer{vocabulary: make(map[string]bool), checksum: fmt.Sprintf("%x", h.Sum(nil))}
	for _, word := range strings.Split(strings.TrimSuffix(string(vocabulary), "\n"), "\n") {
		t.vocabulary[word] = true
	}
	return t
})

type Tokenizer struct {
	vocabulary map[string]bool
	checksum   string
}

func NewTokenizer() *Tokenizer             { return tokenizerOnce() }
func (t *Tokenizer) Checksum() string      { return t.checksum }
func (t *Tokenizer) Count(text string) int { return len(t.Tokens(text)) }

func (t *Tokenizer) Tokens(text string) []string {
	tokens := []string{"[CLS]"}
	for len(text) > 0 {
		first, special := len(text), ""
		for _, s := range []string{"[PAD]", "[UNK]", "[CLS]", "[SEP]", "[MASK]"} {
			if i := strings.Index(text, s); i >= 0 && i < first {
				first, special = i, s
			}
		}
		tokens = append(tokens, t.basic(text[:first])...)
		if special == "" {
			break
		}
		tokens = append(tokens, special)
		text = text[first+len(special):]
	}
	return append(tokens, "[SEP]")
}
func (t *Tokenizer) basic(text string) []string {
	var clean strings.Builder
	for _, r := range text {
		switch {
		case r == 0 || r == '\uFFFD':
		case r == '\t' || r == '\n' || r == '\r':
			clean.WriteByte(' ')
		case unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Co, r) || unicode.Is(unicode.Cs, r):
		case unicode.IsSpace(r):
			clean.WriteByte(' ')
		case chinese(r):
			clean.WriteByte(' ')
			clean.WriteRune(r)
			clean.WriteByte(' ')
		default:
			clean.WriteRune(r)
		}
	}
	var normalized strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(clean.String())) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsPunct(r) || (r >= 33 && r <= 47) || (r >= 58 && r <= 64) || (r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
			normalized.WriteByte(' ')
			normalized.WriteRune(r)
			normalized.WriteByte(' ')
		} else {
			normalized.WriteRune(r)
		}
	}
	var out []string
	for _, word := range strings.Fields(normalized.String()) {
		runes := []rune(word)
		if len(runes) > 100 {
			out = append(out, "[UNK]")
			continue
		}
		var pieces []string
		for start := 0; start < len(runes); {
			end := len(runes)
			piece := ""
			for end > start {
				candidate := string(runes[start:end])
				if start > 0 {
					candidate = "##" + candidate
				}
				if t.vocabulary[candidate] {
					piece = candidate
					break
				}
				end--
			}
			if piece == "" {
				pieces = []string{"[UNK]"}
				break
			}
			pieces = append(pieces, piece)
			start = end
		}
		out = append(out, pieces...)
	}
	return out
}
func chinese(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) || (r >= 0x20000 && r <= 0x2A6DF) || (r >= 0x2A700 && r <= 0x2B73F) || (r >= 0x2B740 && r <= 0x2B81F) || (r >= 0x2B820 && r <= 0x2CEAF) || (r >= 0xF900 && r <= 0xFAFF) || (r >= 0x2F800 && r <= 0x2FA1F)
}
