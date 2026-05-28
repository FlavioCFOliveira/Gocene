// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecbridge installs the production Lucene 10.4 codec as the
// default Codec for the index package.
//
// # Why this package exists (post SPI unification)
//
// Sprint 118 (rmp #4693, #4706) lifted every codec-facing SPI — Codec,
// PostingsFormat, StoredFieldsFormat, FieldInfosFormat,
// SegmentInfoFormat, SegmentInfosFormat, TermVectorsFormat,
// CompoundFormat, and the SegmentReadState / SegmentWriteState structs —
// into a dedicated spi/ package. Both index/ and codecs/ now alias
// those types directly, so the per-format adapters that used to
// translate between the two interface families collapse to identity
// wrappers. This package therefore shrinks to:
//
//   - A bridgeCodec that wires the one remaining deferred member
//     (KnnVectorsFormat via the index-side KnnVectorsFormatFactory
//     abstraction) onto the unified spi.Codec surface that
//     codecs.Lucene104Codec already satisfies.
//   - An init() hook that registers the bridge as the default and
//     publishes the temporary stored-fields format that
//     SortingStoredFieldsConsumer uses for in-RAM reordering.
//
// The package will be deleted entirely once rmp #4707
// (KnnVectorsFormat lift) closes.
//
// # Usage
//
// Production callers blank-import this package to install the bridge
// as the index-side default:
//
//	import _ "github.com/FlavioCFOliveira/Gocene/internal/codecbridge"
package codecbridge

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	corecompressing "github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	l90compressing "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// init installs the production Lucene 10.4 codec as the default codec
// resolved by index.NewIndexWriterConfig, and also registers it under
// its canonical name so that OpenDirectoryReader can resolve it by the
// codec name stored in each segment's SegmentInfo on disk.
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
	// After SPI unification the StoredFieldsFormat surface is identical
	// on both sides; the temp format value satisfies the index alias
	// without any wrapping.
	index.RegisterDefaultTempStoredFieldsFormat(tempStored)

	// Lucene90CompressingTermVectorsFormat in the Gocene port is currently a
	// stub that does not accept tuning options; once it gains the 5-arg
	// constructor matching the Java reference this hook switches to the
	// canonical ("TempTermVectors", NO_COMPRESSION, 128 KB, 1, 10) tuple.
	// In the meantime, leaving DefaultTempTermVectorsFormat unset keeps
	// SortingTermVectorsConsumer surfacing ErrTempTermVectorsFormatUnset on
	// the first use rather than producing a silently-wrong segment.
	_ = l90compressing.NewLucene90CompressingTermVectorsFormat
}

// BridgeForLucene104 returns an index.Codec that delegates to the
// production Lucene104Codec defined in package codecs.
//
// After SPI unification (rmp #4693 / #4706) the inner codecs.Codec
// already satisfies spi.Codec directly. The bridge only needs to add
// the one remaining index-side-only accessor (KnnVectorsFormat via its
// factory abstraction) that stays on the deferred path to rmp #4707.
func BridgeForLucene104() index.Codec {
	return NewBridge(codecs.NewLucene104Codec())
}

// NewBridge wraps an arbitrary codecs.Codec in an index.Codec.
// Exported to allow tests and future callers to install custom codecs
// (e.g. Asserting or filtering codec variants) without going through
// the package-init default.
func NewBridge(c codecs.Codec) index.Codec {
	return &bridgeCodec{inner: c}
}

// bridgeCodec embeds the unified spi.Codec surface (which codecs.Codec
// already satisfies thanks to the SPI unification) and adds the one
// accessor that remains on the index-side-only surface pending
// rmp #4707.
type bridgeCodec struct {
	inner codecs.Codec
}

// Compile-time guarantee that bridgeCodec satisfies index.Codec.
var _ index.Codec = (*bridgeCodec)(nil)

