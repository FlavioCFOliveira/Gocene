// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PerFieldDocValuesFormat is a DocValuesFormat that can use a different
// DocValuesFormat for each field.
// This is the Go port of Lucene's PerFieldDocValuesFormat.
type PerFieldDocValuesFormat struct {
	*BaseDocValuesFormat
	formatProvider FieldDocValuesFormatProvider
}

// FieldDocValuesFormatProvider provides a DocValuesFormat for a given field.
type FieldDocValuesFormatProvider interface {
	// GetDocValuesFormat returns the DocValuesFormat to use for the given field.
	GetDocValuesFormat(field string) DocValuesFormat
}

// FieldDocValuesFormatProviderFunc is a function type that implements FieldDocValuesFormatProvider.
type FieldDocValuesFormatProviderFunc func(field string) DocValuesFormat

// GetDocValuesFormat returns the DocValuesFormat for the given field.
func (f FieldDocValuesFormatProviderFunc) GetDocValuesFormat(field string) DocValuesFormat {
	return f(field)
}

// NewPerFieldDocValuesFormat creates a new PerFieldDocValuesFormat.
func NewPerFieldDocValuesFormat(provider FieldDocValuesFormatProvider) *PerFieldDocValuesFormat {
	return &PerFieldDocValuesFormat{
		BaseDocValuesFormat: NewBaseDocValuesFormat("PerFieldDocValuesFormat"),
		formatProvider:      provider,
	}
}

// NewPerFieldDocValuesFormatWithDefault creates a new PerFieldDocValuesFormat
// with a default format for all fields.
func NewPerFieldDocValuesFormatWithDefault(defaultFormat DocValuesFormat) *PerFieldDocValuesFormat {
	return NewPerFieldDocValuesFormat(FieldDocValuesFormatProviderFunc(func(field string) DocValuesFormat {
		return defaultFormat
	}))
}

// FieldsConsumer returns a DocValuesConsumer that delegates to the appropriate
// DocValuesFormat for each field.
func (f *PerFieldDocValuesFormat) FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error) {
	return NewPerFieldDocValuesConsumer(f.formatProvider, state), nil
}

// FieldsProducer returns a DocValuesProducer that delegates to the appropriate
// DocValuesFormat for each field.
func (f *PerFieldDocValuesFormat) FieldsProducer(state *SegmentReadState) (DocValuesProducer, error) {
	return NewPerFieldDocValuesProducer(f.formatProvider, state)
}

// PerFieldDocValuesConsumer is a DocValuesConsumer that delegates to different
// DocValuesFormats for different fields.
type PerFieldDocValuesConsumer struct {
	formatProvider FieldDocValuesFormatProvider
	state          *SegmentWriteState
	consumers      map[string]DocValuesConsumer
	mu             sync.Mutex
	closed         bool
}

// NewPerFieldDocValuesConsumer creates a new PerFieldDocValuesConsumer.
func NewPerFieldDocValuesConsumer(provider FieldDocValuesFormatProvider, state *SegmentWriteState) *PerFieldDocValuesConsumer {
	return &PerFieldDocValuesConsumer{
		formatProvider: provider,
		state:          state,
		consumers:      make(map[string]DocValuesConsumer),
	}
}

// getConsumer gets or creates a DocValuesConsumer for the given field.
func (c *PerFieldDocValuesConsumer) getConsumer(field *index.FieldInfo) (DocValuesConsumer, error) {
	fieldName := field.Name()

	consumer, ok := c.consumers[fieldName]
	if !ok {
		format := c.formatProvider.GetDocValuesFormat(fieldName)
		if format == nil {
			return nil, fmt.Errorf("no DocValuesFormat available for field %s", fieldName)
		}

		// Create a new state with a segment suffix for this field's format
		fieldState := &SegmentWriteState{
			Directory:     c.state.Directory,
			SegmentInfo:   c.state.SegmentInfo,
			FieldInfos:    c.state.FieldInfos,
			SegmentSuffix: fieldName,
		}

		var err error
		consumer, err = format.FieldsConsumer(fieldState)
		if err != nil {
			return nil, fmt.Errorf("failed to create DocValuesConsumer for field %s: %w", fieldName, err)
		}
		c.consumers[fieldName] = consumer
	}

	return consumer, nil
}

