package flexible

import (
	"fmt"
	"strconv"
	"strings"
)

// ModifierQueryNode represents a query node with a modifier (+ or -).
// The modifier indicates whether the query is required (+) or prohibited (-).
type ModifierQueryNode struct {
	*QueryNodeImpl
	modifier Modifier
}

// Modifier represents the type of modifier.
type Modifier int

const (
	// ModifierNone indicates no modifier.
	ModifierNone Modifier = iota
	// ModifierRequired indicates the query is required (+).
	ModifierRequired
	// ModifierProhibited indicates the query is prohibited (-).
	ModifierProhibited
)

// String returns the string representation of the modifier.
func (m Modifier) String() string {
	switch m {
	case ModifierRequired:
		return "+"
	case ModifierProhibited:
		return "-"
	default:
		return ""
	}
}

// NewModifierQueryNode creates a new ModifierQueryNode.
func NewModifierQueryNode(child QueryNode, modifier Modifier) *ModifierQueryNode {
	node := &ModifierQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		modifier:      modifier,
	}
	if child != nil {
		node.AddChild(child)
	}
	return node
}

// GetModifier returns the modifier.
func (n *ModifierQueryNode) GetModifier() Modifier {
	return n.modifier
}

// SetModifier sets the modifier.
func (n *ModifierQueryNode) SetModifier(modifier Modifier) {
	n.modifier = modifier
}

// ToQueryString returns the query string representation.
func (n *ModifierQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder

	if n.modifier != ModifierNone {
		sb.WriteString(n.modifier.String())
	}

	children := n.GetChildren()
	if len(children) > 0 {
		sb.WriteString(children[0].ToQueryString(escapeSpecialSyntax))
	}

	return sb.String()
}

// CloneTree creates a deep copy of this node.
func (n *ModifierQueryNode) CloneTree() QueryNode {
	cloned := &ModifierQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		modifier:      n.modifier,
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	// Clone child
	children := n.GetChildren()
	if len(children) > 0 {
		cloned.AddChild(children[0].CloneTree())
	}

	return cloned
}

// String returns a string representation of this node.
func (n *ModifierQueryNode) String() string {
	return fmt.Sprintf("<modifier modifier=%s>", n.modifier.String())
}

// BoostQueryNode represents a query node with a boost value.
type BoostQueryNode struct {
	*QueryNodeImpl
	value float64
}

// NewBoostQueryNode creates a new BoostQueryNode.
func NewBoostQueryNode(child QueryNode, value float64) *BoostQueryNode {
	node := &BoostQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		value:         value,
	}
	if child != nil {
		node.AddChild(child)
	}
	return node
}

// GetValue returns the boost value.
func (n *BoostQueryNode) GetValue() float64 {
	return n.value
}

// SetValue sets the boost value.
func (n *BoostQueryNode) SetValue(value float64) {
	n.value = value
}

// ToQueryString returns the query string representation.
func (n *BoostQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder

	children := n.GetChildren()
	if len(children) > 0 {
		sb.WriteString(children[0].ToQueryString(escapeSpecialSyntax))
	}

	if n.value != 1.0 {
		sb.WriteString("^")
		sb.WriteString(strconv.FormatFloat(n.value, 'f', -1, 64))
	}

	return sb.String()
}

// CloneTree creates a deep copy of this node.
func (n *BoostQueryNode) CloneTree() QueryNode {
	cloned := &BoostQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		value:         n.value,
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	// Clone child
	children := n.GetChildren()
	if len(children) > 0 {
		cloned.AddChild(children[0].CloneTree())
	}

	return cloned
}

// String returns a string representation of this node.
func (n *BoostQueryNode) String() string {
	return fmt.Sprintf("<boost value=%f>", n.value)
}

// FuzzyQueryNode represents a fuzzy query node.
type FuzzyQueryNode struct {
	*FieldQueryNode
	minSimilarity float64
	prefixLength  int
}

// NewFuzzyQueryNode creates a new FuzzyQueryNode.
func NewFuzzyQueryNode(field, text string, minSimilarity float64, prefixLength int, begin, end int) *FuzzyQueryNode {
	return &FuzzyQueryNode{
		FieldQueryNode: NewFieldQueryNode(field, text, begin, end),
		minSimilarity:  minSimilarity,
		prefixLength:   prefixLength,
	}
}

// GetMinSimilarity returns the minimum similarity.
func (n *FuzzyQueryNode) GetMinSimilarity() float64 {
	return n.minSimilarity
}

// SetMinSimilarity sets the minimum similarity.
func (n *FuzzyQueryNode) SetMinSimilarity(minSimilarity float64) {
	n.minSimilarity = minSimilarity
}

// GetPrefixLength returns the prefix length.
func (n *FuzzyQueryNode) GetPrefixLength() int {
	return n.prefixLength
}

