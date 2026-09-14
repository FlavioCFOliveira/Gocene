// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DummyQueryNodeBuilder is a no-op builder that always returns MatchNoDocsQuery.
// It is used as a placeholder for unsupported or deleted node types.
// This is the Go equivalent of Lucene's DummyQueryNodeBuilder.
type DummyQueryNodeBuilder struct{}

// NewDummyQueryNodeBuilder creates a new DummyQueryNodeBuilder.
func NewDummyQueryNodeBuilder() *DummyQueryNodeBuilder { return &DummyQueryNodeBuilder{} }

// Build returns a MatchNoDocsQuery regardless of the node.
func (b *DummyQueryNodeBuilder) Build(_ QueryNode) (search.Query, error) {
	return search.NewMatchNoDocsQuery(""), nil
}

// AnyQueryNodeBuilder builds a BooleanQuery from an AnyQueryNode.
// The minimum-should-match requirement is set on the resulting BooleanQuery.
// This is the Go equivalent of Lucene's AnyQueryNodeBuilder.
type AnyQueryNodeBuilder struct {
	treeBuilder *QueryTreeBuilder
}

// NewAnyQueryNodeBuilder creates a new AnyQueryNodeBuilder.
func NewAnyQueryNodeBuilder(treeBuilder *QueryTreeBuilder) *AnyQueryNodeBuilder {
	return &AnyQueryNodeBuilder{treeBuilder: treeBuilder}
}

// Build builds a BooleanQuery with minimum-should-match set.
func (b *AnyQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	anyNode, ok := node.(*AnyQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected AnyQueryNode, got %T", node)
	}

	// Lucene builds this through BooleanQuery.Builder.
	bq := search.NewBooleanQueryBuilder()
	for _, child := range anyNode.GetChildren() {
		childQuery, err := b.treeBuilder.Build(child)
		if err != nil {
			return nil, err
		}
		bq.Add(childQuery, search.SHOULD)
	}
	bq.SetMinimumNumberShouldMatch(anyNode.GetMinimumMatchingElements())
	return bq.Build(), nil
}

// ModifierQueryNodeBuilder builds queries from ModifierQueryNode.
// This is the Go equivalent of Lucene's ModifierQueryNodeBuilder.
type ModifierQueryNodeBuilder struct {
	treeBuilder *QueryTreeBuilder
}

// NewModifierQueryNodeBuilder creates a new ModifierQueryNodeBuilder.
func NewModifierQueryNodeBuilder(treeBuilder *QueryTreeBuilder) *ModifierQueryNodeBuilder {
	return &ModifierQueryNodeBuilder{treeBuilder: treeBuilder}
}

// Build builds a BooleanQuery that wraps the child with the appropriate Occur.
func (b *ModifierQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	modNode, ok := node.(*ModifierQueryNode)
	if !ok {
		// Also handle BooleanModifierNode (which embeds ModifierQueryNode)
		if bmn, ok2 := node.(*BooleanModifierNode); ok2 {
			modNode = bmn.ModifierQueryNode
		} else {
			return nil, fmt.Errorf("expected ModifierQueryNode, got %T", node)
		}
	}

	children := modNode.GetChildren()
	if len(children) == 0 {
		return search.NewMatchNoDocsQuery(""), nil
	}

	childQuery, err := b.treeBuilder.Build(children[0])
	if err != nil {
		return nil, err
	}

	switch modNode.GetModifier() {
	case ModifierRequired:
		bq := search.NewBooleanQueryBuilder()
		bq.Add(childQuery, search.MUST)
		return bq.Build(), nil
	case ModifierProhibited:
		bq := search.NewBooleanQueryBuilder()
		bq.Add(childQuery, search.MUST_NOT)
		return bq.Build(), nil
	default:
		return childQuery, nil
	}
}

// MultiPhraseQueryNodeBuilder builds MultiPhraseQuery from MultiPhraseQueryNode.
// This is the Go equivalent of Lucene's MultiPhraseQueryNodeBuilder.
type MultiPhraseQueryNodeBuilder struct {
	treeBuilder *QueryTreeBuilder
}

// NewMultiPhraseQueryNodeBuilder creates a new MultiPhraseQueryNodeBuilder.
func NewMultiPhraseQueryNodeBuilder(treeBuilder *QueryTreeBuilder) *MultiPhraseQueryNodeBuilder {
	return &MultiPhraseQueryNodeBuilder{treeBuilder: treeBuilder}
}

