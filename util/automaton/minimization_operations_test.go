// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.MinimizationOperations from
// Apache Lucene 10.4.0 (releases/lucene/10.4.0, commit 9983b7c). The
// Lucene file lives under lucene/core/src/test/, i.e. it ships as a
// reference implementation used exclusively by tests in the
// util/automaton package (and is no longer part of the production
// Operations.minimize surface since Lucene 10.x). Accordingly the Go
// port is exposed only to in-package tests via the _test.go suffix; the
// algorithm requires write access to Transition.transitionUpto, which
// is unexported, so co-locating it inside the package is required.
//
// The algorithm implemented is Hopcroft's partition-refinement minimizer
// (1971). It assumes the input is already determinized; minimizeForTest
// (the entry point) determinizes first when needed. The internal data
// structures mirror Lucene field-for-field:
//
//   - reverse[q][x]      : back-edges into state q under symbol x.
//   - partition[j]       : current equivalence class j.
//   - splitblock[j]      : the splitter slice being assembled for class j.
//   - block[q]           : the class index that currently owns state q.
//   - active[j][x]       : doubly-linked list of states in class j that
//                          have at least one back-edge under symbol x.
//   - active2[q][x]      : back-pointer to the StateListNode for (q, x)
//                          so that remove() is O(1).
//   - pending / pending2 : worklist of (class, symbol) pairs to refine,
//                          de-duplicated via the bitset pending2.
//   - split / refine /   : per-iteration scratch bitsets that mark the
//     refine2              states to consider and the classes that need
//                          splitting.
//
// Deviations from Lucene 10.4.0:
//
//  1. Java's hppc IntArrayList / IntHashSet / IntCursor / BitSet are
//     replaced by plain []int, map[int]struct{}, range loops and []bool
//     respectively. The empty-list semantics of StateList.EMPTY are kept
//     (a singleton sentinel that is never mutated) so that the read
//     path in the main loop stays branch-free.
//  2. Java's LinkedList<IntPair> pending becomes a Go slice used as a
//     FIFO via append/[0]/[1:]. The hot path only ever pops the head
//     once per cycle so the amortised cost stays O(1).
//  3. Operations.determinize returns (Automaton, error) in Gocene; the
//     port surfaces the error to the test caller rather than panicking,
//     consistent with the rest of util/automaton.
//  4. The recursive Operations.removeDeadStates(result) tail call is
//     preserved verbatim, including the comment about empty/all-strings
//     fastpaths near the top of the function.
//  5. The Lucene "record IntPair" is rendered as a 2-field struct;
//     StateList / StateListNode keep the same intrusive doubly-linked
//     shape, including the EMPTY sentinel guard in add().
//
// This file deliberately exports nothing: the helper is named
// minimizeForTest and is only callable from other _test.go files in the
// util/automaton package, matching the Lucene src/test/ placement.

package automaton

// minimizationStateList is the Go counterpart of Lucene's StateList
// (intrusive doubly-linked list of state ids). emptyMinimizationStateList
// is a singleton sentinel that must never be mutated; minimizeForTest
// uses it instead of nil to avoid a branch on every active[j][x] read.
type minimizationStateList struct {
	size        int
	first, last *minimizationStateListNode
}

// emptyMinimizationStateList is the read-only sentinel used in place of
// nil. add() asserts that the receiver is not this sentinel.
var emptyMinimizationStateList = &minimizationStateList{}

// add appends a new node carrying q and returns it. The receiver must
// not be emptyMinimizationStateList; callers are expected to swap in a
// fresh minimizationStateList first.
func (sl *minimizationStateList) add(q int) *minimizationStateListNode {
	if sl == emptyMinimizationStateList {
		panic("minimization: add on EMPTY StateList")
	}
	return newMinimizationStateListNode(q, sl)
}

// minimizationStateListNode mirrors Lucene's StateListNode: it carries
// the state id, a pointer back to its owning list (so remove() is O(1))
// and the prev/next intrusive links.
type minimizationStateListNode struct {
	q          int
	next, prev *minimizationStateListNode
	sl         *minimizationStateList
}

