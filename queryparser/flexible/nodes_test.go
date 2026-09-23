// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// Port of
// lucene/queryparser/src/test/org/apache/lucene/queryparser/flexible/core/nodes/TestQueryNode.java
// (Apache Lucene 10.5.0). Gocene renders QueryNode.add(QueryNode),
// add(List), set(List), removeChildren(QueryNode) and containsTag(String) as
// AddChild, AddChildren, SetChildren, RemoveChild and HasTag; the Java
// BooleanQueryNode(List) constructor is NewBooleanQueryNode with an empty
// operator.

import (
	"slices"
	"testing"
)

/* LUCENE-2227 bug in QueryNodeImpl.add() */
func TestQueryNode_testAddChildren(t *testing.T) {
	var nodeA QueryNode = NewFieldQueryNode("foo", "A", 0, 1)
	var nodeB QueryNode = NewFieldQueryNode("foo", "B", 1, 2)
	bq := NewBooleanQueryNode("", []QueryNode{nodeA})
	bq.AddChildren([]QueryNode{nodeB})
	if got := len(bq.GetChildren()); got != 2 {
		t.Fatalf("children = %d, want 2", got)
	}
}

/* LUCENE-3045 bug in QueryNodeImpl.containsTag(String key)*/
func TestQueryNode_testTags(t *testing.T) {
	var node QueryNode = NewFieldQueryNode("foo", "A", 0, 1)

	node.SetTag("TaG", new(struct{ _ byte }))
	if !(len(node.GetTagMap()) > 0) {
		t.Fatal("getTagMap().size() > 0")
	}
	if !node.HasTag("tAg") {
		t.Fatal(`containsTag("tAg")`)
	}
	if node.GetTag("tAg") == nil {
		t.Fatal(`getTag("tAg") != null`)
	}
}

/* LUCENE-5099 - QueryNodeProcessorImpl should set parent to null before returning on processing */
func TestQueryNode_testRemoveFromParent(t *testing.T) {
	booleanNode := NewBooleanQueryNode("", []QueryNode{})
	fieldNode := NewFieldQueryNode("foo", "A", 0, 1)
	if fieldNode.GetParent() != nil {
		t.Fatal("assertNull(fieldNode.getParent())")
	}

	booleanNode.AddChild(fieldNode)
	if fieldNode.GetParent() == nil {
		t.Fatal("assertNotNull(fieldNode.getParent())")
	}

	fieldNode.RemoveFromParent()
	if fieldNode.GetParent() != nil {
		t.Fatal("assertNull(fieldNode.getParent())")
	}
	/* LUCENE-5805 - QueryNodeImpl.removeFromParent does a lot of work without any effect */
	if slices.Contains(booleanNode.GetChildren(), QueryNode(fieldNode)) {
		t.Fatal("assertFalse(booleanNode.getChildren().contains(fieldNode))")
	}

	booleanNode.AddChild(fieldNode)
	if fieldNode.GetParent() == nil {
		t.Fatal("assertNotNull(fieldNode.getParent())")
	}

	booleanNode.SetChildren([]QueryNode{})
	if fieldNode.GetParent() != nil {
		t.Fatal("assertNull(fieldNode.getParent())")
	}
}

func TestQueryNode_testRemoveChildren(t *testing.T) {
	booleanNode := NewBooleanQueryNode("", []QueryNode{})
	fieldNode := NewFieldQueryNode("foo", "A", 0, 1)

	booleanNode.AddChild(fieldNode)
	if !(len(booleanNode.GetChildren()) == 1) {
		t.Fatal("booleanNode.getChildren().size() == 1")
	}

	booleanNode.RemoveChild(fieldNode)
	if !(len(booleanNode.GetChildren()) == 0) {
		t.Fatal("booleanNode.getChildren().size() == 0")
	}
	if fieldNode.GetParent() != nil {
		t.Fatal("assertNull(fieldNode.getParent())")
	}
}
