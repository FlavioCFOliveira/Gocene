// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AllowLeadingWildcardProcessor rejects queries whose term begins with a wildcard
// when the config does not allow leading wildcards.
// This is the Go equivalent of Lucene's AllowLeadingWildcardProcessor.
type AllowLeadingWildcardProcessor struct {
	*QueryNodeProcessorImpl
	config *StandardQueryConfigHandler
}

// NewAllowLeadingWildcardProcessor creates a new AllowLeadingWildcardProcessor.
func NewAllowLeadingWildcardProcessor(config *StandardQueryConfigHandler) *AllowLeadingWildcardProcessor {
	return &AllowLeadingWildcardProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
		config:                 config,
	}
}

// Process walks the tree and raises an error for leading wildcards when disallowed.
func (p *AllowLeadingWildcardProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	if !p.config.IsAllowLeadingWildcard() {
		switch n := queryTree.(type) {
		case *WildcardQueryNode:
			txt := n.GetText()
			if len(txt) > 0 && (txt[0] == '*' || txt[0] == '?') {
				return nil, &QueryNodeParseException{
					QueryNodeException: &QueryNodeException{
						message: NewMessageImpl(MsgInvalidSyntax, "leading wildcard not allowed: "+txt),
					},
				}
			}
		case *PrefixWildcardQueryNode:
			txt := n.GetText()
			if len(txt) > 0 && (txt[0] == '*' || txt[0] == '?') {
				return nil, &QueryNodeParseException{
					QueryNodeException: &QueryNodeException{
						message: NewMessageImpl(MsgInvalidSyntax, "leading wildcard not allowed: "+txt),
					},
				}
			}
		}
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// BooleanQuery2ModifierNodeProcessor wraps bare boolean children in ModifierQueryNodes
// according to the configured default operator (AND/OR).
// This is the Go equivalent of Lucene's BooleanQuery2ModifierNodeProcessor.
type BooleanQuery2ModifierNodeProcessor struct {
	*QueryNodeProcessorImpl
	config *StandardQueryConfigHandler
}

// NewBooleanQuery2ModifierNodeProcessor creates a new BooleanQuery2ModifierNodeProcessor.
func NewBooleanQuery2ModifierNodeProcessor(config *StandardQueryConfigHandler) *BooleanQuery2ModifierNodeProcessor {
	return &BooleanQuery2ModifierNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
		config:                 config,
	}
}

// Process walks the tree and wraps unmodified children in the correct modifier.
func (p *BooleanQuery2ModifierNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	if _, ok := queryTree.(*BooleanQueryNode); ok {
		children := queryTree.GetChildren()
		for i, child := range children {
			if _, alreadyMod := child.(*ModifierQueryNode); !alreadyMod {
				if _, alreadyBoolMod := child.(*BooleanModifierNode); !alreadyBoolMod {
					var mod Modifier
					if p.config.GetDefaultOperator() == "AND" {
						mod = ModifierRequired
					} else {
						mod = ModifierNone
					}
					modNode := NewModifierQueryNode(child, mod)
					children[i] = modNode
				}
			}
		}
		queryTree.SetChildren(children)
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// BooleanSingleChildOptimizationQueryNodeProcessor unwraps boolean nodes that have
// exactly one child, replacing them with the child directly.
// This is the Go equivalent of Lucene's BooleanSingleChildOptimizationQueryNodeProcessor.
type BooleanSingleChildOptimizationQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewBooleanSingleChildOptimizationQueryNodeProcessor creates a new instance.
func NewBooleanSingleChildOptimizationQueryNodeProcessor() *BooleanSingleChildOptimizationQueryNodeProcessor {
	return &BooleanSingleChildOptimizationQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
	}
}

// Process collapses single-child boolean nodes.
func (p *BooleanSingleChildOptimizationQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	// First recurse
	children := queryTree.GetChildren()
	for i, child := range children {
		processed, err := p.Process(child)
		if err != nil {
			return nil, err
		}
		children[i] = processed
	}
	queryTree.SetChildren(children)

	if _, ok := queryTree.(*BooleanQueryNode); ok {
		ch := queryTree.GetChildren()
		if len(ch) == 1 {
			return ch[0], nil
		}
	}
	return queryTree, nil
}

// DefaultPhraseSlopQueryNodeProcessor sets the default phrase slop on PhraseQueryNodes
// that do not already have a slop set.
// This is the Go equivalent of Lucene's DefaultPhraseSlopQueryNodeProcessor.
type DefaultPhraseSlopQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
	config *StandardQueryConfigHandler
}

// NewDefaultPhraseSlopQueryNodeProcessor creates a new DefaultPhraseSlopQueryNodeProcessor.
func NewDefaultPhraseSlopQueryNodeProcessor(config *StandardQueryConfigHandler) *DefaultPhraseSlopQueryNodeProcessor {
	return &DefaultPhraseSlopQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
		config:                 config,
	}
}

