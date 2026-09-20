// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// BooleanModifierNode has the same behaviour as ModifierQueryNode; it only
// indicates that the modifier was added by BooleanQuery2ModifierNodeProcessor
// and not by the user.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.BooleanModifierNode,
// whose whole body is the sole constructor delegating to ModifierQueryNode.
type BooleanModifierNode struct {
	*ModifierQueryNode
}

// NewBooleanModifierNode is the sole constructor.
//
// Mirrors BooleanModifierNode(QueryNode node, Modifier mod).
func NewBooleanModifierNode(node QueryNode, mod Modifier) *BooleanModifierNode {
	return &BooleanModifierNode{ModifierQueryNode: NewModifierQueryNode(node, mod)}
}

// CloneTree deep-copies this node.
func (n *BooleanModifierNode) CloneTree() QueryNode {
	var child QueryNode
	if children := n.GetChildren(); len(children) > 0 {
		child = children[0].CloneTree()
	}
	return NewBooleanModifierNode(child, n.GetModifier())
}
