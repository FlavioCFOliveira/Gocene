// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// SynonymQueryNode is a QueryNode for clauses that are synonyms of each other.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.SynonymQueryNode,
// whose whole body is the sole constructor delegating to BooleanQueryNode.
type SynonymQueryNode struct {
	*BooleanQueryNode
}

// NewSynonymQueryNode is the sole constructor.
//
// Mirrors SynonymQueryNode(List<QueryNode> clauses).
func NewSynonymQueryNode(clauses []QueryNode) *SynonymQueryNode {
	return &SynonymQueryNode{BooleanQueryNode: NewBooleanQueryNode("", clauses)}
}

// CloneTree deep-copies this node.
func (n *SynonymQueryNode) CloneTree() QueryNode {
	children := n.GetChildren()
	cloned := make([]QueryNode, 0, len(children))
	for _, child := range children {
		cloned = append(cloned, child.CloneTree())
	}
	return NewSynonymQueryNode(cloned)
}
