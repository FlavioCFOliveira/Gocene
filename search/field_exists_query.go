// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/FieldExistsQuery.java

// FieldExistsQuery is a query that matches documents that contain either a
// KnnFloatVectorField, a KnnByteVectorField, or a field that indexes norms or
// doc values.
type FieldExistsQuery struct {
	*BaseQuery
	field string
}

// NewFieldExistsQuery creates a query that will match documents that have a value for the given field.
func NewFieldExistsQuery(field string) *FieldExistsQuery {
	if field == "" {
		panic("field must not be empty")
	}
	return &FieldExistsQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
	}
}

// Field returns the field name for this query.
//
// Mirrors FieldExistsQuery.getField().
func (q *FieldExistsQuery) Field() string {
	return q.field
}

// GetDocValuesDocIdSetIterator returns a DocIdSetIterator from the given field
// or nil if the field doesn't exist in the reader or if the reader has no doc
// values for the field.
//
// Mirrors the static
// FieldExistsQuery.getDocValuesDocIdSetIterator(String, LeafReader).
func GetDocValuesDocIdSetIterator(field string, reader index.LeafReader) (DocIdSetIterator, error) {
	fieldInfo := reader.GetFieldInfos().FieldInfo(field)
	if fieldInfo == nil {
		return nil, nil
	}
	switch fieldInfo.DocValuesType() {
	case spi.DocValuesTypeNone:
		return nil, nil
	case spi.DocValuesTypeNumeric:
		dv, err := reader.GetNumericDocValues(field)
		if err != nil || dv == nil {
			return nil, err
		}
		return asDocValuesIterator(dv)
	case spi.DocValuesTypeBinary:
		dv, err := reader.GetBinaryDocValues(field)
		if err != nil || dv == nil {
			return nil, err
		}
		return asDocValuesIterator(dv)
	case spi.DocValuesTypeSorted:
		dv, err := reader.GetSortedDocValues(field)
		if err != nil || dv == nil {
			return nil, err
		}
		return asDocValuesIterator(dv)
	case spi.DocValuesTypeSortedNumeric:
		dv, err := reader.GetSortedNumericDocValues(field)
		if err != nil || dv == nil {
			return nil, err
		}
		return asDocValuesIterator(dv)
	case spi.DocValuesTypeSortedSet:
		dv, err := reader.GetSortedSetDocValues(field)
		if err != nil || dv == nil {
			return nil, err
		}
		return asDocValuesIterator(dv)
	default:
		return nil, fmt.Errorf("FieldExistsQuery: unexpected doc values type %v for field %q", fieldInfo.DocValuesType(), field)
	}
}

// asDocValuesIterator renders the Java assignment of a doc-values instance to a
// DocIdSetIterator variable. Java's doc-values classes all extend
// DocValuesIterator, itself a DocIdSetIterator; Gocene's spi doc-values
// contracts carry the iteration primitives (DocID/NextDoc/Advance/Cost) but the
// numeric and binary ones do not declare intoBitSet/docIDRunEnd, so the widening
// is expressed as a type assertion and a missing member is reported rather than
// silently dropped.
func asDocValuesIterator(dv any) (DocIdSetIterator, error) {
	it, ok := dv.(DocIdSetIterator)
	if !ok {
		return nil, fmt.Errorf("FieldExistsQuery: doc values of type %T are not a DocIdSetIterator", dv)
	}
	return it, nil
}

