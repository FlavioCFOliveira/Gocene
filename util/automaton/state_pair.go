// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.StatePair from Apache Lucene
// 10.4.0 (Apache License 2.0, derived from dk.brics.automaton).

package automaton

import "fmt"

// StatePair represents an unordered pair of state ids drawn from two
// automatons, used as a hash key during product constructions such as
// intersection or sameLanguage checks.
type StatePair struct {
	// S is an optional state id in the product automaton (-1 when unused).
	S int
	// S1 is the first input automaton's state.
	S1 int
	// S2 is the second input automaton's state.
	S2 int
}

// NewStatePair constructs a StatePair with no associated product state.
func NewStatePair(s1, s2 int) StatePair {
	return StatePair{S: -1, S1: s1, S2: s2}
}

// NewStatePairWith constructs a StatePair with an associated product state s.
func NewStatePairWith(s, s1, s2 int) StatePair {
	return StatePair{S: s, S1: s1, S2: s2}
}

// HashCode mirrors Lucene's StatePair.hashCode (s1*31 + s2).
func (p StatePair) HashCode() int { return p.S1*31 + p.S2 }

// Equals reports whether two StatePairs name the same (s1, s2) pair.
func (p StatePair) Equals(other StatePair) bool {
	return p.S1 == other.S1 && p.S2 == other.S2
}

// String renders the pair as "StatePair(s1=.. s2=..)".
func (p StatePair) String() string { return fmt.Sprintf("StatePair(s1=%d s2=%d)", p.S1, p.S2) }
