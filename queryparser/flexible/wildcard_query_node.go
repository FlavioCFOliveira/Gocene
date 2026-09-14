// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// WildcardQueryNode represents a wildcard query. This does not apply to
// phrases. Examples: a*b*c Fl?w? m?ke*g.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.WildcardQueryNode.
type WildcardQueryNode struct {
	*FieldQueryNode
}

// NewWildcardQueryNode creates a WildcardQueryNode.
//
// Mirrors WildcardQueryNode(CharSequence field, CharSequence text, int begin, int end).
func NewWildcardQueryNode(field, text string, begin, end int) *WildcardQueryNode {
	return &WildcardQueryNode{FieldQueryNode: NewFieldQueryNode(field, text, begin, end)}
}

// NewWildcardQueryNodeFromFieldQueryNode creates a WildcardQueryNode from an
// existing FieldQueryNode.
//
// Mirrors WildcardQueryNode(FieldQueryNode fqn).
func NewWildcardQueryNodeFromFieldQueryNode(fqn *FieldQueryNode) *WildcardQueryNode {
	return NewWildcardQueryNode(fqn.GetField(), fqn.GetText(), fqn.GetBegin(), fqn.GetEnd())
}

// ToQueryString renders the wildcard term verbatim — Lucene performs no
// escaping here, because the wildcard characters must survive.
//
// Mirrors WildcardQueryNode#toQueryString(EscapeQuerySyntax).
func (n *WildcardQueryNode) ToQueryString(escaper EscapeQuerySyntax) string {
	if n.IsDefaultField(n.GetField()) {
		return n.GetText()
	}
	return n.GetField() + ":" + n.GetText()
}

// CloneTree deep-copies this node.
//
// Mirrors WildcardQueryNode#cloneTree(), which adds nothing to the superclass
// clone.
func (n *WildcardQueryNode) CloneTree() QueryNode {
	return NewWildcardQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd())
}

// String returns the pseudo-XML debug form.
//
// Mirrors WildcardQueryNode#toString().
func (n *WildcardQueryNode) String() string {
	return "<wildcard field='" + n.GetField() + "' term='" + n.GetText() + "'/>"
}
