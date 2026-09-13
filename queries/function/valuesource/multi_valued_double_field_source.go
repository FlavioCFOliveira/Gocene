// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/function/valuesource/MultiValuedDoubleFieldSource.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// MultiValuedDoubleFieldSource obtains double field values from SortedNumericDocValues,
// selecting a single value per document with the configured selector.
//
// Java subclasses DoubleFieldSource and overrides only getSortField, description,
// getNumericDocValues, equals and hashCode; the Go port embeds DoubleFieldSource
// and installs the doc-values override, so every other behaviour — including
// the per-document exists/advance contract — is inherited unchanged.
//
// Port of org.apache.lucene.queries.function.valuesource.MultiValuedDoubleFieldSource.
type MultiValuedDoubleFieldSource struct {
	DoubleFieldSource
	selector search.SortedNumericSelectorType
}

// NewMultiValuedDoubleFieldSource creates a MultiValuedDoubleFieldSource.
//
// Mirrors MultiValuedDoubleFieldSource(String, SortedNumericSelector.Type).
func NewMultiValuedDoubleFieldSource(field string, selector search.SortedNumericSelectorType) *MultiValuedDoubleFieldSource {
	s := &MultiValuedDoubleFieldSource{
		DoubleFieldSource: *NewDoubleFieldSource(field),
		selector:          selector,
	}
	s.self = s
	s.numericDocValues = s.selectedNumericDocValues
	return s
}

// Description returns "double(<field>,<selector>)".
//
// Mirrors MultiValuedDoubleFieldSource.description().
func (s *MultiValuedDoubleFieldSource) Description() string {
	return fmt.Sprintf("double(%s,%s)", s.Field, s.selector)
}

// selectedNumericDocValues renders the overridden protected
// MultiValuedDoubleFieldSource.getNumericDocValues(Map, LeafReaderContext): it reads the
// field's SortedNumericDocValues and reduces them to a single value per
// document with SortedNumericSelector.wrap.
//
// DocValues.getSortedNumeric never returns null in Java — it substitutes an
// empty instance — so there is no missing-values branch here.
func (s *MultiValuedDoubleFieldSource) selectedNumericDocValues(_ function.Context, readerContext *index.LeafReaderContext) (index.NumericDocValues, error) {
	sortedDv, err := index.GetSortedNumeric(readerContext.LeafReader(), s.Field)
	if err != nil {
		return nil, err
	}
	return search.WrapSortedNumeric(sortedDv, s.selector, spi.SortFieldTypeDouble)
}

// Equals reports value equality.
//
// Mirrors MultiValuedDoubleFieldSource.equals(Object), whose getClass() test becomes the
// Go type assertion.
func (s *MultiValuedDoubleFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*MultiValuedDoubleFieldSource)
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
// Mirrors MultiValuedDoubleFieldSource.hashCode(): super.hashCode() plus the selector.
func (s *MultiValuedDoubleFieldSource) HashCode() int32 {
	return s.DoubleFieldSource.HashCode() + int32(s.selector)
}

var _ function.ValueSource = (*MultiValuedDoubleFieldSource)(nil)