func (b *bridgeCodec) Name() string                         { return b.inner.Name() }
func (b *bridgeCodec) PostingsFormat() index.PostingsFormat { return b.inner.PostingsFormat() }
func (b *bridgeCodec) StoredFieldsFormat() index.StoredFieldsFormat {
	return b.inner.StoredFieldsFormat()
}
func (b *bridgeCodec) FieldInfosFormat() index.FieldInfosFormat { return b.inner.FieldInfosFormat() }
func (b *bridgeCodec) SegmentInfoFormat() index.SegmentInfoFormat {
	return b.inner.SegmentInfoFormat()
}
func (b *bridgeCodec) SegmentInfosFormat() index.SegmentInfosFormat {
	return b.inner.SegmentInfosFormat()
}
func (b *bridgeCodec) TermVectorsFormat() index.TermVectorsFormat {
	return b.inner.TermVectorsFormat()
}
func (b *bridgeCodec) CompoundFormat() index.CompoundFormat { return b.inner.CompoundFormat() }

// knnVectorsFormatProvider is an optional interface satisfied by codecs
// (such as Lucene104Codec) that expose a KNN vectors format. The
// bridge type-asserts to this interface so the codecs.Codec interface
// itself does not need to be widened.
type knnVectorsFormatProvider interface {
	KnnVectorsFormat() codecs.KnnVectorsFormat
}

// KnnVectorsFormat implements index.Codec by delegating to the inner
// codecs.Codec when it satisfies knnVectorsFormatProvider. Returns nil
// when the underlying codec does not support KNN vectors.
// TODO(T4707): once KnnVectorsFormat collapses into spi.Codec this
// method moves to the embedded surface.
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
// index.KnnVectorsFormatFactory. Retained verbatim until rmp #4707.
type knnVectorsFormatAdapter struct {
	inner codecs.KnnVectorsFormat
}

// FieldsWriter constructs a KnnVectorsConsumerWriter from the underlying
// codecs.KnnVectorsFormat. After SPI unification the
// SegmentWriteState struct is shared, so no struct conversion is
// required — the same value is passed through.
func (a *knnVectorsFormatAdapter) FieldsWriter(state *index.SegmentWriteState) (index.KnnVectorsConsumerWriter, error) {
	w, err := a.inner.FieldsWriter(state)
	if err != nil {
		return nil, err
	}
	return &knnVectorsConsumerWriterAdapter{inner: w}, nil
}

// knnVectorsConsumerWriterAdapter adapts a codecs.KnnVectorsWriter
// (which exposes AddField / Flush / Finish / Close and RAM accounting)
// to index.KnnVectorsConsumerWriter. The two interfaces are
// structurally similar for the concrete Lucene99HnswVectorsWriter; the
// principal differences are:
//
//   - AddField return type: the codecs writer returns a concrete
//     per-field writer; the index interface requires (any, error) so
//     the concrete value is boxed.
//   - Flush signature: the codecs writer takes Flush(maxDoc int) while
//     the index interface takes Flush(maxDoc int, sortMap
//     index.SorterDocMap). The sortMap is forwarded only when the
//     underlying writer exposes a matching method.
//   - RamBytesUsed: forwarded when the concrete writer implements
//     util.Accountable; zero otherwise.
type knnVectorsConsumerWriterAdapter struct {
	inner codecs.KnnVectorsWriter
}

type knnAddFielder interface {
	AddField(fi *index.FieldInfo) (any, error)
}

type knnFlusherWithSortMap interface {
	Flush(maxDoc int, sortMap index.SorterDocMap) error
}

type knnFlusher interface {
	Flush(maxDoc int) error
}

func (a *knnVectorsConsumerWriterAdapter) AddField(fi *index.FieldInfo) (any, error) {
	if af, ok := a.inner.(knnAddFielder); ok {
		return af.AddField(fi)
	}
	return nil, nil
}

func (a *knnVectorsConsumerWriterAdapter) Flush(maxDoc int, sortMap index.SorterDocMap) error {
	if sm, ok := a.inner.(knnFlusherWithSortMap); ok {
		return sm.Flush(maxDoc, sortMap)
	}
	if f, ok := a.inner.(knnFlusher); ok {
		return f.Flush(maxDoc)
	}
	return nil
}

func (a *knnVectorsConsumerWriterAdapter) Finish() error { return a.inner.Finish() }
func (a *knnVectorsConsumerWriterAdapter) Close() error  { return a.inner.Close() }

func (a *knnVectorsConsumerWriterAdapter) RamBytesUsed() int64 {
	if acc, ok := a.inner.(util.Accountable); ok {
		return acc.RamBytesUsed()
	}
	return 0
}
