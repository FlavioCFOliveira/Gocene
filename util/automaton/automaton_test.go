// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

import (
	"testing"
)

func TestAutomaton_Basic(t *testing.T) {
	a := MakeString("hello")
	if !Run(a, "hello") {
		t.Error("expected 'hello' to match")
	}
	if Run(a, "world") {
		t.Error("expected 'world' not to match")
	}
	if Run(a, "hel") {
		t.Error("expected partial 'hel' not to match")
	}
	if Run(a, "helloo") {
		t.Error("expected over-length 'helloo' not to match")
	}
}

func TestAutomaton_AnyChar(t *testing.T) {
	a := MakeAnyChar()
	if !Run(a, "a") {
		t.Error("expected 'a' to match any char")
	}
	if !Run(a, "z") {
		t.Error("expected 'z' to match any char")
	}
	if Run(a, "ab") {
		t.Error("expected 'ab' not to match any-single-char")
	}
	if Run(a, "") {
		t.Error("expected empty input not to match any-single-char")
	}
}

func TestAutomaton_AnyString(t *testing.T) {
	a := MakeAnyString()
	if !Run(a, "") {
		t.Error("expected '' to match any string")
	}
	if !Run(a, "hello") {
		t.Error("expected 'hello' to match any string")
	}
	if !Run(a, "a very long string") {
		t.Error("expected long input to match any string")
	}
}

func TestAutomaton_MakeString(t *testing.T) {
	a := MakeString("test")
	if !Run(a, "test") {
		t.Error("expected 'test' to match")
	}
	if Run(a, "testing") {
		t.Error("expected 'testing' not to match")
	}
	if Run(a, "tes") {
		t.Error("expected 'tes' not to match")
	}
}

func TestAutomaton_CharRange(t *testing.T) {
	a := MakeCharRange('a', 'z')
	if !Run(a, "a") {
		t.Error("expected 'a' to match a-z")
	}
	if !Run(a, "m") {
		t.Error("expected 'm' to match a-z")
	}
	if !Run(a, "z") {
		t.Error("expected 'z' to match a-z")
	}
	if Run(a, "A") {
		t.Error("expected 'A' not to match a-z")
	}
	if Run(a, "ab") {
		t.Error("expected 'ab' not to match a-z (single char)")
	}
}

func TestAutomaton_Empty(t *testing.T) {
	a := MakeEmpty()
	if !IsEmpty(a) {
		t.Error("expected makeEmpty() automaton to be empty")
	}
	if Run(a, "") {
		t.Error("expected makeEmpty() not to accept empty string")
	}
}

func TestAutomaton_EmptyString(t *testing.T) {
	a := MakeEmptyString()
	if !Run(a, "") {
		t.Error("expected empty-string automaton to accept ''")
	}
	if Run(a, "a") {
		t.Error("expected empty-string automaton not to accept 'a'")
	}
}

func TestOperations_Union(t *testing.T) {
	u := Union([]*Automaton{MakeString("hello"), MakeString("world")})
	if !Run(mustDeterminize(t, u), "hello") {
		t.Error("expected union to match 'hello'")
	}
	if !Run(mustDeterminize(t, u), "world") {
		t.Error("expected union to match 'world'")
	}
	if Run(mustDeterminize(t, u), "helloworld") {
		t.Error("expected union not to match 'helloworld'")
	}
}

func TestOperations_Concatenate(t *testing.T) {
	c := Concatenate([]*Automaton{MakeString("hello"), MakeString(" "), MakeString("world")})
	if !Run(mustDeterminize(t, c), "hello world") {
		t.Error("expected concat to match 'hello world'")
	}
	if Run(mustDeterminize(t, c), "hello") {
		t.Error("expected concat not to match 'hello' alone")
	}
}

func TestOperations_Repeat(t *testing.T) {
	r := Repeat(MakeString("a"))
	det := mustDeterminize(t, r)
	if !Run(det, "") {
		t.Error("expected a* to match ''")
	}
	if !Run(det, "a") {
		t.Error("expected a* to match 'a'")
	}
	if !Run(det, "aaaa") {
		t.Error("expected a* to match 'aaaa'")
	}
	if Run(det, "b") {
		t.Error("expected a* not to match 'b'")
	}
}

