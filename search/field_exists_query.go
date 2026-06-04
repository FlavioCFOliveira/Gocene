// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// FieldExistsQuery matches documents that have a value in the given field.
// This query finds all documents where the specified field exists,
// regardless of the field's value.
//
// This is the Go port of Lucene's org.apache.lucene.search.FieldExistsQuery.
type FieldExistsQuery struct {
	*BaseQuery
	field string
}

// NewFieldExistsQuery creates a new FieldExistsQuery.
// The field parameter is the name of the field to check for existence.
func NewFieldExistsQuery(field string) *FieldExistsQuery {
	return &FieldExistsQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
	}
}

// GetField returns the field name.
func (q *FieldExistsQuery) GetField() string {
	return q.field
}

// Clone creates a copy of this query.
func (q *FieldExistsQuery) Clone() Query {
	return NewFieldExistsQuery(q.field)
}

// Rewrite returns the query unchanged (FieldExistsQuery has no rewrite
// rules). The explicit override is required because the type embeds
// *BaseQuery: relying on the promoted BaseQuery.Rewrite would return
// the inner *BaseQuery receiver, erasing this query's CreateWeight
// override so the rewritten query would silently match zero documents.
func (q *FieldExistsQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Equals checks if this query equals another.
func (q *FieldExistsQuery) Equals(other Query) bool {
	if o, ok := other.(*FieldExistsQuery); ok {
		return q.field == o.field
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *FieldExistsQuery) HashCode() int {
	h := 0
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	return h
}

// String returns a debug representation of the query.
func (q *FieldExistsQuery) String() string {
	return fmt.Sprintf("FieldExistsQuery [field=%s]", q.field)
}

// CreateWeight builds a constant-score Weight that matches every document
// carrying a value for the field. This is the Go port of
// FieldExistsQuery.createWeight (Lucene 10.4.0), restricted to the
// doc-values-backed path: the per-leaf doc-values iterator for the field is
// resolved from its FieldInfo's DocValuesType and used directly as the
// matching DocIdSetIterator (a doc-values iterator visits exactly the
// documents that have a value).
//
// Gocene does not yet surface norms/vector existence iterators from the
// LeafReader interface used here, so a field that exists only as norms or
// vectors yields no scorer on that leaf (matching Lucene's "iterator == null
// -> return null" no-match fast path rather than mis-matching). Doc-values
// fields — the case the DocValues range-query rewrites depend on — are fully
// supported.
func (q *FieldExistsQuery) CreateWeight(_ *IndexSearcher, _ bool, boost float32) (Weight, error) {
	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		iter, err := fieldExistsDocValuesIterator(ctx, q.field)
		if err != nil {
			return nil, err
		}
		if iter == nil {
			return nil, nil
		}
		return NewConstantScoreScorerSupplierFromIterator(boost, COMPLETE_NO_SCORES, iter), nil
	}
	cacheable := func(ctx *index.LeafReaderContext) bool {
		return index.IsDocValuesCacheable(ctx, q.field)
	}
	return NewConstantScoreWeight(q, boost, supplier, cacheable), nil
}

// fieldExistsDocValuesIterator resolves the leaf's doc-values iterator for
// field, wrapped as a DocIdSetIterator that visits every document carrying a
// value. It dispatches on the field's DocValuesType (read from the leaf's
// FieldInfos, exactly as FieldExistsQuery.createWeight switches on
// fieldInfo.getDocValuesType()). Returns (nil, nil) when the field is absent
// from the leaf or carries no doc values, matching the reference's null
// iterator no-match fast path.
func fieldExistsDocValuesIterator(ctx *index.LeafReaderContext, field string) (DocIdSetIterator, error) {
	if ctx == nil {
		return nil, nil
	}
	leaf := ctx.LeafReader()
	if leaf == nil {
		return nil, nil
	}

	type fieldInfosProvider interface {
		GetFieldInfos() *index.FieldInfos
	}
	fip, ok := leaf.(fieldInfosProvider)
	if !ok {
		return nil, nil
	}
	infos := fip.GetFieldInfos()
	if infos == nil {
		return nil, nil
	}
	fi := infos.GetByName(field)
	if fi == nil {
		return nil, nil
	}

	maxDoc := leaf.MaxDoc()

	// Vectors branch. Mirrors FieldExistsQuery.createWeight's
	// `fieldInfo.getVectorDimension() != 0` case, which uses the field's
	// FloatVectorValues / ByteVectorValues iterator as the existence iterator.
	// Gocene's KNN vector values expose Get(docID), so existence is probed by
	// document id (correct for both dense and sparse storage) rather than
	// driving the values' own iterator.
	if fi.VectorDimension() != 0 {
		switch fi.VectorEncoding() {
		case index.VectorEncodingFloat32:
			provider, ok := leaf.(floatVectorValuesProvider)
			if !ok {
				return nil, nil
			}
			values, err := provider.GetFloatVectorValues(field)
			if err != nil || values == nil {
				return nil, err
			}
			return newVectorValuesIterator(maxDoc, func(docID int) bool {
				v, e := values.Get(docID)
				return e == nil && len(v) != 0
			}), nil
		case index.VectorEncodingByte:
			provider, ok := leaf.(byteVectorValuesProvider)
			if !ok {
				return nil, nil
			}
			values, err := provider.GetByteVectorValues(field)
			if err != nil || values == nil {
				return nil, err
			}
			return newVectorValuesIterator(maxDoc, func(docID int) bool {
				v, e := values.Get(docID)
				return e == nil && len(v) != 0
			}), nil
		default:
			return nil, nil
		}
	}

	type docValuesReader interface {
		GetNumericDocValues(field string) (index.NumericDocValues, error)
		GetBinaryDocValues(field string) (index.BinaryDocValues, error)
		GetSortedDocValues(field string) (index.SortedDocValues, error)
		GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
		GetSortedSetDocValues(field string) (index.SortedSetDocValues, error)
	}
	dvr, ok := leaf.(docValuesReader)
	if !ok {
		return nil, nil
	}

	switch fi.DocValuesType() {
	case index.DocValuesTypeNumeric:
		v, err := dvr.GetNumericDocValues(field)
		if err != nil || v == nil {
			return nil, err
		}
		return newDocValuesExistsIterator(v, maxDoc), nil
	case index.DocValuesTypeBinary:
		v, err := dvr.GetBinaryDocValues(field)
		if err != nil || v == nil {
			return nil, err
		}
		return newDocValuesExistsIterator(v, maxDoc), nil
	case index.DocValuesTypeSorted:
		v, err := dvr.GetSortedDocValues(field)
		if err != nil || v == nil {
			return nil, err
		}
		return newDocValuesExistsIterator(v, maxDoc), nil
	case index.DocValuesTypeSortedNumeric:
		v, err := dvr.GetSortedNumericDocValues(field)
		if err != nil || v == nil {
			return nil, err
		}
		return newDocValuesExistsIterator(v, maxDoc), nil
	case index.DocValuesTypeSortedSet:
		v, err := dvr.GetSortedSetDocValues(field)
		if err != nil || v == nil {
			return nil, err
		}
		return newDocValuesExistsIterator(v, maxDoc), nil
	default:
		return nil, nil
	}
}

// docValuesExistsIterator wraps any doc-values iterator (a type exposing the
// DocIdSetIterator-shaped DocID/NextDoc/Advance/Cost primitives) as a search
// DocIdSetIterator that visits every document carrying a value. It normalises
// the doc-values NO_MORE_DOCS sentinel: Gocene's doc-values iterators use the
// Lucene Integer.MAX_VALUE convention, but some report -1, so both are mapped
// to search.NO_MORE_DOCS.
type docValuesIterator interface {
	DocID() int
	NextDoc() (int, error)
	Advance(target int) (int, error)
	Cost() int64
}

type docValuesExistsIterator struct {
	values docValuesIterator
	cost   int64
	docID  int
}

func newDocValuesExistsIterator(values docValuesIterator, maxDoc int) *docValuesExistsIterator {
	cost := values.Cost()
	if cost <= 0 && maxDoc > 0 {
		cost = int64(maxDoc)
	}
	if cost < 0 {
		cost = 0
	}
	return &docValuesExistsIterator{values: values, cost: cost, docID: -1}
}

func (it *docValuesExistsIterator) DocID() int { return it.docID }

func (it *docValuesExistsIterator) NextDoc() (int, error) {
	id, err := it.values.NextDoc()
	if err != nil {
		return 0, err
	}
	return it.normalize(id), nil
}

func (it *docValuesExistsIterator) Advance(target int) (int, error) {
	id, err := it.values.Advance(target)
	if err != nil {
		return 0, err
	}
	return it.normalize(id), nil
}

func (it *docValuesExistsIterator) normalize(id int) int {
	// math.MaxInt32 is the Lucene DocIdSetIterator.NO_MORE_DOCS sentinel;
	// -1 is the alternate sentinel some Gocene doc-values iterators emit.
	if id == -1 || id >= 2147483647 {
		it.docID = NO_MORE_DOCS
		return NO_MORE_DOCS
	}
	it.docID = id
	return id
}

func (it *docValuesExistsIterator) Cost() int64 { return it.cost }

// DocIDRunEnd returns docID+1, the DocIdSetIterator default (doc-values
// iterators are sparse, so there is no consecutive-run optimisation).
func (it *docValuesExistsIterator) DocIDRunEnd() int {
	if it.docID < 0 || it.docID == NO_MORE_DOCS {
		return it.docID
	}
	return it.docID + 1
}

var _ DocIdSetIterator = (*docValuesExistsIterator)(nil)
