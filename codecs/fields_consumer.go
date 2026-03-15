// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BaseFieldsConsumer provides a base implementation of FieldsConsumer.
// This can be embedded in custom FieldsConsumer implementations to get
// default implementations for common methods.
type BaseFieldsConsumer struct {
	mu     sync.Mutex
	closed bool
	state  *SegmentWriteState
	fields map[string]index.Terms
}

// NewBaseFieldsConsumer creates a new BaseFieldsConsumer.
func NewBaseFieldsConsumer(state *SegmentWriteState) *BaseFieldsConsumer {
	return &BaseFieldsConsumer{
		state:  state,
		fields: make(map[string]index.Terms),
	}
}

// Write writes a field's postings.
// This implements the FieldsConsumer interface.
func (c *BaseFieldsConsumer) Write(field string, terms index.Terms) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("FieldsConsumer is closed")
	}

	c.fields[field] = terms
	return nil
}

// Close releases resources.
// This implements the FieldsConsumer interface.
func (c *BaseFieldsConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.fields = nil
	return nil
}

// IsClosed returns true if this consumer has been closed.
func (c *BaseFieldsConsumer) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// GetState returns the segment write state.
func (c *BaseFieldsConsumer) GetState() *SegmentWriteState {
	return c.state
}

// GetFields returns the fields map (for subclasses).
func (c *BaseFieldsConsumer) GetFields() map[string]index.Terms {
	return c.fields
}

// FieldsConsumerImpl is a concrete implementation of FieldsConsumer
// that stores fields in memory and writes them on close.
type FieldsConsumerImpl struct {
	*BaseFieldsConsumer
	writer FieldWriter
}

// FieldWriter is called to write field data during close.
type FieldWriter interface {
	WriteField(field string, terms index.Terms) error
}

// NewFieldsConsumerImpl creates a new FieldsConsumerImpl.
func NewFieldsConsumerImpl(state *SegmentWriteState, writer FieldWriter) *FieldsConsumerImpl {
	return &FieldsConsumerImpl{
		BaseFieldsConsumer: NewBaseFieldsConsumer(state),
		writer:             writer,
	}
}

// Close writes all fields and releases resources.
func (c *FieldsConsumerImpl) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	// Write all fields
	if c.writer != nil {
		for field, terms := range c.fields {
			if err := c.writer.WriteField(field, terms); err != nil {
				return fmt.Errorf("failed to write field %s: %w", field, err)
			}
		}
	}

	c.closed = true
	c.fields = nil
	return nil
}

// NoOpFieldsConsumer is a FieldsConsumer that does nothing.
// This is useful for testing or when postings are not needed.
type NoOpFieldsConsumer struct {
	*BaseFieldsConsumer
}

// NewNoOpFieldsConsumer creates a new NoOpFieldsConsumer.
func NewNoOpFieldsConsumer(state *SegmentWriteState) *NoOpFieldsConsumer {
	return &NoOpFieldsConsumer{
		BaseFieldsConsumer: NewBaseFieldsConsumer(state),
	}
}

// Write does nothing.
func (c *NoOpFieldsConsumer) Write(field string, terms index.Terms) error {
	return nil
}

// Close does nothing.
func (c *NoOpFieldsConsumer) Close() error {
	return nil
}

// Ensure implementations satisfy the interface
var _ FieldsConsumer = (*BaseFieldsConsumer)(nil)
var _ FieldsConsumer = (*FieldsConsumerImpl)(nil)
var _ FieldsConsumer = (*NoOpFieldsConsumer)(nil)
