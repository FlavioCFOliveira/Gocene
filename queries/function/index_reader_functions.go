// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// IndexReaderFunctions exposes static-style helpers that turn IndexReader
// statistics into [DoubleValuesSource] / [LongValuesSource] instances.
//
// Go port of org.apache.lucene.queries.function.IndexReaderFunctions.
// Provided as exported package-level functions rather than a non-
// instantiable Java class.
//
// Gocene deviation: Lucene reaches into IndexReader via static method
// references (IndexReader::maxDoc, etc.). Go uses interface-guarded
// adapters because the Gocene IndexReaderInterface omits some
// field-stat methods that ship on *index.IndexReader specifically. The
// helpers below succeed against any reader that exposes the needed
// interface; otherwise they return a wrapped error during Rewrite.

// IndexReaderDocFreq returns a source whose constant value is
// reader.DocFreq(term). Errors during rewrite propagate as wrapped errors.
func IndexReaderDocFreq(term *index.Term) DoubleValuesSource {
	return newIndexReaderDoubleValuesSource(
		fmt.Sprintf("docFreq(%s)", term),
		func(r index.IndexReaderInterface) (float64, error) {
			s, ok := r.(termDocFreqReader)
			if !ok {
				return 0, fmt.Errorf("index reader does not implement DocFreq(*Term)")
			}
			v, err := s.DocFreq(term)
			if err != nil {
				return 0, err
			}
			return float64(v), nil
		},
	)
}

// IndexReaderMaxDoc returns a source whose constant value is reader.MaxDoc().
func IndexReaderMaxDoc() DoubleValuesSource {
	return newIndexReaderDoubleValuesSource(
		"maxDoc()",
		func(r index.IndexReaderInterface) (float64, error) { return float64(r.MaxDoc()), nil },
	)
}

// IndexReaderNumDocs returns a source whose constant value is reader.NumDocs().
func IndexReaderNumDocs() DoubleValuesSource {
	return newIndexReaderDoubleValuesSource(
		"numDocs()",
		func(r index.IndexReaderInterface) (float64, error) { return float64(r.NumDocs()), nil },
	)
}

// IndexReaderNumDeletedDocs returns a source whose constant value is
// reader.NumDeletedDocs().
func IndexReaderNumDeletedDocs() DoubleValuesSource {
	return newIndexReaderDoubleValuesSource(
		"numDeletedDocs()",
		func(r index.IndexReaderInterface) (float64, error) { return float64(r.NumDeletedDocs()), nil },
	)
}

// IndexReaderSumTotalTermFreq returns a LongValuesSource yielding
// reader.GetSumTotalTermFreq(field).
func IndexReaderSumTotalTermFreq(field string) LongValuesSource {
	return &sumTotalTermFreqLongValuesSource{field: field}
}

// IndexReaderTermFreq returns a per-doc DoubleValuesSource whose value is
// the freq() of term in the supplied document.
func IndexReaderTermFreq(term *index.Term) DoubleValuesSource {
	return &termFreqDoubleValuesSource{term: term}
}

// IndexReaderTotalTermFreq returns a constant DoubleValuesSource yielding
// reader.TotalTermFreq(term).
func IndexReaderTotalTermFreq(term *index.Term) DoubleValuesSource {
	return newIndexReaderDoubleValuesSource(
		fmt.Sprintf("totalTermFreq(%s)", term),
		func(r index.IndexReaderInterface) (float64, error) {
			s, ok := r.(termTotalTermFreqReader)
			if !ok {
				return 0, fmt.Errorf("index reader does not implement TotalTermFreq(*Term)")
			}
			v, err := s.TotalTermFreq(term)
			if err != nil {
				return 0, err
			}
			return float64(v), nil
		},
	)
}

