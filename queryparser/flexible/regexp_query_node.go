// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "github.com/FlavioCFOliveira/Gocene/util"

// RegexpQueryNode represents a RegexpQuery. Examples: /[a-z]|[0-9]/
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.RegexpQueryNode.
type RegexpQueryNode struct {
	*QueryNodeImpl
	text  string
	field string
}

// NewRegexpQueryNode creates a RegexpQueryNode over the [begin, end) slice of
// text.
//
// Mirrors RegexpQueryNode(CharSequence field, CharSequence text, int begin, int end),
// whose body is `this.text = text.subSequence(begin, end)`.
func NewRegexpQueryNode(field, text string, begin, end int) *RegexpQueryNode {
	n := &RegexpQueryNode{QueryNodeImpl: NewQueryNodeImpl(nil)}
	// RegexpQueryNode extends QueryNodeImpl without calling setLeaf(false).
	n.SetLeaf(true)
	n.field = field
	n.text = text[begin:end]
	return n
}

// NewRegexpQueryNodeWholeText creates a RegexpQueryNode over the whole of text.
//
// Mirrors RegexpQueryNode(CharSequence field, CharSequence text).
func NewRegexpQueryNodeWholeText(field, text string) *RegexpQueryNode {
	return NewRegexpQueryNode(field, text, 0, len(text))
}

// TextToBytesRef returns the regular expression as a BytesRef.
//
// Mirrors RegexpQueryNode#textToBytesRef().
func (n *RegexpQueryNode) TextToBytesRef() *util.BytesRef {
	return util.NewBytesRef([]byte(n.text))
}

// GetText returns the regular expression.
//
// Mirrors RegexpQueryNode#getText().
func (n *RegexpQueryNode) GetText() string { return n.text }

// SetText sets the regular expression.
//
// Mirrors RegexpQueryNode#setText(CharSequence).
func (n *RegexpQueryNode) SetText(text string) { n.text = text }

// GetField returns the field name.
//
// Mirrors RegexpQueryNode#getField().
func (n *RegexpQueryNode) GetField() string { return n.field }

// GetFieldAsString returns the field name as a string.
//
// Mirrors RegexpQueryNode#getFieldAsString().
func (n *RegexpQueryNode) GetFieldAsString() string { return n.field }

// SetField sets the field name.
//
// Mirrors RegexpQueryNode#setField(CharSequence).
func (n *RegexpQueryNode) SetField(field string) { n.field = field }

// ToQueryString renders /text/, prefixed by the field when it is not the
// default one.
//
// Mirrors RegexpQueryNode#toQueryString(EscapeQuerySyntax).
func (n *RegexpQueryNode) ToQueryString(escapeSyntaxParser EscapeQuerySyntax) string {
	if n.IsDefaultField(n.field) {
		return "/" + n.text + "/"
	}
	return n.field + ":/" + n.text + "/"
}

// CloneTree deep-copies this node.
//
// Mirrors RegexpQueryNode#cloneTree().
func (n *RegexpQueryNode) CloneTree() QueryNode {
	clone := &RegexpQueryNode{QueryNodeImpl: NewQueryNodeImpl(nil)}
	clone.SetLeaf(true)
	clone.field = n.field
	clone.text = n.text
	return clone
}

// String returns the pseudo-XML debug form.
//
// Mirrors RegexpQueryNode#toString().
func (n *RegexpQueryNode) String() string {
	return "<regexp field='" + n.field + "' term='" + n.text + "'/>"
}
