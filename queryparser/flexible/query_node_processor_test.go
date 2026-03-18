package flexible

import (
	"testing"
)

func TestQueryNodeProcessorImpl_New(t *testing.T) {
	processor := NewQueryNodeProcessorImpl()

	if len(processor.GetChildProcessors()) != 0 {
		t.Errorf("Expected 0 child processors, got %d", len(processor.GetChildProcessors()))
	}
}

func TestQueryNodeProcessorImpl_AddChildProcessor(t *testing.T) {
	parent := NewQueryNodeProcessorImpl()
	child := NewQueryNodeProcessorImpl()

	parent.AddChildProcessor(child)

	if len(parent.GetChildProcessors()) != 1 {
		t.Errorf("Expected 1 child processor, got %d", len(parent.GetChildProcessors()))
	}
}

func TestQueryNodeProcessorImpl_Process(t *testing.T) {
	processor := NewQueryNodeProcessorImpl()
	node := NewFieldQueryNode("field", "value", 0, 10)

	result, err := processor.Process(node)
	if err != nil {
		t.Errorf("Process() returned error: %v", err)
	}

	if result != node {
		t.Error("Process should return the same node when no children")
	}
}

func TestQueryNodeProcessorPipeline_New(t *testing.T) {
	processor1 := NewQueryNodeProcessorImpl()
	processor2 := NewQueryNodeProcessorImpl()

	pipeline := NewQueryNodeProcessorPipeline([]QueryNodeProcessor{processor1, processor2})

	if len(pipeline.GetPipeline()) != 2 {
		t.Errorf("Expected 2 processors in pipeline, got %d", len(pipeline.GetPipeline()))
	}
}

func TestQueryNodeProcessorPipeline_AddProcessor(t *testing.T) {
	pipeline := NewQueryNodeProcessorPipeline(nil)
	processor := NewQueryNodeProcessorImpl()

	pipeline.AddProcessor(processor)

	if len(pipeline.GetPipeline()) != 1 {
		t.Errorf("Expected 1 processor in pipeline, got %d", len(pipeline.GetPipeline()))
	}
}

func TestQueryNodeProcessorPipeline_Process(t *testing.T) {
	processor := NewQueryNodeProcessorImpl()
	pipeline := NewQueryNodeProcessorPipeline([]QueryNodeProcessor{processor})

	node := NewFieldQueryNode("field", "value", 0, 10)
	result, err := pipeline.Process(node)

	if err != nil {
		t.Errorf("Process() returned error: %v", err)
	}

	if result != node {
		t.Error("Process should return the same node")
	}
}

func TestNoChildOptimizationProcessor_New(t *testing.T) {
	processor := NewNoChildOptimizationProcessor()

	if processor == nil {
		t.Error("NewNoChildOptimizationProcessor should not return nil")
	}
}

func TestNoChildOptimizationProcessor_Process(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() QueryNode
		expected QueryNode
	}{
		{
			name: "nil node",
			setup: func() QueryNode {
				return nil
			},
			expected: nil,
		},
		{
			name: "field node with no children",
			setup: func() QueryNode {
				return NewFieldQueryNode("field", "value", 0, 10)
			},
			expected: nil, // FieldQueryNode is a leaf, should be kept
		},
		{
			name: "boolean node with children",
			setup: func() QueryNode {
				child1 := NewFieldQueryNode("", "value1", 0, 0)
				child2 := NewFieldQueryNode("", "value2", 0, 0)
				return NewBooleanQueryNode("AND", []QueryNode{child1, child2})
			},
			expected: nil, // Will be set to actual result
		},
	}

	processor := NewNoChildOptimizationProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := tt.setup()
			result, err := processor.Process(node)

			if err != nil {
				t.Errorf("Process() returned error: %v", err)
			}

			if tt.name == "boolean node with children" {
				if result == nil {
					t.Error("Boolean node with children should not be removed")
				}
				if bn, ok := result.(*BooleanQueryNode); ok {
					if len(bn.GetChildren()) != 2 {
						t.Errorf("Expected 2 children, got %d", len(bn.GetChildren()))
					}
				}
			}
		})
	}
}

func TestNoChildOptimizationProcessor_RemoveEmptyBoolean(t *testing.T) {
	processor := NewNoChildOptimizationProcessor()

	// Boolean node with no children should be removed
	emptyBoolean := NewBooleanQueryNode("AND", nil)
	result, err := processor.Process(emptyBoolean)

	if err != nil {
		t.Errorf("Process() returned error: %v", err)
	}

	if result != nil {
		t.Error("Empty boolean node should be removed")
	}
}

func TestRemoveDeletedQueryNodesProcessor_New(t *testing.T) {
	processor := NewRemoveDeletedQueryNodesProcessor()

	if processor == nil {
		t.Error("NewRemoveDeletedQueryNodesProcessor should not return nil")
	}
}

