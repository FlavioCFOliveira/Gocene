// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Ported from Apache Lucene 10.5.0:
//
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/lucene50/compressing/Lucene50CompressingStoredFieldsFormat.java

package compressing

import (
	"errors"
	"fmt"

	bccompressing "github.com/FlavioCFOliveira/Gocene/backward_codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// legacyDirectMonotonicMinBlockShift and legacyDirectMonotonicMaxBlockShift
// mirror LegacyDirectMonotonicWriter.MIN_BLOCK_SHIFT / MAX_BLOCK_SHIFT, the
// bounds the constructor validates blockShift against.
const (
	legacyDirectMonotonicMinBlockShift = 2
	legacyDirectMonotonicMaxBlockShift = 22
)

// ErrOldFormatsReadOnly renders
// {@code throw new UnsupportedOperationException("Old formats can't be used
// for writing")}, which fieldsWriter raises.
var ErrOldFormatsReadOnly = errors.New("Old formats can't be used for writing")

// Lucene50CompressingStoredFieldsFormat is a StoredFieldsFormat that
// compresses documents in chunks in order to improve the compression ratio.
//
// For a chunk size of chunkSize bytes, this StoredFieldsFormat does not
// support documents larger than (2^31 - chunkSize) bytes.
//
// For optimal performance, you should use a MergePolicy that returns segments
// that have the biggest byte size first.
//
// Mirrors {@code public class Lucene50CompressingStoredFieldsFormat extends
// StoredFieldsFormat} (Lucene 10.5.0, @lucene.experimental).
type Lucene50CompressingStoredFieldsFormat struct {
	// formatName mirrors {@code protected final String formatName}.
	formatName string
	// segmentSuffix mirrors {@code protected final String segmentSuffix}.
	segmentSuffix string
	// compressionMode mirrors {@code protected final CompressionMode compressionMode}.
	compressionMode bccompressing.CompressionMode
	// chunkSize mirrors {@code protected final int chunkSize}.
	chunkSize int
	// maxDocsPerChunk mirrors {@code protected final int maxDocsPerChunk}.
	maxDocsPerChunk int
	// blockShift mirrors {@code protected final int blockShift}.
	blockShift int
}

// NewLucene50CompressingStoredFieldsFormat creates a new
// Lucene50CompressingStoredFieldsFormat with an empty segment suffix.
//
// Mirrors the five-argument constructor, whose body is
// {@code this(formatName, "", compressionMode, chunkSize, maxDocsPerChunk, blockShift);}.
func NewLucene50CompressingStoredFieldsFormat(
	formatName string,
	compressionMode bccompressing.CompressionMode,
	chunkSize int,
	maxDocsPerChunk int,
	blockShift int,
) (*Lucene50CompressingStoredFieldsFormat, error) {
	return NewLucene50CompressingStoredFieldsFormatWithSuffix(
		formatName, "", compressionMode, chunkSize, maxDocsPerChunk, blockShift)
}

// NewLucene50CompressingStoredFieldsFormatWithSuffix creates a new
// Lucene50CompressingStoredFieldsFormat.
//
// formatName is the name of the format; it is used in the file formats to
// perform CodecUtil codec header checks. segmentSuffix is added to the result
// file name only if it is not the empty string. chunkSize is the minimum byte
// size of a chunk of documents; maxDocsPerChunk is an upper bound on how many
// docs may be stored in a single chunk; blockShift is the log in base 2 of the
// number of chunks to store in an index block.
//
// Mirrors the six-argument constructor.
func NewLucene50CompressingStoredFieldsFormatWithSuffix(
	formatName string,
	segmentSuffix string,
	compressionMode bccompressing.CompressionMode,
	chunkSize int,
	maxDocsPerChunk int,
	blockShift int,
) (*Lucene50CompressingStoredFieldsFormat, error) {
	if chunkSize < 1 {
		return nil, errors.New("chunkSize must be >= 1")
	}
	if maxDocsPerChunk < 1 {
		return nil, errors.New("maxDocsPerChunk must be >= 1")
	}
	if blockShift < legacyDirectMonotonicMinBlockShift || blockShift > legacyDirectMonotonicMaxBlockShift {
		return nil, fmt.Errorf("blockSize must be in %d-%d, got %d",
			legacyDirectMonotonicMinBlockShift, legacyDirectMonotonicMaxBlockShift, blockShift)
	}
	return &Lucene50CompressingStoredFieldsFormat{
		formatName:      formatName,
		segmentSuffix:   segmentSuffix,
		compressionMode: compressionMode,
		chunkSize:       chunkSize,
		maxDocsPerChunk: maxDocsPerChunk,
		blockShift:      blockShift,
	}, nil
}

// Name returns the codec name embedded in segment metadata.
//
// Gocene's spi.StoredFieldsFormat declares Name(); Java's StoredFieldsFormat
// does not, and Lucene identifies the format by formatName in the file
// headers, which is what this returns.
func (f *Lucene50CompressingStoredFieldsFormat) Name() string { return f.formatName }

// FieldsReader reproduces
//
//	return new Lucene50CompressingStoredFieldsReader(
//	    directory, si, segmentSuffix, fn, context, formatName, compressionMode);
func (f *Lucene50CompressingStoredFieldsFormat) FieldsReader(
	directory store.Directory, si *spi.SegmentInfo, fn *spi.FieldInfos, context store.IOContext,
) (spi.StoredFieldsReader, error) {
	return NewLucene50CompressingStoredFieldsReader(
		directory, si, f.segmentSuffix, fn, context, f.formatName, f.compressionMode)
}

// FieldsWriter reproduces
// {@code throw new UnsupportedOperationException("Old formats can't be used for writing");}.
func (f *Lucene50CompressingStoredFieldsFormat) FieldsWriter(
	_ store.Directory, _ *spi.SegmentInfo, _ store.IOContext,
) (spi.StoredFieldsWriter, error) {
	return nil, ErrOldFormatsReadOnly
}

// String reproduces
//
//	return getClass().getSimpleName()
//	    + "(compressionMode=" + compressionMode
//	    + ", chunkSize=" + chunkSize
//	    + ", maxDocsPerChunk=" + maxDocsPerChunk
//	    + ", blockShift=" + blockShift + ")";
func (f *Lucene50CompressingStoredFieldsFormat) String() string {
	return fmt.Sprintf(
		"Lucene50CompressingStoredFieldsFormat(compressionMode=%v, chunkSize=%d, maxDocsPerChunk=%d, blockShift=%d)",
		f.compressionMode, f.chunkSize, f.maxDocsPerChunk, f.blockShift)
}

var _ spi.StoredFieldsFormat = (*Lucene50CompressingStoredFieldsFormat)(nil)
