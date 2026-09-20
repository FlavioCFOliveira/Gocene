// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// FullPrecisionFloatVectorSimilarityValuesSource provides double values that compute vector
// similarity between a query vector and raw full precision vectors indexed in a
// KnnFloatVectorField.
//
// Mirrors org.apache.lucene.search.FullPrecisionFloatVectorSimilarityValuesSource (Lucene 10.5.0).
type FullPrecisionFloatVectorSimilarityValuesSource struct {
	queryVector              []float32
	fieldName                string
	vectorSimilarityFunction index.VectorSimilarityFunction
}

// NewFullPrecisionFloatVectorSimilarityValuesSource creates a new FullPrecisionFloatVectorSimilarityValuesSource.
func NewFullPrecisionFloatVectorSimilarityValuesSource(
	vector []float32,
	fieldName string,
	vectorSimilarityFunction index.VectorSimilarityFunction,
) *FullPrecisionFloatVectorSimilarityValuesSource {
	return &FullPrecisionFloatVectorSimilarityValuesSource{
		queryVector:              vector,
		fieldName:                fieldName,
		vectorSimilarityFunction: vectorSimilarityFunction,
	}
}

// NewFullPrecisionFloatVectorSimilarityValuesSourceDefault creates a new FullPrecisionFloatVectorSimilarityValuesSource
// using the configured vector similarity function for the field.
func NewFullPrecisionFloatVectorSimilarityValuesSourceDefault(vector []float32, fieldName string) *FullPrecisionFloatVectorSimilarityValuesSource {
	return &FullPrecisionFloatVectorSimilarityValuesSource{
		queryVector: vector,
		fieldName:   fieldName,
	}
}

// GetSimilarityScores is a sugar method to fetch full precision similarity score values.
func (s *FullPrecisionFloatVectorSimilarityValuesSource) GetSimilarityScores(ctx *index.LeafReaderContext) (DoubleValues, error) {
	return s.GetValues(ctx, nil)
}

// GetValues returns the similarity scores for documents in the leaf.
// Mirrors org.apache.lucene.search.FullPrecisionFloatVectorSimilarityValuesSource.getValues.
func (s *FullPrecisionFloatVectorSimilarityValuesSource) GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (DoubleValues, error) {
	if ctx == nil {
		return nil, fmt.Errorf("leaf reader context must not be nil")
	}
	reader := ctx.LeafReader()
	if reader == nil {
		return nil, fmt.Errorf("leaf reader must not be nil")
	}

	rawVectorValues, err := reader.GetFloatVectorValues(s.fieldName)
	if err != nil {
		return nil, err
	}
	if rawVectorValues == nil {
		// Lucene calls FloatVectorValues.checkField(ctx.reader(), fieldName)
		// which throws if the field doesn't exist as a vector field.
		// In Gocene, we follow this by checking field info.
		fi := reader.GetFieldInfos().FieldInfo(s.fieldName)
		if fi == nil {
			return nil, fmt.Errorf("field %q not found in index", s.fieldName)
		}
		return nil, fmt.Errorf("field %q is not a float vector field", s.fieldName)
	}

	fi := reader.GetFieldInfos().FieldInfo(s.fieldName)
	if fi == nil || fi.VectorDimension() != len(s.queryVector) {
		return nil, fmt.Errorf("query vector dimension does not match field dimension: %d != %d",
			len(s.queryVector), func() int {
				if fi == nil {
					return -1
				}
				return fi.VectorDimension()
			}())
	}

	// Java's LeafReader.getFloatVectorValues returns the full
	// org.apache.lucene.index.FloatVectorValues, which carries rescorer(),
	// iterator() and vectorValue(ord). Gocene splits that contract in two:
	// spi.FloatVectorValues (what LeafReader returns) is a narrow doc-walking
	// surface, while index.FloatVectorValues carries the KnnVectorValues
	// members this method needs. The reader's concrete values are recovered
	// with a narrow assertion until the two contracts are reconciled.
	vectorValues, ok := rawVectorValues.(index.FloatVectorValues)
	if !ok {
		return nil, fmt.Errorf(
			"field %q: float vector values do not expose the KnnVectorValues surface (%T)",
			s.fieldName, rawVectorValues)
	}

	if s.vectorSimilarityFunction == nil {
		rescorer, err := vectorValues.Rescorer(s.queryVector)
		if err != nil {
			return nil, err
		}
		if rescorer == nil {
			return nil, nil // Corresponds to DoubleValues.EMPTY
		}
		scorer, ok := rescorer.(VectorScorer)
		if !ok {
			return nil, fmt.Errorf(
				"field %q: rescorer is not a VectorScorer (%T)", s.fieldName, rescorer)
		}
		return &floatVectorSimilarityValues{
			scorer:   scorer,
			iterator: scorer.Iterator(),
		}, nil
	}

	iterator := vectorValues.Iterator()
	return &floatVectorSimilarityValues{
		vectorValues:     vectorValues,
		docIndexIterator: iterator,
		simFunc:          s.vectorSimilarityFunction,
		queryVector:      s.queryVector,
	}, nil
}

