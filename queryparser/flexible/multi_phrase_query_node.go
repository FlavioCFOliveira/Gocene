// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"strings"
)

// MultiPhraseQueryNode indicates that its children should be used to build a MultiPhraseQuery.
// This is the Go equivalent of Lucene's MultiPhraseQueryNode.
type MultiPhraseQueryNode struct {
	*QueryNodeImpl
}

// NewMultiPhraseQueryNode creates a new MultiPhraseQueryNode.
func NewMultiPhraseQueryNode() *MultiPhraseQueryNode {
	node := &MultiPhraseQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
	}
	node.SetLeaf(false)
	return node
}

// GetField returns the field of the first child if it is fieldable.
func (n *MultiPhraseQueryNode) GetField() string {
	children := n.GetChildren()
	if len(children) == 0 {
		return ""
	}
	if fieldable, ok := children[0].(FieldableNode); ok {
		return fieldable.GetField()
	}
	return ""
}

// SetField sets the field for all fieldable children.
func (n *MultiPhraseQueryNode) SetField(field string) {
	for _, child := range n.GetChildren() {
		if fieldable, ok := child.(FieldableNode); ok {
			fieldable.SetField(field)
		}
	}
}

// ToQueryString returns a human-readable representation of the multi-phrase query.
func (n *MultiPhraseQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	children := n.GetChildren()
	if len(children) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, child := range children {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(child.ToQueryString(escapeSyntax))
	}
	return "[MTP[" + sb.String() + "]]"
}

// String returns a debug representation.
func (n *MultiPhraseQueryNode) String() string {
	children := n.GetChildren()
	if len(children) == 0 {
		return "<multiPhrase/>"
	}
	var sb strings.Builder
	sb.WriteString("<multiPhrase>")
	for _, child := range children {
		sb.WriteString("\n")
		sb.WriteString(child.String())
	}
	sb.WriteString("\n</multiPhrase>")
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *MultiPhraseQueryNode) CloneTree() QueryNode {
	cloned := &MultiPhraseQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}
	return cloned
}
