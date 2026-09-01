// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// BinarySortField sorts documents by the unsigned byte order of a field's binary values.
//
// This is the Go port of org.apache.lucene.search.BinarySortField from Apache Lucene 10.5.0.
// Unlike a STRING sort (which uses SortedDocValues), BinarySortField compares bytes directly,
// making it efficient for high-cardinality fields.
type BinarySortField struct {
	*SortField
	providerName string
}

// NewBinarySortField creates a sort, possibly in reverse, by the unsigned byte order of the field's value.
// If missingValue is nil, it defaults to STRING_FIRST.
func NewBinarySortField(field string, reverse bool, missingValue interface{}) *BinarySortField {
	return NewBinarySortFieldWithProvider(field, reverse, missingValue, "BinarySortField")
}

// NewBinarySortFieldWithProvider creates a sort for a subclass (or custom provider).
func NewBinarySortFieldWithProvider(field string, reverse bool, missingValue interface{}, providerName string) *BinarySortField {
	// Validate missingValue: must be nil, STRING_FIRST or STRING_LAST.
	if missingValue != nil && missingValue != STRING_FIRST && missingValue != STRING_LAST {
		panic("missing value for BinarySortField must be nil, STRING_FIRST or STRING_LAST")
	}

	// Lucene's BinarySortField defaults to missing first.
	missing := MissingValueFirst
	if missingValue == STRING_LAST {
		missing = MissingValueLast
	}

	return &BinarySortField{
		SortField: &SortField{
			Field:    field,
			Type:     SortFieldTypeCustom,
			Reverse:  reverse,
			Missing:  missing,
		},
		providerName: providerName,
	}
}

// ProviderName returns the registered SortFieldProvider name for this field.
func (bsf *BinarySortField) ProviderName() string {
	return bsf.providerName
}

// Serialize writes the sort field to the output.
// Mirrors BinarySortField.serialize in Lucene 10.5.0.
func (bsf *BinarySortField) Serialize(out store.DataOutput) error {
	if err := out.WriteString(bsf.GetField()); err != nil {
		return err
	}
	var rev int32
	if bsf.GetReverse() {
		rev = 1
	}
	if err := out.WriteInt(rev); err != nil {
		return err
	}
	var mv int32
	if bsf.MissingValue == STRING_FIRST {
		mv = 1
	} else if bsf.MissingValue == STRING_LAST {
		mv = 2
	} else {
		mv = 0
	}
	return out.WriteInt(mv)
}

// getSortKeyDocValues returns the binary doc values for the field.
func (bsf *BinarySortField) getSortKeyDocValues(reader index.IndexReader) (index.BinaryDocValues, error) {
	// In Gocene, we access BinaryDocValues via the reader or a helper.
	// Assuming the reader implementation handles this or we use the index package helper.
	// For this port, we implement the lookup.
	return index.GetBinary(reader, bsf.GetField())
}

// BinarySortFieldProvider is the SPI provider for BinarySortField.
type BinarySortFieldProvider struct{}

func (p *BinarySortFieldProvider) Name() string {
	return "BinarySortField"
}

func (p *BinarySortFieldProvider) ReadSortField(in store.DataInput) (index.SortFieldValue, error) {
	field, err := in.ReadString()
	if err != nil {
		return nil, err
	}
	revInt, err := in.ReadInt()
	if err != nil {
		return nil, err
	}
	reverse := revInt == 1
	mvInt, err := in.ReadInt()
	if err != nil {
		return nil, err
	}
	var missingValue interface{}
	if mvInt == 1 {
		missingValue = STRING_FIRST
	} else if mvInt == 2 {
		missingValue = STRING_LAST
	}
	return NewBinarySortField(field, reverse, missingValue), nil
}

func (p *BinarySortFieldProvider) WriteSortField(sf index.SortFieldValue, out store.DataOutput) error {
	bsf, ok := sf.(*BinarySortField)
	if !ok {
		return fmt.Errorf("cannot serialize sort field: not a BinarySortField")
	}
	return bsf.Serialize(out)
}

func init() {
	index.RegisterSortFieldProvider(&BinarySortFieldProvider{})
}

// binaryValComparator is the internal comparator for BinarySortField.
// Mirrors org.apache.lucene.search.FieldComparator.TermValComparator.
type binaryValComparator struct {
	field          string
	values         [][]byte
	docTerms       index.BinaryDocValues
	bottom         []byte
	topValue       []byte
	missingSortCmp int

	dvSource func(index.IndexReader) (index.BinaryDocValues, error)
}

func newBinaryValComparator(numHits int, field string, sortMissingLast bool, bsf *BinarySortField) *binaryValComparator {
	missingCmp := -1
	if sortMissingLast {
		missingCmp = 1
	}
	return &binaryValComparator{
		field:          field,
		values:         make([][]byte, numHits),
		missingSortCmp: missingCmp,
		dvSource: func(r index.IndexReader) (index.BinaryDocValues, error) {
			return bsf.getSortKeyDocValues(r)
		},
	}
}

func (c *binaryValComparator) compare(slot1, slot2 int) int {
	v1, v2 := c.values[slot1], c.values[slot2]
	return c.compareValues(v1, v2)
}

func (c *binaryValComparator) value(slot int) any {
	return c.values[slot]
}

func (c *binaryValComparator) setReader(reader index.IndexReader) error {
	dv, err := c.dvSource(reader)
	if err != nil {
		return err
	}
	c.docTerms = dv
	return nil
}

func (c *binaryValComparator) Compare(slot1, slot2 int) int {
	return c.compare(slot1, slot2)
}

func (c *binaryValComparator) SetBottom(slot int) {
	c.bottom = c.values[slot]
}

func (c *binaryValComparator) CompareBottom(doc int) int {
	val := c.getValueForDoc(doc)
	return c.compareValues(c.bottom, val)
}

func (c *binaryValComparator) Copy(slot, doc int) {
	val := c.getValueForDoc(doc)
	if val == nil {
		c.values[slot] = nil
	} else {
		cp := make([]byte, len(val))
		copy(cp, val)
		c.values[slot] = cp
	}
}

func (c *binaryValComparator) SetScorer(scorer Scorer) {}

func (c *binaryValComparator) compareValues(v1, v2 []byte) int {
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

func (c *binaryValComparator) getValueForDoc(doc int) []byte {
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

func (c *binaryValComparator) getLeafComparator(ctx *index.LeafReaderContext) (LeafFieldComparator, error) {
	if err := c.setReader(ctx.LeafReader()); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *binaryValComparator) CompareTop(doc int) int {
	val := c.getValueForDoc(doc)
	return c.compareValues(c.topValue, val)
}

func (c *binaryValComparator) SetTopValue(val []byte) {
	c.topValue = val
}
