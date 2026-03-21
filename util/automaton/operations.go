// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

import (
	"fmt"
	"sort"
)

// TooComplexToDeterminizeException is thrown when determinization fails
type TooComplexToDeterminizeException struct {
	msg string
}

func (e *TooComplexToDeterminizeException) Error() string {
	return e.msg
}

// Operations provides operations on automata.
type Operations struct{}

// NewOperations creates a new Operations instance.
func NewOperations() *Operations {
	return &Operations{}
}

// Determinize determinizes an automaton.
func (o *Operations) Determinize(a *Automaton, workLimit int) (*Automaton, error) {
	result := a.Determinize(workLimit)
	if result == a && !a.IsDeterministic() {
		return nil, &TooComplexToDeterminizeException{
			msg: fmt.Sprintf("Determinization work limit exceeded: %d", workLimit),
		}
	}
	return result, nil
}

// Union returns the union of multiple automata.
func (o *Operations) Union(automata []*Automaton) *Automaton {
	if len(automata) == 0 {
		return NewAutomaton()
	}
	if len(automata) == 1 {
		return automata[0].Clone()
	}

	result := NewAutomaton()

	// Create a new initial state
	newInitial := result.CreateState()

	// Offset for state IDs from each automaton
	offsets := make([]int, len(automata))
	currentOffset := 1 // Start after new initial state

	for i, a := range automata {
		offsets[i] = currentOffset
		currentOffset += a.NumStates()
	}

	// Copy states and transitions from each automaton
	for i, a := range automata {
		offset := offsets[i]

		// Create states
		for s := 0; s < a.NumStates(); s++ {
			result.CreateState()
			result.SetAccept(offset+s, a.IsAccept(s))
		}

		// Copy transitions
		for s := 0; s < a.NumStates(); s++ {
			for _, trans := range a.GetTransitions(s) {
				result.AddTransition(offset+s, offset+trans.to, trans.min, trans.max)
			}
		}

		// Add epsilon transition from new initial to old initial
		// (simulated by copying the initial state's transitions)
		initialTrans := a.GetTransitions(a.GetInitialState())
		for _, trans := range initialTrans {
			result.AddTransition(newInitial, offset+trans.to, trans.min, trans.max)
		}

		// If old initial was accepting, new initial should be too
		if a.IsAccept(a.GetInitialState()) {
			result.SetAccept(newInitial, true)
		}
	}

	result.deterministic = false
	return result
}

// Intersection returns the intersection of two automata.
func (o *Operations) Intersection(a1, a2 *Automaton) *Automaton {
	// Product construction
	result := NewAutomaton()

	// Map from (s1, s2) to new state
	stateMap := make(map[string]int)

	// Initial state is (initial1, initial2)
	initial1 := a1.GetInitialState()
	initial2 := a2.GetInitialState()

	if initial1 == -1 || initial2 == -1 {
		return result
	}

	initialKey := fmt.Sprintf("%d,%d", initial1, initial2)
	stateMap[initialKey] = result.CreateState()

	// Work list
	toProcess := [][2]int{{initial1, initial2}}

	for len(toProcess) > 0 {
		pair := toProcess[0]
		toProcess = toProcess[1:]
		s1, s2 := pair[0], pair[1]

		key := fmt.Sprintf("%d,%d", s1, s2)
		newState := stateMap[key]

		// Set accept if both states are accept
		result.SetAccept(newState, a1.IsAccept(s1) && a2.IsAccept(s2))

		// Get transitions from both states
		trans1 := a1.GetTransitions(s1)
		trans2 := a2.GetTransitions(s2)

		// Build transition maps
		map1 := make(map[int]int) // input -> to
		for _, t := range trans1 {
			for c := t.min; c <= t.max; c++ {
				map1[c] = t.to
			}
		}

		for _, t := range trans2 {
			for c := t.min; c <= t.max; c++ {
				if to1, exists := map1[c]; exists {
					newKey := fmt.Sprintf("%d,%d", to1, t.to)
					newTo, exists := stateMap[newKey]
					if !exists {
						newTo = result.CreateState()
						stateMap[newKey] = newTo
						toProcess = append(toProcess, [2]int{to1, t.to})
					}
					result.AddTransition(newState, newTo, c, c)
				}
			}
		}
	}

	result.deterministic = a1.IsDeterministic() && a2.IsDeterministic()
	return result
}