// Rewrite mirrors FieldExistsQuery.rewrite(IndexSearcher).
func (q *FieldExistsQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	reader := searcher.GetIndexReader()
	allReadersRewritable := true

	leaves, err := reader.Leaves()
	if err != nil {
		return nil, err
	}

	for _, context := range leaves {
		leaf := context.LeafReader()
		fieldInfos := leaf.GetFieldInfos()
		fieldInfo := fieldInfos.FieldInfo(q.field)

		if fieldInfo == nil {
			allReadersRewritable = false
			break
		}

		if fieldInfo.HasNorms() { // the field indexes norms
			docCount, err := indexReaderDocCount(reader, q.field)
			if err != nil {
				return nil, err
			}
			if docCount != reader.MaxDoc() {
				allReadersRewritable = false
				break
			}
		} else if fieldInfo.VectorDimension() != 0 { // the field indexes vectors
			size, err := getVectorValuesSize(fieldInfo, leaf)
			if err != nil {
				return nil, err
			}
			if size != leaf.MaxDoc() {
				allReadersRewritable = false
				break
			}
		} else if fieldInfo.DocValuesType() != spi.DocValuesTypeNone { // the field indexes doc values or points
			// This optimization is possible due to LUCENE-9334 enforcing a field to always use the
			// same data structures (all or nothing).
			terms, err := leaf.Terms(q.field)
			if err != nil {
				return nil, err
			}
			pointValues, err := leaf.GetPointValues(q.field)
			if err != nil {
				return nil, err
			}
			docValuesSkipper, err := leaf.GetDocValuesSkipper(q.field)
			if err != nil {
				return nil, err
			}

			termsFull, err := termsCoversLeaf(terms, leaf.MaxDoc())
			if err != nil {
				return nil, err
			}
			pointsFull := pointValues != nil && pointValues.GetDocCount() == leaf.MaxDoc()
			skipperFull := docValuesSkipper != nil && docValuesSkipper.DocCount() == leaf.MaxDoc()

			if !termsFull && !pointsFull && !skipperFull {
				allReadersRewritable = false
				break
			}
		} else {
			return nil, fmt.Errorf("%s", buildFieldExistsErrorMsg(fieldInfo))
		}
	}

	if allReadersRewritable {
		return Instance, nil
	}

	return q.BaseQuery.Rewrite(searcher)
}

// termsCoversLeaf renders `terms == null || terms.getDocCount() != leaf.maxDoc()`
// negated, that is: the field's terms exist and cover every document of the leaf.
func termsCoversLeaf(terms spi.Terms, maxDoc int) (bool, error) {
	if terms == nil {
		return false, nil
	}
	docCount, err := terms.GetDocCount()
	if err != nil {
		return false, err
	}
	return docCount == maxDoc, nil
}

// indexReaderDocCount renders IndexReader.getDocCount(String), whose body sums
// the per-leaf doc counts of the field's terms. Gocene's
// spi.IndexReaderInterface does not declare the method, so its Java body is
// rendered here.
func indexReaderDocCount(reader index.IndexReaderInterface, field string) (int, error) {
	leaves, err := reader.Leaves()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, leaf := range leaves {
		sub, err := leafReaderDocCount(leaf.LeafReader(), field)
		if err != nil {
			return 0, err
		}
		total += sub
	}
	return total, nil
}

// leafReaderDocCount renders LeafReader.getDocCount(String): the doc count of
// the field's terms, or 0 when the field has no terms.
func leafReaderDocCount(reader index.LeafReader, field string) (int, error) {
	terms, err := reader.Terms(field)
	if err != nil {
		return 0, err
	}
	if terms == nil {
		return 0, nil
	}
	return terms.GetDocCount()
}

// CreateWeight mirrors FieldExistsQuery.createWeight(IndexSearcher, ScoreMode, float),
// which returns an anonymous ConstantScoreWeight subclass overriding
// scorerSupplier, count and isCacheable.
func (q *FieldExistsQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	w := &fieldExistsWeight{
		query:     q,
		scoreMode: scoreMode,
	}
	w.ConstantScoreWeight = NewConstantScoreWeight(q, boost, w.scorerSupplier, w.isCacheable)
	return w, nil
}

// ToString mirrors FieldExistsQuery.toString(String).
func (q *FieldExistsQuery) ToString(field string) string {
	return fmt.Sprintf("FieldExistsQuery [field=%s]", q.field)
}

