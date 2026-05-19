// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// SortingStoredFieldsConsumer buffers per-document stored fields into a
// temporary, uncompressed Lucene-style stored-fields segment, then on flush
// reorders the documents into the final segment according to the index sort
// produced by SorterDocMap.
//
// This is the Go port of Apache Lucene 10.4.0's
// org.apache.lucene.index.SortingStoredFieldsConsumer (186 lines).
//
// Sprint 55 "option c" deviations (placeholders inlined for cross-task
// independence; to be replaced when prerequisite ports land):
//
//   - StoredFieldsConsumer (GOC-3394) parent type is not yet ported. This
//     file declares an unexported storedFieldsConsumerBase struct that
//     carries the codec/directory/info/writer state. The exported
//     SortingStoredFieldsConsumer composes that base; once GOC-3394 lands,
//     the base is replaced by the canonical parent (consumers move to
//     embedding rather than composition).
//
//   - Lucene90CompressingStoredFieldsFormat (compressing module) is not yet
//     wired through the codec SPI from package index. The constant TempFormat
//     is exposed as a hook that callers (DocumentsWriterPerThread) inject
//     when constructing the consumer. The default behavior, when no
//     temporary format is supplied, is to return ErrTempFormatUnset on the
//     first stored field — making the wiring gap explicit instead of silent.
//
//   - TrackingTmpOutputDirectoryWrapper (store package, separate task) is
//     stubbed locally as trackingTmpDirectoryWrapper. It records every
//     temporary file opened via CreateOutput so the consumer can delete
//     them during flush/abort. The full Lucene wrapper (with prefix
//     management and CreateTempOutput integration) will replace this stub
//     in its own port task.
//
// All public symbols mirror Lucene 10.4.0 semantics; deviations are scoped
// to internal plumbing that callers do not observe.

// ErrTempFormatUnset is returned by SortingStoredFieldsConsumer.Flush /
// InitStoredFieldsWriter when no temporary StoredFieldsFormat has been
// supplied via SetTempStoredFieldsFormat. This surfaces the Sprint 55
// wiring gap explicitly instead of silently no-oping.
var ErrTempFormatUnset = errors.New("index: SortingStoredFieldsConsumer requires a temporary StoredFieldsFormat (see SetTempStoredFieldsFormat); GOC-3394 / compressing wiring pending")

// storedFieldsConsumerBase is the Sprint 55 placeholder for the parent
// type ported in GOC-3394. It carries the fields the parent owns in
// Lucene: codec, directory, segment info, and the active writer.
//
// When GOC-3394 lands, this struct disappears: SortingStoredFieldsConsumer
// embeds the canonical *StoredFieldsConsumer and the field-access paths
// here switch to the embedded receiver. The migration is mechanical.
type storedFieldsConsumerBase struct {
	codec     Codec
	directory store.Directory
	info      *SegmentInfo
	writer    StoredFieldsWriter
}

// SortingStoredFieldsConsumer specializes the stored-fields consumer for
// segments that are sorted at flush time. Documents are first buffered in
// document-write order to a temporary uncompressed segment, then
// reordered into the codec-provided StoredFieldsWriter using the
// SorterDocMap supplied at flush.
//
// Mirrors org.apache.lucene.index.SortingStoredFieldsConsumer.
type SortingStoredFieldsConsumer struct {
	storedFieldsConsumerBase

	// tmpDirectory is the tracking wrapper around the segment directory
	// where the buffered (pre-sort) stored fields live. nil until
	// InitStoredFieldsWriter creates it.
	tmpDirectory *trackingTmpDirectoryWrapper

	// tempFormat is the StoredFieldsFormat used for the buffered segment.
	// In Lucene this is Lucene90CompressingStoredFieldsFormat with the
	// NO_COMPRESSION mode; until GOC-3394 + the compressing wiring land,
	// callers must inject it via SetTempStoredFieldsFormat.
	tempFormat StoredFieldsFormat
}

// NewSortingStoredFieldsConsumer constructs the consumer for the given
// segment. It mirrors the Lucene constructor
// SortingStoredFieldsConsumer(Codec, Directory, SegmentInfo).
//
// The temporary StoredFieldsFormat used to buffer the pre-sort segment
// must be supplied separately via SetTempStoredFieldsFormat before the
// first stored field is written; otherwise InitStoredFieldsWriter returns
// ErrTempFormatUnset. See the Sprint 55 deviation note on the type doc.
func NewSortingStoredFieldsConsumer(codec Codec, directory store.Directory, info *SegmentInfo) *SortingStoredFieldsConsumer {
	if info == nil {
		// Mirrors Lucene's reliance on a non-null SegmentInfo: there is
		// no useful behaviour the consumer can perform without it.
		return nil
	}
	return &SortingStoredFieldsConsumer{
		storedFieldsConsumerBase: storedFieldsConsumerBase{
			codec:     codec,
			directory: directory,
			info:      info,
		},
	}
}

