// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/join/src/java/org/apache/lucene/search/join/TermsCollector.java

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsCollector is a collector that collects all terms from a specified field
// matching the query.
//
// It renders the abstract class
// org.apache.lucene.search.join.TermsCollector<DV> as an interface, because
// Java's `static TermsCollector<?> create(...)` returns the wildcard type and
// GenericTermsCollector.wrap(TermsCollector<?>) accepts it; a Go generic type
// cannot be named without binding its type argument. The concrete members of
// the Java class live in BaseTermsCollector below.
//
// lucene.experimental
type TermsCollector interface {
	// SimpleCollector renders `extends DocValuesTermsCollector<DV>`, which in
	// turn extends org.apache.lucene.search.SimpleCollector.
	search.SimpleCollector

	// GetCollectorTerms mirrors `public BytesRefHash getCollectorTerms()`.
	GetCollectorTerms() *util.BytesRefHash
}

// BaseTermsCollector carries the concrete members of the abstract class
// org.apache.lucene.search.join.TermsCollector<DV>: the collectorTerms field,
// getCollectorTerms() and scoreMode().
type BaseTermsCollector[DV any] struct {
	DocValuesTermsCollector[DV]

	// collectorTerms renders `final BytesRefHash collectorTerms = new BytesRefHash()`.
	collectorTerms *util.BytesRefHash
}

// NewBaseTermsCollector mirrors `TermsCollector(Function<DV> docValuesCall)`.
func NewBaseTermsCollector[DV any](docValuesCall DocValuesTermsCollectorFunction[DV]) *BaseTermsCollector[DV] {
	return &BaseTermsCollector[DV]{
		DocValuesTermsCollector: *NewDocValuesTermsCollector(docValuesCall),
		collectorTerms:          util.NewBytesRefHash(),
	}
}

// GetCollectorTerms mirrors `public BytesRefHash getCollectorTerms()`.
func (c *BaseTermsCollector[DV]) GetCollectorTerms() *util.BytesRefHash {
	return c.collectorTerms
}

// ScoreMode mirrors
// `public org.apache.lucene.search.ScoreMode scoreMode()`, which returns
// ScoreMode.COMPLETE_NO_SCORES.
func (c *BaseTermsCollector[DV]) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}

// CreateTermsCollector chooses the right TermsCollector implementation.
//
// Mirrors `static TermsCollector<?> create(String field, boolean multipleValuesPerDocument)`.
//
//   - field is the field to collect terms for;
//   - multipleValuesPerDocument states whether the field to collect terms for
//     has multiple values per document.
func CreateTermsCollector(field string, multipleValuesPerDocument bool) TermsCollector {
	if multipleValuesPerDocument {
		return NewTermsCollectorMV(SortedSetDocValues(field))
	}
	return NewTermsCollectorSV(SortedDocValues(field))
}

// TermsCollectorMV is the impl that works with multiple values per document.
//
// Mirrors `static class MV extends TermsCollector<SortedSetDocValues>`.
type TermsCollectorMV struct {
	BaseTermsCollector[index.SortedSetDocValues]
}

// NewTermsCollectorMV mirrors `MV(Function<SortedSetDocValues> docValuesCall)`.
func NewTermsCollectorMV(docValuesCall DocValuesTermsCollectorFunction[index.SortedSetDocValues]) *TermsCollectorMV {
	c := &TermsCollectorMV{BaseTermsCollector: *NewBaseTermsCollector(docValuesCall)}
	c.Outer = c
	return c
}

// Collect mirrors `public void collect(int doc)` of TermsCollector.MV.
func (c *TermsCollectorMV) Collect(doc int) error {
	if doc > c.docValues.DocID() {
		if _, err := c.docValues.Advance(doc); err != nil {
			return err
		}
	}
	if doc == c.docValues.DocID() {
		for i := 0; i < c.docValues.DocValueCount(); i++ {
			ord, err := c.docValues.NextOrd()
			if err != nil {
				return err
			}
			term, err := c.docValues.LookupOrd(ord)
			if err != nil {
				return err
			}
			if _, err := c.collectorTerms.Add(util.WrapBytes(term)); err != nil {
				return err
			}
		}
	}
	return nil
}

// CollectRange carries the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (c *TermsCollectorMV) CollectRange(minDoc, maxDoc int) error {
	return search.DefaultCollectRange(c, minDoc, maxDoc)
}

// CollectStream carries the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (c *TermsCollectorMV) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// TermsCollectorSV is the impl that works with a single value per document.
//
// Mirrors `static class SV extends TermsCollector<SortedDocValues>`.
type TermsCollectorSV struct {
	BaseTermsCollector[index.SortedDocValues]
}

// NewTermsCollectorSV mirrors `SV(Function<SortedDocValues> docValuesCall)`.
func NewTermsCollectorSV(docValuesCall DocValuesTermsCollectorFunction[index.SortedDocValues]) *TermsCollectorSV {
	c := &TermsCollectorSV{BaseTermsCollector: *NewBaseTermsCollector(docValuesCall)}
	c.Outer = c
	return c
}

// Collect mirrors `public void collect(int doc)` of TermsCollector.SV.
func (c *TermsCollectorSV) Collect(doc int) error {
	var term *util.BytesRef
	exists, err := c.docValues.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if exists {
		ord, err := c.docValues.OrdValue()
		if err != nil {
			return err
		}
		value, err := c.docValues.LookupOrd(ord)
		if err != nil {
			return err
		}
		term = util.WrapBytes(value)
	} else {
		// `new BytesRef(BytesRef.EMPTY_BYTES)`.
		term = util.NewBytesRefEmpty()
	}
	if _, err := c.collectorTerms.Add(term); err != nil {
		return err
	}
	return nil
}

// CollectRange carries the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (c *TermsCollectorSV) CollectRange(minDoc, maxDoc int) error {
	return search.DefaultCollectRange(c, minDoc, maxDoc)
}

// CollectStream carries the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (c *TermsCollectorSV) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// interface compliance
var (
	_ TermsCollector = (*TermsCollectorMV)(nil)
	_ TermsCollector = (*TermsCollectorSV)(nil)
)
