// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

import (
	"testing"
)

func TestAutomaton_Basic(t *testing.T) {
	// Create a simple automaton that matches "hello"
	a := NewAutomaton()
	initial := a.GetInitialState()
	h := a.CreateState()
	e := a.CreateState()
	l1 := a.CreateState()
	l2 := a.CreateState()
	o := a.CreateState()

	a.AddTransition(initial, h, 'h', 'h')
	a.AddTransition(h, e, 'e', 'e')
	a.AddTransition(e, l1, 'l', 'l')
	a.AddTransition(l1, l2, 'l', 'l')
	a.AddTransition(l2, o, 'o', 'o')
	a.SetAccept(o, true)

	// Test matching
	if !a.RunString("hello") {
		t.Error("Expected 'hello' to match")
	}

	if a.RunString("world") {
		t.Error("Expected 'world' to not match")
	}

	if a.RunString("hel") {
		t.Error("Expected 'hel' to not match (incomplete)")
	}

	if a.RunString("helloo") {
		t.Error("Expected 'helloo' to not match (too long)")
	}
}

func TestAutomaton_AnyChar(t *testing.T) {
	factory := NewAutomata()
	a := factory.MakeAnyChar()

	if !a.RunString("a") {
		t.Error("Expected 'a' to match any char")
	}

	if !a.RunString("z") {
		t.Error("Expected 'z' to match any char")
	}

	if a.RunString("ab") {
		t.Error("Expected 'ab' to not match any char (too long)")
	}

	if a.RunString("") {
		t.Error("Expected empty string to not match any char")
	}
}

func TestAutomaton_AnyString(t *testing.T) {
	factory := NewAutomata()
	a := factory.MakeAnyString()

	if !a.RunString("") {
		t.Error("Expected empty string to match any string")
	}

	if !a.RunString("hello") {
		t.Error("Expected 'hello' to match any string")
	}

	if !a.RunString("a very long string with many words") {
		t.Error("Expected long string to match any string")
	}
}

func TestAutomaton_MakeString(t *testing.T) {
	factory := NewAutomata()
	a := factory.MakeString("test")

	if !a.RunString("test") {
		t.Error("Expected 'test' to match")
	}

	if a.RunString("testing") {
		t.Error("Expected 'testing' to not match")
	}

	if a.RunString("tes") {
		t.Error("Expected 'tes' to not match")
	}
}

func TestAutomaton_CharRange(t *testing.T) {
	factory := NewAutomata()
	a := factory.MakeCharRange('a', 'z')

	if !a.RunString("a") {
		t.Error("Expected 'a' to match range a-z")
	}

	if !a.RunString("m") {
		t.Error("Expected 'm' to match range a-z")
	}

	if !a.RunString("z") {
		t.Error("Expected 'z' to match range a-z")
	}

	if a.RunString("A") {
		t.Error("Expected 'A' to not match range a-z")
	}

	if a.RunString("ab") {
		t.Error("Expected 'ab' to not match (single char only)")
	}
}

func TestAutomaton_Empty(t *testing.T) {
	factory := NewAutomata()
	a := factory.MakeEmpty()

	if !a.IsEmpty() {
		t.Error("Expected empty automaton to be empty")
	}

	if a.RunString("") {
		t.Error("Expected empty automaton to not match empty string")
	}
}

func TestAutomaton_EmptyString(t *testing.T) {
	factory := NewAutomata()
	a := factory.MakeEmptyString()

	if !a.IsEmptyString() {
		t.Error("Expected empty string automaton to be empty string")
	}

	if !a.RunString("") {
		t.Error("Expected empty string automaton to match empty string")
	}

	if a.RunString("a") {
		t.Error("Expected empty string automaton to not match 'a'")
	}
}