// Field returns the name of the KnnFloatVectorField this source reads.
//
// Apache Lucene 10.5.0 declares no field() on DoubleValuesSource; the accessor
// exists because Gocene's search.DoubleValuesSource carries one, and it returns
// the fieldName the Java class holds privately.
func (s *FullPrecisionFloatVectorSimilarityValuesSource) Field() string {
	return s.fieldName
}

// NeedsScores reports whether the source consumes the underlying query's scores.
func (s *FullPrecisionFloatVectorSimilarityValuesSource) NeedsScores() bool {
	return false
}

// Rewrite returns the source unchanged.
func (s *FullPrecisionFloatVectorSimilarityValuesSource) Rewrite(searcher *IndexSearcher) DoubleValuesSource {
	return s
}

// IsCacheable reports whether results are safe to cache for this leaf.
func (s *FullPrecisionFloatVectorSimilarityValuesSource) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// floatVectorSimilarityValues is the internal iterator implementation.
type floatVectorSimilarityValues struct {
	scorer   VectorScorer
	iterator DocIdSetIterator

	vectorValues     index.FloatVectorValues
	docIndexIterator spi.DocIndexIterator
	simFunc          index.VectorSimilarityFunction
	queryVector      []float32
}

// DoubleValue returns the similarity score for the current document.
func (v *floatVectorSimilarityValues) DoubleValue() (float64, error) {
	if v.scorer != nil {
		score, err := v.scorer.Score()
		return float64(score), err
	}
	if v.vectorValues != nil && v.docIndexIterator != nil {
		vector, err := v.vectorValues.VectorValue(v.docIndexIterator.Index())
		if err != nil {
			return 0, err
		}
		score := v.simFunc.CompareFloat(v.queryVector, vector)
		return float64(score), nil
	}
	return 0, fmt.Errorf("invalid state: neither scorer nor vector values provided")
}

// AdvanceExact positions the reader on doc and reports whether it is present.
func (v *floatVectorSimilarityValues) AdvanceExact(doc int) (bool, error) {
	if v.iterator != nil {
		currentDoc := v.iterator.DocID()
		if doc < currentDoc {
			return false, nil
		}
		if currentDoc == doc {
			return true, nil
		}
		next, err := v.iterator.Advance(doc)
		if err != nil {
			return false, err
		}
		return next == doc, nil
	}
	if v.docIndexIterator != nil {
		currentDoc := v.docIndexIterator.DocID()
		if doc < currentDoc {
			return false, nil
		}
		if currentDoc == doc {
			return true, nil
		}
		next, err := v.docIndexIterator.Advance(doc)
		if err != nil {
			return false, err
		}
		return next == doc, nil
	}
	return false, nil
}
