// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import "github.com/FlavioCFOliveira/Gocene/index"

// init installs the production Lucene 10.4 codec as the default codec
// resolved by index.NewIndexWriterConfig.
//
// Before Sprint 118 / rmp #4696 this registration lived in the
// internal/codecbridge package. Moving it directly into codecs/
// removes the last indirection now that SPI unification (rmp #4693,
// #4706, #4707, #4708) has collapsed every codec-facing interface
// into spi/ — index.Codec is a type alias of spi.Codec, and
// *codecs.Lucene104Codec satisfies it directly.
//
// The temporary stored-fields format used by SortingStoredFieldsConsumer
// is registered separately in codecs/lucene90/compressing/register.go.

// Compile-time assertion that the production codec directly satisfies the
// unified index.Codec contract without any bridge adapter. index.Codec is a
// type alias of spi.Codec after the SPI unification (rmp #4693/#4706/#4707/
// #4708), so this single declaration proves rmp #4696 acceptance criterion 3
// and breaks the build if the codec ever drifts from the SPI surface.
var _ index.Codec = (*Lucene104Codec)(nil)

func init() {
	codec := NewLucene104Codec()
	index.RegisterDefaultCodec(codec)
	index.RegisterNamedCodec(codec.Name(), codec)
	// The per-segment .si reader hook used by spi.ReadSegmentInfos to load the
	// authoritative docCount/metadata (rmp #4785) is registered from package
	// index (see index.init in index_writer.go), so it is always installed
	// whenever the index reader/writer is used — no codecs import required.
}
