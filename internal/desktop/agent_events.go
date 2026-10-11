package desktop

import (
	"slices"
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
	// calls names the agent each tool call belongs to, including calls the
	// history has since dropped, so a late hook for one is not counted twice.
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

// apply folds one reduced hook into the tree. alert means the root has just
// come to need the user, or just handed the turn back, which claims the chime.
func (a *agentActivity) apply(event workbench.AgentEventRequest) (changed, alert bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.currentLocked(event.Provider, event.Generation) {
		return false, false
	}
	at := event.Timestamp
	if at.IsZero() {
		at = a.now()
	}
	before := a.rootStateLocked()
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
		changed = a.promptLocked(event, at)
	case agentevent.HookStop:
		changed = a.stopLocked(event, at)
	case agentevent.HookStopFailure:
		changed = a.needsLocked(event.AgentID, event.AgentType, event.Message, "", at)
	case agentevent.HookNotification:
		if event.NotificationType == notificationPermission || event.NotificationType == notificationElicitation {
			changed = a.needsLocked(event.AgentID, event.AgentType, event.Message, "", at)
		}
	}
	if !changed {
		return false, false
	}
	a.capLocked()
	after := a.rootStateLocked()
	alert = after != before && (after == agentStateNeedsYou || after == agentStateYourTurn)
	if alert {
		a.unread = true
	}
	return true, alert
}

func (a *agentActivity) rootLocked() *agentRecord {
	return a.records[a.keyLocked(agentRootID, true)]
}

func (a *agentActivity) rootStateLocked() string {
	root := a.rootLocked()
	if root == nil {
		return ""
	}
	state, _, _, _ := a.displayLocked(root, a.now())
	return state
}

// reopenLocked returns a finished agent to work when one of its hooks was
// stamped after it finished, whichever order the hooks arrive in. History,
// tokens and the tool count stay.
func reopenLocked(agent *agentRecord, at time.Time) {
	if agent.Root || agent.State == agentLive || !at.After(agent.FinishedAt) {
		return
	}
	agent.resumedAfter = agent.FinishedAt
	agent.State, agent.FinishedAt, agent.Outcome, agent.Reason = agentLive, time.Time{}, "", ""
}

// stale is a stop or outcome stamped before the agent last resumed,
// which belongs to the activation that already finished.
func stale(agent *agentRecord, at time.Time) bool {
	return !agent.resumedAfter.IsZero() && !at.After(agent.resumedAfter)
}

func (a *agentActivity) toolsLocked() bool { return capabilitiesFor(a.provider).Parents }

// displayLocked derives what an agent is doing from what its hooks said: a
// finished agent keeps its outcome, a runner without tool hooks is timed by
// its terminal, and otherwise the open call, the user, or its agents decide.
func (a *agentActivity) displayLocked(r *agentRecord, now time.Time) (state, reason, tool, target string) {
	if r.State != agentLive {
		return r.State, r.Reason, "", ""
	}
	if !a.toolsLocked() {
		if r.Root {
			return agentStateFor(a.stateLocked(now)), "", "", ""
		}
		return agentStateWorking, "", "", ""
	}
	if r.needs != "" {
		return agentStateNeedsYou, r.needs, "", ""
	}
	spawning := false
	for i := len(r.Tools) - 1; i >= 0; i-- {
		call := r.Tools[i]
		if call.Closed {
			continue
		}
		if call.Tool == agentevent.ToolAgent || call.Tool == agentevent.ToolTaskLegacy {
			spawning = true
			continue
		}
		return agentStateWorking, "", call.Tool, call.Target
	}
	waiting := spawning || r.held || a.liveChildLocked(r)
	if r.Root && r.turnEnded {
		if waiting || len(r.background) > 0 {
			return agentStateWaitingAgents, "", "", ""
		}
		return agentStateYourTurn, "", "", ""
	}
	if waiting {
		return agentStateWaitingAgents, "", "", ""
	}
	return agentStateWorking, "", "", ""
}

func (a *agentActivity) liveChildLocked(parent *agentRecord) bool {
	for key, record := range a.records {
		if key.runID == parent.RunID && !record.Root && record.ParentID == parent.ID && record.State == agentLive {
			return true
		}
	}
	return false
}

// needsLocked puts the root in front of the user. Only the orchestrator talks
// to the user, so a request any agent raises is the root's to show. tool is
// the ask call whose result answers it; without one, the asking agent's next
// tool call does.
func (a *agentActivity) needsLocked(id, typ, reason, tool string, at time.Time) bool {
	asker := a.agentLocked(id, typ, at)
	root := a.rootLocked()
	if asker == nil || root == nil || root.State != agentLive {
		return false
	}
	if reason == "" {
		reason = agentReasonQuestion
	}
	root.needs, root.needsTool, root.needsFrom, root.needsAt = reason, tool, asker.ID, at
	return true
}

// answeredLocked clears the root's request once the call that asked it
// returns, or the agent that asked moves on to another tool.
func (a *agentActivity) answeredLocked(caller *agentRecord, toolUse string, at time.Time) {
	root := a.rootLocked()
	if root == nil || root.needs == "" {
		return
	}
	if (root.needsTool != "" && root.needsTool == toolUse) ||
		(root.needsTool == "" && root.needsFrom == caller.ID && at.After(root.needsAt)) {
		root.needs, root.needsTool, root.needsFrom = "", "", ""
	}
}

