// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecbridge installs the production Lucene 10.4 codec as the
// default Codec for the index package.
//
// # Why this package exists
//
// The index/ package defines an index.Codec interface family that mirrors,
// but is not identical to, the codecs/ package's Codec interface family.
// The two families exist because index/ cannot import codecs/ (codecs/
// already imports index/ to reach concrete types such as *index.SegmentInfo
// and *index.FieldInfos), and structurally unifying the two families is a
// large enough refactor that it has been split out as a follow-up task
// (rmp #4669, "SPI unification"). Until that lands, the bridge defined
// here adapts a concrete codecs.Codec into an index.Codec so the
// IndexWriter can persist documents through real Lucene 10.4 formats by
// default.
//
// # Usage
//
// Production callers (binaries, integration tests, examples) blank-import
// this package to install the bridge as the index-side default:
//
//	import _ "github.com/FlavioCFOliveira/Gocene/internal/codecbridge"
//
// The package's init() calls index.RegisterDefaultCodec with a bridge
// wrapping codecs.NewLucene104Codec(). Tests that do not need a real codec
// (purely structural unit tests) may omit the import and accept that
// IndexWriter.AddDocument + Commit will surface index.ErrNoCodec.
//
// # Scope of adaptation
//
// The bridge adapts every interface method exposed by index.Codec:
// PostingsFormat, StoredFieldsFormat, FieldInfosFormat, SegmentInfosFormat,
// SegmentInfoFormat, and TermVectorsFormat. The only non-trivial conversion
// is StoredFieldsWriter.WriteField, where the inbound field is an
// index.IndexableField (the codec-facing narrow interface) but the
// codecs.StoredFieldsWriter contract expects document.IndexableField (the
// document-facing wider interface). That conversion is documented in
// adaptIndexableField below.
package codecbridge

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	corecompressing "github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	l90compressing "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// init installs the production Lucene 10.4 codec as the default codec
// resolved by index.NewIndexWriterConfig, and also registers it under its
// canonical name so that OpenDirectoryReader can resolve it by the codec
// name stored in each segment's SegmentInfo on disk.
//
// init also publishes the temporary stored-fields and term-vectors
// formats used by SortingStoredFieldsConsumer and
// SortingTermVectorsConsumer to buffer per-document state in
// document-write order before reordering at flush time. The canonical
// Lucene 10.4.0 wiring is:
//
//	new Lucene90CompressingStoredFieldsFormat(
//	    "TempStoredFields", NO_COMPRESSION, 128 * 1024, 1, 10);
//	new Lucene90CompressingTermVectorsFormat(
//	    "TempTermVectors", NO_COMPRESSION, 128 * 1024, 1, 10);
//
// We adapt those into the index-side StoredFieldsFormat / TermVectorsFormat
// interfaces through the same adapters used by the codec bridge.
func init() {
	bridge := BridgeForLucene104()
	index.RegisterDefaultCodec(bridge)
	index.RegisterNamedCodec(bridge.Name(), bridge)

	const (
		tempChunkSize       = 128 * 1024
		tempMaxDocsPerChunk = 1
		tempBlockShift      = 10
	)
	tempStored := l90compressing.NewLucene90CompressingStoredFieldsFormatWithOptions(
		"TempStoredFields",
		corecompressing.NO_COMPRESSION,
		tempChunkSize, tempMaxDocsPerChunk, tempBlockShift,
	)
	index.RegisterDefaultTempStoredFieldsFormat(&storedFieldsFormatAdapter{inner: tempStored})

	// Lucene90CompressingTermVectorsFormat in the Gocene port is currently a
	// stub that does not accept tuning options; once it gains the 5-arg
	// constructor matching the Java reference this hook switches to the
	// canonical ("TempTermVectors", NO_COMPRESSION, 128 KB, 1, 10) tuple.
	// In the meantime, registering nil keeps DefaultTempTermVectorsFormat
	// equal to the production "no override" state, so
	// SortingTermVectorsConsumer surfaces ErrTempTermVectorsFormatUnset on
	// the first use rather than producing a silently-wrong segment.
	_ = l90compressing.NewLucene90CompressingTermVectorsFormat
}

// BridgeForLucene104 returns an index.Codec that delegates to the
// production Lucene104Codec defined in package codecs.
//
// The returned value is safe for concurrent use; the underlying
// codecs.Codec implementations are stateless with respect to format
// dispatch.
func BridgeForLucene104() index.Codec {
	return NewBridge(codecs.NewLucene104Codec())
}

