// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermVectorsConsumer is the indexing-time consumer that materialises
// term vectors for the current segment. It is the parent of
// SortingTermVectorsConsumer.
//
// This is the Go port of Apache Lucene 10.4.0's
// org.apache.lucene.index.TermVectorsConsumer (181 lines).
//
// Sprint 55 / GOC-3378 deviations (all internal, none observable on the
// public surface):
//
//   - The Lucene parent extends the abstract TermsHash. That parent type
//     is not yet ported in Gocene; the IntBlockPool / ByteBlockPool /
//     Counter / nextTermsHash fields it owns are inlined here as
//     unexported fields. When TermsHash lands, TermVectorsConsumer will
//     embed *TermsHash and these fields move to the embedded receiver
//     (mechanical migration).
//
//   - Lucene's per-field array is typed TermVectorsConsumerPerField[].
//     That concrete subtype is also not yet ported; the array here is
//     typed []TermVectorsPerFieldHandle, a small interface that captures
//     the only two operations the parent invokes on each element
//     (CompareName for the introSort, FinishDocument for the flush loop).
//     The future TermVectorsConsumerPerField struct satisfies the
//     interface by exposing the same two methods.
//
//   - Lucene's TermVectorsFormat.vectorsWriter takes (Directory,
//     SegmentInfo, IOContext). The Gocene codec interface takes only a
//     *SegmentWriteState; we synthesise a state from the consumer's own
//     directory / SegmentInfo. SegmentWriteState in Gocene has no
//     Context field today, so the FlushInfo computed from lastDocID and
//     bytesUsed is dropped on the floor; once SegmentWriteState gains a
//     Context (tracked alongside the broader codec wiring) the value
//     synthesised here will be attached.
//
//   - Lucene's writer.finish(numDocs) call after the fill loop has no
//     counterpart in the Gocene TermVectorsWriter interface (Close is
//     the only sink). The port omits the finish call and relies on
//     Close to perform whatever finalisation the implementation needs;
//     the gap is preserved as a comment at the call site so a future
//     interface expansion can wire it in without a structural change.
//
//   - Accountable on the writer is approximated: Lucene exposes the
//     active TermVectorsWriter as accountable directly. The Gocene
//     TermVectorsWriter interface does not yet extend Accountable, so the
//     consumer reports zero bytes when no writer is installed and
//     defers to the writer's RamBytesUsed only when it satisfies
//     util.Accountable.
//
//   - Lucene's abort() catches every Throwable when closing the writer;
//     the Go port swallows the error from Close (Lucene calls
//     IOUtils.closeWhileHandlingException) and clears the writer ref so
//     a subsequent reset starts from a clean slate.
//
//   - reset() / startDocument() inline a no-op nextTermsHash hook (the
//     chain is empty in the current port; see the TermsHash deviation
//     above).

// TermVectorsPerFieldHandle is the narrow surface the parent
// TermVectorsConsumer needs to manipulate per-field state during the
// document flush path. The concrete TermVectorsConsumerPerField subtype
// (to be ported separately) will satisfy this interface; tests can
// provide a stub.
//
// CompareName returns -1, 0 or +1 following strings.Compare semantics on
// the field name (Lucene sorts term vector fields by UTF-16 of the field
// name; Go strings are UTF-8 and ascending-byte order coincides with
// UTF-16 ascending order for the BMP range Lucene field names use in
// practice).
//
// FinishDocument serialises the per-field portion of the current
// document into the active TermVectorsWriter. It is invoked once per
// flushed field, in sorted order.
type TermVectorsPerFieldHandle interface {
	CompareName(other TermVectorsPerFieldHandle) int
	FinishDocument() error
}

