// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TotalTermFreqValueSource returns the total term freq (sum of term freqs
// across all documents).
//
// Mirrors
// org.apache.lucene.queries.function.valuesource.TotalTermFreqValueSource
// (@lucene.internal).
type TotalTermFreqValueSource struct {
	function.BaseValueSource
	field        string
	indexedField string
	val          string
	indexedBytes []byte
}

var _ function.ValueSource = (*TotalTermFreqValueSource)(nil)

// NewTotalTermFreqValueSource mirrors
// TotalTermFreqValueSource(String, String, String, BytesRef).
func NewTotalTermFreqValueSource(field, val, indexedField string, indexedBytes []byte) *TotalTermFreqValueSource {
	return &TotalTermFreqValueSource{
		field:        field,
		indexedField: indexedField,
		val:          val,
		indexedBytes: indexedBytes,
	}
}

// Name mirrors name().
func (v *TotalTermFreqValueSource) Name() string { return "totaltermfreq" }

// Description mirrors description().
func (v *TotalTermFreqValueSource) Description() string {
	return v.Name() + "(" + v.field + "," + v.val + ")"
}

// GetValues mirrors getValues(Map, LeafReaderContext), which returns the
// FunctionValues that createWeight stashed in the context under this source.
func (v *TotalTermFreqValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	values, ok := ctx[v].(function.FunctionValues)
	if !ok {
		return nil, fmt.Errorf("valuesource: totaltermfreq requires CreateWeight to run first")
	}
	return values, nil
}

// CreateWeight mirrors createWeight(Map, IndexSearcher).
func (v *TotalTermFreqValueSource) CreateWeight(ctx function.Context, searcher any) error {
	s, ok := searcher.(*search.IndexSearcher)
	if !ok || s == nil {
		return fmt.Errorf("valuesource: totaltermfreq requires an *search.IndexSearcher")
	}
	leaves, err := s.GetTopReaderContext().Leaves()
	if err != nil {
		return err
	}
	var totalTermFreq int64
	term := index.NewTermFromBytes(v.indexedField, v.indexedBytes)
	for _, readerContext := range leaves {
		freq, err := leafTotalTermFreq(readerContext.LeafReader(), term)
		if err != nil {
			return err
		}
		totalTermFreq += freq
	}
	ttf := totalTermFreq
	ctx[v] = docvalues.NewLongDocValues(v, func(int) (int64, error) { return ttf, nil })
	return nil
}

// HashCode mirrors
// getClass().hashCode() + indexedField.hashCode()*29 + indexedBytes.hashCode().
func (v *TotalTermFreqValueSource) HashCode() int32 {
	return hashString("TotalTermFreqValueSource") + hashString(v.indexedField)*29 + hashBytes(v.indexedBytes)
}

// Equals mirrors equals(Object).
func (v *TotalTermFreqValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*TotalTermFreqValueSource)
	if !ok {
		return false
	}
	return v.indexedField == o.indexedField && bytes.Equal(v.indexedBytes, o.indexedBytes)
}

// leafTotalTermFreq renders LeafReader.totalTermFreq(Term). spi.LeafReader does
// not declare it, so the concrete readers that do are reached through this
// narrow interface.
func leafTotalTermFreq(reader index.LeafReader, term *index.Term) (int64, error) {
	type totalTermFreqReader interface {
		TotalTermFreq(term index.Term) (int64, error)
	}
	r, ok := reader.(totalTermFreqReader)
	if !ok {
		return 0, fmt.Errorf("valuesource: leaf reader %T does not expose TotalTermFreq", reader)
	}
	return r.TotalTermFreq(*term)
}
