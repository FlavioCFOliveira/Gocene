// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// DeletedQueryNode represents a node that was deleted from the query node tree.
// This is the Go equivalent of Lucene's DeletedQueryNode.
type DeletedQueryNode struct {
	*QueryNodeImpl
}

// NewDeletedQueryNode creates a new DeletedQueryNode.
func NewDeletedQueryNode() *DeletedQueryNode {
	n := &DeletedQueryNode{QueryNodeImpl: NewQueryNodeImpl(nil)}
	// DeletedQueryNode is a leaf in Lucene: it never calls setLeaf(false).
	n.SetLeaf(true)
	return n
}

// ToQueryString returns the deleted marker.
func (n *DeletedQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	return "[DELETEDCHILD]"
}

// String returns a debug representation.
func (n *DeletedQueryNode) String() string {
	return "<deleted/>"
}

// CloneTree deep-copies this node.
func (n *DeletedQueryNode) CloneTree() QueryNode {
	cloned := &DeletedQueryNode{QueryNodeImpl: NewQueryNodeImpl(nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}
