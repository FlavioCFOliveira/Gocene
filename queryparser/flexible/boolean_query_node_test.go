package flexible

import (
	"testing"
)

func TestBooleanQueryNode_New(t *testing.T) {
	child1 := NewFieldQueryNode("field1", "value1", 0, 10)
	child2 := NewFieldQueryNode("field2", "value2", 11, 20)

	node := NewBooleanQueryNode("AND", []QueryNode{child1, child2})

	if node.GetOperator() != "AND" {
		t.Errorf("GetOperator() = %v, want AND", node.GetOperator())
	}

	if len(node.GetChildren()) != 2 {
		t.Errorf("Expected 2 children, got %d", len(node.GetChildren()))
	}
}

func TestBooleanQueryNode_SetOperator(t *testing.T) {
	node := NewBooleanQueryNode("AND", nil)
	node.SetOperator("OR")

	if node.GetOperator() != "OR" {
		t.Errorf("SetOperator failed, got %v", node.GetOperator())
	}
}

func TestBooleanQueryNode_ToQueryString(t *testing.T) {
	child1 := NewFieldQueryNode("", "value1", 0, 0)
	child2 := NewFieldQueryNode("", "value2", 0, 0)

	node := NewBooleanQueryNode("AND", []QueryNode{child1, child2})
	result := node.ToQueryString(false)

	expected := "value1 AND value2"
	if result != expected {
		t.Errorf("ToQueryString() = %v, want %v", result, expected)
	}
}

func TestBooleanQueryNode_CloneTree(t *testing.T) {
	child1 := NewFieldQueryNode("field1", "value1", 0, 10)
	original := NewBooleanQueryNode("AND", []QueryNode{child1})
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*BooleanQueryNode)

	if clonedNode.GetOperator() != original.GetOperator() {
		t.Error("Clone should have same operator")
	}

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if len(clonedNode.GetChildren()) != len(original.GetChildren()) {
		t.Error("Clone should have same number of children")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}

func TestAndQueryNode_New(t *testing.T) {
	child1 := NewFieldQueryNode("", "value1", 0, 0)
	child2 := NewFieldQueryNode("", "value2", 0, 0)

	node := NewAndQueryNode([]QueryNode{child1, child2})

	if node.GetOperator() != "AND" {
		t.Errorf("Expected operator AND, got %v", node.GetOperator())
	}

	if len(node.GetChildren()) != 2 {
		t.Errorf("Expected 2 children, got %d", len(node.GetChildren()))
	}
}

func TestAndQueryNode_CloneTree(t *testing.T) {
	child := NewFieldQueryNode("field", "value", 0, 10)
	original := NewAndQueryNode([]QueryNode{child})
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*AndQueryNode)

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if len(clonedNode.GetChildren()) != 1 {
		t.Error("Clone should have 1 child")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}

func TestOrQueryNode_New(t *testing.T) {
	child1 := NewFieldQueryNode("", "value1", 0, 0)
	child2 := NewFieldQueryNode("", "value2", 0, 0)

	node := NewOrQueryNode([]QueryNode{child1, child2})

	if node.GetOperator() != "OR" {
		t.Errorf("Expected operator OR, got %v", node.GetOperator())
	}

	if len(node.GetChildren()) != 2 {
		t.Errorf("Expected 2 children, got %d", len(node.GetChildren()))
	}
}

func TestOrQueryNode_CloneTree(t *testing.T) {
	child := NewFieldQueryNode("field", "value", 0, 10)
	original := NewOrQueryNode([]QueryNode{child})
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*OrQueryNode)

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if len(clonedNode.GetChildren()) != 1 {
		t.Error("Clone should have 1 child")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}
