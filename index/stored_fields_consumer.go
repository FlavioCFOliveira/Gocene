// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// StoredValueType discriminates the variant carried by a StoredValue.
//
// It mirrors org.apache.lucene.document.StoredValue.Type from Apache
// Lucene 10.4.0, restricted to the variants StoredFieldsConsumer can
// dispatch. The DATA_INPUT variant is intentionally absent: the
// StoredFieldDataInput type it wraps is not yet ported (see the
// divergence note on StoredFieldsConsumer).
type StoredValueType int

const (
	// StoredValueTypeInteger is a 32-bit signed integer.
	StoredValueTypeInteger StoredValueType = iota

	// StoredValueTypeLong is a 64-bit signed integer.
	StoredValueTypeLong

	// StoredValueTypeFloat is a 32-bit floating-point value.
	StoredValueTypeFloat

	// StoredValueTypeDouble is a 64-bit floating-point value.
	StoredValueTypeDouble

	// StoredValueTypeBinary is a raw byte sequence (BytesRef in Java).
	StoredValueTypeBinary

	// StoredValueTypeString is a UTF-8 string.
	StoredValueTypeString
)

// String renders the variant name for diagnostics.
func (t StoredValueType) String() string {
	switch t {
	case StoredValueTypeInteger:
		return "INTEGER"
	case StoredValueTypeLong:
		return "LONG"
	case StoredValueTypeFloat:
		return "FLOAT"
	case StoredValueTypeDouble:
		return "DOUBLE"
	case StoredValueTypeBinary:
		return "BINARY"
	case StoredValueTypeString:
		return "STRING"
	default:
		return fmt.Sprintf("StoredValueType(%d)", int(t))
	}
}

// StoredValue is the minimal contract StoredFieldsConsumer needs to
// dispatch one stored field to the codec writer. It is the index-package
// view of a stored value: a discriminated union exposing the single typed
// payload selected by Type.
//
// It deliberately does not depend on document.StoredValue. Lucene's
// StoredFieldsConsumer consumes org.apache.lucene.document.StoredValue
// directly, but wiring that concrete type through here would couple the
// index package to document for one switch statement. The local interface
// keeps the dependency direction clean while preserving the observable
// contract: callers (DocumentsWriterPerThread / the indexing chain) supply
// a value that satisfies this interface. Any concrete StoredValue type --
// including a future adapter over document.StoredValue -- implements it.
//
// Only the accessor matching Type is required to return a meaningful
// value; the others may return their zero value, exactly as Lucene's
// per-variant getters assert before returning.
type StoredValue interface {
	// Type reports which variant this value carries.
	Type() StoredValueType

	// IntValue returns the payload for the INTEGER variant.
	IntValue() int32

	// LongValue returns the payload for the LONG variant.
	LongValue() int64

	// FloatValue returns the payload for the FLOAT variant.
	FloatValue() float32

	// DoubleValue returns the payload for the DOUBLE variant.
	DoubleValue() float64

	// BinaryValue returns the payload for the BINARY variant.
	BinaryValue() []byte

	// StringValue returns the payload for the STRING variant.
	StringValue() string
}

