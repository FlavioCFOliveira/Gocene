// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// This file holds the adapters that present the read-side SortedDocValues /
// SortedSetDocValues obtained from the DocValuesProducer handed to
// Lucene90DocValuesConsumer.AddSortedField / AddSortedSetField as the
// internal dvSortedValues / dvSortedSetValues contracts the byte-faithful
// lucene90DVConsumer iterates. The reset closure obtains a fresh forward
// iterator from the producer on every pass (the consumer makes several
// passes over the values), where the Java consumer calls
// valuesProducer.getSorted(field) / getSortedSet(field) again.

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
