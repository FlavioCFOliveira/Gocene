// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"testing"
)

func TestFuzzyQueryNode(t *testing.T) {
	node := NewFuzzyQueryNode("field", "text", 0.8, 1, 5)
	if node.GetSimilarity() != 0.8 {
		t.Errorf("expected similarity 0.8, got %g", node.GetSimilarity())
	}
	if node.GetField() != "field" {
		t.Errorf("expected field field, got %s", node.GetField())
	}
	if node.GetText() != "text" {
		t.Errorf("expected text text, got %s", node.GetText())
	}

	esc := NewEscapeQuerySyntaxImpl()
	expected := "field:text~0.8"
	if got := node.ToQueryString(esc); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}

	cloned := node.CloneTree().(*FuzzyQueryNode)
	if cloned.GetSimilarity() != 0.8 {
		t.Errorf("cloned similarity mismatch")
	}
}

func TestMultiPhraseQueryNode(t *testing.T) {
	node := NewMultiPhraseQueryNode()
	node.AddChild(NewFieldQueryNode("f1", "t1", 1, 3))
	node.AddChild(NewFieldQueryNode("f1", "t2", 4, 6))

	if node.GetField() != "f1" {
		t.Errorf("expected field f1, got %s", node.GetField())
	}

	esc := NewEscapeQuerySyntaxImpl()
	expected := "[MTP[f1:t1,f1:t2]]"
	if got := node.ToQueryString(esc); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}

	node.SetField("f2")
	if node.GetChildren()[0].(*FieldQueryNode).GetField() != "f2" {
		t.Errorf("SetField failed to propagate")
	}
}

func TestDeletedQueryNode(t *testing.T) {
	node := NewDeletedQueryNode()
	esc := NewEscapeQuerySyntaxImpl()
	if got := node.ToQueryString(esc); got != "[DELETEDCHILD]" {
		t.Errorf("expected [DELETEDCHILD], got %s", got)
	}
}

func TestAnyQueryNode(t *testing.T) {
	children := []QueryNode{
		NewFieldQueryNode("f1", "t1", 1, 3),
		NewFieldQueryNode("f1", "t2", 4, 6),
	}
	node := NewAnyQueryNode(children, 1)
	if node.GetMinimumMatchingElements() != 1 {
		t.Errorf("expected min 1, got %d", node.GetMinimumMatchingElements())
	}

	esc := NewEscapeQuerySyntaxImpl()
	expected := "(f1:t1 OR f1:t2)/1"
	if got := node.ToQueryString(esc); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestNoTokenFoundQueryNode(t *testing.T) {
	node := NewNoTokenFoundQueryNode()
	esc := NewEscapeQuerySyntaxImpl()
	if got := node.ToQueryString(esc); got != "[NTF]" {
		t.Errorf("expected [NTF], got %s", got)
	}
}

func TestOpaqueQueryNode(t *testing.T) {
	node := NewOpaqueQueryNode("xpath", "/book/1")
	if node.GetSchema() != "xpath" {
		t.Errorf("expected schema xpath, got %s", node.GetSchema())
	}
	if node.GetValue() != "/book/1" {
		t.Errorf("expected value /book/1, got %s", node.GetValue())
	}

	esc := NewEscapeQuerySyntaxImpl()
	expected := "@xpath:'/book/1'"
	if got := node.ToQueryString(esc); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPathQueryNode(t *testing.T) {
	elements := []PathQueryNodeText{
		{"company", 1, 7},
		{"USA", 9, 12},
		{"California", 14, 23},
	}
	node := NewPathQueryNode(elements)

	if node.GetFirstPathElement() != "company" {
		t.Errorf("expected first company, got %s", node.GetFirstPathElement())
	}

	esc := NewEscapeQuerySyntaxImpl()
	expected := "/company/\"USA\"/\"California\""
	if got := node.ToQueryString(esc); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestProximityQueryNode(t *testing.T) {
	node := NewProximityQueryNode("field", "text", 5, ProximityWord, 1, 5)
	esc := NewEscapeQuerySyntaxImpl()
	expected := "field:text~WORD/5"
	if got := node.ToQueryString(esc); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestSlopQueryNode(t *testing.T) {
	child := NewFieldQueryNode("field", "text", 1, 5)
	node := NewSlopQueryNode(child, 3)
	esc := NewEscapeQuerySyntaxImpl()
	expected := "field:text~3"
	if got := node.ToQueryString(esc); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestTokenizedPhraseQueryNode(t *testing.T) {
	children := []QueryNode{
		NewFieldQueryNode("f1", "t1", 1, 3),
		NewFieldQueryNode("f1", "t2", 4, 6),
	}
	node := NewTokenizedPhraseQueryNode("f1", children)
	esc := NewEscapeQuerySyntaxImpl()
	expected := "[TP[f1:t1,f1:t2]]"
	if got := node.ToQueryString(esc); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
