// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"strings"
	"testing"
)

func TestDeletedQueryNode(t *testing.T) {
	n := NewDeletedQueryNode()
	if n.ToQueryString(false) != "" {
		t.Error("ToQueryString should be empty")
	}
	cloned := n.CloneTree()
	if cloned == nil {
		t.Error("CloneTree returned nil")
	}
	if _, ok := cloned.(*DeletedQueryNode); !ok {
		t.Errorf("CloneTree type = %T, want *DeletedQueryNode", cloned)
	}
}

func TestAnyQueryNode(t *testing.T) {
	c1 := NewFieldQueryNode("f", "a", 0, 1)
	c2 := NewFieldQueryNode("f", "b", 2, 3)
	n := NewAnyQueryNode([]QueryNode{c1, c2}, 1)

	if n.GetMinimumMatchingElements() != 1 {
		t.Errorf("GetMinimumMatchingElements() = %d, want 1", n.GetMinimumMatchingElements())
	}
	qs := n.ToQueryString(false)
	if !strings.Contains(qs, "OR") || !strings.HasSuffix(qs, "/1") {
		t.Errorf("ToQueryString() = %q, unexpected format", qs)
	}

	cloned := n.CloneTree().(*AnyQueryNode)
	if cloned.GetMinimumMatchingElements() != 1 {
		t.Error("clone lost minimumMatchingElements")
	}
	if len(cloned.GetChildren()) != 2 {
		t.Errorf("clone has %d children, want 2", len(cloned.GetChildren()))
	}
}

func TestNoTokenFoundQueryNode(t *testing.T) {
	n := NewNoTokenFoundQueryNode("body", "stop", 0, 4)
	if n.ToQueryString(false) != "" {
		t.Error("ToQueryString should be empty")
	}
	if n.GetField() != "body" {
		t.Errorf("GetField() = %q, want %q", n.GetField(), "body")
	}
	cloned := n.CloneTree()
	if _, ok := cloned.(*NoTokenFoundQueryNode); !ok {
		t.Errorf("CloneTree type = %T, want *NoTokenFoundQueryNode", cloned)
	}
}

func TestOpaqueQueryNode(t *testing.T) {
	n := NewOpaqueQueryNode("geo", "48.8566,2.3522")
	if n.GetSchema() != "geo" {
		t.Errorf("GetSchema() = %q, want geo", n.GetSchema())
	}
	if n.GetValue() != "48.8566,2.3522" {
		t.Errorf("GetValue() = %q unexpected", n.GetValue())
	}
	qs := n.ToQueryString(false)
	if qs != "@geo:48.8566,2.3522" {
		t.Errorf("ToQueryString() = %q, want @geo:48.8566,2.3522", qs)
	}
	cloned := n.CloneTree().(*OpaqueQueryNode)
	if cloned.GetSchema() != "geo" || cloned.GetValue() != "48.8566,2.3522" {
		t.Error("clone fields differ")
	}
}

func TestPathQueryNode(t *testing.T) {
	n := NewPathQueryNode([]string{"root", "child", "leaf"})
	qs := n.ToQueryString(false)
	if qs != "root/child/leaf" {
		t.Errorf("ToQueryString() = %q, want root/child/leaf", qs)
	}
	cloned := n.CloneTree().(*PathQueryNode)
	elems := cloned.GetPathElements()
	if len(elems) != 3 || elems[2] != "leaf" {
		t.Error("clone path elements differ")
	}
}

func TestProximityQueryNode(t *testing.T) {
	n := NewProximityQueryNode("body", "hello world", 5, ProximityWord, 0, 11)
	if n.GetDistance() != 5 {
		t.Errorf("GetDistance() = %d, want 5", n.GetDistance())
	}
	if n.GetProximityType() != ProximityWord {
		t.Error("GetProximityType() != ProximityWord")
	}
	qs := n.ToQueryString(false)
	if !strings.Contains(qs, "WORD") || !strings.Contains(qs, "5") {
		t.Errorf("ToQueryString() = %q unexpected", qs)
	}
	cloned := n.CloneTree().(*ProximityQueryNode)
	if cloned.GetDistance() != 5 {
		t.Error("clone distance differs")
	}
}

func TestQuotedFieldQueryNode(t *testing.T) {
	n := NewQuotedFieldQueryNode("title", "hello world", 0, 11)
	qs := n.ToQueryString(false)
	if qs != `title:"hello world"` {
		t.Errorf("ToQueryString() = %q, want title:\"hello world\"", qs)
	}
	cloned := n.CloneTree().(*QuotedFieldQueryNode)
	if cloned.GetField() != "title" || cloned.GetText() != "hello world" {
		t.Error("clone fields differ")
	}
}

func TestSlopQueryNode(t *testing.T) {
	child := NewFieldQueryNode("f", "quick fox", 0, 9)
	n := NewSlopQueryNode(child, 2)
	if n.GetValue() != 2 {
		t.Errorf("GetValue() = %d, want 2", n.GetValue())
	}
	qs := n.ToQueryString(false)
	if !strings.HasSuffix(qs, "~2") {
		t.Errorf("ToQueryString() = %q, should end with ~2", qs)
	}
	cloned := n.CloneTree().(*SlopQueryNode)
	if cloned.GetValue() != 2 {
		t.Error("clone value differs")
	}
}

func TestTokenizedPhraseQueryNode(t *testing.T) {
	t1 := NewFieldQueryNode("", "quick", 0, 5)
	t2 := NewFieldQueryNode("", "fox", 6, 9)
	n := NewTokenizedPhraseQueryNode("body", []QueryNode{t1, t2})

	if n.GetField() != "body" {
		t.Errorf("GetField() = %q, want body", n.GetField())
	}
	qs := n.ToQueryString(false)
	if qs != `body:"quick fox"` {
		t.Errorf("ToQueryString() = %q, want body:\"quick fox\"", qs)
	}
	cloned := n.CloneTree().(*TokenizedPhraseQueryNode)
	if cloned.GetField() != "body" || len(cloned.GetChildren()) != 2 {
		t.Error("clone fields or children differ")
	}
}

func TestFieldQueryNode_ValueInterface(t *testing.T) {
	n := NewFieldQueryNode("f", "hello", 0, 5)
	if n.GetValue() != "hello" {
		t.Errorf("GetValue() = %v, want hello", n.GetValue())
	}
	n.SetValue("world")
	if n.GetText() != "world" {
		t.Errorf("text after SetValue = %q, want world", n.GetText())
	}
	n.SetValue(42) // non-string: should be no-op
	if n.GetText() != "world" {
		t.Errorf("text after invalid SetValue = %q, want world", n.GetText())
	}
}

func TestFieldableNodeInterface(t *testing.T) {
	var _ FieldableNode = (*FieldQueryNode)(nil)
	var _ FieldableNode = (*QuotedFieldQueryNode)(nil)
	var _ FieldableNode = (*TokenizedPhraseQueryNode)(nil)
}

func TestTextableNodeInterface(t *testing.T) {
	var _ TextableQueryNode = (*FieldQueryNode)(nil)
	var _ TextableQueryNode = (*QuotedFieldQueryNode)(nil)
}
