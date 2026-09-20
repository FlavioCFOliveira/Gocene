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

	// TermsHashBase is the embedded TermsHash parent. Mirrors Lucene's
	// "class TermVectorsConsumer extends TermsHash": it owns intPool,
	// bytePool, termBytePool, bytesUsed and the nextTermsHash link, and
	// supplies the concrete TermsHash methods this type does not override.
	*TermsHashBase

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
	// perFieldByBase maps the embedded *TermsHashPerField of each
	// per-field writer back to its TermVectorsConsumerPerField owner.
	// Lucene recovers the subtype with a cast; Go needs the registry.
	perFieldByBase map[*TermsHashPerField]*TermVectorsConsumerPerField

	// initTermVectorsWriterOverride renders the @Override of
	// initTermVectorsWriter() in SortingTermVectorsConsumer. Java resolves
	// the call inside finishDocument() virtually; Go embedding does not,
	// so the subclass installs its body here and the base dispatches
	// through initTermVectorsWriter(). nil means "no subclass override".
	initTermVectorsWriterOverride func() error
	// accountable holds the active writer when it implements
	// util.Accountable, otherwise nil. Mirrors Lucene's accountable
	// field with NULL_ACCOUNTABLE replaced by a nil check at the read
	// site (RamBytesUsed).
	accountable util.Accountable
}

// NewTermVectorsConsumer constructs the consumer for the given segment.
//
// Mirrors Lucene's constructor
// TermVectorsConsumer(IntBlockPool.Allocator, ByteBlockPool.Allocator,
// Directory, SegmentInfo, Codec), whose body is
//
//	super(intBlockAllocator, byteBlockAllocator, Counter.newCounter(), null);
//
// i.e. the consumer owns a private bytes counter (it is not the chain's
// shared counter) and sits at the tail of the inversion chain, so
// nextTermsHash is null.
//
// Returns nil when info is nil: Lucene relies on a non-null SegmentInfo
// for every observable code path the consumer takes.
func NewTermVectorsConsumer(
	intBlockAllocator util.IntAllocator,
	byteBlockAllocator util.Allocator,
	directory store.Directory,
	info *SegmentInfo,
	codec Codec,
) *TermVectorsConsumer {
	if info == nil {
		return nil
	}
	return &TermVectorsConsumer{
		TermsHashBase:        NewTermsHashBase(intBlockAllocator, byteBlockAllocator, util.NewCounter(), nil),
		Directory:            directory,
		Info:                 info,
		Codec:                codec,
		FlushTerm:            &util.BytesRef{},
		VectorSliceReaderPos: &ByteSliceReader{},
		VectorSliceReaderOff: &ByteSliceReader{},
		perFields:            make([]TermVectorsPerFieldHandle, 1),
	}
}

// Compile-time guarantee that *TermVectorsConsumer is a TermsHash, as
// "class TermVectorsConsumer extends TermsHash" requires in Lucene.
var _ TermsHash = (*TermVectorsConsumer)(nil)