// Concatenate concatenates multiple automata.
func (o *Operations) Concatenate(automata []*Automaton) *Automaton {
	if len(automata) == 0 {
		return NewAutomaton()
	}
	if len(automata) == 1 {
		return automata[0].Clone()
	}

	result := NewAutomaton()

	// Offset for state IDs from each automaton
	offsets := make([]int, len(automata))
	currentOffset := 0

	for i, a := range automata {
		offsets[i] = currentOffset
		currentOffset += a.NumStates()
	}

	// Copy states and transitions
	acceptStates := make([][]int, len(automata)) // accept states for each automaton

	for i, a := range automata {
		offset := offsets[i]

		// Create states
		for s := 0; s < a.NumStates(); s++ {
			result.CreateState()
			if a.IsAccept(s) {
				acceptStates[i] = append(acceptStates[i], offset+s)
			}
		}

		// Copy transitions
		for s := 0; s < a.NumStates(); s++ {
			for _, trans := range a.GetTransitions(s) {
				result.AddTransition(offset+s, offset+trans.to, trans.min, trans.max)
			}
		}
	}

	// Connect accept states to next automaton's initial state
	for i := 0; i < len(automata)-1; i++ {
		nextOffset := offsets[i+1]

		for _, acceptState := range acceptStates[i] {
			// Add transitions from accept state to next initial
			initialTrans := automata[i+1].GetTransitions(automata[i+1].GetInitialState())
			for _, trans := range initialTrans {
				result.AddTransition(acceptState, nextOffset+trans.to, trans.min, trans.max)
			}

			// If next initial is accepting, this state should also be accepting
			if automata[i+1].IsAccept(automata[i+1].GetInitialState()) {
				result.SetAccept(acceptState, true)
			} else {
				result.SetAccept(acceptState, false)
			}
		}
	}

	result.initial = offsets[0] + automata[0].GetInitialState()

	// Accept states are from the last automaton
	for _, s := range acceptStates[len(automata)-1] {
		result.SetAccept(s, true)
	}

	result.deterministic = false
	return result
}

// Minus returns the difference of two automata (a1 - a2).
func (o *Operations) Minus(a1, a2 *Automaton) *Automaton {
	// a1 - a2 = a1 AND NOT(a2)
	notA2 := o.Complement(a2)
	return o.Intersection(a1, notA2)
}

// Complement returns the complement of an automaton.
func (o *Operations) Complement(a *Automaton) *Automaton {
	// Complement: swap accept and non-accept states
	result := a.Clone()
	for i := 0; i < result.NumStates(); i++ {
		result.SetAccept(i, !a.IsAccept(i))
	}
	return result
}

// Optional makes an automaton optional (accepts empty string or original language).
func (o *Operations) Optional(a *Automaton) *Automaton {
	result := a.Clone()
	result.SetAccept(result.GetInitialState(), true)
	return result
}

// Repeat returns the Kleene star of an automaton.
func (o *Operations) Repeat(a *Automaton) *Automaton {
	result := a.Clone()

	// Make initial state accepting (for empty string)
	result.SetAccept(result.GetInitialState(), true)

	// Add transitions from accept states back to initial
	for i := 0; i < result.NumStates(); i++ {
		if result.IsAccept(i) && i != result.GetInitialState() {
			// Copy initial transitions
			initialTrans := result.GetTransitions(result.GetInitialState())
			for _, trans := range initialTrans {
				result.AddTransition(i, trans.to, trans.min, trans.max)
			}
		}
	}

	return result
}

// RepeatMin returns the Kleene plus (one or more repetitions).
func (o *Operations) RepeatMin(a *Automaton, min int) *Automaton {
	if min == 0 {
		return o.Repeat(a)
	}

	// Concatenate min copies
	automata := make([]*Automaton, min)
	for i := 0; i < min; i++ {
		automata[i] = a
	}
	return o.Concatenate(automata)
}

