// Package flexible provides the flexible query parser framework for Lucene-compatible query parsing.
package flexible

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// QueryBuilder is the interface for building Lucene Query objects from QueryNodes.
// Builders transform the query tree into executable search queries.
type QueryBuilder interface {
	// Build builds a Lucene Query from a QueryNode.
	Build(node QueryNode) (search.Query, error)
}

// QueryTreeBuilder is the main builder that manages multiple node builders.
// It dispatches building to specific builders based on node type.
type QueryTreeBuilder struct {
	builders map[string]QueryBuilder
}

// NewQueryTreeBuilder creates a new QueryTreeBuilder.
func NewQueryTreeBuilder() *QueryTreeBuilder {
	return &QueryTreeBuilder{
		builders: make(map[string]QueryBuilder),
	}
}

// SetBuilder sets a builder for a specific node type.
func (b *QueryTreeBuilder) SetBuilder(nodeType string, builder QueryBuilder) {
	b.builders[nodeType] = builder
}

// GetBuilder returns the builder for a specific node type.
func (b *QueryTreeBuilder) GetBuilder(nodeType string) QueryBuilder {
	return b.builders[nodeType]
}

// Build builds a Lucene Query from a QueryNode.
// It dispatches to the appropriate builder based on node type.
func (b *QueryTreeBuilder) Build(node QueryNode) (search.Query, error) {
	if node == nil {
		return nil, fmt.Errorf("cannot build from nil node")
	}

	// Get the node type
	nodeType := getNodeType(node)

	// Find the appropriate builder
	builder := b.builders[nodeType]
	if builder == nil {
		return nil, fmt.Errorf("no builder registered for node type: %s", nodeType)
	}

	return builder.Build(node)
}

// getNodeType returns the type name of a QueryNode.
func getNodeType(node QueryNode) string {
	switch node.(type) {
	case *FieldQueryNode:
		return "FieldQueryNode"
	case *BooleanQueryNode:
		return "BooleanQueryNode"
	case *AndQueryNode:
		return "AndQueryNode"
	case *OrQueryNode:
		return "OrQueryNode"
	case *ModifierQueryNode:
		return "ModifierQueryNode"
	case *BoostQueryNode:
		return "BoostQueryNode"
	case *FuzzyQueryNode:
		return "FuzzyQueryNode"
	case *RangeQueryNode:
		return "RangeQueryNode"
	case *PhraseSlopQueryNode:
		return "PhraseSlopQueryNode"
	case *GroupQueryNode:
		return "GroupQueryNode"
	case *MatchAllDocsQueryNode:
		return "MatchAllDocsQueryNode"
	case *MatchNoDocsQueryNode:
		return "MatchNoDocsQueryNode"
	default:
		return "Unknown"
	}
}

// BooleanQueryNodeBuilder builds BooleanQuery from BooleanQueryNode.
type BooleanQueryNodeBuilder struct {
	queryTreeBuilder *QueryTreeBuilder
}

// NewBooleanQueryNodeBuilder creates a new BooleanQueryNodeBuilder.
func NewBooleanQueryNodeBuilder(queryTreeBuilder *QueryTreeBuilder) *BooleanQueryNodeBuilder {
	return &BooleanQueryNodeBuilder{
		queryTreeBuilder: queryTreeBuilder,
	}
}

// Build builds a BooleanQuery from a BooleanQueryNode.
func (b *BooleanQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	booleanNode, ok := node.(*BooleanQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected BooleanQueryNode, got %T", node)
	}

	// Build boolean query
	booleanQuery := search.NewBooleanQuery()

	// Process children
	for _, child := range booleanNode.GetChildren() {
		childQuery, err := b.queryTreeBuilder.Build(child)
		if err != nil {
			return nil, fmt.Errorf("failed to build child query: %w", err)
		}

		// Check for modifier
		if modifierNode, ok := child.(*ModifierQueryNode); ok {
			switch modifierNode.GetModifier() {
			case ModifierRequired:
				booleanQuery.Add(childQuery, search.MUST)
			case ModifierProhibited:
				booleanQuery.Add(childQuery, search.MUST_NOT)
			default:
				booleanQuery.Add(childQuery, search.SHOULD)
			}
		} else {
			// Default to SHOULD
			booleanQuery.Add(childQuery, search.SHOULD)
		}
	}

	return booleanQuery, nil
}

// FieldQueryNodeBuilder builds TermQuery from FieldQueryNode.
type FieldQueryNodeBuilder struct{}

// NewFieldQueryNodeBuilder creates a new FieldQueryNodeBuilder.
func NewFieldQueryNodeBuilder() *FieldQueryNodeBuilder {
	return &FieldQueryNodeBuilder{}
}

