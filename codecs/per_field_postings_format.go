// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PerFieldPostingsFormat is a PostingsFormat that can use a different
// PostingsFormat for each field.
// This is the Go port of Lucene's PerFieldPostingsFormat.
type PerFieldPostingsFormat struct {
	*BasePostingsFormat
	formatProvider FieldPostingsFormatProvider
}

// FieldPostingsFormatProvider provides a PostingsFormat for a given field.
type FieldPostingsFormatProvider interface {
	// GetPostingsFormat returns the PostingsFormat to use for the given field.
	GetPostingsFormat(field string) PostingsFormat
}

// FieldPostingsFormatProviderFunc is a function type that implements FieldPostingsFormatProvider.
type FieldPostingsFormatProviderFunc func(field string) PostingsFormat

// GetPostingsFormat returns the PostingsFormat for the given field.
func (f FieldPostingsFormatProviderFunc) GetPostingsFormat(field string) PostingsFormat {
	return f(field)
}

// NewPerFieldPostingsFormat creates a new PerFieldPostingsFormat.
func NewPerFieldPostingsFormat(provider FieldPostingsFormatProvider) *PerFieldPostingsFormat {
	return &PerFieldPostingsFormat{
		BasePostingsFormat: NewBasePostingsFormat("PerFieldPostingsFormat"),
		formatProvider:     provider,
	}
}

// NewPerFieldPostingsFormatWithDefault creates a new PerFieldPostingsFormat
// with a default format for all fields.
func NewPerFieldPostingsFormatWithDefault(defaultFormat PostingsFormat) *PerFieldPostingsFormat {
	return NewPerFieldPostingsFormat(FieldPostingsFormatProviderFunc(func(field string) PostingsFormat {
		return defaultFormat
	}))
}

// FieldsConsumer returns a FieldsConsumer that delegates to the appropriate
// PostingsFormat for each field.
func (f *PerFieldPostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	return NewPerFieldFieldsConsumer(f.formatProvider, state), nil
}

// FieldsProducer returns a FieldsProducer that delegates to the appropriate
// PostingsFormat for each field.
func (f *PerFieldPostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	return NewPerFieldFieldsProducer(f.formatProvider, state)
}

// PerFieldFieldsConsumer is a FieldsConsumer that delegates to different
// PostingsFormats for different fields.
type PerFieldFieldsConsumer struct {
	formatProvider FieldPostingsFormatProvider
	state          *SegmentWriteState
	consumers      map[string]FieldsConsumer
	mu             sync.Mutex
	closed         bool
}

// NewPerFieldFieldsConsumer creates a new PerFieldFieldsConsumer.
func NewPerFieldFieldsConsumer(provider FieldPostingsFormatProvider, state *SegmentWriteState) *PerFieldFieldsConsumer {
	return &PerFieldFieldsConsumer{
		formatProvider: provider,
		state:          state,
		consumers:      make(map[string]FieldsConsumer),
	}
}

// Write writes a field's postings using the appropriate PostingsFormat.
func (c *PerFieldFieldsConsumer) Write(field string, terms index.Terms) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("PerFieldFieldsConsumer is closed")
	}

	// Get or create the consumer for this field
	consumer, ok := c.consumers[field]
	if !ok {
		format := c.formatProvider.GetPostingsFormat(field)
		if format == nil {
			return fmt.Errorf("no PostingsFormat available for field %s", field)
		}

		// Create a new state with a segment suffix for this field's format
		fieldState := &SegmentWriteState{
			Directory:     c.state.Directory,
			SegmentInfo:   c.state.SegmentInfo,
			FieldInfos:    c.state.FieldInfos,
			SegmentSuffix: field,
		}

		var err error
		consumer, err = format.FieldsConsumer(fieldState)
		if err != nil {
			return fmt.Errorf("failed to create FieldsConsumer for field %s: %w", field, err)
		}
		c.consumers[field] = consumer
	}

	return consumer.Write(field, terms)
}

