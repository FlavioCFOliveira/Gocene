package flexible

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestQueryTreeBuilder_New(t *testing.T) {
	builder := NewQueryTreeBuilder()

	if builder == nil {
		t.Error("NewQueryTreeBuilder should not return nil")
	}

	if builder.builders == nil {
		t.Error("builders map should be initialized")
	}
}

func TestQueryTreeBuilder_SetBuilder(t *testing.T) {
	builder := NewQueryTreeBuilder()
	fieldBuilder := NewFieldQueryNodeBuilder()

	builder.SetBuilder("FieldQueryNode", fieldBuilder)

	if builder.GetBuilder("FieldQueryNode") != fieldBuilder {
		t.Error("SetBuilder should register the builder")
	}
}

func TestQueryTreeBuilder_Build_NilNode(t *testing.T) {
	builder := NewQueryTreeBuilder()

	_, err := builder.Build(nil)
	if err == nil {
		t.Error("Build should return error for nil node")
	}
}

func TestQueryTreeBuilder_Build_NoBuilder(t *testing.T) {
	builder := NewQueryTreeBuilder()
	node := NewFieldQueryNode("field", "value", 0, 10)

	_, err := builder.Build(node)
	if err == nil {
		t.Error("Build should return error when no builder is registered")
	}
}

func TestFieldQueryNodeBuilder_Build(t *testing.T) {
	builder := NewFieldQueryNodeBuilder()
	node := NewFieldQueryNode("title", "test", 0, 10)

	query, err := builder.Build(node)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Check if it's a TermQuery
	if _, ok := query.(*search.TermQuery); !ok {
		t.Errorf("Expected TermQuery, got %T", query)
	}
}

func TestFieldQueryNodeBuilder_Build_WrongType(t *testing.T) {
	builder := NewFieldQueryNodeBuilder()
	node := NewBooleanQueryNode("AND", nil)

	_, err := builder.Build(node)
	if err == nil {
		t.Error("Build should return error for wrong node type")
	}
}

func TestBooleanQueryNodeBuilder_Build(t *testing.T) {
	// Create tree builder and register builders
	treeBuilder := NewQueryTreeBuilder()
	treeBuilder.SetBuilder("FieldQueryNode", NewFieldQueryNodeBuilder())
	treeBuilder.SetBuilder("BooleanQueryNode", NewBooleanQueryNodeBuilder(treeBuilder))

	// Create boolean node with children
	child1 := NewFieldQueryNode("", "value1", 0, 0)
	child2 := NewFieldQueryNode("", "value2", 0, 0)
	booleanNode := NewBooleanQueryNode("AND", []QueryNode{child1, child2})

	query, err := treeBuilder.Build(booleanNode)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Check if it's a BooleanQuery
	if _, ok := query.(*search.BooleanQuery); !ok {
		t.Errorf("Expected BooleanQuery, got %T", query)
	}
}

func TestBoostQueryNodeBuilder_Build(t *testing.T) {
	// Create tree builder and register builders
	treeBuilder := NewQueryTreeBuilder()
	treeBuilder.SetBuilder("FieldQueryNode", NewFieldQueryNodeBuilder())
	treeBuilder.SetBuilder("BoostQueryNode", NewBoostQueryNodeBuilder(treeBuilder))

	// Create boost node
	child := NewFieldQueryNode("field", "value", 0, 10)
	boostNode := NewBoostQueryNode(child, 2.5)

	query, err := treeBuilder.Build(boostNode)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Check if it's a BoostQuery
	if _, ok := query.(*search.BoostQuery); !ok {
		t.Errorf("Expected BoostQuery, got %T", query)
	}
}

func TestFuzzyQueryNodeBuilder_Build(t *testing.T) {
	builder := NewFuzzyQueryNodeBuilder()
	node := NewFuzzyQueryNode("field", "value", 0.5, 0, 0, 10)

	query, err := builder.Build(node)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Check if it's a FuzzyQuery
	if _, ok := query.(*search.FuzzyQuery); !ok {
		t.Errorf("Expected FuzzyQuery, got %T", query)
	}
}

func TestRangeQueryNodeBuilder_Build(t *testing.T) {
	builder := NewRangeQueryNodeBuilder()
	node := NewRangeQueryNode("field", "start", "end", BoundInclusive, BoundExclusive)

	query, err := builder.Build(node)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Check if it's a TermRangeQuery
	if _, ok := query.(*search.TermRangeQuery); !ok {
		t.Errorf("Expected TermRangeQuery, got %T", query)
	}
}

func TestPhraseQueryNodeBuilder_Build(t *testing.T) {
	builder := NewPhraseQueryNodeBuilder()
	node := NewPhraseSlopQueryNode("field", "term1 term2", 2, 0, 20)

	query, err := builder.Build(node)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Check if it's a PhraseQuery
	if _, ok := query.(*search.PhraseQuery); !ok {
		t.Errorf("Expected PhraseQuery, got %T", query)
	}
}