// NewBridge wraps an arbitrary codecs.Codec in an index.Codec adapter.
// Exported to allow tests and future callers to install custom codecs
// (e.g., Asserting or filtering codec variants) without going through
// the package-init default.
func NewBridge(c codecs.Codec) index.Codec {
	return &bridgeCodec{inner: c}
}

// bridgeCodec implements index.Codec by delegating each format accessor
// to a dedicated adapter that translates between the index and codecs
// SPI families.
type bridgeCodec struct {
	inner codecs.Codec
}

// Compile-time guarantee that bridgeCodec satisfies index.Codec.
var _ index.Codec = (*bridgeCodec)(nil)

func (b *bridgeCodec) Name() string {
	return b.inner.Name()
}

func (b *bridgeCodec) PostingsFormat() index.PostingsFormat {
	return &postingsFormatAdapter{inner: b.inner.PostingsFormat()}
}

func (b *bridgeCodec) StoredFieldsFormat() index.StoredFieldsFormat {
	return &storedFieldsFormatAdapter{inner: b.inner.StoredFieldsFormat()}
}

func (b *bridgeCodec) FieldInfosFormat() index.FieldInfosFormat {
	return &fieldInfosFormatAdapter{inner: b.inner.FieldInfosFormat()}
}

func (b *bridgeCodec) SegmentInfosFormat() index.SegmentInfosFormat {
	return &segmentInfosFormatAdapter{inner: b.inner.SegmentInfosFormat()}
}

func (b *bridgeCodec) SegmentInfoFormat() index.SegmentInfoFormat {
	// codecs.Codec does not expose SegmentInfoFormat directly; the .si
	// format is reachable only via the concrete Lucene99SegmentInfoFormat.
	// Mirror Lucene 10.4 wiring by instantiating it here.
	return &segmentInfoFormatAdapter{inner: codecs.NewLucene99SegmentInfoFormat()}
}

func (b *bridgeCodec) TermVectorsFormat() index.TermVectorsFormat {
	return &termVectorsFormatAdapter{inner: b.inner.TermVectorsFormat()}
}

func (b *bridgeCodec) CompoundFormat() index.CompoundFormat {
	cf := b.inner.CompoundFormat()
	if cf == nil {
		return nil
	}
	return &compoundFormatAdapter{inner: cf}
}

// compoundFormatAdapter adapts a codecs.CompoundFormat to index.CompoundFormat.
// codecs.CompoundDirectory and index.CompoundDirectory are structurally
// equivalent (both embed store.Directory plus CheckIntegrity() error). Any
// concrete value satisfying one satisfies the other, so we return the concrete
// value directly via the index.CompoundDirectory interface.
type compoundFormatAdapter struct {
	inner codecs.CompoundFormat
}

func (a *compoundFormatAdapter) Write(dir store.Directory, si *index.SegmentInfo, ctx store.IOContext) error {
	return a.inner.Write(dir, si, ctx)
}

func (a *compoundFormatAdapter) GetCompoundReader(dir store.Directory, si *index.SegmentInfo) (index.CompoundDirectory, error) {
	cd, err := a.inner.GetCompoundReader(dir, si)
	if err != nil {
		return nil, err
	}
	return cd, nil
}

// knnVectorsFormatProvider is an optional interface satisfied by codecs (such
// as Lucene104Codec) that expose a KNN vectors format. The bridge uses a
// type-assert to this interface so the codecs.Codec interface itself does not
// need to be widened.
type knnVectorsFormatProvider interface {
	KnnVectorsFormat() codecs.KnnVectorsFormat
}

// KnnVectorsFormat implements index.Codec by delegating to the inner
// codecs.Codec when it satisfies knnVectorsFormatProvider. Returns nil when
// the underlying codec does not support KNN vectors, which mirrors
// codec.knnVectorsFormat() returning null in Lucene.
func (b *bridgeCodec) KnnVectorsFormat() index.KnnVectorsFormatFactory {
	kp, ok := b.inner.(knnVectorsFormatProvider)
	if !ok {
		return nil
	}
	f := kp.KnnVectorsFormat()
	if f == nil {
		return nil
	}
	return &knnVectorsFormatAdapter{inner: f}
}

