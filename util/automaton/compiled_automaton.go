// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.CompiledAutomaton from Apache
// Lucene 10.4.0 (Apache License 2.0).

package automaton

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// AutomatonType enumerates the simplified forms a compiled automaton may take.
type AutomatonType int

// AutomatonType values.
const (
	AutomatonTypeNone   AutomatonType = iota // accepts nothing
	AutomatonTypeAll                         // accepts everything
	AutomatonTypeSingle                      // accepts exactly one fixed term
	AutomatonTypeNormal                      // catch-all
)

// CompiledAutomaton holds an analysed, ready-to-run automaton along with the
// simplification hints used by term-dictionary intersections.
type CompiledAutomaton struct {
	// Type is the simplified classification (NONE, ALL, SINGLE, NORMAL).
	Type AutomatonType

	// Term holds the singleton term when Type == SINGLE.
	Term *util.BytesRef

	// RunAutomaton is the byte-level run automaton for Type == NORMAL DFAs.
	RunAutomaton *ByteRunAutomaton

	// Automaton is the underlying byte-level Automaton for Type == NORMAL DFAs.
	Automaton *Automaton

	// CommonSuffixRef is the longest common suffix accepted by NORMAL infinite DFAs (optional).
	CommonSuffixRef *util.BytesRef

	// Finite reports whether the source automaton accepts a finite language.
	Finite bool

	// SinkState identifies a sink state when one exists, else -1.
	SinkState int

	// NfaRunAutomaton holds the lazily-determinizing NFA runner for NORMAL NFAs.
	NfaRunAutomaton *NFARunAutomaton

	// Binary tracks whether the source was already a byte-level automaton.
	Binary bool
}

// Compile classifies the automaton, attempting basic simplifications.
// simplify=true mirrors Lucene's default constructor (CompiledAutomaton(a)).
func Compile(a *Automaton) *CompiledAutomaton {
	return CompileFull(a, false, true, false)
}

// CompileFull mirrors Lucene's CompiledAutomaton(Automaton, finite, simplify, isBinary).
func CompileFull(a *Automaton, finite, simplify, isBinary bool) *CompiledAutomaton {
	if a.NumStates() == 0 {
		a = NewAutomaton()
		a.CreateState()
	}

	c := &CompiledAutomaton{
		SinkState: -1,
		Binary:    isBinary,
	}

	// Simplification path requires a DFA.
	if simplify && a.IsDeterministic() {
		if IsEmpty(a) {
			c.Type = AutomatonTypeNone
			c.Finite = true
			return c
		}
		var isTotal bool
		if isBinary {
			isTotal = IsTotalRange(a, 0, 0xFF)
		} else {
			isTotal = IsTotal(a)
		}
		if isTotal {
			c.Type = AutomatonTypeAll
			c.Finite = false
			return c
		}
		if single := GetSingleton(a); single != nil {
			c.Type = AutomatonTypeSingle
			c.Finite = true
			if isBinary {
				bs := make([]byte, len(single))
				for i, cp := range single {
					bs[i] = byte(cp)
				}
				c.Term = &util.BytesRef{Bytes: bs, Offset: 0, Length: len(bs)}
			} else {
				// Encode the single code-point sequence as UTF-8.
				bs := make([]byte, 0, len(single)*4)
				for _, cp := range single {
					bs = appendUTF8(bs, cp)
				}
				c.Term = &util.BytesRef{Bytes: bs, Offset: 0, Length: len(bs)}
			}
			return c
		}
	}

	c.Type = AutomatonTypeNormal
	c.Finite = finite

	var binary *Automaton
	if isBinary {
		binary = a
	} else {
		binary = NewUTF32ToUTF8().Convert(a)
	}

	// We always run on a DFA; if the source automaton might be an NFA, defer to NFARunAutomaton.
	if !a.IsDeterministic() && !binary.IsDeterministic() {
		c.NfaRunAutomaton = NewNFARunAutomatonAlphabet(binary, 0xFF+1)
		return c
	}

	det, err := Determinize(binary, math.MaxInt32)
	if err != nil {
		// Fall back to NFA runner.
		c.NfaRunAutomaton = NewNFARunAutomatonAlphabet(binary, 0xFF+1)
		return c
	}
	c.RunAutomaton = NewByteRunAutomatonBinary(det, true)
	c.Automaton = c.RunAutomaton.GetAutomaton()
	c.SinkState = findSinkState(c.Automaton)
	return c
}

