// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package asserting

import (
	"fmt"
	"runtime"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// AssertingNormsFormat is a wrapper around a NormsFormat that adds additional assertions.
// Mirrors org.apache.lucene.tests.codecs.asserting.AssertingNormsFormat in Apache Lucene 10.5.0.
type AssertingNormsFormat struct {
	in spi.NormsFormat
}

func NewAssertingNormsFormat(in spi.NormsFormat) *AssertingNormsFormat {
	return &AssertingNormsFormat{in: in}
}

func NewAssertingNormsFormatDefault() *AssertingNormsFormat {
	return NewAssertingNormsFormat(index.GetDefaultCodec().NormsFormat())
}

func (f *AssertingNormsFormat) Name() string {
	return "AssertingNormsFormat(" + f.in.Name() + ")"
}

func (f *AssertingNormsFormat) NormsConsumer(state *spi.SegmentWriteState) (spi.NormsConsumer, error) {
	consumer, err := f.in.NormsConsumer(state)
	if err != nil {
		return nil, err
	}
	if consumer == nil {
		panic("NormsConsumer must not be nil")
	}
	return &assertingNormsConsumer{
		in:     consumer,
		maxDoc: state.SegmentInfo.MaxDoc,
	}, nil
}

func (f *AssertingNormsFormat) NormsProducer(state *spi.SegmentReadState) (spi.NormsProducer, error) {
	if !state.FieldInfos.HasNorms() {
		panic("state.FieldInfos must have norms")
	}
	producer, err := f.in.NormsProducer(state)
	if err != nil {
		return nil, err
	}
	if producer == nil {
		panic("NormsProducer must not be nil")
	}
	return &assertingNormsProducer{
		in:     producer,
		maxDoc: state.SegmentInfo.MaxDoc,
	}, nil
}

type assertingNormsConsumer struct {
	in     spi.NormsConsumer
	maxDoc int
}

func (c *assertingNormsConsumer) AddNormsField(field *spi.FieldInfo, values spi.NormsIterator) error {
	lastDocID := -1
	for values.Next() {
		docID := values.DocID()
		if docID < 0 || docID >= c.maxDoc {
			panic(fmt.Sprintf("docID %d out of bounds [0, %d)", docID, c.maxDoc))
		}
		if docID <= lastDocID {
			panic(fmt.Sprintf("docID %d must be strictly increasing (last was %d)", docID, lastDocID))
		}
		lastDocID = docID
		_, _ = values.LongValue()
	}

	return c.in.AddNormsField(field, values)
}

func (c *assertingNormsConsumer) Close() error {
	err := c.in.Close()
	// Lucene calls close() twice to test idempotency
	_ = c.in.Close()
	return err
}

type assertingNormsProducer struct {
	in     spi.NormsProducer
	maxDoc int
}

func (p *assertingNormsProducer) GetNorms(field *spi.FieldInfo) (index.NumericDocValues, error) {
	if !field.HasNorms() {
		panic("field must have norms")
	}
	values, err := p.in.GetNorms(field)
	if err != nil {
		return nil, err
	}
	if values == nil {
		panic("NumericDocValues must not be nil")
	}
	return newAssertingNumericDocValues(values, p.maxDoc), nil
}

// GetMergeInstance wraps the delegate's merge instance in a fresh asserting
// producer.
//
// Mirrors AssertingNormsProducer.getMergeInstance() of Apache Lucene 10.5.0:
// new AssertingNormsProducer(in.getMergeInstance(), maxDoc, true).
func (p *assertingNormsProducer) GetMergeInstance() spi.NormsProducer {
	return &assertingNormsProducer{
		in:     p.in.GetMergeInstance(),
		maxDoc: p.maxDoc,
	}
}

func (p *assertingNormsProducer) CheckIntegrity() error {
	return p.in.CheckIntegrity()
}

func (p *assertingNormsProducer) Close() error {
	err := p.in.Close()
	// Lucene calls close() twice to test idempotency
	_ = p.in.Close()
	return err
}

type assertingNumericDocValues struct {
	in      index.NumericDocValues
	maxDoc  int
	lastDoc int
	exists  bool
}

func newAssertingNumericDocValues(in index.NumericDocValues, maxDoc int) *assertingNumericDocValues {
	if in.DocID() != -1 {
		panic("NumericDocValues should start unpositioned (docID == -1)")
	}
	return &assertingNumericDocValues{
		in:      in,
		maxDoc:  maxDoc,
		lastDoc: -1,
	}
}

// Note: AssertingNormsProducer.GetNorms uses this. I'll use a helper.

func (v *assertingNumericDocValues) DocID() int {
	return v.in.DocID()
}

func (v *assertingNumericDocValues) NextDoc() (int, error) {
	docID, err := v.in.NextDoc()
	if err != nil {
		return docID, err
	}
	if docID != -1 && (docID <= v.lastDoc || docID >= v.maxDoc) {
		panic(fmt.Sprintf("invalid nextDoc %d (last %d, max %d)", docID, v.lastDoc, v.maxDoc))
	}
	if docID != v.in.DocID() {
		panic(fmt.Sprintf("docID mismatch: %d vs %d", docID, v.in.DocID()))
	}
	v.lastDoc = docID
	v.exists = docID != -1
	return docID, nil
}

func (v *assertingNumericDocValues) Advance(target int) (int, error) {
	if target < 0 || target <= v.in.DocID() {
		panic(fmt.Sprintf("target %d must be >= 0 and > current docID %d", target, v.in.DocID()))
	}
	docID, err := v.in.Advance(target)
	if err != nil {
		return docID, err
	}
	if docID < target || (docID != -1 && docID >= v.maxDoc) {
		panic(fmt.Sprintf("invalid advanced docID %d (target %d, max %d)", docID, target, v.maxDoc))
	}
	v.lastDoc = docID
	v.exists = docID != -1
	return docID, nil
}

func (v *assertingNumericDocValues) AdvanceExact(target int) (bool, error) {
	if target < 0 || target < v.in.DocID() || target >= v.maxDoc {
		panic(fmt.Sprintf("target %d must be in [currentDoc, maxDoc)", v.in.DocID(), v.maxDoc))
	}
	exists, err := v.in.AdvanceExact(target)
	if err != nil {
		return exists, err
	}
	if v.in.DocID() != target {
		panic(fmt.Sprintf("expected docID %d, got %d", target, v.in.DocID()))
	}
	v.lastDoc = target
	v.exists = exists
	return exists, nil
}

func (v *assertingNumericDocValues) LongValue() (int64, error) {
	if !v.exists {
		panic("LongValue called on unpositioned or exhausted iterator")
	}
	return v.in.LongValue()
}

func (v *assertingNumericDocValues) LongValues(size int, docs []int32, values []int64, defaultValue int64) error {
	if size < 0 {
		panic("size must be >= 0")
	}
	if size > 0 {
		if docs[0] < 0 {
			panic("docs[0] must be >= 0")
		}
		for i := 1; i < size; i++ {
			if docs[i] <= docs[i-1] {
				panic(fmt.Sprintf("docs must be strictly increasing: docs[%d]=%d <= docs[%d]=%d", i, docs[i], i-1, docs[i-1]))
			}
		}
		if docs[size-1] >= int32(v.maxDoc) {
			panic(fmt.Sprintf("docs[%d]=%d must be < maxDoc %d", size-1, docs[size-1], v.maxDoc))
		}
	}

	err := v.in.LongValues(size, docs, values, defaultValue)
	if err != nil {
		return err
	}

	expectedDocID := v.in.DocID()
	if size > 0 {
		expectedDocID = int(docs[size-1])
	}

	if v.in.DocID() != expectedDocID {
		panic(fmt.Sprintf("expected docID %d after LongValues, got %d", expectedDocID, v.in.DocID()))
	}

	return nil
}

func (v *assertingNumericDocValues) LongValuesOffset(size int, docs []int32, docsOffset int, values []int64, valuesOffset int, defaultValue int64) error {
	if size < 0 {
		panic("size must be >= 0")
	}
	if size > 0 {
		if docs[docsOffset] < 0 {
			panic("docs[docsOffset] must be >= 0")
		}
		for i := 1; i < size; i++ {
			if docs[docsOffset+i] <= docs[docsOffset+i-1] {
				panic(fmt.Sprintf("docs must be strictly increasing: docs[%d]=%d <= docs[%d]=%d", docsOffset+i, docs[docsOffset+i], docsOffset+i-1, docs[docsOffset+i-1]))
			}
		}
		if docs[docsOffset+size-1] >= int32(v.maxDoc) {
			panic(fmt.Sprintf("docs[%d]=%d must be < maxDoc %d", docsOffset+size-1, docs[docsOffset+size-1], v.maxDoc))
		}
	}

	err := v.in.LongValuesOffset(size, docs, docsOffset, values, valuesOffset, defaultValue)
	if err != nil {
		return err
	}

	expectedDocID := v.in.DocID()
	if size > 0 {
		expectedDocID = int(docs[docsOffset+size-1])
	}

	if v.in.DocID() != expectedDocID {
		panic(fmt.Sprintf("expected docID %d after LongValuesOffset, got %d", expectedDocID, v.in.DocID()))
	}

	return nil
}

func (v *assertingNumericDocValues) RangeIntoBitSet(fromDoc, toDoc int, minValue, maxValue int64, bitSet search.BitSet, offset int) error {
	// Implement as needed, but the Java version focuses on docID bounds.
	return v.in.RangeIntoBitSet(fromDoc, toDoc, minValue, maxValue, bitSet, offset)
}
