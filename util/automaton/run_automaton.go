// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.RunAutomaton from Apache Lucene
// 10.4.0 (Apache License 2.0).

package automaton

import "fmt"

// RunAutomaton is a finite-state automaton optimised for fast matching.
// The initial state is always 0. The class is immutable after construction
// and safe for concurrent use.
type RunAutomaton struct {
	automaton    *Automaton
	alphabetSize int
	size         int
	accept       []uint64 // bitset, len = (size+63)/64
	transitions  []int    // delta[state*len(points)+charClass] -> next state (-1 absent)
	points       []int    // interval start points (sorted ascending)
	classmap     []int    // fast lookup for char number < len(classmap)
}

// NewRunAutomaton constructs a RunAutomaton from the deterministic input a.
// alphabetSize gives the open interval [0, alphabetSize) of valid labels.
func NewRunAutomaton(a *Automaton, alphabetSize int) *RunAutomaton {
	if !a.IsDeterministic() {
		panic("automaton: RunAutomaton requires a deterministic Automaton")
	}
	r := &RunAutomaton{
		automaton:    a,
		alphabetSize: alphabetSize,
	}
	r.points = a.GetStartPoints()
	r.size = a.NumStates()
	if r.size < 1 {
		r.size = 1
	}
	r.accept = make([]uint64, (r.size+63)>>6)
	r.transitions = make([]int, r.size*len(r.points))
	for i := range r.transitions {
		r.transitions[i] = -1
	}
	t := NewTransition()
	for n := 0; n < r.size; n++ {
		if a.IsAccept(n) {
			r.accept[n>>6] |= 1 << uint(n&63)
		}
		t.Source = n
		t.transitionUpto = -1
		for c := 0; c < len(r.points); c++ {
			dest := a.Next(t, r.points[c])
			r.transitions[n*len(r.points)+c] = dest
		}
	}
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

// GetSize returns the number of automaton states.
func (r *RunAutomaton) GetSize() int { return r.size }

// IsAccept reports whether state is accepting.
func (r *RunAutomaton) IsAccept(state int) bool {
	if state < 0 || state >= r.size {
		return false
	}
	return r.accept[state>>6]&(1<<uint(state&63)) != 0
}

// GetCharIntervals returns a defensive copy of the interval start points.
func (r *RunAutomaton) GetCharIntervals() []int {
	out := make([]int, len(r.points))
	copy(out, r.points)
	return out
}

// Step returns the destination state after consuming codepoint c from state,
// or -1 if no such transition exists.
func (r *RunAutomaton) Step(state, c int) int {
	if c < len(r.classmap) {
		return r.transitions[state*len(r.points)+r.classmap[c]]
	}
	return r.transitions[state*len(r.points)+r.getCharClass(c)]
}

func (r *RunAutomaton) getCharClass(c int) int {
	// Binary search for the interval containing c.
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

// HashCode returns a small structural hash.
func (r *RunAutomaton) HashCode() int {
	const prime = 31
	result := 1
	result = prime*result + r.alphabetSize
	result = prime*result + len(r.points)
	result = prime*result + r.size
	return result
}

// String returns a human-readable dump.
func (r *RunAutomaton) String() string {
	out := "initial state: 0\n"
	for i := 0; i < r.size; i++ {
		mark := "[reject]"
		if r.IsAccept(i) {
			mark = "[accept]"
		}
		out += fmt.Sprintf("state %d %s:\n", i, mark)
		for j := 0; j < len(r.points); j++ {
			k := r.transitions[i*len(r.points)+j]
			if k != -1 {
				min := r.points[j]
				var max int
				if j+1 < len(r.points) {
					max = r.points[j+1] - 1
				} else {
					max = r.alphabetSize
				}
				out += fmt.Sprintf(" %d-%d -> %d\n", min, max, k)
			}
		}
	}
	return out
}

// GetAutomaton returns the deterministic source Automaton.
func (r *RunAutomaton) GetAutomaton() *Automaton { return r.automaton }

// GetInitialState returns 0 (always the initial state in Lucene's model).
func (r *RunAutomaton) GetInitialState() int { return 0 }

// RunBytes reports whether r accepts the entire byte slice.
func (r *RunAutomaton) RunBytes(input []byte) bool {
	p := 0
	for _, b := range input {
		p = r.Step(p, int(b)&0xFF)
		if p == -1 {
			return false
		}
	}
	return r.IsAccept(p)
}

// RunString reports whether r accepts the Unicode string.
func (r *RunAutomaton) RunString(input string) bool {
	p := 0
	for _, c := range input {
		p = r.Step(p, int(c))
		if p == -1 {
			return false
		}
	}
	return r.IsAccept(p)
}
