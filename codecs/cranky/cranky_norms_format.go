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

// CrankyNormsFormat is a NormsFormat that randomly throws IOExceptions
// to test the robustness of the indexing chain.
type CrankyNormsFormat struct {
	delegate spi.NormsFormat
	random   *rand.Rand
}

// NewCrankyNormsFormat creates a new CrankyNormsFormat.
func NewCrankyNormsFormat(delegate spi.NormsFormat, random *rand.Rand) *CrankyNormsFormat {
	return &CrankyNormsFormat{
		delegate: delegate,
		random:   random,
	}
}

// Name returns the format name.
func (f *CrankyNormsFormat) Name() string {
	return f.delegate.Name()
}

// NormsConsumer returns a norms consumer.
func (f *CrankyNormsFormat) NormsConsumer(state *index.SegmentWriteState) (spi.NormsConsumer, error) {
	if f.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException from NormsFormat.NormsConsumer()")
	}
	consumer, err := f.delegate.NormsConsumer(state)
	if err != nil {
		return nil, err
	}
	return &crankyNormsConsumer{
		delegate: consumer,
		random:   f.random,
	}, nil
}

// NormsProducer returns a norms producer.
func (f *CrankyNormsFormat) NormsProducer(state *index.SegmentReadState) (spi.NormsProducer, error) {
	return f.delegate.NormsProducer(state)
}

type crankyNormsConsumer struct {
	delegate spi.NormsConsumer
	random   *rand.Rand
}

func (c *crankyNormsConsumer) AddNormsField(field *index.FieldInfo, valuesProducer spi.NormsProducer) error {
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from NormsConsumer.AddNormsField()")
	}
	return c.delegate.AddNormsField(field, valuesProducer)
}

func (c *crankyNormsConsumer) Close() error {
	err := c.delegate.Close()
	if c.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from NormsConsumer.Close()")
	}
	return err
}
