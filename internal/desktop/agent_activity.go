package desktop

import (
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/launch"
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
	Description string
	State       string
	ParentKnown bool
	Root        bool
	Generation  uint64
	StartedAt   time.Time
	FinishedAt  time.Time

	// spawn is the tool_use_id of the Agent call this record is placed by, and
	// confirmed means that call's own result named it.
	spawn     string
	confirmed bool

	Outcome     string
	Tokens      int
	DurationMs  int64
	ToolCount   int
	LastMessage string
	Tools       []agentToolCall
	Files       []string

	// Reason, Tool and Target are filled when a snapshot derives the state.
	// A failed record keeps its reason.
	Reason string
	Tool   string
	Target string

	// resumedAfter is when the activation this agent resumed from finished.
	resumedAfter time.Time

	// needs is the root's: why the session waits on the user, which agent
	// asked, and the ask call whose result clears it.
	needs     string
	needsTool string
	needsFrom string
	needsAt   time.Time
	// turnEnded and background are the root's: its turn is over, and these
	// background agents have not reported their outcome yet. Hooks race, so
	// a stop or prompt older than the latest of the other is stale.
	turnEnded  bool
	background map[string]bool
	promptAt   time.Time
	stopAt     time.Time
}

type agentToolCall struct {
	ID     string
	Tool   string
	Target string
	Start  time.Time
	End    time.Time
	Failed bool
	Closed bool
}

type agentActivitySnapshot struct {
	Provider     string
	Generation   uint64
	Running      bool
	Unread       bool
	Active       int
	Capabilities agentCapabilities
	Records      []agentRecord
	// LiveTypes are the types of the current run's agents still running.
	LiveTypes []string
}

type agentRecordKey struct {
	provider string
	runID    string
	id       string
	root     bool
}

type agentActivity struct {
	mu  sync.Mutex
	now func() time.Time

	provider   string
	generation uint64
	setupRuns  uint64
	// lastRun is the run whose tree is kept until the next one starts.
	lastRun string
	running bool
	waiting bool
	unread  bool
	rung    bool
	spoke   time.Time
	records map[agentRecordKey]*agentRecord
	run     *agentRun
}

func newAgentActivity(now func() time.Time) *agentActivity {
	return &agentActivity{
		now:     now,
		records: map[agentRecordKey]*agentRecord{},
		run:     newAgentRun(),
	}
}

func (a *agentActivity) begin(provider string, generation uint64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if generation == a.generation && a.generation != 0 {
		return false
	}
	now := a.now()
	a.finishActiveLocked(now, agentStateEnded)
	a.keepOnlyLocked(a.lastRun)
	a.provider = provider
	a.generation = generation
	a.run = newAgentRun()
	a.running = true
	a.waiting = false
	a.unread = false
	a.rung = false
	a.spoke = time.Time{}
	runID := strconv.FormatUint(generation, 10)
	a.lastRun = runID
	a.records[a.keyLocked(agentRootID, true)] = &agentRecord{
		ID: agentRootID, RunID: runID, Provider: provider,
		State: agentLive, Root: true, ParentKnown: true,
		Generation: generation, StartedAt: now, turnEnded: true,
	}
	return true
}

// lifecycle is a runner that reports only its children starting and stopping.
// A child hangs off the agent its parent names when that agent is known, and
// off the root otherwise.
func (a *agentActivity) lifecycle(event workbench.DelegatedLifecycleRequest) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.currentLocked(event.Provider, event.Generation) || event.ID == "" {
		return false
	}
	now := a.now()
	key := a.keyLocked(event.ID, false)
	record := a.records[key]
	parentID := agentRootID
	if event.ParentID != "" && a.records[a.keyLocked(event.ParentID, false)] != nil {
		parentID = event.ParentID
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
			ID: event.ID, RunID: a.lastRun, Provider: event.Provider,
			ParentID: parentID, Type: event.Type, State: agentLive, ParentKnown: true,
			Generation: event.Generation, StartedAt: started,
		}
	case workbench.LifecycleStop:
		if record == nil {
			a.records[key] = &agentRecord{
				ID: event.ID, RunID: a.lastRun, Provider: event.Provider,
				ParentID: parentID, Type: event.Type, State: agentStateDone, ParentKnown: true,
				Generation: event.Generation, FinishedAt: now,
			}
			break
		}
		if record.State != agentLive {
			return false
		}
		record.State = agentStateDone
		record.FinishedAt = now
	default:
		return false
	}
	a.capLocked()
	return true
}

func (a *agentActivity) currentLocked(provider string, generation uint64) bool {
	return a.running && generation == a.generation && provider == a.provider
}

func (a *agentActivity) keyLocked(id string, root bool) agentRecordKey {
	return agentRecordKey{provider: a.provider, runID: a.lastRun, id: id, root: root}
}

// attention is a lifecycle-only runner's own word on its root. Waiting and a
// turn ending with nothing delegated still running ask for the chime.
func (a *agentActivity) attention(generation uint64, state string) (changed, alert bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running || generation != a.generation {
		return false, false
	}
	switch state {
	case status.ActivityWaiting:
		a.waiting = true
		a.unread = true
		return true, true
	case status.ActivityWorking:
		a.waiting = false
		a.spoke = a.now()
		return true, false
	case status.ActivityTurnEnded:
		if a.delegatingLocked() {
			return false, false
		}
		a.waiting = true
		a.unread = true
		return true, true
	}
	return false, false
}