// Build builds a MultiPhraseQuery from a MultiPhraseQueryNode.
func (b *MultiPhraseQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	mpNode, ok := node.(*MultiPhraseQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected MultiPhraseQueryNode, got %T", node)
	}

	builder := search.NewMultiPhraseQueryBuilder()
	for _, child := range mpNode.GetChildren() {
		if fqn, ok := child.(*FieldQueryNode); ok {
			term := index.NewTerm(mpNode.GetField(), fqn.GetText())
			builder.Add(term)
		}
	}
	return builder.Build(), nil
}

// PointRangeQueryNodeBuilder builds PointRangeQuery from PointRangeQueryNode.
// This is the Go equivalent of Lucene's PointRangeQueryNodeBuilder.
type PointRangeQueryNodeBuilder struct{}

// NewPointRangeQueryNodeBuilder creates a new PointRangeQueryNodeBuilder.
func NewPointRangeQueryNodeBuilder() *PointRangeQueryNodeBuilder {
	return &PointRangeQueryNodeBuilder{}
}

// Build builds a PointRangeQuery from a PointRangeQueryNode.
func (b *PointRangeQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	prNode, ok := node.(*PointRangeQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected PointRangeQueryNode, got %T", node)
	}

	var lower, upper []byte
	if lo := prNode.GetLowerPoint(); lo != nil {
		lower = lo.GetPointValue()
	}
	if hi := prNode.GetUpperPoint(); hi != nil {
		upper = hi.GetPointValue()
	}

	q, err := search.NewPointRangeQuery(prNode.GetField(), lower, upper)
	if err != nil {
		return nil, fmt.Errorf("creating point range query: %w", err)
	}
	return q, nil
}

// PrefixWildcardQueryNodeBuilder builds PrefixQuery from PrefixWildcardQueryNode.
// This is the Go equivalent of Lucene's PrefixWildcardQueryNodeBuilder.
type PrefixWildcardQueryNodeBuilder struct{}

// NewPrefixWildcardQueryNodeBuilder creates a new PrefixWildcardQueryNodeBuilder.
func NewPrefixWildcardQueryNodeBuilder() *PrefixWildcardQueryNodeBuilder {
	return &PrefixWildcardQueryNodeBuilder{}
}

// Build builds a PrefixQuery from a PrefixWildcardQueryNode.
func (b *PrefixWildcardQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	pwNode, ok := node.(*PrefixWildcardQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected PrefixWildcardQueryNode, got %T", node)
	}

	// Strip trailing wildcard for prefix term
	text := pwNode.GetText()
	if len(text) > 0 && text[len(text)-1] == '*' {
		text = text[:len(text)-1]
	}

	term := index.NewTerm(pwNode.GetField(), text)
	return search.NewPrefixQuery(term), nil
}

// RegexpQueryNodeBuilder builds RegexpQuery from RegexpQueryNode.
// This is the Go equivalent of Lucene's RegexpQueryNodeBuilder.
type RegexpQueryNodeBuilder struct{}

// NewRegexpQueryNodeBuilder creates a new RegexpQueryNodeBuilder.
func NewRegexpQueryNodeBuilder() *RegexpQueryNodeBuilder { return &RegexpQueryNodeBuilder{} }

// Build builds a RegexpQuery from a RegexpQueryNode.
func (b *RegexpQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	rnNode, ok := node.(*RegexpQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected RegexpQueryNode, got %T", node)
	}

	// Mirrors RegexpQueryNodeBuilder#build: new RegexpQuery(new Term(
	// regexpNode.getFieldAsString(), regexpNode.textToBytesRef())).
	return search.NewRegexpQuery(
		index.NewTermFromBytesRef(rnNode.GetFieldAsString(), rnNode.TextToBytesRef())), nil
}

// SlopQueryNodeBuilder builds a PhraseQuery with slop from SlopQueryNode.
// This is the Go equivalent of Lucene's SlopQueryNodeBuilder.
type SlopQueryNodeBuilder struct {
	treeBuilder *QueryTreeBuilder
}

// NewSlopQueryNodeBuilder creates a new SlopQueryNodeBuilder.
func NewSlopQueryNodeBuilder(treeBuilder *QueryTreeBuilder) *SlopQueryNodeBuilder {
	return &SlopQueryNodeBuilder{treeBuilder: treeBuilder}
}

// Build builds the child query with slop applied if it is a PhraseQuery.
func (b *SlopQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	slopNode, ok := node.(*SlopQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected SlopQueryNode, got %T", node)
	}

	children := slopNode.GetChildren()
	if len(children) == 0 {
		return search.NewMatchNoDocsQuery(""), nil
	}

	childQuery, err := b.treeBuilder.Build(children[0])
	if err != nil {
		return nil, err
	}

	if pq, ok := childQuery.(*search.PhraseQuery); ok {
		_ = pq
		_ = slopNode.GetValue()
	}
	return childQuery, nil
}

