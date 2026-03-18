package flexible

import (
	"testing"
)

func TestQueryNodeImpl_New(t *testing.T) {
	tests := []struct {
		name     string
		children []QueryNode
		wantLen  int
	}{
		{
			name:     "empty children",
			children: nil,
			wantLen:  0,
		},
		{
			name:     "single child",
			children: []QueryNode{NewQueryNodeImpl(nil)},
			wantLen:  1,
		},
		{
			name:     "multiple children",
			children: []QueryNode{NewQueryNodeImpl(nil), NewQueryNodeImpl(nil)},
			wantLen:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := NewQueryNodeImpl(tt.children)
			if len(node.GetChildren()) != tt.wantLen {
				t.Errorf("NewQueryNodeImpl() children count = %v, want %v", len(node.GetChildren()), tt.wantLen)
			}
		})
	}
}

func TestQueryNodeImpl_Tags(t *testing.T) {
	node := NewQueryNodeImpl(nil)

	// Test SetTag and GetTag
	node.SetTag("key1", "value1")
	node.SetTag("key2", 123)

	if val := node.GetTag("key1"); val != "value1" {
		t.Errorf("GetTag(key1) = %v, want value1", val)
	}

	if val := node.GetTag("key2"); val != 123 {
		t.Errorf("GetTag(key2) = %v, want 123", val)
	}

	// Test non-existent tag
	if val := node.GetTag("nonexistent"); val != nil {
		t.Errorf("GetTag(nonexistent) = %v, want nil", val)
	}

	// Test HasTag
	if !node.HasTag("key1") {
		t.Error("HasTag(key1) should return true")
	}

	if node.HasTag("nonexistent") {
		t.Error("HasTag(nonexistent) should return false")
	}

	// Test GetTagString
	if val := node.GetTagString("key1"); val != "value1" {
		t.Errorf("GetTagString(key1) = %v, want value1", val)
	}

	// Test RemoveTag
	node.RemoveTag("key1")
	if node.HasTag("key1") {
		t.Error("HasTag(key1) should return false after RemoveTag")
	}

	// Test ClearTags
	node.ClearTags()
	if node.HasTag("key2") {
		t.Error("HasTag(key2) should return false after ClearTags")
	}
}

func TestQueryNodeImpl_Parent(t *testing.T) {
	parent := NewQueryNodeImpl(nil)
	child := NewQueryNodeImpl(nil)

	// Initially no parent
	if child.GetParent() != nil {
		t.Error("New child should have nil parent")
	}

	// Add child to parent
	parent.AddChild(child)

	if child.GetParent() != parent {
		t.Error("Child should have parent after AddChild")
	}

	// Remove child
	parent.RemoveChild(child)

	if child.GetParent() != nil {
		t.Error("Child should have nil parent after RemoveChild")
	}
}

func TestQueryNodeImpl_IsLeaf(t *testing.T) {
	leaf := NewQueryNodeImpl(nil)
	if !leaf.IsLeaf() {
		t.Error("Node with no children should be leaf")
	}

	parent := NewQueryNodeImpl([]QueryNode{leaf})
	if parent.IsLeaf() {
		t.Error("Node with children should not be leaf")
	}
}

func TestQueryNodeImpl_AddChild(t *testing.T) {
	parent := NewQueryNodeImpl(nil)
	child1 := NewQueryNodeImpl(nil)
	child2 := NewQueryNodeImpl(nil)

	// Add first child
	parent.AddChild(child1)
	if len(parent.GetChildren()) != 1 {
		t.Errorf("Expected 1 child, got %d", len(parent.GetChildren()))
	}

	// Add second child
	parent.AddChild(child2)
	if len(parent.GetChildren()) != 2 {
		t.Errorf("Expected 2 children, got %d", len(parent.GetChildren()))
	}

	// Add nil child (should be ignored)
	parent.AddChild(nil)
	if len(parent.GetChildren()) != 2 {
		t.Errorf("Expected 2 children after adding nil, got %d", len(parent.GetChildren()))
	}
}

func TestQueryNodeImpl_RemoveChild(t *testing.T) {
	parent := NewQueryNodeImpl(nil)
	child1 := NewQueryNodeImpl(nil)
	child2 := NewQueryNodeImpl(nil)

	parent.AddChild(child1)
	parent.AddChild(child2)

	// Remove existing child
	if !parent.RemoveChild(child1) {
		t.Error("RemoveChild should return true for existing child")
	}

	if len(parent.GetChildren()) != 1 {
		t.Errorf("Expected 1 child after removal, got %d", len(parent.GetChildren()))
	}

	// Remove non-existent child
	other := NewQueryNodeImpl(nil)
	if parent.RemoveChild(other) {
		t.Error("RemoveChild should return false for non-existent child")
	}
}