// findSinkState mirrors Lucene's helper for prefix-style sink detection.
func findSinkState(a *Automaton) int {
	t := NewTransition()
	for s := 0; s < a.NumStates(); s++ {
		if !a.IsAccept(s) {
			continue
		}
		count := a.InitTransition(s, t)
		for i := 0; i < count; i++ {
			a.GetNextTransition(t)
			if t.Dest == s && t.Min == 0 && t.Max == 0xFF {
				return s
			}
		}
	}
	return -1
}

// RunString reports whether the compiled automaton accepts the Unicode string s.
// For Type==SINGLE comparisons are done byte-wise against the UTF-8 encoding.
func (c *CompiledAutomaton) RunString(s string) bool {
	return c.Run([]byte(s))
}

// Run reports whether the compiled automaton accepts the byte slice.
func (c *CompiledAutomaton) Run(input []byte) bool {
	switch c.Type {
	case AutomatonTypeNone:
		return false
	case AutomatonTypeAll:
		return true
	case AutomatonTypeSingle:
		return bytesEqual(c.Term, input)
	default:
		if c.RunAutomaton != nil {
			return c.RunAutomaton.Run(input, 0, len(input))
		}
		if c.NfaRunAutomaton != nil {
			return RunBytes(c.NfaRunAutomaton, input, 0, len(input))
		}
		return false
	}
}

func bytesEqual(term *util.BytesRef, input []byte) bool {
	if term == nil {
		return false
	}
	if term.Length != len(input) {
		return false
	}
	for i := 0; i < term.Length; i++ {
		if term.Bytes[term.Offset+i] != input[i] {
			return false
		}
	}
	return true
}

// GetTerm returns the singleton term as a Go string (Unicode for non-binary
// automatons, raw bytes for binary). Empty for non-SINGLE types.
func (c *CompiledAutomaton) GetTerm() string {
	if c.Term == nil {
		return ""
	}
	return string(c.Term.Bytes[c.Term.Offset : c.Term.Offset+c.Term.Length])
}

// TypeName returns the Lucene-style type name ("NONE"/"ALL"/"SINGLE"/"NORMAL").
func (c *CompiledAutomaton) TypeName() string {
	switch c.Type {
	case AutomatonTypeNone:
		return "NONE"
	case AutomatonTypeAll:
		return "ALL"
	case AutomatonTypeSingle:
		return "SINGLE"
	default:
		return "NORMAL"
	}
}

// GetAutomaton returns the underlying byte-level Automaton (may be nil for
// NONE/ALL/SINGLE). For NORMAL DFAs this is the deterministic byte-level form.
func (c *CompiledAutomaton) GetAutomaton() *Automaton { return c.Automaton }

// HashCode returns a small structural hash, suitable for cache keying.
func (c *CompiledAutomaton) HashCode() int {
	const prime = 31
	result := 1
	if c.RunAutomaton != nil {
		result = prime*result + c.RunAutomaton.HashCode()
	}
	if c.NfaRunAutomaton != nil {
		result = prime*result + c.NfaRunAutomaton.HashCode()
	}
	if c.Term != nil {
		result = prime*result + c.Term.HashCode()
	}
	result = prime*result + int(c.Type)
	return result
}

// appendUTF8 appends the UTF-8 encoding of code point cp to dst.
func appendUTF8(dst []byte, cp int) []byte {
	switch {
	case cp < 0x80:
		return append(dst, byte(cp))
	case cp < 0x800:
		return append(dst, byte(0xC0|(cp>>6)), byte(0x80|(cp&0x3F)))
	case cp < 0x10000:
		return append(dst, byte(0xE0|(cp>>12)), byte(0x80|((cp>>6)&0x3F)), byte(0x80|(cp&0x3F)))
	default:
		return append(dst, byte(0xF0|(cp>>18)), byte(0x80|((cp>>12)&0x3F)), byte(0x80|((cp>>6)&0x3F)), byte(0x80|(cp&0x3F)))
	}
}
