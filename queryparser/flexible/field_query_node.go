package flexible

import (
	"fmt"
	"strings"
)

// FieldQueryNode represents a query node that contains a field and a text value.
// This is the most common type of query node, representing a term or phrase query
// on a specific field.
type FieldQueryNode struct {
	*QueryNodeImpl
	field    string
	text     string
	begin    int
	end      int
	position int
}

// NewFieldQueryNode creates a new FieldQueryNode.
func NewFieldQueryNode(field, text string, begin, end int) *FieldQueryNode {
	n := &FieldQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		field:         field,
		text:          text,
		begin:         begin,
		end:           end,
	}
	// Lucene's FieldQueryNode constructor ends with setLeaf(true).
	n.SetLeaf(true)
	return n
}

// GetField returns the field name.
func (n *FieldQueryNode) GetField() string {
	return n.field
}

// SetField sets the field name.
func (n *FieldQueryNode) SetField(field string) {
	n.field = field
}

// GetValue returns the value part of the field:value pair, which for a
// FieldQueryNode is its text.
//
// Mirrors org.apache.lucene.queryparser.flexible.core.nodes.FieldQueryNode#getValue().
func (n *FieldQueryNode) GetValue() interface{} { return n.GetText() }

// SetValue sets the value part of the field:value pair, which for a
// FieldQueryNode is its text.
//
// Mirrors org.apache.lucene.queryparser.flexible.core.nodes.FieldQueryNode#setValue(CharSequence).
func (n *FieldQueryNode) SetValue(value interface{}) {
	if s, ok := value.(string); ok {
		n.SetText(s)
		return
	}
	n.SetText(fmt.Sprintf("%v", value))
}

// GetText returns the text value.
func (n *FieldQueryNode) GetText() string {
	return n.text
}

// SetText sets the text value.
func (n *FieldQueryNode) SetText(text string) {
	n.text = text
}

// GetBegin returns the start position in the original query string.
func (n *FieldQueryNode) GetBegin() int {
	return n.begin
}

// SetBegin sets the start position.
func (n *FieldQueryNode) SetBegin(begin int) {
	n.begin = begin
}

// GetEnd returns the end position in the original query string.
func (n *FieldQueryNode) GetEnd() int {
	return n.end
}

// SetEnd sets the end position.
func (n *FieldQueryNode) SetEnd(end int) {
	n.end = end
}

// GetPosition returns the position increment.
func (n *FieldQueryNode) GetPosition() int {
	return n.position
}

// SetPosition sets the position increment.
func (n *FieldQueryNode) SetPosition(position int) {
	n.position = position
}

// ToQueryString returns the query string representation.
func (n *FieldQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	var sb strings.Builder

	if n.field != "" {
		sb.WriteString(n.field)
		sb.WriteString(":")
	}

	sb.WriteString(escapeSyntax.Escape(n.text, "en", EscapeNormal))

	return sb.String()
}

// CloneTree creates a deep copy of this node.
func (n *FieldQueryNode) CloneTree() QueryNode {
	cloned := &FieldQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		field:         n.field,
		text:          n.text,
		begin:         n.begin,
		end:           n.end,
		position:      n.position,
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	return cloned
}

// String returns a string representation of this node.
func (n *FieldQueryNode) String() string {
	return fmt.Sprintf("<field start=%d end=%d field=%s text=%s>", n.begin, n.end, n.field, n.text)
}

// QuotedFieldQueryNode represents phrase query. Example: "life is great"
type QuotedFieldQueryNode struct {
	*FieldQueryNode
}

// NewQuotedFieldQueryNode creates a new QuotedFieldQueryNode.
func NewQuotedFieldQueryNode(field, text string, begin, end int) *QuotedFieldQueryNode {
	return &QuotedFieldQueryNode{
		FieldQueryNode: NewFieldQueryNode(field, text, begin, end),
	}
}

// ToQueryString returns the query string representation.
func (n *QuotedFieldQueryNode) ToQueryString(escapeSyntax EscapeQuerySyntax) string {
	text := escapeSyntax.Escape(n.GetText(), "en", EscapeNormal)
	if n.GetField() == "" || n.GetField() == "_plain" {
		return fmt.Sprintf("\"%s\"", text)
	}
	return fmt.Sprintf("%s:\"%s\"", n.GetField(), text)
}

// String returns a string representation of this node.
func (n *QuotedFieldQueryNode) String() string {
	return fmt.Sprintf("<quotedfield start=%d end=%d field=%s term=%s>", n.GetBegin(), n.GetEnd(), n.GetField(), n.GetText())
}

// CloneTree creates a deep copy of this node.
func (n *QuotedFieldQueryNode) CloneTree() QueryNode {
	cloned := &QuotedFieldQueryNode{
		FieldQueryNode: NewFieldQueryNode(n.GetField(), n.GetText(), n.GetBegin(), n.GetEnd()),
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	return cloned
}
