// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

// NodeTestMockCell is a simple mock implementation of Cell for testing
type NodeTestMockCell struct {
	token  string
	level  int
	bbox   *Rectangle
	isLeaf bool
}

func (m *NodeTestMockCell) GetToken() string                 { return m.token }
func (m *NodeTestMockCell) GetLevel() int                    { return m.level }
func (m *NodeTestMockCell) GetShape() Shape                 { return m.bbox }
func (m *NodeTestMockCell) IsLeaf() bool                    { return m.isLeaf }
func (m *NodeTestMockCell) GetBoundingBox() *Rectangle      { return m.bbox }
func (m *NodeTestMockCell) IntersectsShape(shape Shape) bool { return true }

func TestNewNode(t *testing.T) {
	cell := &NodeTestMockCell{token: "abc", level: 3, bbox: NewRectangle(0, 0, 1, 1), isLeaf: false}

	tests := []struct {
		name     string
		cell     Cell
		parent   *Node
		wantNil  bool
		wantLevel int
	}{
		{
			name:     "valid node with cell",
			cell:     cell,
			parent:   nil,
			wantNil:  false,
			wantLevel: 3,
		},
		{
			name:     "nil cell returns nil",
			cell:     nil,
			parent:   nil,
			wantNil:  true,
			wantLevel: 0,
		},
		{
			name:     "node with parent",
			cell:     &NodeTestMockCell{token: "abcd", level: 4, bbox: NewRectangle(0, 0, 1, 1)},
			parent:   NewNode(cell, nil),
			wantNil:  false,
			wantLevel: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := NewNode(tt.cell, tt.parent)
			if tt.wantNil {
				if node != nil {
					t.Error("expected nil node")
				}
				return
			}
			if node == nil {
				t.Fatal("expected non-nil node")
			}
			if node.GetCell() != tt.cell {
				t.Error("cell mismatch")
			}
			if node.GetParent() != tt.parent {
				t.Error("parent mismatch")
			}
		})
	}
}

func TestNewRootNode(t *testing.T) {
	cell := &NodeTestMockCell{token: "root", level: 0, bbox: NewRectangle(-180, -90, 180, 90), isLeaf: false}

	node := NewRootNode(cell)
	if node == nil {
		t.Fatal("expected non-nil root node")
	}

	if node.GetParent() != nil {
		t.Error("root node should have no parent")
	}

	if node.GetLevel() != 0 {
		t.Errorf("expected level 0, got %d", node.GetLevel())
	}

	if node.GetToken() != "root" {
		t.Errorf("expected token 'root', got %s", node.GetToken())
	}

	if !node.IsRoot() {
		t.Error("root node should return true for IsRoot()")
	}
}

func TestNode_AddChild(t *testing.T) {
	parentCell := &NodeTestMockCell{token: "p", level: 1, bbox: NewRectangle(0, 0, 10, 10)}
	parent := NewNode(parentCell, nil)

	childCell := &NodeTestMockCell{token: "pc", level: 2, bbox: NewRectangle(0, 0, 5, 5)}
	child := NewNode(childCell, nil)

	// Add child
	err := parent.AddChild(child)
	if err != nil {
		t.Fatalf("unexpected error adding child: %v", err)
	}

	// Check child was added
	if parent.GetChildCount() != 1 {
		t.Errorf("expected 1 child, got %d", parent.GetChildCount())
	}

	// Check child's parent is set
	if child.GetParent() != parent {
		t.Error("child's parent should be set to parent")
	}

	// Try adding nil child
	err = parent.AddChild(nil)
	if err == nil {
		t.Error("expected error when adding nil child")
	}
}

func TestNode_GetChild(t *testing.T) {
	parentCell := &NodeTestMockCell{token: "p", level: 1, bbox: NewRectangle(0, 0, 10, 10)}
	parent := NewRootNode(parentCell)

	childCell := &NodeTestMockCell{token: "pa", level: 2, bbox: NewRectangle(0, 0, 5, 5)}
	child := NewNode(childCell, parent)

	// Manually add child for testing
	parent.children["a"] = child
	child.parent = parent

	// Get existing child
	found := parent.GetChild("a")
	if found != child {
		t.Error("GetChild should return the child")
	}

	// Get non-existing child
	notFound := parent.GetChild("z")
	if notFound != nil {
		t.Error("GetChild should return nil for non-existing child")
	}
}

func TestNode_RemoveChild(t *testing.T) {
	parentCell := &NodeTestMockCell{token: "p", level: 1, bbox: NewRectangle(0, 0, 10, 10)}
	parent := NewRootNode(parentCell)

	childCell := &NodeTestMockCell{token: "pa", level: 2, bbox: NewRectangle(0, 0, 5, 5)}
	child := NewNode(childCell, parent)
	parent.children["a"] = child
	child.parent = parent

	// Remove existing child
	removed := parent.RemoveChild("a")
	if !removed {
		t.Error("RemoveChild should return true for existing child")
	}

	if child.GetParent() != nil {
		t.Error("removed child's parent should be nil")
	}

	// Remove non-existing child
	removed = parent.RemoveChild("z")
	if removed {
		t.Error("RemoveChild should return false for non-existing child")
	}
}

