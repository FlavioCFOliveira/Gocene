// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/function/valuesource/MultiValuedIntFieldSource.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// MultiValuedIntFieldSource obtains int field values from SortedNumericDocValues,
// selecting a single value per document with the configured selector.
//
// Java subclasses IntFieldSource and overrides only getSortField, description,
// getNumericDocValues, equals and hashCode; the Go port embeds IntFieldSource
// and installs the doc-values override, so every other behaviour — including
// the per-document exists/advance contract — is inherited unchanged.
//
// Port of org.apache.lucene.queries.function.valuesource.MultiValuedIntFieldSource.
type MultiValuedIntFieldSource struct {
	IntFieldSource
	selector search.SortedNumericSelectorType
}

// NewMultiValuedIntFieldSource creates a MultiValuedIntFieldSource.
//
// Mirrors MultiValuedIntFieldSource(String, SortedNumericSelector.Type).
func NewMultiValuedIntFieldSource(field string, selector search.SortedNumericSelectorType) *MultiValuedIntFieldSource {
	s := &MultiValuedIntFieldSource{
		IntFieldSource: *NewIntFieldSource(field),
		selector:       selector,
	}
	s.self = s
	s.numericDocValues = s.selectedNumericDocValues
	return s
}

// Description returns "int(<field>,<selector>)".
//
// Mirrors MultiValuedIntFieldSource.description().
func (s *MultiValuedIntFieldSource) Description() string {
	return fmt.Sprintf("int(%s,%s)", s.Field, s.selector)
}

// selectedNumericDocValues renders the overridden protected
// MultiValuedIntFieldSource.getNumericDocValues(Map, LeafReaderContext): it reads the
// field's SortedNumericDocValues and reduces them to a single value per
// document with SortedNumericSelector.wrap.
//
// DocValues.getSortedNumeric never returns null in Java — it substitutes an
// empty instance — so there is no missing-values branch here.
func (s *MultiValuedIntFieldSource) selectedNumericDocValues(_ function.Context, readerContext *index.LeafReaderContext) (index.NumericDocValues, error) {
	sortedDv, err := index.GetSortedNumeric(readerContext.LeafReader(), s.Field)
	if err != nil {
		return nil, err
	}
	return search.WrapSortedNumeric(sortedDv, s.selector, spi.SortFieldTypeInt)
}

// Equals reports value equality.
//
// Mirrors MultiValuedIntFieldSource.equals(Object), whose getClass() test becomes the
// Go type assertion.
func (s *MultiValuedIntFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*MultiValuedIntFieldSource)
	if !ok || o == nil {
		return false
	}
	if s.selector != o.selector {
		return false
	}
	return s.Field == o.Field
}

// HashCode returns a stable hash.
//
// Mirrors MultiValuedIntFieldSource.hashCode(): super.hashCode() plus the selector.
func (s *MultiValuedIntFieldSource) HashCode() int32 {
	return s.IntFieldSource.HashCode() + int32(s.selector)
}

var _ function.ValueSource = (*MultiValuedIntFieldSource)(nil)