// Equals mirrors FieldExistsQuery.equals(Object).
func (q *FieldExistsQuery) Equals(other spi.Query) bool {
	if o, ok := other.(*FieldExistsQuery); ok {
		return q.field == o.field
	}
	return false
}

// HashCode mirrors FieldExistsQuery.hashCode(). classHash() is rendered as 0,
// the value Gocene's Query ports use for Java's per-class hash seed.
func (q *FieldExistsQuery) HashCode() int {
	const prime = 31
	hash := 0
	for i := 0; i < len(q.field); i++ {
		hash = prime*hash + int(q.field[i])
	}
	return prime*0 + hash
}

// fieldExistsWeight is the anonymous ConstantScoreWeight subclass declared
// inside FieldExistsQuery.createWeight.
type fieldExistsWeight struct {
	*ConstantScoreWeight
	query     *FieldExistsQuery
	scoreMode ScoreMode
}

// scorerSupplier mirrors the scorerSupplier(LeafReaderContext) override.
func (w *fieldExistsWeight) scorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	reader := ctx.LeafReader()
	fieldInfos := reader.GetFieldInfos()
	fieldInfo := fieldInfos.FieldInfo(w.query.field)

	var iterator DocIdSetIterator

	if fieldInfo == nil {
		return nil, nil
	}

	if fieldInfo.HasNorms() { // the field indexes norms
		norms, err := reader.GetNormValues(w.query.field)
		if err != nil {
			return nil, err
		}
		if norms != nil {
			iterator, err = asDocValuesIterator(norms)
			if err != nil {
				return nil, err
			}
		}
	} else if fieldInfo.VectorDimension() != 0 { // the field indexes vectors
		var err error
		switch fieldInfo.VectorEncoding() {
		case util.VectorEncodingFloat32:
			values, err2 := reader.GetFloatVectorValues(w.query.field)
			if err2 != nil {
				return nil, err2
			}
			iterator, err = knnValuesIterator(values, w.query.field)
		case util.VectorEncodingByte:
			values, err2 := reader.GetByteVectorValues(w.query.field)
			if err2 != nil {
				return nil, err2
			}
			iterator, err = knnValuesIterator(values, w.query.field)
		}
		if err != nil {
			return nil, err
		}
	} else if fieldInfo.DocValuesType() != spi.DocValuesTypeNone { // the field indexes doc values
		var err error
		iterator, err = GetDocValuesDocIdSetIterator(w.query.field, reader)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("%s", buildFieldExistsErrorMsg(fieldInfo))
	}

	if iterator == nil {
		return nil, nil
	}
	return NewConstantScoreScorerSupplierFromIterator(w.Score(), w.scoreMode, iterator), nil
}

