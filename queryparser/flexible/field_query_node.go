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
	field      string
	text       string
	begin      int
	end        int
	position   int
}

// NewFieldQueryNode creates a new FieldQueryNode.
func NewFieldQueryNode(field, text string, begin, end int) *FieldQueryNode {
	return &FieldQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		field:         field,
		text:          text,
		begin:         begin,
		end:           end,
	}
}

// GetField returns the field name.
func (n *FieldQueryNode) GetField() string {
	return n.field
}

// SetField sets the field name.
func (n *FieldQueryNode) SetField(field string) {
	n.field = field
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
func (n *FieldQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder

	if n.field != "" {
		sb.WriteString(n.field)
	sb.WriteString(":")
	}

	if escapeSpecialSyntax {
		sb.WriteString(escapeQueryString(n.text))
	} else {
		sb.WriteString(n.text)
	}

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

// escapeQueryString escapes special characters in the query string.
func escapeQueryString(s string) string {
	// Simple escaping for special characters
	specialChars := []string{"\\", "+", "-", "&&", "||", "!", "(", ")", "{", "}", "[", "]", "^", "\"", "~", "*", "?", ":", "/"}
	result := s
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

// String returns a string representation of this node.
func (n *FieldQueryNode) String() string {
	return fmt.Sprintf("<field start=%d end=%d field=%s text=%s>", n.begin, n.end, n.field, n.text)
}
