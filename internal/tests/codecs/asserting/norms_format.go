// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package asserting

import (
	"fmt"

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
		maxDoc: state.SegmentInfo.MaxDoc(),
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
		maxDoc: state.SegmentInfo.MaxDoc(),
	}, nil
}

type assertingNormsConsumer struct {
	in     spi.NormsConsumer
	maxDoc int
}

// AddNormsField walks the producer's NumericDocValues once to assert the
// docID contract, then forwards the same producer to the delegate.
//
// Mirrors AssertingNormsFormat.AssertingNormsConsumer.addNormsField(FieldInfo,
// NormsProducer) of Apache Lucene 10.5.0.
func (c *assertingNormsConsumer) AddNormsField(field *spi.FieldInfo, valuesProducer spi.NormsProducer) error {
	values, err := valuesProducer.GetNorms(field)
	if err != nil {
		return err
	}

	lastDocID := -1
	for {
		docID, err := values.NextDoc()
		if err != nil {
			return err
		}
		if docID == index.NO_MORE_DOCS {
			break
		}
		if docID < 0 || docID >= c.maxDoc {
			panic(fmt.Sprintf("docID %d out of bounds [0, %d)", docID, c.maxDoc))
		}
		if docID <= lastDocID {
			panic(fmt.Sprintf("docID %d must be strictly increasing (last was %d)", docID, lastDocID))
		}
		lastDocID = docID
		if _, err := values.LongValue(); err != nil {
			return err
		}
	}

	return c.in.AddNormsField(field, valuesProducer)
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
	return index.NewAssertingNumericDocValues(values, p.maxDoc), nil
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
