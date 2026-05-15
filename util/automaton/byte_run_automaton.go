// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.ByteRunAutomaton from Apache
// Lucene 10.4.0 (Apache License 2.0).

package automaton

import "math"

// ByteRunAutomaton matches automata against UTF-8 byte sequences. When the
// source automaton is not already binary the input is converted via UTF32ToUTF8
// before determinization.
type ByteRunAutomaton struct {
	*RunAutomaton
}

// NewByteRunAutomaton converts a (codepoint) automaton to its byte-level
// equivalent and wraps it in a RunAutomaton tuned for 256-symbol alphabets.
// The input must be deterministic.
func NewByteRunAutomaton(a *Automaton) *ByteRunAutomaton {
	return NewByteRunAutomatonBinary(a, false)
}

// NewByteRunAutomatonBinary skips the UTF-8 conversion when isBinary is true.
// The caller is responsible for ensuring the automaton operates on bytes.
func NewByteRunAutomatonBinary(a *Automaton, isBinary bool) *ByteRunAutomaton {
	var src *Automaton
	if isBinary {
		src = a
	} else {
		if !a.IsDeterministic() {
			panic("automaton: ByteRunAutomaton requires a deterministic Automaton")
		}
		converted := NewUTF32ToUTF8().Convert(a)
		// The conversion may emit a non-deterministic automaton, hence the determinize.
		det, err := Determinize(converted, math.MaxInt32)
		if err != nil {
			panic(err)
		}
		src = det
	}
	return &ByteRunAutomaton{RunAutomaton: NewRunAutomaton(src, 256)}
}

// Step satisfies the ByteRunnable interface (already inherited).
// RunBytes is provided via embedded RunAutomaton.

// GetSize satisfies ByteRunnable.
func (b *ByteRunAutomaton) GetSize() int { return b.RunAutomaton.GetSize() }

// Run accepts a byte slice with offset/length, matching ByteRunnable's signature.
func (b *ByteRunAutomaton) Run(s []byte, offset, length int) bool {
	p := 0
	limit := offset + length
	for i := offset; i < limit; i++ {
		p = b.Step(p, int(s[i])&0xFF)
		if p == -1 {
			return false
		}
	}
	return b.IsAccept(p)
}
