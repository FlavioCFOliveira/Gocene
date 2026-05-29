// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// This file exposes an indexing-chain-facing entry point onto the
// Lucene90 doc-values consumer for the SORTED and SORTED_SET value
// types.
//
// The generic codecs.DocValuesConsumer interface (spi.DocValuesConsumer)
// models the SORTED / SORTED_SET write path with the writer-side
// SortedDocValuesIterator / SortedSetDocValuesIterator, which carry only
// ordinals and cannot recover the per-ordinal term bytes the codec needs
// to build the terms dictionary. Lucene resolves this by passing the
// read-side SortedDocValues (with lookupOrd / getValueCount) into the
// consumer; the Gocene index-side accumulators
// (index.SortedDocValuesWriter / SortedSetDocValuesWriter) expose exactly
// that read-side surface via GetDocValues.
//
// The Lucene90DocValuesConsumer.AddSortedField / AddSortedSetField
// methods that take the writer-side iterator therefore return an error
// (the ordinal-only path is unsupported). These FromReader variants are
// the supported bridge: they accept the read-side SPI iterators plus a
// reset closure that re-materialises a fresh forward iterator (the
// consumer makes several passes over the values), wrap them in the
// internal dvSortedValues / dvSortedSetValues contracts, and drive the
// byte-faithful Real() consumer.

// AddSortedFieldFromReader writes a SORTED doc-values field from the
// read-side SortedDocValues produced by the index-side accumulator.
//
// reset must return a fresh forward iterator positioned before the first
// document each time it is called; the consumer iterates the values more
// than once (skip index, ordinals, terms dict).
func (c *Lucene90DocValuesConsumer) AddSortedFieldFromReader(
	field *index.FieldInfo,
	reset func() (spi.SortedDocValues, error),
) error {
	a := &sortedReaderAsDV{reset: reset}
	if err := a.Reset(); err != nil {
		return err
	}
	return c.real.AddSortedField(field, a)
}

// AddSortedSetFieldFromReader writes a SORTED_SET doc-values field from
// the read-side SortedSetDocValues produced by the index-side
// accumulator. reset behaves as in AddSortedFieldFromReader.
func (c *Lucene90DocValuesConsumer) AddSortedSetFieldFromReader(
	field *index.FieldInfo,
	reset func() (spi.SortedSetDocValues, error),
) error {
	a := &sortedSetReaderAsDV{reset: reset}
	if err := a.Reset(); err != nil {
		return err
	}
	return c.real.AddSortedSetField(field, a)
}

// sortedReaderAsDV adapts a read-side spi.SortedDocValues to the internal
// dvSortedValues contract. Reset re-materialises a fresh forward iterator
// via the reset closure. Ordinal lookup (LookupOrd / GetValueCount) is
// served from the live iterator, which retains the full ord->bytes table
// for the segment.
type sortedReaderAsDV struct {
	reset func() (spi.SortedDocValues, error)
	cur   spi.SortedDocValues
}

func (a *sortedReaderAsDV) Reset() error {
	it, err := a.reset()
	if err != nil {
		return err
	}
	a.cur = it
	return nil
}

func (a *sortedReaderAsDV) NextDoc() (int, error) {
	doc, err := a.cur.NextDoc()
	if err != nil {
		return 0, err
	}
	return normalizeDVDoc(doc), nil
}

func (a *sortedReaderAsDV) OrdValue() (int, error) { return a.cur.OrdValue() }

func (a *sortedReaderAsDV) LookupOrd(ord int) ([]byte, error) { return a.cur.LookupOrd(ord) }

func (a *sortedReaderAsDV) GetValueCount() int { return a.cur.GetValueCount() }

// sortedSetReaderAsDV adapts a read-side spi.SortedSetDocValues to the
// internal dvSortedSetValues contract.
type sortedSetReaderAsDV struct {
	reset func() (spi.SortedSetDocValues, error)
	cur   spi.SortedSetDocValues
	// docCount is the number of ordinals bound to the current document,
	// counted lazily on the first NextOrd after NextDoc so DocValueCount
	// can report it without a second pass.
	docOrds []int
	ordPos  int
}

func (a *sortedSetReaderAsDV) Reset() error {
	it, err := a.reset()
	if err != nil {
		return err
	}
	a.cur = it
	a.docOrds = a.docOrds[:0]
	a.ordPos = 0
	return nil
}

func (a *sortedSetReaderAsDV) NextDoc() (int, error) {
	doc, err := a.cur.NextDoc()
	if err != nil {
		return 0, err
	}
	doc = normalizeDVDoc(doc)
	a.docOrds = a.docOrds[:0]
	a.ordPos = 0
	if doc == dvNoMoreDocs {
		return doc, nil
	}
	// Drain the current document's ordinals so DocValueCount can report the
	// count and NextOrd can replay them in order. SortedSetDocValues exposes
	// the ordinals via NextOrd (terminated by -1).
	for {
		ord, oerr := a.cur.NextOrd()
		if oerr != nil {
			return 0, oerr
		}
		if ord == spiSortedSetNoMoreOrds {
			break
		}
		a.docOrds = append(a.docOrds, ord)
	}
	return doc, nil
}

func (a *sortedSetReaderAsDV) NextOrd() (int, error) {
	if a.ordPos >= len(a.docOrds) {
		return spiSortedSetNoMoreOrds, nil
	}
	ord := a.docOrds[a.ordPos]
	a.ordPos++
	return ord, nil
}

// NextOrdTerm walks the ord->bytes table in ascending ordinal order, used by
// the consumer to write the terms dictionary.
func (a *sortedSetReaderAsDV) NextOrdTerm() ([]byte, error) {
	if a.ordPos >= a.cur.GetValueCount() {
		return nil, nil
	}
	term, err := a.cur.LookupOrd(a.ordPos)
	a.ordPos++
	return term, err
}

func (a *sortedSetReaderAsDV) LookupOrd(ord int) ([]byte, error) { return a.cur.LookupOrd(ord) }

func (a *sortedSetReaderAsDV) GetValueCount() int { return a.cur.GetValueCount() }

func (a *sortedSetReaderAsDV) DocValueCount() (int, error) { return len(a.docOrds), nil }

// spiSortedSetNoMoreOrds mirrors the -1 end-of-ordinal-stream sentinel that
// spi.SortedSetDocValues.NextOrd returns when the current document has no more
// ordinals (org.apache.lucene.index.SortedSetDocValues.NO_MORE_ORDS).
const spiSortedSetNoMoreOrds = -1

// normalizeDVDoc translates the exhaustion sentinel of the index-side in-memory
// doc-values read views to the dvNoMoreDocs (DocIdSetIterator.NO_MORE_DOCS =
// Integer.MAX_VALUE) sentinel the codec consumer iterates against.
//
// The in-memory SortedDocValuesWriter / SortedSetDocValuesWriter read views
// return index.NO_MORE_DOCS (== schema.NO_MORE_DOCS == -1, the PostingsEnum
// no-more sentinel) when their forward iterator is exhausted, rather than the
// DocIdSetIterator MAX_VALUE sentinel the doc-values write path uses. Any
// negative doc returned by those views therefore means "exhausted"; map it to
// dvNoMoreDocs so the consumer loop terminates.
func normalizeDVDoc(doc int) int {
	if doc < 0 {
		return dvNoMoreDocs
	}
	return doc
}
