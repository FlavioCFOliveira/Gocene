// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package flexible — interval function nodes for the flexible standard query parser.
// These types mirror Lucene's intervalfn sub-package nodes. Each type is a
// lightweight wrapper that holds the function parameters and produces a
// query-string representation. Execution (interval sources) is handled by the
// intervals package and is out of scope here.
package flexible

import (
	"fmt"
	"strconv"
	"strings"
)

// IntervalFunctionQueryNode is the base interface for all interval function nodes.
// It provides the interval function name.
// This is the Go equivalent of Lucene's IntervalFunction.
type IntervalFunctionQueryNode interface {
	QueryNode
	// GetFunctionName returns the interval function name (e.g. "ORDERED").
	GetFunctionName() string
}

// baseIntervalNode is the embedded base for interval function nodes.
type baseIntervalNode struct {
	*QueryNodeImpl
	name string
}

func newBaseIntervalNode(name string, children []QueryNode) *baseIntervalNode {
	return &baseIntervalNode{
		QueryNodeImpl: NewQueryNodeImpl(children),
		name:          name,
	}
}

// GetFunctionName returns the function name.
func (n *baseIntervalNode) GetFunctionName() string { return n.name }

// formatChildren returns "fn(c1, c2, ...)".
func (n *baseIntervalNode) formatChildren(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	sb.WriteString(n.name)
	sb.WriteRune('(')
	for i, child := range n.GetChildren() {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}
	sb.WriteRune(')')
	return sb.String()
}

// OrderedIntervalNode matches the terms in order.
// This is the Go equivalent of Lucene's intervalfn.Ordered.
type OrderedIntervalNode struct{ *baseIntervalNode }

// NewOrderedIntervalNode creates a new OrderedIntervalNode.
func NewOrderedIntervalNode(children []QueryNode) *OrderedIntervalNode {
	return &OrderedIntervalNode{newBaseIntervalNode("ORDERED", children)}
}

