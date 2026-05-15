// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.Automaton from Apache Lucene 10.4.0.
// Original work licensed under the Apache License 2.0 to the Apache Software
// Foundation; portions derived from dk.brics.automaton, Copyright (c) 2001-2009
// Anders Moeller, redistributed under the BSD-style licence reproduced in the
// upstream Lucene source.

package automaton

import (
	"fmt"
	"sort"
	"strings"
)

// MinCodePoint is the smallest Unicode code point (inclusive).
const MinCodePoint = 0

// MaxCodePoint is the largest Unicode code point (inclusive).
const MaxCodePoint = 0x10FFFF

// Transition represents a single labelled transition from an Automaton state.
// Source/Dest/Min/Max are mutable to match Lucene's iterator pattern via
// InitTransition/GetNextTransition; transitionUpto is internal bookkeeping.
type Transition struct {
	// Source is the originating state.
	Source int
	// Dest is the destination state.
	Dest int
	// Min is the minimum accepted code point label (inclusive).
	Min int
	// Max is the maximum accepted code point label (inclusive).
	Max int

	// transitionUpto tracks the current position when iterating via
	// GetNextTransition. Initialised to -1 so misuse fails loudly.
	transitionUpto int
}

// NewTransition returns a zero-valued Transition ready for iteration.
func NewTransition() *Transition {
	return &Transition{transitionUpto: -1}
}

// String renders the transition in the same shape as Lucene's
// "src --> dst min-max" debug representation.
func (t *Transition) String() string {
	return fmt.Sprintf("%d --> %d %c-%c", t.Source, t.Dest, rune(t.Min), rune(t.Max))
}

// Automaton represents a finite-state automaton over Unicode code points
// using Lucene's packed int[] representation. State 0 is always the initial
// state once at least one state has been created. Add all transitions for a
// single source state before moving on; FinishState (or starting another
// source) sorts and reduces the transitions for that state.
type Automaton struct {
	// states packs (transitionsOffset, transitionCount) pairs.
	// states[2*s]   = offset into transitions for state s (or -1 if none yet)
	// states[2*s+1] = number of transitions leaving state s
	states []int

	// nextState is the write cursor for states; increments by 2 per createState.
	nextState int

	// transitions packs (dest, min, max) triples for each transition, sorted
	// per-source via finishCurrentState.
	transitions []int

	// nextTransition is the write cursor for transitions; increments by 3.
	nextTransition int

	// curState is the source state for which AddTransition is currently appending.
	// Moving to a different source triggers finishCurrentState on the previous.
	curState int

	// isAccept is the accept-state bitmap.
	isAccept []uint64

	// deterministic is true when no state has two transitions whose labels overlap.
	deterministic bool
}

// NewAutomaton creates an empty Automaton with default capacity.
func NewAutomaton() *Automaton {
	return NewAutomatonWithCapacity(2, 2)
}

// NewAutomatonWithCapacity creates an Automaton pre-sized for the given number
// of states and transitions to reduce reallocation pressure.
func NewAutomatonWithCapacity(numStates, numTransitions int) *Automaton {
	if numStates < 1 {
		numStates = 1
	}
	if numTransitions < 1 {
		numTransitions = 1
	}
	return &Automaton{
		states:        make([]int, 0, numStates*2),
		transitions:   make([]int, 0, numTransitions*3),
		curState:      -1,
		deterministic: true,
	}
}

// CreateState appends a new state and returns its id. The very first state
// created is the initial state (state 0).
func (a *Automaton) CreateState() int {
	state := a.nextState / 2
	// states[2*state] = -1 (no transitions yet), states[2*state+1] = 0
	a.states = append(a.states, -1, 0)
	a.nextState += 2
	a.ensureIsAcceptCapacity(state)
	return state
}

// NumStates returns the number of states.
func (a *Automaton) NumStates() int {
	return a.nextState / 2
}

// NumTransitions returns the total transition count across all states.
func (a *Automaton) NumTransitions() int {
	return a.nextTransition / 3
}

// SetAccept marks state as accepting (or not).
func (a *Automaton) SetAccept(state int, accept bool) {
	a.checkState(state)
	a.ensureIsAcceptCapacity(state)
	word := state >> 6
	bit := uint(state & 63)
	if accept {
		a.isAccept[word] |= 1 << bit
	} else {
		a.isAccept[word] &^= 1 << bit
	}
}

// IsAccept reports whether state is an accepting state.
func (a *Automaton) IsAccept(state int) bool {
	if state < 0 || state >= a.NumStates() {
		return false
	}
	word := state >> 6
	bit := uint(state & 63)
	if word >= len(a.isAccept) {
		return false
	}
	return a.isAccept[word]&(1<<bit) != 0
}

