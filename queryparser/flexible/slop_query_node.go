// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "strconv"

// SlopQueryNode wraps a child node and adds a slop value to it.
// This is the Go equivalent of Lucene's SlopQueryNode.
type SlopQueryNode struct {
	*QueryNodeImpl
	value int
}

// NewSlopQueryNode creates a new SlopQueryNode wrapping child with the given slop.
func NewSlopQueryNode(child QueryNode, value int) *SlopQueryNode {
	node := &SlopQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		value:         value,
	}
	if child != nil {
		node.AddChild(child)
	}
	return node
}

// GetValue returns the slop value.
func (n *SlopQueryNode) GetValue() int { return n.value }

// SetValue sets the slop value.
func (n *SlopQueryNode) SetValue(value int) { n.value = value }

// ToQueryString appends ~slop to the child's representation.
func (n *SlopQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	children := n.GetChildren()
	if len(children) == 0 {
		return ""
	}
	return children[0].ToQueryString(escapeSyntax) + "~" + strconv.Itoa(n.value)
}

// CloneTree deep-copies this node.
func (n *SlopQueryNode) CloneTree() QueryNode {
	cloned := &SlopQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		value:         n.value,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}
	return cloned
}

// String returns a debug representation.
func (n *SlopQueryNode) String() string {
	return " <slop value=" + strconv.Itoa(n.value) + ">"
}
