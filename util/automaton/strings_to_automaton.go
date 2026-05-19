// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.StringsToAutomaton from Apache
// Lucene 10.4.0 (commit 9983b7c). Implements the incremental construction
// of a minimal acyclic finite-state automaton from a sorted set of strings,
// per Daciuk, Mihov, Watson and Watson (2000):
//
//	https://aclanthology.org/J00-1002.pdf
//
// Original Apache Lucene source:
//
//	lucene/core/src/java/org/apache/lucene/util/automaton/StringsToAutomaton.java

package automaton

import (
	"fmt"
	"unsafe"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// stringsToAutomatonState mirrors Lucene's StringsToAutomaton.State. It is a
// DFSA state with int labels on outgoing transitions, kept in lexicographic
// order so binary search and tail-suffix manipulation match the Java
// implementation byte-for-byte.
type stringsToAutomatonState struct {
	labels   []int
	states   []*stringsToAutomatonState
	is_final bool //nolint:revive // mirrors Lucene field name; field is private to the file.
}

// getState returns the transition target labeled label, or nil if none exists.
// Mirrors State.getState.
func (s *stringsToAutomatonState) getState(label int) *stringsToAutomatonState {
	// labels are sorted ascending; binary search keeps the algorithm O(log n).
	lo, hi := 0, len(s.labels)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		l := s.labels[mid]
		switch {
		case l < label:
			lo = mid + 1
		case l > label:
			hi = mid
		default:
			return s.states[mid]
		}
	}
	return nil
}

// hasChildren reports whether the state has any outgoing transitions.
// Mirrors State.hasChildren.
func (s *stringsToAutomatonState) hasChildren() bool {
	return len(s.labels) > 0
}

// newState appends a fresh outgoing transition labeled label and returns the
// newly created target state. The caller guarantees that label is strictly
// greater than every existing label on this state (Daciuk's invariant), so
// the lexicographic order of labels is preserved by append. Mirrors
// State.newState.
func (s *stringsToAutomatonState) newState(label int) *stringsToAutomatonState {
	// Defensive invariant check; mirrors the assert in the Java port.
	if n := len(s.labels); n > 0 && s.labels[n-1] >= label {
		panic(fmt.Sprintf("automaton: StringsToAutomaton state already has transition labeled %d", label))
	}
	target := &stringsToAutomatonState{}
	s.labels = append(s.labels, label)
	s.states = append(s.states, target)
	return target
}

// lastChild returns the most recently added transition's target state.
// Mirrors State.lastChild().
func (s *stringsToAutomatonState) lastChild() *stringsToAutomatonState {
	return s.states[len(s.states)-1]
}

// lastChildLabel returns the most recent transition's target iff its label
// matches the requested one, otherwise nil. Mirrors State.lastChild(int).
func (s *stringsToAutomatonState) lastChildLabel(label int) *stringsToAutomatonState {
	idx := len(s.labels) - 1
	if idx >= 0 && s.labels[idx] == label {
		return s.states[idx]
	}
	return nil
}

// replaceLastChild replaces the most recently added transition's target with
// the given state. Mirrors State.replaceLastChild.
func (s *stringsToAutomatonState) replaceLastChild(target *stringsToAutomatonState) {
	s.states[len(s.states)-1] = target
}

// stringsToAutomatonRegistry interns states by structural identity using a
// composite hash plus a slow-path equality check on collisions. A plain
// map[stateKey] approach is unsafe because hash collisions would silently
// merge non-equivalent states; the registry instead buckets candidates and
// compares them with state.equals.
type stringsToAutomatonRegistry struct {
	buckets map[uint64][]*stringsToAutomatonState
}

func newStringsToAutomatonRegistry() *stringsToAutomatonRegistry {
	return &stringsToAutomatonRegistry{buckets: make(map[uint64][]*stringsToAutomatonState)}
}

// get returns a previously interned state structurally equivalent to s, or
// nil if none exists.
func (r *stringsToAutomatonRegistry) get(s *stringsToAutomatonState) *stringsToAutomatonState {
	h := s.structuralHash()
	for _, candidate := range r.buckets[h] {
		if candidate.structurallyEquals(s) {
			return candidate
		}
	}
	return nil
}

