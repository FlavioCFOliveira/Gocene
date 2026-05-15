// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.NFARunAutomaton from Apache Lucene
// 10.4.0 (Apache License 2.0).

package automaton

import "sort"

// NFARunAutomaton runs an NFA directly, lazily determinizing on demand and
// caching the discovered DFA states. Not safe for concurrent use.
type NFARunAutomaton struct {
	automaton    *Automaton
	points       []int
	dstateToOrd  map[uint64][]int
	dstates      []*nfaDState
	alphabetSize int
	classmap     []int
}

const (
	nfaMissing     = -1
	nfaNotComputed = -2
)

// NewNFARunAutomaton constructs an NFARunAutomaton over the full Unicode alphabet.
func NewNFARunAutomaton(a *Automaton) *NFARunAutomaton {
	return NewNFARunAutomatonAlphabet(a, MaxCodePoint+1)
}

// NewNFARunAutomatonAlphabet constructs an NFARunAutomaton over [0, alphabetSize).
func NewNFARunAutomatonAlphabet(a *Automaton, alphabetSize int) *NFARunAutomaton {
	r := &NFARunAutomaton{
		automaton:    a,
		points:       a.GetStartPoints(),
		alphabetSize: alphabetSize,
		dstateToOrd:  make(map[uint64][]int),
	}
	r.findDState(newNFADState(a, []int{0}))
	limit := 256
	if alphabetSize < limit {
		limit = alphabetSize
	}
	r.classmap = make([]int, limit)
	i := 0
	for j := 0; j < limit; j++ {
		if i+1 < len(r.points) && j == r.points[i+1] {
			i++
		}
		r.classmap[j] = i
	}
	return r
}

// Step returns the next dstate ordinal after consuming code point c.
func (r *NFARunAutomaton) Step(state, c int) int {
	if c < len(r.classmap) {
		return r.dstates[state].nextState(r, r.classmap[c])
	}
	return r.dstates[state].nextState(r, r.getCharClass(c))
}

// IsAccept reports whether dstate is accepting.
func (r *NFARunAutomaton) IsAccept(state int) bool { return r.dstates[state].isAccept }

// GetSize returns the current number of materialised DFA states.
func (r *NFARunAutomaton) GetSize() int { return len(r.dstates) }

// HashCode returns a structural hash matching Lucene's RunAutomaton.hashCode.
func (r *NFARunAutomaton) HashCode() int {
	const prime = 31
	result := 1
	result = prime*result + r.alphabetSize
	result = prime*result + len(r.points)
	result = prime*result + len(r.dstates)
	return result
}

// Run reports whether the byte slice s[offset:offset+length] is accepted.
func (r *NFARunAutomaton) Run(s []byte, offset, length int) bool {
	return RunBytes(r, s, offset, length)
}

// InitTransition primes t for iteration over transitions of dstate.
func (r *NFARunAutomaton) InitTransition(state int, t *Transition) int {
	t.Source = state
	t.transitionUpto = -1
	return r.GetNumTransitions(state)
}

// GetNextTransition advances t to the next transition (skipping MISSING slots).
func (r *NFARunAutomaton) GetNextTransition(t *Transition) {
	ds := r.dstates[t.Source]
	t.transitionUpto++
	for ds.transitions[t.transitionUpto] == nfaMissing {
		t.transitionUpto++
	}
	r.setTransitionAccordingly(t, ds)
}

// GetNumTransitions ensures the dstate is fully determinized and returns its
// outgoing transition count.
func (r *NFARunAutomaton) GetNumTransitions(state int) int {
	r.dstates[state].determinize(r)
	return r.dstates[state].outgoingTransitions
}

// GetTransition fills t with the index-th transition of state.
func (r *NFARunAutomaton) GetTransition(state, index int, t *Transition) {
	r.dstates[state].determinize(r)
	outgoing := -1
	t.transitionUpto = -1
	t.Source = state
	for outgoing < index && t.transitionUpto < len(r.points)-1 {
		t.transitionUpto++
		if r.dstates[t.Source].transitions[t.transitionUpto] != nfaMissing {
			outgoing++
		}
	}
	r.setTransitionAccordingly(t, r.dstates[t.Source])
}

func (r *NFARunAutomaton) setTransitionAccordingly(t *Transition, ds *nfaDState) {
	t.Dest = ds.transitions[t.transitionUpto]
	t.Min = r.points[t.transitionUpto]
	if t.transitionUpto == len(r.points)-1 {
		t.Max = r.alphabetSize - 1
	} else {
		t.Max = r.points[t.transitionUpto+1] - 1
	}
}

func (r *NFARunAutomaton) getCharClass(c int) int {
	lo, hi := 0, len(r.points)
	for hi-lo > 1 {
		mid := (lo + hi) >> 1
		if r.points[mid] > c {
			hi = mid
		} else if r.points[mid] < c {
			lo = mid
		} else {
			return mid
		}
	}
	return lo
}

func (r *NFARunAutomaton) findDState(ds *nfaDState) int {
	if ds == nil {
		return nfaMissing
	}
	h := ds.hashCode
	for _, ord := range r.dstateToOrd[h] {
		if dstateEquals(r.dstates[ord], ds) {
			return ord
		}
	}
	ord := len(r.dstates)
	r.dstates = append(r.dstates, ds)
	r.dstateToOrd[h] = append(r.dstateToOrd[h], ord)
	return ord
}

// --- nfaDState ---

type nfaDState struct {
	nfaStates           []int
	transitions         []int
	hashCode            uint64
	isAccept            bool
	stepTransition      *Transition
	minimalMin          int
	minimalMax          int
	hasMinimal          bool
	computedTransitions int
	outgoingTransitions int
}