// SetPrefixLength sets the prefix length.
func (n *FuzzyQueryNode) SetPrefixLength(prefixLength int) {
	n.prefixLength = prefixLength
}

// ToQueryString returns the query string representation.
func (n *FuzzyQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder

	if n.GetField() != "" {
		sb.WriteString(n.GetField())
		sb.WriteString(":")
	}

	if escapeSpecialSyntax {
		sb.WriteString(escapeQueryString(n.GetText()))
	} else {
		sb.WriteString(n.GetText())
	}

	sb.WriteString("~")
	if n.minSimilarity != 0.5 {
		sb.WriteString(strconv.FormatFloat(n.minSimilarity, 'f', -1, 64))
	}

	return sb.String()
}

// CloneTree creates a deep copy of this node.
func (n *FuzzyQueryNode) CloneTree() QueryNode {
	cloned := &FuzzyQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
		minSimilarity:  n.minSimilarity,
		prefixLength:   n.prefixLength,
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	return cloned
}

// String returns a string representation of this node.
func (n *FuzzyQueryNode) String() string {
	return fmt.Sprintf("<fuzzy field=%s text=%s minSim=%f prefixLen=%d>",
		n.GetField(), n.GetText(), n.minSimilarity, n.prefixLength)
}

// RangeQueryNode represents a range query node.
type RangeQueryNode struct {
	*QueryNodeImpl
	field      string
	lower      string
	upper      string
	lowerBound BoundType
	upperBound BoundType
}

// BoundType represents the type of bound (inclusive or exclusive).
type BoundType int

const (
	// BoundExclusive indicates an exclusive bound.
	BoundExclusive BoundType = iota
	// BoundInclusive indicates an inclusive bound.
	BoundInclusive
)

// NewRangeQueryNode creates a new RangeQueryNode.
func NewRangeQueryNode(field, lower, upper string, lowerBound, upperBound BoundType) *RangeQueryNode {
	return &RangeQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		field:         field,
		lower:         lower,
		upper:         upper,
		lowerBound:    lowerBound,
		upperBound:    upperBound,
	}
}

// GetField returns the field name.
func (n *RangeQueryNode) GetField() string {
	return n.field
}

// SetField sets the field name.
func (n *RangeQueryNode) SetField(field string) {
	n.field = field
}

// GetLower returns the lower bound.
func (n *RangeQueryNode) GetLower() string {
	return n.lower
}

// SetLower sets the lower bound.
func (n *RangeQueryNode) SetLower(lower string) {
	n.lower = lower
}

// GetUpper returns the upper bound.
func (n *RangeQueryNode) GetUpper() string {
	return n.upper
}

// SetUpper sets the upper bound.
func (n *RangeQueryNode) SetUpper(upper string) {
	n.upper = upper
}

// GetLowerBound returns the lower bound type.
func (n *RangeQueryNode) GetLowerBound() BoundType {
	return n.lowerBound
}

// SetLowerBound sets the lower bound type.
func (n *RangeQueryNode) SetLowerBound(lowerBound BoundType) {
	n.lowerBound = lowerBound
}

// GetUpperBound returns the upper bound type.
func (n *RangeQueryNode) GetUpperBound() BoundType {
	return n.upperBound
}

// SetUpperBound sets the upper bound type.
func (n *RangeQueryNode) SetUpperBound(upperBound BoundType) {
	n.upperBound = upperBound
}

// IsLowerInclusive returns true if the lower bound is inclusive.
func (n *RangeQueryNode) IsLowerInclusive() bool {
	return n.lowerBound == BoundInclusive
}

// IsUpperInclusive returns true if the upper bound is inclusive.
func (n *RangeQueryNode) IsUpperInclusive() bool {
	return n.upperBound == BoundInclusive
}

// ToQueryString returns the query string representation.
func (n *RangeQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder

	if n.field != "" {
		sb.WriteString(n.field)
		sb.WriteString(":")
	}

	if n.lowerBound == BoundInclusive {
		sb.WriteString("[")
	} else {
		sb.WriteString("{")
	}

	if n.lower == "*" {
		sb.WriteString("*")
	} else if escapeSpecialSyntax {
		sb.WriteString(escapeQueryString(n.lower))
	} else {
		sb.WriteString(n.lower)
	}

	sb.WriteString(" TO ")

	if n.upper == "*" {
		sb.WriteString("*")
	} else if escapeSpecialSyntax {
		sb.WriteString(escapeQueryString(n.upper))
	} else {
		sb.WriteString(n.upper)
	}

	if n.upperBound == BoundInclusive {
		sb.WriteString("]")
	} else {
		sb.WriteString("}")
	}

	return sb.String()
}

// CloneTree creates a deep copy of this node.
func (n *RangeQueryNode) CloneTree() QueryNode {
	cloned := &RangeQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		field:         n.field,
		lower:         n.lower,
		upper:         n.upper,
		lowerBound:    n.lowerBound,
		upperBound:    n.upperBound,
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	return cloned
}

