// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"sync"
)

// Node represents a node in the spatial prefix tree hierarchy.
// Each node corresponds to a cell in the spatial grid and maintains
// relationships with its parent and children nodes.
//
// The Node structure is used internally by prefix tree implementations
// to build and navigate the spatial index hierarchy efficiently.
//
// This is the Go port of Lucene's spatial Node concept.
type Node struct {
	// cell is the spatial cell associated with this node
	cell Cell

	// parent is the parent node (nil for root nodes)
	parent *Node

	// children are the child nodes indexed by their token suffix
	children map[string]*Node

	// level is the depth of this node in the tree (0 for root)
	level int

	// token is the full token path for this node
	token string

	// isLeaf indicates if this node is a leaf (at max detail level)
	isLeaf bool

	// mu protects concurrent access to children
	mu sync.RWMutex
}

// NewNode creates a new Node with the given cell and parent.
//
// Parameters:
//   - cell: The spatial cell for this node
//   - parent: The parent node (nil for root nodes)
//
// Returns a new Node instance.
func NewNode(cell Cell, parent *Node) *Node {
	if cell == nil {
		return nil
	}

	level := cell.GetLevel()
	if parent != nil {
		// Child level is parent level + 1
		level = parent.level + 1
	}

	return &Node{
		cell:     cell,
		parent:   parent,
		children: make(map[string]*Node),
		level:    level,
		token:    cell.GetToken(),
		isLeaf:   cell.IsLeaf(),
	}
}

// NewRootNode creates a new root node for a prefix tree.
//
// Parameters:
//   - cell: The root cell (typically representing the world bounds)
//
// Returns a new root Node instance.
func NewRootNode(cell Cell) *Node {
	return &Node{
		cell:     cell,
		parent:   nil,
		children: make(map[string]*Node),
		level:    0,
		token:    cell.GetToken(),
		isLeaf:   cell.IsLeaf(),
	}
}

// GetCell returns the spatial cell associated with this node.
func (n *Node) GetCell() Cell {
	return n.cell
}

// GetParent returns the parent node.
// Returns nil if this is a root node.
func (n *Node) GetParent() *Node {
	return n.parent
}

// GetChildren returns all child nodes.
// Returns a slice of child nodes (may be empty).
func (n *Node) GetChildren() []*Node {
	n.mu.RLock()
	defer n.mu.RUnlock()

	children := make([]*Node, 0, len(n.children))
	for _, child := range n.children {
		children = append(children, child)
	}
	return children
}

// GetChild returns a specific child node by its token suffix.
//
// Parameters:
//   - tokenSuffix: The token suffix identifying the child
//
// Returns the child node or nil if not found.
func (n *Node) GetChild(tokenSuffix string) *Node {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.children[tokenSuffix]
}

// AddChild adds a child node to this node.
//
// Parameters:
//   - child: The child node to add
//
// Returns an error if the child cannot be added.
func (n *Node) AddChild(child *Node) error {
	if child == nil {
		return fmt.Errorf("child node cannot be nil")
	}

	if child.parent != nil && child.parent != n {
		return fmt.Errorf("child already has a different parent")
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Extract the token suffix (last character of child's token)
	childToken := child.token
	if len(childToken) == 0 {
		return fmt.Errorf("child token cannot be empty")
	}

	// For non-root nodes, the suffix is the last character
	// For root's children, use the full token
	var suffix string
	if n.parent == nil {
		suffix = childToken
	} else {
		suffix = childToken[len(childToken)-1:]
	}

	n.children[suffix] = child
	child.parent = n

	return nil
}

// RemoveChild removes a child node by its token suffix.
//
// Parameters:
//   - tokenSuffix: The token suffix of the child to remove
//
// Returns true if a child was removed, false otherwise.
func (n *Node) RemoveChild(tokenSuffix string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if child, exists := n.children[tokenSuffix]; exists {
		child.parent = nil
		delete(n.children, tokenSuffix)
		return true
	}
	return false
}

// GetLevel returns the depth level of this node in the tree.
// Root nodes are at level 0.
func (n *Node) GetLevel() int {
	return n.level
}

// GetToken returns the full token path for this node.
func (n *Node) GetToken() string {
	return n.token
}

// IsLeaf returns true if this node is a leaf node.
func (n *Node) IsLeaf() bool {
	return n.isLeaf
}

// HasChildren returns true if this node has any children.
func (n *Node) HasChildren() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return len(n.children) > 0
}

// GetChildCount returns the number of children this node has.
func (n *Node) GetChildCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return len(n.children)
}

// GetPath returns the path from the root to this node as a slice of tokens.
// The first element is the root token, the last is this node's token.
func (n *Node) GetPath() []string {
	// Build path from root to this node
	path := make([]string, 0, n.level+1)

	// Traverse up to root
	var nodes []*Node
	current := n
	for current != nil {
		nodes = append([]*Node{current}, nodes...)
		current = current.parent
	}

	// Build token path
	for _, node := range nodes {
		path = append(path, node.token)
	}

	return path
}

// GetDepth returns the depth of this node from the root.
// This is equivalent to GetLevel().
func (n *Node) GetDepth() int {
	return n.level
}

// IsRoot returns true if this is a root node (has no parent).
func (n *Node) IsRoot() bool {
	return n.parent == nil
}