// SetTempStoredFieldsFormat injects the StoredFieldsFormat used for the
// temporary, pre-sort segment. This exists only to bridge the Sprint 55
// wiring gap; once Lucene90CompressingStoredFieldsFormat is reachable
// from package index without import cycles (post GOC-3394), the field
// becomes a package-private constant initialised at package load and
// this setter is removed.
func (c *SortingStoredFieldsConsumer) SetTempStoredFieldsFormat(format StoredFieldsFormat) {
	c.tempFormat = format
}

// TempDirectory returns the tracking wrapper around the segment directory
// that holds the temporary pre-sort stored-fields files. Returns nil if
// InitStoredFieldsWriter has not run yet.
//
// Exposed (Lucene's field is package-private) for test inspection of the
// temporary-file lifecycle.
func (c *SortingStoredFieldsConsumer) TempDirectory() *trackingTmpDirectoryWrapper {
	return c.tmpDirectory
}

// InitStoredFieldsWriter lazily creates the temporary StoredFieldsWriter
// the first time it is called. Subsequent calls are no-ops.
//
// Mirrors the protected initStoredFieldsWriter() in Lucene.
func (c *SortingStoredFieldsConsumer) InitStoredFieldsWriter() error {
	if c.writer != nil {
		return nil
	}
	if c.tempFormat == nil {
		return ErrTempFormatUnset
	}
	c.tmpDirectory = newTrackingTmpDirectoryWrapper(c.directory)
	w, err := c.tempFormat.FieldsWriter(c.tmpDirectory, c.info, store.IOContextDefault)
	if err != nil {
		c.tmpDirectory = nil
		return fmt.Errorf("index: SortingStoredFieldsConsumer init temp writer: %w", err)
	}
	c.writer = w
	return nil
}

// Flush completes the buffered segment, then copies documents into the
// codec's StoredFieldsWriter, reordering through sortMap when non-nil.
//
// Mirrors org.apache.lucene.index.SortingStoredFieldsConsumer.flush.
//
// The integrity check that Lucene performs via the compressing reader is
// elided here (the buffered reader implementation lands with GOC-3394 /
// the compressing port); structurally the call site is preserved as a
// comment so the future hook is obvious.
func (c *SortingStoredFieldsConsumer) Flush(state *SegmentWriteState, sortMap SorterDocMap) error {
	if state == nil || state.SegmentInfo == nil {
		return errors.New("index: SortingStoredFieldsConsumer.Flush requires a non-nil state with SegmentInfo")
	}
	if c.tempFormat == nil || c.tmpDirectory == nil || c.writer == nil {
		// Nothing was buffered. Mirrors the implicit no-op path Lucene
		// gets when initStoredFieldsWriter was never invoked.
		return nil
	}

	// Close the temporary writer (super.flush() in Lucene flushes the
	// buffered writer; in the port we just close it before reopening
	// for read since FieldsWriter does not expose a Flush method).
	if err := c.writer.Close(); err != nil {
		c.cleanupTempFiles()
		return fmt.Errorf("index: SortingStoredFieldsConsumer flush close temp writer: %w", err)
	}
	c.writer = nil

	reader, err := c.tempFormat.FieldsReader(c.tmpDirectory, state.SegmentInfo, state.FieldInfos, store.IOContextDefault)
	if err != nil {
		c.cleanupTempFiles()
		return fmt.Errorf("index: SortingStoredFieldsConsumer flush open temp reader: %w", err)
	}

	// Don't pull a merge instance: stored fields are consumed in random
	// order here, not sequentially.
	sortWriter, err := c.codec.StoredFieldsFormat().FieldsWriter(state.Directory, state.SegmentInfo, store.IOContextDefault)
	if err != nil {
		_ = reader.Close()
		c.cleanupTempFiles()
		return fmt.Errorf("index: SortingStoredFieldsConsumer flush open sort writer: %w", err)
	}

	flushErr := c.copyDocuments(reader, sortWriter, state.SegmentInfo.DocCount(), sortMap)
	closeErr := closeAll(reader, sortWriter)
	c.cleanupTempFiles()

	switch {
	case flushErr != nil:
		return flushErr
	case closeErr != nil:
		return fmt.Errorf("index: SortingStoredFieldsConsumer flush close: %w", closeErr)
	}
	return nil
}

