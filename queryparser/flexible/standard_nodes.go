// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strconv"
	"strings"
)

// AbstractRangeQueryNode is the base for typed range query nodes.
// It carries lower and upper FieldQueryNode children plus inclusivity flags.
// This is the Go equivalent of Lucene's AbstractRangeQueryNode.
type AbstractRangeQueryNode struct {
	*QueryNodeImpl
	field          string
	lowerInclusive bool
	upperInclusive bool
}

// newAbstractRangeQueryNode creates an initialised AbstractRangeQueryNode.
func newAbstractRangeQueryNode(field string, lower, upper QueryNode, lowerInclusive, upperInclusive bool) *AbstractRangeQueryNode {
	children := make([]QueryNode, 0, 2)
	if lower != nil {
		children = append(children, lower)
	}
	if upper != nil {
		children = append(children, upper)
	}
	return &AbstractRangeQueryNode{
		QueryNodeImpl:  NewQueryNodeImpl(children),
		field:          field,
		lowerInclusive: lowerInclusive,
		upperInclusive: upperInclusive,
	}
}

// GetField returns the field name.
func (n *AbstractRangeQueryNode) GetField() string { return n.field }

// SetField sets the field name.
func (n *AbstractRangeQueryNode) SetField(field string) { n.field = field }

// IsLowerInclusive reports whether the lower bound is inclusive.
func (n *AbstractRangeQueryNode) IsLowerInclusive() bool { return n.lowerInclusive }

// IsUpperInclusive reports whether the upper bound is inclusive.
func (n *AbstractRangeQueryNode) IsUpperInclusive() bool { return n.upperInclusive }

// getLower returns the lower bound child, or nil.
func (n *AbstractRangeQueryNode) getLower() QueryNode {
	children := n.GetChildren()
	if len(children) > 0 {
		return children[0]
	}
	return nil
}

// getUpper returns the upper bound child, or nil.
func (n *AbstractRangeQueryNode) getUpper() QueryNode {
	children := n.GetChildren()
	if len(children) > 1 {
		return children[1]
	}
	return nil
}

// TermRangeQueryNode is an AbstractRangeQueryNode for lexicographic term ranges.
// This is the Go equivalent of Lucene's TermRangeQueryNode.
type TermRangeQueryNode struct {
	*AbstractRangeQueryNode
}

// NewTermRangeQueryNode creates a new TermRangeQueryNode.
func NewTermRangeQueryNode(field string, lower, upper QueryNode, lowerInclusive, upperInclusive bool) *TermRangeQueryNode {
	return &TermRangeQueryNode{
		AbstractRangeQueryNode: newAbstractRangeQueryNode(field, lower, upper, lowerInclusive, upperInclusive),
	}
}

