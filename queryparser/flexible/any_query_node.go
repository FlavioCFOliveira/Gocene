// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strconv"
	"strings"
)

// AnyQueryNode represents a query that requires at least minimumMatchingElements
// of its child clauses to match. This is the Go equivalent of Lucene's AnyQueryNode.
type AnyQueryNode struct {
	*BooleanQueryNode
	minimumMatchingElements int
}

// NewAnyQueryNode creates a new AnyQueryNode.
func NewAnyQueryNode(children []QueryNode, minimumMatchingElements int) *AnyQueryNode {
	return &AnyQueryNode{
		BooleanQueryNode:        NewBooleanQueryNode("OR", children),
		minimumMatchingElements: minimumMatchingElements,
	}
}

// GetMinimumMatchingElements returns the minimum match threshold.
func (n *AnyQueryNode) GetMinimumMatchingElements() int {
	return n.minimumMatchingElements
}

// SetMinimumMatchingElements sets the minimum match threshold.
func (n *AnyQueryNode) SetMinimumMatchingElements(min int) {
	n.minimumMatchingElements = min
}

// ToQueryString returns "(<child1> OR <child2> ...)/N".
func (n *AnyQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	children := n.GetChildren()
	var sb strings.Builder
	sb.WriteString("(")
	for i, child := range children {
		if i > 0 {
			sb.WriteString(" OR ")
		}
		sb.WriteString(child.ToQueryString(escapeSyntax))
	}
	sb.WriteString(")/")
	sb.WriteString(strconv.Itoa(n.minimumMatchingElements))
	return sb.String()
}

// String returns a debug representation.
func (n *AnyQueryNode) String() string {
	return fmt.Sprintf("<any min=%d children=%d>", n.minimumMatchingElements, len(n.GetChildren()))
}

// CloneTree deep-copies this node.
func (n *AnyQueryNode) CloneTree() QueryNode {
	cloned := &AnyQueryNode{
		BooleanQueryNode:        NewBooleanQueryNode("OR", nil),
		minimumMatchingElements: n.minimumMatchingElements,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}
	return cloned
}