func TestNode_GetChildren(t *testing.T) {
	parentCell := &NodeTestMockCell{token: "p", level: 1, bbox: NewRectangle(0, 0, 10, 10)}
	parent := NewRootNode(parentCell)

	// Initially no children
	children := parent.GetChildren()
	if len(children) != 0 {
		t.Errorf("expected 0 children, got %d", len(children))
	}

	// Add some children
	for i := 0; i < 3; i++ {
		cell := &NodeTestMockCell{token: string('a' + byte(i)), level: 2}
		child := NewNode(cell, parent)
		parent.children[string('a'+byte(i))] = child
	}

	children = parent.GetChildren()
	if len(children) != 3 {
		t.Errorf("expected 3 children, got %d", len(children))
	}
}

func TestNode_HasChildren(t *testing.T) {
	parentCell := &NodeTestMockCell{token: "p", level: 1, bbox: NewRectangle(0, 0, 10, 10)}
	parent := NewRootNode(parentCell)

	if parent.HasChildren() {
		t.Error("new node should not have children")
	}

	childCell := &NodeTestMockCell{token: "pa", level: 2}
	child := NewNode(childCell, parent)
	parent.children["a"] = child

	if !parent.HasChildren() {
		t.Error("node with children should return true")
	}
}

func TestNode_IsLeaf(t *testing.T) {
	leafCell := &NodeTestMockCell{token: "leaf", level: 5, isLeaf: true}
	leafNode := NewNode(leafCell, nil)

	if !leafNode.IsLeaf() {
		t.Error("node with isLeaf=true should be leaf")
	}

	nonLeafCell := &NodeTestMockCell{token: "nonleaf", level: 2, isLeaf: false}
	nonLeafNode := NewNode(nonLeafCell, nil)

	if nonLeafNode.IsLeaf() {
		t.Error("node with isLeaf=false should not be leaf")
	}
}

func TestNode_GetPath(t *testing.T) {
	// Create a hierarchy: root -> a -> ab
	rootCell := &NodeTestMockCell{token: "", level: 0}
	root := NewRootNode(rootCell)

	aCell := &NodeTestMockCell{token: "a", level: 1}
	nodeA := NewNode(aCell, root)
	root.children["a"] = nodeA
	nodeA.parent = root

	abCell := &NodeTestMockCell{token: "ab", level: 2}
	nodeAB := NewNode(abCell, nodeA)
	nodeA.children["b"] = nodeAB
	nodeAB.parent = nodeA

	path := nodeAB.GetPath()
	if len(path) != 3 {
		t.Fatalf("expected path length 3, got %d", len(path))
	}

	if path[0] != "" || path[1] != "a" || path[2] != "ab" {
		t.Errorf("unexpected path: %v", path)
	}
}

func TestNode_IsDescendantOf(t *testing.T) {
	rootCell := &NodeTestMockCell{token: "", level: 0}
	root := NewRootNode(rootCell)

	childCell := &NodeTestMockCell{token: "a", level: 1}
	child := NewNode(childCell, root)
	root.children["a"] = child
	child.parent = root

	grandChildCell := &NodeTestMockCell{token: "ab", level: 2}
	grandChild := NewNode(grandChildCell, child)
	child.children["b"] = grandChild
	grandChild.parent = child

	// grandChild is descendant of child
	if !grandChild.IsDescendantOf(child) {
		t.Error("grandChild should be descendant of child")
	}

	// grandChild is descendant of root
	if !grandChild.IsDescendantOf(root) {
		t.Error("grandChild should be descendant of root")
	}

	// child is NOT descendant of grandChild
	if child.IsDescendantOf(grandChild) {
		t.Error("child should not be descendant of grandChild")
	}

	// root is NOT descendant of anyone
	if root.IsDescendantOf(child) {
		t.Error("root should not be descendant of child")
	}
}

func TestNode_IsAncestorOf(t *testing.T) {
	rootCell := &NodeTestMockCell{token: "", level: 0}
	root := NewRootNode(rootCell)

	childCell := &NodeTestMockCell{token: "a", level: 1}
	child := NewNode(childCell, root)
	root.children["a"] = child
	child.parent = root

	// root is ancestor of child
	if !root.IsAncestorOf(child) {
		t.Error("root should be ancestor of child")
	}

	// child is NOT ancestor of root
	if child.IsAncestorOf(root) {
		t.Error("child should not be ancestor of root")
	}
}