// ToQueryString emits field:[lower TO upper] or field:{lower TO upper}.
func (n *TermRangeQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	if n.field != "" {
		sb.WriteString(n.field)
		sb.WriteString(":")
	}
	if n.lowerInclusive {
		sb.WriteRune('[')
	} else {
		sb.WriteRune('{')
	}
	if lo := n.getLower(); lo != nil {
		sb.WriteString(lo.ToQueryString(escapeSpecialSyntax))
	} else {
		sb.WriteString("*")
	}
	sb.WriteString(" TO ")
	if hi := n.getUpper(); hi != nil {
		sb.WriteString(hi.ToQueryString(escapeSpecialSyntax))
	} else {
		sb.WriteString("*")
	}
	if n.upperInclusive {
		sb.WriteRune(']')
	} else {
		sb.WriteRune('}')
	}
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *TermRangeQueryNode) CloneTree() QueryNode {
	var lo, hi QueryNode
	if l := n.getLower(); l != nil {
		lo = l.CloneTree()
	}
	if h := n.getUpper(); h != nil {
		hi = h.CloneTree()
	}
	cloned := &TermRangeQueryNode{
		AbstractRangeQueryNode: newAbstractRangeQueryNode(n.field, lo, hi, n.lowerInclusive, n.upperInclusive),
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *TermRangeQueryNode) String() string {
	return fmt.Sprintf("<term_range field=%s>", n.field)
}

// PointQueryNode represents a single point value node used in point range queries.
// This is the Go equivalent of Lucene's PointQueryNode.
type PointQueryNode struct {
	*FieldQueryNode
	value []byte
}

// NewPointQueryNode creates a new PointQueryNode.
func NewPointQueryNode(field string, value []byte) *PointQueryNode {
	node := &PointQueryNode{
		FieldQueryNode: NewFieldQueryNode(field, string(value), 0, len(value)),
		value:          make([]byte, len(value)),
	}
	copy(node.value, value)
	return node
}

// GetPointValue returns the raw byte value.
func (n *PointQueryNode) GetPointValue() []byte {
	out := make([]byte, len(n.value))
	copy(out, n.value)
	return out
}

// CloneTree deep-copies this node.
func (n *PointQueryNode) CloneTree() QueryNode {
	cloned := NewPointQueryNode(n.GetField(), n.value)
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *PointQueryNode) String() string {
	return fmt.Sprintf("<point field=%s value=%x>", n.GetField(), n.value)
}

// PointRangeQueryNode is an AbstractRangeQueryNode for numeric/binary point ranges.
// This is the Go equivalent of Lucene's PointRangeQueryNode.
type PointRangeQueryNode struct {
	*AbstractRangeQueryNode
}

// NewPointRangeQueryNode creates a new PointRangeQueryNode.
func NewPointRangeQueryNode(field string, lower, upper *PointQueryNode, lowerInclusive, upperInclusive bool) *PointRangeQueryNode {
	return &PointRangeQueryNode{
		AbstractRangeQueryNode: newAbstractRangeQueryNode(field, lower, upper, lowerInclusive, upperInclusive),
	}
}

// GetLowerPoint returns the lower bound PointQueryNode, or nil.
func (n *PointRangeQueryNode) GetLowerPoint() *PointQueryNode {
	if lo := n.getLower(); lo != nil {
		if pqn, ok := lo.(*PointQueryNode); ok {
			return pqn
		}
	}
	return nil
}

// GetUpperPoint returns the upper bound PointQueryNode, or nil.
func (n *PointRangeQueryNode) GetUpperPoint() *PointQueryNode {
	if hi := n.getUpper(); hi != nil {
		if pqn, ok := hi.(*PointQueryNode); ok {
			return pqn
		}
	}
	return nil
}

// ToQueryString emits field:[lower TO upper].
func (n *PointRangeQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	if n.field != "" {
		sb.WriteString(n.field)
		sb.WriteString(":")
	}
	if n.lowerInclusive {
		sb.WriteRune('[')
	} else {
		sb.WriteRune('{')
	}
	if lo := n.getLower(); lo != nil {
		sb.WriteString(lo.ToQueryString(escapeSpecialSyntax))
	} else {
		sb.WriteString("*")
	}
	sb.WriteString(" TO ")
	if hi := n.getUpper(); hi != nil {
		sb.WriteString(hi.ToQueryString(escapeSpecialSyntax))
	} else {
		sb.WriteString("*")
	}
	if n.upperInclusive {
		sb.WriteRune(']')
	} else {
		sb.WriteRune('}')
	}
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *PointRangeQueryNode) CloneTree() QueryNode {
	var lo, hi *PointQueryNode
	if l := n.GetLowerPoint(); l != nil {
		lo = l.CloneTree().(*PointQueryNode)
	}
	if h := n.GetUpperPoint(); h != nil {
		hi = h.CloneTree().(*PointQueryNode)
	}
	cloned := NewPointRangeQueryNode(n.field, lo, hi, n.lowerInclusive, n.upperInclusive)
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *PointRangeQueryNode) String() string {
	return fmt.Sprintf("<point_range field=%s>", n.field)
}

// PrefixWildcardQueryNode represents a prefix query (e.g. "foo*").
// This is the Go equivalent of Lucene's PrefixWildcardQueryNode.
type PrefixWildcardQueryNode struct {
	*FieldQueryNode
}

// NewPrefixWildcardQueryNode creates a new PrefixWildcardQueryNode.
func NewPrefixWildcardQueryNode(field, text string, begin, end int) *PrefixWildcardQueryNode {
	return &PrefixWildcardQueryNode{FieldQueryNode: NewFieldQueryNode(field, text, begin, end)}
}

// ToQueryString emits field:prefix*.
func (n *PrefixWildcardQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	if n.GetField() != "" {
		sb.WriteString(n.GetField())
		sb.WriteString(":")
	}
	text := n.GetText()
	if escapeSpecialSyntax {
		text = escapeQueryString(text)
	}
	sb.WriteString(text)
	if !strings.HasSuffix(text, "*") {
		sb.WriteString("*")
	}
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *PrefixWildcardQueryNode) CloneTree() QueryNode {
	cloned := &PrefixWildcardQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *PrefixWildcardQueryNode) String() string {
	return fmt.Sprintf("<prefix field=%s text=%s>", n.GetField(), n.GetText())
}

// WildcardQueryNode represents a wildcard query (contains * or ?).
// This is the Go equivalent of Lucene's WildcardQueryNode.
type WildcardQueryNode struct {
	*FieldQueryNode
}

// NewWildcardQueryNode creates a new WildcardQueryNode.
func NewWildcardQueryNode(field, text string, begin, end int) *WildcardQueryNode {
	return &WildcardQueryNode{FieldQueryNode: NewFieldQueryNode(field, text, begin, end)}
}

// CloneTree deep-copies this node.
func (n *WildcardQueryNode) CloneTree() QueryNode {
	cloned := &WildcardQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *WildcardQueryNode) String() string {
	return fmt.Sprintf("<wildcard field=%s text=%s>", n.GetField(), n.GetText())
}

// RegexpQueryNode represents a regular-expression query.
// This is the Go equivalent of Lucene's RegexpQueryNode.
type RegexpQueryNode struct {
	*FieldQueryNode
}

// NewRegexpQueryNode creates a new RegexpQueryNode.
func NewRegexpQueryNode(field, text string, begin, end int) *RegexpQueryNode {
	return &RegexpQueryNode{FieldQueryNode: NewFieldQueryNode(field, text, begin, end)}
}

// ToQueryString emits field:/regexp/.
func (n *RegexpQueryNode) ToQueryString(_ bool) string {
	var sb strings.Builder
	if n.GetField() != "" {
		sb.WriteString(n.GetField())
		sb.WriteString(":")
	}
	sb.WriteRune('/')
	sb.WriteString(n.GetText())
	sb.WriteRune('/')
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *RegexpQueryNode) CloneTree() QueryNode {
	cloned := &RegexpQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *RegexpQueryNode) String() string {
	return fmt.Sprintf("<regexp field=%s text=%s>", n.GetField(), n.GetText())
}

// SynonymQueryNode holds multiple synonymous terms for a single field position.
// This is the Go equivalent of Lucene's SynonymQueryNode.
type SynonymQueryNode struct {
	*QueryNodeImpl
	field string
}

// NewSynonymQueryNode creates a new SynonymQueryNode with the given synonym terms.
func NewSynonymQueryNode(field string, synonymNodes []QueryNode) *SynonymQueryNode {
	return &SynonymQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(synonymNodes),
		field:         field,
	}
}

