// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// CollectionStatistics holds statistics for a collection (field).
//
// This is the Go port of Lucene's org.apache.lucene.search.CollectionStatistics.
type CollectionStatistics struct {
	field        string
	maxDoc       int
	docCount     int
	sumTotalTermFreq int64
	sumDocFreq   int64
}

// NewCollectionStatistics creates new collection statistics.
func NewCollectionStatistics(field string, maxDoc, docCount int, sumTotalTermFreq, sumDocFreq int64) *CollectionStatistics {
	return &CollectionStatistics{
		field:            field,
		maxDoc:           maxDoc,
		docCount:         docCount,
		sumTotalTermFreq: sumTotalTermFreq,
		sumDocFreq:       sumDocFreq,
	}
}

// Field returns the field name.
func (s *CollectionStatistics) Field() string {
	return s.field
}

// MaxDoc returns the maximum document ID.
func (s *CollectionStatistics) MaxDoc() int {
	return s.maxDoc
}

// DocCount returns the document count.
func (s *CollectionStatistics) DocCount() int {
	return s.docCount
}

// SumTotalTermFreq returns the sum of total term frequencies.
func (s *CollectionStatistics) SumTotalTermFreq() int64 {
	return s.sumTotalTermFreq
}

// SumDocFreq returns the sum of document frequencies.
func (s *CollectionStatistics) SumDocFreq() int64 {
	return s.sumDocFreq
}

// TermStatistics holds statistics for a term.
//
// This is the Go port of Lucene's org.apache.lucene.search.TermStatistics.
type TermStatistics struct {
	term     *index.Term
	docFreq  int
	totalTermFreq int64
}

// NewTermStatistics creates new term statistics.
func NewTermStatistics(term *index.Term, docFreq int, totalTermFreq int64) *TermStatistics {
	return &TermStatistics{
		term:          term,
		docFreq:       docFreq,
		totalTermFreq: totalTermFreq,
	}
}

// Term returns the term.
func (s *TermStatistics) Term() *index.Term {
	return s.term
}

// DocFreq returns the document frequency.
func (s *TermStatistics) DocFreq() int {
	return s.docFreq
}

// TotalTermFreq returns the total term frequency.
func (s *TermStatistics) TotalTermFreq() int64 {
	return s.totalTermFreq
}
