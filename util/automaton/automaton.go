// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package automaton provides finite-state automata for regular expressions.
// This is a Go port of Lucene's automaton package.
package automaton

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// DefaultDeterminizeWorkLimit is the default work limit for determinization.
	DefaultDeterminizeWorkLimit = 10000

	// Special state values
	invalidState = -1
)

// State represents a state in the automaton.
type State struct {
	id          int
	transitions []*Transition
	accept      bool
}

// Transition represents a transition between states.
type Transition struct {
	to  int
	min int // inclusive
	max int // inclusive
}

// Automaton represents a finite-state automaton.
type Automaton struct {
	states        []*State
	initial       int
	deterministic bool
	minimized     bool
	totality      int // Total number of transitions
}

// NewAutomaton creates a new empty automaton.
func NewAutomaton() *Automaton {
	return &Automaton{
		states:        make([]*State, 0),
		initial:       invalidState,
		deterministic: true,
		minimized:     false,
	}
}

// CreateState creates a new state and returns its ID.
func (a *Automaton) CreateState() int {
	state := &State{
		id:          len(a.states),
		transitions: make([]*Transition, 0),
		accept:      false,
	}
	a.states = append(a.states, state)
	if a.initial == invalidState {
		a.initial = state.id
	}
	return state.id
}

// SetAccept marks a state as accepting.
func (a *Automaton) SetAccept(state int, accept bool) {
	if state >= 0 && state < len(a.states) {
		a.states[state].accept = accept
	}
}

// IsAccept returns true if the state is accepting.
func (a *Automaton) IsAccept(state int) bool {
	if state >= 0 && state < len(a.states) {
		return a.states[state].accept
	}
	return false
}

// GetInitialState returns the initial state.
func (a *Automaton) GetInitialState() int {
	return a.initial
}

// AddTransition adds a transition between states.
func (a *Automaton) AddTransition(from, to, min, max int) {
	if from < 0 || from >= len(a.states) || to < 0 || to >= len(a.states) {
		return
	}
	trans := &Transition{
		to:  to,
		min: min,
		max: max,
	}
	a.states[from].transitions = append(a.states[from].transitions, trans)
	a.totality++
	a.minimized = false
}

// GetTransitions returns all transitions from a state.
func (a *Automaton) GetTransitions(state int) []*Transition {
	if state >= 0 && state < len(a.states) {
		return a.states[state].transitions
	}
	return nil
}

// IsEmpty returns true if the automaton accepts no strings.
func (a *Automaton) IsEmpty() bool {
	return len(a.states) == 0 || a.initial == invalidState
}

// IsEmptyString returns true if the automaton accepts only the empty string.
func (a *Automaton) IsEmptyString() bool {
	if a.IsEmpty() {
		return false
	}
	return a.IsAccept(a.initial) && len(a.GetTransitions(a.initial)) == 0
}

// IsDeterministic returns true if the automaton is deterministic.
func (a *Automaton) IsDeterministic() bool {
	return a.deterministic
}

// NumStates returns the number of states.
func (a *Automaton) NumStates() int {
	return len(a.states)
}

// IsFinite returns true if the automaton is finite (accepts finite language).
func (a *Automaton) IsFinite() bool {
	// Simple check: if there's a cycle that can reach an accept state, it's infinite
	visited := make([]bool, len(a.states))
	recStack := make([]bool, len(a.states))

	var hasCycleToAccept func(int) bool
	hasCycleToAccept = func(state int) bool {
		visited[state] = true
		recStack[state] = true

		for _, trans := range a.states[state].transitions {
			if !visited[trans.to] {
				if hasCycleToAccept(trans.to) {
					return true
				}
			} else if recStack[trans.to] && a.states[trans.to].accept {
				return true
			}
		}

		recStack[state] = false
		return false
	}

	return !hasCycleToAccept(a.initial)
}

// String returns a string representation of the automaton.
func (a *Automaton) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Automaton(%d states, initial=%d):\n", len(a.states), a.initial))
	for i, state := range a.states {
		acceptStr := ""
		if state.accept {
			acceptStr = " [accept]"
		}
		sb.WriteString(fmt.Sprintf("  State %d%s:\n", i, acceptStr))
		for _, trans := range state.transitions {
			label := fmt.Sprintf("%c-%c", trans.min, trans.max)
			if trans.min == trans.max {
				if trans.min >= 0 && trans.min <= 127 {
					label = fmt.Sprintf("%c", trans.min)
				} else {
					label = fmt.Sprintf("\\u%04x", trans.min)
				}
			}
			sb.WriteString(fmt.Sprintf("    -> %d on %s\n", trans.to, label))
		}
	}
	return sb.String()
}

