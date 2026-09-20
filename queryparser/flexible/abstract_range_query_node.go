// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strings"
)

// AbstractRangeQueryNode should be extended by nodes intending to represent
// range queries.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.AbstractRangeQueryNode.
// Java parameterises the class on the bound type
// (<T extends FieldValuePairQueryNode<?>>); the Go port stores the bounds as
// plain QueryNode children — exactly as Java does — and each extender exposes
// its own typed accessors.
type AbstractRangeQueryNode struct {
	*QueryNodeImpl
	lowerInclusive bool
	upperInclusive bool
}

// newAbstractRangeQueryNode constructs an AbstractRangeQueryNode; it should be
// invoked only by its extenders.
//
// Mirrors the protected Java constructor, which runs setLeaf(false) and
// allocate().
func newAbstractRangeQueryNode() *AbstractRangeQueryNode {
	return &AbstractRangeQueryNode{QueryNodeImpl: NewQueryNodeImpl(nil)}
}

// GetField returns the field associated with this node, taken from the lower
// bound if present and otherwise from the upper bound.
//
// Mirrors AbstractRangeQueryNode#getField().
func (n *AbstractRangeQueryNode) GetField() string {
	field := ""

	lower := n.getBound(0)
	upper := n.getBound(1)

	if lower != nil {
		field = lower.GetField()
	} else if upper != nil {
		field = upper.GetField()
	}

	return field
}

// SetField sets the field associated with this node, propagating it to both
// bounds.
//
// Mirrors AbstractRangeQueryNode#setField(CharSequence).
func (n *AbstractRangeQueryNode) SetField(fieldName string) {
	lower := n.getBound(0)
	upper := n.getBound(1)

	if lower != nil {
		lower.SetField(fieldName)
	}

	if upper != nil {
		upper.SetField(fieldName)
	}
}

// getBound returns the bound stored at idx, or nil when this node holds no
// bounds. Java reads getChildren().get(idx) directly; setBounds only populates
// the children when both bounds are present, so the two forms agree.
func (n *AbstractRangeQueryNode) getBound(idx int) FieldValuePairQueryNode {
	children := n.GetChildren()
	if idx >= len(children) {
		return nil
	}
	bound, ok := children[idx].(FieldValuePairQueryNode)
	if !ok {
		return nil
	}
	return bound
}

// GetLowerBound returns the lower bound node.
//
// Mirrors AbstractRangeQueryNode#getLowerBound().
func (n *AbstractRangeQueryNode) GetLowerBound() FieldValuePairQueryNode { return n.getBound(0) }

// GetUpperBound returns the upper bound node.
//
// Mirrors AbstractRangeQueryNode#getUpperBound().
func (n *AbstractRangeQueryNode) GetUpperBound() FieldValuePairQueryNode { return n.getBound(1) }

// IsLowerInclusive reports whether the lower bound is inclusive.
//
// Mirrors AbstractRangeQueryNode#isLowerInclusive().
func (n *AbstractRangeQueryNode) IsLowerInclusive() bool { return n.lowerInclusive }

// IsUpperInclusive reports whether the upper bound is inclusive.
//
// Mirrors AbstractRangeQueryNode#isUpperInclusive().
func (n *AbstractRangeQueryNode) IsUpperInclusive() bool { return n.upperInclusive }

// SetBounds sets the lower and upper bounds.
//
// Mirrors AbstractRangeQueryNode#setBounds(T, T, boolean, boolean): nothing is
// stored unless both bounds are supplied, and the two bounds must carry the
// same field name.
func (n *AbstractRangeQueryNode) SetBounds(lower, upper FieldValuePairQueryNode, lowerInclusive, upperInclusive bool) {
	if lower != nil && upper != nil {
		lowerField := lower.GetField()
		upperField := upper.GetField()

		if upperField != lowerField {
			panic("lower and upper bounds should have the same field name!")
		}

		n.lowerInclusive = lowerInclusive
		n.upperInclusive = upperInclusive

		children := make([]QueryNode, 0, 2)
		children = append(children, lower)
		children = append(children, upper)

		n.SetChildren(children)
	}
}

// ToQueryString renders the range, without a field prefix, using "..." for an
// absent bound and a single space as the separator.
//
// Mirrors AbstractRangeQueryNode#toQueryString(EscapeQuerySyntax).
func (n *AbstractRangeQueryNode) ToQueryString(escapeSyntaxParser EscapeQuerySyntax) string {
	var sb strings.Builder

	lower := n.GetLowerBound()
	upper := n.GetUpperBound()

	if n.lowerInclusive {
		sb.WriteByte('[')
	} else {
		sb.WriteByte('{')
	}

	if lower != nil {
		sb.WriteString(lower.ToQueryString(escapeSyntaxParser))
	} else {
		sb.WriteString("...")
	}

	sb.WriteByte(' ')

	if upper != nil {
		sb.WriteString(upper.ToQueryString(escapeSyntaxParser))
	} else {
		sb.WriteString("...")
	}

	if n.upperInclusive {
		sb.WriteByte(']')
	} else {
		sb.WriteByte('}')
	}

	return sb.String()
}

// stringWithCanonicalName renders the pseudo-XML debug form of
// AbstractRangeQueryNode#toString(). Java interpolates
// getClass().getCanonicalName(); Go has no inherited concrete-class lookup, so
// each extender passes its own canonical name.
func (n *AbstractRangeQueryNode) stringWithCanonicalName(canonicalName string) string {
	var sb strings.Builder
	sb.WriteString("<")
	sb.WriteString(canonicalName)
	sb.WriteString(" lowerInclusive=")
	sb.WriteString(fmt.Sprintf("%t", n.IsLowerInclusive()))
	sb.WriteString(" upperInclusive=")
	sb.WriteString(fmt.Sprintf("%t", n.IsUpperInclusive()))
	sb.WriteString(">\n\t")
	sb.WriteString(fmt.Sprintf("%v", n.GetUpperBound()))
	sb.WriteString("\n\t")
	sb.WriteString(fmt.Sprintf("%v", n.GetLowerBound()))
	sb.WriteString("\n")
	sb.WriteString("</")
	sb.WriteString(canonicalName)
	sb.WriteString(">\n")

	return sb.String()
}