// TermVectorsConsumer materialises term vectors for the current segment.
//
// Mirrors org.apache.lucene.index.TermVectorsConsumer (package-private
// in Lucene; exported here because the port's package surface is
// flatter than Lucene's).
type TermVectorsConsumer struct {
	// Directory hosts the segment's term-vector files.
	Directory store.Directory
	// Info describes the segment being flushed.
	Info *SegmentInfo
	// Codec supplies the active TermVectorsFormat.
	Codec Codec
	// Writer is the active TermVectorsWriter; nil until
	// InitTermVectorsWriter creates it.
	Writer TermVectorsWriter

	// FlushTerm is the scratch BytesRef the per-field handle uses when
	// emitting terms during FinishDocument. Mirrors Lucene's package-
	// private flushTerm field.
	FlushTerm *util.BytesRef
	// VectorSliceReaderPos / VectorSliceReaderOff are scratch readers
	// the per-field handle uses to walk position and offset slices.
	// Mirrors Lucene's vectorSliceReaderPos / vectorSliceReaderOff.
	VectorSliceReaderPos *ByteSliceReader
	VectorSliceReaderOff *ByteSliceReader

	// intPool / bytePool / bytesUsed are owned by the (not-yet-ported)
	// TermsHash parent in Lucene. They live here until that port lands;
	// see the type-doc deviation note.
	intPool   *util.IntBlockPool
	bytePool  *util.ByteBlockPool
	bytesUsed *util.Counter

	// hasVectors becomes true the first time SetHasVectors is called for
	// the current segment. It gates flush() and finishDocument().
	hasVectors bool
	// numVectorFields counts the entries currently populated in
	// perFields for the active document.
	numVectorFields int
	// LastDocID is the count of documents the active writer has
	// observed so far. Exposed because the per-field handle reads it
	// during finishDocument.
	LastDocID int
	// perFields stores the per-field handles registered for the current
	// document. Mirrors Lucene's perFields array; grown via oversize.
	perFields []TermVectorsPerFieldHandle
	// accountable holds the active writer when it implements
	// util.Accountable, otherwise nil. Mirrors Lucene's accountable
	// field with NULL_ACCOUNTABLE replaced by a nil check at the read
	// site (RamBytesUsed).
	accountable util.Accountable
}

// NewTermVectorsConsumer constructs the consumer for the given segment.
// Mirrors Lucene's constructor TermVectorsConsumer(IntBlockPool.Allocator,
// ByteBlockPool.Allocator, Directory, SegmentInfo, Codec); the per-block
// allocators are owned by the not-yet-ported TermsHash parent and so
// are absent from the Sprint 55 signature.
//
// Returns nil when info is nil: Lucene relies on a non-null SegmentInfo
// for every observable code path the consumer takes.
func NewTermVectorsConsumer(directory store.Directory, info *SegmentInfo, codec Codec) *TermVectorsConsumer {
	if info == nil {
		return nil
	}
	return &TermVectorsConsumer{
		Directory:            directory,
		Info:                 info,
		Codec:                codec,
		FlushTerm:            &util.BytesRef{},
		VectorSliceReaderPos: &ByteSliceReader{},
		VectorSliceReaderOff: &ByteSliceReader{},
		intPool:              util.NewIntBlockPool(),
		bytePool:             util.NewByteBlockPool(util.NewDirectAllocator()),
		bytesUsed:            util.NewCounter(),
		perFields:            make([]TermVectorsPerFieldHandle, 1),
	}
}

// Flush completes the current segment's term vectors. It mirrors the
// package-private flush(Map<String,TermsHashPerField>, SegmentWriteState,
// Sorter.DocMap, NormsProducer) in Lucene.
//
// The first two Lucene parameters (fieldsToFlush, norms) are not used
// by the parent body (only the unported nextTermsHash chain consumes
// them); they are omitted from the port signature until that chain
// arrives. sortMap is accepted for future symmetry with the sorting
// subclass but is currently ignored by the parent path, exactly as
// Lucene does (the parent flush body does not touch sortMap).
func (c *TermVectorsConsumer) Flush(state *SegmentWriteState, sortMap SorterDocMap) error {
	_ = sortMap // mirror Lucene: the parent flush body does not use sortMap
	if c == nil || c.Writer == nil {
		return nil
	}
	if state == nil || state.SegmentInfo == nil {
		return errors.New("index: TermVectorsConsumer.Flush requires a non-nil state with SegmentInfo")
	}
	numDocs := state.SegmentInfo.DocCount()
	if numDocs <= 0 {
		return fmt.Errorf("index: TermVectorsConsumer.Flush expected numDocs > 0, got %d", numDocs)
	}
	fillErr := c.Fill(numDocs)
	// Lucene calls writer.finish(numDocs) here; the Gocene
	// TermVectorsWriter interface does not expose a Finish method, so
	// finalisation is deferred to Close. See the type-doc deviation.
	closeErr := c.Writer.Close()
	c.Writer = nil
	c.accountable = nil
	switch {
	case fillErr != nil:
		return fmt.Errorf("index: TermVectorsConsumer flush fill: %w", fillErr)
	case closeErr != nil:
		return fmt.Errorf("index: TermVectorsConsumer flush close: %w", closeErr)
	}
	return nil
}

// Fill emits empty term-vector documents up to (but not including)
// docID. Mirrors Lucene's package-private fill(int).
//
// Returns an error wrapping the underlying writer error if any
// StartDocument/FinishDocument call fails.
func (c *TermVectorsConsumer) Fill(docID int) error {
	if c == nil || c.Writer == nil {
		return nil
	}
	for c.LastDocID < docID {
		if err := c.Writer.StartDocument(0); err != nil {
			return fmt.Errorf("fill start doc %d: %w", c.LastDocID, err)
		}
		if err := c.Writer.FinishDocument(); err != nil {
			return fmt.Errorf("fill finish doc %d: %w", c.LastDocID, err)
		}
		c.LastDocID++
	}
	return nil
}