func TestAutomaton_Clone(t *testing.T) {
	factory := NewAutomata()
	original := factory.MakeString("test")
	clone := original.Clone()

	if !original.Equals(clone) {
		t.Error("Expected clone to equal original")
	}

	// Modify clone (by creating new state) - should not affect original
	clone.CreateState()
	if original.NumStates() == clone.NumStates() {
		t.Error("Expected clone to be independent of original")
	}
}

func TestAutomaton_HashCode(t *testing.T) {
	factory := NewAutomata()
	a1 := factory.MakeString("test")
	a2 := factory.MakeString("test")
	a3 := factory.MakeString("different")

	if a1.HashCode() != a2.HashCode() {
		t.Error("Expected equal automatons to have same hash code")
	}

	// Note: different automatons might have same hash code (collision)
	// but it's unlikely
	if a1.HashCode() == a3.HashCode() {
		t.Log("Warning: hash collision detected (this is acceptable)")
	}
}

func TestOperations_Union(t *testing.T) {
	factory := NewAutomata()
	a1 := factory.MakeString("hello")
	a2 := factory.MakeString("world")

	ops := NewOperations()
	union := ops.Union([]*Automaton{a1, a2})

	if !union.RunString("hello") {
		t.Error("Expected union to match 'hello'")
	}

	if !union.RunString("world") {
		t.Error("Expected union to match 'world'")
	}

	if union.RunString("helloworld") {
		t.Error("Expected union to not match 'helloworld'")
	}
}

func TestOperations_Concatenate(t *testing.T) {
	factory := NewAutomata()
	a1 := factory.MakeString("hello")
	a2 := factory.MakeString(" ")
	a3 := factory.MakeString("world")

	ops := NewOperations()
	concat := ops.Concatenate([]*Automaton{a1, a2, a3})

	if !concat.RunString("hello world") {
		t.Error("Expected concatenation to match 'hello world'")
	}

	if concat.RunString("hello") {
		t.Error("Expected concatenation to not match 'hello' alone")
	}
}

func TestOperations_Repeat(t *testing.T) {
	factory := NewAutomata()
	a := factory.MakeString("a")

	ops := NewOperations()
	star := ops.Repeat(a)

	if !star.RunString("") {
		t.Error("Expected star to match empty string")
	}

	if !star.RunString("a") {
		t.Error("Expected star to match 'a'")
	}

	if !star.RunString("aaaa") {
		t.Error("Expected star to match 'aaaa'")
	}

	if star.RunString("b") {
		t.Error("Expected star to not match 'b'")
	}
}

func TestCompiledAutomaton(t *testing.T) {
	factory := NewAutomata()
	a := factory.MakeString("test")

	compiled := Compile(a)

	if compiled == nil {
		t.Fatal("Expected compiled automaton to not be nil")
	}

	if !compiled.RunString("test") {
		t.Error("Expected compiled automaton to match 'test'")
	}

	if compiled.RunString("testing") {
		t.Error("Expected compiled automaton to not match 'testing'")
	}

	// Check type
	if compiled.Type() != "SINGLE" {
		t.Errorf("Expected type SINGLE, got %s", compiled.Type())
	}

	// Check term
	if compiled.GetTerm() != "test" {
		t.Errorf("Expected term 'test', got '%s'", compiled.GetTerm())
	}
}

func TestRunAutomaton(t *testing.T) {
	factory := NewAutomata()
	a := factory.MakeString("hello")
	compiled := Compile(a)

	run := NewRunAutomaton(compiled)

	if !run.RunString("hello") {
		t.Error("Expected run automaton to match 'hello'")
	}

	if run.RunString("world") {
		t.Error("Expected run automaton to not match 'world'")
	}
}

func BenchmarkAutomaton_RunString(b *testing.B) {
	factory := NewAutomata()
	a := factory.MakeString("hello")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.RunString("hello")
	}
}

func BenchmarkCompiledAutomaton_RunString(b *testing.B) {
	factory := NewAutomata()
	a := factory.MakeString("hello")
	compiled := Compile(a)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled.RunString("hello")
	}
}