// HashCode returns a hash code for the automaton.
func (a *Automaton) HashCode() int {
	h := len(a.states)
	h = 31*h + a.initial
	for i, state := range a.states {
		h = 31*h + i
		if state.accept {
			h = 31*h + 1
		}
		for _, trans := range state.transitions {
			h = 31*h + trans.to
			h = 31*h + trans.min
			h = 31*h + trans.max
		}
	}
	return h
}

// Equals checks if two automatons are equal.
func (a *Automaton) Equals(other *Automaton) bool {
	if a == other {
		return true
	}
	if a == nil || other == nil {
		return false
	}
	if len(a.states) != len(other.states) {
		return false
	}

	// Check state by state
	for i := range a.states {
		if a.states[i].accept != other.states[i].accept {
			return false
		}
		if len(a.states[i].transitions) != len(other.states[i].transitions) {
			return false
		}
		for j, trans := range a.states[i].transitions {
			otherTrans := other.states[i].transitions[j]
			if trans.to != otherTrans.to || trans.min != otherTrans.min || trans.max != otherTrans.max {
				return false
			}
		}
	}
	return a.initial == other.initial
}

// Clone creates a deep copy of the automaton.
func (a *Automaton) Clone() *Automaton {
	result := NewAutomaton()
	result.initial = a.initial
	result.deterministic = a.deterministic
	result.minimized = a.minimized

	// Create states
	for i := 0; i < len(a.states); i++ {
		result.CreateState()
		result.SetAccept(i, a.states[i].accept)
	}

	// Copy transitions
	for i, state := range a.states {
		for _, trans := range state.transitions {
			result.AddTransition(i, trans.to, trans.min, trans.max)
		}
	}

	return result
}

// Step performs a single transition from state on input.
// Returns the destination state or -1 if no transition.
func (a *Automaton) Step(state int, input int) int {
	if state < 0 || state >= len(a.states) {
		return -1
	}
	for _, trans := range a.states[state].transitions {
		if input >= trans.min && input <= trans.max {
			return trans.to
		}
	}
	return -1
}

// Determinize converts the automaton to a deterministic one.
func (a *Automaton) Determinize(workLimit int) *Automaton {
	if a.deterministic {
		return a
	}

	// Subset construction algorithm
	result := NewAutomaton()

	// Map from set of states to new state ID
	stateSets := make(map[string]int)

	// Initial state set
	initialSet := []int{a.initial}
	initialKey := stateSetKey(initialSet)
	stateSets[initialKey] = result.CreateState()

	// Work list for states to process
	toProcess := [][]int{initialSet}

	workDone := 0
	for len(toProcess) > 0 {
		if workLimit > 0 && workDone >= workLimit {
			// Work limit exceeded, return original
			return a
		}

		currentSet := toProcess[0]
		toProcess = toProcess[1:]

		currentKey := stateSetKey(currentSet)
		currentState := stateSets[currentKey]

		// Check if this is an accept state
		isAccept := false
		for _, s := range currentSet {
			if a.IsAccept(s) {
				isAccept = true
				break
			}
		}
		result.SetAccept(currentState, isAccept)

		// Collect all transitions from states in currentSet
		transMap := make(map[int][]int) // input -> set of destination states
		for _, s := range currentSet {
			for _, trans := range a.GetTransitions(s) {
				for c := trans.min; c <= trans.max; c++ {
					transMap[c] = append(transMap[c], trans.to)
				}
			}
		}

		// Sort inputs for deterministic behavior
		inputs := make([]int, 0, len(transMap))
		for c := range transMap {
			inputs = append(inputs, c)
		}
		sort.Ints(inputs)

		// Create transitions for each input
		for _, c := range inputs {
			destSet := transMap[c]
			// Remove duplicates and sort
			destSet = uniqueInts(destSet)
			sort.Ints(destSet)

			destKey := stateSetKey(destSet)
			destState, exists := stateSets[destKey]
			if !exists {
				destState = result.CreateState()
				stateSets[destKey] = destState
				toProcess = append(toProcess, destSet)
			}

			result.AddTransition(currentState, destState, c, c)
			workDone++
		}
	}

	result.deterministic = true
	return result
}

