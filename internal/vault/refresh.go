package vault

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
)

type IndexOptions struct {
	Provider          Provider
	InstallationID    string
	StateDir          string
	ReconcileInterval time.Duration
	Failure           string
}
type IndexStatus struct {
	State       string       `json:"state"`
	Dependency  string       `json:"dependency"`
	Error       string       `json:"error,omitempty"`
	Generation  string       `json:"generation,omitempty"`
	Fingerprint *Fingerprint `json:"fingerprint,omitempty"`
	Indexed     int          `json:"indexed"`
	Pending     int          `json:"pending"`
	Excluded    int          `json:"excluded"`
	Unresolved  int          `json:"unresolved"`
	Competing   int          `json:"competing"`
	Chunks      int          `json:"chunks"`
	Completed   int          `json:"completed"`
	Total       int          `json:"total"`
}
type profileWorker struct {
	profile       Profile
	canonicalRoot string
	stateDir      string
	options       IndexOptions
	validate      func() error
	mu            sync.RWMutex
	ledger        identityLedger
	ledgerErr     error
	current       *generation
	usable        bool
	allowed       map[string]string
	lineage       map[string]LineageState
	status        IndexStatus
	retry         chan struct{}
	cancel        context.CancelFunc
	done          chan struct{}
}

func (w *profileWorker) signal() {
	select {
	case w.retry <- struct{}{}:
	default:
	}
}
func (w *profileWorker) setFailure(err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status.State = StateUnavailable
	if w.current != nil && w.usable {
		w.status.State = StateIncomplete
	}
	switch {
	case errors.Is(err, ErrModelMissing):
		w.status.State = StateUnavailable
		w.status.Dependency = DependencyModelMissing
		w.status.Error = ErrModelMissing.Error()
		w.usable = false
	case errors.Is(err, ErrInvalidVector), errors.Is(err, ErrProviderResponse), errors.Is(err, ErrFingerprintChanged):
		w.status.State = StateUnavailable
		w.status.Dependency = DependencyIncompatible
		w.status.Error = ErrInvalidVector.Error()
		w.usable = false
	case errors.Is(err, ErrProviderUnavailable):
		w.status.State = StateUnavailable
		w.status.Dependency = DependencyUnavailable
		w.status.Error = ErrProviderUnavailable.Error()
		w.usable = false
	case errors.Is(err, ErrIdentityState):
		w.status.Error = ErrIdentityState.Error()
	case errors.Is(err, ErrInvalidConfig):
		w.status.State = StateUnavailable
		w.status.Error = ErrInvalidConfig.Error()
		w.current = nil
		w.usable = false
	case errors.Is(err, ErrUnavailable):
		w.status.State = StateUnavailable
		w.status.Error = ErrUnavailable.Error()
	default:
		w.status.Error = ErrIndexState.Error()
	}
}
func (w *profileWorker) inventory() (inventory, error) {
	if err := w.validate(); err != nil {
		return inventory{}, err
	}
	canonical, err := filepath.EvalSymlinks(w.profile.Root)
	if err != nil {
		return inventory{}, ErrUnavailable
	}
	if canonical != w.canonicalRoot {
		return inventory{}, ErrInvalidConfig
	}
	return scan(w.profile)
}
func (w *profileWorker) observe(inv inventory) {
	w.mu.Lock()
	defer w.mu.Unlock()
	documents := map[string]Document{}
	hashes := map[string]string{}
	excluded := inv.invalid + inv.unsupported
	conflicts := map[string]bool{}
	for _, entry := range inv.entries {
		id := entry.Document.ID
		if !entry.Supported {
			continue
		}
		hash := contentHash(entry.Content)
		if inv.conflicts[id] || w.ledger.conflicts(id, hash) {
			conflicts[id] = true
			continue
		}
		documents[id] = entry.Document
		hashes[id] = hash
	}
	for id := range conflicts {
		delete(documents, id)
		delete(hashes, id)
	}
	excluded += len(conflicts)
	lineage := ResolveLineage(documents)
	allowed := map[string]string{}
	unresolved, competing := 0, 0
	for id, document := range documents {
		state := lineage[id]
		unresolved += len(state.Unresolved)
		if len(state.Successors) > 1 {
			competing++
		}
		if state.Invalid || state.EffectiveState != "active" || document.Kind == "dead_end" {
			excluded++
			continue
		}
		allowed[id] = hashes[id]
	}
	w.allowed = allowed
	w.lineage = lineage
	w.status.Excluded = excluded
	w.status.Unresolved = unresolved
	w.status.Competing = competing
}
func (w *profileWorker) snapshot(inv inventory) IndexStatus {
	w.observe(inv)
	w.mu.RLock()
	defer w.mu.RUnlock()
	status := w.status
	if status.Fingerprint != nil {
		fp := *status.Fingerprint
		status.Fingerprint = &fp
	}
	if w.current != nil {
		status.Generation = w.current.Name
		if w.usable {
			for id, hash := range w.allowed {
				state := w.current.Lineage[id]
				if w.current.Hashes[id] == hash && state.EffectiveState == "active" && !state.Invalid && w.current.Excluded[id] == "" {
					status.Indexed++
				}
			}
			for _, chunk := range w.current.Chunks {
				if w.allowed[chunk.ArtifactID] == chunk.SourceHash {
					status.Chunks++
				}
			}
		}
	}
	chunkExcluded := 0
	if w.current != nil {
		for id := range w.allowed {
			if w.current.Hashes[id] == w.allowed[id] && w.current.Excluded[id] != "" {
				chunkExcluded++
			}
		}
	}
	status.Excluded += chunkExcluded
	status.Pending = max(0, len(w.allowed)-status.Indexed-chunkExcluded)
	if status.State == StateReady && (status.Pending > 0 || status.Excluded > 0 || status.Unresolved > 0) {
		status.State = StateIncomplete
	}
	return status
}
func (w *profileWorker) checkIdentity(result ReadResult) error {
	if !result.Supported {
		return nil
	}
	ledger, err := loadLedger(w.stateDir)
	w.mu.RLock()
	known := w.ledger
	w.mu.RUnlock()
	if known.conflicts(result.Document.ID, contentHash(result.Content)) {
		return ErrConflict
	}
	if err != nil {
		return nil
	}
	if ledger.conflicts(result.Document.ID, contentHash(result.Content)) {
		return ErrConflict
	}
	return nil
}
func acquireWriter(ctx context.Context, root *os.Root) (*os.File, error) {
	if info, err := root.Lstat(workerLockFilename); err == nil && !info.Mode().IsRegular() {
		return nil, ErrIndexState
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	file, err := root.OpenFile(workerLockFilename, os.O_CREATE|os.O_RDWR|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, ErrIndexState
	}
	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			file.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			file.Close()
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
func releaseWriter(file *os.File) { unix.Flock(int(file.Fd()), unix.LOCK_UN); file.Close() }
func (w *profileWorker) run(ctx context.Context) {
	defer close(w.done)
	interval := w.options.ReconcileInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if err := os.MkdirAll(w.stateDir, 0700); err != nil {
			w.setFailure(ErrIdentityState)
		} else {
			root, err := os.OpenRoot(w.stateDir)
			if err == nil {
				w.mu.Lock()
				w.status.State = IndexWaiting
				w.mu.Unlock()
				lock, lockErr := acquireWriter(ctx, root)
				root.Close()
				if lockErr == nil {
					w.own(ctx, ticker)
					releaseWriter(lock)
					return
				}
				if ctx.Err() != nil {
					return
				}
				w.setFailure(ErrIdentityState)
			} else {
				w.setFailure(ErrIdentityState)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-w.retry:
		case <-ticker.C:
		}
	}
}
func (w *profileWorker) own(ctx context.Context, ticker *time.Ticker) {
	events := watchProfile(ctx, w.profile)
	var done chan error
	var cancel context.CancelFunc
	dirty := true
	for {
		if dirty && done == nil {
			jobCtx, jobCancel := context.WithCancel(ctx)
			cancel = jobCancel
			done = make(chan error, 1)
			jobDone := done
			go func() { jobDone <- w.refresh(jobCtx) }()
			dirty = false
		}
		select {
		case <-ctx.Done():
			if cancel != nil {
				cancel()
				<-done
			}
			return
		case <-ticker.C:
			dirty = true
		case <-w.retry:
			dirty = true
			if cancel != nil {
				cancel()
			}
		case <-events:
			if inv, err := w.inventory(); err == nil {
				w.observe(inv)
			} else {
				w.setFailure(err)
			}
			dirty = true
			if cancel != nil {
				cancel()
			}
		case err := <-done:
			if cancel != nil {
				cancel()
			}
			cancel = nil
			done = nil
			if errors.Is(err, ErrSourceChanged) {
				dirty = true
			} else if err != nil {
				w.setFailure(err)
			}
		}
	}
}
func watchProfile(ctx context.Context, profile Profile) <-chan struct{} {
	changes := make(chan struct{}, 1)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return changes
	}
	notify := func() {
		select {
		case changes <- struct{}{}:
		default:
		}
	}
	add := func() {
		root, err := os.OpenRoot(profile.Root)
		if err != nil {
			return
		}
		defer root.Close()
		_ = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return fs.SkipDir
			}
			if !entry.IsDir() {
				return nil
			}
			if entry.Name() != "." && strings.HasPrefix(entry.Name(), ".") {
				return fs.SkipDir
			}
			_ = watcher.Add(filepath.Join(profile.Root, filepath.FromSlash(path)))
			return nil
		})
	}
	add()
	go func() {
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				relative, err := filepath.Rel(profile.Root, event.Name)
				if err != nil || !filepath.IsLocal(relative) {
					continue
				}
				hidden := false
				for _, component := range strings.Split(relative, string(filepath.Separator)) {
					hidden = hidden || strings.HasPrefix(component, ".")
				}
				if hidden {
					continue
				}
				if event.Has(fsnotify.Create) {
					add()
				}
				notify()
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
				notify()
			}
		}
	}()
	return changes
}
func inventorySources(inv inventory) map[string]string {
	sources := map[string]string{}
	for _, entry := range inv.entries {
		sources[entry.Reference.Path] = contentHash(entry.Content)
	}
	return sources
}
func (w *profileWorker) refresh(ctx context.Context) error {
	inv, err := w.inventory()
	if err != nil {
		return err
	}
	ledger, err := loadLedger(w.stateDir)
	if err != nil {
		return err
	}
	if ledger.observe(inv) {
		if err := saveLedger(w.stateDir, ledger); err != nil {
			return err
		}
	}
	w.mu.Lock()
	w.ledger = ledger
	w.ledgerErr = nil
	w.status.State = IndexBuilding
	w.status.Error = ""
	w.status.Completed = 0
	w.status.Total = 0
	w.mu.Unlock()
	w.observe(inv)
	if err := ctx.Err(); err != nil {
		return err
	}
	cache, err := openCache(w.profile, w.options.InstallationID)
	if err != nil {
		return err
	}
	defer cache.Close()
	w.mu.Lock()
	w.status.State = IndexWaiting
	w.mu.Unlock()
	cacheLock, err := acquireWriter(ctx, cache)
	if err != nil {
		return err
	}
	defer releaseWriter(cacheLock)
	w.mu.Lock()
	w.status.State = IndexBuilding
	w.mu.Unlock()
	if err := cleanupStaging(cache); err != nil {
		return ErrIndexState
	}
	prior, _ := loadGeneration(cache)
	fp, err := w.options.Provider.Probe(ctx)
	if err != nil {
		return err
	}
	if !validFingerprint(fp) {
		return ErrInvalidVector
	}
	w.mu.Lock()
	w.status.Dependency = DependencyReady
	w.status.Fingerprint = &fp
	if prior != nil && prior.Fingerprint == fp {
		w.current = prior
		w.usable = true
	} else {
		prior = nil
		if w.current != nil && w.current.Fingerprint == fp {
			w.usable = true
		} else {
			w.current = nil
			w.usable = false
		}
	}
	w.mu.Unlock()
	g, err := w.build(ctx, inv, ledger, fp, prior)
	if err != nil {
		return err
	}
	latest, err := w.inventory()
	if err != nil {
		return err
	}
	w.observe(latest)
	if !reflect.DeepEqual(inventorySources(latest), g.Sources) {
		return ErrSourceChanged
	}
	confirmed, err := w.options.Provider.Probe(ctx)
	if err != nil {
		return err
	}
	if confirmed != fp {
		w.mu.Lock()
		w.current = nil
		w.usable = false
		w.mu.Unlock()
		return ErrFingerprintChanged
	}
	latest, err = w.inventory()
	if err != nil {
		return err
	}
	w.observe(latest)
	if !reflect.DeepEqual(inventorySources(latest), g.Sources) {
		return ErrSourceChanged
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if prior == nil || !reflect.DeepEqual(prior.Sources, g.Sources) || !reflect.DeepEqual(prior.Hashes, g.Hashes) || !reflect.DeepEqual(prior.Lineage, g.Lineage) || !reflect.DeepEqual(prior.Documents, g.Documents) || !reflect.DeepEqual(prior.Chunks, g.Chunks) || !reflect.DeepEqual(prior.Excluded, g.Excluded) {
		if err := writeGeneration(cache, g); err != nil {
			return ErrIndexState
		}
		previous := ""
		if prior != nil {
			previous = prior.Name
		}
		pruneGenerations(cache, g.Name, previous)
	} else {
		g = prior
	}
	w.mu.Lock()
	w.current = g
	w.usable = true
	w.status.State = StateReady
	w.status.Error = ""
	w.status.Completed = w.status.Total
	w.mu.Unlock()
	return nil
}
func (w *profileWorker) build(ctx context.Context, inv inventory, ledger identityLedger, fp Fingerprint, prior *generation) (*generation, error) {
	name, err := generationName()
	if err != nil {
		return nil, err
	}
	g := &generation{Excluded: map[string]string{}, Version: indexSchemaVersion, Name: name, Chunker: ChunkerVersion, Fingerprint: fp, Sources: inventorySources(inv), Documents: map[string]Document{}, Hashes: map[string]string{}, Chunks: []indexedChunk{}, Vectors: []float32{}}
	entries := map[string]ReadResult{}
	for _, entry := range inv.entries {
		id := entry.Document.ID
		hash := contentHash(entry.Content)
		if !entry.Supported || inv.conflicts[id] || ledger.conflicts(id, hash) {
			continue
		}
		metadata := entry.Document
		metadata.Body = ""
		g.Documents[id] = metadata
		g.Hashes[id] = hash
		entries[id] = entry
	}
	g.Lineage = ResolveLineage(g.Documents)
	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		state := g.Lineage[id]
		if state.Invalid || state.EffectiveState != "active" || g.Documents[id].Kind == "dead_end" {
			continue
		}
		chunks, err := ChunkDocument(entries[id].Document.Body, bodyFirstLine(entries[id]))
		if err != nil {
			g.Excluded[id] = ErrInputBudget.Error()
			continue
		}
		for _, chunk := range chunks {
			g.Chunks = append(g.Chunks, indexedChunk{ArtifactID: id, SourceHash: g.Hashes[id], Chunk: chunk})
		}
	}
	w.mu.Lock()
	w.status.Total = len(g.Chunks)
	w.mu.Unlock()
	reusable := map[string][]float32{}
	if prior != nil && prior.Fingerprint == fp {
		for index, chunk := range prior.Chunks {
			reusable[chunk.SourceHash+"\n"+chunk.Input] = prior.Vectors[index*EmbeddingDimensions : (index+1)*EmbeddingDimensions]
		}
	}
	g.Vectors = make([]float32, len(g.Chunks)*EmbeddingDimensions)
	pending := []int{}
	for index, chunk := range g.Chunks {
		if vector, ok := reusable[chunk.SourceHash+"\n"+chunk.Input]; ok {
			copy(g.Vectors[index*EmbeddingDimensions:], vector)
		} else {
			pending = append(pending, index)
		}
	}
	w.mu.Lock()
	w.status.Completed = len(g.Chunks) - len(pending)
	w.mu.Unlock()
	for offset := 0; offset < len(pending); offset += 16 {
		batch := pending[offset:min(offset+16, len(pending))]
		inputs := make([]string, len(batch))
		for index, chunkIndex := range batch {
			inputs[index] = g.Chunks[chunkIndex].Input
		}
		vectors, err := w.options.Provider.Embed(ctx, fp, inputs)
		if err != nil {
			return nil, err
		}
		if len(vectors) != len(batch) {
			return nil, ErrInvalidVector
		}
		for index, chunkIndex := range batch {
			if err := NormalizeVector(vectors[index]); err != nil {
				return nil, err
			}
			copy(g.Vectors[chunkIndex*EmbeddingDimensions:], vectors[index])
		}
		w.mu.Lock()
		w.status.Completed += len(batch)
		w.mu.Unlock()
	}
	g.Postings, g.Lengths = postingsFor(g.Chunks)
	return g, validateGeneration(g)
}

func canonicalProfileRoot(path string) string {
	clean := filepath.Clean(path)
	candidate := clean
	suffix := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			return filepath.Join(append([]string{resolved}, suffix...)...)
		}
		if !os.IsNotExist(err) {
			return clean
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return clean
		}
		suffix = append([]string{filepath.Base(candidate)}, suffix...)
		candidate = parent
	}
}

func (w *profileWorker) applyIdentityConflicts(inv inventory) inventory {
	disk, err := loadLedger(w.stateDir)
	w.mu.RLock()
	known := w.ledger
	w.mu.RUnlock()
	for _, entry := range inv.entries {
		if !entry.Supported {
			continue
		}
		id, hash := entry.Document.ID, contentHash(entry.Content)
		if known.conflicts(id, hash) || (err == nil && disk.conflicts(id, hash)) {
			inv.conflicts[id] = true
		}
	}
	return inv
}