// knnVectorsFormatAdapter adapts a codecs.KnnVectorsFormat to
// index.KnnVectorsFormatFactory. The adaptation is one-to-one for the
// FieldsWriter method: both accept *index.SegmentWriteState; the return
// type is wrapped by knnVectorsConsumerWriterAdapter.
type knnVectorsFormatAdapter struct {
	inner codecs.KnnVectorsFormat
}

// FieldsWriter constructs a KnnVectorsConsumerWriter from the underlying
// codecs.KnnVectorsFormat. The index.SegmentWriteState is converted to a
// codecs.SegmentWriteState (same fields, different package types) before
// dispatch. The returned writer is wrapped by knnVectorsConsumerWriterAdapter.
func (a *knnVectorsFormatAdapter) FieldsWriter(state *index.SegmentWriteState) (index.KnnVectorsConsumerWriter, error) {
	cs := &codecs.SegmentWriteState{
		Directory:     state.Directory,
		SegmentInfo:   state.SegmentInfo,
		FieldInfos:    state.FieldInfos,
		SegmentSuffix: state.SegmentSuffix,
	}
	w, err := a.inner.FieldsWriter(cs)
	if err != nil {
		return nil, err
	}
	return &knnVectorsConsumerWriterAdapter{inner: w}, nil
}

// knnVectorsConsumerWriterAdapter adapts a codecs.KnnVectorsWriter (which
// exposes AddField / Flush / Finish / Close and RAM accounting) to
// index.KnnVectorsConsumerWriter. The two interfaces are structurally
// similar for the concrete Lucene99HnswVectorsWriter; the principal
// differences are:
//
//   - AddField return type: the codecs writer returns a concrete per-field
//     writer; the index interface requires (any, error) so the concrete
//     value is boxed.
//   - Flush signature: the codecs writer takes Flush(maxDoc int) while the
//     index interface takes Flush(maxDoc int, sortMap index.SorterDocMap).
//     The sortMap is forwarded only when the underlying writer exposes a
//     matching method; otherwise it is silently ignored, matching the
//     existing deviation noted in lucene99_hnsw_vectors_writer.go.
//   - RamBytesUsed: forwarded when the concrete writer implements
//     util.Accountable; zero otherwise.
type knnVectorsConsumerWriterAdapter struct {
	inner codecs.KnnVectorsWriter
}

// knnAddFielder is the optional narrow interface satisfied by concrete
// KnnVectorsWriter implementations that expose AddField(fi) (any, error).
type knnAddFielder interface {
	AddField(fi *index.FieldInfo) (any, error)
}

// knnFlusherWithSortMap is the optional interface for writers that accept
// the sortMap argument on Flush.
type knnFlusherWithSortMap interface {
	Flush(maxDoc int, sortMap index.SorterDocMap) error
}

// knnFlusher is the common interface for writers that only take maxDoc.
type knnFlusher interface {
	Flush(maxDoc int) error
}

// AddField delegates to the inner writer. The concrete return value is
// boxed as any to satisfy the index.KnnVectorsConsumerWriter contract.
func (a *knnVectorsConsumerWriterAdapter) AddField(fi *index.FieldInfo) (any, error) {
	if af, ok := a.inner.(knnAddFielder); ok {
		return af.AddField(fi)
	}
	// inner does not expose AddField; return nil which signals the consumer
	// that no per-field writer is available (non-fatal for indexing chains
	// that guard on the returned handle being nil).
	return nil, nil
}

// Flush delegates to the inner writer, passing sortMap when the writer
// supports it, otherwise falling back to the maxDoc-only variant.
func (a *knnVectorsConsumerWriterAdapter) Flush(maxDoc int, sortMap index.SorterDocMap) error {
	if sm, ok := a.inner.(knnFlusherWithSortMap); ok {
		return sm.Flush(maxDoc, sortMap)
	}
	if f, ok := a.inner.(knnFlusher); ok {
		return f.Flush(maxDoc)
	}
	return nil
}

// Finish delegates to the inner writer.
func (a *knnVectorsConsumerWriterAdapter) Finish() error {
	return a.inner.Finish()
}

// Close delegates to the inner writer.
func (a *knnVectorsConsumerWriterAdapter) Close() error {
	return a.inner.Close()
}

// RamBytesUsed returns the inner writer's RAM estimate when it implements
// util.Accountable; zero otherwise.
func (a *knnVectorsConsumerWriterAdapter) RamBytesUsed() int64 {
	if acc, ok := a.inner.(util.Accountable); ok {
		return acc.RamBytesUsed()
	}
	return 0
}
