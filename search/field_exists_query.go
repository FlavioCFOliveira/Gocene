// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FieldExistsQuery is a query that matches documents that contain either a vector field
// or a field that indexes norms or doc values.
//
// This is the Go port of org.apache.lucene.search.FieldExistsQuery (Lucene 10.5.0).
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
func (q *FieldExistsQuery) Field() string {
	return q.field
}

// Rewrite rewrites the query to a simpler form.
func (q *FieldExistsQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	reader := searcher.GetIndexReader()
	allReadersRewritable := true

	for _, context := range reader.Leaves() {
		leaf := context.Reader()
		fieldInfo := leaf.GetFieldInfos().FieldInfo(q.field)

		if fieldInfo == nil {
			allReadersRewritable = false
			break
		}

		if fieldInfo.HasNorms() { // the field indexes norms
			if reader.DocCount(q.field) != reader.MaxDoc() {
				allReadersRewritable = false
				break
			}
		} else if fieldInfo.GetVectorDimension() != 0 { // the field indexes vectors
			size, err := getVectorValuesSize(fieldInfo, leaf)
			if err != nil || size != leaf.MaxDoc() {
				allReadersRewritable = false
				break
			}
		} else if fieldInfo.GetDocValuesType() != index.DocValuesTypeNone { // the field indexes doc values or points
			// This optimization is possible due to LUCENE-9334 enforcing a field to always use the
			// same data structures (all or nothing).
			terms := leaf.Terms(q.field)
			pointValues := leaf.GetPointValues(q.field)
			docValuesSkipper := leaf.GetDocValuesSkipper(q.field)

			if (terms == nil || terms.DocCount() != leaf.MaxDoc()) &&
				(pointValues == nil || pointValues.DocCount() != leaf.MaxDoc()) &&
				(docValuesSkipper == nil || docValuesSkipper.DocCount() != leaf.MaxDoc()) {
				allReadersRewritable = false
				break
			}
		} else {
			// This is an illegal state according to Lucene's FieldExistsQuery
			allReadersRewritable = false
			break
		}
	}

	if allReadersRewritable {
		return MatchAllDocsQuery, nil
	}

	return q.BaseQuery.Rewrite(searcher)
}

// CreateWeight creates a Weight for this query.
func (q *FieldExistsQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return &fieldExistsWeight{
		BaseWeight: *NewBaseWeight(q),
		query:      q,
		boost:      boost,
	}, nil
}

func (q *FieldExistsQuery) ToString(field string) string {
	return fmt.Sprintf("FieldExistsQuery [field=%s]", q.field)
}

func (q *FieldExistsQuery) Equals(other Query) bool {
	if o, ok := other.(*FieldExistsQuery); ok {
		return q.field == o.field
	}
	return false
}

func (q *FieldExistsQuery) HashCode() int {
	return 31*0 + q.field.HashCode() // Simplified classHash() = 0
}

type fieldExistsWeight struct {
	BaseWeight
	query *FieldExistsQuery
	boost float32
}

func (w *fieldExistsWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	reader := ctx.Reader()
	fieldInfos := reader.GetFieldInfos()
	fieldInfo := fieldInfos.FieldInfo(w.query.field)

	if fieldInfo == nil {
		return nil, nil
	}

	var iterator DocIdSetIterator
	if fieldInfo.HasNorms() { // the field indexes norms
		iterator = reader.GetNormValues(w.query.field)
	} else if fieldInfo.GetVectorDimension() != 0 { // the field indexes vectors
		switch fieldInfo.GetVectorEncoding() {
		case index.VectorEncodingFloat32:
			iterator = reader.GetFloatVectorValues(w.query.field).Iterator()
		case index.VectorEncodingByte:
			iterator = reader.GetByteVectorValues(w.query.field).Iterator()
		}
	} else if fieldInfo.GetDocValuesType() != index.DocValuesTypeNone { // the field indexes doc values
		switch fieldInfo.GetDocValuesType() {
		case index.DocValuesTypeNumeric:
			iterator = reader.GetNumericDocValues(w.query.field)
		case index.DocValuesTypeBinary:
			iterator = reader.GetBinaryDocValues(w.query.field)
		case index.DocValuesTypeSorted:
			iterator = reader.GetSortedDocValues(w.query.field)
		case index.DocValuesTypeSortedNumeric:
			iterator = reader.GetSortedNumericDocValues(w.query.field)
		case index.DocValuesTypeSortedSet:
			iterator = reader.GetSortedSetDocValues(w.query.field)
		}
	}

	if iterator == nil {
		return nil, nil
	}

	// We use a constant score of 'boost'
	return NewConstantScoreScorerSupplier(iterator, w.boost), nil
}

func (w *fieldExistsWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	reader := ctx.Reader()
	fieldInfos := reader.GetFieldInfos()
	fieldInfo := fieldInfos.FieldInfo(w.query.field)

	if fieldInfo == nil {
		return 0, nil
	}

	if fieldInfo.HasNorms() { // the field indexes norms
		if reader.DocCount(w.query.field) == reader.MaxDoc() {
			return reader.NumDocs(), nil
		}
		return w.BaseWeight.Count(ctx)
	}

	count := -1
	if fieldInfo.HasVectorValues() { // the field indexes vectors
		count, _ = getVectorValuesSize(fieldInfo, reader)
	} else if fieldInfo.GetDocValuesType() != index.DocValuesTypeNone { // the field indexes doc values
		if fieldInfo.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeNone {
			docValuesSkipper := reader.GetDocValuesSkipper(w.query.field)
			if docValuesSkipper == nil {
				count = 0
			} else {
				count = docValuesSkipper.DocCount()
			}
		} else if reader.HasDeletions() == false {
			if fieldInfo.GetPointDimensionCount() > 0 {
				pointValues := reader.GetPointValues(w.query.field)
				if pointValues == nil {
				    count = 0
				} else {
				    count = pointValues.DocCount()
				}
			} else if fieldInfo.GetIndexOptions() != index.IndexOptionsNone {
				terms := reader.Terms(w.query.field)
				if terms == nil {
				    count = 0
				} else {
				    count = terms.DocCount()
				}
			}
		}
	}

	if count == 0 {
		return 0, nil
	} else if count == reader.MaxDoc() {
		return reader.NumDocs(), nil
	} else if count >= 0 && reader.HasDeletions() == false {
		return count, nil
	}

	return w.BaseWeight.Count(ctx)
}

func getVectorValuesSize(fi *index.FieldInfo, reader index.LeafReader) (int, error) {
	switch fi.GetVectorEncoding() {
	case index.VectorEncodingFloat32:
		v := reader.GetFloatVectorValues(fi.Name)
		if v == nil {
			return 0, fmt.Errorf("unexpected null float vector values")
		}
		return v.Size(), nil
	case index.VectorEncodingByte:
		v := reader.GetByteVectorValues(fi.Name)
		if v == nil {
			return 0, fmt.Errorf("unexpected null byte vector values")
		}
		return v.Size(), nil
	}
	return 0, fmt.Errorf("unsupported vector encoding")
}
