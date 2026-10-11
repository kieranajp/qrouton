package desktop

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/status"
)

// AgentTree answers the Agents pane on load; agentTreeEvent only fires on a
// change after that.
func (c *Chrome) AgentTree(slug string) status.AgentTree {
	c.mu.RLock()
	tree := c.tree
	c.mu.RUnlock()
	if tree == nil {
		return agentTree(agentActivitySnapshot{}, "")
	}
	return tree(slug)
}

func (c *Chrome) answerTrees(tree func(slug string) status.AgentTree) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tree = tree
}

// agentTree is what the Agents pane draws. fallback names the runner a session
// with no announced run was assembled with.
func agentTree(snapshot agentActivitySnapshot, fallback string) status.AgentTree {
	provider := snapshot.Provider
	if provider == "" {
		provider = fallback
	}
	tree := status.AgentTree{
		Provider: provider, ProviderLabel: launch.RunnerLabel(provider),
		Coverage: coverageNote(provider, capabilitiesFor(provider)), Tools: capabilitiesFor(provider).Parents,
		Nodes: []status.AgentNode{},
	}
	for _, record := range snapshot.Records {
		node := status.AgentNode{
			ID: record.ID, Run: record.RunID, Parent: record.ParentID, Root: record.Root,
			Placed: record.ParentKnown, Type: record.Type, Description: record.Description,
			State: record.State, Reason: record.Reason, Tool: record.Tool, Target: record.Target,
			Started: record.StartedAt, Finished: record.FinishedAt,
			ToolCount: record.ToolCount, Tokens: record.Tokens, LastMessage: record.LastMessage,
			Files: append([]string{}, record.Files...), Tools: make([]status.AgentToolEvent, 0, len(record.Tools)),
		}
		if node.Root {
			node.Parent = ""
		}
		for _, call := range record.Tools {
			node.Tools = append(node.Tools, status.AgentToolEvent{
				Tool: call.Tool, Target: call.Target, Start: call.Start, End: call.End, Failed: call.Failed,
			})
		}
		tree.Nodes = append(tree.Nodes, node)
	}
	return tree
}

func coverageNote(provider string, caps agentCapabilities) string {
	label := launch.RunnerLabel(provider)
	switch {
	case provider == "" || caps.Parents:
		return ""
	case !caps.Children:
		return fmt.Sprintf(agentCoverageRootOnly, label)
	default:
		return fmt.Sprintf(agentCoverageNoTools, label)
	}
}

// agentTrees pushes the shown session's tree, at most once per interval and
// never twice the same. A timer flushes it, so the last change before a quiet
// spell still draws.
type agentTrees struct {
	emit     emitter
	interval time.Duration
	shown    func() *sessionState
	build    func(*sessionState) status.AgentTree

	mu      sync.Mutex
	pending map[*sessionState]bool
	last    map[string]status.AgentTree
}

func newAgentTrees(emit emitter, interval time.Duration, shown func() *sessionState, build func(*sessionState) status.AgentTree) *agentTrees {
	return &agentTrees{
		emit: emit, interval: interval, shown: shown, build: build,
		pending: map[*sessionState]bool{}, last: map[string]status.AgentTree{},
	}
}

func (t *agentTrees) schedule(state *sessionState) {
	if state == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pending[state] {
		return
	}
	t.pending[state] = true
	time.AfterFunc(t.interval, func() { t.flush(state) })
}

func (t *agentTrees) flush(state *sessionState) {
	t.mu.Lock()
	delete(t.pending, state)
	t.mu.Unlock()
	if t.shown() != state {
		return
	}
	tree := t.build(state)
	slug := state.slug()
	t.mu.Lock()
	if previous, ok := t.last[slug]; ok && reflect.DeepEqual(previous, tree) {
		t.mu.Unlock()
		return
	}
	t.last[slug] = tree
	t.mu.Unlock()
	t.emit(agentTreeEvent+slug, tree)
}