// promptLocked is the user, or a background agent's result, starting the
// root's next turn.
func (a *agentActivity) promptLocked(event workbench.AgentEventRequest, at time.Time) bool {
	for _, n := range event.Notifications {
		a.taskNotificationLocked(n, at)
	}
	if event.AgentID != "" {
		return len(event.Notifications) > 0
	}
	root := a.rootLocked()
	if root == nil || at.Before(root.stopAt) {
		return len(event.Notifications) > 0
	}
	root.promptAt = at
	root.turnEnded = false
	root.needs, root.needsTool, root.needsFrom = "", "", ""
	return true
}

func (a *agentActivity) stopLocked(event workbench.AgentEventRequest, at time.Time) bool {
	if event.AgentID != "" {
		return false
	}
	root := a.rootLocked()
	if root == nil || at.Before(root.promptAt) {
		return false
	}
	root.stopAt = at
	root.turnEnded = true
	root.needs, root.needsTool, root.needsFrom = "", "", ""
	root.background = map[string]bool{}
	for _, id := range event.BackgroundTasks {
		if child := a.records[a.keyLocked(id, false)]; child == nil || child.Outcome == "" {
			root.background[id] = true
		}
	}
	if event.Message != "" {
		root.LastMessage = event.Message
	}
	return true
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
		Type: typ, State: agentLive, Generation: a.generation, StartedAt: at,
	}
	a.records[key] = record
	return record
}

func (a *agentActivity) toolStartLocked(event workbench.AgentEventRequest, at time.Time) bool {
	caller := a.agentLocked(event.AgentID, event.AgentType, at)
	if caller == nil {
		return false
	}
	if _, seen := a.run.calls[event.ToolUseID]; !seen {
		a.answeredLocked(caller, "", at)
	}
	reopenLocked(caller, at)
	if caller.held && at.After(caller.heldAt) {
		caller.held = false
	}
	answered := false
	if event.ToolUseID != "" {
		if _, seen := a.run.calls[event.ToolUseID]; seen {
			answered = true
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
	switch {
	case answered:
	case event.Tool == agentevent.ToolAskUserQuestion:
		a.needsLocked(event.AgentID, event.AgentType, firstOf(event.Target, agentReasonQuestion), event.ToolUseID, at)
	case event.Tool == agentevent.ToolExitPlanMode:
		a.needsLocked(event.AgentID, event.AgentType, agentReasonPlan, event.ToolUseID, at)
	}
	return true
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (a *agentActivity) toolEndLocked(event workbench.AgentEventRequest, at time.Time) bool {
	caller := a.agentLocked(event.AgentID, event.AgentType, at)
	if caller == nil {
		return false
	}
	failed := event.Hook == agentevent.HookPostToolUseFailure
	reopenLocked(caller, at)
	a.answeredLocked(caller, event.ToolUseID, at)
	if event.Interrupt && caller.Root && !at.Before(caller.promptAt) {
		caller.turnEnded, caller.stopAt = true, at
	}
	if !failed && event.Target != "" && slices.Contains(agentevent.FileWriteTools, event.Tool) &&
		!slices.Contains(caller.Files, event.Target) && len(caller.Files) < agentFileLimit {
		caller.Files = append(caller.Files, event.Target)
	}
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
	if failed && event.Spawn != nil {
		if spawn := a.run.spawns[event.ToolUseID]; spawn != nil && spawn.child != "" {
			if child := a.records[a.keyLocked(spawn.child, false)]; child != nil && child.State == agentLive {
				child.State, child.Reason, child.FinishedAt = agentStateFailed, event.Message, at
			}
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
	reopenLocked(record, at)
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
	if stale(record, at) {
		return false
	}
	if record.spawn == "" {
		a.matchLocked(record)
	}
	if event.Message != "" {
		record.LastMessage = event.Message
	}
	// background_tasks lists the whole session's tasks, the stopping agent's
	// own included, so only its live children in the tree hold it open.
	if record.State == agentLive && a.liveChildLocked(record) {
		record.held, record.heldAt = true, at
		return true
	}
	record.held = false
	if record.State == agentLive {
		record.State = agentStateDone
	}
	if record.FinishedAt.IsZero() {
		record.FinishedAt = at
	}
	return true
}

func (a *agentActivity) taskNotificationLocked(n workbench.TaskNotification, at time.Time) {
	if root := a.rootLocked(); root != nil {
		delete(root.background, n.TaskID)
	}
	var child *agentRecord
	if spawn := a.run.spawns[n.ToolUseID]; spawn != nil {
		child = a.confirmLocked(spawn, n.TaskID, at)
	} else if child = a.records[a.keyLocked(n.TaskID, false)]; child == nil {
		return
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
		if record.spawn == "" && !record.Root && !record.ParentKnown && record.State == agentLive &&
			record.ID != spawn.caller && record.Type == spawn.typ {
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
			State: agentLive, Generation: a.generation, StartedAt: at,
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
	if stale(record, at) {
		return
	}
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
		record.State = agentStateDone
	} else {
		record.State = agentStateFailed
		record.Reason = outcome.Status
	}
	if record.FinishedAt.IsZero() {
		record.FinishedAt = at
	}
}