// NextAcceptState returns the smallest accepting state id >= from, or -1.
// Equivalent to Lucene's BitSet.nextSetBit on the accept-state bitmap.
func (a *Automaton) NextAcceptState(from int) int {
	numStates := a.NumStates()
	if from >= numStates {
		return -1
	}
	if from < 0 {
		from = 0
	}
	for s := from; s < numStates; s++ {
		if a.IsAccept(s) {
			return s
		}
	}
	return -1
}

// AcceptCardinality returns the number of accepting states.
func (a *Automaton) AcceptCardinality() int {
	count := 0
	numStates := a.NumStates()
	for s := 0; s < numStates; s++ {
		if a.IsAccept(s) {
			count++
		}
	}
	return count
}

// AddTransition adds a transition source -> dest for the inclusive label range
// [minLabel, maxLabel]. All transitions for a given source must be appended
// before moving to a different source.
func (a *Automaton) AddTransition(source, dest, minLabel, maxLabel int) {
	a.checkState(source)
	a.checkState(dest)
	if a.curState != source {
		if a.curState != -1 {
			a.finishCurrentState()
		}
		a.curState = source
		if a.states[2*source] != -1 {
			panic(fmt.Sprintf("automaton: source state %d already has transitions finalised", source))
		}
		a.states[2*source] = a.nextTransition
	}
	a.transitions = append(a.transitions, dest, minLabel, maxLabel)
	a.nextTransition += 3
	a.states[2*source+1]++
}

// AddTransitionSingle is a sugar wrapper for label-only transitions.
func (a *Automaton) AddTransitionSingle(source, dest, label int) {
	a.AddTransition(source, dest, label, label)
}

// AddEpsilon adds a virtual epsilon edge by copying dest's outgoing
// transitions onto source, and propagating dest's accept bit.
// dest must already have all its transitions added.
func (a *Automaton) AddEpsilon(source, dest int) {
	t := NewTransition()
	count := a.InitTransition(dest, t)
	for i := 0; i < count; i++ {
		a.GetNextTransition(t)
		a.AddTransition(source, t.Dest, t.Min, t.Max)
	}
	if a.IsAccept(dest) {
		a.SetAccept(source, true)
	}
}

// Copy appends a deep copy of other onto this automaton; the copied states
// are numbered contiguously starting at the current state count.
func (a *Automaton) Copy(other *Automaton) {
	stateOffset := a.NumStates()
	// Allocate state slots and copy accept bits.
	otherNumStates := other.NumStates()
	for s := 0; s < otherNumStates; s++ {
		a.CreateState()
		if other.IsAccept(s) {
			a.SetAccept(stateOffset+s, true)
		}
	}
	// Copy transitions while remapping destinations.
	t := NewTransition()
	for s := 0; s < otherNumStates; s++ {
		n := other.InitTransition(s, t)
		for i := 0; i < n; i++ {
			other.GetNextTransition(t)
			a.AddTransition(stateOffset+s, stateOffset+t.Dest, t.Min, t.Max)
		}
	}
	if !other.deterministic {
		a.deterministic = false
	}
}

// FinishState finishes the current source state, sorting and reducing its
// transitions. Must be called once you've finished adding transitions to the
// last source state.
func (a *Automaton) FinishState() {
	if a.curState != -1 {
		a.finishCurrentState()
		a.curState = -1
	}
}

// IsDeterministic reports whether this automaton is deterministic. Note that
// this flag is only authoritative after FinishState has been called.
func (a *Automaton) IsDeterministic() bool {
	return a.deterministic
}

// GetNumTransitions returns the number of transitions leaving state.
func (a *Automaton) GetNumTransitions(state int) int {
	a.checkState(state)
	count := a.states[2*state+1]
	if count == -1 {
		return 0
	}
	return count
}

// InitTransition primes t for iteration over the transitions leaving state.
// Returns the number of transitions; iterate by calling GetNextTransition
// exactly that many times.
func (a *Automaton) InitTransition(state int, t *Transition) int {
	a.checkState(state)
	t.Source = state
	t.transitionUpto = a.states[2*state]
	return a.GetNumTransitions(state)
}

// GetNextTransition advances t to the next transition for the source state
// previously primed via InitTransition.
func (a *Automaton) GetNextTransition(t *Transition) {
	t.Dest = a.transitions[t.transitionUpto]
	t.Min = a.transitions[t.transitionUpto+1]
	t.Max = a.transitions[t.transitionUpto+2]
	t.transitionUpto += 3
}

