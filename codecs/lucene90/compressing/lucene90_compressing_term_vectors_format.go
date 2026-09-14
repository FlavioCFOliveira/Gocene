// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"fmt"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene90CompressingTermVectorsFormat is a TermVectorsFormat that compresses
// chunks of documents together in order to improve the compression ratio.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsFormat
// of Apache Lucene 10.5.0.
type Lucene90CompressingTermVectorsFormat struct {
	formatName      string
	segmentSuffix   string
	compressionMode compressing.CompressionMode
	chunkSize       int
	blockSize       int
	maxDocsPerChunk int
}

// NewLucene90CompressingTermVectorsFormat creates a new
// Lucene90CompressingTermVectorsFormat.
//
// formatName is the name of the format, which is written into the header of
// the vectors file; segmentSuffix is appended to the segment name of every
// file this format produces; compressionMode compresses the term and payload
// bytes; chunkSize is the minimum byte size of a chunk of documents;
// maxDocsPerChunk is the maximum number of documents in a chunk; and
// blockSize is the number of chunks to store in an index block.
//
// Java throws IllegalArgumentException when chunkSize or blockSize is below 1;
// the Go signature carries no error, so both checks panic, which is how this
// module renders IllegalArgumentException from a constructor.
func NewLucene90CompressingTermVectorsFormat(
	formatName, segmentSuffix string,
	compressionMode compressing.CompressionMode,
	chunkSize, maxDocsPerChunk, blockSize int,
) *Lucene90CompressingTermVectorsFormat {
	f := &Lucene90CompressingTermVectorsFormat{
		formatName:      formatName,
		segmentSuffix:   segmentSuffix,
		compressionMode: compressionMode,
	}
	if chunkSize < 1 {
		panic("chunkSize must be >= 1")
	}
	f.chunkSize = chunkSize
	f.maxDocsPerChunk = maxDocsPerChunk
	if blockSize < 1 {
		panic("blockSize must be >= 1")
	}
	f.blockSize = blockSize
	return f
}

// Name returns the simple class name. org.apache.lucene.codecs.TermVectorsFormat
// has no name accessor; the spi surface requires one and this renders
// getClass().getSimpleName(), the name toString() prints.
func (f *Lucene90CompressingTermVectorsFormat) Name() string {
	return "Lucene90CompressingTermVectorsFormat"
}

// VectorsReader returns a Lucene90CompressingTermVectorsReader over the
// segment's term vectors files. Mirrors
// Lucene90CompressingTermVectorsFormat.vectorsReader.
func (f *Lucene90CompressingTermVectorsFormat) VectorsReader(
	directory store.Directory,
	segmentInfo *index.SegmentInfo,
	fieldInfos *index.FieldInfos,
	context store.IOContext,
) (gcodecs.TermVectorsReader, error) {
	return newLucene90CompressingTermVectorsReader(
		directory, segmentInfo, f.segmentSuffix, fieldInfos, context, f.formatName, f.compressionMode)
}

// VectorsWriter returns a Lucene90CompressingTermVectorsWriter for the
// segment described by state. Mirrors
// Lucene90CompressingTermVectorsFormat.vectorsWriter(Directory, SegmentInfo,
// IOContext); the spi surface passes the three arguments inside the
// SegmentWriteState.
func (f *Lucene90CompressingTermVectorsFormat) VectorsWriter(
	state *gcodecs.SegmentWriteState,
) (gcodecs.TermVectorsWriter, error) {
	return newLucene90CompressingTermVectorsWriter(
		state.Directory,
		state.SegmentInfo,
		f.segmentSuffix,
		state.Context,
		f.formatName,
		f.compressionMode,
		f.chunkSize,
		f.maxDocsPerChunk,
		f.blockSize)
}

// String mirrors Lucene90CompressingTermVectorsFormat.toString().
func (f *Lucene90CompressingTermVectorsFormat) String() string {
	return fmt.Sprintf("%s(compressionMode=%v, chunkSize=%d, maxDocsPerChunk=%d, blockSize=%d)",
		"Lucene90CompressingTermVectorsFormat", f.compressionMode, f.chunkSize, f.maxDocsPerChunk, f.blockSize)
}

var _ gcodecs.TermVectorsFormat = (*Lucene90CompressingTermVectorsFormat)(nil)
