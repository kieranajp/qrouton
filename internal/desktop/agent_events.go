package desktop

import (
	"sort"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// agentRun is what one run's hooks have said that is not yet a record: the
// Agent calls waiting for a child, and every tool call by its id. Hooks arrive
// in any order, so each side of a pair is kept until the other turns up.
type agentRun struct {
	spawns  map[string]*agentSpawn
	pending []*agentSpawn
	// calls names the agent each tool call belongs to.
	calls map[string]string
}

// agentSpawn is one Agent tool call. child is provisional until confirmed,
// which only the call's own result or its task notification can do.
type agentSpawn struct {
	toolUse     string
	caller      string
	typ         string
	description string
	child       string
	confirmed   bool
}

func newAgentRun() *agentRun {
	return &agentRun{spawns: map[string]*agentSpawn{}, calls: map[string]string{}}
}

// apply folds one reduced hook into the tree, and reports whether the tree
// changed.
func (a *agentActivity) apply(event workbench.AgentEventRequest) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.currentLocked(event.Provider, event.Generation) {
		return false
	}
	at := event.Timestamp
	if at.IsZero() {
		at = a.now()
	}
	changed := false
	switch event.Hook {
	case agentevent.HookPreToolUse:
		changed = a.toolStartLocked(event, at)
	case agentevent.HookPostToolUse, agentevent.HookPostToolUseFailure:
		changed = a.toolEndLocked(event, at)
	case agentevent.HookSubagentStart:
		changed = a.subagentStartLocked(event, at)
	case agentevent.HookSubagentStop:
		changed = a.subagentStopLocked(event, at)
	case agentevent.HookUserPromptSubmit:
		for _, n := range event.Notifications {
			a.taskNotificationLocked(n, at)
			changed = true
		}
	}
	if changed {
		a.capLocked()
	}
	return changed
}

// agentLocked is the record a hook fired in: the root for an empty id, and a
// placeholder for an agent whose start has not arrived yet.
func (a *agentActivity) agentLocked(id, typ string, at time.Time) *agentRecord {
	if id == "" {
		return a.records[a.keyLocked(agentRootID, true)]
	}
	key := a.keyLocked(id, false)
	if record := a.records[key]; record != nil {
		if record.Type == "" {
			record.Type = typ
		}
		return record
	}
	record := &agentRecord{
		ID: id, RunID: a.lastRun, Provider: a.provider, ParentID: agentRootID,
		Type: typ, State: agentStateActive, Generation: a.generation, StartedAt: at,
	}
	a.records[key] = record
	return record
}

func (a *agentActivity) toolStartLocked(event workbench.AgentEventRequest, at time.Time) bool {
	caller := a.agentLocked(event.AgentID, event.AgentType, at)
	if caller == nil {
		return false
	}
	if event.ToolUseID != "" {
		if _, seen := a.run.calls[event.ToolUseID]; seen {
			if call := a.callLocked(event.ToolUseID); call != nil && call.Start.IsZero() {
				call.Start = at
			}
		} else {
			a.recordCallLocked(caller, agentToolCall{ID: event.ToolUseID, Tool: event.Tool, Target: event.Target, Start: at})
		}
	}
	if event.Spawn != nil && event.ToolUseID != "" {
		spawn := a.spawnLocked(event, caller)
		if spawn.child == "" {
			a.queueLocked(spawn)
		}
	}
	return true
}

