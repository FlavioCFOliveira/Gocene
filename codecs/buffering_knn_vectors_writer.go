// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// BufferingKnnVectorsWriter is a KnnVectorsWriter that buffers every vector
// in memory and flushes the segment in one shot when Close is invoked. It is
// the Go port of org.apache.lucene.codecs.BufferingKnnVectorsWriter from
// Apache Lucene 10.4.0.
//
// The Java reference is an abstract class that wires the buffering surface
// (per-field KnnFieldVectorsWriter, addField, flush dispatch, ramBytesUsed
// tracking) and leaves the per-format encoding to subclasses via the
// abstract writeField method. Go has no abstract classes, so the contract
// is split into two pieces:
//   - BufferingKnnVectorsWriter (this struct) owns the buffering state.
//   - The Hook supplies the format-specific writeField/finish callbacks.
//
// Concrete codec writers compose this struct with their own Hook:
//
//	type myCodecKnnWriter struct {
//	    *codecs.BufferingKnnVectorsWriter
//	}
//
//	func NewMyCodecKnnWriter(state *codecs.SegmentWriteState) (codecs.KnnVectorsWriter, error) {
//	    hook := codecs.BufferingKnnVectorsHook{
//	        WriteFloatField: func(fi *schema.FieldInfo, w *codecs.BufferedFloatVectorField) error { ... },
//	        WriteByteField:  func(fi *schema.FieldInfo, w *codecs.BufferedByteVectorField) error { ... },
//	    }
//	    return &myCodecKnnWriter{BufferingKnnVectorsWriter: codecs.NewBufferingKnnVectorsWriter(state, hook)}, nil
//	}
type BufferingKnnVectorsWriter struct {
	state      *SegmentWriteState
	hook       BufferingKnnVectorsHook
	mu         sync.Mutex
	closed     bool
	floats     map[string]*BufferedFloatVectorField
	bytes      map[string]*BufferedByteVectorField
	fieldOrder []string // insertion order so flush dispatch is deterministic
}

// BufferingKnnVectorsHook supplies the codec-specific writeField callbacks
// invoked by BufferingKnnVectorsWriter.Close. Exactly one of WriteFloatField
// or WriteByteField is dispatched per field depending on the field's vector
// encoding.
type BufferingKnnVectorsHook struct {
	// WriteFloatField is called for fields whose VectorEncoding is FLOAT32.
	WriteFloatField func(fi *schema.FieldInfo, field *BufferedFloatVectorField) error

	// WriteByteField is called for fields whose VectorEncoding is BYTE.
	WriteByteField func(fi *schema.FieldInfo, field *BufferedByteVectorField) error

	// OnFinish is an optional hook invoked once after every field has been
	// flushed; codecs use it to write trailing metadata or footers.
	OnFinish func() error
}

// NewBufferingKnnVectorsWriter constructs a buffering writer wired to the
// supplied hook.
func NewBufferingKnnVectorsWriter(state *SegmentWriteState, hook BufferingKnnVectorsHook) *BufferingKnnVectorsWriter {
	return &BufferingKnnVectorsWriter{
		state:  state,
		hook:   hook,
		floats: make(map[string]*BufferedFloatVectorField),
		bytes:  make(map[string]*BufferedByteVectorField),
	}
}

// AddFloatField registers a new FLOAT32-encoded vector field and returns the
// TypedKnnFieldVectorsWriter consumers should call into.
func (w *BufferingKnnVectorsWriter) AddFloatField(fi *schema.FieldInfo) (TypedKnnFieldVectorsWriter[float32], error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, fmt.Errorf("BufferingKnnVectorsWriter: closed")
	}
	name := fi.Name()
	if _, dup := w.floats[name]; dup {
		return nil, fmt.Errorf("BufferingKnnVectorsWriter: field %q already added", name)
	}
	if _, dup := w.bytes[name]; dup {
		return nil, fmt.Errorf("BufferingKnnVectorsWriter: field %q already added as byte field", name)
	}
	bf := &BufferedFloatVectorField{
		FieldInfo: fi,
		Dimension: fi.VectorDimension(),
	}
	w.floats[name] = bf
	w.fieldOrder = append(w.fieldOrder, name)
	return bf, nil
}

