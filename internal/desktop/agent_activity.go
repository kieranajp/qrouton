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
	provider   string
	generation uint64
	id         string
	root       bool
}

type agentActivity struct {
	mu        sync.Mutex
	now       func() time.Time
	retention time.Duration

	provider   string
	generation uint64
	running    bool
	waiting    bool
	spoke      time.Time
	records    map[agentRecordKey]*agentRecord
}

func newAgentActivity(now func() time.Time, retention time.Duration) *agentActivity {
	return &agentActivity{now: now, retention: retention, records: map[agentRecordKey]*agentRecord{}}
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
	a.running = true
	a.waiting = false
	a.spoke = time.Time{}
	key := agentRecordKey{provider: provider, generation: generation, id: agentRootID, root: true}
	a.records[key] = &agentRecord{
		ID: agentRootID, RunID: strconv.FormatUint(generation, 10), Provider: provider,
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
	key := agentRecordKey{provider: event.Provider, generation: event.Generation, id: event.ID}
	record := a.records[key]
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
			ID: event.ID, RunID: strconv.FormatUint(event.Generation, 10), Provider: event.Provider,
			ParentID: event.ParentID, Type: event.Type, Role: delegatedRole(event.Type),
			State: agentStateActive, ParentKnown: event.ParentID != "", Generation: event.Generation,
			StartedAt: started,
		}
		return true
	case workbench.LifecycleStop:
		if record == nil {
			a.records[key] = &agentRecord{
				ID: event.ID, RunID: strconv.FormatUint(event.Generation, 10), Provider: event.Provider,
				ParentID: event.ParentID, Type: event.Type, Role: delegatedRole(event.Type),
				State: agentStateFinished, ParentKnown: event.ParentID != "", Generation: event.Generation,
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

func (a *agentActivity) output() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		a.spoke = a.now()
	}
}

func (a *agentActivity) input() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		a.waiting = false
		a.spoke = a.now()
	}
}

func (a *agentActivity) exit(code int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running {
		return false
	}
	now := a.now()
	rootState := agentStateFinished
	if code != 0 {
		rootState = agentStateFailed
	}
	a.finishActiveLocked(now, agentStateFinished)
	key := agentRecordKey{provider: a.provider, generation: a.generation, id: agentRootID, root: true}
	if root := a.records[key]; root != nil {
		root.State = rootState
		root.FinishedAt = now
	}
	a.running = false
	a.waiting = false
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
			switch {
			case a.waiting:
				copy.State = agentStateWaiting
			case !a.spoke.IsZero() && now.Sub(a.spoke) < activityQuiet:
				copy.State = agentStateWorking
			default:
				copy.State = agentStateIdle
			}
		}
		records = append(records, copy)
	}
	sort.Slice(records, func(i, j int) bool {
		if (records[i].Generation == a.generation) != (records[j].Generation == a.generation) {
			return records[i].Generation == a.generation
		}
		if records[i].Generation != records[j].Generation {
			return records[i].StartedAt.After(records[j].StartedAt)
		}
		if records[i].Root != records[j].Root {
			return records[i].Root
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

func capabilitiesFor(provider string) agentCapabilities {
	switch provider {
	case agentProviderClaude:
		return agentCapabilities{Attention: true, Children: true}
	case agentProviderCodex, agentProviderOpenCode:
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
