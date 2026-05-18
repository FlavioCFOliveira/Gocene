// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "testing"

func TestStringUtils_ToString(t *testing.T) {
	su := StringUtils{}
	if su.ToString(nil) != "" {
		t.Error("nil should return empty string")
	}
	if su.ToString("hello") != "hello" {
		t.Errorf("string passthrough failed")
	}
	type stringer struct{ s string }
	// Does not implement String() — falls through to empty
	if got := su.ToString(stringer{"x"}); got != "" {
		t.Errorf("non-stringer should return empty, got %q", got)
	}
}

func TestUnescapedCharSequence(t *testing.T) {
	u := NewUnescapedCharSequence(`hello\ world`)
	// \ before space: space is escaped, backslash consumed
	if u.Length() != 11 { // "hello world" = 11 chars, one with escape
		t.Errorf("Length() = %d, want 11", u.Length())
	}
	// index 5 is the space that was escaped
	if !u.WasEscaped(5) {
		t.Error("index 5 (space) should be escaped")
	}
	if u.WasEscaped(0) {
		t.Error("index 0 ('h') should not be escaped")
	}
	if u.String() != "hello world" {
		t.Errorf("String() = %q, want hello world", u.String())
	}
	esc := u.ToStringEscaped()
	if esc != `hello\ world` {
		t.Errorf("ToStringEscaped() = %q, want hello\\ world", esc)
	}
}

func TestUnescapedCharSequence_SubSequence(t *testing.T) {
	u := NewUnescapedCharSequence("abc")
	sub := u.SubSequence(1, 3)
	if sub.String() != "bc" {
		t.Errorf("SubSequence(1,3) = %q, want bc", sub.String())
	}
}

func TestQueryNodeOperation_Union(t *testing.T) {
	op := QueryNodeOperation{}
	n1 := NewAndQueryNode([]QueryNode{NewFieldQueryNode("f", "a", 0, 1)})
	n2 := NewAndQueryNode([]QueryNode{NewFieldQueryNode("f", "b", 0, 1), NewFieldQueryNode("f", "c", 0, 1)})
	merged := op.UnionQueryNodesContents(n1, n2)
	if len(merged.GetChildren()) != 3 {
		t.Errorf("union children = %d, want 3", len(merged.GetChildren()))
	}
}

func TestQueryNodeOperation_NilSafe(t *testing.T) {
	op := QueryNodeOperation{}
	result := op.UnionQueryNodesContents(nil, nil)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.GetChildren()) != 0 {
		t.Errorf("children = %d, want 0", len(result.GetChildren()))
	}
}