// ToQueryString returns ORDERED(c1, c2, ...).
func (n *OrderedIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *OrderedIntervalNode) CloneTree() QueryNode {
	cloned := &OrderedIntervalNode{newBaseIntervalNode("ORDERED", nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, c := range n.GetChildren() {
		cloned.AddChild(c.CloneTree())
	}
	return cloned
}

// String returns debug text.
func (n *OrderedIntervalNode) String() string {
	return fmt.Sprintf("<ordered children=%d>", len(n.GetChildren()))
}

// UnorderedIntervalNode matches the terms in any order.
// This is the Go equivalent of Lucene's intervalfn.Unordered.
type UnorderedIntervalNode struct{ *baseIntervalNode }

// NewUnorderedIntervalNode creates a new UnorderedIntervalNode.
func NewUnorderedIntervalNode(children []QueryNode) *UnorderedIntervalNode {
	return &UnorderedIntervalNode{newBaseIntervalNode("UNORDERED", children)}
}

// ToQueryString returns UNORDERED(c1, c2, ...).
func (n *UnorderedIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *UnorderedIntervalNode) CloneTree() QueryNode {
	cloned := &UnorderedIntervalNode{newBaseIntervalNode("UNORDERED", nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, c := range n.GetChildren() {
		cloned.AddChild(c.CloneTree())
	}
	return cloned
}

// String returns debug text.
func (n *UnorderedIntervalNode) String() string {
	return fmt.Sprintf("<unordered children=%d>", len(n.GetChildren()))
}

// UnorderedNoOverlapsIntervalNode matches terms in any order without overlaps.
// This is the Go equivalent of Lucene's intervalfn.UnorderedNoOverlaps.
type UnorderedNoOverlapsIntervalNode struct{ *baseIntervalNode }

// NewUnorderedNoOverlapsIntervalNode creates a new node.
func NewUnorderedNoOverlapsIntervalNode(children []QueryNode) *UnorderedNoOverlapsIntervalNode {
	return &UnorderedNoOverlapsIntervalNode{newBaseIntervalNode("UNORDERED_NO_OVERLAPS", children)}
}

// ToQueryString returns UNORDERED_NO_OVERLAPS(c1, c2, ...).
func (n *UnorderedNoOverlapsIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *UnorderedNoOverlapsIntervalNode) CloneTree() QueryNode {
	cloned := &UnorderedNoOverlapsIntervalNode{newBaseIntervalNode("UNORDERED_NO_OVERLAPS", nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, c := range n.GetChildren() {
		cloned.AddChild(c.CloneTree())
	}
	return cloned
}

// String returns debug text.
func (n *UnorderedNoOverlapsIntervalNode) String() string { return "<unordered_no_overlaps>" }

// OrIntervalNode matches any of its child intervals.
// This is the Go equivalent of Lucene's intervalfn.Or.
type OrIntervalNode struct{ *baseIntervalNode }

// NewOrIntervalNode creates a new OrIntervalNode.
func NewOrIntervalNode(children []QueryNode) *OrIntervalNode {
	return &OrIntervalNode{newBaseIntervalNode("OR", children)}
}

// ToQueryString returns OR(c1, c2, ...).
func (n *OrIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *OrIntervalNode) CloneTree() QueryNode {
	cloned := &OrIntervalNode{newBaseIntervalNode("OR", nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, c := range n.GetChildren() {
		cloned.AddChild(c.CloneTree())
	}
	return cloned
}

// String returns debug text.
func (n *OrIntervalNode) String() string {
	return fmt.Sprintf("<or_interval children=%d>", len(n.GetChildren()))
}

// PhraseIntervalNode matches terms as a phrase.
// This is the Go equivalent of Lucene's intervalfn.Phrase.
type PhraseIntervalNode struct{ *baseIntervalNode }

// NewPhraseIntervalNode creates a new PhraseIntervalNode.
func NewPhraseIntervalNode(children []QueryNode) *PhraseIntervalNode {
	return &PhraseIntervalNode{newBaseIntervalNode("PHRASE", children)}
}

// ToQueryString returns PHRASE(c1, c2, ...).
func (n *PhraseIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *PhraseIntervalNode) CloneTree() QueryNode {
	cloned := &PhraseIntervalNode{newBaseIntervalNode("PHRASE", nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, c := range n.GetChildren() {
		cloned.AddChild(c.CloneTree())
	}
	return cloned
}

// String returns debug text.
func (n *PhraseIntervalNode) String() string { return "<phrase_interval>" }

// analyticIntervalNode is a base for interval nodes with a numeric parameter.
type analyticIntervalNode struct {
	*baseIntervalNode
	param int
}

func newAnalyticIntervalNode(name string, param int, children []QueryNode) *analyticIntervalNode {
	return &analyticIntervalNode{
		baseIntervalNode: newBaseIntervalNode(name, children),
		param:            param,
	}
}

func (n *analyticIntervalNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	sb.WriteString(n.name)
	sb.WriteRune('(')
	sb.WriteString(strconv.Itoa(n.param))
	for _, child := range n.GetChildren() {
		sb.WriteString(", ")
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}
	sb.WriteRune(')')
	return sb.String()
}

// MaxGapsIntervalNode requires children to match within a maximum gap.
// This is the Go equivalent of Lucene's intervalfn.MaxGaps.
type MaxGapsIntervalNode struct{ *analyticIntervalNode }

// NewMaxGapsIntervalNode creates a new MaxGapsIntervalNode.
func NewMaxGapsIntervalNode(maxGaps int, children []QueryNode) *MaxGapsIntervalNode {
	return &MaxGapsIntervalNode{newAnalyticIntervalNode("MAX_GAPS", maxGaps, children)}
}

// GetMaxGaps returns the maximum gap.
func (n *MaxGapsIntervalNode) GetMaxGaps() int { return n.param }

// CloneTree deep-copies this node.
func (n *MaxGapsIntervalNode) CloneTree() QueryNode {
	cloned := &MaxGapsIntervalNode{newAnalyticIntervalNode("MAX_GAPS", n.param, nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, c := range n.GetChildren() {
		cloned.AddChild(c.CloneTree())
	}
	return cloned
}

// String returns debug text.
func (n *MaxGapsIntervalNode) String() string {
	return fmt.Sprintf("<max_gaps gaps=%d>", n.param)
}

// MaxWidthIntervalNode requires children to match within a maximum width.
// This is the Go equivalent of Lucene's intervalfn.MaxWidth.
type MaxWidthIntervalNode struct{ *analyticIntervalNode }

// NewMaxWidthIntervalNode creates a new MaxWidthIntervalNode.
func NewMaxWidthIntervalNode(maxWidth int, children []QueryNode) *MaxWidthIntervalNode {
	return &MaxWidthIntervalNode{newAnalyticIntervalNode("MAX_WIDTH", maxWidth, children)}
}

// GetMaxWidth returns the maximum width.
func (n *MaxWidthIntervalNode) GetMaxWidth() int { return n.param }

// CloneTree deep-copies this node.
func (n *MaxWidthIntervalNode) CloneTree() QueryNode {
	cloned := &MaxWidthIntervalNode{newAnalyticIntervalNode("MAX_WIDTH", n.param, nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, c := range n.GetChildren() {
		cloned.AddChild(c.CloneTree())
	}
	return cloned
}

// String returns debug text.
func (n *MaxWidthIntervalNode) String() string {
	return fmt.Sprintf("<max_width width=%d>", n.param)
}

// AtLeastIntervalNode requires at least N child intervals to match.
// This is the Go equivalent of Lucene's intervalfn.AtLeast.
type AtLeastIntervalNode struct{ *analyticIntervalNode }

// NewAtLeastIntervalNode creates a new AtLeastIntervalNode.
func NewAtLeastIntervalNode(minShouldMatch int, children []QueryNode) *AtLeastIntervalNode {
	return &AtLeastIntervalNode{newAnalyticIntervalNode("AT_LEAST", minShouldMatch, children)}
}

// GetMinShouldMatch returns the minimum-should-match count.
func (n *AtLeastIntervalNode) GetMinShouldMatch() int { return n.param }

// CloneTree deep-copies this node.
func (n *AtLeastIntervalNode) CloneTree() QueryNode {
	cloned := &AtLeastIntervalNode{newAnalyticIntervalNode("AT_LEAST", n.param, nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	for _, c := range n.GetChildren() {
		cloned.AddChild(c.CloneTree())
	}
	return cloned
}

// String returns debug text.
func (n *AtLeastIntervalNode) String() string {
	return fmt.Sprintf("<at_least min=%d>", n.param)
}

// binaryIntervalNode is a base for two-operand interval nodes.
type binaryIntervalNode struct {
	*baseIntervalNode
}

func newBinaryIntervalNode(name string, left, right QueryNode) *binaryIntervalNode {
	children := make([]QueryNode, 0, 2)
	if left != nil {
		children = append(children, left)
	}
	if right != nil {
		children = append(children, right)
	}
	return &binaryIntervalNode{baseIntervalNode: newBaseIntervalNode(name, children)}
}

// BeforeIntervalNode requires the first child to match before the second.
// This is the Go equivalent of Lucene's intervalfn.Before.
type BeforeIntervalNode struct{ *binaryIntervalNode }

// NewBeforeIntervalNode creates a new BeforeIntervalNode.
func NewBeforeIntervalNode(left, right QueryNode) *BeforeIntervalNode {
	return &BeforeIntervalNode{newBinaryIntervalNode("BEFORE", left, right)}
}

// ToQueryString returns BEFORE(left, right).
func (n *BeforeIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *BeforeIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &BeforeIntervalNode{newBinaryIntervalNode("BEFORE", l, r)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *BeforeIntervalNode) String() string { return "<before>" }

// AfterIntervalNode requires the first child to match after the second.
// This is the Go equivalent of Lucene's intervalfn.After.
type AfterIntervalNode struct{ *binaryIntervalNode }

// NewAfterIntervalNode creates a new AfterIntervalNode.
func NewAfterIntervalNode(left, right QueryNode) *AfterIntervalNode {
	return &AfterIntervalNode{newBinaryIntervalNode("AFTER", left, right)}
}

// ToQueryString returns AFTER(left, right).
func (n *AfterIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *AfterIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &AfterIntervalNode{newBinaryIntervalNode("AFTER", l, r)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *AfterIntervalNode) String() string { return "<after>" }

// ContainedByIntervalNode requires the first interval to be contained by the second.
// This is the Go equivalent of Lucene's intervalfn.ContainedBy.
type ContainedByIntervalNode struct{ *binaryIntervalNode }

// NewContainedByIntervalNode creates a new ContainedByIntervalNode.
func NewContainedByIntervalNode(small, big QueryNode) *ContainedByIntervalNode {
	return &ContainedByIntervalNode{newBinaryIntervalNode("CONTAINED_BY", small, big)}
}

// ToQueryString returns CONTAINED_BY(small, big).
func (n *ContainedByIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *ContainedByIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &ContainedByIntervalNode{newBinaryIntervalNode("CONTAINED_BY", l, r)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *ContainedByIntervalNode) String() string { return "<contained_by>" }

// ContainingIntervalNode requires the first interval to contain the second.
// This is the Go equivalent of Lucene's intervalfn.Containing.
type ContainingIntervalNode struct{ *binaryIntervalNode }

// NewContainingIntervalNode creates a new ContainingIntervalNode.
func NewContainingIntervalNode(big, small QueryNode) *ContainingIntervalNode {
	return &ContainingIntervalNode{newBinaryIntervalNode("CONTAINING", big, small)}
}

// ToQueryString returns CONTAINING(big, small).
func (n *ContainingIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *ContainingIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &ContainingIntervalNode{newBinaryIntervalNode("CONTAINING", l, r)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *ContainingIntervalNode) String() string { return "<containing>" }

// NotContainedByIntervalNode requires the first interval NOT to be contained by the second.
// This is the Go equivalent of Lucene's intervalfn.NotContainedBy.
type NotContainedByIntervalNode struct{ *binaryIntervalNode }

// NewNotContainedByIntervalNode creates a new node.
func NewNotContainedByIntervalNode(small, big QueryNode) *NotContainedByIntervalNode {
	return &NotContainedByIntervalNode{newBinaryIntervalNode("NOT_CONTAINED_BY", small, big)}
}

// ToQueryString returns NOT_CONTAINED_BY(small, big).
func (n *NotContainedByIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *NotContainedByIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &NotContainedByIntervalNode{newBinaryIntervalNode("NOT_CONTAINED_BY", l, r)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *NotContainedByIntervalNode) String() string { return "<not_contained_by>" }

// NotContainingIntervalNode requires the first interval NOT to contain the second.
// This is the Go equivalent of Lucene's intervalfn.NotContaining.
type NotContainingIntervalNode struct{ *binaryIntervalNode }

// NewNotContainingIntervalNode creates a new node.
func NewNotContainingIntervalNode(big, small QueryNode) *NotContainingIntervalNode {
	return &NotContainingIntervalNode{newBinaryIntervalNode("NOT_CONTAINING", big, small)}
}

// ToQueryString returns NOT_CONTAINING(big, small).
func (n *NotContainingIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *NotContainingIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &NotContainingIntervalNode{newBinaryIntervalNode("NOT_CONTAINING", l, r)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *NotContainingIntervalNode) String() string { return "<not_containing>" }

// NonOverlappingIntervalNode matches intervals that do not overlap.
// This is the Go equivalent of Lucene's intervalfn.NonOverlapping.
type NonOverlappingIntervalNode struct{ *binaryIntervalNode }

// NewNonOverlappingIntervalNode creates a new node.
func NewNonOverlappingIntervalNode(a, b QueryNode) *NonOverlappingIntervalNode {
	return &NonOverlappingIntervalNode{newBinaryIntervalNode("NON_OVERLAPPING", a, b)}
}

// ToQueryString returns NON_OVERLAPPING(a, b).
func (n *NonOverlappingIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *NonOverlappingIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &NonOverlappingIntervalNode{newBinaryIntervalNode("NON_OVERLAPPING", l, r)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *NonOverlappingIntervalNode) String() string { return "<non_overlapping>" }

// OverlappingIntervalNode matches intervals that overlap.
// This is the Go equivalent of Lucene's intervalfn.Overlapping.
type OverlappingIntervalNode struct{ *binaryIntervalNode }

// NewOverlappingIntervalNode creates a new node.
func NewOverlappingIntervalNode(a, b QueryNode) *OverlappingIntervalNode {
	return &OverlappingIntervalNode{newBinaryIntervalNode("OVERLAPPING", a, b)}
}

// ToQueryString returns OVERLAPPING(a, b).
func (n *OverlappingIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *OverlappingIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &OverlappingIntervalNode{newBinaryIntervalNode("OVERLAPPING", l, r)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *OverlappingIntervalNode) String() string { return "<overlapping>" }

// WithinIntervalNode requires interval a to be within N positions of interval b.
// This is the Go equivalent of Lucene's intervalfn.Within.
type WithinIntervalNode struct {
	*analyticIntervalNode
	source QueryNode
}

// NewWithinIntervalNode creates a new WithinIntervalNode.
func NewWithinIntervalNode(within int, source, reference QueryNode) *WithinIntervalNode {
	children := make([]QueryNode, 0, 2)
	if source != nil {
		children = append(children, source)
	}
	if reference != nil {
		children = append(children, reference)
	}
	return &WithinIntervalNode{
		analyticIntervalNode: newAnalyticIntervalNode("WITHIN", within, children),
	}
}

// CloneTree deep-copies this node.
func (n *WithinIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &WithinIntervalNode{
		analyticIntervalNode: newAnalyticIntervalNode("WITHIN", n.param, []QueryNode{l, r}),
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *WithinIntervalNode) String() string {
	return fmt.Sprintf("<within dist=%d>", n.param)
}

// NotWithinIntervalNode requires interval a NOT to be within N positions of interval b.
// This is the Go equivalent of Lucene's intervalfn.NotWithin.
type NotWithinIntervalNode struct{ *analyticIntervalNode }

// NewNotWithinIntervalNode creates a new NotWithinIntervalNode.
func NewNotWithinIntervalNode(notWithin int, source, reference QueryNode) *NotWithinIntervalNode {
	children := make([]QueryNode, 0, 2)
	if source != nil {
		children = append(children, source)
	}
	if reference != nil {
		children = append(children, reference)
	}
	return &NotWithinIntervalNode{newAnalyticIntervalNode("NOT_WITHIN", notWithin, children)}
}

// CloneTree deep-copies this node.
func (n *NotWithinIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &NotWithinIntervalNode{newAnalyticIntervalNode("NOT_WITHIN", n.param, []QueryNode{l, r})}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *NotWithinIntervalNode) String() string {
	return fmt.Sprintf("<not_within dist=%d>", n.param)
}

// ExtendIntervalNode extends the match of a source interval by the extent of a reference.
// This is the Go equivalent of Lucene's intervalfn.Extend.
type ExtendIntervalNode struct{ *binaryIntervalNode }

// NewExtendIntervalNode creates a new ExtendIntervalNode.
func NewExtendIntervalNode(source, reference QueryNode) *ExtendIntervalNode {
	return &ExtendIntervalNode{newBinaryIntervalNode("EXTEND", source, reference)}
}

// ToQueryString returns EXTEND(source, reference).
func (n *ExtendIntervalNode) ToQueryString(e bool) string { return n.formatChildren(e) }

// CloneTree deep-copies this node.
func (n *ExtendIntervalNode) CloneTree() QueryNode {
	var l, r QueryNode
	children := n.GetChildren()
	if len(children) > 0 {
		l = children[0].CloneTree()
	}
	if len(children) > 1 {
		r = children[1].CloneTree()
	}
	cloned := &ExtendIntervalNode{newBinaryIntervalNode("EXTEND", l, r)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *ExtendIntervalNode) String() string { return "<extend>" }

// AnalyzedTermIntervalNode holds an analyzed term for use in interval queries.
// This is the Go equivalent of Lucene's intervalfn.AnalyzedText.
type AnalyzedTermIntervalNode struct {
	*QueryNodeImpl
	term string
}

// NewAnalyzedTermIntervalNode creates a new AnalyzedTermIntervalNode.
func NewAnalyzedTermIntervalNode(term string) *AnalyzedTermIntervalNode {
	return &AnalyzedTermIntervalNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		term:          term,
	}
}

// GetTerm returns the term string.
func (n *AnalyzedTermIntervalNode) GetTerm() string { return n.term }

// ToQueryString returns the term.
func (n *AnalyzedTermIntervalNode) ToQueryString(_ bool) string { return n.term }

// CloneTree deep-copies this node.
func (n *AnalyzedTermIntervalNode) CloneTree() QueryNode {
	cloned := &AnalyzedTermIntervalNode{QueryNodeImpl: NewQueryNodeImpl(nil), term: n.term}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *AnalyzedTermIntervalNode) String() string {
	return fmt.Sprintf("<analyzed_term term=%s>", n.term)
}

// FuzzyTermIntervalNode holds a fuzzy term for use in interval queries.
// This is the Go equivalent of Lucene's intervalfn.FuzzyTerm.
type FuzzyTermIntervalNode struct {
	*QueryNodeImpl
	term           string
	maxEdits       int
	prefixLen      int
	transpositions bool
}

// NewFuzzyTermIntervalNode creates a new FuzzyTermIntervalNode.
func NewFuzzyTermIntervalNode(term string, maxEdits, prefixLen int, transpositions bool) *FuzzyTermIntervalNode {
	return &FuzzyTermIntervalNode{
		QueryNodeImpl:  NewQueryNodeImpl(nil),
		term:           term,
		maxEdits:       maxEdits,
		prefixLen:      prefixLen,
		transpositions: transpositions,
	}
}

// GetTerm returns the fuzzy term.
func (n *FuzzyTermIntervalNode) GetTerm() string { return n.term }

// GetMaxEdits returns the maximum edit distance.
func (n *FuzzyTermIntervalNode) GetMaxEdits() int { return n.maxEdits }

// ToQueryString returns term~maxEdits.
func (n *FuzzyTermIntervalNode) ToQueryString(_ bool) string {
	return n.term + "~" + strconv.Itoa(n.maxEdits)
}

// CloneTree deep-copies this node.
func (n *FuzzyTermIntervalNode) CloneTree() QueryNode {
	cloned := &FuzzyTermIntervalNode{
		QueryNodeImpl:  NewQueryNodeImpl(nil),
		term:           n.term,
		maxEdits:       n.maxEdits,
		prefixLen:      n.prefixLen,
		transpositions: n.transpositions,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *FuzzyTermIntervalNode) String() string {
	return fmt.Sprintf("<fuzzy_term term=%s maxEdits=%d>", n.term, n.maxEdits)
}

// WildcardIntervalNode holds a wildcard pattern for use in interval queries.
// This is the Go equivalent of Lucene's intervalfn.Wildcard.
type WildcardIntervalNode struct {
	*QueryNodeImpl
	pattern string
}

// NewWildcardIntervalNode creates a new WildcardIntervalNode.
func NewWildcardIntervalNode(pattern string) *WildcardIntervalNode {
	return &WildcardIntervalNode{QueryNodeImpl: NewQueryNodeImpl(nil), pattern: pattern}
}

// GetPattern returns the wildcard pattern.
func (n *WildcardIntervalNode) GetPattern() string { return n.pattern }

// ToQueryString returns the pattern.
func (n *WildcardIntervalNode) ToQueryString(_ bool) string { return n.pattern }

// CloneTree deep-copies this node.
func (n *WildcardIntervalNode) CloneTree() QueryNode {
	cloned := &WildcardIntervalNode{QueryNodeImpl: NewQueryNodeImpl(nil), pattern: n.pattern}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns debug text.
func (n *WildcardIntervalNode) String() string {
	return fmt.Sprintf("<wildcard_interval pattern=%s>", n.pattern)
}
