// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// init registers the temporary stored-fields format used by
// SortingStoredFieldsConsumer.
//
// Before Sprint 118 / rmp #4696 this registration lived in the
// internal/codecbridge bridge package. Moving it into the compressing
// package, where the format is defined, keeps the registrations close to
// their constructors and eliminates the need for a dedicated bridge.
func init() {
	tempStored := NewLucene90CompressingStoredFieldsFormatWithOptions(
		"TempStoredFields",
		compressing.NO_COMPRESSION,
		128*1024, 1, 10,
	)
	index.RegisterDefaultTempStoredFieldsFormat(tempStored)

	// Arms codecs.NewLucene90CompressingStoredFieldsFormat, the spelling of
	// this package's constructor that package codecs needs but cannot reach
	// directly (this package imports codecs).
	gcodecs.RegisterLucene90CompressingStoredFieldsFormat(
		func(opts gcodecs.Lucene90CompressingStoredFieldsFormatOptions) (gcodecs.StoredFieldsFormat, error) {
			return NewLucene90CompressingStoredFieldsFormatWithOptions(
				opts.FormatName,
				opts.CompressionMode,
				opts.ChunkSize,
				opts.MaxDocsPerChunk,
				opts.BlockShift,
			), nil
		})

	// Lucene90CompressingTermVectorsFormat in the Gocene port is currently a
	// stub that does not accept tuning options; once it gains the 5-arg
	// constructor matching the Java reference this hook switches to the
	// canonical ("TempTermVectors", NO_COMPRESSION, 128 KB, 1, 10) tuple.
	// In the meantime, leaving DefaultTempTermVectorsFormat unset keeps
	// SortingTermVectorsConsumer surfacing ErrTempTermVectorsFormatUnset on
	// the first use rather than producing a silently-wrong segment.
	_ = NewLucene90CompressingTermVectorsFormat
}
