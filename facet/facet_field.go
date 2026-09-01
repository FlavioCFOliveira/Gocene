// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FacetField is a field used for facets.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetField.
type FacetField struct {
	document.IndexableField
	dim  string
	path []string
}

// TYPE is the FieldType used for storing facet values.
var TYPE = index.NewFieldType()

// NewFacetField creates a new FacetField from dim and path.
func NewFacetField(dim string, path ...string) *FacetField {
	verifyLabel(dim)
	for _, label := range path {
		verifyLabel(label)
	}
	if len(path) == 0 {
		panic("path must have at least one element")
	}

	return &FacetField{
		IndexableField: document.NewField("dummy", "dummy", TYPE),
		dim:            dim,
		path:           path,
	}
}

// Dim returns the dimension for this field.
func (f *FacetField) Dim() string {
	return f.dim
}

// Path returns the path for this field.
func (f *FacetField) Path() []string {
	return f.path
}

func (f *FacetField) String() string {
	return fmt.Sprintf("FacetField(dim=%s path=%v)", f.dim, f.path)
}

// verifyLabel verifies the label is not null or empty string.
func verifyLabel(label string) {
	if label == "" {
		panic("empty or null components not allowed; got: \"\"")
	}
}
