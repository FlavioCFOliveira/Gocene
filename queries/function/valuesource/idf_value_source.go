// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// IDFValueSource returns TFIDFSimilarity.idf(long, long) for every document.
//
// Note that the configured Similarity for the field must be a subclass of
// TFIDFSimilarity.
//
// Mirrors org.apache.lucene.queries.function.valuesource.IDFValueSource
// (@lucene.internal), which extends DocFreqValueSource.
type IDFValueSource struct {
	DocFreqValueSource
}

var _ function.ValueSource = (*IDFValueSource)(nil)

// NewIDFValueSource mirrors IDFValueSource(String, String, String, BytesRef).
func NewIDFValueSource(field, val, indexedField string, indexedBytes []byte) *IDFValueSource {
	return &IDFValueSource{DocFreqValueSource: *NewDocFreqValueSource(field, val, indexedField, indexedBytes)}
}

// Name mirrors name().
func (v *IDFValueSource) Name() string { return "idf" }

// Description mirrors the inherited description(), which calls the overridden
// name().
func (v *IDFValueSource) Description() string {
	return v.Name() + "(" + v.field + "," + v.val + ")"
}

// GetValues mirrors getValues(Map, LeafReaderContext).
func (v *IDFValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	searcher, err := searcherFromContext(ctx)
	if err != nil {
		return nil, err
	}
	sim := asTFIDF(searcher.GetSimilarity(), v.field)
	if sim == nil {
		return nil, fmt.Errorf("requires a TFIDFSimilarity (such as ClassicSimilarity)")
	}
	docfreq, err := index.DocFreq(searcher.GetIndexReader(), index.NewTermFromBytes(v.indexedField, v.indexedBytes))
	if err != nil {
		return nil, err
	}
	idf := sim.Idf(int64(docfreq), int64(searcher.GetIndexReader().MaxDoc()))
	return newConstDoubleDocValues(float64(idf), v), nil
}

// asTFIDF tries extra hard to cast the sim to TFIDFSimilarity. Mirrors the
// package-private static IDFValueSource#asTFIDF(Similarity, String).
func asTFIDF(sim search.Similarity, field string) *search.TFIDFSimilarity {
	for {
		wrapper, ok := sim.(*search.PerFieldSimilarityWrapper)
		if !ok {
			break
		}
		sim = wrapper.GetFieldSimilarity(field)
	}
	if tfidf, ok := sim.(*search.TFIDFSimilarity); ok {
		return tfidf
	}
	return nil
}
