package vault

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

type Query struct {
	Text  string   `json:"query"`
	Kinds []string `json:"kinds,omitempty"`
	Limit int      `json:"limit,omitempty"`
}

type LexicalScore struct {
	Rank  int     `json:"rank"`
	Score float64 `json:"score"`
}

type DenseScore struct {
	Rank   int     `json:"rank"`
	Cosine float64 `json:"cosine"`
}

type Hit struct {
	ID         string        `json:"id"`
	Ref        string        `json:"ref"`
	Profile    string        `json:"profile"`
	Kind       string        `json:"kind,omitempty"`
	Title      string        `json:"title"`
	Breadcrumb string        `json:"breadcrumb"`
	Line       int           `json:"line"`
	Through    int           `json:"through"`
	Snippet    string        `json:"snippet"`
	BM25       *LexicalScore `json:"bm25"`
	Dense      *DenseScore   `json:"dense"`
	FusedRank  int           `json:"fused_rank"`
	AdmittedBy string        `json:"admitted_by"`
}

// Status reports each channel apart. Pending counts windows still waiting for
// an embedding while Semantic is partial.
type Status struct {
	Lexical  string `json:"lexical"`
	Semantic string `json:"semantic"`
	Pending  int    `json:"pending,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type Result struct {
	Hits   []Hit  `json:"hits"`
	Status Status `json:"status"`
}

var (
	tokenPattern = regexp.MustCompile(`[a-z0-9]+`)
	stopword     = map[string]bool{}
)

func init() {
	for _, w := range strings.Fields(stopwords) {
		stopword[w] = true
	}
}

func tokens(s string) []string {
	var out []string
	for _, t := range tokenPattern.FindAllString(strings.ToLower(s), -1) {
		if !stopword[t] {
			out = append(out, stem(t))
		}
	}
	return out
}

func stem(w string) string {
	switch {
	case len(w) > 5 && strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y"
	case len(w) > 5 && strings.HasSuffix(w, "ing"):
		return w[:len(w)-3]
	case len(w) > 4 && strings.HasSuffix(w, "ed"):
		return w[:len(w)-2]
	case len(w) > 4 && strings.HasSuffix(w, "es") && !strings.HasSuffix(w, "ses"):
		return w[:len(w)-1]
	case len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
		return w[:len(w)-1]
	}
	return w
}

type bm25 struct {
	tf    []map[string]int
	dl    []int
	avgdl float64
	df    map[string]int
}

func newBM25(texts []string) *bm25 {
	b := &bm25{df: map[string]int{}}
	total := 0
	for _, text := range texts {
		tf := map[string]int{}
		toks := tokens(text)
		for _, t := range toks {
			tf[t]++
		}
		for t := range tf {
			b.df[t]++
		}
		b.tf = append(b.tf, tf)
		b.dl = append(b.dl, len(toks))
		total += len(toks)
	}
	if len(texts) > 0 {
		b.avgdl = float64(total) / float64(len(texts))
	}
	return b
}

func (b *bm25) scores(query string) []float64 {
	n := float64(len(b.tf))
	scores := make([]float64, len(b.tf))
	seen := map[string]bool{}
	for _, t := range tokens(query) {
		df := float64(b.df[t])
		if seen[t] || df == 0 {
			continue
		}
		seen[t] = true
		idf := math.Log(1 + (n-df+0.5)/(df+0.5))
		for i, tf := range b.tf {
			if f := float64(tf[t]); f > 0 {
				scores[i] += idf * f * (bm25K1 + 1) / (f + bm25K1*(1-bm25B+bm25B*float64(b.dl[i])/b.avgdl))
			}
		}
	}
	return scores
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range min(len(a), len(b)) {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// match is a document's best chunk in one channel; window is -1 for BM25.
type match struct {
	entry  int
	chunk  int
	window int
	score  float64
}

// ordered sorts a channel's matches best first, breaking ties by path order.
func ordered(best map[int]match) []match {
	out := make([]match, 0, len(best))
	for _, m := range best {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b match) int {
		if a.score != b.score {
			if a.score > b.score {
				return -1
			}
			return 1
		}
		return a.entry - b.entry
	})
	return out
}

func entries(ms []match) []int {
	out := make([]int, len(ms))
	for i, m := range ms {
		out[i] = m.entry
	}
	return out
}

func rrfOrder(lexical, dense []int) []int {
	score := map[int]float64{}
	for _, list := range [][]int{lexical, dense} {
		for rank, e := range list {
			score[e] += 1 / float64(rrfK+rank+1)
		}
	}
	order := make([]int, 0, len(score))
	for e := range score {
		order = append(order, e)
	}
	slices.SortFunc(order, func(a, b int) int {
		if score[a] != score[b] {
			if score[a] > score[b] {
				return -1
			}
			return 1
		}
		return a - b
	})
	return order
}

type admission struct {
	entry int
	fused int
	by    string
}

// fuse takes RRF's top limit, then any document in either channel's top three
// that RRF left out, so a win only one channel sees still reaches the reader.
func fuse(lexical, dense []int, limit int) []admission {
	order := rrfOrder(lexical, dense)
	fused := make(map[int]int, len(order))
	for i, e := range order {
		fused[e] = i + 1
	}
	seen := map[int]bool{}
	var out []admission
	admit := func(list []int, n int, by string) {
		for _, e := range list[:min(n, len(list))] {
			if !seen[e] {
				seen[e] = true
				out = append(out, admission{entry: e, fused: fused[e], by: by})
			}
		}
	}
	admit(order, limit, AdmittedRRF)
	admit(lexical, channelTop, AdmittedBM25)
	admit(dense, channelTop, AdmittedDense)
	return out
}

func snippet(text string) string {
	flat := strings.Join(strings.Fields(text), " ")
	if len(flat) <= snippetChars {
		return flat
	}
	cut := snippetChars
	for cut > 0 && !utf8.RuneStart(flat[cut]) {
		cut--
	}
	return flat[:cut] + ellipsis
}
