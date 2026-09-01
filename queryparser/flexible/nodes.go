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

// Compile-time assertions that concrete types satisfy the marker interfaces.
var (
	_ FieldableNode           = (*FieldQueryNode)(nil)
	_ TextableQueryNode       = (*FieldQueryNode)(nil)
	_ FieldableNode           = (*QuotedFieldQueryNode)(nil)
	_ TextableQueryNode       = (*QuotedFieldQueryNode)(nil)
	_ FieldableNode           = (*TokenizedPhraseQueryNode)(nil)
	_ FieldValuePairQueryNode = (*FieldQueryNode)(nil)
)
