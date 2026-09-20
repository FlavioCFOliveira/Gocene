// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strings"
)

// AnyQueryNode represents an ANY operator performed on a list of nodes.
//
// Ported from org.apache.lucene.queryparser.flexible.core.nodes.AnyQueryNode
// (Apache Lucene 10.5.0). Java extends AndQueryNode.
type AnyQueryNode struct {
	*AndQueryNode
	field                   string
	minimumMatchingElements int
}

// NewAnyQueryNode creates a new AnyQueryNode over the given clauses.
//
// Mirrors AnyQueryNode(List<QueryNode> clauses, CharSequence field, int
// minimumMatchingElements).
func NewAnyQueryNode(clauses []QueryNode, field string, minimumMatchingElements int) *AnyQueryNode {
	anyNode := &AnyQueryNode{
		AndQueryNode:            NewAndQueryNode(clauses),
		field:                   field,
		minimumMatchingElements: minimumMatchingElements,
	}

	if clauses != nil {
		for _, clause := range clauses {
			if fq, ok := clause.(*FieldQueryNode); ok {
				fq.SetField(field)
			}
		}
	}

	return anyNode
}

// GetMinimumMatchingElements returns the minimum matching elements.
func (n *AnyQueryNode) GetMinimumMatchingElements() int {
	return n.minimumMatchingElements
}

// GetField returns the field, or the empty string if the field is unspecified.
func (n *AnyQueryNode) GetField() string {
	return n.field
}

// GetFieldAsString returns the field as a string, or the empty string if the
// field is unspecified. Mirrors AnyQueryNode.getFieldAsString().
func (n *AnyQueryNode) GetFieldAsString() string {
	return n.field
}

// SetField sets the field.
func (n *AnyQueryNode) SetField(field string) {
	n.field = field
}

// ToQueryString returns the query string representation.
func (n *AnyQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	anySTR := fmt.Sprintf("ANY %d", n.minimumMatchingElements)

	var sb strings.Builder
	children := n.GetChildren()
	for i, child := range children {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(child.ToQueryString(escapeSyntax))
	}

	res := sb.String()
	if isDefaultField(n.field) {
		return fmt.Sprintf("( %s ) %s", res, anySTR)
	}
	return fmt.Sprintf("%s:(( %s ) %s)", n.field, res, anySTR)
}

// String returns a string representation of this node.
func (n *AnyQueryNode) String() string {
	children := n.GetChildren()
	if len(children) == 0 {
		return fmt.Sprintf("<any field='%s'  matchelements=%d/>", n.field, n.minimumMatchingElements)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<any field='%s'  matchelements=%d>", n.field, n.minimumMatchingElements))
	for _, child := range children {
		sb.WriteString("\n")
		sb.WriteString(child.String())
	}
	sb.WriteString("\n</any>")
	return sb.String()
}

// CloneTree creates a deep copy of this node.
func (n *AnyQueryNode) CloneTree() QueryNode {
	cloned := &AnyQueryNode{
		AndQueryNode:            NewAndQueryNode(nil),
		field:                   n.field,
		minimumMatchingElements: n.minimumMatchingElements,
	}

	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}

	return cloned
}
