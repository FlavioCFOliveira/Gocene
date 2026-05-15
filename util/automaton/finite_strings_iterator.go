// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.FiniteStringsIterator from Apache
// Lucene 10.4.0 (Apache License 2.0).

package automaton

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FiniteStringsIterator enumerates all strings accepted by a finite automaton.
// If the automaton has cycles the iterator may return ErrAutomatonHasCycles.
type FiniteStringsIterator struct {
	a               *Automaton
	endState        int
	pathStates      []bool
	emitEmptyString bool
	nodes           []finiteStringsNode
	stack           []int
	depth           int
}

// ErrAutomatonHasCycles is returned when the iterator encounters a cycle.
var ErrAutomatonHasCycles = errors.New("automaton: has cycles")

// NewFiniteStringsIterator constructs an iterator over all accepted strings.
func NewFiniteStringsIterator(a *Automaton) *FiniteStringsIterator {
	return NewFiniteStringsIteratorRange(a, 0, -1)
}

// NewFiniteStringsIteratorRange constructs an iterator that walks paths from
// startState to either endState (when >= 0) or to any accept state.
func NewFiniteStringsIteratorRange(a *Automaton, startState, endState int) *FiniteStringsIterator {
	it := &FiniteStringsIterator{
		a:               a,
		endState:        endState,
		pathStates:      make([]bool, a.NumStates()),
		emitEmptyString: a.NumStates() > 0 && a.IsAccept(0),
	}
	if a.NumStates() > startState && a.GetNumTransitions(startState) > 0 {
		it.pathStates[startState] = true
		it.nodes = append(it.nodes, newFiniteStringsNode(a, startState))
		it.stack = append(it.stack, startState)
		it.depth = 1
	}
	return it
}

// Next returns the next accepted string and (nil, nil) when exhausted.
// The returned IntsRef references the iterator's internal buffer and is only
// valid until the next call.
func (it *FiniteStringsIterator) Next() (*util.IntsRef, error) {
	if it.emitEmptyString {
		it.emitEmptyString = false
		empty := []int{}
		return &util.IntsRef{Ints: empty, Offset: 0, Length: 0}, nil
	}
	for it.depth > 0 {
		node := &it.nodes[it.depth-1]
		label := node.nextLabel(it.a)
		if label != -1 {
			it.stack[it.depth-1] = label
			to := node.to
			if it.a.GetNumTransitions(to) != 0 && to != it.endState {
				if it.pathStates[to] {
					return nil, ErrAutomatonHasCycles
				}
				it.pathStates[to] = true
				if it.depth == len(it.nodes) {
					it.nodes = append(it.nodes, finiteStringsNode{})
					it.stack = append(it.stack, 0)
				}
				it.nodes[it.depth] = newFiniteStringsNode(it.a, to)
				it.stack[it.depth] = 0
				it.depth++
			} else if it.endState == to || it.a.IsAccept(to) {
				out := append([]int(nil), it.stack[:it.depth]...)
				return &util.IntsRef{Ints: out, Offset: 0, Length: len(out)}, nil
			}
		} else {
			state := node.state
			it.pathStates[state] = false
			it.depth--
			if it.a.IsAccept(state) {
				out := append([]int(nil), it.stack[:it.depth]...)
				return &util.IntsRef{Ints: out, Offset: 0, Length: len(out)}, nil
			}
		}
	}
	return nil, nil
}

type finiteStringsNode struct {
	state      int
	to         int
	transition int
	label      int
	t          *Transition
}

func newFiniteStringsNode(a *Automaton, state int) finiteStringsNode {
	n := finiteStringsNode{state: state, transition: 0, t: NewTransition()}
	a.GetTransition(state, 0, n.t)
	n.label = n.t.Min
	n.to = n.t.Dest
	return n
}

func (n *finiteStringsNode) nextLabel(a *Automaton) int {
	if n.label > n.t.Max {
		n.transition++
		if n.transition >= a.GetNumTransitions(n.state) {
			n.label = -1
			return -1
		}
		a.GetTransition(n.state, n.transition, n.t)
		n.label = n.t.Min
		n.to = n.t.Dest
	}
	lbl := n.label
	n.label++
	return lbl
}
