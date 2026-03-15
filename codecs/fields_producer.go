// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BaseFieldsProducer provides a base implementation of FieldsProducer.
// This can be embedded in custom FieldsProducer implementations to get
// default implementations for common methods.
type BaseFieldsProducer struct {
	mu     sync.RWMutex
	closed bool
	fields map[string]index.Terms
}

// NewBaseFieldsProducer creates a new BaseFieldsProducer.
func NewBaseFieldsProducer() *BaseFieldsProducer {
	return &BaseFieldsProducer{
		fields: make(map[string]index.Terms),
	}
}

// Terms returns the terms for a field.
// This implements the FieldsProducer interface.
func (p *BaseFieldsProducer) Terms(field string) (index.Terms, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("FieldsProducer is closed")
	}

	terms, ok := p.fields[field]
	if !ok {
		return nil, nil
	}

	return terms, nil
}

// Close releases resources.
// This implements the FieldsProducer interface.
func (p *BaseFieldsProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	p.fields = nil
	return nil
}

// SetTerms sets the terms for a field.
// This is a helper method for subclasses.
func (p *BaseFieldsProducer) SetTerms(field string, terms index.Terms) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.fields[field] = terms
}

// IsClosed returns true if this producer has been closed.
func (p *BaseFieldsProducer) IsClosed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.closed
}

// FieldsProducerImpl is a concrete implementation of FieldsProducer
// that can be used directly or as a base for more complex implementations.
type FieldsProducerImpl struct {
	*BaseFieldsProducer
	state *SegmentReadState
}

// NewFieldsProducerImpl creates a new FieldsProducerImpl.
func NewFieldsProducerImpl(state *SegmentReadState) *FieldsProducerImpl {
	return &FieldsProducerImpl{
		BaseFieldsProducer: NewBaseFieldsProducer(),
		state:              state,
	}
}

// GetState returns the segment read state.
func (p *FieldsProducerImpl) GetState() *SegmentReadState {
	return p.state
}

// EmptyFieldsProducer is a FieldsProducer with no fields.
// This is useful for segments that have no postings.
type EmptyFieldsProducer struct {
	*BaseFieldsProducer
}

// NewEmptyFieldsProducer creates a new EmptyFieldsProducer.
func NewEmptyFieldsProducer() *EmptyFieldsProducer {
	return &EmptyFieldsProducer{
		BaseFieldsProducer: NewBaseFieldsProducer(),
	}
}

// Terms returns nil for all fields.
func (p *EmptyFieldsProducer) Terms(field string) (index.Terms, error) {
	return nil, nil
}

// Ensure implementations satisfy the interface
var _ FieldsProducer = (*BaseFieldsProducer)(nil)
var _ FieldsProducer = (*FieldsProducerImpl)(nil)
var _ FieldsProducer = (*EmptyFieldsProducer)(nil)