func TestNode_GetSiblings(t *testing.T) {
	rootCell := &NodeTestMockCell{token: "", level: 0}
	root := NewRootNode(rootCell)

	// Create two children
	child1Cell := &NodeTestMockCell{token: "a", level: 1}
	child1 := NewNode(child1Cell, root)
	root.children["a"] = child1
	child1.parent = root

	child2Cell := &NodeTestMockCell{token: "b", level: 1}
	child2 := NewNode(child2Cell, root)
	root.children["b"] = child2
	child2.parent = root

	siblings := child1.GetSiblings()
	if len(siblings) != 1 || siblings[0] != child2 {
		t.Errorf("expected 1 sibling (child2), got %v", siblings)
	}

	// Root has no siblings
	rootSiblings := root.GetSiblings()
	if len(rootSiblings) != 0 {
		t.Errorf("root should have no siblings, got %d", len(rootSiblings))
	}
}

func TestNode_Prune(t *testing.T) {
	rootCell := &NodeTestMockCell{token: "", level: 0}
	root := NewRootNode(rootCell)

	childCell := &NodeTestMockCell{token: "a", level: 1}
	child := NewNode(childCell, root)
	root.children["a"] = child
	child.parent = root

	if !root.HasChildren() {
		t.Fatal("root should have children before prune")
	}

	root.Prune()

	if root.HasChildren() {
		t.Error("root should have no children after prune")
	}

	if child.GetParent() != nil {
		t.Error("child's parent should be nil after prune")
	}
}

func TestNode_Traverse(t *testing.T) {
	rootCell := &NodeTestMockCell{token: "", level: 0}
	root := NewRootNode(rootCell)

	childCell := &NodeTestMockCell{token: "a", level: 1}
	child := NewNode(childCell, root)
	root.children["a"] = child
	child.parent = root

	grandChildCell := &NodeTestMockCell{token: "ab", level: 2}
	grandChild := NewNode(grandChildCell, child)
	child.children["b"] = grandChild
	grandChild.parent = child

	visited := 0
	root.Traverse(func(node *Node) bool {
		visited++
		return true
	})

	if visited != 3 {
		t.Errorf("expected to visit 3 nodes, visited %d", visited)
	}

	// Test early termination
	visited = 0
	root.Traverse(func(node *Node) bool {
		visited++
		return visited < 2 // Stop after 2 nodes
	})

	if visited != 2 {
		t.Errorf("expected to visit 2 nodes (early termination), visited %d", visited)
	}
}

func TestNode_Find(t *testing.T) {
	rootCell := &NodeTestMockCell{token: "", level: 0}
	root := NewRootNode(rootCell)

	childCell := &NodeTestMockCell{token: "a", level: 1}
	child := NewNode(childCell, root)
	root.children["a"] = child
	child.parent = root

	// Find existing node
	found := root.Find("a")
	if found != child {
		t.Error("Find should return the child node")
	}

	// Find root itself
	foundRoot := root.Find("")
	if foundRoot != root {
		t.Error("Find should return root when searching for root token")
	}

	// Find non-existing node
	notFound := root.Find("xyz")
	if notFound != nil {
		t.Error("Find should return nil for non-existing token")
	}
}

func TestNode_GetStats(t *testing.T) {
	rootCell := &NodeTestMockCell{token: "", level: 0}
	root := NewRootNode(rootCell)

	childCell := &NodeTestMockCell{token: "a", level: 1, isLeaf: false}
	child := NewNode(childCell, root)
	root.children["a"] = child
	child.parent = root

	leafCell := &NodeTestMockCell{token: "ab", level: 2, isLeaf: true}
	leaf := NewNode(leafCell, child)
	child.children["b"] = leaf
	leaf.parent = child

	stats := root.GetStats()

	if stats.NodeCount != 3 {
		t.Errorf("expected 3 nodes, got %d", stats.NodeCount)
	}

	if stats.LeafCount != 1 {
		t.Errorf("expected 1 leaf, got %d", stats.LeafCount)
	}

	if stats.MaxDepth != 2 {
		t.Errorf("expected max depth 2, got %d", stats.MaxDepth)
	}
}

func TestDefaultNodeFactory(t *testing.T) {
	factory := &DefaultNodeFactory{}

	cell := &NodeTestMockCell{token: "test", level: 1}

	// Create node
	node := factory.CreateNode(cell, nil)
	if node == nil {
		t.Fatal("expected non-nil node from factory")
	}

	// Create root node
	root := factory.CreateRootNode(cell)
	if root == nil {
		t.Fatal("expected non-nil root from factory")
	}

	if root.GetParent() != nil {
		t.Error("factory-created root should have no parent")
	}
}

func TestNode_String(t *testing.T) {
	cell := &NodeTestMockCell{token: "test", level: 2}
	node := NewNode(cell, nil)

	str := node.String()
	if str == "" {
		t.Error("String should not return empty")
	}

	// Should contain token and level
	expected := "Node(token=test, level=2"
	if len(str) < len(expected) || str[:len(expected)] != expected {
		t.Errorf("expected string to start with '%s', got '%s'", expected, str)
	}
}
