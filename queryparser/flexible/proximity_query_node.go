// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "fmt"

// ProximityType indicates whether a ProximityQueryNode is word- or sentence-based.
type ProximityType int

const (
	// ProximityWord requires terms to be within N words of each other.
	ProximityWord ProximityType = iota
	// ProximitySentence requires terms to be within the same sentence.
	ProximitySentence
)

// ProximityQueryNode represents a proximity query (WITHIN N WORDS / SENTENCE).
// This is the Go equivalent of Lucene's ProximityQueryNode.
type ProximityQueryNode struct {
	*FieldQueryNode
	distance      int
	proximityType ProximityType
}

// NewProximityQueryNode creates a new ProximityQueryNode.
func NewProximityQueryNode(field, text string, distance int, proximityType ProximityType, begin, end int) *ProximityQueryNode {
	return &ProximityQueryNode{
		FieldQueryNode: NewFieldQueryNode(field, text, begin, end),
		distance:       distance,
		proximityType:  proximityType,
	}
}

// GetDistance returns the proximity distance.
func (n *ProximityQueryNode) GetDistance() int { return n.distance }

// GetProximityType returns the proximity type.
func (n *ProximityQueryNode) GetProximityType() ProximityType { return n.proximityType }

// ToQueryString returns a human-readable proximity expression.
func (n *ProximityQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	var typeName string
	switch n.proximityType {
	case ProximitySentence:
		typeName = "SENTENCE"
	default:
		typeName = "WORD"
	}
	text := n.GetText()
	text = escapeSyntax.Escape(text, "en", EscapeNormal)
	return fmt.Sprintf("%s:%s~%s/%d", n.GetField(), text, typeName, n.distance)
}

// String returns a debug representation.
func (n *ProximityQueryNode) String() string {
	return fmt.Sprintf("<proximity field=%s text=%s distance=%d>", n.GetField(), n.GetText(), n.distance)
}

// CloneTree deep-copies this node.
func (n *ProximityQueryNode) CloneTree() QueryNode {
	cloned := &ProximityQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
		distance:       n.distance,
		proximityType:  n.proximityType,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}