// GetTransition loads the index-th transition leaving state into t.
func (a *Automaton) GetTransition(state, index int, t *Transition) {
	a.checkState(state)
	off := a.states[2*state] + 3*index
	t.Source = state
	t.Dest = a.transitions[off]
	t.Min = a.transitions[off+1]
	t.Max = a.transitions[off+2]
	t.transitionUpto = off + 3
}

// Step performs a single step from state on label, assuming determinism.
// Returns the destination state, or -1 if there is no matching transition.
func (a *Automaton) Step(state, label int) int {
	if state < 0 || state >= a.NumStates() {
		return -1
	}
	off := a.states[2*state]
	numTransitions := a.states[2*state+1]
	if off == -1 || numTransitions <= 0 {
		return -1
	}
	low, high := 0, numTransitions-1
	for low <= high {
		mid := (low + high) >> 1
		trIdx := off + 3*mid
		minLabel := a.transitions[trIdx+1]
		if minLabel > label {
			high = mid - 1
			continue
		}
		maxLabel := a.transitions[trIdx+2]
		if maxLabel < label {
			low = mid + 1
			continue
		}
		return a.transitions[trIdx]
	}
	return -1
}

// GetStartPoints returns the sorted set of distinct interval start points
// across all transitions. The result always begins with MinCodePoint and adds
// max+1 for each transition whose max < MaxCodePoint.
func (a *Automaton) GetStartPoints() []int {
	seen := make(map[int]struct{}, 16)
	seen[MinCodePoint] = struct{}{}
	numStates := a.NumStates()
	for s := 0; s < numStates; s++ {
		off := a.states[2*s]
		count := a.states[2*s+1]
		if off == -1 || count <= 0 {
			continue
		}
		for i := 0; i < count; i++ {
			min := a.transitions[off+3*i+1]
			max := a.transitions[off+3*i+2]
			seen[min] = struct{}{}
			if max < MaxCodePoint {
				seen[max+1] = struct{}{}
			}
		}
	}
	points := make([]int, 0, len(seen))
	for p := range seen {
		points = append(points, p)
	}
	sort.Ints(points)
	return points
}

// Next implements deterministic transition lookup matching Lucene's
// Automaton.next. transition.transitionUpto on entry is the per-state
// transition index from which to begin the binary search; on return it is
// updated to the matched transition index (or to the insertion point on miss).
func (a *Automaton) Next(transition *Transition, label int) int {
	state := transition.Source
	off := a.states[2*state]
	numTransitions := a.states[2*state+1]
	low := transition.transitionUpto
	if low < 0 {
		low = 0
	}
	high := numTransitions - 1
	for low <= high {
		mid := (low + high) >> 1
		idx := off + 3*mid
		minLabel := a.transitions[idx+1]
		if minLabel > label {
			high = mid - 1
			continue
		}
		maxLabel := a.transitions[idx+2]
		if maxLabel < label {
			low = mid + 1
			continue
		}
		dest := a.transitions[idx]
		transition.Dest = dest
		transition.Min = minLabel
		transition.Max = maxLabel
		transition.transitionUpto = mid
		return dest
	}
	transition.Dest = -1
	transition.transitionUpto = low
	return -1
}

// String renders the automaton in a stable, human-readable form (states then
// transitions). The exact format is not part of the public contract.
func (a *Automaton) String() string {
	var sb strings.Builder
	numStates := a.NumStates()
	fmt.Fprintf(&sb, "Automaton(states=%d, transitions=%d, deterministic=%v)\n",
		numStates, a.NumTransitions(), a.deterministic)
	t := NewTransition()
	for s := 0; s < numStates; s++ {
		acceptStr := ""
		if a.IsAccept(s) {
			acceptStr = " [accept]"
		}
		fmt.Fprintf(&sb, "  state %d%s\n", s, acceptStr)
		count := a.InitTransition(s, t)
		for i := 0; i < count; i++ {
			a.GetNextTransition(t)
			fmt.Fprintf(&sb, "    --> %d  %d-%d\n", t.Dest, t.Min, t.Max)
		}
	}
	return sb.String()
}

// --- internal helpers ---

func (a *Automaton) checkState(state int) {
	if state < 0 || state >= a.NumStates() {
		panic(fmt.Sprintf("automaton: state %d out of bounds (numStates=%d)", state, a.NumStates()))
	}
}

func (a *Automaton) ensureIsAcceptCapacity(state int) {
	required := (state >> 6) + 1
	if len(a.isAccept) < required {
		grown := make([]uint64, required)
		copy(grown, a.isAccept)
		a.isAccept = grown
	}
}

