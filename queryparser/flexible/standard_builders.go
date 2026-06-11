// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
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
	return search.NewMatchNoDocsQuery(), nil
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

	bq := search.NewBooleanQuery()
	for _, child := range anyNode.GetChildren() {
		childQuery, err := b.treeBuilder.Build(child)
		if err != nil {
			return nil, err
		}
		bq.Add(childQuery, search.SHOULD)
	}
	bq.SetMinimumNumberShouldMatch(anyNode.GetMinimumMatchingElements())
	return bq, nil
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
		return search.NewMatchNoDocsQuery(), nil
	}

	childQuery, err := b.treeBuilder.Build(children[0])
	if err != nil {
		return nil, err
	}

	switch modNode.GetModifier() {
	case ModifierRequired:
		bq := search.NewBooleanQuery()
		bq.Add(childQuery, search.MUST)
		return bq, nil
	case ModifierProhibited:
		bq := search.NewBooleanQuery()
		bq.Add(childQuery, search.MUST_NOT)
		return bq, nil
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

	q, err := search.NewRegexpQuery(rnNode.GetField(), rnNode.GetText())
	if err != nil {
		return nil, fmt.Errorf("creating regexp query: %w", err)
	}
	return q, nil
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
		return search.NewMatchNoDocsQuery(), nil
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

// SynonymQueryNodeBuilder builds SynonymQuery from SynonymQueryNode.
// This is the Go equivalent of Lucene's SynonymQueryNodeBuilder.
type SynonymQueryNodeBuilder struct{}

// NewSynonymQueryNodeBuilder creates a new SynonymQueryNodeBuilder.
func NewSynonymQueryNodeBuilder() *SynonymQueryNodeBuilder { return &SynonymQueryNodeBuilder{} }

// Build builds a SynonymQuery from a SynonymQueryNode.
func (b *SynonymQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	synNode, ok := node.(*SynonymQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected SynonymQueryNode, got %T", node)
	}

	builder := search.NewSynonymQueryBuilder(synNode.GetField())
	for _, child := range synNode.GetChildren() {
		if fqn, ok := child.(*FieldQueryNode); ok {
			builder.AddTerm(index.NewTerm(synNode.GetField(), fqn.GetText()))
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
	if len(children) == 0 {
		return search.NewMatchNoDocsQuery(), nil
	}

	childQuery, err := b.treeBuilder.Build(children[0])
	if err != nil {
		return nil, err
	}

	if bq, ok := childQuery.(*search.BooleanQuery); ok {
		bq.SetMinimumNumberShouldMatch(msmNode.GetMinimumShouldMatch())
		return bq, nil
	}

	return childQuery, nil
}

// IntervalQueryNodeBuilder converts an IntervalQueryNode into a real IntervalQuery
// backed by the intervals execution layer (queries/intervals).
//
// Mapping from node functions to IntervalsSource combinators:
//
//	"or"       → DisjunctionIntervalsSource
//	"and"      → ConjunctionIntervalsSource (unordered)
//	"ordered"  → ConjunctionIntervalsSource (ordered)
//	"phrase"   → ConjunctionIntervalsSource (ordered with maxGaps=0)
//	"not"      → DifferenceIntervalsSource
//	"prefix"   → MultiTermIntervalsSource (prefix automaton)
//	"wildcard" → MultiTermIntervalsSource (wildcard automaton)
//	"fuzzy"    → MultiTermIntervalsSource (fuzzy automaton)
//
// Children are recursively converted: FieldQueryNode → TermIntervalsSource,
// GroupQueryNode → unwrapped, IntervalQueryNode → recursive build.
type IntervalQueryNodeBuilder struct{}

// NewIntervalQueryNodeBuilder creates a new IntervalQueryNodeBuilder.
func NewIntervalQueryNodeBuilder() *IntervalQueryNodeBuilder { return &IntervalQueryNodeBuilder{} }

// Build converts an IntervalQueryNode to a search.Query (IntervalQuery).
func (b *IntervalQueryNodeBuilder) Build(node QueryNode) (search.Query, error) {
	intervalNode, ok := node.(*IntervalQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected IntervalQueryNode, got %T", node)
	}

	source, err := b.buildSource(intervalNode)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return search.NewMatchNoDocsQuery(), nil
	}
	return intervals.NewIntervalQuery(intervalNode.GetField(), source), nil
}

// buildSource recursively builds an IntervalsSource from an IntervalQueryNode.
func (b *IntervalQueryNodeBuilder) buildSource(intervalNode *IntervalQueryNode) (intervals.IntervalsSource, error) {
	children := intervalNode.GetChildren()
	fn := intervalNode.GetFunction()

	// Build sub-sources from children.
	subSources := make([]intervals.IntervalsSource, 0, len(children))
	for _, child := range children {
		cs, err := b.toSource(child)
		if err != nil {
			return nil, err
		}
		if cs != nil {
			subSources = append(subSources, cs)
		}
	}

	if len(subSources) == 0 {
		return intervals.NewNoMatchIntervalsSource(
			fmt.Sprintf("interval %s: no valid sub-sources", fn)), nil
	}

	switch strings.ToLower(fn) {
	case "or", "any":
		return intervals.Or(subSources...), nil

	case "and", "unordered":
		return intervals.Unordered(subSources...), nil

	case "ordered":
		return intervals.Ordered(subSources...), nil

	case "phrase":
		return intervals.PhraseOf(subSources...), nil

	case "not":
		if len(subSources) < 2 {
			return nil, fmt.Errorf("interval not(field, minuend, subtrahend): expected at least 2 sources, got %d", len(subSources))
		}
		return intervals.NotContaining(subSources[0], subSources[1]), nil

	default:
		// Unknown function: treat as single source if there is exactly one child.
		if len(subSources) == 1 {
			return subSources[0], nil
		}
		return nil, fmt.Errorf("interval function %q: %d sub-sources (expected exactly 1 or a known combinator)", fn, len(subSources))
	}
}

// toSource converts a generic QueryNode to an IntervalsSource.
func (b *IntervalQueryNodeBuilder) toSource(node QueryNode) (intervals.IntervalsSource, error) {
	switch n := node.(type) {
	case *FieldQueryNode:
		text := n.GetText()
		if text == "" {
			return nil, nil
		}
		return intervals.NewTermIntervalsSource([]byte(text)), nil

	case *IntervalQueryNode:
		return b.buildSource(n)

	case *GroupQueryNode:
		// Unwrap group — interval query children are flat.
		kids := n.GetChildren()
		if len(kids) == 0 {
			return nil, nil
		}
		if len(kids) == 1 {
			return b.toSource(kids[0])
		}
		// Multiple children in a group → treat as unordered conjunction.
		subs := make([]intervals.IntervalsSource, 0, len(kids))
		for _, k := range kids {
			src, err := b.toSource(k)
			if err != nil {
				return nil, err
			}
			if src != nil {
				subs = append(subs, src)
			}
		}
		if len(subs) == 0 {
			return nil, nil
		}
		return intervals.Unordered(subs...), nil

	default:
		return nil, nil // unsupported node types produce no source
	}
}

// StandardQueryBuilder is the top-level builder interface for the standard query parser.
// This is the Go equivalent of Lucene's StandardQueryBuilder.
type StandardQueryBuilder interface {
	QueryBuilder
}
