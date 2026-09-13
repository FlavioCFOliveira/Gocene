// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SumTotalTermFreqValueSource returns the number of tokens (sum of term freqs
// across all documents, across all terms).
//
// Mirrors
// org.apache.lucene.queries.function.valuesource.SumTotalTermFreqValueSource
// (@lucene.internal).
type SumTotalTermFreqValueSource struct {
	function.BaseValueSource
	indexedField string
}

var _ function.ValueSource = (*SumTotalTermFreqValueSource)(nil)

// NewSumTotalTermFreqValueSource mirrors
// SumTotalTermFreqValueSource(String).
func NewSumTotalTermFreqValueSource(indexedField string) *SumTotalTermFreqValueSource {
	return &SumTotalTermFreqValueSource{indexedField: indexedField}
}

// Name mirrors name().
func (v *SumTotalTermFreqValueSource) Name() string { return "sumtotaltermfreq" }

// Description mirrors description().
func (v *SumTotalTermFreqValueSource) Description() string {
	return v.Name() + "(" + v.indexedField + ")"
}

// GetValues mirrors getValues(Map, LeafReaderContext), which returns the
// FunctionValues that createWeight stashed in the context under this source.
func (v *SumTotalTermFreqValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	values, ok := ctx[v].(function.FunctionValues)
	if !ok {
		return nil, fmt.Errorf("valuesource: sumtotaltermfreq requires CreateWeight to run first")
	}
	return values, nil
}

// CreateWeight mirrors createWeight(Map, IndexSearcher).
func (v *SumTotalTermFreqValueSource) CreateWeight(ctx function.Context, searcher any) error {
	s, ok := searcher.(*search.IndexSearcher)
	if !ok || s == nil {
		return fmt.Errorf("valuesource: sumtotaltermfreq requires an *search.IndexSearcher")
	}
	leaves, err := s.GetTopReaderContext().Leaves()
	if err != nil {
		return err
	}
	var sumTotalTermFreq int64
	for _, readerContext := range leaves {
		terms, err := index.GetTerms(readerContext.LeafReader(), v.indexedField)
		if err != nil {
			return err
		}
		value, err := terms.GetSumTotalTermFreq()
		if err != nil {
			return err
		}
		sumTotalTermFreq += value
	}
	ttf := sumTotalTermFreq
	ctx[v] = docvalues.NewLongDocValues(v, func(int) (int64, error) { return ttf, nil })
	return nil
}

// HashCode mirrors getClass().hashCode() + indexedField.hashCode().
func (v *SumTotalTermFreqValueSource) HashCode() int32 {
	return hashString("SumTotalTermFreqValueSource") + hashString(v.indexedField)
}

// Equals mirrors equals(Object).
func (v *SumTotalTermFreqValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*SumTotalTermFreqValueSource)
	if !ok {
		return false
	}
	return v.indexedField == o.indexedField
}
