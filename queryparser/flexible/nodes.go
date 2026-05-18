// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strconv"
	"strings"
)

// FieldableNode is implemented by query nodes that carry a field name.
// This is the Go equivalent of Lucene's FieldableNode.
type FieldableNode interface {
	QueryNode
	// GetField returns the field name.
	GetField() string
	// SetField sets the field name.
	SetField(field string)
}

// TextableQueryNode is implemented by query nodes that carry a text value.
// This is the Go equivalent of Lucene's TextableQueryNode.
type TextableQueryNode interface {
	QueryNode
	// GetText returns the text.
	GetText() string
	// SetText sets the text.
	SetText(text string)
}

// ValueQueryNode is implemented by query nodes that carry a single typed value.
// This is the Go equivalent of Lucene's ValueQueryNode.
type ValueQueryNode interface {
	QueryNode
	// GetValue returns the node value.
	GetValue() interface{}
	// SetValue sets the node value.
	SetValue(value interface{})
}

// FieldValuePairQueryNode is implemented by nodes that represent a field:value pair.
// This is the Go equivalent of Lucene's FieldValuePairQueryNode.
type FieldValuePairQueryNode interface {
	FieldableNode
	// GetValue returns the value part of the pair.
	GetValue() interface{}
	// SetValue sets the value part of the pair.
	SetValue(value interface{})
}

// DeletedQueryNode is a placeholder node used when a node is logically removed
// from the query tree during processing without structurally deleting it.
// This is the Go equivalent of Lucene's DeletedQueryNode.
type DeletedQueryNode struct {
	*QueryNodeImpl
}

// NewDeletedQueryNode creates a new DeletedQueryNode.
func NewDeletedQueryNode() *DeletedQueryNode {
	return &DeletedQueryNode{QueryNodeImpl: NewQueryNodeImpl(nil)}
}

// ToQueryString returns an empty string — deleted nodes contribute nothing.
func (n *DeletedQueryNode) ToQueryString(_ bool) string { return "" }

// CloneTree deep-copies this node.
func (n *DeletedQueryNode) CloneTree() QueryNode {
	cloned := &DeletedQueryNode{QueryNodeImpl: NewQueryNodeImpl(nil)}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *DeletedQueryNode) String() string { return "<deleted>" }

// AnyQueryNode represents a query that requires at least minimumMatchingElements
// of its child clauses to match. It is the Go equivalent of Lucene's AnyQueryNode.
type AnyQueryNode struct {
	*BooleanQueryNode
	minimumMatchingElements int
}

// NewAnyQueryNode creates a new AnyQueryNode.
func NewAnyQueryNode(children []QueryNode, minimumMatchingElements int) *AnyQueryNode {
	return &AnyQueryNode{
		BooleanQueryNode:        NewBooleanQueryNode("OR", children),
		minimumMatchingElements: minimumMatchingElements,
	}
}

// GetMinimumMatchingElements returns the minimum match threshold.
func (n *AnyQueryNode) GetMinimumMatchingElements() int {
	return n.minimumMatchingElements
}

// SetMinimumMatchingElements sets the minimum match threshold.
func (n *AnyQueryNode) SetMinimumMatchingElements(min int) {
	n.minimumMatchingElements = min
}

