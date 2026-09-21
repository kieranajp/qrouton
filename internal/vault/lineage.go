package vault

import "sort"

type LineageState struct {
	EffectiveState string   `json:"effectiveState"`
	Successors     []string `json:"successors"`
	Unresolved     []Edge   `json:"unresolved"`
	Invalid        bool     `json:"invalid"`
}

func ResolveLineage(documents map[string]Document) map[string]LineageState {
	result := make(map[string]LineageState, len(documents))
	for id, d := range documents {
		result[id] = LineageState{EffectiveState: d.State, Successors: []string{}, Unresolved: []Edge{}}
	}
	indices := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	stack := []string{}
	next := 1
	var visit func(string)
	visit = func(id string) {
		indices[id] = next
		low[id] = next
		next++
		stack = append(stack, id)
		onStack[id] = true
		for _, edge := range documents[id].Lineage {
			if _, exists := documents[edge.Target]; !exists {
				continue
			}
			if indices[edge.Target] == 0 {
				visit(edge.Target)
				low[id] = min(low[id], low[edge.Target])
			} else if onStack[edge.Target] {
				low[id] = min(low[id], indices[edge.Target])
			}
		}
		if low[id] != indices[id] {
			return
		}
		component := []string{}
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[last] = false
			component = append(component, last)
			if last == id {
				break
			}
		}
		invalid := len(component) > 1
		if !invalid {
			for _, edge := range documents[id].Lineage {
				invalid = invalid || edge.Target == id
			}
		}
		if invalid {
			for _, member := range component {
				state := result[member]
				state.Invalid = true
				result[member] = state
			}
		}
	}
	for id := range documents {
		if indices[id] == 0 {
			visit(id)
		}
	}
	for id, d := range documents {
		state := result[id]
		if state.Invalid {
			continue
		}
		for _, edge := range d.Lineage {
			target, exists := result[edge.Target]
			if !exists || target.Invalid {
				state.Unresolved = append(state.Unresolved, edge)
				continue
			}
			if edge.Relation == "supersedes" {
				target.Successors = append(target.Successors, id)
				if target.EffectiveState != "abandoned" {
					target.EffectiveState = "superseded"
				}
				result[edge.Target] = target
			}
		}
		current := result[id]
		current.Unresolved = state.Unresolved
		result[id] = current
	}
	for id, state := range result {
		sort.Strings(state.Successors)
		result[id] = state
	}
	return result
}
