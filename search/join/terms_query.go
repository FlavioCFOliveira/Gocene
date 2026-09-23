// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// termsQueryBaseRAMBytes renders
// RamUsageEstimator.shallowSizeOfInstance(TermsQuery.class).
var termsQueryBaseRAMBytes = util.ShallowSizeOf(TermsQuery{})

// TermsQuery is a query that has an array of terms from a specific field. This
// query will match documents have one or more terms in the specified field
// that match with the terms specified in the array.
//
// Port of the package-private org.apache.lucene.search.join.TermsQuery
// (lucene/join/src/java/org/apache/lucene/search/join/TermsQuery.java, Apache
// Lucene 10.5.0), a MultiTermQuery that also implements Accountable.
//
// @lucene.experimental
type TermsQuery struct {
	*search.MultiTermQuery

	terms *util.BytesRefHash
	ords  []int

	// These fields are used for equals() and hashcode() only
	fromField string
	fromQuery search.Query
	// id of the context rather than the context itself in order not to hold
	// references to index readers
	indexReaderContextID any

	ramBytesUsed int64 // cache
}

// newTermsQuery renders the package-private constructor
// TermsQuery(String toField, BytesRefHash terms, String fromField, Query
// fromQuery, Object indexReaderContextId).
//
// toField is the field that should contain terms that are specified in the
// next parameter; terms are the terms that matching documents should have
// (the terms must be sorted by natural order); indexReaderContextID refers to
// the top level index reader used to create the set of terms in the previous
// parameter.
func newTermsQuery(toField string, terms *util.BytesRefHash, fromField string, fromQuery search.Query, indexReaderContextID any) *TermsQuery {
	q := &TermsQuery{
		MultiTermQuery:       search.NewMultiTermQuery(toField, search.ConstantScoreBlendedRewrite),
		terms:                terms,
		fromField:            fromField,
		fromQuery:            fromQuery,
		indexReaderContextID: indexReaderContextID,
	}
	q.MultiTermQuery.SetOwner(q)
	q.ords = terms.Sort()

	q.ramBytesUsed = termsQueryBaseRAMBytes +
		util.SizeOfString(toField) +
		util.SizeOfString(fromField) +
		sizeOfQuery(fromQuery) +
		util.SizeOfIntSlice(q.ords) +
		terms.RamBytesUsed()
	return q
}

// sizeOfQuery renders RamUsageEstimator.sizeOfObject(query,
// QUERY_DEFAULT_RAM_BYTES_USED): the query's own estimate when it is
// Accountable, the default otherwise.
func sizeOfQuery(q search.Query) int64 {
	if q == nil {
		return 0
	}
	if acc, ok := q.(util.Accountable); ok {
		return acc.RamBytesUsed()
	}
	return util.RamQueryDefaultBytesUsed
}

// Visit renders visit(QueryVisitor): visitor.visitLeaf(this).
func (q *TermsQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

// GetTermsEnumWithAttributes renders the getTermsEnum(Terms, AttributeSource)
// override.
func (q *TermsQuery) GetTermsEnumWithAttributes(terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error) {
	if q.terms.Size() == 0 {
		return &index.EmptyTermsEnum{}, nil
	}

	it, err := terms.Iterator()
	if err != nil {
		return nil, err
	}
	return NewSeekingTermSetTermsEnum(it, q.terms, q.ords), nil
}

// GetTermsCount renders getTermsCount().
func (q *TermsQuery) GetTermsCount() int64 {
	return int64(q.terms.Size())
}

// ToString renders toString(String).
func (q *TermsQuery) ToString(string) string {
	return "TermsQuery{" + "field=" + q.GetField() + "fromQuery=" + queryToString(q.fromQuery, q.GetField()) + "}"
}

// String renders toString().
func (q *TermsQuery) String() string {
	return q.ToString("")
}

// Equals renders equals(Object).
func (q *TermsQuery) Equals(obj spi.Query) bool {
	if other, ok := obj.(*TermsQuery); ok && other == q {
		return true
	}
	if !q.MultiTermQuery.Equals(obj) {
		return false
	}
	other, ok := obj.(*TermsQuery)
	if !ok {
		return false
	}
	return q.GetField() == other.GetField() &&
		q.fromField == other.fromField &&
		queriesEqual(q.fromQuery, other.fromQuery) &&
		q.indexReaderContextID == other.indexReaderContextID
}

// HashCode renders hashCode(): classHash() + Objects.hash(field, fromField,
// fromQuery, indexReaderContextId). classHash() is the MultiTermQuery
// rendering (a stable per-type constant); the context id is an opaque Java
// Object whose identity hash has no Go counterpart, so it contributes its
// dynamic type's hash only.
func (q *TermsQuery) HashCode() int {
	h := int32(1)
	h = 31*h + int32(javaStringHash(q.GetField()))
	h = 31*h + int32(javaStringHash(q.fromField))
	if q.fromQuery != nil {
		h = 31*h + int32(q.fromQuery.HashCode())
	} else {
		h = 31 * h
	}
	if q.indexReaderContextID != nil {
		h = 31*h + int32(javaStringHash(reflect.TypeOf(q.indexReaderContextID).String()))
	} else {
		h = 31 * h
	}
	return int(int32(javaStringHash(reflect.TypeOf(q).String())) + h)
}

// RamBytesUsed renders ramBytesUsed().
func (q *TermsQuery) RamBytesUsed() int64 {
	return q.ramBytesUsed
}

// queryToString renders Query.toString(String field) for a search.Query.
func queryToString(q search.Query, field string) string {
	if q == nil {
		return "null"
	}
	if ts, ok := q.(interface{ ToString(string) string }); ok {
		return ts.ToString(field)
	}
	if s, ok := q.(interface{ String(string) string }); ok {
		return s.String(field)
	}
	if s, ok := q.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

// javaStringHash renders java.lang.String#hashCode() over the UTF-16 units.
func javaStringHash(s string) int {
	var h int32
	for _, r := range s {
		if r <= 0xFFFF {
			h = 31*h + int32(r)
			continue
		}
		r -= 0x10000
		h = 31*h + int32(0xD800+(r>>10))
		h = 31*h + int32(0xDC00+(r&0x3FF))
	}
	return int(h)
}

var (
	_ search.Query               = (*TermsQuery)(nil)
	_ search.MultiTermQueryOwner = (*TermsQuery)(nil)
	_ util.Accountable           = (*TermsQuery)(nil)
)