// ToQueryString returns "(<child1> OR <child2> ...)/N".
func (n *AnyQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	children := n.GetChildren()
	var sb strings.Builder
	sb.WriteString("(")
	for i, child := range children {
		if i > 0 {
			sb.WriteString(" OR ")
		}
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}
	sb.WriteString(")/")
	sb.WriteString(strconv.Itoa(n.minimumMatchingElements))
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *AnyQueryNode) CloneTree() QueryNode {
	cloned := &AnyQueryNode{
		BooleanQueryNode:        NewBooleanQueryNode("OR", nil),
		minimumMatchingElements: n.minimumMatchingElements,
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
func (n *AnyQueryNode) String() string {
	return fmt.Sprintf("<any min=%d children=%d>", n.minimumMatchingElements, len(n.GetChildren()))
}

// NoTokenFoundQueryNode represents a query node produced when the analyzer emits
// no tokens for a term. It is the Go equivalent of Lucene's NoTokenFoundQueryNode.
type NoTokenFoundQueryNode struct {
	*FieldQueryNode
}

// NewNoTokenFoundQueryNode creates a new NoTokenFoundQueryNode.
func NewNoTokenFoundQueryNode(field, text string, begin, end int) *NoTokenFoundQueryNode {
	return &NoTokenFoundQueryNode{FieldQueryNode: NewFieldQueryNode(field, text, begin, end)}
}

// ToQueryString returns an empty string — no token means no query contribution.
func (n *NoTokenFoundQueryNode) ToQueryString(_ bool) string { return "" }

// CloneTree deep-copies this node.
func (n *NoTokenFoundQueryNode) CloneTree() QueryNode {
	cloned := &NoTokenFoundQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *NoTokenFoundQueryNode) String() string {
	return fmt.Sprintf("<no_token field=%s text=%s>", n.GetField(), n.GetText())
}

// OpaqueQueryNode holds an arbitrary query fragment that cannot be parsed further.
// Its schema and value are opaque to the framework.
// This is the Go equivalent of Lucene's OpaqueQueryNode.
type OpaqueQueryNode struct {
	*QueryNodeImpl
	schema string
	value  string
}

// NewOpaqueQueryNode creates a new OpaqueQueryNode.
func NewOpaqueQueryNode(schema, value string) *OpaqueQueryNode {
	return &OpaqueQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		schema:        schema,
		value:         value,
	}
}

// GetSchema returns the schema identifier.
func (n *OpaqueQueryNode) GetSchema() string { return n.schema }

// GetValue returns the opaque value string.
func (n *OpaqueQueryNode) GetValue() string { return n.value }

// ToQueryString returns "@schema:value".
func (n *OpaqueQueryNode) ToQueryString(_ bool) string {
	return "@" + n.schema + ":" + n.value
}

// CloneTree deep-copies this node.
func (n *OpaqueQueryNode) CloneTree() QueryNode {
	cloned := &OpaqueQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		schema:        n.schema,
		value:         n.value,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *OpaqueQueryNode) String() string {
	return fmt.Sprintf("<opaque schema=%s value=%s>", n.schema, n.value)
}

// PathQueryNode represents a query on a hierarchical path field.
// It is the Go equivalent of Lucene's PathQueryNode.
type PathQueryNode struct {
	*QueryNodeImpl
	pathElements []string
}

// NewPathQueryNode creates a new PathQueryNode from a slice of path elements.
func NewPathQueryNode(pathElements []string) *PathQueryNode {
	elems := make([]string, len(pathElements))
	copy(elems, pathElements)
	return &PathQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		pathElements:  elems,
	}
}

// GetPathElements returns the path elements.
func (n *PathQueryNode) GetPathElements() []string {
	out := make([]string, len(n.pathElements))
	copy(out, n.pathElements)
	return out
}

// ToQueryString returns the path joined with '/'.
func (n *PathQueryNode) ToQueryString(_ bool) string {
	return strings.Join(n.pathElements, "/")
}

// CloneTree deep-copies this node.
func (n *PathQueryNode) CloneTree() QueryNode {
	cloned := &PathQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		pathElements:  make([]string, len(n.pathElements)),
	}
	copy(cloned.pathElements, n.pathElements)
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *PathQueryNode) String() string {
	return fmt.Sprintf("<path %s>", strings.Join(n.pathElements, "/"))
}

// ProximityType indicates whether a ProximityQueryNode is word- or sentence-based.
type ProximityType int

const (
	// ProximityWord requires terms to be within N words of each other.
	ProximityWord ProximityType = iota
	// ProximitySentence requires terms to be within the same sentence.
	ProximitySentence
)

// ProximityQueryNode represents a proximity query (WITHIN N WORDS / SENTENCE).
// It is the Go equivalent of Lucene's ProximityQueryNode.
type ProximityQueryNode struct {
	*FieldQueryNode
	distance      int
	proximityType ProximityType
}

// NewProximityQueryNode creates a new ProximityQueryNode.
func NewProximityQueryNode(field, text string, distance int, proximityType ProximityType, begin, end int) *ProximityQueryNode {
	return &ProximityQueryNode{
		FieldQueryNode: NewFieldQueryNode(field, text, begin, end),
		distance:       distance,
		proximityType:  proximityType,
	}
}

// GetDistance returns the proximity distance.
func (n *ProximityQueryNode) GetDistance() int { return n.distance }

// GetProximityType returns the proximity type.
func (n *ProximityQueryNode) GetProximityType() ProximityType { return n.proximityType }

// ToQueryString returns a human-readable proximity expression.
func (n *ProximityQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var typeName string
	switch n.proximityType {
	case ProximitySentence:
		typeName = "SENTENCE"
	default:
		typeName = "WORD"
	}
	text := n.GetText()
	if escapeSpecialSyntax {
		text = escapeQueryString(text)
	}
	return fmt.Sprintf("%s:%s~%s/%d", n.GetField(), text, typeName, n.distance)
}

// CloneTree deep-copies this node.
func (n *ProximityQueryNode) CloneTree() QueryNode {
	cloned := &ProximityQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
		distance:       n.distance,
		proximityType:  n.proximityType,
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *ProximityQueryNode) String() string {
	return fmt.Sprintf("<proximity field=%s text=%s distance=%d>", n.GetField(), n.GetText(), n.distance)
}

// QuotedFieldQueryNode is a FieldQueryNode whose text was surrounded by quotes in
// the original query string. It is the Go equivalent of Lucene's QuotedFieldQueryNode.
type QuotedFieldQueryNode struct {
	*FieldQueryNode
}