// IndexReaderSumDocFreq returns a constant DoubleValuesSource yielding
// reader.GetSumDocFreq(field).
func IndexReaderSumDocFreq(field string) DoubleValuesSource {
	return newIndexReaderDoubleValuesSource(
		fmt.Sprintf("sumDocFreq(%s)", field),
		func(r index.IndexReaderInterface) (float64, error) {
			s, ok := r.(fieldSumDocFreqReader)
			if !ok {
				return 0, fmt.Errorf("index reader does not implement GetSumDocFreq(field)")
			}
			v, err := s.GetSumDocFreq(field)
			if err != nil {
				return 0, err
			}
			return float64(v), nil
		},
	)
}

// IndexReaderDocCount returns a constant DoubleValuesSource yielding
// reader.GetDocCount(field).
func IndexReaderDocCount(field string) DoubleValuesSource {
	return newIndexReaderDoubleValuesSource(
		fmt.Sprintf("docCount(%s)", field),
		func(r index.IndexReaderInterface) (float64, error) {
			s, ok := r.(fieldDocCountReader)
			if !ok {
				return 0, fmt.Errorf("index reader does not implement GetDocCount(field)")
			}
			v, err := s.GetDocCount(field)
			if err != nil {
				return 0, err
			}
			return float64(v), nil
		},
	)
}

// ------------------------------------------------------------------------
// Internal adapters: reader-stat interfaces and the constant-rewrite source.
// ------------------------------------------------------------------------

type termDocFreqReader interface {
	DocFreq(term *index.Term) (int, error)
}
type termTotalTermFreqReader interface {
	TotalTermFreq(term *index.Term) (int64, error)
}
type fieldSumDocFreqReader interface {
	GetSumDocFreq(field string) (int64, error)
}
type fieldDocCountReader interface {
	GetDocCount(field string) (int, error)
}
type fieldSumTotalTermFreqReader interface {
	GetSumTotalTermFreq(field string) (int64, error)
}

// readerFunction is the Go counterpart to the Java @FunctionalInterface
// IndexReaderFunctions.ReaderFunction.
type readerFunction func(reader index.IndexReaderInterface) (float64, error)

// indexReaderDoubleValuesSource is the deferred constant source whose
// value materialises during Rewrite.
type indexReaderDoubleValuesSource struct {
	description string
	fn          readerFunction
}

func newIndexReaderDoubleValuesSource(description string, fn readerFunction) *indexReaderDoubleValuesSource {
	return &indexReaderDoubleValuesSource{description: description, fn: fn}
}

func (s *indexReaderDoubleValuesSource) GetValues(_ *index.LeafReaderContext, _ DoubleValues) (DoubleValues, error) {
	return nil, fmt.Errorf("function: IndexReaderFunction must be rewritten before use")
}
func (s *indexReaderDoubleValuesSource) NeedsScores() bool                           { return false }
func (s *indexReaderDoubleValuesSource) IsCacheable(_ *index.LeafReaderContext) bool { return false }
func (s *indexReaderDoubleValuesSource) Rewrite(searcher *search.IndexSearcher) (DoubleValuesSource, error) {
	if searcher == nil {
		return nil, fmt.Errorf("function: IndexReaderFunctions need a non-nil searcher")
	}
	value, err := s.fn(searcher.GetIndexReader())
	if err != nil {
		return nil, fmt.Errorf("function: index reader function: %w", err)
	}
	return &noCacheConstantDoubleValuesSource{value: value, parent: s}, nil
}
func (s *indexReaderDoubleValuesSource) Equals(other DoubleValuesSource) bool {
	o, ok := other.(*indexReaderDoubleValuesSource)
	if !ok || o == nil {
		return false
	}
	return s.description == o.description
}
func (s *indexReaderDoubleValuesSource) HashCode() int32     { return hashString(s.description) }
func (s *indexReaderDoubleValuesSource) Description() string { return s.description }
func (s *indexReaderDoubleValuesSource) Explain(_ *index.LeafReaderContext, _ int, _ search.Explanation) (search.Explanation, error) {
	return search.NewExplanation(true, 0, s.description), nil
}