func TestQueryNodeImpl_ReplaceChild(t *testing.T) {
	parent := NewQueryNodeImpl(nil)
	oldChild := NewQueryNodeImpl(nil)
	newChild := NewQueryNodeImpl(nil)

	parent.AddChild(oldChild)

	// Replace existing child
	if !parent.ReplaceChild(oldChild, newChild) {
		t.Error("ReplaceChild should return true for existing child")
	}

	if len(parent.GetChildren()) != 1 {
		t.Errorf("Expected 1 child after replace, got %d", len(parent.GetChildren()))
	}

	if parent.GetChildren()[0] != newChild {
		t.Error("Parent should have new child after replace")
	}

	if newChild.GetParent() != parent {
		t.Error("New child should have parent after replace")
	}

	if oldChild.GetParent() != nil {
		t.Error("Old child should have nil parent after replace")
	}

	// Replace non-existent child
	other := NewQueryNodeImpl(nil)
	if parent.ReplaceChild(other, oldChild) {
		t.Error("ReplaceChild should return false for non-existent child")
	}
}

func TestQueryNodeImpl_SetChildren(t *testing.T) {
	parent := NewQueryNodeImpl(nil)
	child1 := NewQueryNodeImpl(nil)
	child2 := NewQueryNodeImpl(nil)

	// Set children
	parent.SetChildren([]QueryNode{child1, child2})

	if len(parent.GetChildren()) != 2 {
		t.Errorf("Expected 2 children, got %d", len(parent.GetChildren()))
	}

	// Replace with new children
	child3 := NewQueryNodeImpl(nil)
	parent.SetChildren([]QueryNode{child3})

	if len(parent.GetChildren()) != 1 {
		t.Errorf("Expected 1 child after SetChildren, got %d", len(parent.GetChildren()))
	}

	if child1.GetParent() != nil {
		t.Error("Old child should have nil parent after SetChildren")
	}

	if child3.GetParent() != parent {
		t.Error("New child should have parent after SetChildren")
	}
}

func TestQueryNodeImpl_CloneTree(t *testing.T) {
	// Create a tree: parent -> [child1 -> [grandchild], child2]
	grandchild := NewQueryNodeImpl(nil)
	grandchild.SetTag("level", "grandchild")

	child1 := NewQueryNodeImpl([]QueryNode{grandchild})
	child1.SetTag("level", "child1")

	child2 := NewQueryNodeImpl(nil)
	child2.SetTag("level", "child2")

	parent := NewQueryNodeImpl([]QueryNode{child1, child2})
	parent.SetTag("level", "parent")

	// Clone the tree
	cloned := parent.CloneTree()
	clonedImpl := cloned.(*QueryNodeImpl)

	// Verify structure
	if len(clonedImpl.GetChildren()) != 2 {
		t.Errorf("Expected 2 children in clone, got %d", len(clonedImpl.GetChildren()))
	}

	// Verify tags are copied
	if clonedImpl.GetTag("level") != "parent" {
		t.Error("Clone should have copied tags")
	}

	// Verify it's a deep copy (different objects)
	if cloned == parent {
		t.Error("Clone should be a different object")
	}

	// Verify children are also cloned
	clonedChild1 := clonedImpl.GetChildren()[0]
	if clonedChild1 == child1 {
		t.Error("Cloned child should be a different object")
	}

	// Verify parent references are set correctly in clone
	if clonedChild1.GetParent() != cloned {
		t.Error("Cloned child should have cloned parent as parent")
	}
}

func TestQueryNodeImpl_String(t *testing.T) {
	node := NewQueryNodeImpl(nil)
	// Base implementation returns empty string for leaf
	if s := node.String(); s != "" {
		t.Errorf("String() = %q, want empty string", s)
	}
}

func TestQueryNodeImpl_ToQueryString(t *testing.T) {
	// Create a simple tree
	child1 := NewQueryNodeImpl(nil)
	child2 := NewQueryNodeImpl(nil)
	parent := NewQueryNodeImpl([]QueryNode{child1, child2})

	// Base implementation returns space-separated children
	result := parent.ToQueryString(false)
	if result != " " {
		t.Errorf("ToQueryString() = %q, want space", result)
	}
}

func TestQueryNodeImpl_GetTagKeys(t *testing.T) {
	node := NewQueryNodeImpl(nil)

	// Empty tags
	keys := node.GetTagKeys()
	if keys != nil {
		t.Error("GetTagKeys should return nil for empty tags")
	}

	// With tags
	node.SetTag("key1", "value1")
	node.SetTag("key2", "value2")

	keys = node.GetTagKeys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	// Verify keys exist (order not guaranteed)
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}
	if !keyMap["key1"] || !keyMap["key2"] {
		t.Error("GetTagKeys should return all tag keys")
	}
}

func TestQueryNodeImpl_ChildReparenting(t *testing.T) {
	// Test that adding a child to a new parent removes it from old parent
	parent1 := NewQueryNodeImpl(nil)
	parent2 := NewQueryNodeImpl(nil)
	child := NewQueryNodeImpl(nil)

	parent1.AddChild(child)
	if child.GetParent() != parent1 {
		t.Error("Child should have parent1 as parent")
	}

	// Add to new parent
	parent2.AddChild(child)
	if child.GetParent() != parent2 {
		t.Error("Child should have parent2 as parent after reparenting")
	}

	// Verify removed from old parent
	if len(parent1.GetChildren()) != 0 {
		t.Error("Child should be removed from parent1")
	}
}