// Flush completes the current segment's term vectors. It mirrors the
// package-private flush(Map<String,TermsHashPerField>, SegmentWriteState,
// Sorter.DocMap, NormsProducer) in Lucene.
//
// fieldsToFlush, sortMap and norms are part of the TermsHash contract but
// are not read by this body: Lucene's TermVectorsConsumer.flush overrides
// TermsHash.flush without calling super (the consumer is the tail of the
// chain, so there is nothing to delegate to) and touches only writer,
// state.segmentInfo.maxDoc() and lastDocID.
func (c *TermVectorsConsumer) Flush(
	fieldsToFlush map[string]*TermsHashPerField,
	state *SegmentWriteState,
	sortMap SorterDocMap,
	norms any,
) error {
	_ = fieldsToFlush // mirror Lucene: the override body does not use it
	_ = sortMap       // mirror Lucene: the override body does not use it
	_ = norms         // mirror Lucene: the override body does not use it
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

	// Rebind the writer to the final SegmentWriteState when the concrete
	// implementation supports it.  This is required because IndexWriter.Commit
	// creates a fresh SegmentInfo (with a new segment ID) after the writer was
	// lazily opened during indexing, and the .tvd/.tvx headers must match the
	// ID advertised by the .si file.
	if stateSetter, ok := c.Writer.(interface {
		SetSegmentWriteState(state *SegmentWriteState) error
	}); ok {
		if err := stateSetter.SetSegmentWriteState(state); err != nil {
			return fmt.Errorf("index: TermVectorsConsumer flush rebind state: %w", err)
		}
	}

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
	// Mirrors TermVectorsConsumer.initTermVectorsWriter
	// (TermVectorsConsumer.java:103-110): IOContext.flush(new
	// FlushInfo(lastDocID, bytesUsed.get())) then
	// codec.termVectorsFormat().vectorsWriter(directory, info, context).
	context := store.NewFlushContext(&store.FlushInfo{
		NumDocs:              c.LastDocID,
		EstimatedSegmentSize: c.BytesUsed.Get(),
	})
	w, err := format.VectorsWriter(c.Directory, c.Info, context)
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

// initTermVectorsWriter performs the virtual dispatch Java gets for free:
// it runs the subclass override when one is installed, otherwise the base
// InitTermVectorsWriter body. Every in-class call site of
// initTermVectorsWriter() in Lucene goes through this.
func (c *TermVectorsConsumer) initTermVectorsWriter() error {
	if c.initTermVectorsWriterOverride != nil {
		return c.initTermVectorsWriterOverride()
	}
	return c.InitTermVectorsWriter()
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

	if err := c.initTermVectorsWriter(); err != nil {
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

	c.TermsHashBase.Reset()
	c.ResetFields()
	return nil
}

// Abort closes the in-flight writer and resets pool state, swallowing
// any error from Close (Lucene calls IOUtils.closeWhileHandlingException).
// Mirrors Lucene's public abort().
func (c *TermVectorsConsumer) Abort() {
	c.TermsHashBase.Abort()
	if c.Writer != nil {
		_ = c.Writer.Close()
		c.Writer = nil
		c.accountable = nil
	}
	c.TermsHashBase.Reset()
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

// AddField creates the per-field term-vectors writer for the field about
// to be inverted and returns its embedded TermsHashPerField, exactly as
// Lucene's
//
//	public TermsHashPerField addField(FieldInvertState invertState, FieldInfo fieldInfo) {
//	  return new TermVectorsConsumerPerField(invertState, this, fieldInfo);
//	}
//
// does. The concrete writer is recoverable from the returned base via
// PerFieldFor.
//
// Lucene's constructor cannot fail: the only failure conditions the
// Gocene constructor reports (nil arguments, IndexOptions.NONE) are
// Java assertions, which raise AssertionError. The port therefore
// panics on them, matching the Java failure mode, and keeps the
// TermsHash.AddField signature free of an error channel just as the
// Java method is free of a checked exception.
func (c *TermVectorsConsumer) AddField(invertState *FieldInvertState, fieldInfo *FieldInfo) *TermsHashPerField {
	pf, err := NewTermVectorsConsumerPerField(invertState, c, fieldInfo, TermVectorsAttributeProvider{})
	if err != nil {
		panic(fmt.Sprintf("index: TermVectorsConsumer.AddField: %v", err))
	}
	if c.perFieldByBase == nil {
		c.perFieldByBase = make(map[*TermsHashPerField]*TermVectorsConsumerPerField)
	}
	c.perFieldByBase[pf.TermsHashPerField] = pf
	return pf.TermsHashPerField
}

// PerFieldFor recovers the TermVectorsConsumerPerField that owns base, or
// nil when base was not produced by this consumer's AddField.
//
// Lucene reaches the subtype with a cast — the map exists because Go has
// no downcast from the embedded *TermsHashPerField to its owner.
func (c *TermVectorsConsumer) PerFieldFor(base *TermsHashPerField) *TermVectorsConsumerPerField {
	return c.perFieldByBase[base]
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
// the per-field array. Mirrors Lucene's package-private startDocument(),
// which overrides TermsHash.startDocument without delegating (the
// consumer is the tail of the chain).
func (c *TermVectorsConsumer) StartDocument() error {
	c.ResetFields()
	c.numVectorFields = 0
	return nil
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