// StoredFieldsConsumer is the Go port of Apache Lucene 10.4.0's
// org.apache.lucene.index.StoredFieldsConsumer.
//
// It mediates between DocumentsWriterPerThread and the codec
// StoredFieldsWriter: it lazily opens the writer on the first document,
// emits empty documents to keep the writer's document counter aligned
// with the indexer's docID stream, dispatches each stored field by its
// StoredValue variant, and flushes or aborts the writer at segment end.
//
// Lucene's class is package-private and subclassed only by
// SortingStoredFieldsConsumer. Gocene exports the type and its methods
// so the existing SortingStoredFieldsConsumer (and future consumers)
// can embed it instead of carrying the placeholder
// storedFieldsConsumerBase struct introduced in Sprint 55. The exported
// surface is the minimal translation of Lucene's package-private
// methods; no extra behaviour is added.
//
// Divergences from Lucene 10.4.0, all confined to internal plumbing:
//
//   - WriteField takes the index-local StoredValue interface rather than
//     org.apache.lucene.document.StoredValue, to avoid coupling the index
//     package to document for one type switch (see the StoredValue doc).
//
//   - WriteField bridges StoredValue to the index-package IndexableField
//     contract through an unexported storedValueField adapter, because
//     Gocene's StoredFieldsWriter.WriteField consumes IndexableField
//     rather than the per-type overloads Lucene's StoredFieldsWriter
//     exposes. The adapter is stored-only and carries exactly the one
//     typed value the StoredValue holds.
//
//   - The DATA_INPUT StoredValue variant has no representation here: the
//     StoredFieldDataInput type it wraps is not yet ported. Lucene routes
//     it to StoredFieldsWriter.writeField(FieldInfo, StoredFieldDataInput).
//     StoredValueType therefore omits the variant entirely.
//
//   - maxDoc is read via SegmentInfo.DocCount; SegmentInfo exposes no
//     separate maxDoc() accessor in Gocene. This matches the precedent
//     set by SortingStoredFieldsConsumer.Flush.
type StoredFieldsConsumer struct {
	codec     Codec
	directory store.Directory
	info      *SegmentInfo

	// writer is the codec StoredFieldsWriter. It is nil until the first
	// call to InitStoredFieldsWriter (mirrors Lucene's lazy allocation).
	writer StoredFieldsWriter

	// lastDoc tracks the highest docID for which a document has been
	// started. It begins at -1 so the first started document is docID 0.
	lastDoc int
}

// NewStoredFieldsConsumer constructs the consumer for one segment.
// It mirrors Lucene's StoredFieldsConsumer(Codec, Directory, SegmentInfo).
func NewStoredFieldsConsumer(codec Codec, directory store.Directory, info *SegmentInfo) *StoredFieldsConsumer {
	return &StoredFieldsConsumer{
		codec:     codec,
		directory: directory,
		info:      info,
		lastDoc:   -1,
	}
}

// Writer returns the active codec StoredFieldsWriter, or nil if no
// document has been started yet. Lucene's field is package-private;
// it is exposed here for test inspection and for SortingStoredFieldsConsumer.
func (c *StoredFieldsConsumer) Writer() StoredFieldsWriter { return c.writer }

// InitStoredFieldsWriter lazily creates the codec StoredFieldsWriter the
// first time it is called. Subsequent calls are no-ops.
//
// Mirrors the protected initStoredFieldsWriter() in Lucene. Lucene's
// TODO about allocating in the constructor is preserved by keeping
// the lazy behaviour rather than moving it into NewStoredFieldsConsumer.
func (c *StoredFieldsConsumer) InitStoredFieldsWriter() error {
	if c.writer != nil {
		return nil
	}
	w, err := c.codec.StoredFieldsFormat().FieldsWriter(c.directory, c.info, store.IOContextDefault)
	if err != nil {
		return fmt.Errorf("index: StoredFieldsConsumer init writer: %w", err)
	}
	c.writer = w
	return nil
}

// StartDocument prepares the writer for the document with the given
// docID. If docID skips ahead of the last started document, empty
// documents are emitted to fill the gap so the writer's internal
// document counter stays aligned with the indexer's docID stream.
//
// Mirrors org.apache.lucene.index.StoredFieldsConsumer.startDocument.
// Lucene asserts lastDoc < docID; the port returns an error instead,
// since Go has no assertion mechanism and a backwards docID is a
// caller bug worth surfacing.
func (c *StoredFieldsConsumer) StartDocument(docID int) error {
	if c.lastDoc >= docID {
		return fmt.Errorf("index: StoredFieldsConsumer.StartDocument: docID %d is not greater than last started doc %d", docID, c.lastDoc)
	}
	if err := c.InitStoredFieldsWriter(); err != nil {
		return err
	}
	for {
		c.lastDoc++
		if c.lastDoc >= docID {
			break
		}
		if err := c.writer.StartDocument(); err != nil {
			return fmt.Errorf("index: StoredFieldsConsumer.StartDocument: start filler doc %d: %w", c.lastDoc, err)
		}
		if err := c.writer.FinishDocument(); err != nil {
			return fmt.Errorf("index: StoredFieldsConsumer.StartDocument: finish filler doc %d: %w", c.lastDoc, err)
		}
	}
	if err := c.writer.StartDocument(); err != nil {
		return fmt.Errorf("index: StoredFieldsConsumer.StartDocument: start doc %d: %w", docID, err)
	}
	return nil
}

