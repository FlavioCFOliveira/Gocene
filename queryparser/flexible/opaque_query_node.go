// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "fmt"

// OpaqueQueryNode is used for specify values that are not supposed to be parsed by the
// parser. This is the Go equivalent of Lucene's OpaqueQueryNode.
type OpaqueQueryNode struct {
	*QueryNodeImpl
	schema string
	value  string
}

// NewOpaqueQueryNode creates a new OpaqueQueryNode.
func NewOpaqueQueryNode(schema, value string) *OpaqueQueryNode {
	node := &OpaqueQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		schema:        schema,
		value:         value,
	}
	node.SetLeaf(true)
	return node
}

// GetSchema returns the schema identifier.
func (n *OpaqueQueryNode) GetSchema() string { return n.schema }

// GetValue returns the opaque value string.
func (n *OpaqueQueryNode) GetValue() string { return n.value }

// ToQueryString returns "@schema:'value'".
func (n *OpaqueQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	return "@" + n.schema + ":'" + n.value + "'"
}

// String returns a debug representation.
func (n *OpaqueQueryNode) String() string {
	return fmt.Sprintf("<opaque schema=%s value=%s>", n.schema, n.value)
}

// CloneTree deep-copies this node.
func (n *OpaqueQueryNode) CloneTree() QueryNode {
	cloned := &OpaqueQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		schema:        n.schema,
		value:         n.value,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}