// InitTermVectorsWriter lazily creates the codec's TermVectorsWriter.
// Mirrors Lucene's package-private initTermVectorsWriter().
//
// Sprint 55 deviation: Lucene calls
// codec.termVectorsFormat().vectorsWriter(directory, info,
// IOContext.flush(new FlushInfo(lastDocID, bytesUsed.get()))). The
// Gocene TermVectorsFormat takes a *SegmentWriteState; we synthesise
// one from the consumer's own directory and SegmentInfo, attaching the
// FlushInfo-derived IOContext so writers that inspect Context observe
// the same intent.
func (c *TermVectorsConsumer) InitTermVectorsWriter() error {
	if c == nil {
		return errors.New("index: TermVectorsConsumer is nil")
	}
	if c.Writer != nil {
		return nil
	}
	if c.Codec == nil {
		return errors.New("index: TermVectorsConsumer has no Codec")
	}
	format := c.Codec.TermVectorsFormat()
	if format == nil {
		return errors.New("index: TermVectorsConsumer codec has no TermVectorsFormat")
	}
	// Lucene synthesises an IOContext from a FlushInfo(lastDocID,
	// bytesUsed.get()); SegmentWriteState in Gocene has no Context
	// field yet, so the value is dropped on the floor. See the
	// type-doc deviation.
	_ = store.NewFlushContext(&store.FlushInfo{
		NumDocs:              c.LastDocID,
		EstimatedSegmentSize: c.bytesUsed.Get(),
	})
	state := &SegmentWriteState{
		Directory:   c.Directory,
		SegmentInfo: c.Info,
	}
	w, err := format.VectorsWriter(state)
	if err != nil {
		return fmt.Errorf("index: TermVectorsConsumer init writer: %w", err)
	}
	c.Writer = w
	c.LastDocID = 0
	if a, ok := w.(util.Accountable); ok {
		c.accountable = a
	} else {
		c.accountable = nil
	}
	return nil
}

// SetHasVectors marks the segment as having at least one document with
// term vectors. Subsequent FinishDocument calls flush per-field state.
// Mirrors Lucene's package-private setHasVectors().
func (c *TermVectorsConsumer) SetHasVectors() {
	c.hasVectors = true
}

// HasVectors reports whether SetHasVectors has been called. Lucene's
// field is package-private; the accessor is exposed here for the
// per-field type port to query without depending on package-private
// state.
func (c *TermVectorsConsumer) HasVectors() bool {
	return c.hasVectors
}

// FinishDocument serialises the current document's term vectors into
// the active writer. Mirrors Lucene's package-private finishDocument(int).
//
// Pre-conditions mirror Lucene: it is a no-op when SetHasVectors has
// not been called for the current segment. The per-field handles
// (added via AddFieldToFlush during the per-field finishDocument hook)
// are sorted by field name before being flushed; sort is in-place and
// idempotent.
//
// Returns an error wrapping the first underlying failure (init, fill,
// or any per-field FinishDocument) and aborts the rest of the flush
// loop so the writer is not left in a half-populated state. The
// Java original throws via assertions on lastDocID/docID drift; the
// Go port returns an error so the check is observable in release.
func (c *TermVectorsConsumer) FinishDocument(docID int) error {
	if c == nil {
		return errors.New("index: TermVectorsConsumer is nil")
	}
	if !c.hasVectors {
		return nil
	}

	// Lucene: ArrayUtil.introSort(perFields, 0, numVectorFields).
	// We delegate to sort.Slice on the populated prefix; the comparator
	// is supplied by the per-field handle so we do not need to know the
	// concrete type.
	if c.numVectorFields > 1 {
		sub := c.perFields[:c.numVectorFields]
		sort.Slice(sub, func(i, j int) bool {
			return sub[i].CompareName(sub[j]) < 0
		})
	}

	if err := c.InitTermVectorsWriter(); err != nil {
		return err
	}
	if err := c.Fill(docID); err != nil {
		return err
	}

	if err := c.Writer.StartDocument(c.numVectorFields); err != nil {
		return fmt.Errorf("finish doc %d start: %w", docID, err)
	}
	for i := 0; i < c.numVectorFields; i++ {
		if err := c.perFields[i].FinishDocument(); err != nil {
			return fmt.Errorf("finish doc %d field %d: %w", docID, i, err)
		}
	}
	if err := c.Writer.FinishDocument(); err != nil {
		return fmt.Errorf("finish doc %d finish: %w", docID, err)
	}

	if c.LastDocID != docID {
		return fmt.Errorf("index: TermVectorsConsumer lastDocID=%d docID=%d", c.LastDocID, docID)
	}
	c.LastDocID++

	c.reset()
	c.ResetFields()
	return nil
}

