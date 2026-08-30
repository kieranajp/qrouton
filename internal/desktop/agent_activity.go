package desktop

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type agentCapabilities struct {
	Attention bool
	Children  bool
	Parents   bool
	Outcomes  bool
}

type agentRecord struct {
	ID          string
	RunID       string
	Provider    string
	ParentID    string
	Type        string
	Role        string
	State       string
	ParentKnown bool
	Root        bool
	Generation  uint64
	StartedAt   time.Time
	FinishedAt  time.Time
}

type agentActivitySnapshot struct {
	Provider     string
	Running      bool
	Attention    bool
	Active       int
	Capabilities agentCapabilities
	Records      []agentRecord
}

type agentRecordKey struct {
	provider string
	runID    string
	id       string
	root     bool
}

type agentActivity struct {
	mu        sync.Mutex
	now       func() time.Time
	retention time.Duration

	provider   string
	generation uint64
	setupRuns  uint64
	running    bool
	waiting    bool
	spoke      time.Time
	records    map[agentRecordKey]*agentRecord
	stopped    map[agentRecordKey]struct{}
}

func newAgentActivity(now func() time.Time, retention time.Duration) *agentActivity {
	return &agentActivity{
		now:       now,
		retention: retention,
		records:   map[agentRecordKey]*agentRecord{},
		stopped:   map[agentRecordKey]struct{}{},
	}
}

func (a *agentActivity) begin(provider string, generation uint64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	a.pruneLocked(now)
	if generation == a.generation && a.generation != 0 {
		return false
	}
	a.finishActiveLocked(now, agentStateFinished)
	a.provider = provider
	a.generation = generation
	a.stopped = map[agentRecordKey]struct{}{}
	a.running = true
	a.waiting = false
	a.spoke = time.Time{}
	runID := strconv.FormatUint(generation, 10)
	key := agentRecordKey{provider: provider, runID: runID, id: agentRootID, root: true}
	a.records[key] = &agentRecord{
		ID: agentRootID, RunID: runID, Provider: provider,
		Role: agentRoleOrchestrator, State: agentStateActive, Root: true,
		Generation: generation, StartedAt: now,
	}
	return true
}

func (a *agentActivity) lifecycle(event workbench.DelegatedLifecycleRequest) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	a.pruneLocked(now)
	if !a.running || event.Generation != a.generation || event.Provider != a.provider || event.ID == "" {
		return false
	}
	runID := strconv.FormatUint(event.Generation, 10)
	key := agentRecordKey{provider: event.Provider, runID: runID, id: event.ID}
	if _, stopped := a.stopped[key]; stopped {
		return false
	}
	record := a.records[key]
	role := delegatedRole(event.Type)
	parentID := event.ParentID
	if role == agentRoleLead {
		parentID = agentRootID
	} else if parentID == "" {
		parentID = a.soleActiveLeadLocked(event.Provider, runID)
	}
	switch event.Kind {
	case workbench.LifecycleStart:
		if record != nil {
			return false
		}
		started := event.Timestamp
		if started.IsZero() {
			started = now
		}
		a.records[key] = &agentRecord{
			ID: event.ID, RunID: runID, Provider: event.Provider,
			ParentID: parentID, Type: event.Type, Role: role,
			State: agentStateActive, ParentKnown: parentID != "", Generation: event.Generation,
			StartedAt: started,
		}
		return true
	case workbench.LifecycleStop:
		a.stopped[key] = struct{}{}
		if record == nil {
			a.records[key] = &agentRecord{
				ID: event.ID, RunID: runID, Provider: event.Provider,
				ParentID: parentID, Type: event.Type, Role: role,
				State: agentStateFinished, ParentKnown: parentID != "", Generation: event.Generation,
				FinishedAt: now,
			}
			return true
		}
		if record.State != agentStateActive {
			return false
		}
		record.State = agentStateFinished
		record.FinishedAt = now
		return true
	default:
		return false
	}
}

func (a *agentActivity) soleActiveLeadLocked(provider, runID string) string {
	lead := ""
	for _, record := range a.records {
		if record.Provider != provider || record.RunID != runID || record.Role != agentRoleLead || record.State != agentStateActive {
			continue
		}
		if lead != "" {
			return ""
		}
		lead = record.ID
	}
	return lead
}

func (a *agentActivity) attention(generation uint64, state string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running || generation != a.generation {
		return false
	}
	switch state {
	case status.ActivityWaiting:
		a.waiting = true
	case status.ActivityWorking:
		a.waiting = false
		a.spoke = a.now()
	default:
		return false
	}
	return true
}

// output and input are the conversation PTY's own timing, which the workbench
// reads whether or not a runner has announced a generation to attribute it to.
func (a *agentActivity) output() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.spoke = a.now()
}

func (a *agentActivity) input() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waiting = false
	a.spoke = a.now()
}