// copyDocuments walks the buffered reader in sorted order, copying every
// field of every document into the codec writer.
func (c *SortingStoredFieldsConsumer) copyDocuments(reader StoredFieldsReader, sortWriter StoredFieldsWriter, maxDoc int, sortMap SorterDocMap) error {
	visitor := &copyVisitor{writer: sortWriter}
	for docID := 0; docID < maxDoc; docID++ {
		if err := sortWriter.StartDocument(); err != nil {
			return fmt.Errorf("index: SortingStoredFieldsConsumer flush start doc %d: %w", docID, err)
		}
		sourceDoc := docID
		if sortMap != nil {
			sourceDoc = sortMap.NewToOld(docID)
		}
		if err := reader.VisitDocument(sourceDoc, visitor); err != nil {
			return fmt.Errorf("index: SortingStoredFieldsConsumer flush visit doc %d (source %d): %w", docID, sourceDoc, err)
		}
		if visitor.err != nil {
			return fmt.Errorf("index: SortingStoredFieldsConsumer flush copy doc %d: %w", docID, visitor.err)
		}
		if err := sortWriter.FinishDocument(); err != nil {
			return fmt.Errorf("index: SortingStoredFieldsConsumer flush finish doc %d: %w", docID, err)
		}
	}
	return nil
}

// Abort closes the in-flight writer and deletes any buffered temporary
// files, swallowing per-file errors as Lucene does.
//
// Mirrors org.apache.lucene.index.SortingStoredFieldsConsumer.abort.
func (c *SortingStoredFieldsConsumer) Abort() {
	if c.writer != nil {
		_ = c.writer.Close()
		c.writer = nil
	}
	if c.tmpDirectory != nil {
		for _, name := range c.tmpDirectory.TemporaryFiles() {
			_ = c.directory.DeleteFile(name)
		}
		c.tmpDirectory = nil
	}
}

// cleanupTempFiles is the flush-path equivalent of Abort's temp-file
// removal: it deletes every file the tracking wrapper recorded, ignoring
// individual delete errors, then clears the wrapper.
func (c *SortingStoredFieldsConsumer) cleanupTempFiles() {
	if c.tmpDirectory == nil {
		return
	}
	for _, name := range c.tmpDirectory.TemporaryFiles() {
		_ = c.directory.DeleteFile(name)
	}
	c.tmpDirectory = nil
}

// closeAll closes both arguments and returns the first non-nil error.
// Mirrors IOUtils.close semantics for the two-arg flush path.
func closeAll(reader StoredFieldsReader, writer StoredFieldsWriter) error {
	rerr := reader.Close()
	werr := writer.Close()
	if rerr != nil {
		return rerr
	}
	return werr
}

// copyVisitor copies every field it observes into the wrapped writer. It
// mirrors SortingStoredFieldsConsumer.CopyVisitor.
//
// Gocene's StoredFieldVisitor exposes value-typed callbacks (string,
// []byte, int, int64, float32, float64) rather than the Java overloads;
// the copier therefore wraps each value in a minimal IndexableField on
// the fly so it can be handed to StoredFieldsWriter.WriteField.
//
// Any error returned by the underlying writer is captured in copyVisitor.err
// rather than panicked; Flush inspects err between Visit/FinishDocument.
type copyVisitor struct {
	writer StoredFieldsWriter
	err    error
}

func (v *copyVisitor) write(field IndexableField) {
	if v.err != nil {
		return
	}
	if err := v.writer.WriteField(field); err != nil {
		v.err = err
	}
}

func (v *copyVisitor) StringField(field string, value string) {
	v.write(&copiedField{name: field, kind: copiedString, str: value})
}

func (v *copyVisitor) BinaryField(field string, value []byte) {
	// Mirrors Lucene's TODO: avoid the copy if upstream can guarantee
	// stable byte slices across the FinishDocument boundary.
	buf := make([]byte, len(value))
	copy(buf, value)
	v.write(&copiedField{name: field, kind: copiedBinary, bin: buf})
}

func (v *copyVisitor) IntField(field string, value int) {
	v.write(&copiedField{name: field, kind: copiedInt, num: int64(value)})
}

func (v *copyVisitor) LongField(field string, value int64) {
	v.write(&copiedField{name: field, kind: copiedLong, num: value})
}

func (v *copyVisitor) FloatField(field string, value float32) {
	v.write(&copiedField{name: field, kind: copiedFloat, f32: value})
}