// Abort closes the in-flight writer and resets pool state, swallowing
// any error from Close (Lucene calls IOUtils.closeWhileHandlingException).
// Mirrors Lucene's public abort().
func (c *TermVectorsConsumer) Abort() {
	c.abortBase()
	if c.Writer != nil {
		_ = c.Writer.Close()
		c.Writer = nil
		c.accountable = nil
	}
	c.reset()
}

// abortBase mirrors the super.abort() call: the (unported) TermsHash
// parent just calls reset(). It is split out so future test peers can
// observe the order Lucene defines (parent abort → writer close).
func (c *TermVectorsConsumer) abortBase() {
	c.reset()
}

// reset clears the int/byte pools. Mirrors the TermsHash parent's
// package-private reset() (intPool.reset(false,false);
// bytePool.reset(false,false)).
func (c *TermVectorsConsumer) reset() {
	if c.intPool != nil {
		c.intPool.Reset(false, false)
	}
	if c.bytePool != nil {
		c.bytePool.Reset(false, false)
	}
}

// ResetFields clears the per-field array between documents. Mirrors
// Lucene's package-private resetFields(). Exposed for the per-field
// type port that calls it during its own setup path.
func (c *TermVectorsConsumer) ResetFields() {
	for i := range c.perFields {
		c.perFields[i] = nil
	}
	c.numVectorFields = 0
}

// AddField appends a per-field handle for the field about to be
// inverted. Mirrors Lucene's addField(FieldInvertState, FieldInfo).
//
// Sprint 55 deviation: the Lucene method returns the freshly created
// TermVectorsConsumerPerField. That subtype is not yet ported; the
// Gocene method instead accepts a constructor callback so the caller
// (typically DocumentsWriterPerThread) can wire whatever per-field
// implementation it wants without forcing this file to import or know
// the concrete type. The callback receives the invertState / fieldInfo
// the Java constructor consumes and must return a TermVectorsPerField
// Handle.
//
// invertState and fieldInfo are passed through unchanged so the
// callback observes the same inputs the Java constructor does.
func (c *TermVectorsConsumer) AddField(invertState *FieldInvertState, fieldInfo *FieldInfo, build func(*FieldInvertState, *FieldInfo) TermVectorsPerFieldHandle) (TermVectorsPerFieldHandle, error) {
	if build == nil {
		return nil, errors.New("index: TermVectorsConsumer.AddField requires a non-nil build callback")
	}
	handle := build(invertState, fieldInfo)
	if handle == nil {
		return nil, errors.New("index: TermVectorsConsumer.AddField build returned nil handle")
	}
	return handle, nil
}

// AddFieldToFlush registers a per-field handle for the current
// document's flush. Mirrors Lucene's package-private
// addFieldToFlush(TermVectorsConsumerPerField).
//
// The backing array grows via ArrayUtil.oversize semantics; the port
// uses util.OversizeRefs which mirrors Lucene's NUM_BYTES_OBJECT_REF
// growth curve.
func (c *TermVectorsConsumer) AddFieldToFlush(field TermVectorsPerFieldHandle) {
	if c.numVectorFields == len(c.perFields) {
		newSize := util.Oversize(c.numVectorFields+1, util.NumBytesObjectRef)
		grown := make([]TermVectorsPerFieldHandle, newSize)
		copy(grown, c.perFields)
		c.perFields = grown
	}
	c.perFields[c.numVectorFields] = field
	c.numVectorFields++
}

// StartDocument prepares the consumer for a fresh document by resetting
// the per-field array. Mirrors Lucene's package-private startDocument().
func (c *TermVectorsConsumer) StartDocument() {
	c.ResetFields()
	c.numVectorFields = 0
}

// NumVectorFields reports how many per-field handles are currently
// staged for the next FinishDocument call. Lucene's field is package-
// private; the accessor exists for the per-field type port and for
// tests that need to assert the staging invariant.
func (c *TermVectorsConsumer) NumVectorFields() int {
	return c.numVectorFields
}

// RamBytesUsed reports the writer's reported allocation, or 0 when no
// writer is installed (or when the installed writer does not implement
// util.Accountable). Mirrors Lucene's accountable field — Java keeps a
// NULL_ACCOUNTABLE sentinel to avoid the nil check; the Go port uses a
// nil check at the read site (see the type-doc deviation note).
func (c *TermVectorsConsumer) RamBytesUsed() int64 {
	if c.accountable == nil {
		return 0
	}
	return c.accountable.RamBytesUsed()
}