// put interns s.
func (r *stringsToAutomatonRegistry) put(s *stringsToAutomatonState) {
	h := s.structuralHash()
	r.buckets[h] = append(r.buckets[h], s)
}

// structuralHash mirrors State.hashCode: combines is_final, the label slice,
// and the pointer identity of outgoing states. The use of pointer identity
// is sound because children are interned before their parents.
func (s *stringsToAutomatonState) structuralHash() uint64 {
	var h uint64
	if s.is_final {
		h = 1
	}
	h ^= h*31 + uint64(len(s.labels))
	for _, c := range s.labels {
		h ^= h*31 + uint64(uint32(c))
	}
	for _, t := range s.states {
		// Pointer-identity hash; equivalent to System.identityHashCode in Java.
		h ^= uint64(uintptr(pointerOf(t)))
	}
	return h
}

// structurallyEquals mirrors State.equals: same accept flag, same labels, and
// outgoing transitions pointing to the same state instances (pointer
// identity).
func (s *stringsToAutomatonState) structurallyEquals(other *stringsToAutomatonState) bool {
	if s.is_final != other.is_final {
		return false
	}
	if len(s.labels) != len(other.labels) {
		return false
	}
	for i, l := range s.labels {
		if l != other.labels[i] {
			return false
		}
	}
	for i, t := range s.states {
		if t != other.states[i] {
			return false
		}
	}
	return true
}

// stringsToAutomaton is the porting target of Lucene's StringsToAutomaton.
// Instances are short-lived: build a fresh one per call to BuildStringUnion
// / BuildStringUnionFromIterator.
type stringsToAutomaton struct {
	stateRegistry *stringsToAutomatonRegistry
	root          *stringsToAutomatonState
	previous      *util.BytesRefBuilder
}

func newStringsToAutomaton() *stringsToAutomaton {
	return &stringsToAutomaton{
		stateRegistry: newStringsToAutomatonRegistry(),
		root:          &stringsToAutomatonState{},
	}
}

// setPrevious copies current into the internal scratch buffer. Mirrors
// StringsToAutomaton.setPrevious.
func (b *stringsToAutomaton) setPrevious(current *util.BytesRef) {
	if b.previous == nil {
		b.previous = util.NewBytesRefBuilder()
	}
	b.previous.CopyBytesRef(current)
}

// completeAndConvert performs the final minimization and exports the result
// as a standard Automaton via Builder. Mirrors
// StringsToAutomaton.completeAndConvert.
func (b *stringsToAutomaton) completeAndConvert() *Automaton {
	if b.stateRegistry == nil {
		panic("automaton: StringsToAutomaton already finalized")
	}
	if b.root.hasChildren() {
		b.replaceOrRegister(b.root)
	}
	b.stateRegistry = nil

	out := NewBuilder()
	visited := make(map[*stringsToAutomatonState]int)
	convertStringsToAutomaton(out, b.root, visited)
	return out.Finish()
}

// convertStringsToAutomaton walks the interned DFSA in post-order, assigning
// a fresh Automaton state id to each distinct stringsToAutomatonState. The
// visited map serves the same role as IdentityHashMap in the Java port.
func convertStringsToAutomaton(out *Builder, s *stringsToAutomatonState, visited map[*stringsToAutomatonState]int) int {
	if id, ok := visited[s]; ok {
		return id
	}
	id := out.CreateState()
	out.SetAccept(id, s.is_final)
	visited[s] = id
	for i, target := range s.states {
		out.AddTransitionSingle(id, convertStringsToAutomaton(out, target, visited), s.labels[i])
	}
	return id
}