// RepeatMinMax returns an automaton accepting between min and max repetitions.
func (o *Operations) RepeatMinMax(a *Automaton, min, max int) *Automaton {
	if min > max {
		return NewAutomaton()
	}

	// Create min required copies
	base := o.RepeatMin(a, min)

	// Add optional copies up to max
	for i := min; i < max; i++ {
		opt := o.Optional(a)
		base = o.Concatenate([]*Automaton{base, opt})
	}

	return base
}

// IsEmptyLanguage returns true if the automaton accepts no strings.
func (o *Operations) IsEmptyLanguage(a *Automaton) bool {
	return a.IsEmpty()
}

// IsEmptyString returns true if the automaton accepts only the empty string.
func (o *Operations) IsEmptyString(a *Automaton) bool {
	return a.IsEmptyString()
}

// IsTotal returns true if the automaton accepts all strings.
func (o *Operations) IsTotal(a *Automaton) bool {
	// Check if initial state is accepting and has self-loop for all chars
	if !a.IsAccept(a.GetInitialState()) {
		return false
	}

	// Check for self-loop on all possible inputs
	for _, trans := range a.GetTransitions(a.GetInitialState()) {
		if trans.to != a.GetInitialState() {
			return false
		}
	}

	// Check if there are no other transitions
	for i := 0; i < a.NumStates(); i++ {
		if i != a.GetInitialState() && len(a.GetTransitions(i)) > 0 {
			return false
		}
	}

	return true
}

// SameLanguage returns true if two automata accept the same language.
func (o *Operations) SameLanguage(a1, a2 *Automaton) bool {
	// Two automata accept the same language if:
	// (a1 - a2) is empty AND (a2 - a1) is empty
	minus1 := o.Minus(a1, a2)
	minus2 := o.Minus(a2, a1)

	return o.IsEmptyLanguage(minus1) && o.IsEmptyLanguage(minus2)
}

// SubsetOf returns true if the language of a1 is a subset of a2.
func (o *Operations) SubsetOf(a1, a2 *Automaton) bool {
	// a1 is subset of a2 iff (a1 - a2) is empty
	minus := o.Minus(a1, a2)
	return o.IsEmptyLanguage(minus)
}

// HasDeadStates returns true if the automaton has unreachable or dead states.
func (o *Operations) HasDeadStates(a *Automaton) bool {
	// Find reachable states
	reachable := make(map[int]bool)
	toProcess := []int{a.GetInitialState()}

	for len(toProcess) > 0 {
		state := toProcess[0]
		toProcess = toProcess[1:]

		if reachable[state] {
			continue
		}
		reachable[state] = true

		for _, trans := range a.GetTransitions(state) {
			if !reachable[trans.to] {
				toProcess = append(toProcess, trans.to)
			}
		}
	}

	// Check if all states are reachable
	return len(reachable) != a.NumStates()
}

// RemoveDeadStates removes unreachable and dead states.
func (o *Operations) RemoveDeadStates(a *Automaton) *Automaton {
	// Find reachable states
	reachable := make(map[int]bool)
	toProcess := []int{a.GetInitialState()}

	for len(toProcess) > 0 {
		state := toProcess[0]
		toProcess = toProcess[1:]

		if reachable[state] {
			continue
		}
		reachable[state] = true

		for _, trans := range a.GetTransitions(state) {
			if !reachable[trans.to] {
				toProcess = append(toProcess, trans.to)
			}
		}
	}

	// Find states that can reach accept states
	canReachAccept := make(map[int]bool)
	for i := 0; i < a.NumStates(); i++ {
		if a.IsAccept(i) {
			canReachAccept[i] = true
		}
	}

	// Propagate backwards
	changed := true
	for changed {
		changed = false
		for i := 0; i < a.NumStates(); i++ {
			if canReachAccept[i] {
				continue
			}
			for _, trans := range a.GetTransitions(i) {
				if canReachAccept[trans.to] {
					canReachAccept[i] = true
					changed = true
					break
				}
			}
		}
	}

	// Build new automaton with only useful states
	result := NewAutomaton()
	stateMap := make(map[int]int)

	for i := 0; i < a.NumStates(); i++ {
		if reachable[i] && canReachAccept[i] {
			stateMap[i] = result.CreateState()
			result.SetAccept(stateMap[i], a.IsAccept(i))
		}
	}

	// Copy transitions
	for i := 0; i < a.NumStates(); i++ {
		if from, exists := stateMap[i]; exists {
			for _, trans := range a.GetTransitions(i) {
				if to, exists := stateMap[trans.to]; exists {
					result.AddTransition(from, to, trans.min, trans.max)
				}
			}
		}
	}

	if initial, exists := stateMap[a.GetInitialState()]; exists {
		result.initial = initial
	}

	return result
}