// ring claims the one chime a session gets between two things the user types,
// whichever of its runner's hooks or tools asks for it first.
func (a *agentActivity) ring() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rung {
		return false
	}
	a.rung = true
	return true
}

func (a *agentActivity) delegatingLocked() bool {
	for _, record := range a.records {
		if !record.Root && record.Generation == a.generation && record.State == agentLive {
			return true
		}
	}
	return false
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
	if root := a.rootLocked(); root != nil {
		root.needs, root.needsTool, root.needsFrom = "", "", ""
	}
	a.waiting = false
	a.unread = false
	a.rung = false
	a.spoke = a.now()
}

func (a *agentActivity) seen() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.unread = false
}

// state is what the rail says about this session. A runner that reports its
// tool calls is read from its events; any other from its own waiting hook and
// the PTY's timing, never its contents.
func (a *agentActivity) state() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	root := a.rootLocked()
	if !a.toolsLocked() || root == nil || root.State != agentLive {
		return a.stateLocked(now)
	}
	switch state, _, _, _ := a.displayLocked(root, now); state {
	case agentStateNeedsYou, agentStateYourTurn:
		return status.ActivityWaiting
	case agentStateWorking, agentStateWaitingAgents:
		return status.ActivityWorking
	}
	return status.ActivityIdle
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

func (a *agentActivity) exitWithProvider(provider string, code int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	rootState := agentStateDone
	if code != 0 {
		rootState = agentStateFailed
	}
	if !a.running {
		if provider == "" {
			return false
		}
		a.setupRuns++
		runID := agentSetupRunPrefix + strconv.FormatUint(a.setupRuns, 10)
		a.keepOnlyLocked(a.lastRun)
		a.lastRun = runID
		a.provider = provider
		a.waiting = false
		a.spoke = time.Time{}
		key := agentRecordKey{provider: provider, runID: runID, id: agentRootID, root: true}
		a.records[key] = &agentRecord{
			ID: agentRootID, RunID: runID, Provider: provider, ParentKnown: true,
			State: rootState, Root: true, StartedAt: now, FinishedAt: now,
		}
		return true
	}
	if root := a.records[a.keyLocked(agentRootID, true)]; root != nil {
		root.State = rootState
		root.FinishedAt = now
	}
	a.finishActiveLocked(now, agentStateEnded)
	a.spoke = time.Time{}
	return true
}

func (a *agentActivity) snapshot() agentActivitySnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	records := make([]agentRecord, 0, len(a.records))
	active := 0
	var live []string
	for _, record := range a.records {
		if record.State == agentLive {
			active++
			if a.running && !record.Root && record.Generation == a.generation && record.Type != "" {
				live = append(live, record.Type)
			}
		}
		copy := *record
		copy.Tools = slices.Clone(record.Tools)
		copy.Files = slices.Clone(record.Files)
		copy.background = nil
		copy.State, copy.Reason, copy.Tool, copy.Target = a.displayLocked(record, now)
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
		Provider: a.provider, Generation: a.generation, Running: a.running, Unread: a.running && a.unread,
		Active: active, Capabilities: capabilitiesFor(a.provider), Records: records,
		LiveTypes: live,
	}
}

// finishActiveLocked closes the run: whatever had not stopped by then ended
// with it rather than finishing.
func (a *agentActivity) finishActiveLocked(now time.Time, state string) {
	for _, record := range a.records {
		if record.State != agentLive {
			continue
		}
		record.State = state
		record.FinishedAt = now
	}
	a.running = false
	a.waiting = false
}

// keepOnlyLocked drops every run but one, which is how the previous run's tree
// stays on screen until the next relaunch.
func (a *agentActivity) keepOnlyLocked(runID string) {
	for key := range a.records {
		if key.runID != runID {
			delete(a.records, key)
		}
	}
}

// capLocked drops the oldest finished leaves once the tree outgrows its cap. A
// live agent or one with children is never dropped.
func (a *agentActivity) capLocked() {
	if len(a.records) <= agentRecordCap {
		return
	}
	type runAgent struct{ run, id string }
	parents := map[runAgent]bool{}
	for key, record := range a.records {
		parents[runAgent{key.runID, record.ParentID}] = true
	}
	var leaves []agentRecordKey
	for key, record := range a.records {
		if record.Root || record.State == agentLive || parents[runAgent{key.runID, record.ID}] {
			continue
		}
		leaves = append(leaves, key)
	}
	sort.Slice(leaves, func(i, j int) bool {
		return a.records[leaves[i]].FinishedAt.Before(a.records[leaves[j]].FinishedAt)
	})
	for _, key := range leaves {
		if len(a.records) <= agentRecordCap {
			return
		}
		delete(a.records, key)
	}
}

func agentStateFor(activity string) string {
	switch activity {
	case status.ActivityWaiting:
		return agentStateYourTurn
	case status.ActivityWorking:
		return agentStateWorking
	default:
		return agentStateIdle
	}
}

func capabilitiesFor(provider string) agentCapabilities {
	hooks := launch.RunnerHooks(provider)
	tools := slices.Contains(hooks, agentevent.HookPostToolUse)
	return agentCapabilities{
		Attention: slices.Contains(hooks, agentevent.HookNotification),
		Children:  slices.Contains(hooks, agentevent.HookSubagentStart),
		Parents:   tools,
		Outcomes:  tools,
	}
}
