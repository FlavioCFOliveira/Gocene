// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.Automata from Apache Lucene 10.4.0.
// Originally derived from dk.brics.automaton, Copyright (c) 2001-2009
// Anders Moeller (BSD-style licence).

package automaton

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MaxStringUnionTermLength matches Lucene's Automata.MAX_STRING_UNION_TERM_LENGTH.
const MaxStringUnionTermLength = 1000

// MakeEmpty returns a new deterministic automaton with the empty language.
func MakeEmpty() *Automaton {
	a := NewAutomaton()
	a.FinishState()
	return a
}

// MakeEmptyString returns a new deterministic automaton accepting only "".
func MakeEmptyString() *Automaton {
	a := NewAutomaton()
	a.CreateState()
	a.SetAccept(0, true)
	return a
}

// MakeAnyString returns a new deterministic automaton accepting all strings.
func MakeAnyString() *Automaton {
	a := NewAutomaton()
	s := a.CreateState()
	a.SetAccept(s, true)
	a.AddTransition(s, s, MinCodePoint, MaxCodePoint)
	a.FinishState()
	return a
}

// MakeAnyBinary returns a new deterministic automaton accepting all binary terms.
func MakeAnyBinary() *Automaton {
	a := NewAutomaton()
	s := a.CreateState()
	a.SetAccept(s, true)
	a.AddTransition(s, s, 0, 255)
	a.FinishState()
	return a
}

// MakeNonEmptyBinary returns an automaton accepting all binary terms except the empty string.
func MakeNonEmptyBinary() *Automaton {
	a := NewAutomaton()
	s1 := a.CreateState()
	s2 := a.CreateState()
	a.SetAccept(s2, true)
	a.AddTransition(s1, s2, 0, 255)
	a.AddTransition(s2, s2, 0, 255)
	a.FinishState()
	return a
}

// MakeAnyChar returns a new deterministic automaton accepting any single codepoint.
func MakeAnyChar() *Automaton {
	return MakeCharRange(MinCodePoint, MaxCodePoint)
}

// AppendAnyChar appends a transition that accepts any codepoint from state to a new state.
func AppendAnyChar(a *Automaton, state int) int {
	newState := a.CreateState()
	a.AddTransition(state, newState, MinCodePoint, MaxCodePoint)
	return newState
}

// MakeChar returns an automaton accepting the single codepoint c.
func MakeChar(c int) *Automaton {
	return MakeCharRange(c, c)
}

// AppendChar appends a transition from state accepting c to a new state.
func AppendChar(a *Automaton, state, c int) int {
	newState := a.CreateState()
	a.AddTransition(state, newState, c, c)
	return newState
}

// MakeCharRange returns an automaton accepting a single codepoint in [min, max].
func MakeCharRange(min, max int) *Automaton {
	if min > max {
		return MakeEmpty()
	}
	a := NewAutomaton()
	s1 := a.CreateState()
	s2 := a.CreateState()
	a.SetAccept(s2, true)
	a.AddTransition(s1, s2, min, max)
	a.FinishState()
	return a
}

// MakeCharSet returns a minimal automaton accepting any of the provided codepoints.
func MakeCharSet(codepoints []int) *Automaton {
	return MakeCharClass(codepoints, codepoints)
}

// MakeCharClass returns a minimal automaton accepting any codepoint within any of the given ranges.
func MakeCharClass(starts, ends []int) *Automaton {
	if len(starts) != len(ends) {
		panic("automaton: starts must match ends")
	}
	if len(starts) == 0 {
		return MakeEmpty()
	}
	a := NewAutomaton()
	s1 := a.CreateState()
	s2 := a.CreateState()
	a.SetAccept(s2, true)
	for i := range starts {
		a.AddTransition(s1, s2, starts[i], ends[i])
	}
	a.FinishState()
	return a
}

// MakeString returns an automaton accepting exactly the given Unicode string.
func MakeString(s string) *Automaton {
	a := NewAutomaton()
	last := a.CreateState()
	for _, r := range s {
		state := a.CreateState()
		a.AddTransition(last, state, int(r), int(r))
		last = state
	}
	a.SetAccept(last, true)
	a.FinishState()
	return a
}

// MakeBinary returns an automaton accepting the single binary BytesRef term.
func MakeBinary(term *util.BytesRef) *Automaton {
	a := NewAutomaton()
	last := a.CreateState()
	if term != nil {
		for i := 0; i < term.Length; i++ {
			state := a.CreateState()
			label := int(term.Bytes[term.Offset+i]) & 0xFF
			a.AddTransition(last, state, label, label)
			last = state
		}
	}
	a.SetAccept(last, true)
	a.FinishState()
	return a
}