// String returns a string representation of this node.
func (n *RangeQueryNode) String() string {
	lowerChar := "{"
	if n.lowerBound == BoundInclusive {
		lowerChar = "["
	}
	upperChar := "}"
	if n.upperBound == BoundInclusive {
		upperChar = "]"
	}
	return fmt.Sprintf("<range field=%s %s%s TO %s%s>",
		n.field, lowerChar, n.lower, n.upper, upperChar)
}

// PhraseSlopQueryNode represents a phrase query with a slop value.
type PhraseSlopQueryNode struct {
	*FieldQueryNode
	slop int
}

// NewPhraseSlopQueryNode creates a new PhraseSlopQueryNode.
func NewPhraseSlopQueryNode(field, text string, slop, begin, end int) *PhraseSlopQueryNode {
	return &PhraseSlopQueryNode{
		FieldQueryNode: NewFieldQueryNode(field, text, begin, end),
		slop:           slop,
	}
}

// GetSlop returns the slop value.
func (n *PhraseSlopQueryNode) GetSlop() int {
	return n.slop
}

// SetSlop sets the slop value.
func (n *PhraseSlopQueryNode) SetSlop(slop int) {
	n.slop = slop
}

// ToQueryString returns the query string representation.
func (n *PhraseSlopQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder

	if n.GetField() != "" {
		sb.WriteString(n.GetField())
		sb.WriteString(":")
	}

	sb.WriteString("\"")
	if escapeSpecialSyntax {
		sb.WriteString(escapeQueryString(n.GetText()))
	} else {
		sb.WriteString(n.GetText())
	}
	sb.WriteString("\"")

	if n.slop != 0 {
		sb.WriteString("~")
		sb.WriteString(strconv.Itoa(n.slop))
	}

	return sb.String()
}

// CloneTree creates a deep copy of this node.
func (n *PhraseSlopQueryNode) CloneTree() QueryNode {
	cloned := &PhraseSlopQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
		slop:           n.slop,
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	return cloned
}

// String returns a string representation of this node.
func (n *PhraseSlopQueryNode) String() string {
	return fmt.Sprintf("<phrase field=%s text=%s slop=%d>", n.GetField(), n.GetText(), n.slop)
}

// GroupQueryNode represents a grouped query node (parentheses).
type GroupQueryNode struct {
	*QueryNodeImpl
}

// NewGroupQueryNode creates a new GroupQueryNode.
func NewGroupQueryNode(child QueryNode) *GroupQueryNode {
	node := &GroupQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
	}
	if child != nil {
		node.AddChild(child)
	}
	return node
}

// ToQueryString returns the query string representation.
func (n *GroupQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder

	sb.WriteString("(")

	children := n.GetChildren()
	for i, child := range children {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}

	sb.WriteString(")")

	return sb.String()
}

// CloneTree creates a deep copy of this node.
func (n *GroupQueryNode) CloneTree() QueryNode {
	cloned := &GroupQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	// Clone children
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}

	return cloned
}

// String returns a string representation of this node.
func (n *GroupQueryNode) String() string {
	return fmt.Sprintf("<group children=%d>", len(n.GetChildren()))
}

// MatchAllDocsQueryNode represents a query that matches all documents.
type MatchAllDocsQueryNode struct {
	*QueryNodeImpl
}

// NewMatchAllDocsQueryNode creates a new MatchAllDocsQueryNode.
func NewMatchAllDocsQueryNode() *MatchAllDocsQueryNode {
	return &MatchAllDocsQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
	}
}

// ToQueryString returns the query string representation.
func (n *MatchAllDocsQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	return "*:*"
}

// CloneTree creates a deep copy of this node.
func (n *MatchAllDocsQueryNode) CloneTree() QueryNode {
	cloned := &MatchAllDocsQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	return cloned
}

// String returns a string representation of this node.
func (n *MatchAllDocsQueryNode) String() string {
	return "<match_all>"
}

// MatchNoDocsQueryNode represents a query that matches no documents.
type MatchNoDocsQueryNode struct {
	*QueryNodeImpl
}

// NewMatchNoDocsQueryNode creates a new MatchNoDocsQueryNode.
func NewMatchNoDocsQueryNode() *MatchNoDocsQueryNode {
	return &MatchNoDocsQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
	}
}

// ToQueryString returns the query string representation.
func (n *MatchNoDocsQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	return "+ - + -"
}

// CloneTree creates a deep copy of this node.
func (n *MatchNoDocsQueryNode) CloneTree() QueryNode {
	cloned := &MatchNoDocsQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	return cloned
}

// String returns a string representation of this node.
func (n *MatchNoDocsQueryNode) String() string {
	return "<match_none>"
}
