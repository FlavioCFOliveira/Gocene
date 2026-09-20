// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// PrefixWildcardQueryNode represents a wildcard query that matches abc* or *.
// This does not apply to phrases; it is a special case of the original Lucene
// parser.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.PrefixWildcardQueryNode.
type PrefixWildcardQueryNode struct {
	*WildcardQueryNode
}

// NewPrefixWildcardQueryNode creates a PrefixWildcardQueryNode.
//
// Mirrors PrefixWildcardQueryNode(CharSequence field, CharSequence text, int begin, int end).
func NewPrefixWildcardQueryNode(field, text string, begin, end int) *PrefixWildcardQueryNode {
	return &PrefixWildcardQueryNode{WildcardQueryNode: NewWildcardQueryNode(field, text, begin, end)}
}

// NewPrefixWildcardQueryNodeFromFieldQueryNode creates a
// PrefixWildcardQueryNode from an existing FieldQueryNode.
//
// Mirrors PrefixWildcardQueryNode(FieldQueryNode fqn).
func NewPrefixWildcardQueryNodeFromFieldQueryNode(fqn *FieldQueryNode) *PrefixWildcardQueryNode {
	return NewPrefixWildcardQueryNode(fqn.GetField(), fqn.GetText(), fqn.GetBegin(), fqn.GetEnd())
}

// CloneTree deep-copies this node.
//
// Mirrors PrefixWildcardQueryNode#cloneTree(), which adds nothing to the
// superclass clone.
func (n *PrefixWildcardQueryNode) CloneTree() QueryNode {
	return NewPrefixWildcardQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd())
}

// String returns the pseudo-XML debug form.
//
// Mirrors PrefixWildcardQueryNode#toString().
func (n *PrefixWildcardQueryNode) String() string {
	return "<prefixWildcard field='" + n.GetField() + "' term='" + n.GetText() + "'/>"
}