// Minimize minimizes the automaton.
func (a *Automaton) Minimize() *Automaton {
	if a.minimized {
		return a
	}

	if !a.deterministic {
		a = a.Determinize(DefaultDeterminizeWorkLimit)
	}

	// Hopcroft's algorithm for DFA minimization
	// Simplified implementation
	numStates := len(a.states)
	if numStates == 0 {
		return NewAutomaton()
	}

	// Partition states into accept and non-accept
	partitions := make([]int, numStates)
	acceptPartition := 1
	nonAcceptPartition := 0

	for i := 0; i < numStates; i++ {
		if a.IsAccept(i) {
			partitions[i] = acceptPartition
		} else {
			partitions[i] = nonAcceptPartition
		}
	}

	// Refine partitions
	changed := true
	for changed && numStates > 1 {
		changed = false
		newPartitions := make([]int, numStates)
		copy(newPartitions, partitions)

		// Check each state
		for i := 0; i < numStates; i++ {
			for j := i + 1; j < numStates; j++ {
				if partitions[i] == partitions[j] {
					// Check if states are distinguishable
					if a.areDistinguishable(i, j, partitions) {
						// Split partition
						newPartitions[j] = len(newPartitions)
						changed = true
					}
				}
			}
		}
		partitions = newPartitions
	}

	// Build minimized automaton
	result := NewAutomaton()
	stateMap := make(map[int]int) // old partition -> new state

	for i := 0; i < numStates; i++ {
		partition := partitions[i]
		if _, exists := stateMap[partition]; !exists {
			stateMap[partition] = result.CreateState()
			result.SetAccept(stateMap[partition], a.IsAccept(i))
		}
	}

	// Copy transitions (one per partition pair)
	seenTrans := make(map[string]bool)
	for i := 0; i < numStates; i++ {
		fromPartition := partitions[i]
		newFrom := stateMap[fromPartition]

		for _, trans := range a.GetTransitions(i) {
			toPartition := partitions[trans.to]
			newTo := stateMap[toPartition]

			key := fmt.Sprintf("%d:%d:%d:%d", newFrom, newTo, trans.min, trans.max)
			if !seenTrans[key] {
				seenTrans[key] = true
				result.AddTransition(newFrom, newTo, trans.min, trans.max)
			}
		}
	}

	result.initial = stateMap[partitions[a.initial]]
	result.deterministic = true
	result.minimized = true

	return result
}

// areDistinguishable checks if two states are distinguishable.
func (a *Automaton) areDistinguishable(s1, s2 int, partitions []int) bool {
	// Check if transitions lead to different partitions
	trans1 := a.GetTransitions(s1)
	trans2 := a.GetTransitions(s2)

	if len(trans1) != len(trans2) {
		return true
	}

	// Build transition maps
	map1 := make(map[int]int) // input -> partition
	for _, t := range trans1 {
		for c := t.min; c <= t.max; c++ {
			map1[c] = partitions[t.to]
		}
	}

	for _, t := range trans2 {
		for c := t.min; c <= t.max; c++ {
			if p, exists := map1[c]; !exists || p != partitions[t.to] {
				return true
			}
		}
	}

	return false
}

// Run runs the automaton on input starting from state.
// Returns true if the automaton accepts the input.
func (a *Automaton) Run(input []byte, state int) bool {
	current := state
	for _, b := range input {
		current = a.Step(current, int(b))
		if current == -1 {
			return false
		}
	}
	return a.IsAccept(current)
}

// RunString runs the automaton on a string input.
func (a *Automaton) RunString(input string) bool {
	return a.Run([]byte(input), a.initial)
}

// Helper functions

func stateSetKey(states []int) string {
	var sb strings.Builder
	for i, s := range states {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%d", s))
	}
	return sb.String()
}

func uniqueInts(ints []int) []int {
	seen := make(map[int]bool)
	result := make([]int, 0, len(ints))
	for _, i := range ints {
		if !seen[i] {
			seen[i] = true
			result = append(result, i)
		}
	}
	return result
}

// ByteToInt converts a byte to an int code point.
func ByteToInt(b byte) int {
	return int(b)
}

// RuneToInt converts a rune to an int code point.
func RuneToInt(r rune) int {
	return int(r)
}

// IntToRune converts an int code point to a rune.
func IntToRune(i int) rune {
	if i < 0 || i > utf8.MaxRune {
		return utf8.RuneError
	}
	return rune(i)
}
