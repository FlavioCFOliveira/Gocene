// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
)

// FuzzyQueryNode represents a element that contains field/text/similarity tuple.
// This is the Go equivalent of Lucene's FuzzyQueryNode.
type FuzzyQueryNode struct {
	*FieldQueryNode
	similarity   float32
	prefixLength int
}

// NewFuzzyQueryNode creates a new FuzzyQueryNode.
func NewFuzzyQueryNode(field, term string, minSimilarity float32, begin, end int) *FuzzyQueryNode {
	return &FuzzyQueryNode{
		FieldQueryNode: NewFieldQueryNode(field, term, begin, end),
		similarity:    minSimilarity,
	}
}

// GetSimilarity returns the similarity value.
func (n *FuzzyQueryNode) GetSimilarity() float32 {
	return n.similarity
}

// SetSimilarity sets the similarity value.
func (n *FuzzyQueryNode) SetSimilarity(similarity float32) {
	n.similarity = similarity
}

// GetPrefixLength returns the prefix length.
func (n *FuzzyQueryNode) GetPrefixLength() int {
	return n.prefixLength
}

// SetPrefixLength sets the prefix length.
func (n *FuzzyQueryNode) SetPrefixLength(prefixLength int) {
	n.prefixLength = prefixLength
}

// ToQueryString returns the fuzzy query representation.
func (n *FuzzyQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	termEscaped := escapeSyntax.Escape(n.GetText(), "en", EscapeNormal)
	if n.GetField() == "" {
		return fmt.Sprintf("%s~%g", termEscaped, n.similarity)
	}
	return fmt.Sprintf("%s:%s~%g", n.GetField(), termEscaped, n.similarity)
}

// String returns a debug representation.
func (n *FuzzyQueryNode) String() string {
	return fmt.Sprintf("<fuzzy field=%s similarity=%g term=%s>", n.GetField(), n.similarity, n.GetText())
}

// CloneTree deep-copies this node.
func (n *FuzzyQueryNode) CloneTree() QueryNode {
	cloned := &FuzzyQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
		similarity:    n.similarity,
		prefixLength:  n.prefixLength,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}