func (v *copyVisitor) DoubleField(field string, value float64) {
	v.write(&copiedField{name: field, kind: copiedDouble, f64: value})
}

// copiedField is the minimal IndexableField the copyVisitor produces. It
// is stored-only and carries exactly one typed value (the one the visitor
// observed). It exists because Gocene's StoredFieldVisitor surface gives
// us raw values, not IndexableField instances we could pass through.
type copiedField struct {
	name string
	kind copiedKind
	str  string
	bin  []byte
	num  int64
	f32  float32
	f64  float64
}

type copiedKind uint8

const (
	copiedString copiedKind = iota
	copiedBinary
	copiedInt
	copiedLong
	copiedFloat
	copiedDouble
)

// Name implements IndexableField.
func (f *copiedField) Name() string { return f.name }

// FieldType implements IndexableField with a stored-only marker type.
func (f *copiedField) FieldType() FieldTypeInterface { return copiedFieldType{} }

// StringValue implements IndexableField.
func (f *copiedField) StringValue() string {
	if f.kind == copiedString {
		return f.str
	}
	return ""
}

// BinaryValue implements IndexableField.
func (f *copiedField) BinaryValue() []byte {
	if f.kind == copiedBinary {
		return f.bin
	}
	return nil
}

// NumericValue implements IndexableField. Returns the concrete numeric
// type matching the visitor callback that produced the field; returns nil
// for non-numeric kinds.
func (f *copiedField) NumericValue() interface{} {
	switch f.kind {
	case copiedInt:
		return int(f.num)
	case copiedLong:
		return f.num
	case copiedFloat:
		return f.f32
	case copiedDouble:
		return f.f64
	default:
		return nil
	}
}

// copiedFieldType marks the copied field as stored-only. Every other
// indexing property is false because the copier is feeding a stored-only
// writer.
type copiedFieldType struct{}

func (copiedFieldType) IsIndexed() bool                 { return false }
func (copiedFieldType) IsStored() bool                  { return true }
func (copiedFieldType) IsTokenized() bool               { return false }
func (copiedFieldType) GetIndexOptions() IndexOptions   { return IndexOptionsNone }
func (copiedFieldType) GetDocValuesType() DocValuesType { return DocValuesTypeNone }
func (copiedFieldType) StoreTermVectors() bool          { return false }
func (copiedFieldType) StoreTermVectorPositions() bool  { return false }
func (copiedFieldType) StoreTermVectorOffsets() bool    { return false }

// trackingTmpDirectoryWrapper is the Sprint 55 stand-in for
// org.apache.lucene.index.TrackingTmpOutputDirectoryWrapper. It records
// every file opened via CreateOutput / CreateTempOutput so the consumer
// can delete those files during flush or abort.
//
// All other Directory operations delegate untouched to the inner
// directory. Once the canonical wrapper ships under store/, this type
// is replaced wholesale; callers consume it only through Directory.
type trackingTmpDirectoryWrapper struct {
	store.Directory
	files []string
}

func newTrackingTmpDirectoryWrapper(inner store.Directory) *trackingTmpDirectoryWrapper {
	return &trackingTmpDirectoryWrapper{Directory: inner}
}

// TemporaryFiles returns the names of every file the wrapper has handed
// out, in creation order. The returned slice is a defensive copy.
func (w *trackingTmpDirectoryWrapper) TemporaryFiles() []string {
	out := make([]string, len(w.files))
	copy(out, w.files)
	return out
}

// CreateOutput records the name before delegating.
func (w *trackingTmpDirectoryWrapper) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	out, err := w.Directory.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}
	w.files = append(w.files, name)
	return out, nil
}

// CreateTempOutput records the resulting name before delegating. The
// canonical Lucene wrapper synthesises a tracking-specific prefix here;
// the Sprint 55 stub defers to the underlying directory's prefix logic.
func (w *trackingTmpDirectoryWrapper) CreateTempOutput(prefix, suffix string, ctx store.IOContext) (store.IndexOutput, error) {
	type tempCreator interface {
		CreateTempOutput(prefix, suffix string, ctx store.IOContext) (store.IndexOutput, error)
	}
	if tc, ok := w.Directory.(tempCreator); ok {
		out, err := tc.CreateTempOutput(prefix, suffix, ctx)
		if err != nil {
			return nil, err
		}
		w.files = append(w.files, out.GetName())
		return out, nil
	}
	// Fallback: synthesise a name and call CreateOutput.
	name := fmt.Sprintf("%s_tmp_%d_%s", prefix, len(w.files), suffix)
	return w.CreateOutput(name, ctx)
}
