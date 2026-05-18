// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"strings"
	"testing"
)

func TestTermRangeQueryNode(t *testing.T) {
	lo := NewFieldQueryNode("", "aaa", 0, 3)
	hi := NewFieldQueryNode("", "zzz", 4, 7)
	n := NewTermRangeQueryNode("f", lo, hi, true, false)

	qs := n.ToQueryString(false)
	if qs != "f:[aaa TO zzz}" {
		t.Errorf("ToQueryString() = %q, want f:[aaa TO zzz}", qs)
	}

	cloned := n.CloneTree().(*TermRangeQueryNode)
	if cloned.GetField() != "f" {
		t.Errorf("clone field = %q", cloned.GetField())
	}
	if !cloned.IsLowerInclusive() || cloned.IsUpperInclusive() {
		t.Error("clone bounds wrong")
	}
}

func TestPointQueryNode(t *testing.T) {
	n := NewPointQueryNode("price", []byte{0x00, 0x01, 0x02})
	v := n.GetPointValue()
	if len(v) != 3 || v[2] != 0x02 {
		t.Errorf("GetPointValue() = %v", v)
	}
	cloned := n.CloneTree().(*PointQueryNode)
	cv := cloned.GetPointValue()
	if len(cv) != 3 {
		t.Errorf("clone value len = %d", len(cv))
	}
}

func TestPrefixWildcardQueryNode(t *testing.T) {
	n := NewPrefixWildcardQueryNode("title", "foo", 0, 3)
	qs := n.ToQueryString(false)
	if qs != "title:foo*" {
		t.Errorf("ToQueryString() = %q, want title:foo*", qs)
	}

	// If already ends with *, don't double it
	n2 := NewPrefixWildcardQueryNode("title", "foo*", 0, 4)
	qs2 := n2.ToQueryString(false)
	if qs2 != "title:foo*" {
		t.Errorf("ToQueryString() = %q, want title:foo*", qs2)
	}

	cloned := n.CloneTree().(*PrefixWildcardQueryNode)
	if cloned.GetText() != "foo" {
		t.Error("clone text wrong")
	}
}

func TestWildcardQueryNode(t *testing.T) {
	n := NewWildcardQueryNode("body", "foo*bar", 0, 7)
	if n.GetText() != "foo*bar" {
		t.Errorf("GetText() = %q", n.GetText())
	}
	cloned := n.CloneTree().(*WildcardQueryNode)
	if cloned.GetField() != "body" {
		t.Error("clone field wrong")
	}
}

func TestRegexpQueryNode(t *testing.T) {
	n := NewRegexpQueryNode("url", "https?://.*", 0, 11)
	qs := n.ToQueryString(false)
	if qs != "url:/https?://.*/}" {
		// acceptable formats: url:/https?:\/\/.*/
		if !strings.Contains(qs, "/https?://") {
			t.Errorf("ToQueryString() = %q, want url:/regexp/", qs)
		}
	}
	cloned := n.CloneTree().(*RegexpQueryNode)
	if cloned.GetText() != "https?://.*" {
		t.Error("clone text wrong")
	}
}

func TestSynonymQueryNode(t *testing.T) {
	c1 := NewFieldQueryNode("body", "car", 0, 3)
	c2 := NewFieldQueryNode("body", "automobile", 0, 10)
	n := NewSynonymQueryNode("body", []QueryNode{c1, c2})

	qs := n.ToQueryString(false)
	if !strings.Contains(qs, "OR") {
		t.Errorf("ToQueryString() = %q, should contain OR", qs)
	}
	cloned := n.CloneTree().(*SynonymQueryNode)
	if len(cloned.GetChildren()) != 2 {
		t.Errorf("clone children = %d, want 2", len(cloned.GetChildren()))
	}
}

func TestBooleanModifierNode(t *testing.T) {
	child := NewFieldQueryNode("f", "x", 0, 1)
	n := NewBooleanModifierNode(child, ModifierRequired)
	if n.GetModifier() != ModifierRequired {
		t.Error("modifier wrong")
	}
	cloned := n.CloneTree().(*BooleanModifierNode)
	if cloned.GetModifier() != ModifierRequired {
		t.Error("clone modifier wrong")
	}
}

func TestMinShouldMatchNode(t *testing.T) {
	child := NewOrQueryNode([]QueryNode{
		NewFieldQueryNode("f", "a", 0, 1),
		NewFieldQueryNode("f", "b", 0, 1),
	})
	n := NewMinShouldMatchNode(child, 2)
	if n.GetMinimumShouldMatch() != 2 {
		t.Errorf("GetMinimumShouldMatch() = %d, want 2", n.GetMinimumShouldMatch())
	}
	qs := n.ToQueryString(false)
	if !strings.HasSuffix(qs, "@2") {
		t.Errorf("ToQueryString() = %q, should end with @2", qs)
	}
	cloned := n.CloneTree().(*MinShouldMatchNode)
	if cloned.GetMinimumShouldMatch() != 2 {
		t.Error("clone min wrong")
	}
}

func TestMultiPhraseQueryNode(t *testing.T) {
	c1 := NewFieldQueryNode("", "quick", 0, 5)
	c2 := NewFieldQueryNode("", "fox", 6, 9)
	n := NewMultiPhraseQueryNode("body", []QueryNode{c1, c2})
	qs := n.ToQueryString(false)
	if !strings.Contains(qs, "body:") || !strings.Contains(qs, "quick") {
		t.Errorf("ToQueryString() = %q unexpected", qs)
	}
	cloned := n.CloneTree().(*MultiPhraseQueryNode)
	if cloned.GetField() != "body" || len(cloned.GetChildren()) != 2 {
		t.Error("clone wrong")
	}
}

func TestIntervalQueryNode(t *testing.T) {
	c1 := NewFieldQueryNode("", "quick", 0, 5)
	n := NewIntervalQueryNode("body", "ORDERED", []QueryNode{c1})
	qs := n.ToQueryString(false)
	if !strings.Contains(qs, "ORDERED") || !strings.Contains(qs, "body") {
		t.Errorf("ToQueryString() = %q unexpected", qs)
	}
	cloned := n.CloneTree().(*IntervalQueryNode)
	if cloned.GetFunction() != "ORDERED" {
		t.Error("clone function wrong")
	}
}