// newMinimizationStateListNode wires the node into sl as the new tail.
// The "if sl.size++ == 0" check reproduces the Lucene constructor
// exactly: post-increment semantics, head-is-tail on the first insert.
func newMinimizationStateListNode(q int, sl *minimizationStateList) *minimizationStateListNode {
	node := &minimizationStateListNode{q: q, sl: sl}
	if sl.size == 0 {
		sl.first = node
		sl.last = node
	} else {
		sl.last.next = node
		node.prev = sl.last
		sl.last = node
	}
	sl.size++
	return node
}

// remove unlinks this node from its owning list in O(1). The list's
// first/last are updated only when this node was the head or tail
// respectively, matching the Lucene branchless updates.
func (n *minimizationStateListNode) remove() {
	n.sl.size--
	if n.sl.first == n {
		n.sl.first = n.next
	} else {
		n.prev.next = n.next
	}
	if n.sl.last == n {
		n.sl.last = n.prev
	} else {
		n.next.prev = n.prev
	}
}

// minimizationIntPair is the Go counterpart of Lucene's IntPair record.
// n1 is the class index, n2 is the symbol index into sigma.
type minimizationIntPair struct {
	n1, n2 int
}

// minimizeForTest minimises (and determinises if needed) the given
// automaton using Hopcroft's algorithm. It is the Go port of
// MinimizationOperations.minimize from Lucene 10.4.0 and is only
// reachable from in-package _test.go files. The determinizeWorkLimit
// parameter is forwarded to Determinize unchanged; callers that have no
// specific budget should pass DefaultDeterminizeWorkLimit.
//
// The implementation mirrors the Lucene source line-for-line so that
// future cross-checks against upstream patches stay mechanical. Comments
// inside the function reference the corresponding Lucene block when the
// translation is non-obvious.
func minimizeForTest(a *Automaton, determinizeWorkLimit int) (*Automaton, error) {
	// Fastpath for the trivial empty-language automaton. Mirrors
	// Lucene's "Fastmatch for common case" guard at the top of
	// MinimizationOperations.minimize.
	if a.NumStates() == 0 || (!a.IsAccept(0) && a.GetNumTransitions(0) == 0) {
		return NewAutomaton(), nil
	}
	det, err := Determinize(a, determinizeWorkLimit)
	if err != nil {
		return nil, err
	}
	a = det
	// Fastpath for the "accepts all strings" automaton. The Lucene
	// version inspects transition 0 directly; we replicate the same
	// single-transition / full-range check.
	if a.GetNumTransitions(0) == 1 {
		t := NewTransition()
		a.GetTransition(0, 0, t)
		if t.Dest == 0 && t.Min == MinCodePoint && t.Max == MaxCodePoint {
			return a, nil
		}
	}
	a = Totalize(a)

	// initialize data structures
	sigma := a.GetStartPoints()
	sigmaLen, statesLen := len(sigma), a.NumStates()

	reverse := make([][][]int, statesLen)
	partition := make([]map[int]struct{}, statesLen)
	splitblock := make([][]int, statesLen)
	block := make([]int, statesLen)
	active := make([][]*minimizationStateList, statesLen)
	active2 := make([][]*minimizationStateListNode, statesLen)
	pending := make([]minimizationIntPair, 0)
	pending2 := make([]bool, sigmaLen*statesLen)
	split := make([]bool, statesLen)
	refine := make([]bool, statesLen)
	refine2 := make([]bool, statesLen)

	for q := 0; q < statesLen; q++ {
		reverse[q] = make([][]int, sigmaLen)
		active[q] = make([]*minimizationStateList, sigmaLen)
		active2[q] = make([]*minimizationStateListNode, sigmaLen)
		splitblock[q] = nil
		partition[q] = make(map[int]struct{})
		for x := 0; x < sigmaLen; x++ {
			active[q][x] = emptyMinimizationStateList
		}
	}

	// find initial partition and reverse edges
	transition := NewTransition()
	for q := 0; q < statesLen; q++ {
		j := 1
		if a.IsAccept(q) {
			j = 0
		}
		partition[j][q] = struct{}{}
		block[q] = j
		transition.Source = q
		transition.transitionUpto = -1
		for x := 0; x < sigmaLen; x++ {
			dest := a.Next(transition, sigma[x])
			if dest < 0 {
				// Totalize guarantees full coverage of every code
				// point, so Next must succeed for every sigma entry.
				// A negative dest here means the input was not
				// totalised correctly, which is a programmer error.
				panic("minimization: missing transition after Totalize")
			}
			r := reverse[dest]
			r[x] = append(r[x], q)
		}
	}

	// initialize active sets
	for j := 0; j <= 1; j++ {
		for x := 0; x < sigmaLen; x++ {
			for q := range partition[j] {
				if reverse[q][x] != nil {
					stateList := active[j][x]
					if stateList == emptyMinimizationStateList {
						stateList = &minimizationStateList{}
						active[j][x] = stateList
					}
					active2[q][x] = stateList.add(q)
				}
			}
		}
	}

	// initialize pending
	for x := 0; x < sigmaLen; x++ {
		j := 1
		if active[0][x].size <= active[1][x].size {
			j = 0
		}
		pending = append(pending, minimizationIntPair{n1: j, n2: x})
		pending2[x*statesLen+j] = true
	}

	// process pending until fixed point
	k := 2
	for len(pending) > 0 {
		ip := pending[0]
		pending = pending[1:]
		p := ip.n1
		x := ip.n2
		pending2[x*statesLen+p] = false

		// find states that need to be split off their blocks
		for m := active[p][x].first; m != nil; m = m.next {
			r := reverse[m.q][x]
			if r != nil {
				for _, i := range r {
					if !split[i] {
						split[i] = true
						j := block[i]
						splitblock[j] = append(splitblock[j], i)
						if !refine2[j] {
							refine2[j] = true
							refine[j] = true
						}
					}
				}
			}
		}

		// refine blocks
		for j := 0; j < statesLen; j++ {
			if !refine[j] {
				continue
			}
			sb := splitblock[j]
			if len(sb) < len(partition[j]) {
				b1 := partition[j]
				b2 := partition[k]
				for _, s := range sb {
					delete(b1, s)
					b2[s] = struct{}{}
					block[s] = k
					for c := 0; c < sigmaLen; c++ {
						sn := active2[s][c]
						if sn != nil && sn.sl == active[j][c] {
							sn.remove()
							stateList := active[k][c]
							if stateList == emptyMinimizationStateList {
								stateList = &minimizationStateList{}
								active[k][c] = stateList
							}
							active2[s][c] = stateList.add(s)
						}
					}
				}
				// update pending
				for c := 0; c < sigmaLen; c++ {
					aj := active[j][c].size
					ak := active[k][c].size
					ofs := c * statesLen
					if !pending2[ofs+j] && 0 < aj && aj <= ak {
						pending2[ofs+j] = true
						pending = append(pending, minimizationIntPair{n1: j, n2: c})
					} else {
						pending2[ofs+k] = true
						pending = append(pending, minimizationIntPair{n1: k, n2: c})
					}
				}
				k++
			}
			refine2[j] = false
			for _, s := range sb {
				split[s] = false
			}
			splitblock[j] = sb[:0]
		}
		for j := range refine {
			refine[j] = false
		}
	}

	result := NewAutomaton()
	t := NewTransition()

	// make a new state for each equivalence class, set initial state
	stateMap := make([]int, statesLen)
	stateRep := make([]int, k)

	result.CreateState()

	for n := 0; n < k; n++ {
		_, isInitial := partition[n][0]
		var newState int
		if isInitial {
			newState = 0
		} else {
			newState = result.CreateState()
		}
		for q := range partition[n] {
			stateMap[q] = newState
			result.SetAccept(newState, a.IsAccept(q))
			stateRep[newState] = q // select representative
		}
	}

	// build transitions and set acceptance
	for n := 0; n < k; n++ {
		numTransitions := a.InitTransition(stateRep[n], t)
		for i := 0; i < numTransitions; i++ {
			a.GetNextTransition(t)
			result.AddTransition(n, stateMap[t.Dest], t.Min, t.Max)
		}
	}
	result.FinishState()

	return RemoveDeadStates(result), nil
}