// GetLiveStates returns the set of states that can reach an accept state.
func (o *Operations) GetLiveStates(a *Automaton) map[int]bool {
	live := make(map[int]bool)

	// Accept states are live
	for i := 0; i < a.NumStates(); i++ {
		if a.IsAccept(i) {
			live[i] = true
		}
	}

	// Propagate backwards
	changed := true
	for changed {
		changed = false
		for i := 0; i < a.NumStates(); i++ {
			if live[i] {
				continue
			}
			for _, trans := range a.GetTransitions(i) {
				if live[trans.to] {
					live[i] = true
					changed = true
					break
				}
			}
		}
	}

	return live
}

// SortStates sorts the states by some criteria (for determinism).
func (o *Operations) SortStates(a *Automaton) *Automaton {
	result := a.Clone()
	// Sorting transitions for each state
	for i := 0; i < result.NumStates(); i++ {
		trans := result.GetTransitions(i)
		sort.Slice(trans, func(j, k int) bool {
			if trans[j].min != trans[k].min {
				return trans[j].min < trans[k].min
			}
			return trans[j].to < trans[k].to
		})
	}
	return result
}

// TopologicalSort returns a topological sort of states (if acyclic).
func (o *Operations) TopologicalSort(a *Automaton) ([]int, bool) {
	// Kahn's algorithm
	inDegree := make([]int, a.NumStates())

	// Calculate in-degrees
	for i := 0; i < a.NumStates(); i++ {
		for _, trans := range a.GetTransitions(i) {
			inDegree[trans.to]++
		}
	}

	// Queue of nodes with no incoming edges
	queue := make([]int, 0)
	for i := 0; i < a.NumStates(); i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	result := make([]int, 0, a.NumStates())

	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]
		result = append(result, state)

		for _, trans := range a.GetTransitions(state) {
			inDegree[trans.to]--
			if inDegree[trans.to] == 0 {
				queue = append(queue, trans.to)
			}
		}
	}

	// If result doesn't include all states, there's a cycle
	return result, len(result) == a.NumStates()
}

// IsFinite returns true if the automaton accepts a finite language.
func (o *Operations) IsFinite(a *Automaton) bool {
	return a.IsFinite()
}

// GetFiniteStrings returns all strings accepted by a finite automaton.
// Limited to maxCount strings.
func (o *Operations) GetFiniteStrings(a *Automaton, maxCount int) []string {
	if !a.IsFinite() {
		return nil
	}

	// BFS from initial state
	results := make([]string, 0)
	type state struct {
		node int
		path string
	}

	queue := []state{{a.GetInitialState(), ""}}
	visited := make(map[string]bool)

	for len(queue) > 0 && len(results) < maxCount {
		curr := queue[0]
		queue = queue[1:]

		if a.IsAccept(curr.node) {
			results = append(results, curr.path)
		}

		// Visit transitions
		for _, trans := range a.GetTransitions(curr.node) {
			for c := trans.min; c <= trans.max && len(results) < maxCount; c++ {
				newPath := curr.path + string(rune(c))
				key := fmt.Sprintf("%d:%s", trans.to, newPath)
				if !visited[key] {
					visited[key] = true
					queue = append(queue, state{trans.to, newPath})
				}
			}
		}
	}

	return results
}