// AddNumericField writes a numeric doc values field.
func (c *PerFieldDocValuesConsumer) AddNumericField(field *index.FieldInfo, values NumericDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}

	consumer, err := c.getConsumer(field)
	if err != nil {
		return err
	}

	return consumer.AddNumericField(field, values)
}

// AddBinaryField writes a binary doc values field.
func (c *PerFieldDocValuesConsumer) AddBinaryField(field *index.FieldInfo, values BinaryDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}

	consumer, err := c.getConsumer(field)
	if err != nil {
		return err
	}

	return consumer.AddBinaryField(field, values)
}

// AddSortedField writes a sorted doc values field.
func (c *PerFieldDocValuesConsumer) AddSortedField(field *index.FieldInfo, values SortedDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}

	consumer, err := c.getConsumer(field)
	if err != nil {
		return err
	}

	return consumer.AddSortedField(field, values)
}

// AddSortedSetField writes a sorted set doc values field.
func (c *PerFieldDocValuesConsumer) AddSortedSetField(field *index.FieldInfo, values SortedSetDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}

	consumer, err := c.getConsumer(field)
	if err != nil {
		return err
	}

	return consumer.AddSortedSetField(field, values)
}

// AddSortedNumericField writes a sorted numeric doc values field.
func (c *PerFieldDocValuesConsumer) AddSortedNumericField(field *index.FieldInfo, values SortedNumericDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}

	consumer, err := c.getConsumer(field)
	if err != nil {
		return err
	}

	return consumer.AddSortedNumericField(field, values)
}

// Close closes all delegated consumers.
func (c *PerFieldDocValuesConsumer) Close() error {
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

// PerFieldDocValuesProducer is a DocValuesProducer that delegates to different
// DocValuesFormats for different fields.
type PerFieldDocValuesProducer struct {
	formatProvider FieldDocValuesFormatProvider
	state          *SegmentReadState
	producers      map[string]DocValuesProducer
	mu             sync.RWMutex
	closed         bool
}

// NewPerFieldDocValuesProducer creates a new PerFieldDocValuesProducer.
func NewPerFieldDocValuesProducer(provider FieldDocValuesFormatProvider, state *SegmentReadState) (*PerFieldDocValuesProducer, error) {
	return &PerFieldDocValuesProducer{
		formatProvider: provider,
		state:          state,
		producers:      make(map[string]DocValuesProducer),
	}, nil
}

// getProducer gets or creates a DocValuesProducer for the given field.
func (p *PerFieldDocValuesProducer) getProducer(field *index.FieldInfo) (DocValuesProducer, error) {
	fieldName := field.Name()

	producer, ok := p.producers[fieldName]
	if !ok {
		format := p.formatProvider.GetDocValuesFormat(fieldName)
		if format == nil {
			return nil, nil
		}

		// Create a new state with a segment suffix for this field's format
		fieldState := &SegmentReadState{
			Directory:     p.state.Directory,
			SegmentInfo:   p.state.SegmentInfo,
			FieldInfos:    p.state.FieldInfos,
			SegmentSuffix: fieldName,
		}

		var err error
		producer, err = format.FieldsProducer(fieldState)
		if err != nil {
			return nil, fmt.Errorf("failed to create DocValuesProducer for field %s: %w", fieldName, err)
		}
		p.producers[fieldName] = producer
	}

	return producer, nil
}

// GetNumeric returns a NumericDocValues for the given field.
func (p *PerFieldDocValuesProducer) GetNumeric(field *index.FieldInfo) (NumericDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("PerFieldDocValuesProducer is closed")
	}

	producer, err := p.getProducer(field)
	if err != nil {
		return nil, err
	}
	if producer == nil {
		return nil, nil
	}

	return producer.GetNumeric(field)
}

