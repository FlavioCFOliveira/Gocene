// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
)

// FacetField is a field used for storing facet values.
// It is a Go port of Lucene's org.apache.lucene.facet.FacetField.
type FacetField struct {
	dim  string
	path []string
	ft   *document.FieldType
}

// FacetFieldType is the field type used for storing facet values.
// In Lucene, this is a default FieldType.
var FacetFieldType = document.NewFieldType()

// NewFacetField creates a new FacetField from the given dimension and path.
func NewFacetField(dim string, path ...string) (*FacetField, error) {
	if dim == "" {
		return nil, fmt.Errorf("dimension cannot be empty")
	}
	for _, label := range path {
		if label == "" {
			return nil, fmt.Errorf("path components cannot be empty")
		}
	}
	if len(path) == 0 {
		return nil, fmt.Errorf("path must have at least one element")
	}

	return &FacetField{
		dim:  dim,
		path: path,
		ft:   FacetFieldType,
	}, nil
}

// Name returns the name of the field.
func (ff *FacetField) Name() string {
	return "dummy"
}

// FieldType returns the FieldType for this field.
func (ff *FacetField) FieldType() *document.FieldType {
	return ff.ft
}

// StringValue returns the string value of the field.
func (ff *FacetField) StringValue() string {
	return ""
}

// ReaderValue returns a reader for the field value.
func (ff *FacetField) ReaderValue() io.Reader {
	return nil
}

// BinaryValue returns the binary value of the field.
func (ff *FacetField) BinaryValue() []byte {
	return nil
}

// NumericValue returns the numeric value of the field.
func (ff *FacetField) NumericValue() interface{} {
	return nil
}

// TokenStream returns a TokenStream for the field value.
func (ff *FacetField) TokenStream() analysis.TokenStream {
	return nil
}

// Dim returns the dimension of the facet.
func (ff *FacetField) Dim() string {
	return ff.dim
}

// Path returns the path of the facet.
func (ff *FacetField) Path() []string {
	return ff.path
}

// String returns a string representation of the FacetField.
func (ff *FacetField) String() string {
	return fmt.Sprintf("FacetField(dim=%s path=%v)", ff.dim, ff.path)
}

// VerifyLabel verifies that the label is not null or empty.
// This is a port of FacetField.verifyLabel.
func VerifyLabel(label string) error {
	if label == "" {
		return fmt.Errorf("empty or null components not allowed; got: %s", label)
	}
	return nil
}
