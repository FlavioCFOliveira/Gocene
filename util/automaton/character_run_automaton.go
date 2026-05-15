// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.CharacterRunAutomaton from
// Apache Lucene 10.4.0 (Apache License 2.0).

package automaton

// CharacterRunAutomaton is a RunAutomaton specialised for matching Unicode
// codepoint sequences (Java char[] / Go strings/runes).
type CharacterRunAutomaton struct {
	*RunAutomaton
}

// NewCharacterRunAutomaton constructs a CharacterRunAutomaton from a
// deterministic Automaton over Unicode code points.
func NewCharacterRunAutomaton(a *Automaton) *CharacterRunAutomaton {
	return &CharacterRunAutomaton{RunAutomaton: NewRunAutomaton(a, MaxCodePoint+1)}
}

// RunString reports whether s (decoded as UTF-8 → codepoints) is accepted.
func (c *CharacterRunAutomaton) RunString(s string) bool {
	p := 0
	for _, r := range s {
		p = c.Step(p, int(r))
		if p == -1 {
			return false
		}
	}
	return c.IsAccept(p)
}

// RunRunes reports whether the codepoint slice in [offset, offset+length) is accepted.
func (c *CharacterRunAutomaton) RunRunes(s []rune, offset, length int) bool {
	p := 0
	for i := offset; i < offset+length; i++ {
		p = c.Step(p, int(s[i]))
		if p == -1 {
			return false
		}
	}
	return c.IsAccept(p)
}