// Build builds a TermQuery from a FieldQueryNode.
func (b *FieldQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	fieldNode, ok := node.(*FieldQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected FieldQueryNode, got %T", node)
	}

	// Create term query
	term := index.NewTerm(fieldNode.GetField(), fieldNode.GetText())
	termQuery := search.NewTermQuery(term)
	return termQuery, nil
}

// BoostQueryNodeBuilder builds a boosted query from BoostQueryNode.
type BoostQueryNodeBuilder struct {
	queryTreeBuilder *QueryTreeBuilder
}

// NewBoostQueryNodeBuilder creates a new BoostQueryNodeBuilder.
func NewBoostQueryNodeBuilder(queryTreeBuilder *QueryTreeBuilder) *BoostQueryNodeBuilder {
	return &BoostQueryNodeBuilder{
		queryTreeBuilder: queryTreeBuilder,
	}
}

// Build builds a boosted query from a BoostQueryNode.
func (b *BoostQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	boostNode, ok := node.(*BoostQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected BoostQueryNode, got %T", node)
	}

	// Build the child query
	children := boostNode.GetChildren()
	if len(children) == 0 {
		return nil, fmt.Errorf("BoostQueryNode has no children")
	}

	childQuery, err := b.queryTreeBuilder.Build(children[0])
	if err != nil {
		return nil, fmt.Errorf("failed to build child query: %w", err)
	}

	// Apply boost using BoostQuery wrapper
	boostedQuery := search.NewBoostQuery(childQuery, float32(boostNode.GetValue()))

	return boostedQuery, nil
}

// FuzzyQueryNodeBuilder builds FuzzyQuery from FuzzyQueryNode.
type FuzzyQueryNodeBuilder struct{}

// NewFuzzyQueryNodeBuilder creates a new FuzzyQueryNodeBuilder.
func NewFuzzyQueryNodeBuilder() *FuzzyQueryNodeBuilder {
	return &FuzzyQueryNodeBuilder{}
}

// Build builds a FuzzyQuery from a FuzzyQueryNode.
func (b *FuzzyQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	fuzzyNode, ok := node.(*FuzzyQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected FuzzyQueryNode, got %T", node)
	}

	// Create fuzzy query
	// Convert minSimilarity (0.0-1.0) to maxEdits (integer)
	// This is a simplified conversion - in practice, this would use Levenshtein distance calculation
	maxEdits := calculateMaxEdits(fuzzyNode.GetMinSimilarity(), len(fuzzyNode.GetText()))

	term := index.NewTerm(fuzzyNode.GetField(), fuzzyNode.GetText())
	fuzzyQuery := search.NewFuzzyQueryWithParams(term, maxEdits, fuzzyNode.GetPrefixLength(), 50)

	return fuzzyQuery, nil
}

// calculateMaxEdits converts a similarity threshold to max edit distance.
func calculateMaxEdits(minSimilarity float64, termLength int) int {
	// Simple conversion: similarity < 0.5 = 2 edits, >= 0.5 = 1 edit, = 1.0 = 0 edits
	if minSimilarity >= 1.0 {
		return 0
	}
	if minSimilarity >= 0.5 {
		return 1
	}
	return 2
}

// RangeQueryNodeBuilder builds TermRangeQuery from RangeQueryNode.
type RangeQueryNodeBuilder struct{}

// NewRangeQueryNodeBuilder creates a new RangeQueryNodeBuilder.
func NewRangeQueryNodeBuilder() *RangeQueryNodeBuilder {
	return &RangeQueryNodeBuilder{}
}

// Build builds a TermRangeQuery from a RangeQueryNode.
func (b *RangeQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	rangeNode, ok := node.(*RangeQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected RangeQueryNode, got %T", node)
	}

	// Convert string bounds to bytes
	var lowerBytes, upperBytes []byte
	if rangeNode.GetLower() != "*" {
		lowerBytes = []byte(rangeNode.GetLower())
	}
	if rangeNode.GetUpper() != "*" {
		upperBytes = []byte(rangeNode.GetUpper())
	}

	// Create range query
	rangeQuery := search.NewTermRangeQuery(
		rangeNode.GetField(),
		lowerBytes,
		upperBytes,
		rangeNode.IsLowerInclusive(),
		rangeNode.IsUpperInclusive(),
	)

	return rangeQuery, nil
}

// PhraseQueryNodeBuilder builds PhraseQuery from PhraseSlopQueryNode.
type PhraseQueryNodeBuilder struct{}

// NewPhraseQueryNodeBuilder creates a new PhraseQueryNodeBuilder.
func NewPhraseQueryNodeBuilder() *PhraseQueryNodeBuilder {
	return &PhraseQueryNodeBuilder{}
}

