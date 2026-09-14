// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strings"
)

// PathQueryNodeText contains the text, begin position and end position in the query.
type PathQueryNodeText struct {
	Value string
	Begin int
	End   int
}

// PathQueryNode is used to store queries like /company/USA/California.
// This is the Go equivalent of Lucene's PathQueryNode.
type PathQueryNode struct {
	*QueryNodeImpl
	values []PathQueryNodeText
}

// NewPathQueryNode creates a new PathQueryNode from a slice of path elements.
func NewPathQueryNode(pathElements []PathQueryNodeText) *PathQueryNode {
	if len(pathElements) < 2 {
		// In Lucene this is a RuntimeException
		panic("PathQueryNode requires 2 or more path elements.")
	}
	elems := make([]PathQueryNodeText, len(pathElements))
	copy(elems, pathElements)
	n := &PathQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		values:        elems,
	}
	// PathQueryNode is a leaf in Lucene: it never calls setLeaf(false).
	n.SetLeaf(true)
	return n
}

// GetPathElements returns the list of path elements.
func (n *PathQueryNode) GetPathElements() []PathQueryNodeText {
	out := make([]PathQueryNodeText, len(n.values))
	copy(out, n.values)
	return out
}

// SetPathElements sets the path elements.
func (n *PathQueryNode) SetPathElements(elements []PathQueryNodeText) {
	n.values = elements
}

// GetPathElement returns a specific path element by index.
func (n *PathQueryNode) GetPathElement(index int) PathQueryNodeText {
	return n.values[index]
}

// GetFirstPathElement returns the value of the first path element.
func (n *PathQueryNode) GetFirstPathElement() string {
	return n.values[0].Value
}

// GetPathElementsFrom returns a slice of path elements starting from startIndex.
func (n *PathQueryNode) GetPathElementsFrom(startIndex int) []PathQueryNodeText {
	if startIndex >= len(n.values) {
		return nil
	}
	res := make([]PathQueryNodeText, len(n.values)-startIndex)
	copy(res, n.values[startIndex:])
	return res
}

func (n *PathQueryNode) getPathString() string {
	var sb strings.Builder
	for _, v := range n.values {
		sb.WriteString("/")
		sb.WriteString(v.Value)
	}
	return sb.String()
}

// ToQueryString returns the path joined by '/' with quoted elements except the first.
func (n *PathQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	var sb strings.Builder
	sb.WriteString("/")
	sb.WriteString(n.GetFirstPathElement())

	for _, v := range n.GetPathElementsFrom(1) {
		val := escapeSyntax.Escape(v.Value, "en", EscapeString)
		sb.WriteString("/\"")
		sb.WriteString(val)
		sb.WriteString("\"")
	}
	return sb.String()
}

// String returns a debug representation.
func (n *PathQueryNode) String() string {
	text := n.values[0]
	return fmt.Sprintf("<path start=%d end=%d path=%s>", text.Begin, text.End, n.getPathString())
}

// CloneTree deep-copies this node.
func (n *PathQueryNode) CloneTree() QueryNode {
	cloned := &PathQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		values:        make([]PathQueryNodeText, len(n.values)),
	}
	copy(cloned.values, n.values)
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}