func TestRemoveDeletedQueryNodesProcessor_Process(t *testing.T) {
	processor := NewRemoveDeletedQueryNodesProcessor()

	// Node not marked as deleted
	node := NewFieldQueryNode("field", "value", 0, 10)
	result, err := processor.Process(node)

	if err != nil {
		t.Errorf("Process() returned error: %v", err)
	}

	if result == nil {
		t.Error("Non-deleted node should not be removed")
	}
}

func TestRemoveDeletedQueryNodesProcessor_RemoveDeleted(t *testing.T) {
	processor := NewRemoveDeletedQueryNodesProcessor()

	// Node marked as deleted
	node := NewFieldQueryNode("field", "value", 0, 10)
	node.SetTag(TagDeleted, true)

	result, err := processor.Process(node)

	if err != nil {
		t.Errorf("Process() returned error: %v", err)
	}

	if result != nil {
		t.Error("Deleted node should be removed")
	}
}

func TestRemoveDeletedQueryNodesProcessor_RemoveDeletedChildren(t *testing.T) {
	processor := NewRemoveDeletedQueryNodesProcessor()

	// Parent with one deleted child
	child1 := NewFieldQueryNode("", "value1", 0, 0)
	child2 := NewFieldQueryNode("", "value2", 0, 0)
	child2.SetTag(TagDeleted, true)

	parent := NewBooleanQueryNode("AND", []QueryNode{child1, child2})

	result, err := processor.Process(parent)

	if err != nil {
		t.Errorf("Process() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Parent should not be removed")
	}

	if bn, ok := result.(*BooleanQueryNode); ok {
		if len(bn.GetChildren()) != 1 {
			t.Errorf("Expected 1 child after removal, got %d", len(bn.GetChildren()))
		}
	}
}

func TestPhraseSlopQueryNodeProcessor_New(t *testing.T) {
	processor := NewPhraseSlopQueryNodeProcessor()

	if processor == nil {
		t.Error("NewPhraseSlopQueryNodeProcessor should not return nil")
	}
}

func TestPhraseSlopQueryNodeProcessor_Process(t *testing.T) {
	processor := NewPhraseSlopQueryNodeProcessor()

	// Phrase node with negative slop
	node := NewPhraseSlopQueryNode("field", "phrase", -5, 0, 10)
	result, err := processor.Process(node)

	if err != nil {
		t.Errorf("Process() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if phrase, ok := result.(*PhraseSlopQueryNode); ok {
		if phrase.GetSlop() != 0 {
			t.Errorf("Expected slop to be normalized to 0, got %d", phrase.GetSlop())
		}
	}
}

func TestBoostQueryNodeProcessor_New(t *testing.T) {
	processor := NewBoostQueryNodeProcessor()

	if processor == nil {
		t.Error("NewBoostQueryNodeProcessor should not return nil")
	}
}

func TestBoostQueryNodeProcessor_Process(t *testing.T) {
	processor := NewBoostQueryNodeProcessor()

	// Boost node with negative value
	child := NewFieldQueryNode("", "value", 0, 0)
	node := NewBoostQueryNode(child, -2.0)

	result, err := processor.Process(node)

	if err != nil {
		t.Errorf("Process() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if boost, ok := result.(*BoostQueryNode); ok {
		if boost.GetValue() != 1.0 {
			t.Errorf("Expected boost to be normalized to 1.0, got %f", boost.GetValue())
		}
	}
}

func TestBoostQueryNodeProcessor_ZeroBoost(t *testing.T) {
	processor := NewBoostQueryNodeProcessor()

	// Boost node with zero value
	child := NewFieldQueryNode("", "value", 0, 0)
	node := NewBoostQueryNode(child, 0.0)

	result, err := processor.Process(node)

	if err != nil {
		t.Errorf("Process() returned error: %v", err)
	}

	if boost, ok := result.(*BoostQueryNode); ok {
		if boost.GetValue() != 1.0 {
			t.Errorf("Expected boost to be normalized to 1.0, got %f", boost.GetValue())
		}
	}
}

func TestIsLeafNodeType(t *testing.T) {
	tests := []struct {
		node     QueryNode
		expected bool
	}{
		{NewFieldQueryNode("", "", 0, 0), true},
		{NewFuzzyQueryNode("", "", 0.5, 0, 0, 0), true},
		{NewPhraseSlopQueryNode("", "", 0, 0, 0), true},
		{NewMatchAllDocsQueryNode(), true},
		{NewMatchNoDocsQueryNode(), true},
		{NewBooleanQueryNode("AND", nil), false},
		{NewAndQueryNode(nil), false},
		{NewOrQueryNode(nil), false},
		{NewGroupQueryNode(nil), false},
	}

	for _, tt := range tests {
		result := isLeafNodeType(tt.node)
		if result != tt.expected {
			t.Errorf("isLeafNodeType(%T) = %v, want %v", tt.node, result, tt.expected)
		}
	}
}