// Process sets default phrase slop on PhraseSlopQueryNode children.
func (p *DefaultPhraseSlopQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	if n, ok := queryTree.(*PhraseSlopQueryNode); ok {
		if n.GetValue() == 0 {
			n.SetValue(p.config.GetPhraseSlop())
		}
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// FuzzyQueryNodeProcessor converts FuzzyQueryNodes to search.FuzzyQuery.
// This is the Go equivalent of Lucene's FuzzyQueryNodeProcessor.
type FuzzyQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
	config *StandardQueryConfigHandler
}

// NewFuzzyQueryNodeProcessor creates a new FuzzyQueryNodeProcessor.
func NewFuzzyQueryNodeProcessor(config *StandardQueryConfigHandler) *FuzzyQueryNodeProcessor {
	return &FuzzyQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
		config:                 config,
	}
}

// Process resolves FuzzyQueryNodes.
func (p *FuzzyQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// IntervalQueryNodeProcessor processes IntervalQueryNode trees.
// This is the Go equivalent of Lucene's IntervalQueryNodeProcessor.
type IntervalQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewIntervalQueryNodeProcessor creates a new IntervalQueryNodeProcessor.
func NewIntervalQueryNodeProcessor() *IntervalQueryNodeProcessor {
	return &IntervalQueryNodeProcessor{QueryNodeProcessorImpl: NewQueryNodeProcessorImpl()}
}

// Process walks the tree; interval nodes are handled by the builder.
func (p *IntervalQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// MatchAllDocsQueryNodeProcessor converts MatchAllDocsQueryNode to the appropriate form.
// This is the Go equivalent of Lucene's MatchAllDocsQueryNodeProcessor.
type MatchAllDocsQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewMatchAllDocsQueryNodeProcessor creates a new MatchAllDocsQueryNodeProcessor.
func NewMatchAllDocsQueryNodeProcessor() *MatchAllDocsQueryNodeProcessor {
	return &MatchAllDocsQueryNodeProcessor{QueryNodeProcessorImpl: NewQueryNodeProcessorImpl()}
}

// Process is a no-op pass-through (MatchAllDocsQueryNode is handled in builder).
func (p *MatchAllDocsQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// MultiFieldQueryNodeProcessor expands a bare term query across multiple default fields.
// This is the Go equivalent of Lucene's MultiFieldQueryNodeProcessor.
type MultiFieldQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
	fields []string
}

// NewMultiFieldQueryNodeProcessor creates a new MultiFieldQueryNodeProcessor.
func NewMultiFieldQueryNodeProcessor(fields []string) *MultiFieldQueryNodeProcessor {
	return &MultiFieldQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
		fields:                 fields,
	}
}

// Process expands FieldQueryNodes with no field into OR-of-fields expansions.
func (p *MultiFieldQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil || len(p.fields) == 0 {
		return queryTree, nil
	}
	// Recurse children first
	children := queryTree.GetChildren()
	for i, child := range children {
		processed, err := p.Process(child)
		if err != nil {
			return nil, err
		}
		children[i] = processed
	}
	queryTree.SetChildren(children)

	if n, ok := queryTree.(*FieldQueryNode); ok && n.GetField() == "" {
		if len(p.fields) == 1 {
			n.SetField(p.fields[0])
			return n, nil
		}
		nodes := make([]QueryNode, len(p.fields))
		for i, f := range p.fields {
			clone := n.CloneTree().(*FieldQueryNode)
			clone.SetField(f)
			nodes[i] = clone
		}
		return NewOrQueryNode(nodes), nil
	}
	return queryTree, nil
}

// MultiTermRewriteMethodProcessor sets the rewrite method on multi-term query nodes.
// This is the Go equivalent of Lucene's MultiTermRewriteMethodProcessor.
type MultiTermRewriteMethodProcessor struct {
	*QueryNodeProcessorImpl
	rewriteMethod string
}

// NewMultiTermRewriteMethodProcessor creates a new MultiTermRewriteMethodProcessor.
func NewMultiTermRewriteMethodProcessor(rewriteMethod string) *MultiTermRewriteMethodProcessor {
	if rewriteMethod == "" {
		rewriteMethod = "constant_score"
	}
	return &MultiTermRewriteMethodProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
		rewriteMethod:          rewriteMethod,
	}
}

