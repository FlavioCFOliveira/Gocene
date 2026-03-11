// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TextField is a field for tokenized, indexed text content.
// The text is tokenized and indexed, making it searchable.
// The field can be stored or not, depending on the constructor used.
//
// This is the Go port of Lucene's org.apache.lucene.document.TextField.
type TextField struct {
	*Field
}

var (
	// TextFieldTypeStored is the FieldType for a stored TextField.
	// The text is tokenized, indexed, and stored.
	TextFieldTypeStored *FieldType

	// TextFieldTypeNotStored is the FieldType for a non-stored TextField.
	// The text is tokenized and indexed, but not stored.
	TextFieldTypeNotStored *FieldType
)

func init() {
	// Initialize the FieldTypes
	TextFieldTypeStored = NewFieldType().
		SetIndexed(true).
		SetStored(true).
		SetTokenized(true).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	TextFieldTypeStored.Freeze()

	TextFieldTypeNotStored = NewFieldType().
		SetIndexed(true).
		SetStored(false).
		SetTokenized(true).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	TextFieldTypeNotStored.Freeze()
}

// NewTextField creates a new TextField with the given name and value.
// If stored is true, the field value will be stored in the index.
func NewTextField(name string, value string, stored bool) (*TextField, error) {
	ft := TextFieldTypeNotStored
	if stored {
		ft = TextFieldTypeStored
	}

	field, err := NewField(name, value, ft)
	if err != nil {
		return nil, err
	}

	return &TextField{Field: field}, nil
}

// NewTextFieldFromReader creates a new TextField from an io.Reader.
// The content is read during indexing. This field type is not stored.
func NewTextFieldFromReader(name string, reader io.Reader) (*TextField, error) {
	field, err := NewField(name, reader, TextFieldTypeNotStored)
	if err != nil {
		return nil, err
	}

	return &TextField{Field: field}, nil
}
