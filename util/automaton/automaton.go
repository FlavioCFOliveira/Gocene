// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.Automaton and
// org.apache.lucene.util.automaton.Transition from Apache Lucene 10.5.0
// (Apache License 2.0).

package automaton

import (
	"fmt"
	"math/bits"
	"sort"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MinCodePoint is the minimum Unicode code point, mirroring
// Character.MIN_CODE_POINT.
const MinCodePoint = 0

// MaxCodePoint is the maximum Unicode code point, mirroring
// Character.MAX_CODE_POINT.
const MaxCodePoint = 0x10FFFF

// Transition holds one transition from an Automaton. This is typically used
// temporarily when iterating through transitions by invoking
// Automaton.InitTransition and Automaton.GetNextTransition.
type Transition struct {
	// Source state.
	Source int
	// Dest is the destination state.
	Dest int
	// Min is the minimum accepted label (inclusive).
	Min int
	// Max is the maximum accepted label (inclusive).
	Max int
	// transitionUpto remembers where we are in the iteration; init to -1 to
	// provoke a failure if GetNextTransition is called without a preceding
	// InitTransition.
	transitionUpto int
}

// NewTransition creates a new empty transition.
func NewTransition() *Transition {
	return &Transition{transitionUpto: -1}
}

// Automaton represents an automaton and all its states and transitions. States
// are integers and must be created using CreateState. Mark a state as an accept
// state using SetAccept. Add transitions using AddTransition. Each state must
// have all of its transitions added at once; if this is too restrictive then use
// Builder instead. State 0 is always the initial state. Once a state is
// finished, either because you have started adding transitions to another state
// or you call FinishState, then that state's transitions are sorted (first by
// min, then max, then dest) and reduced (transitions with adjacent labels going
// to the same dest are combined).
type Automaton struct {
	// nextState is where we next write to the states slice; this increments by 2
	// for each added state because we pack a pointer to the transitions slice and
	// a count of how many transitions leave the state.
	nextState int

	// nextTransition is where we next write to in the transitions slice; this
	// increments by 3 for each added transition because we pack min, max, dest in
	// sequence.
	nextTransition int

	// curState is the state we are currently adding transitions to; the caller
	// must add all transitions for this state before moving onto another state.
	curState int

	// states holds, for each state, the index in the transitions slice where this
	// state's leaving transitions are stored (or -1 if this state has not added
	// any transitions yet), followed by the number of transitions.
	states []int

	// isAccept is the accept-state bit set, standing in for java.util.BitSet.
	isAccept []uint64

	// transitions holds toState, min, max for each transition.
	transitions []int

	// deterministic is true if no state has two transitions leaving with the same
	// label.
	deterministic bool
}

// automatonBaseRAMBytes accounts for the object header and the fixed field
// footprint that Java sums as NUM_BYTES_OBJECT_HEADER, three object references,
// three ints and one boolean. In Go that is the shallow size of the struct.
var automatonBaseRAMBytes = util.ShallowSizeOf(Automaton{})

// NewAutomaton creates an automaton with no states.
//
// Mirrors the sole no-argument Java constructor, which delegates to (2, 2).
func NewAutomaton() *Automaton {
	return NewAutomatonWithCapacity(2, 2)
}

// NewAutomatonWithCapacity creates an automaton with enough space for the given
// number of states and transitions.
func NewAutomatonWithCapacity(numStates, numTransitions int) *Automaton {
	return &Automaton{
		curState:      -1,
		states:        make([]int, numStates*2),
		isAccept:      make([]uint64, (numStates+63)>>6),
		transitions:   make([]int, numTransitions*3),
		deterministic: true,
	}
}

// CreateState creates a new state and returns its id. The first state created
// is 0, which is always the initial state.
func (a *Automaton) CreateState() int {
	a.growStates()
	state := a.nextState / 2
	a.states[a.nextState] = -1
	a.nextState += 2
	return state
}

// SetAccept sets or clears this state as an accept state.
func (a *Automaton) SetAccept(state int, accept bool) {
	checkIndex(state, a.NumStates())
	a.setAcceptBit(state, accept)
}

// GetSortedTransitions is sugar to get all transitions for all states. This is
// object-heavy; it is better to iterate state by state instead.
func (a *Automaton) GetSortedTransitions() [][]Transition {
	numStates := a.NumStates()
	transitions := make([][]Transition, numStates)
	for s := 0; s < numStates; s++ {
		numTransitions := a.GetNumTransitions(s)
		transitions[s] = make([]Transition, numTransitions)
		for t := 0; t < numTransitions; t++ {
			transition := NewTransition()
			a.GetTransition(s, t, transition)
			transitions[s][t] = *transition
		}
	}
	return transitions
}

// AcceptCardinality returns the number of accept states, mirroring
// getAcceptStates().cardinality().
func (a *Automaton) AcceptCardinality() int {
	count := 0
	for _, w := range a.isAccept {
		count += bits.OnesCount64(w)
	}
	return count
}

// IsAccept returns true if this state is an accept state.
func (a *Automaton) IsAccept(state int) bool {
	if state < 0 {
		return false
	}
	word := state >> 6
	if word >= len(a.isAccept) {
		return false
	}
	return a.isAccept[word]&(1<<uint(state&63)) != 0
}

// AddTransitionSingle adds a new transition with min = max = label.
//
// Mirrors the Java overload addTransition(int source, int dest, int label).
func (a *Automaton) AddTransitionSingle(source, dest, label int) {
	a.AddTransition(source, dest, label, label)
}

// AddTransition adds a new transition with the specified source, dest, min and
// max.
func (a *Automaton) AddTransition(source, dest, min, max int) {
	bounds := a.nextState / 2
	checkIndex(source, bounds)
	checkIndex(dest, bounds)

	a.growTransitions()
	if a.curState != source {
		if a.curState != -1 {
			a.finishCurrentState()
		}

		// Move to next source:
		a.curState = source
		if a.states[2*a.curState] != -1 {
			panic(fmt.Sprintf("automaton: from state (%d) already had transitions added", source))
		}
		a.states[2*a.curState] = a.nextTransition
	}

	a.transitions[a.nextTransition] = dest
	a.nextTransition++
	a.transitions[a.nextTransition] = min
	a.nextTransition++
	a.transitions[a.nextTransition] = max
	a.nextTransition++

	// Increment transition count for this state
	a.states[2*a.curState+1]++
}

// AddEpsilon adds a [virtual] epsilon transition between source and dest. The
// dest state must already have all transitions added because this method simply
// copies those same transitions over to source.
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

// Copy copies over all states and transitions from other. The state numbers are
// sequentially assigned (appended).
func (a *Automaton) Copy(other *Automaton) {
	// Bulk copy and then fixup the state pointers:
	stateOffset := a.NumStates()
	a.states = growInts(a.states, a.nextState+other.nextState)
	copy(a.states[a.nextState:a.nextState+other.nextState], other.states[:other.nextState])
	for i := 0; i < other.nextState; i += 2 {
		if a.states[a.nextState+i] != -1 {
			a.states[a.nextState+i] += a.nextTransition
		}
	}
	a.nextState += other.nextState
	otherNumStates := other.NumStates()
	for state := 0; state < otherNumStates; state++ {
		if other.IsAccept(state) {
			a.SetAccept(stateOffset+state, true)
		}
	}

	// Bulk copy and then fixup dest for each transition:
	a.transitions = growInts(a.transitions, a.nextTransition+other.nextTransition)
	copy(a.transitions[a.nextTransition:a.nextTransition+other.nextTransition], other.transitions[:other.nextTransition])
	for i := 0; i < other.nextTransition; i += 3 {
		a.transitions[a.nextTransition+i] += stateOffset
	}
	a.nextTransition += other.nextTransition

	if !other.deterministic {
		a.deterministic = false
	}
}

// finishCurrentState freezes the last state, sorting and reducing the
// transitions.
func (a *Automaton) finishCurrentState() {
	numTransitions := a.states[2*a.curState+1]

	offset := a.states[2*a.curState]
	start := offset / 3
	a.sortTransitions(start, numTransitions, destMinMaxLess)

	// Reduce any "adjacent" transitions:
	upto := 0
	minLabel := -1
	maxLabel := -1
	dest := -1

	for i := 0; i < numTransitions; i++ {
		tDest := a.transitions[offset+3*i]
		tMin := a.transitions[offset+3*i+1]
		tMax := a.transitions[offset+3*i+2]

		if dest == tDest {
			if tMin <= maxLabel+1 {
				if tMax > maxLabel {
					maxLabel = tMax
				}
			} else {
				if dest != -1 {
					a.transitions[offset+3*upto] = dest
					a.transitions[offset+3*upto+1] = minLabel
					a.transitions[offset+3*upto+2] = maxLabel
					upto++
				}
				minLabel = tMin
				maxLabel = tMax
			}
		} else {
			if dest != -1 {
				a.transitions[offset+3*upto] = dest
				a.transitions[offset+3*upto+1] = minLabel
				a.transitions[offset+3*upto+2] = maxLabel
				upto++
			}
			dest = tDest
			minLabel = tMin
			maxLabel = tMax
		}
	}

	if dest != -1 {
		// Last transition
		a.transitions[offset+3*upto] = dest
		a.transitions[offset+3*upto+1] = minLabel
		a.transitions[offset+3*upto+2] = maxLabel
		upto++
	}

	a.nextTransition -= (numTransitions - upto) * 3
	a.states[2*a.curState+1] = upto

	// Sort transitions by min/max/dest:
	a.sortTransitions(start, upto, minMaxDestLess)

	if a.deterministic && upto > 1 {
		lastMax := a.transitions[offset+2]
		for i := 1; i < upto; i++ {
			minLabel = a.transitions[offset+3*i+1]
			if minLabel <= lastMax {
				a.deterministic = false
				break
			}
			lastMax = a.transitions[offset+3*i+2]
		}
	}
}

// IsDeterministic returns true if this automaton is deterministic (for every
// state there is only one transition for each label).
func (a *Automaton) IsDeterministic() bool {
	return a.deterministic
}

// FinishState finishes the current state; call this once you are done adding
// transitions for a state. This is automatically called if you start adding
// transitions to a new source state, but for the last state you add you need to
// call this method yourself.
func (a *Automaton) FinishState() {
	if a.curState != -1 {
		a.finishCurrentState()
		a.curState = -1
	}
}

// NumStates returns how many states this automaton has.
func (a *Automaton) NumStates() int {
	return a.nextState / 2
}

// NumTransitions returns how many transitions this automaton has.
func (a *Automaton) NumTransitions() int {
	return a.nextTransition / 3
}

// GetNumTransitions returns the number of transitions leaving state.
func (a *Automaton) GetNumTransitions(state int) int {
	count := a.states[2*state+1]
	if count == -1 {
		return 0
	}
	return count
}

func (a *Automaton) growStates() {
	if a.nextState+2 > len(a.states) {
		a.states = growInts(a.states, a.nextState+2)
	}
}

func (a *Automaton) growTransitions() {
	if a.nextTransition+3 > len(a.transitions) {
		a.transitions = growInts(a.transitions, a.nextTransition+3)
	}
}

// growInts mirrors ArrayUtil.grow(int[], int): grow only when the slice is
// shorter than minSize, over-allocating as Lucene does.
func growInts(array []int, minSize int) []int {
	if len(array) < minSize {
		return util.GrowExact(array, util.Oversize(minSize, util.NumBytesObjectRef))
	}
	return array
}

// destMinMaxLess orders transitions by dest ascending, then min label
// ascending, then max label ascending.
//
// Mirrors Automaton.destMinMaxSorter.
func destMinMaxLess(a *Automaton, i, j int) bool {
	iStart := 3 * i
	jStart := 3 * j

	// First dest:
	if a.transitions[iStart] != a.transitions[jStart] {
		return a.transitions[iStart] < a.transitions[jStart]
	}
	// Then min:
	if a.transitions[iStart+1] != a.transitions[jStart+1] {
		return a.transitions[iStart+1] < a.transitions[jStart+1]
	}
	// Then max:
	return a.transitions[iStart+2] < a.transitions[jStart+2]
}

// minMaxDestLess orders transitions by min label ascending, then max label
// ascending, then dest ascending.
//
// Mirrors Automaton.minMaxDestSorter.
func minMaxDestLess(a *Automaton, i, j int) bool {
	iStart := 3 * i
	jStart := 3 * j

	// First min:
	if a.transitions[iStart+1] != a.transitions[jStart+1] {
		return a.transitions[iStart+1] < a.transitions[jStart+1]
	}
	// Then max:
	if a.transitions[iStart+2] != a.transitions[jStart+2] {
		return a.transitions[iStart+2] < a.transitions[jStart+2]
	}
	// Then dest:
	return a.transitions[iStart] < a.transitions[jStart]
}

// transitionSorter adapts a contiguous run of packed transitions to
// sort.Interface. from is the index of the first transition in the run.
type transitionSorter struct {
	a    *Automaton
	from int
	n    int
	less func(a *Automaton, i, j int) bool
}

func (s *transitionSorter) Len() int { return s.n }

func (s *transitionSorter) Less(i, j int) bool {
	return s.less(s.a, s.from+i, s.from+j)
}

func (s *transitionSorter) Swap(i, j int) {
	iStart := 3 * (s.from + i)
	jStart := 3 * (s.from + j)
	t := s.a.transitions
	t[iStart], t[jStart] = t[jStart], t[iStart]
	t[iStart+1], t[jStart+1] = t[jStart+1], t[iStart+1]
	t[iStart+2], t[jStart+2] = t[jStart+2], t[iStart+2]
}

// sortTransitions sorts n transitions starting at transition index start.
// Lucene uses InPlaceMergeSorter, which is stable; sort.Stable is its Go
// counterpart.
func (a *Automaton) sortTransitions(start, n int, less func(*Automaton, int, int) bool) {
	if n < 2 {
		return
	}
	sort.Stable(&transitionSorter{a: a, from: start, n: n, less: less})
}

// InitTransition primes t for iteration over the transitions leaving state and
// returns how many there are.
func (a *Automaton) InitTransition(state int, t *Transition) int {
	t.Source = state
	t.transitionUpto = a.states[2*state]
	return a.GetNumTransitions(state)
}

// GetNextTransition advances t to the next transition leaving t.Source.
func (a *Automaton) GetNextTransition(t *Transition) {
	t.Dest = a.transitions[t.transitionUpto]
	t.transitionUpto++
	t.Min = a.transitions[t.transitionUpto]
	t.transitionUpto++
	t.Max = a.transitions[t.transitionUpto]
	t.transitionUpto++
}

// GetTransition fills t with the index-th transition leaving state.
func (a *Automaton) GetTransition(state, index int, t *Transition) {
	i := a.states[2*state] + 3*index
	t.Source = state
	t.Dest = a.transitions[i]
	i++
	t.Min = a.transitions[i]
	i++
	t.Max = a.transitions[i]
}

// appendCharString mirrors Automaton.appendCharString.
func appendCharString(c int, b *strings.Builder) {
	if c >= 0x21 && c <= 0x7e && c != '\\' && c != '"' {
		b.WriteRune(rune(c))
		return
	}
	b.WriteString("\\\\U")
	s := strconv.FormatInt(int64(c), 16)
	switch {
	case c < 0x10:
		b.WriteString("0000000")
	case c < 0x100:
		b.WriteString("000000")
	case c < 0x1000:
		b.WriteString("00000")
	case c < 0x10000:
		b.WriteString("0000")
	case c < 0x100000:
		b.WriteString("000")
	case c < 0x1000000:
		b.WriteString("00")
	case c < 0x10000000:
		b.WriteString("0")
	}
	b.WriteString(s)
}

// String mirrors the toString() that Automaton inherits from java.lang.Object:
// Lucene's Automaton does not override toString, so call sites such as
// AutomatonQuery.toString() receive an identity string rather than a rendering
// of the automaton. Use ToDot for Lucene's graphviz rendering.
func (a *Automaton) String() string {
	return fmt.Sprintf("Automaton@%p", a)
}

// ToDot returns the dot (graphviz) representation of this automaton. This is
// extremely useful for visualizing the automaton.
func (a *Automaton) ToDot() string {
	var b strings.Builder
	b.WriteString("digraph Automaton {\n")
	b.WriteString("  rankdir = LR\n")
	b.WriteString("  node [width=0.2, height=0.2, fontsize=8]\n")
	numStates := a.NumStates()
	if numStates > 0 {
		b.WriteString("  initial [shape=plaintext,label=\"\"]\n")
		b.WriteString("  initial -> 0\n")
	}

	t := NewTransition()

	for state := 0; state < numStates; state++ {
		b.WriteString("  ")
		b.WriteString(strconv.Itoa(state))
		if a.IsAccept(state) {
			b.WriteString(" [shape=doublecircle,label=\"")
			b.WriteString(strconv.Itoa(state))
			b.WriteString("\"]\n")
		} else {
			b.WriteString(" [shape=circle,label=\"")
			b.WriteString(strconv.Itoa(state))
			b.WriteString("\"]\n")
		}
		numTransitions := a.InitTransition(state, t)
		for i := 0; i < numTransitions; i++ {
			a.GetNextTransition(t)
			b.WriteString("  ")
			b.WriteString(strconv.Itoa(state))
			b.WriteString(" -> ")
			b.WriteString(strconv.Itoa(t.Dest))
			b.WriteString(" [label=\"")
			appendCharString(t.Min, &b)
			if t.Max != t.Min {
				b.WriteByte('-')
				appendCharString(t.Max, &b)
			}
			b.WriteString("\"]\n")
		}
	}
	b.WriteByte('}')
	return b.String()
}

// GetStartPoints returns the sorted slice of all interval start points.
func (a *Automaton) GetStartPoints() []int {
	pointset := make(map[int]struct{})
	pointset[MinCodePoint] = struct{}{}
	for s := 0; s < a.nextState; s += 2 {
		trans := a.states[s]
		limit := trans + 3*a.states[s+1]
		for trans < limit {
			minLabel := a.transitions[trans+1]
			maxLabel := a.transitions[trans+2]
			pointset[minLabel] = struct{}{}
			if maxLabel < MaxCodePoint {
				pointset[maxLabel+1] = struct{}{}
			}
			trans += 3
		}
	}
	points := make([]int, 0, len(pointset))
	for p := range pointset {
		points = append(points, p)
	}
	sort.Ints(points)
	return points
}

// Step performs a lookup in transitions, assuming determinism. It returns the
// destination state, or -1 if there is no matching outgoing transition.
func (a *Automaton) Step(state, label int) int {
	return a.next(state, 0, label, nil)
}

// Next looks for the next transition that matches the provided label, assuming
// determinism.
//
// This method is similar to Step but is used more efficiently when iterating
// over multiple transitions from the same source state. It keeps the latest
// reached transition index in transition.transitionUpto so the next call to this
// method can continue from there instead of restarting from the first
// transition. transition is updated with the matched transition, or with
// Dest = -1 if there is no match.
func (a *Automaton) Next(transition *Transition, label int) int {
	return a.next(transition.Source, transition.transitionUpto, label, transition)
}

// next looks for the next transition that matches the provided label, assuming
// determinism, starting from fromTransitionIndex (inclusive; negative is
// interpreted as 0). transition may be nil, in which case nothing is updated.
func (a *Automaton) next(state, fromTransitionIndex, label int, transition *Transition) int {
	stateIndex := 2 * state
	firstTransitionIndex := a.states[stateIndex]
	numTransitions := a.states[stateIndex+1]

	// Since transitions are sorted,
	// binary search the transition for which label is within [minLabel, maxLabel].
	low := fromTransitionIndex
	if low < 0 {
		low = 0
	}
	high := numTransitions - 1
	for low <= high {
		mid := int(uint(low+high) >> 1)
		transitionIndex := firstTransitionIndex + 3*mid
		minLabel := a.transitions[transitionIndex+1]
		if minLabel > label {
			high = mid - 1
		} else {
			maxLabel := a.transitions[transitionIndex+2]
			if maxLabel < label {
				low = mid + 1
			} else {
				destState := a.transitions[transitionIndex]
				if transition != nil {
					transition.Dest = destState
					transition.Min = minLabel
					transition.Max = maxLabel
					transition.transitionUpto = mid
				}
				return destState
			}
		}
	}
	destState := -1
	if transition != nil {
		transition.Dest = destState
		transition.transitionUpto = low
	}
	return destState
}

// RamBytesUsed returns the approximate memory footprint of this automaton.
//
// Mirrors Automaton.ramBytesUsed(): the fixed struct footprint, the two packed
// int slices, and the accept bit set (its own header plus size()/8 bytes).
func (a *Automaton) RamBytesUsed() int64 {
	return automatonBaseRAMBytes +
		util.SizeOfIntSlice(a.states) +
		util.SizeOfIntSlice(a.transitions) +
		int64(util.NumBytesArrayHeader) +
		int64(len(a.isAccept))*8
}

// setAcceptBit sets or clears the accept bit for state, growing the bit set as
// java.util.BitSet does.
func (a *Automaton) setAcceptBit(state int, accept bool) {
	word := state >> 6
	if !accept {
		if word < len(a.isAccept) {
			a.isAccept[word] &^= 1 << uint(state&63)
		}
		return
	}
	if word >= len(a.isAccept) {
		grown := make([]uint64, word+1)
		copy(grown, a.isAccept)
		a.isAccept = grown
	}
	a.isAccept[word] |= 1 << uint(state&63)
}

// checkIndex mirrors Objects.checkIndex: it fails when index is outside
// [0, length).
func checkIndex(index, length int) {
	if index < 0 || index >= length {
		panic(fmt.Sprintf("automaton: index %d out of bounds for length %d", index, length))
	}
}