// Build builds a PhraseQuery from a PhraseSlopQueryNode.
func (b *PhraseQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	phraseNode, ok := node.(*PhraseSlopQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected PhraseSlopQueryNode, got %T", node)
	}

	// Split text into terms
	terms := splitTerms(phraseNode.GetText())

	// Create index terms
	indexTerms := make([]*index.Term, len(terms))
	for i, term := range terms {
		indexTerms[i] = index.NewTerm(phraseNode.GetField(), term)
	}

	// Create phrase query with slop
	phraseQuery := search.NewPhraseQueryWithSlop(phraseNode.GetSlop(), phraseNode.GetField(), indexTerms...)

	return phraseQuery, nil
}

// splitTerms splits text into individual terms.
func splitTerms(text string) []string {
	// Simple space-based splitting
	// In a real implementation, this would use the analyzer
	var terms []string
	start := 0
	for i, c := range text {
		if c == ' ' {
			if i > start {
				terms = append(terms, text[start:i])
			}
			start = i + 1
		}
	}
	if start < len(text) {
		terms = append(terms, text[start:])
	}
	return terms
}

// TermRangeQueryNodeBuilder builds TermRangeQuery from RangeQueryNode.
// This is an alias for RangeQueryNodeBuilder.
type TermRangeQueryNodeBuilder struct {
	*RangeQueryNodeBuilder
}

// NewTermRangeQueryNodeBuilder creates a new TermRangeQueryNodeBuilder.
func NewTermRangeQueryNodeBuilder() *TermRangeQueryNodeBuilder {
	return &TermRangeQueryNodeBuilder{
		RangeQueryNodeBuilder: NewRangeQueryNodeBuilder(),
	}
}

// WildcardQueryNodeBuilder builds WildcardQuery from FieldQueryNode with wildcards.
type WildcardQueryNodeBuilder struct{}

// NewWildcardQueryNodeBuilder creates a new WildcardQueryNodeBuilder.
func NewWildcardQueryNodeBuilder() *WildcardQueryNodeBuilder {
	return &WildcardQueryNodeBuilder{}
}

// Build builds a WildcardQuery from a FieldQueryNode.
func (b *WildcardQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	fieldNode, ok := node.(*FieldQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected FieldQueryNode, got %T", node)
	}

	// Create wildcard query
	term := index.NewTerm(fieldNode.GetField(), fieldNode.GetText())
	wildcardQuery := search.NewWildcardQuery(term)
	return wildcardQuery, nil
}

// GroupQueryNodeBuilder builds queries from GroupQueryNode.
type GroupQueryNodeBuilder struct {
	queryTreeBuilder *QueryTreeBuilder
}

// NewGroupQueryNodeBuilder creates a new GroupQueryNodeBuilder.
func NewGroupQueryNodeBuilder(queryTreeBuilder *QueryTreeBuilder) *GroupQueryNodeBuilder {
	return &GroupQueryNodeBuilder{
		queryTreeBuilder: queryTreeBuilder,
	}
}

// Build builds a query from a GroupQueryNode.
// Groups are transparent - they just return their child's query.
func (b *GroupQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	groupNode, ok := node.(*GroupQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected GroupQueryNode, got %T", node)
	}

	// Build the child query
	children := groupNode.GetChildren()
	if len(children) == 0 {
		return nil, fmt.Errorf("GroupQueryNode has no children")
	}

	return b.queryTreeBuilder.Build(children[0])
}

// MatchAllDocsQueryNodeBuilder builds MatchAllDocsQuery.
type MatchAllDocsQueryNodeBuilder struct{}

// NewMatchAllDocsQueryNodeBuilder creates a new MatchAllDocsQueryNodeBuilder.
func NewMatchAllDocsQueryNodeBuilder() *MatchAllDocsQueryNodeBuilder {
	return &MatchAllDocsQueryNodeBuilder{}
}

// Build builds a MatchAllDocsQuery.
func (b *MatchAllDocsQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	_, ok := node.(*MatchAllDocsQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected MatchAllDocsQueryNode, got %T", node)
	}

	return search.NewMatchAllDocsQuery(), nil
}

// MatchNoDocsQueryNodeBuilder builds MatchNoDocsQuery.
type MatchNoDocsQueryNodeBuilder struct{}

// NewMatchNoDocsQueryNodeBuilder creates a new MatchNoDocsQueryNodeBuilder.
func NewMatchNoDocsQueryNodeBuilder() *MatchNoDocsQueryNodeBuilder {
	return &MatchNoDocsQueryNodeBuilder{}
}

// Build builds a MatchNoDocsQuery.
func (b *MatchNoDocsQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	_, ok := node.(*MatchNoDocsQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected MatchNoDocsQueryNode, got %T", node)
	}

	return search.NewMatchNoDocsQuery(), nil
}