func TestWildcardQueryNodeBuilder_Build(t *testing.T) {
	builder := NewWildcardQueryNodeBuilder()
	node := NewFieldQueryNode("field", "val*e", 0, 10)

	query, err := builder.Build(node)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Check if it's a WildcardQuery
	if _, ok := query.(*search.WildcardQuery); !ok {
		t.Errorf("Expected WildcardQuery, got %T", query)
	}
}

func TestGroupQueryNodeBuilder_Build(t *testing.T) {
	// Create tree builder and register builders
	treeBuilder := NewQueryTreeBuilder()
	treeBuilder.SetBuilder("FieldQueryNode", NewFieldQueryNodeBuilder())
	treeBuilder.SetBuilder("GroupQueryNode", NewGroupQueryNodeBuilder(treeBuilder))

	// Create group node
	child := NewFieldQueryNode("field", "value", 0, 10)
	groupNode := NewGroupQueryNode(child)

	query, err := treeBuilder.Build(groupNode)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Group should return the child's query (TermQuery)
	if _, ok := query.(*search.TermQuery); !ok {
		t.Errorf("Expected TermQuery (child of group), got %T", query)
	}
}

func TestMatchAllDocsQueryNodeBuilder_Build(t *testing.T) {
	builder := NewMatchAllDocsQueryNodeBuilder()
	node := NewMatchAllDocsQueryNode()

	query, err := builder.Build(node)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Check if it's a MatchAllDocsQuery
	if _, ok := query.(*search.MatchAllDocsQuery); !ok {
		t.Errorf("Expected MatchAllDocsQuery, got %T", query)
	}
}

func TestMatchNoDocsQueryNodeBuilder_Build(t *testing.T) {
	builder := NewMatchNoDocsQueryNodeBuilder()
	node := NewMatchNoDocsQueryNode()

	query, err := builder.Build(node)
	if err != nil {
		t.Errorf("Build() returned error: %v", err)
	}

	if query == nil {
		t.Error("Build should return a query")
	}

	// Check if it's a MatchNoDocsQuery
	if _, ok := query.(*search.MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery, got %T", query)
	}
}

func TestGetNodeType(t *testing.T) {
	tests := []struct {
		node     QueryNode
		expected string
	}{
		{NewFieldQueryNode("", "", 0, 0), "FieldQueryNode"},
		{NewBooleanQueryNode("AND", nil), "BooleanQueryNode"},
		{NewAndQueryNode(nil), "AndQueryNode"},
		{NewOrQueryNode(nil), "OrQueryNode"},
		{NewModifierQueryNode(nil, ModifierNone), "ModifierQueryNode"},
		{NewBoostQueryNode(nil, 1.0), "BoostQueryNode"},
		{NewFuzzyQueryNode("", "", 0.5, 0, 0, 0), "FuzzyQueryNode"},
		{NewRangeQueryNode("", "", "", BoundInclusive, BoundInclusive), "RangeQueryNode"},
		{NewPhraseSlopQueryNode("", "", 0, 0, 0), "PhraseSlopQueryNode"},
		{NewGroupQueryNode(nil), "GroupQueryNode"},
		{NewMatchAllDocsQueryNode(), "MatchAllDocsQueryNode"},
		{NewMatchNoDocsQueryNode(), "MatchNoDocsQueryNode"},
	}

	for _, tt := range tests {
		result := getNodeType(tt.node)
		if result != tt.expected {
			t.Errorf("getNodeType(%T) = %v, want %v", tt.node, result, tt.expected)
		}
	}
}

func TestCalculateMaxEdits(t *testing.T) {
	tests := []struct {
		similarity float64
		termLength int
		expected   int
	}{
		{1.0, 10, 0},
		{0.8, 10, 1},
		{0.5, 10, 1},
		{0.4, 10, 2},
		{0.0, 10, 2},
	}

	for _, tt := range tests {
		result := calculateMaxEdits(tt.similarity, tt.termLength)
		if result != tt.expected {
			t.Errorf("calculateMaxEdits(%f, %d) = %d, want %d", tt.similarity, tt.termLength, result, tt.expected)
		}
	}
}

func TestSplitTerms(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"single", []string{"single"}},
		{"term1 term2", []string{"term1", "term2"}},
		{"  multiple   spaces  ", []string{"multiple", "spaces"}},
	}

	for _, tt := range tests {
		result := splitTerms(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitTerms(%q) returned %d terms, want %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i, term := range result {
			if term != tt.expected[i] {
				t.Errorf("splitTerms(%q)[%d] = %q, want %q", tt.input, i, term, tt.expected[i])
			}
		}
	}
}