// noCacheConstantDoubleValuesSource is the rewritten form of
// indexReaderDoubleValuesSource: its value has been materialised once at
// rewrite time and is yielded for every doc thereafter.
type noCacheConstantDoubleValuesSource struct {
	value  float64
	parent DoubleValuesSource
}

func (n *noCacheConstantDoubleValuesSource) GetValues(_ *index.LeafReaderContext, _ DoubleValues) (DoubleValues, error) {
	return &constantDoubleValues{value: n.value}, nil
}
func (n *noCacheConstantDoubleValuesSource) NeedsScores() bool { return false }
func (n *noCacheConstantDoubleValuesSource) IsCacheable(_ *index.LeafReaderContext) bool {
	return false
}
func (n *noCacheConstantDoubleValuesSource) Rewrite(_ *search.IndexSearcher) (DoubleValuesSource, error) {
	return n, nil
}
func (n *noCacheConstantDoubleValuesSource) Equals(other DoubleValuesSource) bool {
	o, ok := other.(*noCacheConstantDoubleValuesSource)
	if !ok || o == nil {
		return false
	}
	return n.value == o.value && n.parent.Equals(o.parent)
}
func (n *noCacheConstantDoubleValuesSource) HashCode() int32 {
	return combineHash(hashFloat64(n.value), n.parent.HashCode())
}
func (n *noCacheConstantDoubleValuesSource) Description() string { return n.parent.Description() }
func (n *noCacheConstantDoubleValuesSource) Explain(_ *index.LeafReaderContext, _ int, _ search.Explanation) (search.Explanation, error) {
	return search.NewExplanation(true, float32(n.value), n.parent.Description()), nil
}

// sumTotalTermFreqLongValuesSource implements the deferred constant
// LongValuesSource backing IndexReaderSumTotalTermFreq.
type sumTotalTermFreqLongValuesSource struct {
	field string
}

