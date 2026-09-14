// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "fmt"

// PointQueryNode represents a field query that holds a point value. It is
// similar to FieldQueryNode, however GetValue returns the point value rather
// than its text.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.PointQueryNode.
//
// Divergence, pre-existing and carried forward unchanged: Lucene stores the
// bound as a java.lang.Number together with the java.text.NumberFormat used to
// render it. Gocene has no NumberFormat port, so the value is held as the raw
// encoded bytes and rendered as their string form.
type PointQueryNode struct {
	*QueryNodeImpl
	field string
	value []byte
}

// NewPointQueryNode creates a PointQueryNode for the given field and value.
//
// Mirrors the Java constructor PointQueryNode(CharSequence, Number, NumberFormat).
func NewPointQueryNode(field string, value []byte) *PointQueryNode {
	n := &PointQueryNode{QueryNodeImpl: NewQueryNodeImpl(nil)}
	// PointQueryNode extends QueryNodeImpl without calling setLeaf(false).
	n.SetLeaf(true)
	n.SetField(field)
	n.value = make([]byte, len(value))
	copy(n.value, value)
	return n
}

// GetField returns the field associated with this node.
//
// Mirrors PointQueryNode#getField().
func (n *PointQueryNode) GetField() string { return n.field }

// SetField sets the field associated with this node.
//
// Mirrors PointQueryNode#setField(CharSequence).
func (n *PointQueryNode) SetField(fieldName string) { n.field = fieldName }

// GetValue returns the point value.
//
// Mirrors PointQueryNode#getValue().
func (n *PointQueryNode) GetValue() interface{} { return n.GetPointValue() }

// SetValue sets the point value.
//
// Mirrors PointQueryNode#setValue(Number).
func (n *PointQueryNode) SetValue(value interface{}) {
	b, ok := value.([]byte)
	if !ok {
		b = []byte(fmt.Sprintf("%v", value))
	}
	n.value = make([]byte, len(b))
	copy(n.value, b)
}

// GetPointValue returns a copy of the raw encoded point value.
func (n *PointQueryNode) GetPointValue() []byte {
	out := make([]byte, len(n.value))
	copy(out, n.value)
	return out
}

// getTermEscaped returns the value rendered and escaped with the supplied
// EscapeQuerySyntax.
//
// Mirrors the protected PointQueryNode#getTermEscaped(EscapeQuerySyntax),
// which escapes with Locale.ROOT and Type.NORMAL.
func (n *PointQueryNode) getTermEscaped(escaper EscapeQuerySyntax) string {
	return escaper.Escape(string(n.value), "", EscapeNormal)
}

// ToQueryString renders the node, omitting the field prefix when the field is
// the default one.
//
// Mirrors PointQueryNode#toQueryString(EscapeQuerySyntax).
func (n *PointQueryNode) ToQueryString(escapeSyntaxParser EscapeQuerySyntax) string {
	if n.IsDefaultField(n.field) {
		return n.getTermEscaped(escapeSyntaxParser)
	}
	return n.field + ":" + n.getTermEscaped(escapeSyntaxParser)
}

// CloneTree deep-copies this node.
func (n *PointQueryNode) CloneTree() QueryNode {
	return NewPointQueryNode(n.field, n.value)
}

// String returns the pseudo-XML debug form.
//
// Mirrors PointQueryNode#toString().
func (n *PointQueryNode) String() string {
	return "<numeric field='" + n.field + "' number='" + string(n.value) + "'/>"
}
