// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "strconv"

// MinShouldMatchNode represents a minimum-should-match restriction on a
// GroupQueryNode.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.MinShouldMatchNode.
type MinShouldMatchNode struct {
	*QueryNodeImpl
	// MinShouldMatch and GroupQueryNode are public final fields in Java.
	MinShouldMatch int
	GroupQueryNode *GroupQueryNode
}

// NewMinShouldMatchNode is the sole constructor.
//
// Mirrors MinShouldMatchNode(int minShouldMatch, GroupQueryNode groupQueryNode),
// which runs setLeaf(false); allocate(); add(groupQueryNode).
func NewMinShouldMatchNode(minShouldMatch int, groupQueryNode *GroupQueryNode) *MinShouldMatchNode {
	n := &MinShouldMatchNode{
		QueryNodeImpl:  NewQueryNodeImpl(nil),
		MinShouldMatch: minShouldMatch,
		GroupQueryNode: groupQueryNode,
	}
	n.AddChild(groupQueryNode)
	return n
}

// GetMinimumShouldMatch returns the minimum-should-match value.
func (n *MinShouldMatchNode) GetMinimumShouldMatch() int { return n.MinShouldMatch }

// ToQueryString renders the group followed by "@" and the minimum count.
//
// Mirrors MinShouldMatchNode#toQueryString(EscapeQuerySyntax).
func (n *MinShouldMatchNode) ToQueryString(escapeSyntaxParser EscapeQuerySyntax) string {
	return n.GroupQueryNode.ToQueryString(escapeSyntaxParser) + "@" + strconv.Itoa(n.MinShouldMatch)
}

// CloneTree deep-copies this node.
func (n *MinShouldMatchNode) CloneTree() QueryNode {
	group, _ := n.GroupQueryNode.CloneTree().(*GroupQueryNode)
	return NewMinShouldMatchNode(n.MinShouldMatch, group)
}
