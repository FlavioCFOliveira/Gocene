// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecbridge installs the production Lucene 10.4 codec as the
// default Codec for the index package.
//
// # Why this package still exists (post SPI unification)
//
// Sprint 118 (rmp #4693, #4706, #4707) lifted every codec-facing SPI —
// Codec, PostingsFormat, StoredFieldsFormat, FieldInfosFormat,
// SegmentInfoFormat, SegmentInfosFormat, TermVectorsFormat,
// CompoundFormat, KnnVectorsFormat, and the SegmentReadState /
// SegmentWriteState structs — into a dedicated spi/ package. Both
// index/ and codecs/ now alias those types directly, so the per-format
// adapters that used to translate between the two interface families
// collapsed to identity wrappers and were removed.
//
// After rmp #4707 closed the KnnVectorsFormat lift, the wide
// codecs.Lucene104Codec already satisfies the full index.Codec surface
// (index.Codec is now a pure alias of spi.Codec). The bridge is
// therefore an identity wrapper retained only to:
//
//   - centralise the init() hook that registers the production codec as
//     the index-side default and publishes the temporary stored-fields
//     format used by SortingStoredFieldsConsumer; and
//   - keep a single ownership boundary that future T4696 work can clean
//     up once the bridge package is removed entirely.
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
// After SPI unification (rmp #4693 / #4706 / #4707) the inner
// codecs.Codec satisfies the full index.Codec surface directly; the
// bridge is now a pure identity wrapper retained for the registration
// hook in init().
func BridgeForLucene104() index.Codec {
	return NewBridge(codecs.NewLucene104Codec())
}

// NewBridge wraps an arbitrary codecs.Codec in an index.Codec.
// Exported to allow tests and future callers to install custom codecs
// (e.g. Asserting or filtering codec variants) without going through
// the package-init default.
//
// After the rmp #4707 KnnVectorsFormat lift the wrapper is an identity
// function: a codecs.Codec already satisfies index.Codec. The exported
// signature is preserved so existing callers compile without churn.
func NewBridge(c codecs.Codec) index.Codec {
	return c
}