func (a *agentActivity) toolEndLocked(event workbench.AgentEventRequest, at time.Time) bool {
	caller := a.agentLocked(event.AgentID, event.AgentType, at)
	if caller == nil {
		return false
	}
	failed := event.Hook == agentevent.HookPostToolUseFailure
	if event.ToolUseID != "" {
		if _, seen := a.run.calls[event.ToolUseID]; seen {
			if call := a.callLocked(event.ToolUseID); call != nil {
				call.End, call.Failed, call.Closed = at, failed, true
			}
		} else {
			a.recordCallLocked(caller, agentToolCall{
				ID: event.ToolUseID, Tool: event.Tool, Target: event.Target, End: at, Failed: failed, Closed: true,
			})
		}
	}
	if event.Spawn == nil || event.ToolUseID == "" || event.ChildID == "" {
		return true
	}
	child := a.confirmLocked(a.spawnLocked(event, caller), event.ChildID, at)
	if event.Outcome != nil {
		outcomeLocked(child, *event.Outcome, event.Message, at)
	}
	return true
}

func (a *agentActivity) subagentStartLocked(event workbench.AgentEventRequest, at time.Time) bool {
	if event.AgentID == "" {
		return false
	}
	record := a.agentLocked(event.AgentID, event.AgentType, at)
	if at.Before(record.StartedAt) {
		record.StartedAt = at
	}
	if record.spawn == "" {
		a.matchLocked(record)
	}
	return true
}

// subagentStopLocked marks an agent stopped. A stop naming neither a type nor
// a start the tracker saw is dropped: Claude Code sends some at exit for
// agents nothing else ever mentioned.
func (a *agentActivity) subagentStopLocked(event workbench.AgentEventRequest, at time.Time) bool {
	if event.AgentID == "" {
		return false
	}
	if a.records[a.keyLocked(event.AgentID, false)] == nil && event.AgentType == "" {
		return false
	}
	record := a.agentLocked(event.AgentID, event.AgentType, at)
	if record.spawn == "" {
		a.matchLocked(record)
	}
	if record.State == agentStateActive {
		record.State = agentStateFinished
	}
	if record.FinishedAt.IsZero() {
		record.FinishedAt = at
	}
	if event.Message != "" {
		record.LastMessage = event.Message
	}
	return true
}

func (a *agentActivity) taskNotificationLocked(n workbench.TaskNotification, at time.Time) {
	var child *agentRecord
	if spawn := a.run.spawns[n.ToolUseID]; spawn != nil {
		child = a.confirmLocked(spawn, n.TaskID, at)
	} else {
		child = a.agentLocked(n.TaskID, "", at)
	}
	outcomeLocked(child, n.Outcome, n.Result, at)
}

// spawnLocked is the Agent call an event names, made the first time either of
// its hooks arrives and filled in by whichever comes second.
func (a *agentActivity) spawnLocked(event workbench.AgentEventRequest, caller *agentRecord) *agentSpawn {
	spawn := a.run.spawns[event.ToolUseID]
	if spawn == nil {
		spawn = &agentSpawn{toolUse: event.ToolUseID}
		a.run.spawns[event.ToolUseID] = spawn
	}
	spawn.caller = caller.ID
	if spawn.typ == "" {
		spawn.typ = event.Spawn.Type
	}
	if spawn.description == "" {
		spawn.description = event.Spawn.Description
	}
	return spawn
}

// queueLocked waits a spawn for its child, unless an unplaced child of its
// type already started.
func (a *agentActivity) queueLocked(spawn *agentSpawn) {
	for _, record := range a.recordsByStartLocked() {
		if record.spawn == "" && !record.Root && !record.ParentKnown && record.ID != spawn.caller && record.Type == spawn.typ {
			a.bindLocked(record, spawn, false)
			return
		}
	}
	a.run.pending = append(a.run.pending, spawn)
}

// matchLocked places a child under the oldest Agent call of its type still
// waiting for one. Without one it hangs off the root, unplaced.
func (a *agentActivity) matchLocked(record *agentRecord) {
	for _, spawn := range a.run.pending {
		if spawn.typ == record.Type {
			a.bindLocked(record, spawn, false)
			return
		}
	}
	record.ParentID = agentRootID
	record.ParentKnown = false
}

