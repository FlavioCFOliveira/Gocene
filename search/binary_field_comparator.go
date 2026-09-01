// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// BinaryFieldComparator orders documents by the unsigned byte order of their binary values.
//
// This is the Go port of Lucene's org.apache.lucene.search.FieldComparator.TermValComparator.
type BinaryFieldComparator struct {
	field          string
	values         [][]byte
	docTerms       spi.BinaryDocValues
	bottom         []byte
	topValue       []byte
	missingSortCmp int

	dvSource func(index.IndexReader) (spi.BinaryDocValues, error)
}

func NewBinaryFieldComparator(numHits int, field string, sortMissingLast bool, bsf *BinarySortField) *BinaryFieldComparator {
	missingCmp := -1
	if sortMissingLast {
		missingCmp = 1
	}
	return &BinaryFieldComparator{
		field:          field,
		values:         make([][]byte, numHits),
		missingSortCmp: missingCmp,
		dvSource: func(r index.IndexReader) (spi.BinaryDocValues, error) {
			return bsf.GetSortKeyDocValues(r)
		},
	}
}

func (c *BinaryFieldComparator) compare(slot1, slot2 int) int {
	v1, v2 := c.values[slot1], c.values[slot2]
	return c.compareValues(v1, v2)
}

func (c *BinaryFieldComparator) value(slot int) any {
	return c.values[slot]
}

func (c *BinaryFieldComparator) setReader(reader index.IndexReader) error {
	dv, err := c.dvSource(reader)
	if err != nil {
		return err
	}
	c.docTerms = dv
	return nil
}

func (c *BinaryFieldComparator) Compare(slot1, slot2 int) int {
	return c.compare(slot1, slot2)
}

func (c *BinaryFieldComparator) SetBottom(slot int) {
	c.bottom = c.values[slot]
}

func (c *BinaryFieldComparator) CompareBottom(doc int) int {
	val := c.getValueForDoc(doc)
	return c.compareValues(c.bottom, val)
}

func (c *BinaryFieldComparator) Copy(slot, doc int) {
	val := c.getValueForDoc(doc)
	if val == nil {
		c.values[slot] = nil
	} else {
		cp := make([]byte, len(val))
		copy(cp, val)
		c.values[slot] = cp
	}
}

func (c *BinaryFieldComparator) SetScorer(scorer Scorer) {}

func (c *BinaryFieldComparator) compareValues(v1, v2 []byte) int {
	if v1 == nil {
		if v2 == nil {
			return 0
		}
		return c.missingSortCmp
	} else if v2 == nil {
		return -c.missingSortCmp
	}
	return bytes.Compare(v1, v2)
}

func (c *BinaryFieldComparator) getValueForDoc(doc int) []byte {
	if c.docTerms == nil {
		return nil
	}
	exists, err := c.docTerms.AdvanceExact(doc)
	if err != nil || !exists {
		return nil
	}
	val, err := c.docTerms.BinaryValue()
	if err != nil {
		return nil
	}
	return val
}

func (c *BinaryFieldComparator) getLeafComparator(ctx *index.LeafReaderContext) (LeafFieldComparator, error) {
	if err := c.setReader(ctx.LeafReader()); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *BinaryFieldComparator) CompareTop(doc int) int {
	val := c.getValueForDoc(doc)
	return c.compareValues(c.topValue, val)
}

func (c *BinaryFieldComparator) SetTopValue(val []byte) {
	c.topValue = val
}
