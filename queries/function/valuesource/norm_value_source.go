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

// NormValueSource returns the decoded norm for every document.
//
// Note that the configured Similarity for the field must be a subclass of
// TFIDFSimilarity, the contribution of the TF must be 1 when the freq is 1, and
// the contribution of the IDF must be 1 when docFreq == docCount == 1.
//
// Mirrors org.apache.lucene.queries.function.valuesource.NormValueSource
// (@lucene.internal).
type NormValueSource struct {
	function.BaseValueSource
	field string
}

var _ function.ValueSource = (*NormValueSource)(nil)

// NewNormValueSource mirrors NormValueSource(String).
func NewNormValueSource(field string) *NormValueSource {
	return &NormValueSource{field: field}
}

// Name mirrors name().
func (v *NormValueSource) Name() string { return "norm" }

// Description mirrors description().
func (v *NormValueSource) Description() string { return v.Name() + "(" + v.field + ")" }

// CreateWeight mirrors createWeight(Map, IndexSearcher).
func (v *NormValueSource) CreateWeight(ctx function.Context, searcher any) error {
	ctx["searcher"] = searcher
	return nil
}

// GetValues mirrors getValues(Map, LeafReaderContext).
func (v *NormValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	searcher, err := searcherFromContext(ctx)
	if err != nil {
		return nil, err
	}
	similarity := asTFIDF(searcher.GetSimilarity(), v.field)
	if similarity == nil {
		return nil, fmt.Errorf("requires a TFIDFSimilarity (such as ClassicSimilarity)")
	}
	// Only works if the contribution of the tf is 1 when the freq is 1 and the
	// contribution of the idf is 1 when docCount == docFreq == 1.
	simScorer := similarity.Scorer104(
		1,
		search.NewCollectionStatistics(v.field, 1, 1, 1, 1),
		search.NewTermStatistics(index.NewTermFromBytes(v.field, []byte("bogus")), 1, 1),
	)
	norms, err := readerContext.LeafReader().GetNormValues(v.field)
	if err != nil {
		return nil, err
	}

	lastDocID := -1
	return docvalues.NewFloatDocValues(v, func(docID int) (float32, error) {
		if docID < lastDocID {
			return 0, fmt.Errorf("docs out of order: lastDocID=%d docID=%d", lastDocID, docID)
		}
		lastDocID = docID
		norm := int64(1)
		if norms != nil {
			ok, err := norms.AdvanceExact(docID)
			if err != nil {
				return 0, err
			}
			if ok {
				norm, err = norms.LongValue()
				if err != nil {
					return 0, err
				}
			}
		}
		return simScorer.Score104(1, norm), nil
	}), nil
}

// Equals mirrors equals(Object).
func (v *NormValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*NormValueSource)
	if !ok {
		return false
	}
	return v.field == o.field
}

// HashCode mirrors getClass().hashCode() + field.hashCode().
func (v *NormValueSource) HashCode() int32 {
	return hashString("NormValueSource") + hashString(v.field)
}
