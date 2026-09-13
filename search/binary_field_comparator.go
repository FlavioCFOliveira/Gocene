// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// BinaryFieldComparator sorts by a field's natural term sort order. All
// comparisons are done using unsigned byte order, which is slow for medium to
// large result sets but possibly very fast for very small result sets.
//
// This is the Go port of org.apache.lucene.search.FieldComparator.TermValComparator,
// which extends FieldComparator<BytesRef> and implements LeafFieldComparator;
// the Go type therefore satisfies both [FieldComparator] and
// [LeafFieldComparator].
type BinaryFieldComparator struct {
	BaseFieldComparator

	field          string
	values         [][]byte
	docTerms       spi.BinaryDocValues
	bottom         []byte
	topValue       []byte
	missingSortCmp int

	dvSource func(index.IndexReader) (spi.BinaryDocValues, error)
}

// NewBinaryFieldComparator creates the comparator for numHits queue slots.
//
// bsf is the BinarySortField whose getSortKeyDocValues override resolves the
// per-document sort key; it is nil for a plain Type.STRING_VAL SortField, in
// which case the comparator reads the field's BinaryDocValues directly — the
// body of BinarySortField.getSortKeyDocValues, and of
// TermValComparator.getBinaryDocValues, which is DocValues.getBinary(reader, field).
//
// Mirrors FieldComparator.TermValComparator(int, String, boolean).
func NewBinaryFieldComparator(numHits int, field string, sortMissingLast bool, bsf *BinarySortField) *BinaryFieldComparator {
	missingCmp := -1
	if sortMissingLast {
		missingCmp = 1
	}
	return &BinaryFieldComparator{
		field:          field,
		values:         make([][]byte, numHits),
		missingSortCmp: missingCmp,
		// Java's BinarySortField.getComparator overrides
		// TermValComparator.getBinaryDocValues(LeafReaderContext, String) to
		// call getSortKeyDocValues(context.reader()), whose default body is
		// DocValues.getBinary(reader, field). Gocene's LeafReaderContext
		// exposes IndexReaderInterface, which does not declare the
		// doc-values accessor, so the leaf reader is reached through the
		// narrow type assertion already used by LatLonPointDistanceComparator
		// and XYPointDistanceComparator. A reader without that surface falls
		// back to an empty stream, mirroring DocValues.getBinary's
		// null-defence path.
		dvSource: func(r index.IndexReader) (spi.BinaryDocValues, error) {
			provider, ok := r.(binaryDocValuesProvider)
			if !ok {
				return index.EmptyBinary(), nil
			}
			if bsf == nil {
				return provider.GetBinaryDocValues(field)
			}
			return bsf.GetSortKeyDocValues(provider)
		},
	}
}

// Compare compares the terms cached in the two slots.
//
// Mirrors TermValComparator.compare.
func (c *BinaryFieldComparator) Compare(slot1, slot2 int) int {
	return c.CompareValues(c.values[slot1], c.values[slot2])
}

// Value returns the term cached in the slot, or nil for a missing value.
//
// Mirrors TermValComparator.value.
func (c *BinaryFieldComparator) Value(slot int) any {
	if c.values[slot] == nil {
		return nil
	}
	return c.values[slot]
}

// setReader binds the comparator to a leaf reader's binary doc values.
func (c *BinaryFieldComparator) setReader(reader index.IndexReader) error {
	if reader == nil {
		c.docTerms = index.EmptyBinary()
		return nil
	}
	dv, err := c.dvSource(reader)
	if err != nil {
		return err
	}
	c.docTerms = dv
	return nil
}

// GetLeafComparator binds the comparator to the segment's binary doc values and
// returns itself, as TermValComparator does (it implements LeafFieldComparator).
//
// Mirrors TermValComparator.getLeafComparator(LeafReaderContext).
func (c *BinaryFieldComparator) GetLeafComparator(context *index.LeafReaderContext) (LeafFieldComparator, error) {
	var reader index.IndexReader
	if context != nil && context.LeafReader() != nil {
		reader = context.LeafReader()
	}
	if err := c.setReader(reader); err != nil {
		return nil, err
	}
	return c, nil
}

// SetBottom records the bottom slot's term.
//
// Mirrors TermValComparator.setBottom.
func (c *BinaryFieldComparator) SetBottom(slot int) error {
	c.bottom = c.values[slot]
	return nil
}

// CompareBottom compares the bottom term with the term of doc.
//
// Mirrors TermValComparator.compareBottom.
func (c *BinaryFieldComparator) CompareBottom(doc int) (int, error) {
	val, err := c.getValueForDoc(doc)
	if err != nil {
		return 0, err
	}
	return c.CompareValues(c.bottom, val), nil
}

// CompareTop compares the top term with the term of doc.
//
// Mirrors TermValComparator.compareTop.
func (c *BinaryFieldComparator) CompareTop(doc int) (int, error) {
	val, err := c.getValueForDoc(doc)
	if err != nil {
		return 0, err
	}
	return c.CompareValues(c.topValue, val), nil
}

// SetTopValue records the top term. A nil value is fine: it means the last doc
// of the prior search was missing this value.
//
// Mirrors TermValComparator.setTopValue.
func (c *BinaryFieldComparator) SetTopValue(value any) {
	c.topValue = bytesRefValue(value, "SetTopValue")
}

// Copy caches the term of doc into the slot.
//
// Mirrors TermValComparator.copy.
func (c *BinaryFieldComparator) Copy(slot, doc int) error {
	val, err := c.getValueForDoc(doc)
	if err != nil {
		return err
	}
	if val == nil {
		c.values[slot] = nil
		return nil
	}
	cp := make([]byte, len(val))
	copy(cp, val)
	c.values[slot] = cp
	return nil
}

// SetScorer is empty, as TermValComparator.setScorer is.
func (c *BinaryFieldComparator) SetScorer(Scorable) error { return nil }

// CompetitiveIterator returns nil: TermValComparator does not override the
// LeafFieldComparator default, which returns null.
func (c *BinaryFieldComparator) CompetitiveIterator() (DocIdSetIterator, error) { return nil, nil }

// SetHitsThresholdReached is empty: TermValComparator does not override the
// LeafFieldComparator default, whose body is empty.
func (c *BinaryFieldComparator) SetHitsThresholdReached() error { return nil }

// CompareValues orders two terms; a missing value sorts first or last according
// to the comparator's missing-value placement.
//
// Mirrors TermValComparator.compareValues(BytesRef, BytesRef), which overrides
// the FieldComparator default.
func (c *BinaryFieldComparator) CompareValues(first, second any) int {
	val1 := bytesRefValue(first, "CompareValues")
	val2 := bytesRefValue(second, "CompareValues")
	// missing always sorts first:
	if val1 == nil {
		if val2 == nil {
			return 0
		}
		return c.missingSortCmp
	} else if val2 == nil {
		return -c.missingSortCmp
	}
	return bytes.Compare(val1, val2)
}

// getValueForDoc positions the bound iterator on doc and returns its term, or
// nil when the document has no value.
//
// Mirrors the private TermValComparator.getValueForDoc, which declares
// `throws IOException`; the error is propagated here rather than discarded.
func (c *BinaryFieldComparator) getValueForDoc(doc int) ([]byte, error) {
	if c.docTerms == nil {
		return nil, nil
	}
	exists, err := c.docTerms.AdvanceExact(doc)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return c.docTerms.BinaryValue()
}

var (
	_ FieldComparator     = (*BinaryFieldComparator)(nil)
	_ LeafFieldComparator = (*BinaryFieldComparator)(nil)
)
