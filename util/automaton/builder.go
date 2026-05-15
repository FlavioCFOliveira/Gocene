// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.Automaton.Builder from Apache
// Lucene 10.4.0 (Apache License 2.0, derived from dk.brics.automaton).

package automaton

import "sort"

// Builder records transitions out of order and finalises them into a packed
// Automaton via Finish. Use Builder when the surrounding code cannot supply
// all transitions for one source state contiguously.
type Builder struct {
	nextState   int
	isAccept    []uint64
	transitions []int // packed: src, dest, min, max
}

// NewBuilder allocates an empty Builder.
func NewBuilder() *Builder {
	return NewBuilderWithCapacity(16, 16)
}

// NewBuilderWithCapacity allocates a Builder pre-sized for the given limits.
func NewBuilderWithCapacity(numStates, numTransitions int) *Builder {
	return &Builder{
		transitions: make([]int, 0, numTransitions*4),
	}
}

// CreateState reserves a new state and returns its id (0 for the very first).
func (b *Builder) CreateState() int {
	state := b.nextState
	b.nextState++
	b.ensureAcceptCapacity(state)
	return state
}

// SetAccept marks state as accepting.
func (b *Builder) SetAccept(state int, accept bool) {
	if state < 0 || state >= b.nextState {
		panic("builder: state out of range")
	}
	b.ensureAcceptCapacity(state)
	word := state >> 6
	bit := uint(state & 63)
	if accept {
		b.isAccept[word] |= 1 << bit
	} else {
		b.isAccept[word] &^= 1 << bit
	}
}

// IsAccept reports whether state is accepting.
func (b *Builder) IsAccept(state int) bool {
	word := state >> 6
	bit := uint(state & 63)
	if word >= len(b.isAccept) {
		return false
	}
	return b.isAccept[word]&(1<<bit) != 0
}

// GetNumStates returns the current state count.
func (b *Builder) GetNumStates() int { return b.nextState }

// AddTransition records a transition without imposing ordering constraints.
func (b *Builder) AddTransition(source, dest, min, max int) {
	b.transitions = append(b.transitions, source, dest, min, max)
}

// AddTransitionSingle records a label-only transition.
func (b *Builder) AddTransitionSingle(source, dest, label int) {
	b.AddTransition(source, dest, label, label)
}

// AddEpsilon mirrors Automaton.AddEpsilon, copying transitions from dest onto source.
func (b *Builder) AddEpsilon(source, dest int) {
	// Snapshot len because we append while reading.
	current := len(b.transitions)
	for i := 0; i < current; i += 4 {
		if b.transitions[i] == dest {
			b.AddTransition(source, b.transitions[i+1], b.transitions[i+2], b.transitions[i+3])
		}
	}
	if b.IsAccept(dest) {
		b.SetAccept(source, true)
	}
}

// Copy appends all states and transitions from other onto this builder.
func (b *Builder) Copy(other *Automaton) {
	offset := b.nextState
	otherNumStates := other.NumStates()
	for s := 0; s < otherNumStates; s++ {
		ns := b.CreateState()
		if other.IsAccept(s) {
			b.SetAccept(ns, true)
		}
	}
	t := NewTransition()
	for s := 0; s < otherNumStates; s++ {
		n := other.InitTransition(s, t)
		for i := 0; i < n; i++ {
			other.GetNextTransition(t)
			b.AddTransition(offset+s, offset+t.Dest, t.Min, t.Max)
		}
	}
}

// CopyStates appends only the state slots from other (without transitions).
func (b *Builder) CopyStates(other *Automaton) {
	otherNumStates := other.NumStates()
	for s := 0; s < otherNumStates; s++ {
		ns := b.CreateState()
		if other.IsAccept(s) {
			b.SetAccept(ns, true)
		}
	}
}

// Finish materialises the recorded states/transitions into a packed Automaton.
func (b *Builder) Finish() *Automaton {
	numStates := b.nextState
	numTransitions := len(b.transitions) / 4
	a := NewAutomatonWithCapacity(numStates, numTransitions)
	for s := 0; s < numStates; s++ {
		a.CreateState()
		if b.IsAccept(s) {
			a.SetAccept(s, true)
		}
	}
	if numTransitions == 0 {
		a.FinishState()
		return a
	}
	// Stable-sort transitions by (src asc, min asc, max asc, dest asc).
	idx := make([]int, numTransitions)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool {
		ai := idx[i] * 4
		aj := idx[j] * 4
		if b.transitions[ai] != b.transitions[aj] {
			return b.transitions[ai] < b.transitions[aj]
		}
		if b.transitions[ai+2] != b.transitions[aj+2] {
			return b.transitions[ai+2] < b.transitions[aj+2]
		}
		if b.transitions[ai+3] != b.transitions[aj+3] {
			return b.transitions[ai+3] < b.transitions[aj+3]
		}
		return b.transitions[ai+1] < b.transitions[aj+1]
	})
	for _, k := range idx {
		off := k * 4
		a.AddTransition(b.transitions[off], b.transitions[off+1], b.transitions[off+2], b.transitions[off+3])
	}
	a.FinishState()
	return a
}

func (b *Builder) ensureAcceptCapacity(state int) {
	required := (state >> 6) + 1
	if len(b.isAccept) < required {
		grown := make([]uint64, required)
		copy(grown, b.isAccept)
		b.isAccept = grown
	}
}