// add inserts a single term into the in-progress automaton. asBinary toggles
// between per-byte (binary) and per-codepoint (UTF-8) transition labels.
// Mirrors StringsToAutomaton.add.
func (b *stringsToAutomaton) add(current *util.BytesRef, asBinary bool) error {
	if current.Length > MaxStringUnionTermLength {
		return fmt.Errorf(
			"automaton: StringsToAutomaton does not allow terms larger than %d UTF-8 bytes, got %s",
			MaxStringUnionTermLength, current)
	}
	if b.stateRegistry == nil {
		panic("automaton: StringsToAutomaton already finalized")
	}
	if b.previous != nil && util.BytesRefCompare(b.previous.Get(), current) > 0 {
		return fmt.Errorf(
			"automaton: input must be in sorted UTF-8 order: %s >= %s",
			b.previous.Get(), current)
	}
	b.setPrevious(current)

	// Reusable codepoint slot when building per-codepoint automata.
	var codePoint *util.UTF8CodePoint

	bytes := current.Bytes
	pos := current.Offset
	max := current.Offset + current.Length

	state := b.root
	var next *stringsToAutomatonState

	// Descend along the longest existing suffix shared by the previous insert.
	if asBinary {
		for pos < max {
			next = state.lastChildLabel(int(bytes[pos]) & 0xff)
			if next == nil {
				break
			}
			state = next
			pos++
		}
	} else {
		for pos < max {
			codePoint = util.CodePointAt(bytes, pos, codePoint)
			next = state.lastChildLabel(codePoint.CodePoint)
			if next == nil {
				break
			}
			state = next
			pos += codePoint.NumBytes
		}
	}

	// Once the previous term and the new term diverge, freeze the divergent
	// tail of the previous one via replaceOrRegister.
	if state.hasChildren() {
		b.replaceOrRegister(state)
	}

	// Append the suffix that is unique to the new term.
	if asBinary {
		for pos < max {
			state = state.newState(int(bytes[pos]) & 0xff)
			pos++
		}
	} else {
		for pos < max {
			codePoint = util.CodePointAt(bytes, pos, codePoint)
			state = state.newState(codePoint.CodePoint)
			pos += codePoint.NumBytes
		}
	}
	state.is_final = true
	return nil
}

// replaceOrRegister freezes the rightmost path under state, replacing each
// recursively-frozen child with its already-interned equivalent when one
// exists. Mirrors StringsToAutomaton.replaceOrRegister.
func (b *stringsToAutomaton) replaceOrRegister(state *stringsToAutomatonState) {
	child := state.lastChild()
	if child.hasChildren() {
		b.replaceOrRegister(child)
	}
	if registered := b.stateRegistry.get(child); registered != nil {
		state.replaceLastChild(registered)
	} else {
		b.stateRegistry.put(child)
	}
}

// BuildStringUnion builds a minimal, deterministic Automaton accepting the
// union of the supplied UTF-8 BytesRef terms. The input must be in
// binary-sorted order; asBinary toggles between per-byte and per-codepoint
// transition labels. Mirrors StringsToAutomaton.build(Iterable, boolean).
//
// Prefer the higher-level entry points in [Automata] (MakeStringUnion,
// MakeBinaryStringUnion) for end-user code.
func BuildStringUnion(input []*util.BytesRef, asBinary bool) (*Automaton, error) {
	b := newStringsToAutomaton()
	for _, term := range input {
		if err := b.add(term, asBinary); err != nil {
			return nil, err
		}
	}
	return b.completeAndConvert(), nil
}

// BuildStringUnionFromIterator builds a minimal, deterministic Automaton
// accepting the union of the BytesRef terms yielded by it. The iterator must
// yield terms in binary-sorted order. Mirrors
// StringsToAutomaton.build(BytesRefIterator, boolean).
func BuildStringUnionFromIterator(it util.BytesRefIterator, asBinary bool) (*Automaton, error) {
	if it == nil {
		return nil, fmt.Errorf("automaton: BuildStringUnionFromIterator requires a non-nil iterator")
	}
	b := newStringsToAutomaton()
	for {
		term, err := it.Next()
		if err != nil {
			return nil, err
		}
		if term == nil {
			break
		}
		if err := b.add(term, asBinary); err != nil {
			return nil, err
		}
	}
	return b.completeAndConvert(), nil
}

// pointerOf returns the bit pattern of the pointer s as a uintptr. It is
// equivalent to Java's System.identityHashCode for the purposes of building
// a content hash that distinguishes state instances. The result is stable
// for the lifetime of the build because none of the produced states are
// freed until completeAndConvert returns.
func pointerOf(s *stringsToAutomatonState) uintptr {
	return uintptr(unsafe.Pointer(s))
}