// WriteField dispatches one stored field to the codec writer, selecting
// the writer call by the StoredValue's variant.
//
// Mirrors org.apache.lucene.index.StoredFieldsConsumer.writeField. Lucene
// throws AssertionError on an unknown variant; the port returns an error.
// There is no DATA_INPUT variant: StoredFieldDataInput is not yet ported
// (see the type-level divergence note).
func (c *StoredFieldsConsumer) WriteField(fi *FieldInfo, value StoredValue) error {
	if value == nil {
		return errors.New("index: StoredFieldsConsumer.WriteField: value is nil")
	}
	field, err := newStoredValueField(fi, value)
	if err != nil {
		return err
	}
	if err := c.writer.WriteField(field); err != nil {
		return fmt.Errorf("index: StoredFieldsConsumer.WriteField: %w", err)
	}
	return nil
}

// FinishDocument finalizes the current document on the writer.
//
// Mirrors org.apache.lucene.index.StoredFieldsConsumer.finishDocument.
func (c *StoredFieldsConsumer) FinishDocument() error {
	if err := c.writer.FinishDocument(); err != nil {
		return fmt.Errorf("index: StoredFieldsConsumer.FinishDocument: %w", err)
	}
	return nil
}

// Finish emits empty documents until the writer has seen maxDoc
// documents, covering trailing docIDs that carried no stored fields.
//
// Mirrors org.apache.lucene.index.StoredFieldsConsumer.finish.
func (c *StoredFieldsConsumer) Finish(maxDoc int) error {
	for c.lastDoc < maxDoc-1 {
		if err := c.StartDocument(c.lastDoc + 1); err != nil {
			return err
		}
		if err := c.FinishDocument(); err != nil {
			return err
		}
	}
	return nil
}

// Flush finalizes the codec writer for the segment and then closes it.
// The writer is closed even if Finish fails, mirroring Lucene's
// try/finally with IOUtils.close.
//
// Mirrors org.apache.lucene.index.StoredFieldsConsumer.flush. The
// sortMap parameter is part of the contract subclasses override (it is
// consumed by SortingStoredFieldsConsumer); the base implementation
// ignores it, exactly as Lucene's base flush does.
//
// Lucene's base flush calls writer.finish(state.segmentInfo.maxDoc())
// unconditionally; if no document was ever started, writer is nil here
// and the call would panic. Lucene never hits that path because
// DocumentsWriterPerThread always drives at least Finish(maxDoc) first.
// The port guards it explicitly so a nil writer is a no-op rather than
// a panic.
func (c *StoredFieldsConsumer) Flush(state *SegmentWriteState, sortMap SorterDocMap) error {
	if state == nil || state.SegmentInfo == nil {
		return errors.New("index: StoredFieldsConsumer.Flush requires a non-nil state with SegmentInfo")
	}
	_ = sortMap // base implementation ignores sortMap; see method doc.
	if c.writer == nil {
		return nil
	}
	finishErr := c.writer.Finish(state.SegmentInfo.DocCount())
	closeErr := c.writer.Close()
	if finishErr != nil {
		return fmt.Errorf("index: StoredFieldsConsumer.Flush: %w", finishErr)
	}
	if closeErr != nil {
		return fmt.Errorf("index: StoredFieldsConsumer.Flush close: %w", closeErr)
	}
	return nil
}

// Abort closes the codec writer, discarding any error.
//
// Mirrors org.apache.lucene.index.StoredFieldsConsumer.abort, which uses
// IOUtils.closeWhileHandlingException.
func (c *StoredFieldsConsumer) Abort() {
	if c.writer != nil {
		_ = c.writer.Close() // best-effort close on the abort path.
	}
}

// storedValueField adapts a StoredValue to the index-package
// IndexableField contract so it can be handed to StoredFieldsWriter.
// WriteField. It is stored-only and carries exactly the single typed
// value the StoredValue holds.
//
// Gocene's StoredFieldsWriter.WriteField takes one IndexableField, while
// Lucene's StoredFieldsWriter exposes a per-type writeField overload;
// this adapter bridges that gap inside StoredFieldsConsumer.WriteField.
type storedValueField struct {
	name    string
	variant StoredValueType
	str     string
	bin     []byte
	i32     int32
	i64     int64
	f32     float32
	f64     float64
}