// AddByteField registers a new BYTE-encoded vector field and returns the
// TypedKnnFieldVectorsWriter consumers should call into.
func (w *BufferingKnnVectorsWriter) AddByteField(fi *schema.FieldInfo) (TypedKnnFieldVectorsWriter[byte], error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, fmt.Errorf("BufferingKnnVectorsWriter: closed")
	}
	name := fi.Name()
	if _, dup := w.bytes[name]; dup {
		return nil, fmt.Errorf("BufferingKnnVectorsWriter: field %q already added", name)
	}
	if _, dup := w.floats[name]; dup {
		return nil, fmt.Errorf("BufferingKnnVectorsWriter: field %q already added as float field", name)
	}
	bf := &BufferedByteVectorField{
		FieldInfo: fi,
		Dimension: fi.VectorDimension(),
	}
	w.bytes[name] = bf
	w.fieldOrder = append(w.fieldOrder, name)
	return bf, nil
}

// WriteField is the KnnVectorsWriter interface entry-point. Buffering
// writers do not handle on-the-fly merges through this path; the codec-side
// merge orchestrator should construct a fresh BufferingKnnVectorsWriter for
// the merged segment and re-emit values through AddFloatField/AddByteField.
// This implementation returns an error to flag misuse.
func (w *BufferingKnnVectorsWriter) WriteField(fieldInfo *schema.FieldInfo, reader KnnVectorsReader) error {
	return fmt.Errorf("BufferingKnnVectorsWriter: WriteField is unsupported on buffering writers; use AddFloatField/AddByteField for the merged segment")
}

// AddField is unimplemented on BufferingKnnVectorsWriter: codec authors
// that wrap this helper expose their own AddField via the
// AddFloatField / AddByteField typed factories, which embed
// encoding-aware bookkeeping the wide non-generic surface cannot
// represent. Implementations that need wide-AddField semantics should
// dispatch from their own AddField to the typed factories.
func (w *BufferingKnnVectorsWriter) AddField(fieldInfo *schema.FieldInfo) (KnnFieldVectorsWriter, error) {
	return nil, fmt.Errorf("BufferingKnnVectorsWriter: AddField not supported; use AddFloatField or AddByteField on the concrete buffering writer")
}

// Flush is a no-op for BufferingKnnVectorsWriter: every per-field write
// happens in Close via the configured hook. The signature is preserved
// to satisfy [KnnVectorsWriter]; maxDoc and sortMap are accepted but
// ignored.
func (w *BufferingKnnVectorsWriter) Flush(maxDoc int, sortMap spi.SorterDocMap) error {
	_ = maxDoc
	_ = sortMap
	return nil
}

// Finish is a no-op for BufferingKnnVectorsWriter: actual flushing happens
// in Close, where the hook dispatches per-field writes.
func (w *BufferingKnnVectorsWriter) Finish() error {
	return nil
}

// RamBytesUsed returns the sum of per-field RAM usage estimates.
// Satisfies the [KnnVectorsWriter] (Accountable) contract. The legacy
// upper-case [BufferingKnnVectorsWriter.RAMBytesUsed] spelling forwards
// here for backwards-compatibility with pre-T4707 callers.
func (w *BufferingKnnVectorsWriter) RamBytesUsed() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	var total int64
	for _, f := range w.floats {
		total += f.RAMBytesUsed()
	}
	for _, f := range w.bytes {
		total += f.RAMBytesUsed()
	}
	return total
}

// RAMBytesUsed returns the sum of per-field RAM usage estimates.
//
// Deprecated: use [BufferingKnnVectorsWriter.RamBytesUsed] (the lower-
// case spelling matches the wide [KnnVectorsWriter] Accountable
// contract). Retained as a thin alias for the existing test surface.
func (w *BufferingKnnVectorsWriter) RAMBytesUsed() int64 {
	return w.RamBytesUsed()
}

// CheckIntegrity is a no-op for buffering writers since nothing is on disk
// yet. Included to satisfy the KnnVectorsWriter shape parity expected by
// some test harnesses.
func (w *BufferingKnnVectorsWriter) CheckIntegrity() error {
	return nil
}

