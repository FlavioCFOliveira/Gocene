// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

import "testing"

func TestRegExp_LiteralString(t *testing.T) {
	r, err := NewRegExp("hello")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	det := mustDeterminize(t, a)
	if !Run(det, "hello") {
		t.Error("expected match for literal 'hello'")
	}
	if Run(det, "hell") {
		t.Error("expected no match for 'hell'")
	}
}

func TestRegExp_AnyChar(t *testing.T) {
	r, err := NewRegExp(".")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	det := mustDeterminize(t, a)
	if !Run(det, "a") {
		t.Error("expected '.' to match 'a'")
	}
	if Run(det, "") {
		t.Error("expected '.' not to match ''")
	}
}

func TestRegExp_AnyString(t *testing.T) {
	r, err := NewRegExp("@")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	det := mustDeterminize(t, a)
	if !Run(det, "") {
		t.Error("expected '@' to match empty string")
	}
	if !Run(det, "anything") {
		t.Error("expected '@' to match 'anything'")
	}
}

func TestRegExp_Repeat(t *testing.T) {
	r, err := NewRegExp("a*")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	det := mustDeterminize(t, a)
	if !Run(det, "") || !Run(det, "a") || !Run(det, "aaa") {
		t.Error("expected a* to match '', 'a', 'aaa'")
	}
	if Run(det, "b") {
		t.Error("expected a* not to match 'b'")
	}
}

func TestRegExp_Union(t *testing.T) {
	r, err := NewRegExp("a|b")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	det := mustDeterminize(t, a)
	if !Run(det, "a") || !Run(det, "b") {
		t.Error("expected union to match 'a' and 'b'")
	}
	if Run(det, "c") {
		t.Error("expected union not to match 'c'")
	}
}

func TestRegExp_CharClass(t *testing.T) {
	r, err := NewRegExp("[abc]")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	det := mustDeterminize(t, a)
	for _, in := range []string{"a", "b", "c"} {
		if !Run(det, in) {
			t.Errorf("expected [abc] to match %q", in)
		}
	}
	if Run(det, "d") {
		t.Error("expected [abc] not to match 'd'")
	}
}

func TestRegExp_CharRange(t *testing.T) {
	r, err := NewRegExp("[a-z]+")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	det := mustDeterminize(t, a)
	if !Run(det, "hello") {
		t.Error("expected [a-z]+ to match 'hello'")
	}
	if Run(det, "Hello") {
		t.Error("expected [a-z]+ not to match 'Hello'")
	}
}

func TestRegExp_DigitClass(t *testing.T) {
	r, err := NewRegExp(`\d+`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	det := mustDeterminize(t, a)
	if !Run(det, "123") {
		t.Error("expected \\d+ to match '123'")
	}
	if Run(det, "abc") {
		t.Error("expected \\d+ not to match 'abc'")
	}
}

func TestRegExp_RepeatRange(t *testing.T) {
	r, err := NewRegExp("a{2,3}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	det := mustDeterminize(t, a)
	if Run(det, "a") {
		t.Error("expected a{2,3} not to match 'a'")
	}
	if !Run(det, "aa") || !Run(det, "aaa") {
		t.Error("expected a{2,3} to match 'aa' and 'aaa'")
	}
	if Run(det, "aaaa") {
		t.Error("expected a{2,3} not to match 'aaaa'")
	}
}

func TestRegExp_CaseInsensitiveRejected(t *testing.T) {
	r, err := NewRegExpFlags("abc", RegExpAll, RegExpCaseInsensitive)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := r.ToAutomaton(); err == nil {
		t.Error("expected case-insensitive ToAutomaton to fail until CaseFolding is ported")
	}
}