// confirmLocked fixes a child to the call whose result named it. A provisional
// match that paired two same-type spawns the wrong way round is swapped back.
func (a *agentActivity) confirmLocked(spawn *agentSpawn, childID string, at time.Time) *agentRecord {
	child := a.records[a.keyLocked(childID, false)]
	if child == nil {
		child = &agentRecord{
			ID: childID, RunID: a.lastRun, Provider: a.provider, Type: spawn.typ,
			State: agentStateActive, Generation: a.generation, StartedAt: at,
		}
		a.records[a.keyLocked(childID, false)] = child
	}
	if child.confirmed && child.spawn == spawn.toolUse {
		return child
	}
	var displaced *agentRecord
	if spawn.child != "" && spawn.child != childID {
		displaced = a.records[a.keyLocked(spawn.child, false)]
	}
	var freed *agentSpawn
	if child.spawn != "" && child.spawn != spawn.toolUse {
		if previous := a.run.spawns[child.spawn]; previous != nil && !previous.confirmed {
			previous.child = ""
			freed = previous
		}
	}
	a.bindLocked(child, spawn, true)
	if displaced != nil && !displaced.confirmed {
		displaced.spawn = ""
		if freed != nil && freed.typ == displaced.Type {
			a.bindLocked(displaced, freed, false)
			freed = nil
		} else {
			a.matchLocked(displaced)
		}
	}
	if freed != nil {
		a.run.pending = append(a.run.pending, freed)
	}
	return child
}

func (a *agentActivity) bindLocked(child *agentRecord, spawn *agentSpawn, confirmed bool) {
	child.spawn = spawn.toolUse
	child.confirmed = confirmed
	child.ParentID = spawn.caller
	if child.ParentID == "" {
		child.ParentID = agentRootID
	}
	child.ParentKnown = true
	if child.Type == "" {
		child.Type = spawn.typ
	}
	if child.Description == "" {
		child.Description = spawn.description
	}
	spawn.child = child.ID
	spawn.confirmed = confirmed
	for i, waiting := range a.run.pending {
		if waiting == spawn {
			a.run.pending = append(a.run.pending[:i], a.run.pending[i+1:]...)
			break
		}
	}
}

func (a *agentActivity) recordsByStartLocked() []*agentRecord {
	out := make([]*agentRecord, 0, len(a.records))
	for key, record := range a.records {
		if key.runID == a.lastRun {
			out = append(out, record)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].StartedAt.Before(out[j].StartedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (a *agentActivity) recordCallLocked(agent *agentRecord, call agentToolCall) {
	agent.ToolCount++
	agent.Tools = append(agent.Tools, call)
	if extra := len(agent.Tools) - agentToolHistory; extra > 0 {
		for _, dropped := range agent.Tools[:extra] {
			delete(a.run.calls, dropped.ID)
		}
		agent.Tools = append([]agentToolCall(nil), agent.Tools[extra:]...)
	}
	a.run.calls[call.ID] = agent.ID
}

func (a *agentActivity) callLocked(id string) *agentToolCall {
	owner := a.run.calls[id]
	agent := a.records[a.keyLocked(owner, owner == agentRootID)]
	if agent == nil {
		return nil
	}
	for i := range agent.Tools {
		if agent.Tools[i].ID == id {
			return &agent.Tools[i]
		}
	}
	return nil
}

func outcomeLocked(record *agentRecord, outcome workbench.AgentOutcome, message string, at time.Time) {
	record.Outcome = outcome.Status
	if outcome.Tokens > 0 {
		record.Tokens = outcome.Tokens
	}
	if outcome.DurationMs > 0 {
		record.DurationMs = outcome.DurationMs
	}
	record.ToolCount = max(record.ToolCount, outcome.ToolUses)
	if message != "" && record.LastMessage == "" {
		record.LastMessage = message
	}
	if outcome.Status == agentOutcomeCompleted {
		record.State = agentStateFinished
	} else {
		record.State = agentStateFailed
	}
	if record.FinishedAt.IsZero() {
		record.FinishedAt = at
	}
}
