package vault

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type fakeIndexProvider struct {
	mu          sync.Mutex
	fingerprint Fingerprint
	failure     error
	gate        <-chan struct{}
	embeds      int
	started     chan struct{}
}

func newFakeIndexProvider() *fakeIndexProvider {
	return &fakeIndexProvider{fingerprint: Fingerprint{Model: EmbeddingModel, Digest: "digest-a", Tokenizer: NewTokenizer().Checksum(), Preprocessing: PreprocessingVersion, Dimensions: EmbeddingDimensions, MaxTokens: EmbeddingMaxTokens}, started: make(chan struct{}, 100)}
}
func (p *fakeIndexProvider) Probe(ctx context.Context) (Fingerprint, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Fingerprint{}, err
	}
	return p.fingerprint, p.failure
}
func (p *fakeIndexProvider) Embed(ctx context.Context, fp Fingerprint, inputs []string) ([][]float32, error) {
	p.mu.Lock()
	p.embeds++
	gate, err, current := p.gate, p.failure, p.fingerprint
	p.mu.Unlock()
	select {
	case p.started <- struct{}{}:
	default:
	}
	if err != nil {
		return nil, err
	}
	if fp != current {
		return nil, ErrFingerprintChanged
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	vectors := make([][]float32, len(inputs))
	for i := range inputs {
		vectors[i] = make([]float32, EmbeddingDimensions)
		vectors[i][i%EmbeddingDimensions] = 1
	}
	return vectors, nil
}
func (p *fakeIndexProvider) calls() int { p.mu.Lock(); defer p.mu.Unlock(); return p.embeds }
func indexedFixture(t *testing.T, provider *fakeIndexProvider) (*Service, Profile, IndexOptions) {
	t.Helper()
	profile := Profile{ID: "shared", Name: "Shared", Root: t.TempDir()}
	put(t, profile.Root, "session/R1.md", fixtureBytes(t))
	options := IndexOptions{Provider: provider, InstallationID: "installation", StateDir: t.TempDir(), ReconcileInterval: time.Hour}
	service, err := NewIndexedService([]Profile{profile}, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { service.Close() })
	return service, profile, options
}
func statusFor(service *Service) IndexStatus {
	status := service.Status(Scope{ReadProfiles: []string{"shared"}})
	if len(status.Profiles) == 0 || status.Profiles[0].Index == nil {
		return IndexStatus{State: status.State}
	}
	return *status.Profiles[0].Index
}
func awaitIndex(t *testing.T, service *Service, condition func(IndexStatus) bool) IndexStatus {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var status IndexStatus
	for time.Now().Before(deadline) {
		status = statusFor(service)
		if condition(status) {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("index did not settle: %+v", status)
	return status
}
func awaitReady(t *testing.T, service *Service) IndexStatus {
	return awaitIndex(t, service, func(status IndexStatus) bool { return status.State == StateReady && status.Pending == 0 })
}
func writeDocument(t *testing.T, profile Profile, id string, edges ...Edge) {
	t.Helper()
	d := fixtureDocument()
	d.ID = id
	d.Lineage = edges
	b, err := Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	put(t, profile.Root, id+".md", b)
}
func TestIndexBuildReuseAndFingerprintReplacement(t *testing.T) {
	provider := newFakeIndexProvider()
	service, profile, options := indexedFixture(t, provider)
	first := awaitReady(t, service)
	if first.Indexed != 1 || first.Chunks == 0 || first.Fingerprint == nil {
		t.Fatal(first)
	}
	before := provider.calls()
	if err := service.Retry(profile.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	again := awaitReady(t, service)
	if again.Generation != first.Generation || provider.calls() != before {
		t.Fatalf("unchanged index rebuilt: %+v, calls %d/%d", again, provider.calls(), before)
	}
	provider.mu.Lock()
	provider.fingerprint.Digest = "digest-b"
	provider.mu.Unlock()
	service.Retry(profile.ID)
	changed := awaitIndex(t, service, func(status IndexStatus) bool {
		return status.State == StateReady && status.Fingerprint != nil && status.Fingerprint.Digest == "digest-b"
	})
	if changed.Generation == first.Generation || provider.calls() <= before {
		t.Fatal("fingerprint change reused vectors")
	}
	cache, err := openCache(profile, options.InstallationID)
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	g, err := loadGeneration(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Vectors) != len(g.Chunks)*EmbeddingDimensions || len(g.Postings["evidence"]) == 0 {
		t.Fatal("missing contiguous vectors/postings")
	}
	if g.Chunks[0].StartLine <= 1 {
		t.Fatal("chunk location excludes frontmatter offset")
	}
}
func TestIdentitySurvivesCacheDeletionRootMoveAndUnavailableModel(t *testing.T) {
	provider := newFakeIndexProvider()
	service, profile, options := indexedFixture(t, provider)
	awaitReady(t, service)
	service.Close()
	root := t.TempDir()
	profile.Root = root
	b := append(fixtureBytes(t), []byte("Edited immutable bytes\n")...)
	put(t, root, "session/R1.md", b)
	provider.mu.Lock()
	provider.failure = ErrProviderUnavailable
	provider.mu.Unlock()
	moved, err := NewIndexedService([]Profile{profile}, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	defer moved.Close()
	if _, err := moved.Read(Scope{ReadProfiles: []string{profile.ID}}, Reference{ID: "session/R1"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("moved root resurrected identity: %v", err)
	}
	awaitIndex(t, moved, func(status IndexStatus) bool { return status.State == StateUnavailable && status.Excluded == 1 })
	if err := os.RemoveAll(filepath.Join(root, ".qrouton")); err != nil {
		t.Fatal(err)
	}
	moved.Close()
	restarted, err := NewIndexedService([]Profile{profile}, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if _, err := restarted.Read(Scope{ReadProfiles: []string{profile.ID}}, Reference{ID: "session/R1"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("cache deletion resurrected identity: %v", err)
	}
}
func TestDirectReadsDoNotWaitForIndexOrMissingModel(t *testing.T) {
	provider := newFakeIndexProvider()
	gate := make(chan struct{})
	provider.gate = gate
	service, _, _ := indexedFixture(t, provider)
	select {
	case <-provider.started:
	case <-time.After(3 * time.Second):
		t.Fatal("embedding never started")
	}
	start := time.Now()
	result, err := service.Read(Scope{ReadProfiles: []string{"shared"}}, Reference{ID: "session/R1"})
	if err != nil || !strings.Contains(result.Content, "Evidence") || time.Since(start) > time.Second {
		t.Fatalf("read waited for embedding: %v", err)
	}
	service.Close()
	provider.mu.Lock()
	provider.gate = nil
	provider.failure = ErrModelMissing
	provider.mu.Unlock()
	unavailable, _, _ := indexedFixture(t, provider)
	status := awaitIndex(t, unavailable, func(status IndexStatus) bool { return status.Dependency == DependencyModelMissing })
	if status.Indexed != 0 {
		t.Fatal(status)
	}
	if _, err := unavailable.Read(Scope{ReadProfiles: []string{"shared"}}, Reference{ID: "session/R1"}); err != nil {
		t.Fatal(err)
	}
}
func TestExclusionsApplyBeforeSlowBuildAndRecoverAfterRemoval(t *testing.T) {
	provider := newFakeIndexProvider()
	service, profile, _ := indexedFixture(t, provider)
	first := awaitReady(t, service)
	gate := make(chan struct{})
	provider.mu.Lock()
	provider.gate = gate
	provider.mu.Unlock()
	writeDocument(t, profile, "session/R2", Edge{Relation: "supersedes", Target: "session/R1"})
	service.Retry(profile.ID)
	status := awaitIndex(t, service, func(status IndexStatus) bool {
		return status.Excluded == 1 && status.Indexed == 0 && status.Pending == 1
	})
	if status.Generation != first.Generation {
		t.Fatal("prior generation lost during build")
	}
	if err := os.Remove(filepath.Join(profile.Root, "session/R1.md")); err != nil {
		t.Fatal(err)
	}
	service.Retry(profile.ID)
	awaitIndex(t, service, func(status IndexStatus) bool { return status.Unresolved == 1 && status.Indexed == 0 })
	provider.mu.Lock()
	provider.gate = nil
	provider.mu.Unlock()
	close(gate)
	awaitIndex(t, service, func(status IndexStatus) bool {
		return status.State == StateIncomplete && status.Indexed == 1 && status.Unresolved == 1
	})
}
func TestCorruptGenerationAndInterruptedStagingRebuild(t *testing.T) {
	provider := newFakeIndexProvider()
	service, profile, options := indexedFixture(t, provider)
	first := awaitReady(t, service)
	service.Close()
	cachePath := filepath.Join(profile.Root, ".qrouton/index", options.InstallationID)
	if err := os.WriteFile(filepath.Join(cachePath, first.Generation, generationVectors), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cachePath, ".staging-interrupted"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cachePath, ".staging-interrupted", "partial"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	before := provider.calls()
	restarted, err := NewIndexedService([]Profile{profile}, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	recovered := awaitReady(t, restarted)
	if recovered.Generation == first.Generation || provider.calls() <= before {
		t.Fatal("corrupt cache was reused")
	}
	if _, err := os.Stat(filepath.Join(cachePath, ".staging-interrupted")); !os.IsNotExist(err) {
		t.Fatal("interrupted staging survived")
	}
}
func TestCacheRejectsSymlinksAndSpecialFiles(t *testing.T) {
	for _, component := range []string{".qrouton", "installation"} {
		t.Run(component, func(t *testing.T) {
			root := t.TempDir()
			profile := Profile{ID: "shared", Name: "Shared", Root: root}
			put(t, root, "session/R1.md", fixtureBytes(t))
			outside := filepath.Join(root, "other")
			if err := os.Mkdir(outside, 0700); err != nil {
				t.Fatal(err)
			}
			put(t, outside, "marker", []byte("untouched"))
			target := filepath.Join(root, ".qrouton")
			if component == "installation" {
				if err := os.MkdirAll(filepath.Join(root, ".qrouton/index"), 0700); err != nil {
					t.Fatal(err)
				}
				target = filepath.Join(root, ".qrouton/index/installation")
			}
			relative, err := filepath.Rel(filepath.Dir(target), outside)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(relative, target); err != nil {
				t.Fatal(err)
			}
			options := IndexOptions{Provider: newFakeIndexProvider(), InstallationID: "installation", StateDir: t.TempDir(), ReconcileInterval: time.Hour}
			service, err := NewIndexedService([]Profile{profile}, nil, options)
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			awaitIndex(t, service, func(status IndexStatus) bool { return status.State == StateUnavailable })
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 1 {
				t.Fatalf("symlink target changed: %v %v", entries, err)
			}
			if _, err := service.Read(Scope{ReadProfiles: []string{"shared"}}, Reference{ID: "session/R1"}); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("fifo", func(t *testing.T) {
		provider := newFakeIndexProvider()
		service, profile, options := indexedFixture(t, provider)
		awaitReady(t, service)
		service.Close()
		current := filepath.Join(profile.Root, ".qrouton/index", options.InstallationID, currentFilename)
		if err := os.Remove(current); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(current, 0600); err != nil {
			t.Fatal(err)
		}
		restarted, err := NewIndexedService([]Profile{profile}, nil, options)
		if err != nil {
			t.Fatal(err)
		}
		defer restarted.Close()
		awaitIndex(t, restarted, func(status IndexStatus) bool { return status.State == StateUnavailable })
	})
}
func TestWorkerOwnershipCancelsAndDoesNotBlockReads(t *testing.T) {
	provider := newFakeIndexProvider()
	service, profile, options := indexedFixture(t, provider)
	awaitReady(t, service)
	second, err := NewIndexedService([]Profile{profile}, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	awaitIndex(t, second, func(status IndexStatus) bool { return status.State == IndexWaiting })
	if _, err := second.Read(Scope{ReadProfiles: []string{"shared"}}, Reference{ID: "session/R1"}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	second.Close()
	if time.Since(start) > time.Second {
		t.Fatal("writer lock ignored cancellation")
	}
}
func TestWriterLockAcrossProcesses(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	lock, err := acquireWriter(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseWriter(lock)
	command := exec.Command(os.Args[0], "-test.run=^TestWriterLockProcessHelper$")
	command.Env = append(os.Environ(), "QROUTON_VAULT_LOCK_FIXTURE="+directory)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("child lock check: %v %s", err, output)
	}
}
func TestWriterLockProcessHelper(t *testing.T) {
	directory := os.Getenv("QROUTON_VAULT_LOCK_FIXTURE")
	if directory == "" {
		t.Skip("subprocess fixture")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	file, err := acquireWriter(ctx, root)
	if file != nil {
		releaseWriter(file)
		t.Fatal("acquired held process lock")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
func TestLineageCyclesCompetingAndUnresolved(t *testing.T) {
	docs := map[string]Document{}
	for _, id := range []string{"session/A", "session/B", "session/C", "session/D", "session/E"} {
		d := fixtureDocument()
		d.ID = id
		docs[id] = d
	}
	a := docs["session/A"]
	a.Lineage = []Edge{{Relation: "derived_from", Target: "session/B"}}
	docs[a.ID] = a
	b := docs["session/B"]
	b.Lineage = []Edge{{Relation: "derived_from", Target: a.ID}}
	docs[b.ID] = b
	c := docs["session/C"]
	c.Lineage = []Edge{{Relation: "supersedes", Target: "session/E"}, {Relation: "derived_from", Target: "missing/R1"}}
	docs[c.ID] = c
	d := docs["session/D"]
	d.Lineage = []Edge{{Relation: "supersedes", Target: "session/E"}}
	docs[d.ID] = d
	lineage := ResolveLineage(docs)
	if !lineage[a.ID].Invalid || !lineage[b.ID].Invalid || len(lineage[c.ID].Unresolved) != 1 || lineage["session/E"].EffectiveState != "superseded" || len(lineage["session/E"].Successors) != 2 {
		t.Fatalf("%+v", lineage)
	}
}

func TestOversizedHeadingQuarantinesOnlyItsDocument(t *testing.T) {
	provider := newFakeIndexProvider()
	service, profile, _ := indexedFixture(t, provider)
	awaitReady(t, service)
	document := fixtureDocument()
	document.ID = "session/R2"
	document.Body = "# " + strings.Repeat("long ", 300) + "\n\nContent.\n"
	b, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	put(t, profile.Root, "session/R2.md", b)
	service.Retry(profile.ID)
	status := awaitIndex(t, service, func(status IndexStatus) bool {
		return status.State == StateIncomplete && status.Excluded == 1 && status.Pending == 0
	})
	if status.Indexed != 1 {
		t.Fatal(status)
	}
	if _, err := service.Read(Scope{ReadProfiles: []string{"shared"}}, Reference{ID: document.ID}); err != nil {
		t.Fatal(err)
	}
}
func TestDynamicRootOverlapStopsWorker(t *testing.T) {
	shared, private := t.TempDir(), t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(shared, alias); err != nil {
		t.Fatal(err)
	}
	profiles := []Profile{{ID: "shared", Name: "Shared", Root: alias}, {ID: "private", Name: "Private", Root: private}}
	put(t, shared, "session/R1.md", fixtureBytes(t))
	options := IndexOptions{Provider: newFakeIndexProvider(), InstallationID: "installation", StateDir: t.TempDir(), ReconcileInterval: time.Hour}
	service, err := NewIndexedService(profiles, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	awaitReady(t, service)
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(private, alias); err != nil {
		t.Fatal(err)
	}
	service.Retry("shared")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		service.workers["shared"].mu.RLock()
		status := service.workers["shared"].status
		service.workers["shared"].mu.RUnlock()
		if status.State == StateUnavailable && status.Error == ErrInvalidConfig.Error() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("worker did not reject dynamically overlapping roots")
}
func TestFailedCacheWriteRetainsCompatibleGenerationAndExcludesRemovals(t *testing.T) {
	provider := newFakeIndexProvider()
	service, profile, options := indexedFixture(t, provider)
	first := awaitReady(t, service)
	gate := make(chan struct{})
	provider.mu.Lock()
	provider.gate = gate
	provider.mu.Unlock()
	writeDocument(t, profile, "session/R2")
	service.Retry(profile.ID)
	awaitIndex(t, service, func(status IndexStatus) bool { return status.State == IndexBuilding && status.Pending == 1 })
	current := filepath.Join(profile.Root, ".qrouton/index", options.InstallationID, currentFilename)
	if err := os.Remove(current); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(current, 0600); err != nil {
		t.Fatal(err)
	}
	close(gate)
	status := awaitIndex(t, service, func(status IndexStatus) bool { return status.State == StateIncomplete && status.Error != "" })
	if status.Generation != first.Generation || status.Indexed != 1 || status.Pending != 1 {
		t.Fatal(status)
	}
	if err := os.Remove(filepath.Join(profile.Root, "session/R1.md")); err != nil {
		t.Fatal(err)
	}
	status = awaitIndex(t, service, func(status IndexStatus) bool { return status.Indexed == 0 })
	if status.Indexed != 0 {
		t.Fatal("removed artifact retained coverage")
	}
}

func TestSourceMutationDuringEmbeddingNeverActivatesOldChunks(t *testing.T) {
	provider := newFakeIndexProvider()
	gate := make(chan struct{})
	provider.gate = gate
	service, profile, _ := indexedFixture(t, provider)
	select {
	case <-provider.started:
	case <-time.After(3 * time.Second):
		t.Fatal("embedding never started")
	}
	put(t, profile.Root, "session/R1.md", append(fixtureBytes(t), []byte("Sync edited immutable bytes\n")...))
	close(gate)
	service.Retry(profile.ID)
	status := awaitIndex(t, service, func(status IndexStatus) bool {
		return status.State == StateIncomplete && status.Excluded == 1 && status.Pending == 0
	})
	if status.Chunks != 0 || status.Indexed != 0 {
		t.Fatal(status)
	}
}

func TestPeriodicReconciliationFindsInitiallyMissingRoot(t *testing.T) {
	profile := Profile{ID: "shared", Name: "Shared", Root: filepath.Join(t.TempDir(), "missing")}
	options := IndexOptions{Provider: newFakeIndexProvider(), InstallationID: "installation", StateDir: t.TempDir(), ReconcileInterval: 30 * time.Millisecond}
	service, err := NewIndexedService([]Profile{profile}, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	awaitIndex(t, service, func(status IndexStatus) bool { return status.State == StateUnavailable })
	put(t, profile.Root, "session/R1.md", fixtureBytes(t))
	if status := awaitReady(t, service); status.Indexed != 1 {
		t.Fatal(status)
	}
}

func TestCacheOwnershipAcrossProcessesWithDifferentProfileIDs(t *testing.T) {
	provider := newFakeIndexProvider()
	provider.gate = make(chan struct{})
	service, profile, options := indexedFixture(t, provider)
	select {
	case <-provider.started:
	case <-time.After(3 * time.Second):
		t.Fatal("writer did not reach embedding")
	}
	staging := filepath.Join(profile.Root, ".qrouton/index", options.InstallationID, ".staging-owned")
	if err := os.Mkdir(staging, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(staging, "marker")
	if err := os.WriteFile(marker, []byte("owned by first writer"), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestCacheOwnershipProcessHelper$")
	command.Env = append(os.Environ(), "QROUTON_CACHE_LOCK_ROOT="+profile.Root, "QROUTON_CACHE_LOCK_STATE="+options.StateDir, "QROUTON_CACHE_LOCK_INSTALL="+options.InstallationID)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("competing process: %v %s", err, output)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("competing profile cleaned owned staging:", err)
	}
	service.Close()
}
func TestCacheOwnershipProcessHelper(t *testing.T) {
	root := os.Getenv("QROUTON_CACHE_LOCK_ROOT")
	if root == "" {
		t.Skip("subprocess fixture")
	}
	provider := newFakeIndexProvider()
	profile := Profile{ID: "other-profile", Name: "Other", Root: root}
	options := IndexOptions{Provider: provider, StateDir: os.Getenv("QROUTON_CACHE_LOCK_STATE"), InstallationID: os.Getenv("QROUTON_CACHE_LOCK_INSTALL"), ReconcileInterval: time.Hour}
	service, err := NewIndexedService([]Profile{profile}, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	deadline := time.Now().Add(3 * time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		status := service.Status(Scope{ReadProfiles: []string{profile.ID}})
		_, ledgerErr := os.Stat(filepath.Join(profileStatePath(options.StateDir, profile), identityFilename))
		if ledgerErr == nil && len(status.Profiles) == 1 && status.Profiles[0].Index.State == IndexWaiting {
			waiting = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("different profile did not wait for physical cache ownership")
	}
	time.Sleep(100 * time.Millisecond)
	if provider.calls() != 0 {
		t.Fatal("competing profile reached embedding while cache owned")
	}
	start := time.Now()
	service.Close()
	if time.Since(start) > time.Second {
		t.Fatal("cache lock ignored cancellation")
	}
}
func TestRestartStatusCountsDurableIdentityConflict(t *testing.T) {
	provider := newFakeIndexProvider()
	service, profile, options := indexedFixture(t, provider)
	awaitReady(t, service)
	service.Close()
	put(t, profile.Root, "session/R1.md", append(fixtureBytes(t), []byte("Edited canonical bytes\n")...))
	restarted, err := NewIndexedService([]Profile{profile}, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	scope := Scope{ReadProfiles: []string{profile.ID}}
	status := restarted.Status(scope)
	if len(status.Profiles) != 1 || status.Profiles[0].Documents != 0 || status.Profiles[0].Conflicts != 1 {
		t.Fatalf("durable conflict count: %+v", status.Profiles)
	}
	if _, err := restarted.Read(scope, Reference{ID: "session/R1"}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
