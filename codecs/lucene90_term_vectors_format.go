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

package codecs

// Lucene 9.0 term vectors format constants. Mirrors
// org.apache.lucene.codecs.lucene90.Lucene90TermVectorsFormat which extends
// Lucene90CompressingTermVectorsFormat with a fixed parameter tuple.
//
// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene90/
// Lucene90TermVectorsFormat.java (Lucene 10.4.0).
//
// File layout (three files per segment):
//
//   - .tvm (vector metadata): IndexHeader, PackedIntsVersion (VInt),
//     ChunkSize (VInt), ChunkIndexMetadata (FieldsIndexWriter), ChunkCount
//     (VLong), DirtyChunkCount (VLong), DirtyDocsCount (VLong), Footer.
//   - .tvd (vector data):     IndexHeader, Chunk * ChunkCount, Footer. Each
//     chunk holds DocBase, ChunkDocs, NumFields, FieldNums, FieldNumOffs,
//     Flags, NumTerms, TermLengths, TermFreqs, Positions, StartOffsets,
//     Lengths, PayloadLengths and the LZ4-compressed TermAndPayloads blob.
//   - .tvx (vector index):    IndexHeader, ChunkIndex (FieldsIndexWriter),
//     Footer.
//
// Terms and payloads are compressed with LZ4 (FAST); packed-int blocks of
// 64 values back numeric streams (PrefixLength, SuffixLength,
// TermFreqMinus1, PositionDelta, StartOffsetDelta, LengthMinusTermLength,
// PayloadLength). AvgCharsPerTerm is emitted only when at least one field
// has both positions and offsets.
const (
	// Lucene90TermVectorsFormatName is the codec name embedded in the .tvd
	// header (see CodecUtil.writeIndexHeader). Mirrors the Java constant
	// "Lucene90TermVectorsData".
	Lucene90TermVectorsFormatName = "Lucene90TermVectorsData"

	// Lucene90TermVectorsFormatSegmentSuffix is appended to file names
	// produced by this format (empty for the base Lucene90 codec).
	Lucene90TermVectorsFormatSegmentSuffix = ""

	// Lucene90TermVectorsFormatChunkSize is the minimum byte size of a
	// chunk of accumulated term-vector payload before a flush. Matches
	// 1 << 12 in Java.
	Lucene90TermVectorsFormatChunkSize = 1 << 12

	// Lucene90TermVectorsFormatMaxDocsPerChunk caps the number of
	// documents per chunk before a forced flush. Matches 128 in Java.
	Lucene90TermVectorsFormatMaxDocsPerChunk = 128

	// Lucene90TermVectorsFormatBlockShift is the number of chunks stored
	// per index block (FieldsIndexWriter). Matches 10 in Java.
	Lucene90TermVectorsFormatBlockShift = 10
)

// Lucene90TermVectorsFormat is the Lucene 9.0 term-vectors format. It is a
// thin wrapper over [CompressingTermVectorsFormat] with the parameter
// tuple ("Lucene90TermVectorsData", "", FAST, 1<<12, 128, 10) hardcoded by
// the Java sub-class.
//
// The embedded [CompressingTermVectorsFormat] supplies the chunked LZ4
// writer/reader implementation; this type pins the byte-compatible
// constants required by the Lucene 9.0 codec family. The block-shift
// parameter (FieldsIndexWriter granularity) is captured here for later
// wiring once the index file (.tvx) is fully ported.
//
// Note (divergence): the Gocene [CompressingTermVectorsFormat] does not
// yet implement the FieldsIndexWriter-backed .tvx layout described in the
// Java javadoc. This type therefore reaches byte-compatibility only for
// the .tvm/.tvd payload structure handled by [CompressingTermVectorsWriter];
// the .tvx file is emitted by the existing simplified writer until the
// full FieldsIndexWriter port lands. The block-shift constant is recorded
// for that follow-up.
type Lucene90TermVectorsFormat struct {
	*CompressingTermVectorsFormat
	blockShift int
}

// NewLucene90TermVectorsFormat builds a Lucene 9.0 term-vectors format
// with the canonical Java parameter tuple.
func NewLucene90TermVectorsFormat() *Lucene90TermVectorsFormat {
	// Java passes CompressionMode.FAST (LZ4 fast). The top-level codecs
	// package exposes the same algorithm as CompressionModeLZ4Fast.
	base := NewCompressingTermVectorsFormat(
		CompressionModeLZ4Fast,
		Lucene90TermVectorsFormatChunkSize,
		Lucene90TermVectorsFormatMaxDocsPerChunk,
	)
	base.BaseTermVectorsFormat = NewBaseTermVectorsFormat(Lucene90TermVectorsFormatName)
	return &Lucene90TermVectorsFormat{
		CompressingTermVectorsFormat: base,
		blockShift:                   Lucene90TermVectorsFormatBlockShift,
	}
}

// BlockShift returns the number of chunks stored per index block (the
// FieldsIndexWriter granularity). Matches the Java blockSize constructor
// argument.
func (f *Lucene90TermVectorsFormat) BlockShift() int {
	return f.blockShift
}

// SegmentSuffix returns the suffix appended to files produced by this
// format. Empty for the base Lucene 9.0 codec; mirrors the Java
// segmentSuffix constructor argument.
func (f *Lucene90TermVectorsFormat) SegmentSuffix() string {
	return Lucene90TermVectorsFormatSegmentSuffix
}