// GetField returns the field name.
func (n *SynonymQueryNode) GetField() string { return n.field }

// SetField sets the field name.
func (n *SynonymQueryNode) SetField(field string) { n.field = field }

// ToQueryString emits (syn1 OR syn2 ...).
func (n *SynonymQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	sb.WriteRune('(')
	for i, child := range n.GetChildren() {
		if i > 0 {
			sb.WriteString(" OR ")
		}
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}
	sb.WriteRune(')')
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *SynonymQueryNode) CloneTree() QueryNode {
	cloned := &SynonymQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		field:         n.field,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}
	return cloned
}

// String returns a debug representation.
func (n *SynonymQueryNode) String() string {
	return fmt.Sprintf("<synonym field=%s terms=%d>", n.field, len(n.GetChildren()))
}

// BooleanModifierNode wraps a child boolean node and attaches a modifier.
// This is the Go equivalent of Lucene's BooleanModifierNode.
type BooleanModifierNode struct {
	*ModifierQueryNode
}

// NewBooleanModifierNode creates a new BooleanModifierNode.
func NewBooleanModifierNode(child QueryNode, modifier Modifier) *BooleanModifierNode {
	return &BooleanModifierNode{ModifierQueryNode: NewModifierQueryNode(child, modifier)}
}

// CloneTree deep-copies this node.
func (n *BooleanModifierNode) CloneTree() QueryNode {
	cloned := &BooleanModifierNode{ModifierQueryNode: &ModifierQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		modifier:      n.modifier,
	}}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}
	return cloned
}

// MinShouldMatchNode wraps a boolean query node and carries a minimum-should-match count.
// This is the Go equivalent of Lucene's MinShouldMatchNode.
type MinShouldMatchNode struct {
	*QueryNodeImpl
	minimumShouldMatch int
}