// MakeStringFromCodePoints returns an automaton accepting the single codepoint sequence.
func MakeStringFromCodePoints(word []int, offset, length int) *Automaton {
	a := NewAutomaton()
	a.CreateState()
	s := 0
	for i := offset; i < offset+length; i++ {
		s2 := a.CreateState()
		a.AddTransition(s, s2, word[i], word[i])
		s = s2
	}
	a.SetAccept(s, true)
	a.FinishState()
	return a
}

// MakeDecimalInterval matches Lucene's Automata.makeDecimalInterval. min and max
// are inclusive base-10 integers; digits >0 pins the encoded length, otherwise
// any number of leading zeros is accepted.
func MakeDecimalInterval(min, max, digits int) (*Automaton, error) {
	x := strconv.Itoa(min)
	y := strconv.Itoa(max)
	if min > max || (digits > 0 && len(y) > digits) {
		return nil, fmt.Errorf("automaton: invalid decimal interval [%d,%d] digits=%d", min, max, digits)
	}
	var d int
	if digits > 0 {
		d = digits
	} else {
		d = len(y)
	}
	xb := make([]byte, 0, d)
	for i := len(x); i < d; i++ {
		xb = append(xb, '0')
	}
	xb = append(xb, x...)
	yb := make([]byte, 0, d)
	for i := len(y); i < d; i++ {
		yb = append(yb, '0')
	}
	yb = append(yb, y...)
	xs := string(xb)
	ys := string(yb)

	b := NewBuilder()

	if digits <= 0 {
		// Reserve real initial state at index 0; the between routine starts at the next.
		b.CreateState()
	}

	var initials []int
	between(b, xs, ys, 0, &initials, digits <= 0)

	a1 := b.Finish()

	if digits <= 0 {
		a1.AddTransition(0, 0, '0', '0')
		for _, p := range initials {
			a1.AddEpsilon(0, p)
		}
		a1.FinishState()
	}

	return RemoveDeadStates(a1), nil
}

// anyOfRightLength accepts decimal numbers of length x[n:].
func anyOfRightLength(b *Builder, x string, n int) int {
	s := b.CreateState()
	if len(x) == n {
		b.SetAccept(s, true)
	} else {
		b.AddTransition(s, anyOfRightLength(b, x, n+1), '0', '9')
	}
	return s
}

// atLeast accepts decimal numbers ≥ x[n:] of length len(x)-n.
func atLeast(b *Builder, x string, n int, initials *[]int, zeros bool) int {
	s := b.CreateState()
	if len(x) == n {
		b.SetAccept(s, true)
	} else {
		if zeros {
			*initials = append(*initials, s)
		}
		c := x[n]
		b.AddTransition(s, atLeast(b, x, n+1, initials, zeros && c == '0'), int(c), int(c))
		if c < '9' {
			b.AddTransition(s, anyOfRightLength(b, x, n+1), int(c+1), '9')
		}
	}
	return s
}

// atMost accepts decimal numbers ≤ x[n:] of length len(x)-n.
func atMost(b *Builder, x string, n int) int {
	s := b.CreateState()
	if len(x) == n {
		b.SetAccept(s, true)
	} else {
		c := x[n]
		b.AddTransition(s, atMost(b, x, n+1), int(c), int(c))
		if c > '0' {
			b.AddTransition(s, anyOfRightLength(b, x, n+1), '0', int(c-1))
		}
	}
	return s
}

// between accepts decimal numbers in [x[n:], y[n:]] of length len(x)-n.
func between(b *Builder, x, y string, n int, initials *[]int, zeros bool) int {
	s := b.CreateState()
	if len(x) == n {
		b.SetAccept(s, true)
	} else {
		if zeros {
			*initials = append(*initials, s)
		}
		cx := x[n]
		cy := y[n]
		if cx == cy {
			b.AddTransition(s, between(b, x, y, n+1, initials, zeros && cx == '0'), int(cx), int(cx))
		} else {
			b.AddTransition(s, atLeast(b, x, n+1, initials, zeros && cx == '0'), int(cx), int(cx))
			b.AddTransition(s, atMost(b, y, n+1), int(cy), int(cy))
			if cx+1 < cy {
				b.AddTransition(s, anyOfRightLength(b, x, n+1), int(cx+1), int(cy-1))
			}
		}
	}
	return s
}