// Close closes all delegated consumers.
func (c *PerFieldFieldsConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	var lastErr error
	for field, consumer := range c.consumers {
		if err := consumer.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close consumer for field %s: %w", field, err)
		}
	}
	c.consumers = nil

	return lastErr
}

// PerFieldFieldsProducer is a FieldsProducer that delegates to different
// PostingsFormats for different fields.
type PerFieldFieldsProducer struct {
	formatProvider FieldPostingsFormatProvider
	state          *SegmentReadState
	producers      map[string]FieldsProducer
	mu             sync.RWMutex
	closed         bool
}

// NewPerFieldFieldsProducer creates a new PerFieldFieldsProducer.
func NewPerFieldFieldsProducer(provider FieldPostingsFormatProvider, state *SegmentReadState) (*PerFieldFieldsProducer, error) {
	return &PerFieldFieldsProducer{
		formatProvider: provider,
		state:          state,
		producers:      make(map[string]FieldsProducer),
	}, nil
}

// Terms returns the terms for a field using the appropriate PostingsFormat.
func (p *PerFieldFieldsProducer) Terms(field string) (index.Terms, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("PerFieldFieldsProducer is closed")
	}

	// Get or create the producer for this field
	producer, ok := p.producers[field]
	if !ok {
		format := p.formatProvider.GetPostingsFormat(field)
		if format == nil {
			return nil, nil
		}

		// Create a new state with a segment suffix for this field's format
		fieldState := &SegmentReadState{
			Directory:     p.state.Directory,
			SegmentInfo:   p.state.SegmentInfo,
			FieldInfos:    p.state.FieldInfos,
			SegmentSuffix: field,
		}

		var err error
		producer, err = format.FieldsProducer(fieldState)
		if err != nil {
			return nil, fmt.Errorf("failed to create FieldsProducer for field %s: %w", field, err)
		}
		p.producers[field] = producer
	}

	return producer.Terms(field)
}

// Close closes all delegated producers.
func (p *PerFieldFieldsProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	var lastErr error
	for field, producer := range p.producers {
		if err := producer.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close producer for field %s: %w", field, err)
		}
	}
	p.producers = nil

	return lastErr
}

// MapFieldPostingsFormatProvider is a FieldPostingsFormatProvider that
// maps specific fields to specific PostingsFormats.
type MapFieldPostingsFormatProvider struct {
	mu            sync.RWMutex
	fieldFormats  map[string]PostingsFormat
	defaultFormat PostingsFormat
}

// NewMapFieldPostingsFormatProvider creates a new MapFieldPostingsFormatProvider.
func NewMapFieldPostingsFormatProvider(defaultFormat PostingsFormat) *MapFieldPostingsFormatProvider {
	return &MapFieldPostingsFormatProvider{
		fieldFormats:  make(map[string]PostingsFormat),
		defaultFormat: defaultFormat,
	}
}

// SetFormat sets the PostingsFormat for a specific field.
func (p *MapFieldPostingsFormatProvider) SetFormat(field string, format PostingsFormat) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fieldFormats[field] = format
}

// GetPostingsFormat returns the PostingsFormat for the given field.
func (p *MapFieldPostingsFormatProvider) GetPostingsFormat(field string) PostingsFormat {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if format, ok := p.fieldFormats[field]; ok {
		return format
	}
	return p.defaultFormat
}

// Ensure implementations satisfy the interfaces
var _ PostingsFormat = (*PerFieldPostingsFormat)(nil)
var _ FieldsConsumer = (*PerFieldFieldsConsumer)(nil)
var _ FieldsProducer = (*PerFieldFieldsProducer)(nil)
var _ FieldPostingsFormatProvider = (*MapFieldPostingsFormatProvider)(nil)
var _ FieldPostingsFormatProvider = (FieldPostingsFormatProviderFunc)(nil)
