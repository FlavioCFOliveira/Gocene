// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// NoTokenFoundQueryNode is used if a term is converted into no tokens by the
// tokenizer/lemmatizer/analyzer. This is the Go equivalent of Lucene's NoTokenFoundQueryNode.
type NoTokenFoundQueryNode struct {
	*DeletedQueryNode
}

// NewNoTokenFoundQueryNode creates a new NoTokenFoundQueryNode.
func NewNoTokenFoundQueryNode() *NoTokenFoundQueryNode {
	return &NoTokenFoundQueryNode{DeletedQueryNode: NewDeletedQueryNode()}
}

// ToQueryString returns the no-token-found marker.
func (n *NoTokenFoundQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	return "[NTF]"
}

// String returns a debug representation.
func (n *NoTokenFoundQueryNode) String() string {
	return "<notokenfound/>"
}

// CloneTree deep-copies this node.
func (n *NoTokenFoundQueryNode) CloneTree() QueryNode {
	cloned := &NoTokenFoundQueryNode{DeletedQueryNode: NewDeletedQueryNode()}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}
