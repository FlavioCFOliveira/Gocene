// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.TransitionAccessor from Apache
// Lucene 10.4.0 (Apache License 2.0).

package automaton

// TransitionAccessor is the Go analogue of Lucene's TransitionAccessor
// interface. It provides random-access and iterator-style access to the
// transitions leaving each state of an automaton-like structure.
type TransitionAccessor interface {
	// InitTransition primes t for iteration over transitions leaving state.
	// Returns the number of transitions.
	InitTransition(state int, t *Transition) int
	// GetNextTransition advances t to the next transition.
	GetNextTransition(t *Transition)
	// GetNumTransitions returns the number of transitions for state.
	GetNumTransitions(state int) int
	// GetTransition fills t with the index-th transition of state.
	GetTransition(state, index int, t *Transition)
}
