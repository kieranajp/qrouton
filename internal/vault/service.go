package vault

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Root string `json:"root"`
}

// ReadRequest names a document by ref or artifact id. Line opens a range, and a
// Through below it runs to the end of the file.
type ReadRequest struct {
	Ref     string `json:"ref"`
	Line    int    `json:"line,omitempty"`
	Through int    `json:"through,omitempty"`
	Full    bool   `json:"full,omitempty"`
}

type Excerpt struct {
	Document
	Ref       string `json:"ref"`
	Profile   string `json:"profile"`
	Line      int    `json:"line"`
	Through   int    `json:"through"`
	Lines     int    `json:"lines"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

type entry struct {
	name   string
	doc    Document
	chunks []chunk
}

// Service is one profile's in-memory index. Every search and read stats the
// vault first, at most once per interval, so an edit shows without a restart.
type Service struct {
	profile  Profile
	embedder Embedder
	cache    string
	interval time.Duration

	mu       sync.Mutex
	walked   time.Time
	rootErr  error
	stamps   map[string]stamp
	byName   map[string]*entry
	entries  []*entry
	chunkAt  [][2]int
	lexicon  *bm25
	header   *cacheHeader
	loaded   cacheHeader
	vectors  map[string][][]float32
	building bool
}

var profileID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func New(profile Profile, embedder Embedder, cacheDir string) (*Service, error) {
	if !profileID.MatchString(profile.ID) || !filepath.IsAbs(profile.Root) {
		return nil, ErrInvalidProfile
	}
	return &Service{
		profile:  profile,
		embedder: embedder,
		cache:    filepath.Join(cacheDir, profile.ID+cacheExtension),
		interval: walkInterval,
		stamps:   map[string]stamp{},
		byName:   map[string]*entry{},
		vectors:  map[string][][]float32{},
	}, nil
}

// Warm indexes the vault and embeds its whole backlog, so the first search
// does not wait on a cold build.
func (s *Service) Warm(ctx context.Context) {
	s.mu.Lock()
	_ = s.refreshLocked(true)
	s.mu.Unlock()
	if _, err := s.model(ctx); err != nil {
		return
	}
	s.mu.Lock()
	if s.building {
		s.mu.Unlock()
		return
	}
	s.building = true
	s.mu.Unlock()
	s.build(ctx)
}

func (s *Service) Search(ctx context.Context, q Query) (Result, error) {
	if strings.TrimSpace(q.Text) == "" {
		return Result{}, ErrQueryRequired
	}
	if q.Limit <= 0 {
		q.Limit = defaultLimit
	}
	q.Limit = min(q.Limit, maxLimit)
	for _, kind := range q.Kinds {
		if !kinds[kind] {
			return Result{}, fmt.Errorf("%w: %q", ErrUnknownKind, kind)
		}
	}
	s.mu.Lock()
	err := s.refreshLocked(false)
	s.mu.Unlock()
	if err != nil {
		return Result{Hits: []Hit{}, Status: Status{Lexical: StatusUnavailable, Semantic: StatusUnavailable, Reason: err.Error()}}, nil
	}
	status := s.semantic(ctx)
	var vector []float32
	if status.Semantic != StatusUnavailable {
		vectors, err := s.embedder.Embed(ctx, []string{q.Text})
		if err != nil {
			s.forget()
			status = Status{Lexical: StatusReady, Semantic: StatusUnavailable, Reason: err.Error()}
		} else {
			vector = vectors[0]
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lexical, dense := s.channelsLocked(q, vector)
	return Result{Hits: s.hitsLocked(lexical, dense, q.Limit), Status: status}, nil
}

func (s *Service) Read(req ReadRequest) (Excerpt, error) {
	ref := strings.TrimSpace(req.Ref)
	if ref == "" {
		return Excerpt{}, ErrRefRequired
	}
	s.mu.Lock()
	err := s.refreshLocked(false)
	e := s.lookupLocked(ref)
	s.mu.Unlock()
	if err != nil {
		return Excerpt{}, err
	}
	if e == nil {
		return Excerpt{}, fmt.Errorf("%w: %s", ErrUnknownDocument, ref)
	}
	b, err := readDocument(s.profile.Root, e.name)
	if err != nil {
		return Excerpt{}, err
	}
	lines := strings.SplitAfter(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	first, last := 1, len(lines)
	if req.Line > 0 {
		first = min(req.Line, max(last, 1))
		if req.Through >= first {
			last = min(req.Through, last)
		}
	}
	ex := Excerpt{Document: e.doc, Ref: s.ref(e), Profile: s.profile.ID, Line: first, Through: last, Lines: len(lines)}
	if first <= last {
		ex.Content = strings.Join(lines[first-1:last], "")
	}
	if !req.Full && len(ex.Content) > readLimit {
		cut := readLimit
		for cut > 0 && !utf8.RuneStart(ex.Content[cut]) {
			cut--
		}
		ex.Content, ex.Truncated = ex.Content[:cut], true
	}
	return ex, nil
}

func (s *Service) ref(e *entry) string { return s.profile.ID + refSeparator + e.name }

func (s *Service) lookupLocked(ref string) *entry {
	if name, ok := strings.CutPrefix(ref, s.profile.ID+refSeparator); ok {
		return s.byName[name]
	}
	for _, e := range s.entries {
		if e.doc.ID == ref {
			return e
		}
	}
	return nil
}

// refreshLocked re-parses only files whose size or mtime moved, and rebuilds
// BM25 whole when anything did. A same-size edit inside one mtime tick is missed.
func (s *Service) refreshLocked(force bool) error {
	now := time.Now()
	if !force && !s.walked.IsZero() && now.Sub(s.walked) < s.interval {
		return s.rootErr
	}
	s.walked = now
	stamps, err := walk(s.profile.Root)
	if err != nil {
		s.rootErr = fmt.Errorf("%w: %s", ErrUnavailable, s.profile.Root)
		s.stamps, s.byName = map[string]stamp{}, map[string]*entry{}
		s.reindexLocked()
		return s.rootErr
	}
	s.rootErr = nil
	changed := s.lexicon == nil
	for name, st := range stamps {
		if old, ok := s.stamps[name]; ok && old == st {
			continue
		}
		changed = true
		if !s.loadLocked(name) {
			delete(stamps, name)
		}
	}
	for name := range s.stamps {
		if _, ok := stamps[name]; !ok {
			delete(s.byName, name)
			changed = true
		}
	}
	s.stamps = stamps
	if changed {
		s.reindexLocked()
	}
	return nil
}

// loadLocked reports false when the file could not be read, so the next walk
// tries it again.
func (s *Service) loadLocked(name string) bool {
	b, err := readDocument(s.profile.Root, name)
	if err != nil {
		delete(s.byName, name)
		return false
	}
	doc, body, line, ok := parseDocument(name, string(b))
	if !ok {
		delete(s.byName, name)
		return true
	}
	s.byName[name] = &entry{name: name, doc: doc, chunks: chunkDocument(doc.Title, body, line)}
	return true
}

func (s *Service) reindexLocked() {
	s.entries = s.entries[:0]
	for _, e := range s.byName {
		s.entries = append(s.entries, e)
	}
	slices.SortFunc(s.entries, func(a, b *entry) int { return strings.Compare(a.name, b.name) })
	s.chunkAt = s.chunkAt[:0]
	var texts []string
	for i, e := range s.entries {
		for j, c := range e.chunks {
			s.chunkAt = append(s.chunkAt, [2]int{i, j})
			texts = append(texts, c.breadcrumb+"\n"+c.text)
		}
	}
	s.lexicon = newBM25(texts)
}

func (s *Service) pendingLocked() []window {
	var out []window
	seen := map[string]bool{}
	for _, e := range s.entries {
		for _, c := range e.chunks {
			for _, w := range c.windows {
				if _, ok := s.vectors[w.hash]; !ok && !seen[w.hash] {
					seen[w.hash] = true
					out = append(out, w)
				}
			}
		}
	}
	return out
}

// model probes Ollama until it answers, then keeps the answer until an embed
// fails. A new model or digest discards the vectors built for the old one.
func (s *Service) model(ctx context.Context) (cacheHeader, error) {
	s.mu.Lock()
	if s.header != nil {
		header := *s.header
		s.mu.Unlock()
		return header, nil
	}
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	m, err := s.embedder.Model(ctx)
	if err != nil {
		return cacheHeader{}, err
	}
	header := cacheHeader{Model: m.Name, Digest: m.Digest, Preprocessing: preprocessing}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded != header {
		s.vectors, s.loaded = loadCache(s.cache, header), header
	}
	s.header = &header
	return header, nil
}

func (s *Service) forget() {
	s.mu.Lock()
	s.header = nil
	s.mu.Unlock()
}

// semantic embeds a backlog of one batch before answering, and hands anything
// larger to a background build while dense ranks what is already embedded.
func (s *Service) semantic(ctx context.Context) Status {
	if _, err := s.model(ctx); err != nil {
		return Status{Lexical: StatusReady, Semantic: StatusUnavailable, Reason: err.Error()}
	}
	s.mu.Lock()
	pending := s.pendingLocked()
	switch {
	case len(pending) == 0:
		s.mu.Unlock()
		return Status{Lexical: StatusReady, Semantic: StatusReady}
	case s.building:
		s.mu.Unlock()
		return Status{Lexical: StatusReady, Semantic: StatusPartial, Pending: len(pending)}
	case len(pending) > embedBatch:
		s.building = true
		s.mu.Unlock()
		go s.build(context.Background())
		return Status{Lexical: StatusReady, Semantic: StatusPartial, Pending: len(pending)}
	}
	s.mu.Unlock()
	if err := s.embed(ctx, pending); err != nil {
		return Status{Lexical: StatusReady, Semantic: StatusPartial, Pending: len(pending), Reason: err.Error()}
	}
	s.save()
	return Status{Lexical: StatusReady, Semantic: StatusReady}
}

func (s *Service) build(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		s.building = false
		s.mu.Unlock()
	}()
	for batch := 1; ; batch++ {
		if batch%saveEvery == 0 {
			s.save()
		}
		s.mu.Lock()
		pending := s.pendingLocked()
		s.mu.Unlock()
		if len(pending) == 0 {
			break
		}
		if err := s.embed(ctx, pending[:min(embedBatch, len(pending))]); err != nil {
			s.forget()
			break
		}
	}
	s.save()
}

func (s *Service) embed(ctx context.Context, windows []window) error {
	s.mu.Lock()
	loaded := s.loaded
	s.mu.Unlock()
	inputs := make([]string, len(windows))
	for i, w := range windows {
		inputs[i] = w.input
	}
	vectors, err := embedAll(ctx, s.embedder, inputs)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded != loaded {
		return ErrModelChanged
	}
	for i, w := range windows {
		s.vectors[w.hash] = vectors[i]
	}
	return nil
}

// save keeps only the vectors a current window still uses.
func (s *Service) save() {
	s.mu.Lock()
	if s.header == nil {
		s.mu.Unlock()
		return
	}
	live := map[string][][]float32{}
	for _, e := range s.entries {
		for _, c := range e.chunks {
			for _, w := range c.windows {
				if v, ok := s.vectors[w.hash]; ok {
					live[w.hash] = v
				}
			}
		}
	}
	s.vectors = live
	b, err := encodeCache(*s.header, live)
	s.mu.Unlock()
	if err == nil {
		_ = writeCache(s.cache, b)
	}
}

func (s *Service) channelsLocked(q Query, vector []float32) (lexical, dense []match) {
	allowed := func(e *entry) bool { return len(q.Kinds) == 0 || slices.Contains(q.Kinds, e.doc.Kind) }
	lexBest := map[int]match{}
	for i, score := range s.lexicon.scores(q.Text) {
		at := s.chunkAt[i]
		if score <= 0 || !allowed(s.entries[at[0]]) {
			continue
		}
		if m, ok := lexBest[at[0]]; !ok || score > m.score {
			lexBest[at[0]] = match{entry: at[0], chunk: at[1], window: -1, score: score}
		}
	}
	denseBest := map[int]match{}
	for ei, e := range s.entries {
		if vector == nil || !allowed(e) {
			continue
		}
		for ci, c := range e.chunks {
			for wi, w := range c.windows {
				for _, v := range s.vectors[w.hash] {
					score := cosine(vector, v)
					if m, ok := denseBest[ei]; !ok || score > m.score {
						denseBest[ei] = match{entry: ei, chunk: ci, window: wi, score: score}
					}
				}
			}
		}
	}
	return ordered(lexBest), ordered(denseBest)
}

// hitsLocked places each hit at the chunk its better-ranked channel chose.
func (s *Service) hitsLocked(lexical, dense []match, limit int) []Hit {
	lexRank, denseRank := map[int]int{}, map[int]int{}
	for i, m := range lexical {
		lexRank[m.entry] = i
	}
	for i, m := range dense {
		denseRank[m.entry] = i
	}
	hits := []Hit{}
	for _, a := range fuse(entries(lexical), entries(dense), limit) {
		e := s.entries[a.entry]
		hit := Hit{ID: e.doc.ID, Ref: s.ref(e), Profile: s.profile.ID, Kind: e.doc.Kind, Title: e.doc.Title, FusedRank: a.fused, AdmittedBy: a.by}
		var chosen match
		if i, ok := lexRank[a.entry]; ok {
			chosen = lexical[i]
			hit.BM25 = &LexicalScore{Rank: i + 1, Score: round(chosen.score)}
		}
		if i, ok := denseRank[a.entry]; ok {
			hit.Dense = &DenseScore{Rank: i + 1, Cosine: round(dense[i].score)}
			if hit.BM25 == nil || i+1 < hit.BM25.Rank {
				chosen = dense[i]
			}
		}
		c := e.chunks[chosen.chunk]
		hit.Breadcrumb, hit.Line, hit.Through = c.breadcrumb, c.start, c.end
		text := c.text
		if chosen.window >= 0 {
			w := c.windows[chosen.window]
			hit.Line, hit.Through, text = w.start, w.end, w.text
		}
		hit.Snippet = snippet(text)
		hits = append(hits, hit)
	}
	return hits
}

func round(x float64) float64 { return math.Round(x*1e4) / 1e4 }