// newStoredValueField builds the adapter for one stored field, copying
// the typed value out of the StoredValue. It rejects an unknown variant,
// mirroring the AssertionError default in Lucene's writeField.
func newStoredValueField(info *FieldInfo, value StoredValue) (*storedValueField, error) {
	if info == nil {
		return nil, errors.New("index: StoredFieldsConsumer.WriteField: FieldInfo is nil")
	}
	f := &storedValueField{name: info.Name(), variant: value.Type()}
	switch value.Type() {
	case StoredValueTypeInteger:
		f.i32 = value.IntValue()
	case StoredValueTypeLong:
		f.i64 = value.LongValue()
	case StoredValueTypeFloat:
		f.f32 = value.FloatValue()
	case StoredValueTypeDouble:
		f.f64 = value.DoubleValue()
	case StoredValueTypeBinary:
		f.bin = value.BinaryValue()
	case StoredValueTypeString:
		f.str = value.StringValue()
	default:
		// Handle DATA_INPUT variant via type assertion: if the value provides
		// a streamed DataInput (StoredFieldDataInput), materialise the bytes as binary.
		type dataInputProvider interface {
			GetDataInputValue() *StoredFieldDataInput
		}
		if dip, ok := value.(dataInputProvider); ok {
			dsi := dip.GetDataInputValue()
			if dsi == nil || dsi.In == nil {
				return nil, fmt.Errorf("index: StoredFieldsConsumer.WriteField: StoredFieldDataInput has nil In")
			}
			buf := make([]byte, dsi.Length)
			if err := dsi.In.ReadBytes(buf); err != nil {
				return nil, fmt.Errorf("index: StoredFieldsConsumer.WriteField: read DataInput bytes: %w", err)
			}
			f.bin = buf
			f.variant = StoredValueTypeBinary
			return f, nil
		}
		return nil, fmt.Errorf("index: StoredFieldsConsumer.WriteField: unknown StoredValue type %s", value.Type())
	}
	return f, nil
}

// Name implements IndexableField.
func (f *storedValueField) Name() string { return f.name }

// FieldType implements IndexableField with a stored-only marker type.
func (f *storedValueField) FieldType() FieldTypeInterface { return storedValueFieldType{} }

// StringValue implements IndexableField. Returns the payload only for the
// STRING variant; "" otherwise.
func (f *storedValueField) StringValue() string {
	if f.variant == StoredValueTypeString {
		return f.str
	}
	return ""
}

// BinaryValue implements IndexableField. Returns the payload only for the
// BINARY variant; nil otherwise.
func (f *storedValueField) BinaryValue() []byte {
	if f.variant == StoredValueTypeBinary {
		return f.bin
	}
	return nil
}

// NumericValue implements IndexableField. Returns the concrete numeric
// type matching the StoredValue variant; nil for non-numeric variants.
func (f *storedValueField) NumericValue() interface{} {
	switch f.variant {
	case StoredValueTypeInteger:
		return f.i32
	case StoredValueTypeLong:
		return f.i64
	case StoredValueTypeFloat:
		return f.f32
	case StoredValueTypeDouble:
		return f.f64
	default:
		return nil
	}
}

// storedValueFieldType marks the adapted field as stored-only. Every
// indexing property is false: the adapter only ever feeds a stored-fields
// writer.
type storedValueFieldType struct{}

func (storedValueFieldType) IsIndexed() bool                 { return false }
func (storedValueFieldType) IsStored() bool                  { return true }
func (storedValueFieldType) IsTokenized() bool               { return false }
func (storedValueFieldType) GetIndexOptions() IndexOptions   { return IndexOptionsNone }
func (storedValueFieldType) GetDocValuesType() DocValuesType { return DocValuesTypeNone }
func (storedValueFieldType) StoreTermVectors() bool          { return false }
func (storedValueFieldType) StoreTermVectorPositions() bool  { return false }
func (storedValueFieldType) StoreTermVectorOffsets() bool    { return false }

// Compile-time assertion that the adapter satisfies IndexableField.
var _ IndexableField = (*storedValueField)(nil)