// GetBinary returns a BinaryDocValues for the given field.
func (p *PerFieldDocValuesProducer) GetBinary(field *index.FieldInfo) (BinaryDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("PerFieldDocValuesProducer is closed")
	}

	producer, err := p.getProducer(field)
	if err != nil {
		return nil, err
	}
	if producer == nil {
		return nil, nil
	}

	return producer.GetBinary(field)
}

// GetSorted returns a SortedDocValues for the given field.
func (p *PerFieldDocValuesProducer) GetSorted(field *index.FieldInfo) (SortedDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("PerFieldDocValuesProducer is closed")
	}

	producer, err := p.getProducer(field)
	if err != nil {
		return nil, err
	}
	if producer == nil {
		return nil, nil
	}

	return producer.GetSorted(field)
}

// GetSortedSet returns a SortedSetDocValues for the given field.
func (p *PerFieldDocValuesProducer) GetSortedSet(field *index.FieldInfo) (SortedSetDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("PerFieldDocValuesProducer is closed")
	}

	producer, err := p.getProducer(field)
	if err != nil {
		return nil, err
	}
	if producer == nil {
		return nil, nil
	}

	return producer.GetSortedSet(field)
}

// GetSortedNumeric returns a SortedNumericDocValues for the given field.
func (p *PerFieldDocValuesProducer) GetSortedNumeric(field *index.FieldInfo) (SortedNumericDocValues, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("PerFieldDocValuesProducer is closed")
	}

	producer, err := p.getProducer(field)
	if err != nil {
		return nil, err
	}
	if producer == nil {
		return nil, nil
	}

	return producer.GetSortedNumeric(field)
}

// CheckIntegrity checks the integrity of the doc values.
func (p *PerFieldDocValuesProducer) CheckIntegrity() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return fmt.Errorf("PerFieldDocValuesProducer is closed")
	}

	for field, producer := range p.producers {
		if err := producer.CheckIntegrity(); err != nil {
			return fmt.Errorf("integrity check failed for field %s: %w", field, err)
		}
	}

	return nil
}

// Close closes all delegated producers.
func (p *PerFieldDocValuesProducer) Close() error {
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

// MapFieldDocValuesFormatProvider is a FieldDocValuesFormatProvider that
// maps specific fields to specific DocValuesFormats.
type MapFieldDocValuesFormatProvider struct {
	mu            sync.RWMutex
	fieldFormats  map[string]DocValuesFormat
	defaultFormat DocValuesFormat
}

// NewMapFieldDocValuesFormatProvider creates a new MapFieldDocValuesFormatProvider.
func NewMapFieldDocValuesFormatProvider(defaultFormat DocValuesFormat) *MapFieldDocValuesFormatProvider {
	return &MapFieldDocValuesFormatProvider{
		fieldFormats:  make(map[string]DocValuesFormat),
		defaultFormat: defaultFormat,
	}
}

// SetFormat sets the DocValuesFormat for a specific field.
func (p *MapFieldDocValuesFormatProvider) SetFormat(field string, format DocValuesFormat) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fieldFormats[field] = format
}

// GetDocValuesFormat returns the DocValuesFormat for the given field.
func (p *MapFieldDocValuesFormatProvider) GetDocValuesFormat(field string) DocValuesFormat {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if format, ok := p.fieldFormats[field]; ok {
		return format
	}
	return p.defaultFormat
}

// Ensure implementations satisfy the interfaces
var _ DocValuesFormat = (*PerFieldDocValuesFormat)(nil)
var _ DocValuesConsumer = (*PerFieldDocValuesConsumer)(nil)
var _ DocValuesProducer = (*PerFieldDocValuesProducer)(nil)
var _ FieldDocValuesFormatProvider = (*MapFieldDocValuesFormatProvider)(nil)
var _ FieldDocValuesFormatProvider = (FieldDocValuesFormatProviderFunc)(nil)