// state is what the rail says about this session: waiting comes from the
// runner's own hook, working and idle are the PTY's timing, never its contents.
func (a *agentActivity) state() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stateLocked(a.now())
}

func (a *agentActivity) stateLocked(now time.Time) string {
	switch {
	case a.waiting:
		return status.ActivityWaiting
	case !a.spoke.IsZero() && now.Sub(a.spoke) < activityQuiet:
		return status.ActivityWorking
	default:
		return status.ActivityIdle
	}
}

func (a *agentActivity) exit(code int) bool {
	return a.exitWithProvider("", code)
}

func (a *agentActivity) exitWithProvider(provider string, code int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	a.pruneLocked(now)
	rootState := agentStateFinished
	if code != 0 {
		rootState = agentStateFailed
	}
	if !a.running {
		if provider == "" {
			return false
		}
		a.setupRuns++
		runID := agentSetupRunPrefix + strconv.FormatUint(a.setupRuns, 10)
		a.provider = provider
		a.waiting = false
		a.spoke = time.Time{}
		key := agentRecordKey{provider: provider, runID: runID, id: agentRootID, root: true}
		a.records[key] = &agentRecord{
			ID: agentRootID, RunID: runID, Provider: provider, Role: agentRoleOrchestrator,
			State: rootState, Root: true, StartedAt: now, FinishedAt: now,
		}
		return true
	}
	a.finishActiveLocked(now, agentStateFinished)
	runID := strconv.FormatUint(a.generation, 10)
	key := agentRecordKey{provider: a.provider, runID: runID, id: agentRootID, root: true}
	if root := a.records[key]; root != nil {
		root.State = rootState
		root.FinishedAt = now
	}
	a.running = false
	a.waiting = false
	a.spoke = time.Time{}
	return true
}

func (a *agentActivity) snapshot() agentActivitySnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	a.pruneLocked(now)
	records := make([]agentRecord, 0, len(a.records))
	active := 0
	for _, record := range a.records {
		if record.State == agentStateActive {
			active++
		}
		copy := *record
		if copy.Root && copy.State == agentStateActive {
			copy.State = agentStateFor(a.stateLocked(now))
		}
		records = append(records, copy)
	}
	sort.Slice(records, func(i, j int) bool {
		iCurrent := a.generation != 0 && records[i].Generation == a.generation
		jCurrent := a.generation != 0 && records[j].Generation == a.generation
		if iCurrent != jCurrent {
			return iCurrent
		}
		if records[i].Generation != records[j].Generation {
			return records[i].StartedAt.After(records[j].StartedAt)
		}
		if records[i].Root != records[j].Root {
			return records[i].Root
		}
		if !records[i].StartedAt.Equal(records[j].StartedAt) {
			return records[i].StartedAt.After(records[j].StartedAt)
		}
		if records[i].RunID != records[j].RunID {
			return records[i].RunID < records[j].RunID
		}
		return records[i].ID < records[j].ID
	})
	return agentActivitySnapshot{
		Provider: a.provider, Running: a.running, Attention: a.running && a.waiting,
		Active: active, Capabilities: capabilitiesFor(a.provider), Records: records,
	}
}

func (a *agentActivity) activeCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	count := 0
	for _, record := range a.records {
		if record.State == agentStateActive {
			count++
		}
	}
	return count
}

func (a *agentActivity) earliestExpiry() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	var earliest time.Time
	for _, record := range a.records {
		if record.FinishedAt.IsZero() {
			continue
		}
		expires := record.FinishedAt.Add(a.retention)
		if earliest.IsZero() || expires.Before(earliest) {
			earliest = expires
		}
	}
	return earliest
}

func (a *agentActivity) finishActiveLocked(now time.Time, delegatedState string) {
	for _, record := range a.records {
		if record.State != agentStateActive {
			continue
		}
		record.State = delegatedState
		record.FinishedAt = now
	}
	a.running = false
	a.waiting = false
}

func (a *agentActivity) pruneLocked(now time.Time) {
	for key, record := range a.records {
		if !record.FinishedAt.IsZero() && !now.Before(record.FinishedAt.Add(a.retention)) {
			delete(a.records, key)
		}
	}
}

func agentStateFor(activity string) string {
	switch activity {
	case status.ActivityWaiting:
		return agentStateWaiting
	case status.ActivityWorking:
		return agentStateWorking
	default:
		return agentStateIdle
	}
}

func capabilitiesFor(provider string) agentCapabilities {
	switch provider {
	case agentProviderClaude:
		return agentCapabilities{Attention: true, Children: true}
	case agentProviderCodex:
		return agentCapabilities{Children: true}
	case agentProviderOpenCode:
		return agentCapabilities{}
	default:
		return agentCapabilities{}
	}
}

func delegatedRole(agentType string) string {
	switch {
	case agentType == "":
		return agentRoleUnavailable
	case strings.HasSuffix(agentType, "-lead"):
		return agentRoleLead
	default:
		return agentRoleSpecialist
	}
}
