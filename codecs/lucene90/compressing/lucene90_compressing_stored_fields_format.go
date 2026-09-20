// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version
//	2.0 (the "License"); you may not use this file except in compliance
//	with the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//	implied. See the License for the specific language governing
//	permissions and limitations under the License.

package compressing

import (
	"fmt"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene90CompressingStoredFieldsFormat mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsFormat.
//
// The on-disk chunk layout (LZ4/Deflate compressed sub-blocks with a
// monotonic per-block index) is fully implemented. The type is
// configurable so callers such as Lucene90StoredFieldsFormat can
// supply the BEST_SPEED / BEST_COMPRESSION preset parameters.
type Lucene90CompressingStoredFieldsFormat struct {
	formatName      string
	compressionMode compressing.CompressionMode
	chunkSize       int
	maxDocsPerChunk int
	blockShift      int
}

// NewLucene90CompressingStoredFieldsFormat builds a
// Lucene90CompressingStoredFieldsFormat with zero-valued tuning
// parameters. The constructor is preserved for backwards compatibility
// with the original Sprint 48 stub; prefer
// NewLucene90CompressingStoredFieldsFormatWithOptions when the preset
// values are known.
func NewLucene90CompressingStoredFieldsFormat() *Lucene90CompressingStoredFieldsFormat {
	return &Lucene90CompressingStoredFieldsFormat{}
}

// NewLucene90CompressingStoredFieldsFormatWithOptions builds a
// Lucene90CompressingStoredFieldsFormat configured for a specific mode.
//
// It is the Go counterpart of the 5-arg Java constructor
// Lucene90CompressingStoredFieldsFormat(String, CompressionMode, int,
// int, int).
func NewLucene90CompressingStoredFieldsFormatWithOptions(
	formatName string,
	compressionMode compressing.CompressionMode,
	chunkSize, maxDocsPerChunk, blockShift int,
) *Lucene90CompressingStoredFieldsFormat {
	return &Lucene90CompressingStoredFieldsFormat{
		formatName:      formatName,
		compressionMode: compressionMode,
		chunkSize:       chunkSize,
		maxDocsPerChunk: maxDocsPerChunk,
		blockShift:      blockShift,
	}
}

// FormatName returns the on-disk format tag (one of
// "Lucene90StoredFieldsFastData" / "Lucene90StoredFieldsHighData").
func (f *Lucene90CompressingStoredFieldsFormat) FormatName() string { return f.formatName }

// CompressionMode returns the configured CompressionMode singleton.
func (f *Lucene90CompressingStoredFieldsFormat) CompressionMode() compressing.CompressionMode {
	return f.compressionMode
}

// ChunkSize returns the target uncompressed chunk size in bytes.
func (f *Lucene90CompressingStoredFieldsFormat) ChunkSize() int { return f.chunkSize }

// MaxDocsPerChunk returns the cap on documents per stored-fields chunk.
func (f *Lucene90CompressingStoredFieldsFormat) MaxDocsPerChunk() int { return f.maxDocsPerChunk }

// BlockShift returns the per-fields-index block shift (one block per
// 1 << blockShift chunks).
func (f *Lucene90CompressingStoredFieldsFormat) BlockShift() int { return f.blockShift }

// Name implements [gcodecs.StoredFieldsFormat].
func (f *Lucene90CompressingStoredFieldsFormat) Name() string {
	return "Lucene90CompressingStoredFieldsFormat"
}

// FieldsReader opens the stored-fields reader for the given segment.
func (f *Lucene90CompressingStoredFieldsFormat) FieldsReader(
	dir store.Directory,
	si *index.SegmentInfo,
	fn *index.FieldInfos,
	ctx store.IOContext,
) (gcodecs.StoredFieldsReader, error) {
	if f.formatName == "" {
		return nil, fmt.Errorf(
			"lucene90/compressing: FieldsReader called on zero-value format; use NewLucene90CompressingStoredFieldsFormatWithOptions",
		)
	}
	return newLucene90CompressingStoredFieldsReader(dir, si, fn, ctx, f.formatName, f.compressionMode)
}

// FieldsWriter opens the stored-fields writer for the given segment.
func (f *Lucene90CompressingStoredFieldsFormat) FieldsWriter(
	dir store.Directory,
	si *index.SegmentInfo,
	ctx store.IOContext,
) (gcodecs.StoredFieldsWriter, error) {
	if f.formatName == "" {
		return nil, fmt.Errorf(
			"lucene90/compressing: FieldsWriter called on zero-value format; use NewLucene90CompressingStoredFieldsFormatWithOptions",
		)
	}
	return newLucene90CompressingStoredFieldsWriter(
		dir, si, ctx, f.formatName, f.compressionMode, f.chunkSize, f.maxDocsPerChunk, f.blockShift,
	)
}

// Compile-time guarantee.
var _ gcodecs.StoredFieldsFormat = (*Lucene90CompressingStoredFieldsFormat)(nil)

// ---------------------------------------------------------------------------
// in-memory fields index builder (replaces Lucene's FieldsIndexWriter)
// ---------------------------------------------------------------------------
