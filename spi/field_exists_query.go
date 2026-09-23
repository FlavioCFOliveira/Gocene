// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import "fmt"

// FieldExistsQueryGetDocValuesDocIdSetIterator returns a DocIdSetIterator
// from the given field or nil if the field doesn't exist in the reader or if
// the reader has no doc values for the field.
//
// Mirrors the static
// org.apache.lucene.search.FieldExistsQuery.getDocValuesDocIdSetIterator(String,
// LeafReader). It lives in spi so that package index (PendingSoftDeletes,
// IndexWriter, CheckIndex) and package search share one rendering without an
// import cycle; search.GetDocValuesDocIdSetIterator delegates here.
func FieldExistsQueryGetDocValuesDocIdSetIterator(field string, reader LeafReader) (DocIdSetIterator, error) {
	fieldInfo := reader.GetFieldInfos().FieldInfo(field)
	if fieldInfo == nil {
		return nil, nil
	}
	switch fieldInfo.DocValuesType() {
	case DocValuesTypeNone:
		return nil, nil
	case DocValuesTypeNumeric:
		dv, err := reader.GetNumericDocValues(field)
		if err != nil || dv == nil {
			return nil, err
		}
		return asDocValuesIterator(dv)
	case DocValuesTypeBinary:
		dv, err := reader.GetBinaryDocValues(field)
		if err != nil || dv == nil {
			return nil, err
		}
		return asDocValuesIterator(dv)
	case DocValuesTypeSorted:
		dv, err := reader.GetSortedDocValues(field)
		if err != nil || dv == nil {
			return nil, err
		}
		return asDocValuesIterator(dv)
	case DocValuesTypeSortedNumeric:
		dv, err := reader.GetSortedNumericDocValues(field)
		if err != nil || dv == nil {
			return nil, err
		}
		return asDocValuesIterator(dv)
	case DocValuesTypeSortedSet:
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