// finishCurrentState sorts the current source state's transitions and merges
// adjacent transitions with the same dest. It also detects determinism loss.
func (a *Automaton) finishCurrentState() {
	state := a.curState
	off := a.states[2*state]
	num := a.states[2*state+1]
	if num <= 0 {
		return
	}

	// First: sort by (dest, min, max) to merge adjacent transitions with same dest.
	a.sortTransitions(off, num, sortByDestMinMax)

	upto := 0
	min := -1
	max := -1
	dest := -1
	for i := 0; i < num; i++ {
		td := a.transitions[off+3*i]
		tmin := a.transitions[off+3*i+1]
		tmax := a.transitions[off+3*i+2]
		if dest == td {
			if tmin <= max+1 {
				if tmax > max {
					max = tmax
				}
			} else {
				if dest != -1 {
					a.transitions[off+3*upto] = dest
					a.transitions[off+3*upto+1] = min
					a.transitions[off+3*upto+2] = max
					upto++
				}
				min = tmin
				max = tmax
			}
		} else {
			if dest != -1 {
				a.transitions[off+3*upto] = dest
				a.transitions[off+3*upto+1] = min
				a.transitions[off+3*upto+2] = max
				upto++
			}
			dest = td
			min = tmin
			max = tmax
		}
	}
	if dest != -1 {
		a.transitions[off+3*upto] = dest
		a.transitions[off+3*upto+1] = min
		a.transitions[off+3*upto+2] = max
		upto++
	}

	removed := num - upto
	if removed > 0 {
		// Shift the tail of the transitions slice left and shrink length.
		copy(a.transitions[off+3*upto:], a.transitions[off+3*num:])
		a.transitions = a.transitions[:len(a.transitions)-3*removed]
		a.nextTransition -= 3 * removed
		// Fix up offsets of later states (higher than curState) that point past this region.
		for s := 0; s < a.NumStates(); s++ {
			if s == state {
				continue
			}
			o := a.states[2*s]
			if o > off {
				a.states[2*s] = o - 3*removed
			}
		}
	}
	a.states[2*state+1] = upto

	// Then: sort by (min, max, dest) for binary-search-friendly layout.
	a.sortTransitions(off, upto, sortByMinMaxDest)

	if a.deterministic && upto > 1 {
		lastMax := a.transitions[off+2]
		for i := 1; i < upto; i++ {
			mn := a.transitions[off+3*i+1]
			if mn <= lastMax {
				a.deterministic = false
				break
			}
			lastMax = a.transitions[off+3*i+2]
		}
	}
}

// transitionsView returns a sortable view over num transitions starting at off.
type transitionView struct {
	data []int
	off  int
	num  int
	mode int
}

const (
	sortByDestMinMax = 1
	sortByMinMaxDest = 2
)

func (a *Automaton) sortTransitions(off, num, mode int) {
	if num <= 1 {
		return
	}
	tv := &transitionView{data: a.transitions, off: off, num: num, mode: mode}
	sort.Sort(tv)
}

func (v *transitionView) Len() int { return v.num }
func (v *transitionView) Swap(i, j int) {
	iStart := v.off + 3*i
	jStart := v.off + 3*j
	v.data[iStart], v.data[jStart] = v.data[jStart], v.data[iStart]
	v.data[iStart+1], v.data[jStart+1] = v.data[jStart+1], v.data[iStart+1]
	v.data[iStart+2], v.data[jStart+2] = v.data[jStart+2], v.data[iStart+2]
}
func (v *transitionView) Less(i, j int) bool {
	iStart := v.off + 3*i
	jStart := v.off + 3*j
	switch v.mode {
	case sortByDestMinMax:
		// dest ascending, min ascending, max ascending
		iDest := v.data[iStart]
		jDest := v.data[jStart]
		if iDest != jDest {
			return iDest < jDest
		}
		iMin := v.data[iStart+1]
		jMin := v.data[jStart+1]
		if iMin != jMin {
			return iMin < jMin
		}
		return v.data[iStart+2] < v.data[jStart+2]
	default:
		// sortByMinMaxDest: min ascending, max ascending, dest ascending
		iMin := v.data[iStart+1]
		jMin := v.data[jStart+1]
		if iMin != jMin {
			return iMin < jMin
		}
		iMax := v.data[iStart+2]
		jMax := v.data[jStart+2]
		if iMax != jMax {
			return iMax < jMax
		}
		return v.data[iStart] < v.data[jStart]
	}
}