// Process tags multi-term nodes with the rewrite method.
func (p *MultiTermRewriteMethodProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	switch queryTree.(type) {
	case *WildcardQueryNode, *PrefixWildcardQueryNode, *RegexpQueryNode, *FuzzyQueryNode:
		queryTree.SetTag("rewriteMethod", p.rewriteMethod)
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// OpenRangeQueryNodeProcessor converts open-ended range query nodes by replacing
// nil bounds with the appropriate sentinel.
// This is the Go equivalent of Lucene's OpenRangeQueryNodeProcessor.
type OpenRangeQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewOpenRangeQueryNodeProcessor creates a new OpenRangeQueryNodeProcessor.
func NewOpenRangeQueryNodeProcessor() *OpenRangeQueryNodeProcessor {
	return &OpenRangeQueryNodeProcessor{QueryNodeProcessorImpl: NewQueryNodeProcessorImpl()}
}

// Process is a no-op pass-through; open ranges are handled by the range builder.
func (p *OpenRangeQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// PointQueryNodeProcessor converts PointQueryNode to a form the builder can handle.
// This is the Go equivalent of Lucene's PointQueryNodeProcessor.
type PointQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
	config *StandardQueryConfigHandler
}

// NewPointQueryNodeProcessor creates a new PointQueryNodeProcessor.
func NewPointQueryNodeProcessor(config *StandardQueryConfigHandler) *PointQueryNodeProcessor {
	return &PointQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
		config:                 config,
	}
}

// Process is a pass-through; point queries are built directly by the builder.
func (p *PointQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// PointRangeQueryNodeProcessor converts PointRangeQueryNode bounds to byte encoding.
// This is the Go equivalent of Lucene's PointRangeQueryNodeProcessor.
type PointRangeQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewPointRangeQueryNodeProcessor creates a new PointRangeQueryNodeProcessor.
func NewPointRangeQueryNodeProcessor() *PointRangeQueryNodeProcessor {
	return &PointRangeQueryNodeProcessor{QueryNodeProcessorImpl: NewQueryNodeProcessorImpl()}
}

// Process is a pass-through; point range queries are built directly by the builder.
func (p *PointRangeQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// RegexpQueryNodeProcessor converts RegexpQueryNode into a tagged node for the builder.
// This is the Go equivalent of Lucene's RegexpQueryNodeProcessor.
type RegexpQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewRegexpQueryNodeProcessor creates a new RegexpQueryNodeProcessor.
func NewRegexpQueryNodeProcessor() *RegexpQueryNodeProcessor {
	return &RegexpQueryNodeProcessor{QueryNodeProcessorImpl: NewQueryNodeProcessorImpl()}
}

// Process is a pass-through; regex queries are built by the builder.
func (p *RegexpQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// RemoveEmptyNonLeafQueryNodeProcessor removes non-leaf boolean nodes that have
// no children remaining after processing.
// This is the Go equivalent of Lucene's RemoveEmptyNonLeafQueryNodeProcessor.
type RemoveEmptyNonLeafQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewRemoveEmptyNonLeafQueryNodeProcessor creates a new instance.
func NewRemoveEmptyNonLeafQueryNodeProcessor() *RemoveEmptyNonLeafQueryNodeProcessor {
	return &RemoveEmptyNonLeafQueryNodeProcessor{QueryNodeProcessorImpl: NewQueryNodeProcessorImpl()}
}

// Process removes empty non-leaf boolean nodes.
func (p *RemoveEmptyNonLeafQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	children := queryTree.GetChildren()
	kept := children[:0]
	for _, child := range children {
		processed, err := p.Process(child)
		if err != nil {
			return nil, err
		}
		if processed != nil {
			kept = append(kept, processed)
		}
	}
	queryTree.SetChildren(kept)

	if _, ok := queryTree.(*BooleanQueryNode); ok {
		if len(queryTree.GetChildren()) == 0 {
			return nil, nil
		}
	}
	return queryTree, nil
}

// TermRangeQueryNodeProcessor converts TermRangeQueryNode bounds to proper byte form.
// This is the Go equivalent of Lucene's TermRangeQueryNodeProcessor.
type TermRangeQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewTermRangeQueryNodeProcessor creates a new TermRangeQueryNodeProcessor.
func NewTermRangeQueryNodeProcessor() *TermRangeQueryNodeProcessor {
	return &TermRangeQueryNodeProcessor{QueryNodeProcessorImpl: NewQueryNodeProcessorImpl()}
}

// Process is a pass-through; range queries are built by the builder.
func (p *TermRangeQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// WildcardQueryNodeProcessor lowercases wildcard query terms when configured.
// This is the Go equivalent of Lucene's WildcardQueryNodeProcessor.
type WildcardQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
	config *StandardQueryConfigHandler
}

// NewWildcardQueryNodeProcessor creates a new WildcardQueryNodeProcessor.
func NewWildcardQueryNodeProcessor(config *StandardQueryConfigHandler) *WildcardQueryNodeProcessor {
	return &WildcardQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
		config:                 config,
	}
}

// Process lowercases wildcard terms when lowercaseExpandedTerms is enabled.
func (p *WildcardQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}
	if p.config.IsLowercaseExpandedTerms() {
		switch n := queryTree.(type) {
		case *WildcardQueryNode:
			n.SetText(strings.ToLower(n.GetText()))
		case *PrefixWildcardQueryNode:
			n.SetText(strings.ToLower(n.GetText()))
		}
	}
	for _, child := range queryTree.GetChildren() {
		if _, err := p.Process(child); err != nil {
			return nil, err
		}
	}
	return queryTree, nil
}

// StandardQueryNodeProcessorPipelineFull is the full standard processor pipeline
// as defined by Lucene's StandardQueryNodeProcessorPipeline.
// This is distinct from the simplified StandardQueryNodeProcessorPipeline in
// standard_query_parser.go which uses a subset of processors.
type StandardQueryNodeProcessorPipelineFull struct {
	*QueryNodeProcessorPipeline
}

// NewStandardQueryNodeProcessorPipelineFull creates the full standard processor pipeline.
func NewStandardQueryNodeProcessorPipelineFull(config *StandardQueryConfigHandler) *StandardQueryNodeProcessorPipelineFull {
	processors := []QueryNodeProcessor{
		NewAllowLeadingWildcardProcessor(config),
		NewMultiTermRewriteMethodProcessor(config.multiTermRewriteMethod),
		NewFuzzyQueryNodeProcessor(config),
		NewBooleanQuery2ModifierNodeProcessor(config),
		NewDefaultPhraseSlopQueryNodeProcessor(config),
		NewBoostQueryNodeProcessor(),
		NewMultiFieldQueryNodeProcessor(nil),
		NewRegexpQueryNodeProcessor(),
		NewWildcardQueryNodeProcessor(config),
		NewBooleanSingleChildOptimizationQueryNodeProcessor(),
		NewOpenRangeQueryNodeProcessor(),
		NewPointQueryNodeProcessor(config),
		NewPointRangeQueryNodeProcessor(),
		NewTermRangeQueryNodeProcessor(),
		NewMatchAllDocsQueryNodeProcessor(),
		NewIntervalQueryNodeProcessor(),
		NewRemoveEmptyNonLeafQueryNodeProcessor(),
		NewRemoveDeletedQueryNodesProcessor(),
		NewNoChildOptimizationProcessor(),
		NewPhraseSlopQueryNodeProcessor(),
	}
	return &StandardQueryNodeProcessorPipelineFull{
		QueryNodeProcessorPipeline: NewQueryNodeProcessorPipeline(processors),
	}
}

// ---- Point encoding helpers used by builders ----

// encodeInt encodes an int32 value to a 4-byte big-endian byte slice.
func encodeInt(v int32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(v))
	return b
}

// encodeLong encodes an int64 value to an 8-byte big-endian byte slice.
func encodeLong(v int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

// encodeFloat encodes a float32 value to a 4-byte big-endian byte slice using
// the same sortable encoding as Lucene's NumericUtils.floatToSortableInt.
func encodeFloat(v float32) []byte {
	bits := math32ToSortable(v)
	return encodeInt(int32(bits))
}

// encodeDouble encodes a float64 value to an 8-byte big-endian byte slice using
// the same sortable encoding as Lucene's NumericUtils.doubleToSortableLong.
func encodeDouble(v float64) []byte {
	bits := math64ToSortable(v)
	return encodeLong(int64(bits))
}

// math32ToSortable converts a float32 bit pattern to a sortable int32 bit pattern.
func math32ToSortable(f float32) uint32 {
	bits := math.Float32bits(f)
	if bits>>31 != 0 {
		bits ^= 0xffffffff
	} else {
		bits ^= 0x80000000
	}
	return bits
}

// math64ToSortable converts a float64 bit pattern to a sortable int64 bit pattern.
func math64ToSortable(f float64) uint64 {
	bits := math.Float64bits(f)
	if bits>>63 != 0 {
		bits ^= 0xffffffffffffffff
	} else {
		bits ^= 0x8000000000000000
	}
	return bits
}

// ---- PointRangeQueryNodeBuilder (standard) ----

// StandardPointRangeQueryNodeBuilder builds PointRangeQuery from a PointRangeQueryNode.
// This is the Go equivalent of Lucene's standard PointRangeQueryNodeBuilder.
type StandardPointRangeQueryNodeBuilder struct{}

// Build constructs a search.PointRangeQuery from a PointRangeQueryNode.
func (b *StandardPointRangeQueryNodeBuilder) Build(queryNode QueryNode) (search.Query, error) {
	n, ok := queryNode.(*PointRangeQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected PointRangeQueryNode, got %T", queryNode)
	}
	var lower, upper *PointQueryNode
	children := n.GetChildren()
	if len(children) >= 2 {
		lower, _ = children[0].(*PointQueryNode)
		upper, _ = children[1].(*PointQueryNode)
	}
	var lo, hi []byte
	if lower != nil {
		lo = lower.GetPointValue()
	}
	if upper != nil {
		hi = upper.GetPointValue()
	}
	return search.NewPointRangeQuery(n.GetField(), lo, hi)
}

// Ensure compile-time interface satisfaction.
var _ QueryBuilder = (*StandardPointRangeQueryNodeBuilder)(nil)

// ---- TermRangeQueryNodeBuilder (standard) ----

// StandardTermRangeQueryNodeBuilder builds TermRangeQuery from a TermRangeQueryNode.
type StandardTermRangeQueryNodeBuilder struct{}

// Build constructs a search.TermRangeQuery from a TermRangeQueryNode.
func (b *StandardTermRangeQueryNodeBuilder) Build(queryNode QueryNode) (search.Query, error) {
	n, ok := queryNode.(*TermRangeQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected TermRangeQueryNode, got %T", queryNode)
	}
	children := n.GetChildren()
	var lo, hi string
	if len(children) >= 1 {
		if fq, ok := children[0].(*FieldQueryNode); ok {
			lo = fq.GetText()
		}
	}
	if len(children) >= 2 {
		if fq, ok := children[1].(*FieldQueryNode); ok {
			hi = fq.GetText()
		}
	}
	q := search.NewTermRangeQueryWithStrings(n.GetField(), lo, hi, n.IsLowerInclusive(), n.IsUpperInclusive())
	return q, nil
}

// Ensure compile-time interface satisfaction.
var _ QueryBuilder = (*StandardTermRangeQueryNodeBuilder)(nil)

// ---- WildcardQueryNodeBuilder (standard) ----

// StandardWildcardQueryNodeBuilder builds WildcardQuery from WildcardQueryNode or PrefixWildcardQueryNode.
type StandardWildcardQueryNodeBuilder struct{}

// Build constructs a search.WildcardQuery or search.PrefixQuery.
func (b *StandardWildcardQueryNodeBuilder) Build(queryNode QueryNode) (search.Query, error) {
	switch n := queryNode.(type) {
	case *PrefixWildcardQueryNode:
		text := n.GetText()
		// strip trailing * for prefix query
		text = strings.TrimRight(text, "*")
		term := index.NewTerm(n.GetField(), text)
		return search.NewPrefixQuery(term), nil
	case *WildcardQueryNode:
		term := index.NewTerm(n.GetField(), n.GetText())
		return search.NewWildcardQuery(term), nil
	default:
		return nil, fmt.Errorf("expected WildcardQueryNode or PrefixWildcardQueryNode, got %T", queryNode)
	}
}

// Ensure compile-time interface satisfaction.
var _ QueryBuilder = (*StandardWildcardQueryNodeBuilder)(nil)

// ---- RegexpQueryNodeBuilder (standard) ----

// StandardRegexpQueryNodeBuilder builds RegexpQuery from a RegexpQueryNode.
type StandardRegexpQueryNodeBuilder struct{}

// Build constructs a search.RegexpQuery.
func (b *StandardRegexpQueryNodeBuilder) Build(queryNode QueryNode) (search.Query, error) {
	n, ok := queryNode.(*RegexpQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected RegexpQueryNode, got %T", queryNode)
	}
	q, err := search.NewRegexpQuery(n.GetField(), n.GetText())
	if err != nil {
		return nil, err
	}
	return q, nil
}

// Ensure compile-time interface satisfaction.
var _ QueryBuilder = (*StandardRegexpQueryNodeBuilder)(nil)

// ---- FuzzyQueryNodeBuilder (standard) ----

// StandardFuzzyQueryNodeBuilder builds FuzzyQuery from a FuzzyQueryNode.
type StandardFuzzyQueryNodeBuilder struct{}

// Build constructs a search.FuzzyQuery.
func (b *StandardFuzzyQueryNodeBuilder) Build(queryNode QueryNode) (search.Query, error) {
	n, ok := queryNode.(*FuzzyQueryNode)
	if !ok {
		return nil, fmt.Errorf("expected FuzzyQueryNode, got %T", queryNode)
	}
	minSim := n.GetMinSimilarity()
	prefixLen := n.GetPrefixLength()

	// Convert similarity to maxEdits (Lucene convention: float < 1 = ratio, >= 1 = integer edits)
	var maxEdits int
	if minSim >= 1.0 {
		maxEdits = int(minSim)
	} else {
		// ratio-based: compute max edits from term length
		text := n.GetText()
		termLen := utf8.RuneCountInString(text)
		maxEdits = int((1.0 - minSim) * float64(termLen))
		if maxEdits > 2 {
			maxEdits = 2
		}
	}
	term := index.NewTerm(n.GetField(), n.GetText())
	return search.NewFuzzyQueryWithParams(term, maxEdits, prefixLen, 50), nil
}

// Ensure compile-time interface satisfaction.
var _ QueryBuilder = (*StandardFuzzyQueryNodeBuilder)(nil)