func (s *sumTotalTermFreqLongValuesSource) GetValues(_ *index.LeafReaderContext, _ DoubleValues) (LongValues, error) {
	return nil, fmt.Errorf("function: IndexReaderFunction must be rewritten before use")
}
func (s *sumTotalTermFreqLongValuesSource) NeedsScores() bool                           { return false }
func (s *sumTotalTermFreqLongValuesSource) IsCacheable(_ *index.LeafReaderContext) bool { return false }
func (s *sumTotalTermFreqLongValuesSource) Rewrite(searcher *search.IndexSearcher) (LongValuesSource, error) {
	if searcher == nil {
		return nil, fmt.Errorf("function: sumTotalTermFreq needs a non-nil searcher")
	}
	reader, ok := searcher.GetIndexReader().(fieldSumTotalTermFreqReader)
	if !ok {
		return nil, fmt.Errorf("function: index reader does not implement GetSumTotalTermFreq(field)")
	}
	v, err := reader.GetSumTotalTermFreq(s.field)
	if err != nil {
		return nil, err
	}
	return &noCacheConstantLongValuesSource{value: v, parent: s}, nil
}
func (s *sumTotalTermFreqLongValuesSource) Equals(other LongValuesSource) bool {
	o, ok := other.(*sumTotalTermFreqLongValuesSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field
}
func (s *sumTotalTermFreqLongValuesSource) HashCode() int32 { return hashString(s.field) }
func (s *sumTotalTermFreqLongValuesSource) Description() string {
	return fmt.Sprintf("sumTotalTermFreq(%s)", s.field)
}

// noCacheConstantLongValuesSource is the rewritten form returned by the
// SumTotalTermFreq source.
type noCacheConstantLongValuesSource struct {
	value  int64
	parent LongValuesSource
}

func (n *noCacheConstantLongValuesSource) GetValues(_ *index.LeafReaderContext, _ DoubleValues) (LongValues, error) {
	return &constantLongValues{value: n.value}, nil
}
func (n *noCacheConstantLongValuesSource) NeedsScores() bool                           { return false }
func (n *noCacheConstantLongValuesSource) IsCacheable(_ *index.LeafReaderContext) bool { return false }
func (n *noCacheConstantLongValuesSource) Rewrite(_ *search.IndexSearcher) (LongValuesSource, error) {
	return n, nil
}
func (n *noCacheConstantLongValuesSource) Equals(other LongValuesSource) bool {
	o, ok := other.(*noCacheConstantLongValuesSource)
	if !ok || o == nil {
		return false
	}
	return n.value == o.value && n.parent.Equals(o.parent)
}
func (n *noCacheConstantLongValuesSource) HashCode() int32 {
	return combineHash(hashInt32(int32(n.value)), n.parent.HashCode())
}
func (n *noCacheConstantLongValuesSource) Description() string { return n.parent.Description() }

type constantLongValues struct {
	value int64
}

func (c *constantLongValues) LongValue() (int64, error)        { return c.value, nil }
func (c *constantLongValues) AdvanceExact(_ int) (bool, error) { return true, nil }

// termFreqDoubleValuesSource yields a per-doc termFreq via the leaf's
// TermsEnum and PostingsEnum.
type termFreqDoubleValuesSource struct {
	term *index.Term
}

func (s *termFreqDoubleValuesSource) GetValues(ctx *index.LeafReaderContext, _ DoubleValues) (DoubleValues, error) {
	leaf := ctx.LeafReader()
	if leaf == nil {
		return EmptyDoubleValues, nil
	}
	terms, err := leaf.Terms(s.term.Field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return EmptyDoubleValues, nil
	}
	te, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}
	ok, err := te.SeekExact(s.term)
	if err != nil {
		return nil, err
	}
	if !ok {
		return EmptyDoubleValues, nil
	}
	// 0 == PostingsEnum.NONE in Lucene; positive flags request positions,
	// offsets, payloads. termFreq only needs Freq() so we pass 0.
	pe, err := te.Postings(0)
	if err != nil {
		return nil, err
	}
	if pe == nil {
		return EmptyDoubleValues, nil
	}
	return &termFreqDoubleValues{pe: pe}, nil
}
func (s *termFreqDoubleValuesSource) NeedsScores() bool                           { return false }
func (s *termFreqDoubleValuesSource) IsCacheable(_ *index.LeafReaderContext) bool { return true }
func (s *termFreqDoubleValuesSource) Rewrite(_ *search.IndexSearcher) (DoubleValuesSource, error) {
	return s, nil
}
func (s *termFreqDoubleValuesSource) Equals(other DoubleValuesSource) bool {
	o, ok := other.(*termFreqDoubleValuesSource)
	if !ok || o == nil {
		return false
	}
	return s.term.Equals(o.term)
}
func (s *termFreqDoubleValuesSource) HashCode() int32 { return int32(s.term.HashCode()) }
func (s *termFreqDoubleValuesSource) Description() string {
	return fmt.Sprintf("termFreq(%s)", s.term)
}
func (s *termFreqDoubleValuesSource) Explain(_ *index.LeafReaderContext, _ int, _ search.Explanation) (search.Explanation, error) {
	return search.NewExplanation(true, 0, s.Description()), nil
}

// termFreqDoubleValues wraps a PostingsEnum and exposes its per-doc Freq.
type termFreqDoubleValues struct {
	pe index.PostingsEnum
}

func (t *termFreqDoubleValues) DoubleValue() (float64, error) {
	f, err := t.pe.Freq()
	if err != nil {
		return 0, err
	}
	return float64(f), nil
}

func (t *termFreqDoubleValues) AdvanceExact(doc int) (bool, error) {
	if t.pe.DocID() > doc {
		return false, nil
	}
	if t.pe.DocID() == doc {
		return true, nil
	}
	advanced, err := t.pe.Advance(doc)
	if err != nil {
		return false, err
	}
	return advanced == doc, nil
}