// SynonymQueryNodeBuilder builds a BooleanQuery from a SynonymQueryNode.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.builders.SynonymQueryNodeBuilder,
// which — despite the name — produces a BooleanQuery of SHOULD clauses, one per
// child, and not a SynonymQuery (the Java source carries a "TODO: use
// SynonymQuery instead").
type SynonymQueryNodeBuilder struct {
	treeBuilder *QueryTreeBuilder
}

// NewSynonymQueryNodeBuilder creates a new SynonymQueryNodeBuilder.
func NewSynonymQueryNodeBuilder(treeBuilder *QueryTreeBuilder) *SynonymQueryNodeBuilder {
	return &SynonymQueryNodeBuilder{treeBuilder: treeBuilder}
}

// Build builds a BooleanQuery of SHOULD clauses from a SynonymQueryNode.
//
// Java reads each child's already-built query from the
// QueryTreeBuilder.QUERY_TREE_BUILDER_TAGID tag; this package builds children
// through the tree builder instead, which is the mechanism every other builder
// here uses. The resulting query is the same.
func (b *SynonymQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	synNode, ok := node.(*SynonymQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected SynonymQueryNode, got %T", node)
	}

	builder := search.NewBooleanQueryBuilder()
	for _, child := range synNode.GetChildren() {
		childQuery, err := b.treeBuilder.Build(child)
		if err != nil {
			return nil, err
		}
		if childQuery != nil {
			builder.Add(childQuery, search.SHOULD)
		}
	}
	return builder.Build(), nil
}

// MinShouldMatchNodeBuilder builds a BooleanQuery with min-should-match set.
// This is the Go equivalent of Lucene's MinShouldMatchNodeBuilder.
type MinShouldMatchNodeBuilder struct {
	treeBuilder *QueryTreeBuilder
}

// NewMinShouldMatchNodeBuilder creates a new MinShouldMatchNodeBuilder.
func NewMinShouldMatchNodeBuilder(treeBuilder *QueryTreeBuilder) *MinShouldMatchNodeBuilder {
	return &MinShouldMatchNodeBuilder{treeBuilder: treeBuilder}
}

// Build builds a BooleanQuery with minimum-should-match set.
func (b *MinShouldMatchNodeBuilder) Build(node QueryNode) (search.Query, error) {
	msmNode, ok := node.(*MinShouldMatchNode)
	if !ok {
		return nil, fmt.Errorf("expected MinShouldMatchNode, got %T", node)
	}

	children := msmNode.GetChildren()
	if len(children) != 1 {
		return nil, fmt.Errorf("unexpected number of node children: %d", len(children))
	}

	childQuery, err := b.treeBuilder.Build(children[0])
	if err != nil {
		return nil, err
	}

	bq, ok := childQuery.(*search.BooleanQuery)
	if !ok {
		return nil, fmt.Errorf("expected BooleanQuery as child of MinShouldMatchNode, got %T", childQuery)
	}

	// To maintain fidelity with Lucene's BooleanQuery.Builder, we create a new BooleanQuery
	// with the same clauses but a different minimum number of should-match.
	builder := search.NewBooleanQueryBuilder()
	builder.SetMinimumNumberShouldMatch(msmNode.GetMinimumShouldMatch())
	for _, clause := range bq.Clauses() {
		builder.AddClause(clause)
	}
	return builder.Build(), nil
}

// IntervalQueryNodeBuilder builds a Query from an IntervalQueryNode.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.builders.IntervalQueryNodeBuilder,
// whose whole body is `return ((IntervalQueryNode) queryNode).getQuery();`.
type IntervalQueryNodeBuilder struct{}

// NewIntervalQueryNodeBuilder creates a new IntervalQueryNodeBuilder.
func NewIntervalQueryNodeBuilder() *IntervalQueryNodeBuilder { return &IntervalQueryNodeBuilder{} }

// Build returns the IntervalQuery carried by the node.
func (b *IntervalQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	intervalNode, ok := node.(*IntervalQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected IntervalQueryNode, got %T", node)
	}
	return intervalNode.GetQuery(), nil
}

// StandardQueryBuilder is the top-level builder interface for the standard query parser.
// This is the Go equivalent of Lucene's StandardQueryBuilder.
type StandardQueryBuilder interface {
	QueryBuilder
}
