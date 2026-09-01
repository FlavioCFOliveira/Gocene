// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strings"
)

// TokenizedPhraseQueryNode holds a sequence of already-analyzed tokens that
// form a phrase. This is the Go equivalent of Lucene's TokenizedPhraseQueryNode.
type TokenizedPhraseQueryNode struct {
	*QueryNodeImpl
	field string
}

// NewTokenizedPhraseQueryNode creates a new TokenizedPhraseQueryNode for field,
// with the provided token nodes as children.
func NewTokenizedPhraseQueryNode(field string, tokenNodes []QueryNode) *TokenizedPhraseQueryNode {
	return &TokenizedPhraseQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(tokenNodes),
		field:         field,
	}
}

// GetField returns the field this phrase applies to.
func (n *TokenizedPhraseQueryNode) GetField() string { return n.field }

// SetField sets the field name.
func (n *TokenizedPhraseQueryNode) SetField(field string) { n.field = field }

// ToQueryString emits [TP[token1,token2...]].
func (n *TokenizedPhraseQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
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
	return "[TP[" + sb.String() + "]]"
}

// String returns a debug representation.
func (n *TokenizedPhraseQueryNode) String() string {
	return fmt.Sprintf("<tokenized_phrase field=%s tokens=%d>", n.field, len(n.GetChildren()))
}

// CloneTree deep-copies this node.
func (n *TokenizedPhraseQueryNode) CloneTree() QueryNode {
	cloned := &TokenizedPhraseQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		field:         n.field,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}
	return cloned
}