// IsDescendantOf checks if this node is a descendant of the given node.
//
// Parameters:
//   - ancestor: The potential ancestor node
//
// Returns true if this node is a descendant of the ancestor.
func (n *Node) IsDescendantOf(ancestor *Node) bool {
	if ancestor == nil || n == ancestor {
		return false
	}

	current := n.parent
	for current != nil {
		if current == ancestor {
			return true
		}
		current = current.parent
	}
	return false
}

// IsAncestorOf checks if this node is an ancestor of the given node.
//
// Parameters:
//   - descendant: The potential descendant node
//
// Returns true if this node is an ancestor of the descendant.
func (n *Node) IsAncestorOf(descendant *Node) bool {
	if descendant == nil || n == descendant {
		return false
	}
	return descendant.IsDescendantOf(n)
}

// GetSiblings returns all sibling nodes (nodes with the same parent).
// Returns an empty slice if this is a root node.
func (n *Node) GetSiblings() []*Node {
	if n.parent == nil {
		return []*Node{}
	}

	siblings := make([]*Node, 0)
	for _, child := range n.parent.GetChildren() {
		if child != n {
			siblings = append(siblings, child)
		}
	}
	return siblings
}

// GetSubtree returns all nodes in the subtree rooted at this node.
// Includes this node and all descendants.
//
// Parameters:
//   - maxDepth: Maximum depth to traverse (-1 for unlimited)
//
// Returns a slice of all nodes in the subtree.
func (n *Node) GetSubtree(maxDepth int) []*Node {
	result := []*Node{n}

	if maxDepth == 0 {
		return result
	}

	for _, child := range n.GetChildren() {
		childDepth := maxDepth
		if maxDepth > 0 {
			childDepth = maxDepth - 1
		}
		result = append(result, child.GetSubtree(childDepth)...)
	}

	return result
}

// Prune removes all children from this node.
func (n *Node) Prune() {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, child := range n.children {
		child.parent = nil
	}
	n.children = make(map[string]*Node)
}

// String returns a string representation of this node.
func (n *Node) String() string {
	return fmt.Sprintf("Node(token=%s, level=%d, children=%d)",
		n.token, n.level, n.GetChildCount())
}

// NodeVisitor is a function type for visiting nodes during traversal.
type NodeVisitor func(node *Node) bool

// Traverse performs a depth-first traversal of the subtree.
//
// Parameters:
//   - visitor: Function called for each node. Return false to stop traversal.
//
// Returns true if traversal completed, false if stopped by visitor.
func (n *Node) Traverse(visitor NodeVisitor) bool {
	if !visitor(n) {
		return false
	}

	for _, child := range n.GetChildren() {
		if !child.Traverse(visitor) {
			return false
		}
	}

	return true
}

// TraverseBreadth performs a breadth-first traversal of the subtree.
//
// Parameters:
//   - visitor: Function called for each node. Return false to stop traversal.
//
// Returns true if traversal completed, false if stopped by visitor.
func (n *Node) TraverseBreadth(visitor NodeVisitor) bool {
	queue := []*Node{n}

	for len(queue) > 0 {
		// Dequeue
		node := queue[0]
		queue = queue[1:]

		if !visitor(node) {
			return false
		}

		// Enqueue children
		queue = append(queue, node.GetChildren()...)
	}

	return true
}

// Find finds a node in the subtree by its token.
//
// Parameters:
//   - token: The token to search for
//
// Returns the found node or nil if not found.
func (n *Node) Find(token string) *Node {
	if n.token == token {
		return n
	}

	// Check if this token could be a descendant
	if len(token) <= len(n.token) || token[:len(n.token)] != n.token {
		return nil
	}

	// Search in children
	for _, child := range n.GetChildren() {
		if found := child.Find(token); found != nil {
			return found
		}
	}

	return nil
}

// NodeStats provides statistics about a node and its subtree.
type NodeStats struct {
	// NodeCount is the total number of nodes in the subtree
	NodeCount int

	// LeafCount is the number of leaf nodes
	LeafCount int

	// MaxDepth is the maximum depth from this node
	MaxDepth int
}

// GetStats returns statistics about this node and its subtree.
func (n *Node) GetStats() *NodeStats {
	stats := &NodeStats{}
	n.calculateStats(stats, 0)
	return stats
}

// calculateStats recursively calculates statistics.
func (n *Node) calculateStats(stats *NodeStats, currentDepth int) {
	stats.NodeCount++

	if n.IsLeaf() || !n.HasChildren() {
		stats.LeafCount++
	}

	if currentDepth > stats.MaxDepth {
		stats.MaxDepth = currentDepth
	}

	for _, child := range n.GetChildren() {
		child.calculateStats(stats, currentDepth+1)
	}
}

// NodeFactory creates nodes for a prefix tree.
// This is used by prefix tree implementations to create nodes with
// implementation-specific cell types.
type NodeFactory interface {
	// CreateNode creates a new node for the given cell and parent.
	CreateNode(cell Cell, parent *Node) *Node

	// CreateRootNode creates a new root node for the given cell.
	CreateRootNode(cell Cell) *Node
}

// DefaultNodeFactory is the default implementation of NodeFactory.
type DefaultNodeFactory struct{}

// CreateNode creates a new node using the default Node constructor.
func (f *DefaultNodeFactory) CreateNode(cell Cell, parent *Node) *Node {
	return NewNode(cell, parent)
}

// CreateRootNode creates a new root node using the default constructor.
func (f *DefaultNodeFactory) CreateRootNode(cell Cell) *Node {
	return NewRootNode(cell)
}

// Ensure DefaultNodeFactory implements NodeFactory
var _ NodeFactory = (*DefaultNodeFactory)(nil)