// NewMinShouldMatchNode creates a new MinShouldMatchNode.
func NewMinShouldMatchNode(child QueryNode, minimumShouldMatch int) *MinShouldMatchNode {
	node := &MinShouldMatchNode{
		QueryNodeImpl:      NewQueryNodeImpl(nil),
		minimumShouldMatch: minimumShouldMatch,
	}
	if child != nil {
		node.AddChild(child)
	}
	return node
}

// GetMinimumShouldMatch returns the minimum-should-match value.
func (n *MinShouldMatchNode) GetMinimumShouldMatch() int { return n.minimumShouldMatch }

// SetMinimumShouldMatch sets the minimum-should-match value.
func (n *MinShouldMatchNode) SetMinimumShouldMatch(v int) { n.minimumShouldMatch = v }

// ToQueryString appends @N to child representation.
func (n *MinShouldMatchNode) ToQueryString(escapeSpecialSyntax bool) string {
	children := n.GetChildren()
	if len(children) == 0 {
		return ""
	}
	return children[0].ToQueryString(escapeSpecialSyntax) + "@" + strconv.Itoa(n.minimumShouldMatch)
}

// CloneTree deep-copies this node.
func (n *MinShouldMatchNode) CloneTree() QueryNode {
	cloned := &MinShouldMatchNode{
		QueryNodeImpl:      NewQueryNodeImpl(nil),
		minimumShouldMatch: n.minimumShouldMatch,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}
	return cloned
}

// String returns a debug representation.
func (n *MinShouldMatchNode) String() string {
	return fmt.Sprintf("<min_should_match min=%d>", n.minimumShouldMatch)
}

// MultiPhraseQueryNode holds multiple alternative token arrays for each position,
// forming a multi-phrase query.
// This is the Go equivalent of Lucene's MultiPhraseQueryNode.
type MultiPhraseQueryNode struct {
	*QueryNodeImpl
	field string
}

// NewMultiPhraseQueryNode creates a new MultiPhraseQueryNode.
func NewMultiPhraseQueryNode(field string, children []QueryNode) *MultiPhraseQueryNode {
	return &MultiPhraseQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(children),
		field:         field,
	}
}

// GetField returns the field name.
func (n *MultiPhraseQueryNode) GetField() string { return n.field }

// SetField sets the field name.
func (n *MultiPhraseQueryNode) SetField(field string) { n.field = field }

// ToQueryString emits a phrase-like representation.
func (n *MultiPhraseQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	if n.field != "" {
		sb.WriteString(n.field)
		sb.WriteString(":")
	}
	sb.WriteRune('"')
	for i, child := range n.GetChildren() {
		if i > 0 {
			sb.WriteRune(' ')
		}
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}
	sb.WriteRune('"')
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *MultiPhraseQueryNode) CloneTree() QueryNode {
	cloned := &MultiPhraseQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		field:         n.field,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}
	return cloned
}

// String returns a debug representation.
func (n *MultiPhraseQueryNode) String() string {
	return fmt.Sprintf("<multi_phrase field=%s positions=%d>", n.field, len(n.GetChildren()))
}

// IntervalQueryNode represents an interval function query.
// This is the Go equivalent of Lucene's IntervalQueryNode.
type IntervalQueryNode struct {
	*QueryNodeImpl
	field    string
	function string
}

// NewIntervalQueryNode creates a new IntervalQueryNode.
func NewIntervalQueryNode(field, function string, children []QueryNode) *IntervalQueryNode {
	return &IntervalQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(children),
		field:         field,
		function:      function,
	}
}

// GetField returns the field name.
func (n *IntervalQueryNode) GetField() string { return n.field }

// SetField sets the field name.
func (n *IntervalQueryNode) SetField(field string) { n.field = field }

// GetFunction returns the interval function name.
func (n *IntervalQueryNode) GetFunction() string { return n.function }

// ToQueryString emits fn(field, child1, child2, ...).
func (n *IntervalQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	sb.WriteString(n.function)
	sb.WriteRune('(')
	sb.WriteString(n.field)
	for _, child := range n.GetChildren() {
		sb.WriteString(", ")
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}
	sb.WriteRune(')')
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *IntervalQueryNode) CloneTree() QueryNode {
	cloned := &IntervalQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		field:         n.field,
		function:      n.function,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}
	return cloned
}

// String returns a debug representation.
func (n *IntervalQueryNode) String() string {
	return fmt.Sprintf("<interval field=%s fn=%s>", n.field, n.function)
}
