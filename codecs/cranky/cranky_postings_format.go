// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package cranky

import (
	"fmt"
	"math/rand"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// CrankyPostingsFormat is a PostingsFormat that randomly throws IOExceptions
// to test the robustness of the indexing chain.
type CrankyPostingsFormat struct {
	delegate spi.PostingsFormat
	random   *rand.Rand
}

// NewCrankyPostingsFormat creates a new CrankyPostingsFormat.
func NewCrankyPostingsFormat(delegate spi.PostingsFormat, random *rand.Rand) *CrankyPostingsFormat {
	return &CrankyPostingsFormat{
		delegate: delegate,
		random:   random,
	}
}

// Name returns the format name.
func (f *CrankyPostingsFormat) Name() string {
	return f.delegate.Name()
}

// FieldsConsumer returns a fields consumer.
func (f *CrankyPostingsFormat) FieldsConsumer(state *index.SegmentWriteState) (spi.FieldsConsumer, error) {
	if f.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException from PostingsFormat.FieldsConsumer()")
	}
	consumer, err := f.delegate.FieldsConsumer(state)
	if err != nil {
		return nil, err
	}
	return &crankyFieldsConsumer{
		delegate: consumer,
		random:   f.random,
	}, nil
}

// FieldsProducer returns a fields producer.
func (f *CrankyPostingsFormat) FieldsProducer(state *index.SegmentReadState) (spi.FieldsProducer, error) {
	return f.delegate.FieldsProducer(state)
}

type crankyFieldsConsumer struct {
	delegate spi.FieldsConsumer
	random   *rand.Rand
}

func (c *crankyFieldsConsumer) Write(field string, terms spi.Terms) error {
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from FieldsConsumer.Write()")
	}
	return c.delegate.Write(field, terms)
}

func (c *crankyFieldsConsumer) Close() error {
	err := c.delegate.Close()
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from FieldsConsumer.Close()")
	}
	return err
}