func TestCompiledAutomaton(t *testing.T) {
	a := MakeString("test")
	c := Compile(a)
	if c == nil {
		t.Fatal("compile returned nil")
	}
	if !c.RunString("test") {
		t.Error("expected compiled('test') to accept 'test'")
	}
	if c.RunString("testing") {
		t.Error("expected compiled('test') not to accept 'testing'")
	}
	if c.TypeName() != "SINGLE" {
		t.Errorf("expected SINGLE type, got %s", c.TypeName())
	}
	if got := c.GetTerm(); got != "test" {
		t.Errorf("expected term 'test', got %q", got)
	}
}

func TestRunAutomaton_Basic(t *testing.T) {
	a := mustDeterminize(t, MakeString("hello"))
	run := NewCharacterRunAutomaton(a)
	if !run.RunString("hello") {
		t.Error("expected run automaton to accept 'hello'")
	}
	if run.RunString("world") {
		t.Error("expected run automaton not to accept 'world'")
	}
}

func TestByteRunAutomaton_Basic(t *testing.T) {
	a := mustDeterminize(t, MakeString("hi"))
	run := NewByteRunAutomaton(a)
	if !run.Run([]byte("hi"), 0, 2) {
		t.Error("expected byte run automaton to accept 'hi'")
	}
	if run.Run([]byte("ho"), 0, 2) {
		t.Error("expected byte run automaton not to accept 'ho'")
	}
}

func TestUTF32ToUTF8_Roundtrip(t *testing.T) {
	a := MakeString("é") // 2-byte UTF-8 codepoint
	conv := NewUTF32ToUTF8().Convert(a)
	det := mustDeterminize(t, conv)
	if !NewByteRunAutomatonBinary(det, true).Run([]byte("é"), 0, len([]byte("é"))) {
		t.Error("expected UTF-8 conversion to accept 'é'")
	}
}

func TestFiniteStringsIterator(t *testing.T) {
	a := mustDeterminize(t, Union([]*Automaton{MakeString("a"), MakeString("bc")}))
	it := NewFiniteStringsIterator(a)
	seen := map[string]bool{}
	for {
		ints, err := it.Next()
		if err != nil {
			t.Fatalf("iterator error: %v", err)
		}
		if ints == nil {
			break
		}
		s := make([]rune, 0, ints.Length)
		for i := 0; i < ints.Length; i++ {
			s = append(s, rune(ints.Ints[ints.Offset+i]))
		}
		seen[string(s)] = true
	}
	if !seen["a"] || !seen["bc"] {
		t.Errorf("expected to see 'a' and 'bc' in iterator; got %v", seen)
	}
}

func TestLimitedFiniteStringsIterator(t *testing.T) {
	a := mustDeterminize(t, Union([]*Automaton{MakeString("a"), MakeString("b"), MakeString("c")}))
	it, err := NewLimitedFiniteStringsIterator(a, 2)
	if err != nil {
		t.Fatalf("limited iterator: %v", err)
	}
	count := 0
	for {
		ints, err := it.Next()
		if err != nil {
			t.Fatalf("iterator error: %v", err)
		}
		if ints == nil {
			break
		}
		count++
	}
	if count != 2 {
		t.Errorf("expected at most 2 results, got %d", count)
	}
}

func TestStatePair(t *testing.T) {
	p1 := NewStatePair(1, 2)
	p2 := NewStatePair(1, 2)
	if !p1.Equals(p2) {
		t.Error("expected equal state pairs")
	}
	if p1.HashCode() != p2.HashCode() {
		t.Error("expected equal hash codes")
	}
}

func TestTooComplexToDeterminizeError(t *testing.T) {
	err := NewTooComplexToDeterminizeError(NewAutomaton(), 100)
	if !IsTooComplexToDeterminize(err) {
		t.Error("expected IsTooComplexToDeterminize to be true")
	}
}

// --- helpers ---

func mustDeterminize(t *testing.T, a *Automaton) *Automaton {
	t.Helper()
	det, err := Determinize(a, DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize failed: %v", err)
	}
	return det
}
