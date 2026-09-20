// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// BaseFieldsConsumer provides a base implementation of FieldsConsumer.
// This can be embedded in custom FieldsConsumer implementations to get
// default implementations for common methods.
type BaseFieldsConsumer struct {
	mu     sync.Mutex
	closed bool
	state  *SegmentWriteState
	fields map[string]spi.Terms
}

// NewBaseFieldsConsumer creates a new BaseFieldsConsumer.
func NewBaseFieldsConsumer(state *SegmentWriteState) *BaseFieldsConsumer {
	return &BaseFieldsConsumer{
		state:  state,
		fields: make(map[string]spi.Terms),
	}
}

// Write buffers the postings of every field the given Fields exposes.
// This implements the FieldsConsumer interface.
func (c *BaseFieldsConsumer) Write(fields spi.Fields, norms spi.NormsProducer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("FieldsConsumer is closed")
	}
	if fields == nil {
		return nil
	}

	it, err := fields.Iterator()
	if err != nil {
		return err
	}
	for {
		field, err := it.Next()
		if err != nil {
			return err
		}
		if field == "" {
			return nil
		}
		terms, err := fields.Terms(field)
		if err != nil {
			return err
		}
		if terms == nil {
			continue
		}
		c.fields[field] = terms
	}
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
func (c *BaseFieldsConsumer) GetFields() map[string]spi.Terms {
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
	WriteField(field string, terms spi.Terms) error
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
func (c *NoOpFieldsConsumer) Write(fields spi.Fields, norms spi.NormsProducer) error {
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

// FieldsConsumerBase carries the one concrete member of the abstract class
// org.apache.lucene.codecs.FieldsConsumer of Apache Lucene 10.5.0: merge. The
// abstract members (write and close) are the methods of [FieldsConsumer].
//
// A consumer that inherits Java's merge embeds a FieldsConsumerBase built with
// [NewFieldsConsumerBase], passing itself as impl: impl is the receiver on
// which Merge invokes the abstract Write, which Java dispatches through this.
// A subclass that overrides merge declares its own Merge method, which shadows
// the promoted one; inside it, calling the embedded FieldsConsumerBase.Merge
// is Java's super.merge(...). A subclass whose own Write overrides its
// parent's re-points impl at itself in its constructor, exactly as the
// Overrides back-pointers of codecs/uniformsplit do.
//
// NAMING NOTE: the module spells such a carrier Base<JavaClass>
// ([BaseDocValuesConsumer], [BaseNormsConsumer]). BaseFieldsConsumer is taken
// in this package by the in-memory buffering helper above, which has no
// counterpart in Lucene 10.5.0, so the carrier is spelled FieldsConsumerBase.
// The difference is one of spelling only; nothing observable changes.
type FieldsConsumerBase struct {
	impl FieldsConsumer
}

// NewFieldsConsumerBase returns the base of the consumer impl. Mirrors the
// protected constructor FieldsConsumer() (FieldsConsumer.java:38).
func NewFieldsConsumerBase(impl FieldsConsumer) *FieldsConsumerBase {
	return &FieldsConsumerBase{impl: impl}
}

// SetImpl re-points the back-pointer at the most-derived instance. It renders
// the fact that Java's `this` inside FieldsConsumer.merge is the subclass
// being constructed, which Go cannot express while the base constructor runs.
func (b *FieldsConsumerBase) SetImpl(impl FieldsConsumer) {
	b.impl = impl
}

// Merge merges in the fields from the readers in mergeState. The default
// implementation skips and maps around deleted documents, and calls
// Write(Fields, NormsProducer). Implementations can override this method for
// more sophisticated merging (bulk-byte copying, etc).
//
// Mirrors org.apache.lucene.codecs.FieldsConsumer#merge(MergeState,
// NormsProducer) (FieldsConsumer.java:72-96).
func (b *FieldsConsumerBase) Merge(mergeState *index.MergeState, norms NormsProducer) error {
	fields := make([]index.Fields, 0, len(mergeState.FieldsProducers))
	slices := make([]index.ReaderSlice, 0, len(mergeState.FieldsProducers))

	docBase := 0

	for readerIndex := 0; readerIndex < len(mergeState.FieldsProducers); readerIndex++ {
		f := mergeState.FieldsProducers[readerIndex]

		maxDoc := mergeState.MaxDocs[readerIndex]
		if f != nil {
			if err := mergeState.CheckAborted(); err != nil {
				return err
			}
			if err := f.CheckIntegrity(); err != nil {
				return err
			}
			slices = append(slices, index.NewReaderSlice(docBase, maxDoc, readerIndex))
			fields = append(fields, f)
		}
		docBase += maxDoc
	}

	mergedFields := index.NewMappedMultiFields(mergeState, index.NewMultiFields(fields, slices))
	return b.impl.Write(mergedFields, norms)
}