func newNFADState(a *Automaton, nfaStates []int) *nfaDState {
	ds := &nfaDState{
		nfaStates:      nfaStates,
		stepTransition: NewTransition(),
	}
	h := uint64(len(nfaStates))
	for _, s := range nfaStates {
		h += mix64(uint64(s))
		if a.IsAccept(s) {
			ds.isAccept = true
		}
	}
	ds.hashCode = h
	return ds
}

func dstateEquals(a, b *nfaDState) bool {
	if a.hashCode != b.hashCode || len(a.nfaStates) != len(b.nfaStates) {
		return false
	}
	for i := range a.nfaStates {
		if a.nfaStates[i] != b.nfaStates[i] {
			return false
		}
	}
	return true
}

func (ds *nfaDState) initTransitions(r *NFARunAutomaton) {
	if ds.transitions == nil {
		ds.transitions = make([]int, len(r.points))
		for i := range ds.transitions {
			ds.transitions[i] = nfaNotComputed
		}
	}
}

func (ds *nfaDState) nextState(r *NFARunAutomaton, charClass int) int {
	ds.initTransitions(r)
	if ds.transitions[charClass] == nfaNotComputed {
		nextDS := ds.step(r, r.points[charClass])
		ord := r.findDState(nextDS)
		ds.assignTransition(charClass, ord)
		// Optionally widen to neighbouring char classes within the same min/max window.
		if ds.hasMinimal {
			cls := charClass
			for cls > 0 && r.points[cls-1] >= ds.minimalMin {
				cls--
				ds.assignTransition(cls, ds.transitions[charClass])
			}
			cls = charClass
			for cls < len(r.points)-1 && r.points[cls+1] <= ds.minimalMax {
				cls++
				ds.assignTransition(cls, ds.transitions[charClass])
			}
			ds.hasMinimal = false
		}
	}
	return ds.transitions[charClass]
}

func (ds *nfaDState) assignTransition(charClass, dest int) {
	if ds.transitions[charClass] == nfaNotComputed {
		ds.computedTransitions++
		ds.transitions[charClass] = dest
		if dest != nfaMissing {
			ds.outgoingTransitions++
		}
	}
}

func (ds *nfaDState) step(r *NFARunAutomaton, c int) *nfaDState {
	dest := make(map[int]struct{}, 4)
	left := -1
	right := r.alphabetSize
	for _, s := range ds.nfaStates {
		n := r.automaton.InitTransition(s, ds.stepTransition)
		for i := 0; i < n; i++ {
			r.automaton.GetNextTransition(ds.stepTransition)
			if ds.stepTransition.Min <= c && ds.stepTransition.Max >= c {
				dest[ds.stepTransition.Dest] = struct{}{}
				if ds.stepTransition.Min > left {
					left = ds.stepTransition.Min
				}
				if ds.stepTransition.Max < right {
					right = ds.stepTransition.Max
				}
			}
			if ds.stepTransition.Max < c {
				if ds.stepTransition.Max+1 > left {
					left = ds.stepTransition.Max + 1
				}
			}
			if ds.stepTransition.Min > c {
				if ds.stepTransition.Min-1 < right {
					right = ds.stepTransition.Min - 1
				}
				break
			}
		}
	}
	if len(dest) == 0 {
		return nil
	}
	out := make([]int, 0, len(dest))
	for k := range dest {
		out = append(out, k)
	}
	sort.Ints(out)
	ds.minimalMin = left
	ds.minimalMax = right
	ds.hasMinimal = true
	return newNFADState(r.automaton, out)
}

func (ds *nfaDState) determinize(r *NFARunAutomaton) {
	if ds.transitions != nil && ds.computedTransitions == len(ds.transitions) {
		return
	}
	ds.initTransitions(r)
	// Use PointTransitionSet-like merge for all outgoing edges.
	type seg struct{ point, charClass int }
	points := newPointTransitionSet()
	for _, s := range ds.nfaStates {
		n := r.automaton.InitTransition(s, ds.stepTransition)
		for i := 0; i < n; i++ {
			r.automaton.GetNextTransition(ds.stepTransition)
			points.add(ds.stepTransition)
		}
	}
	if points.count == 0 {
		for i := range ds.transitions {
			ds.transitions[i] = nfaMissing
		}
		ds.computedTransitions = len(ds.transitions)
		return
	}
	points.sort()
	statesSet := newRefCountSet()
	lastPoint := -1
	charClass := 0
	for i := 0; i < points.count; i++ {
		point := points.points[i].point
		if statesSet.size() > 0 {
			values := append([]int(nil), statesSet.values()...)
			ord := r.findDState(newNFADState(r.automaton, values))
			for charClass < len(r.points) && r.points[charClass] < lastPoint {
				ds.assignTransition(charClass, nfaMissing)
				charClass++
			}
			for charClass < len(r.points) && r.points[charClass] < point {
				ds.assignTransition(charClass, ord)
				charClass++
			}
		}
		// Ends close intervals.
		ends := points.points[i].ends
		for j := 0; j < ends.next; j += 3 {
			statesSet.decr(ends.data[j])
		}
		ends.next = 0
		// Starts open intervals.
		starts := points.points[i].starts
		for j := 0; j < starts.next; j += 3 {
			statesSet.incr(starts.data[j])
		}
		starts.next = 0
		lastPoint = point
	}
	for charClass < len(ds.transitions) {
		ds.assignTransition(charClass, nfaMissing)
		charClass++
	}
	ds.computedTransitions = len(ds.transitions)
}

// mix64 is a fast bit-mixing function used to derive hash buckets.
func mix64(x uint64) uint64 {
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x
}
