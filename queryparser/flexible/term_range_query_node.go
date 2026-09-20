// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// TermRangeQueryNode represents a range query composed by FieldQueryNode
// bounds, which means the bound values are strings.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.TermRangeQueryNode.
type TermRangeQueryNode struct {
	*AbstractRangeQueryNode
}

// NewTermRangeQueryNode constructs a TermRangeQueryNode using the given
// FieldQueryNode bounds.
//
// Mirrors the Java constructor, whose whole body is
// setBounds(lower, upper, lowerInclusive, upperInclusive).
func NewTermRangeQueryNode(lower, upper *FieldQueryNode, lowerInclusive, upperInclusive bool) *TermRangeQueryNode {
	n := &TermRangeQueryNode{AbstractRangeQueryNode: newAbstractRangeQueryNode()}
	var lowerBound, upperBound FieldValuePairQueryNode
	if lower != nil {
		lowerBound = lower
	}
	if upper != nil {
		upperBound = upper
	}
	n.SetBounds(lowerBound, upperBound, lowerInclusive, upperInclusive)
	return n
}

// GetLowerBoundField returns the lower bound as a FieldQueryNode, or nil.
//
// Java reaches the typed bound through the generic parameter
// AbstractRangeQueryNode<FieldQueryNode>#getLowerBound(); Go names the typed
// accessor separately because the base returns the interface type.
func (n *TermRangeQueryNode) GetLowerBoundField() *FieldQueryNode {
	bound, _ := n.GetLowerBound().(*FieldQueryNode)
	return bound
}

// GetUpperBoundField returns the upper bound as a FieldQueryNode, or nil.
func (n *TermRangeQueryNode) GetUpperBoundField() *FieldQueryNode {
	bound, _ := n.GetUpperBound().(*FieldQueryNode)
	return bound
}

// CloneTree deep-copies this node.
func (n *TermRangeQueryNode) CloneTree() QueryNode {
	var lower, upper *FieldQueryNode
	if b := n.GetLowerBoundField(); b != nil {
		lower, _ = b.CloneTree().(*FieldQueryNode)
	}
	if b := n.GetUpperBoundField(); b != nil {
		upper, _ = b.CloneTree().(*FieldQueryNode)
	}
	return NewTermRangeQueryNode(lower, upper, n.IsLowerInclusive(), n.IsUpperInclusive())
}

// String returns the pseudo-XML debug form inherited from
// AbstractRangeQueryNode#toString().
func (n *TermRangeQueryNode) String() string {
	return n.stringWithCanonicalName(
		"org.apache.lucene.queryparser.flexible.standard.nodes.TermRangeQueryNode")
}