// MakeBinaryInterval returns an automaton accepting all binary terms in the
// requested closed/open interval. Either bound may be nil to be open-ended;
// in that case the corresponding inclusive flag must be true.
func MakeBinaryInterval(minTerm *util.BytesRef, minInclusive bool, maxTerm *util.BytesRef, maxInclusive bool) (*Automaton, error) {
	if minTerm == nil && !minInclusive {
		return nil, fmt.Errorf("automaton: minInclusive must be true when min is open-ended")
	}
	if maxTerm == nil && !maxInclusive {
		return nil, fmt.Errorf("automaton: maxInclusive must be true when max is open-ended")
	}
	if minTerm == nil {
		minTerm = &util.BytesRef{}
		minInclusive = true
	}

	var cmp int
	if maxTerm != nil {
		cmp = compareBytesRef(minTerm, maxTerm)
	} else {
		cmp = -1
		if minTerm.Length == 0 {
			if minInclusive {
				return MakeAnyBinary(), nil
			}
			return MakeNonEmptyBinary(), nil
		}
	}

	if cmp == 0 {
		if !minInclusive || !maxInclusive {
			return MakeEmpty(), nil
		}
		return MakeBinary(minTerm), nil
	}
	if cmp > 0 {
		return MakeEmpty(), nil
	}

	if maxTerm != nil && bytesRefStartsWith(maxTerm, minTerm) && suffixIsZeros(maxTerm, minTerm.Length) {
		// Finite-case path.
		maxLength := maxTerm.Length
		if !maxInclusive {
			maxLength--
		}
		if maxLength == minTerm.Length {
			if !minInclusive {
				return MakeEmpty(), nil
			}
			return MakeBinary(minTerm), nil
		}
		a := NewAutomaton()
		lastState := a.CreateState()
		for i := 0; i < minTerm.Length; i++ {
			state := a.CreateState()
			label := int(minTerm.Bytes[minTerm.Offset+i]) & 0xFF
			a.AddTransition(lastState, state, label, label)
			lastState = state
		}
		if minInclusive {
			a.SetAccept(lastState, true)
		}
		for i := minTerm.Length; i < maxLength; i++ {
			state := a.CreateState()
			a.AddTransition(lastState, state, 0, 0)
			a.SetAccept(state, true)
			lastState = state
		}
		a.FinishState()
		return a, nil
	}

	a := NewAutomaton()
	startState := a.CreateState()
	sinkState := a.CreateState()
	a.SetAccept(sinkState, true)
	a.AddTransition(sinkState, sinkState, 0, 255)

	equalPrefix := true
	lastState := startState
	firstMaxState := -1
	sharedPrefixLength := 0
	for i := 0; i < minTerm.Length; i++ {
		minLabel := int(minTerm.Bytes[minTerm.Offset+i]) & 0xFF
		var maxLabel int
		if maxTerm != nil && equalPrefix && i < maxTerm.Length {
			maxLabel = int(maxTerm.Bytes[maxTerm.Offset+i]) & 0xFF
		} else {
			maxLabel = -1
		}

		var nextState int
		if minInclusive && i == minTerm.Length-1 && (!equalPrefix || minLabel != maxLabel) {
			nextState = sinkState
		} else {
			nextState = a.CreateState()
		}

		if equalPrefix {
			if minLabel == maxLabel {
				a.AddTransition(lastState, nextState, minLabel, minLabel)
			} else if maxTerm == nil {
				equalPrefix = false
				sharedPrefixLength = 0
				a.AddTransition(lastState, sinkState, minLabel+1, 0xFF)
				a.AddTransition(lastState, nextState, minLabel, minLabel)
			} else {
				a.AddTransition(lastState, nextState, minLabel, minLabel)
				if maxLabel > minLabel+1 {
					a.AddTransition(lastState, sinkState, minLabel+1, maxLabel-1)
				}
				if maxInclusive || i < maxTerm.Length-1 {
					firstMaxState = a.CreateState()
					if i < maxTerm.Length-1 {
						a.SetAccept(firstMaxState, true)
					}
					a.AddTransition(lastState, firstMaxState, maxLabel, maxLabel)
				}
				equalPrefix = false
				sharedPrefixLength = i
			}
		} else {
			a.AddTransition(lastState, nextState, minLabel, minLabel)
			if minLabel < 255 {
				a.AddTransition(lastState, sinkState, minLabel+1, 255)
			}
		}
		lastState = nextState
	}

	if !equalPrefix && lastState != sinkState && lastState != startState {
		a.AddTransition(lastState, sinkState, 0, 255)
	}

	if minInclusive {
		a.SetAccept(lastState, true)
	}

	if maxTerm != nil {
		if firstMaxState == -1 {
			sharedPrefixLength = minTerm.Length
		} else {
			lastState = firstMaxState
			sharedPrefixLength++
		}
		for i := sharedPrefixLength; i < maxTerm.Length; i++ {
			maxLabel := int(maxTerm.Bytes[maxTerm.Offset+i]) & 0xFF
			if maxLabel > 0 {
				a.AddTransition(lastState, sinkState, 0, maxLabel-1)
			}
			if maxInclusive || i < maxTerm.Length-1 {
				nextState := a.CreateState()
				if i < maxTerm.Length-1 {
					a.SetAccept(nextState, true)
				}
				a.AddTransition(lastState, nextState, maxLabel, maxLabel)
				lastState = nextState
			}
		}
		if maxInclusive {
			a.SetAccept(lastState, true)
		}
	}
	a.FinishState()
	return a, nil
}