// NewQuotedFieldQueryNode creates a new QuotedFieldQueryNode.
func NewQuotedFieldQueryNode(field, text string, begin, end int) *QuotedFieldQueryNode {
	return &QuotedFieldQueryNode{FieldQueryNode: NewFieldQueryNode(field, text, begin, end)}
}

// ToQueryString returns field:"text".
func (n *QuotedFieldQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	if n.GetField() != "" {
		sb.WriteString(n.GetField())
		sb.WriteString(":")
	}
	sb.WriteString(`"`)
	text := n.GetText()
	if escapeSpecialSyntax {
		text = escapeQueryString(text)
	}
	sb.WriteString(text)
	sb.WriteString(`"`)
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *QuotedFieldQueryNode) CloneTree() QueryNode {
	cloned := &QuotedFieldQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
	}
	for _, k := range n.GetTagKeys() {
		cloned.SetTag(k, n.GetTag(k))
	}
	return cloned
}

// String returns a debug representation.
func (n *QuotedFieldQueryNode) String() string {
	return fmt.Sprintf("<quoted field=%s text=%q>", n.GetField(), n.GetText())
}

// SlopQueryNode wraps a child node and adds a slop value to it.
// This is the Go equivalent of Lucene's SlopQueryNode.
type SlopQueryNode struct {
	*QueryNodeImpl
	value int
}

// NewSlopQueryNode creates a new SlopQueryNode wrapping child with the given slop.
func NewSlopQueryNode(child QueryNode, value int) *SlopQueryNode {
	node := &SlopQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		value:         value,
	}
	if child != nil {
		node.AddChild(child)
	}
	return node
}

// GetValue returns the slop value.
func (n *SlopQueryNode) GetValue() int { return n.value }

// SetValue sets the slop value.
func (n *SlopQueryNode) SetValue(value int) { n.value = value }

// ToQueryString appends ~slop to the child's representation.
func (n *SlopQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	children := n.GetChildren()
	if len(children) == 0 {
		return ""
	}
	return children[0].ToQueryString(escapeSpecialSyntax) + "~" + strconv.Itoa(n.value)
}

// CloneTree deep-copies this node.
func (n *SlopQueryNode) CloneTree() QueryNode {
	cloned := &SlopQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		value:         n.value,
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
func (n *SlopQueryNode) String() string {
	return fmt.Sprintf("<slop value=%d>", n.value)
}

// TokenizedPhraseQueryNode holds a sequence of already-analyzed tokens that
// form a phrase. It is the Go equivalent of Lucene's TokenizedPhraseQueryNode.
type TokenizedPhraseQueryNode struct {
	*QueryNodeImpl
	field string
}

// NewTokenizedPhraseQueryNode creates a new TokenizedPhraseQueryNode for field,
// with the provided token nodes as children.
func NewTokenizedPhraseQueryNode(field string, tokenNodes []QueryNode) *TokenizedPhraseQueryNode {
	node := &TokenizedPhraseQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(tokenNodes),
		field:         field,
	}
	return node
}

// GetField returns the field this phrase applies to.
func (n *TokenizedPhraseQueryNode) GetField() string { return n.field }

// SetField sets the field name.
func (n *TokenizedPhraseQueryNode) SetField(field string) { n.field = field }

// ToQueryString emits field:"token1 token2 ...".
func (n *TokenizedPhraseQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder
	if n.field != "" {
		sb.WriteString(n.field)
		sb.WriteString(":")
	}
	sb.WriteString(`"`)
	for i, child := range n.GetChildren() {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}
	sb.WriteString(`"`)
	return sb.String()
}

// CloneTree deep-copies this node.
func (n *TokenizedPhraseQueryNode) CloneTree() QueryNode {
	cloned := &TokenizedPhraseQueryNode{
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
func (n *TokenizedPhraseQueryNode) String() string {
	return fmt.Sprintf("<tokenized_phrase field=%s tokens=%d>", n.field, len(n.GetChildren()))
}

// Compile-time assertions that concrete types satisfy the marker interfaces.
var (
	_ FieldableNode           = (*FieldQueryNode)(nil)
	_ TextableQueryNode       = (*FieldQueryNode)(nil)
	_ FieldableNode           = (*QuotedFieldQueryNode)(nil)
	_ TextableQueryNode       = (*QuotedFieldQueryNode)(nil)
	_ FieldableNode           = (*TokenizedPhraseQueryNode)(nil)
	_ FieldValuePairQueryNode = (*FieldQueryNode)(nil)
)

// GetField satisfies FieldableNode for FieldQueryNode (already defined in field_query_node.go).
// GetText satisfies TextableQueryNode for FieldQueryNode (already defined).

// GetValue on FieldQueryNode returns the text as a string value,
// satisfying both ValueQueryNode and FieldValuePairQueryNode.
func (n *FieldQueryNode) GetValue() interface{} { return n.text }

// SetValue on FieldQueryNode stores a string value.
func (n *FieldQueryNode) SetValue(value interface{}) {
	if s, ok := value.(string); ok {
		n.text = s
	}
}
