// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

// This file contains compatibility shims for call-sites that predate the
// Lucene-faithful packed-int Automaton. The methods are convenience wrappers
// expressed in terms of the canonical API.

// Clone produces a deep copy of the automaton, including state ids, transitions,
// accept bitmap, and determinism flag.
func (a *Automaton) Clone() *Automaton {
	c := NewAutomatonWithCapacity(a.NumStates()+1, a.NumTransitions()+1)
	c.Copy(a)
	c.deterministic = a.deterministic
	c.FinishState()
	return c
}

// Equals returns true if two automatons share state count, accept bits, and
// transition structure in the same order. Used for value-equality checks by
// callers that need structural identity rather than language equivalence.
func (a *Automaton) Equals(other *Automaton) bool {
	if a == other {
		return true
	}
	if a == nil || other == nil {
		return false
	}
	if a.NumStates() != other.NumStates() || a.NumTransitions() != other.NumTransitions() {
		return false
	}
	t1 := NewTransition()
	t2 := NewTransition()
	for s := 0; s < a.NumStates(); s++ {
		if a.IsAccept(s) != other.IsAccept(s) {
			return false
		}
		n1 := a.InitTransition(s, t1)
		n2 := other.InitTransition(s, t2)
		if n1 != n2 {
			return false
		}
		for i := 0; i < n1; i++ {
			a.GetNextTransition(t1)
			other.GetNextTransition(t2)
			if t1.Dest != t2.Dest || t1.Min != t2.Min || t1.Max != t2.Max {
				return false
			}
		}
	}
	return true
}

// HashCode returns a small structural hash code suitable for cache keys.
func (a *Automaton) HashCode() int {
	const prime = 31
	h := a.NumStates()
	h = prime*h + a.NumTransitions()
	t := NewTransition()
	for s := 0; s < a.NumStates(); s++ {
		if a.IsAccept(s) {
			h = prime*h + 1
		}
		n := a.InitTransition(s, t)
		for i := 0; i < n; i++ {
			a.GetNextTransition(t)
			h = prime*h + t.Dest
			h = prime*h + t.Min
			h = prime*h + t.Max
		}
	}
	return h
}

// IsEmpty reports whether the automaton accepts no strings. Wrapper around
// the package-level IsEmpty so legacy callers can use method syntax.
func (a *Automaton) IsEmpty() bool { return IsEmpty(a) }

// GetInitialState returns 0 when there is at least one state, else -1.
// Legacy callers used GetInitialState() before state 0 became canonical.
func (a *Automaton) GetInitialState() int {
	if a.NumStates() == 0 {
		return -1
	}
	return 0
}

// GetTransitions returns the transitions leaving state as a slice of pointers
// to fresh Transition structs. Legacy convenience accessor; prefer
// InitTransition + GetNextTransition for hot paths.
func (a *Automaton) GetTransitions(state int) []*Transition {
	n := a.GetNumTransitions(state)
	out := make([]*Transition, n)
	for i := 0; i < n; i++ {
		t := NewTransition()
		a.GetTransition(state, i, t)
		out[i] = t
	}
	return out
}

// Run reports whether the automaton accepts the UTF-8 byte slice.
// Legacy convenience wrapper; prefer ByteRunAutomaton for hot paths.
func (a *Automaton) Run(input []byte, state int) bool {
	if state < 0 {
		return false
	}
	cur := state
	for _, b := range input {
		cur = a.Step(cur, int(b)&0xFF)
		if cur == -1 {
			return false
		}
	}
	return a.IsAccept(cur)
}

// RunString reports whether the automaton accepts the Unicode string.
// Legacy convenience wrapper.
func (a *Automaton) RunString(input string) bool {
	return Run(a, input)
}