// MakeStringUnion accepts the union of UTF-8 BytesRef terms. Terms must be sorted.
func MakeStringUnion(terms []*util.BytesRef) *Automaton {
	if len(terms) == 0 {
		return MakeEmpty()
	}
	// Conservative implementation: build a trie automaton from the sorted terms.
	a := NewAutomaton()
	a.CreateState()
	for _, term := range terms {
		s := 0
		for i := 0; i < term.Length; i++ {
			b := int(term.Bytes[term.Offset+i]) & 0xFF
			ns := -1
			// Look for an existing transition with label b leaving s.
			t := NewTransition()
			count := a.InitTransition(s, t)
			for k := 0; k < count; k++ {
				a.GetNextTransition(t)
				if t.Min <= b && b <= t.Max && t.Min == t.Max {
					ns = t.Dest
					break
				}
			}
			if ns == -1 {
				ns = a.CreateState()
				a.AddTransition(s, ns, b, b)
			}
			s = ns
		}
		a.SetAccept(s, true)
	}
	a.FinishState()
	return a
}

// MakeStringUnionFromStrings is a convenience entry point taking Go strings.
func MakeStringUnionFromStrings(terms []string) *Automaton {
	refs := make([]*util.BytesRef, len(terms))
	for i, s := range terms {
		bs := []byte(s)
		refs[i] = &util.BytesRef{Bytes: bs, Offset: 0, Length: len(bs)}
	}
	return MakeStringUnion(refs)
}

// --- helpers ---

func compareBytesRef(a, b *util.BytesRef) int {
	la := a.Length
	lb := b.Length
	n := la
	if lb < n {
		n = lb
	}
	for i := 0; i < n; i++ {
		ai := a.Bytes[a.Offset+i]
		bi := b.Bytes[b.Offset+i]
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	if la == lb {
		return 0
	}
	if la < lb {
		return -1
	}
	return 1
}

func bytesRefStartsWith(haystack, prefix *util.BytesRef) bool {
	if prefix.Length > haystack.Length {
		return false
	}
	for i := 0; i < prefix.Length; i++ {
		if haystack.Bytes[haystack.Offset+i] != prefix.Bytes[prefix.Offset+i] {
			return false
		}
	}
	return true
}

func suffixIsZeros(br *util.BytesRef, fromLen int) bool {
	for i := fromLen; i < br.Length; i++ {
		if br.Bytes[br.Offset+i] != 0 {
			return false
		}
	}
	return true
}

// AutomataFacade preserves the legacy Automata{} factory style for callers that
// still construct it explicitly; new code should call the package-level Make*
// functions directly.
type AutomataFacade struct{}

// NewAutomata returns an AutomataFacade. Provided for source compatibility with
// the previous package layout.
func NewAutomata() *AutomataFacade { return &AutomataFacade{} }

// MakeEmpty mirrors the package-level MakeEmpty.
func (AutomataFacade) MakeEmpty() *Automaton { return MakeEmpty() }

// MakeEmptyString mirrors the package-level MakeEmptyString.
func (AutomataFacade) MakeEmptyString() *Automaton { return MakeEmptyString() }

// MakeAnyChar mirrors the package-level MakeAnyChar.
func (AutomataFacade) MakeAnyChar() *Automaton { return MakeAnyChar() }

// MakeAnyString mirrors the package-level MakeAnyString.
func (AutomataFacade) MakeAnyString() *Automaton { return MakeAnyString() }

// MakeString mirrors the package-level MakeString.
func (AutomataFacade) MakeString(s string) *Automaton { return MakeString(s) }

// MakeChar mirrors the package-level MakeChar.
func (AutomataFacade) MakeChar(c rune) *Automaton { return MakeChar(int(c)) }

// MakeCharRange mirrors the package-level MakeCharRange.
func (AutomataFacade) MakeCharRange(start, end rune) *Automaton {
	if start > end {
		start, end = end, start
	}
	return MakeCharRange(int(start), int(end))
}

// MakeStringUnion mirrors the package-level MakeStringUnion.
func (AutomataFacade) MakeStringUnion(terms []*util.BytesRef) *Automaton {
	return MakeStringUnion(terms)
}

// MakeStringUnionFromStrings mirrors the package-level MakeStringUnionFromStrings.
func (AutomataFacade) MakeStringUnionFromStrings(terms []string) *Automaton {
	return MakeStringUnionFromStrings(terms)
}