// Close flushes every buffered field through the configured Hook in
// insertion order, then calls Hook.OnFinish if non-nil, then releases the
// buffer maps.
func (w *BufferingKnnVectorsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true

	for _, name := range w.fieldOrder {
		if bf, ok := w.floats[name]; ok {
			if w.hook.WriteFloatField == nil {
				return fmt.Errorf("BufferingKnnVectorsWriter: WriteFloatField hook not set for field %q", name)
			}
			if err := w.hook.WriteFloatField(bf.FieldInfo, bf); err != nil {
				return fmt.Errorf("BufferingKnnVectorsWriter: WriteFloatField(%q): %w", name, err)
			}
			continue
		}
		if bf, ok := w.bytes[name]; ok {
			if w.hook.WriteByteField == nil {
				return fmt.Errorf("BufferingKnnVectorsWriter: WriteByteField hook not set for field %q", name)
			}
			if err := w.hook.WriteByteField(bf.FieldInfo, bf); err != nil {
				return fmt.Errorf("BufferingKnnVectorsWriter: WriteByteField(%q): %w", name, err)
			}
		}
	}

	if w.hook.OnFinish != nil {
		if err := w.hook.OnFinish(); err != nil {
			return fmt.Errorf("BufferingKnnVectorsWriter: OnFinish: %w", err)
		}
	}

	w.floats = nil
	w.bytes = nil
	w.fieldOrder = nil
	return nil
}

// BufferedFloatVectorField holds the in-memory state of a single FLOAT32
// vector field; it satisfies TypedKnnFieldVectorsWriter[float32].
type BufferedFloatVectorField struct {
	FieldInfo *schema.FieldInfo
	Dimension int
	DocIDs    []int       // strictly increasing
	Vectors   [][]float32 // one per docID, exactly Dimension elements
}

// AddValue records a per-document vector.
func (b *BufferedFloatVectorField) AddValue(docID int, vectorValue []float32) error {
	if len(vectorValue) != b.Dimension {
		return fmt.Errorf("BufferedFloatVectorField: field %q expects dim %d, got %d", b.FieldInfo.Name(), b.Dimension, len(vectorValue))
	}
	if n := len(b.DocIDs); n > 0 && b.DocIDs[n-1] >= docID {
		return fmt.Errorf("BufferedFloatVectorField: field %q out-of-order docID %d (last %d)", b.FieldInfo.Name(), docID, b.DocIDs[n-1])
	}
	cp := make([]float32, len(vectorValue))
	copy(cp, vectorValue)
	b.DocIDs = append(b.DocIDs, docID)
	b.Vectors = append(b.Vectors, cp)
	return nil
}

// RAMBytesUsed estimates the field's in-memory footprint.
func (b *BufferedFloatVectorField) RAMBytesUsed() int64 {
	// 4 bytes per docID + 4 bytes per float32 element + slice headers.
	const docIDBytes = 4
	const floatBytes = 4
	return int64(len(b.DocIDs))*docIDBytes + int64(len(b.Vectors))*int64(b.Dimension)*floatBytes
}

// Finish is a no-op: no per-field finalization needed.
func (b *BufferedFloatVectorField) Finish() error { return nil }

// BufferedByteVectorField holds the in-memory state of a single BYTE
// vector field; it satisfies TypedKnnFieldVectorsWriter[byte].
type BufferedByteVectorField struct {
	FieldInfo *schema.FieldInfo
	Dimension int
	DocIDs    []int
	Vectors   [][]byte
}

// AddValue records a per-document byte vector.
func (b *BufferedByteVectorField) AddValue(docID int, vectorValue []byte) error {
	if len(vectorValue) != b.Dimension {
		return fmt.Errorf("BufferedByteVectorField: field %q expects dim %d, got %d", b.FieldInfo.Name(), b.Dimension, len(vectorValue))
	}
	if n := len(b.DocIDs); n > 0 && b.DocIDs[n-1] >= docID {
		return fmt.Errorf("BufferedByteVectorField: field %q out-of-order docID %d (last %d)", b.FieldInfo.Name(), docID, b.DocIDs[n-1])
	}
	cp := make([]byte, len(vectorValue))
	copy(cp, vectorValue)
	b.DocIDs = append(b.DocIDs, docID)
	b.Vectors = append(b.Vectors, cp)
	return nil
}

// RAMBytesUsed estimates the field's in-memory footprint.
func (b *BufferedByteVectorField) RAMBytesUsed() int64 {
	const docIDBytes = 4
	return int64(len(b.DocIDs))*docIDBytes + int64(len(b.Vectors))*int64(b.Dimension)
}

// Finish is a no-op: no per-field finalization needed.
func (b *BufferedByteVectorField) Finish() error { return nil }

// Ensure BufferingKnnVectorsWriter satisfies KnnVectorsWriter.
var _ KnnVectorsWriter = (*BufferingKnnVectorsWriter)(nil)
