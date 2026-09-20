// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/function/valuesource/MultiValuedLongFieldSource.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// MultiValuedLongFieldSource obtains long field values from SortedNumericDocValues,
// selecting a single value per document with the configured selector.
//
// Java subclasses LongFieldSource and overrides only getSortField, description,
// getNumericDocValues, equals and hashCode; the Go port embeds LongFieldSource
// and installs the doc-values override, so every other behaviour — including
// the per-document exists/advance contract — is inherited unchanged.
//
// Port of org.apache.lucene.queries.function.valuesource.MultiValuedLongFieldSource.
type MultiValuedLongFieldSource struct {
	LongFieldSource
	selector search.SortedNumericSelectorType
}

// NewMultiValuedLongFieldSource creates a MultiValuedLongFieldSource.
//
// Mirrors MultiValuedLongFieldSource(String, SortedNumericSelector.Type).
func NewMultiValuedLongFieldSource(field string, selector search.SortedNumericSelectorType) *MultiValuedLongFieldSource {
	s := &MultiValuedLongFieldSource{
		LongFieldSource: *NewLongFieldSource(field),
		selector:        selector,
	}
	s.self = s
	s.numericDocValues = s.selectedNumericDocValues
	return s
}

// Description returns "long(<field>,<selector>)".
//
// Mirrors MultiValuedLongFieldSource.description().
func (s *MultiValuedLongFieldSource) Description() string {
	return fmt.Sprintf("long(%s,%s)", s.Field, s.selector)
}

// selectedNumericDocValues renders the overridden protected
// MultiValuedLongFieldSource.getNumericDocValues(Map, LeafReaderContext): it reads the
// field's SortedNumericDocValues and reduces them to a single value per
// document with SortedNumericSelector.wrap.
//
// DocValues.getSortedNumeric never returns null in Java — it substitutes an
// empty instance — so there is no missing-values branch here.
func (s *MultiValuedLongFieldSource) selectedNumericDocValues(_ function.Context, readerContext *index.LeafReaderContext) (index.NumericDocValues, error) {
	sortedDv, err := index.GetSortedNumeric(readerContext.LeafReader(), s.Field)
	if err != nil {
		return nil, err
	}
	return search.WrapSortedNumeric(sortedDv, s.selector, spi.SortFieldTypeLong)
}

// Equals reports value equality.
//
// Mirrors MultiValuedLongFieldSource.equals(Object), whose getClass() test becomes the
// Go type assertion.
func (s *MultiValuedLongFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*MultiValuedLongFieldSource)
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
// Mirrors MultiValuedLongFieldSource.hashCode(): super.hashCode() plus the selector.
func (s *MultiValuedLongFieldSource) HashCode() int32 {
	return s.LongFieldSource.HashCode() + int32(s.selector)
}

var _ function.ValueSource = (*MultiValuedLongFieldSource)(nil)