// Count mirrors the count(LeafReaderContext) override.
func (w *fieldExistsWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	reader := ctx.LeafReader()
	fieldInfos := reader.GetFieldInfos()
	fieldInfo := fieldInfos.FieldInfo(w.query.field)

	if fieldInfo == nil {
		return 0, nil
	}

	if fieldInfo.HasNorms() { // the field indexes norms
		// If every field has a value then we can shortcut
		docCount, err := leafReaderDocCount(reader, w.query.field)
		if err != nil {
			return 0, err
		}
		if docCount == reader.MaxDoc() {
			return reader.NumDocs(), nil
		}
		return w.ConstantScoreWeight.Count(ctx)
	}

	count := -1
	if fieldInfo.HasVectorValues() { // the field indexes vectors
		size, err := getVectorValuesSize(fieldInfo, reader)
		if err != nil {
			return 0, err
		}
		count = size
	} else if fieldInfo.DocValuesType() != spi.DocValuesTypeNone { // the field indexes doc values
		if fieldInfo.DocValuesSkipIndexType() != spi.DocValuesSkipIndexTypeNone {
			docValuesSkipper, err := reader.GetDocValuesSkipper(w.query.field)
			if err != nil {
				return 0, err
			}
			if docValuesSkipper == nil {
				count = 0
			} else {
				count = docValuesSkipper.DocCount()
			}
		} else if !reader.HasDeletions() {
			// No deletions: we can use points or terms doc count as a proxy for doc values.
			if fieldInfo.PointDimensionCount() > 0 {
				pointValues, err := reader.GetPointValues(w.query.field)
				if err != nil {
					return 0, err
				}
				if pointValues == nil {
					count = 0
				} else {
					count = pointValues.GetDocCount()
				}
			} else if fieldInfo.IndexOptions() != spi.IndexOptionsNone {
				terms, err := reader.Terms(w.query.field)
				if err != nil {
					return 0, err
				}
				if terms == nil {
					count = 0
				} else {
					count, err = terms.GetDocCount()
					if err != nil {
						return 0, err
					}
				}
			}
		}
	} else {
		return 0, fmt.Errorf("%s", buildFieldExistsErrorMsg(fieldInfo))
	}

	if count == 0 {
		// One of the above cases shows the field is not present on this leaf
		return 0, nil
	} else if count == reader.MaxDoc() {
		// All docs in the leaf (live or deleted) have the field. Return the count of live docs.
		return reader.NumDocs(), nil
	} else if count >= 0 && !reader.HasDeletions() {
		// No deleted docs. The computed count can be trusted.
		return count, nil
	}
	// Some docs don't have the field and some docs are deleted.
	// Need to scan to get the correct intersection between field exists docs and live docs.
	return w.ConstantScoreWeight.Count(ctx)
}

// isCacheable mirrors the isCacheable(LeafReaderContext) override.
func (w *fieldExistsWeight) isCacheable(ctx *index.LeafReaderContext) bool {
	fieldInfos := ctx.LeafReader().GetFieldInfos()
	fieldInfo := fieldInfos.FieldInfo(w.query.field)

	if fieldInfo != nil && fieldInfo.DocValuesType() != spi.DocValuesTypeNone {
		return index.IsDocValuesCacheable(ctx, w.query.field)
	}

	return true
}

// buildFieldExistsErrorMsg mirrors the private
// FieldExistsQuery.buildErrorMsg(FieldInfo).
func buildFieldExistsErrorMsg(fieldInfo *index.FieldInfo) string {
	return "FieldExistsQuery requires that the field indexes doc values, norms or vectors, but field '" +
		fieldInfo.Name() +
		"' exists and indexes neither of these data structures"
}

// getVectorValuesSize mirrors the private
// FieldExistsQuery.getVectorValuesSize(FieldInfo, LeafReader).
func getVectorValuesSize(fi *index.FieldInfo, reader index.LeafReader) (int, error) {
	switch fi.VectorEncoding() {
	case util.VectorEncodingFloat32:
		v, err := reader.GetFloatVectorValues(fi.Name())
		if err != nil {
			return 0, err
		}
		if v == nil {
			return 0, fmt.Errorf("unexpected null float vector values")
		}
		return v.Size(), nil
	case util.VectorEncodingByte:
		v, err := reader.GetByteVectorValues(fi.Name())
		if err != nil {
			return 0, err
		}
		if v == nil {
			return 0, fmt.Errorf("unexpected null byte vector values")
		}
		return v.Size(), nil
	}
	return 0, fmt.Errorf("unsupported vector encoding")
}

// knnValuesIterator renders KnnVectorValues.iterator(), which Java reaches
// directly on the value returned by LeafReader.getFloatVectorValues /
// getByteVectorValues. Gocene's spi.FloatVectorValues and spi.ByteVectorValues
// do not declare iterator(); index.KnnVectorValues does, so the call is
// expressed as a type assertion whose failure is reported rather than silenced.
func knnValuesIterator(values any, field string) (DocIdSetIterator, error) {
	if values == nil {
		return nil, nil
	}
	kvv, ok := values.(index.KnnVectorValues)
	if !ok {
		return nil, fmt.Errorf("FieldExistsQuery: vector values of type %T for field %q do not expose an iterator", values, field)
	}
	return kvv.Iterator(), nil
}
