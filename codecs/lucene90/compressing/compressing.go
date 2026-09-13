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

// Package compressing hosts the Lucene90 compressing stored-fields and
// term-vectors codecs. It is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.
//
// # Wire format
//
// The package produces three files per segment:
//
//   - .fdt  — field data, IndexHeader(formatName, VERSION_CURRENT) + chunks +
//     IndexFooter. Each chunk: docBase(VInt) + numDocs<<2|flags(VInt) +
//     numStoredFields per doc (StoredFieldsInts) + lengths per doc
//     (StoredFieldsInts) + compressed payload.
//
//   - .fdm  — metadata, IndexHeader("Lucene90FieldsIndexMeta", VERSION_CURRENT)
//
//   - chunkSize(VInt) + numDocs(Int) + blockShift(Int) + (totalChunks+1)(Int)
//
//   - DirectMonotonicWriter(docs) data offsets/lengths
//
//   - DirectMonotonicWriter(startPointers) data offsets/lengths
//
//   - maxPointer(Long) + numChunks(VLong) + numDirtyChunks(VLong)
//
//   - numDirtyDocs(VLong) + IndexFooter.
//
//   - .fdx  — index, IndexHeader("Lucene90FieldsIndexIdx", VERSION_CURRENT) +
//     DirectMonotonic data blobs + IndexFooter.
//
// # Field encoding in .fdt chunks
//
// Each stored field is prefixed with a VLong of (fieldNumber << TYPE_BITS | type)
// where fieldNumber is the 0-based per-document sequential ID assigned by the
// writer in place of the FieldInfo.number that Lucene uses. Numeric values use
// ZInt/TLong/ZFloat/ZDouble encodings. Strings and binary blobs are prefixed
// with a VInt length.
//
// The reader resolves field names from the embedded field number via the
// FieldInfos passed at construction time (identical to Lucene's approach).
// When FieldInfos is nil the field name defaults to the empty string.
//
// For segments whose fields appear in the same order across every document and
// whose FieldInfo numbers are assigned 0, 1, 2, … in that order (the common
// case for newly created segments), Gocene's sequential per-document IDs
// match Lucene's FieldInfo.number values and the .fdt bytes are byte-identical.
//
// # Divergences from Lucene 10.4.0
//
//   - Gocene's StoredFieldsWriter.WriteField receives an IndexableField with
//     no FieldInfo, so sequential 0-based field IDs are used instead of the
//     FieldInfo.number values that Lucene stamps into infoAndBits. For segments
//     where FieldInfo numbers are dense 0-based integers matching the write
//     order, the output is byte-identical to Lucene's.
//
//   - The FieldsIndexWriter does not use CreateTempOutput (not part of
//     Gocene's Directory interface). Doc-count and start-pointer streams are
//     buffered in memory via ByteBuffersDataOutput and flushed into the .fdx
//     at finish time.
//
//   - Merge (bulk chunk copy) is not implemented; the writer always flushes
//     document-by-document. Bulk merge is an optimisation and does not affect
//     correctness.
package compressing

// Lucene90CompressingTermVectorsFormat mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsFormat.
type Lucene90CompressingTermVectorsFormat struct{}

// NewLucene90CompressingTermVectorsFormat builds a
// Lucene90CompressingTermVectorsFormat.
func NewLucene90CompressingTermVectorsFormat() *Lucene90CompressingTermVectorsFormat {
	return &Lucene90CompressingTermVectorsFormat{}
}

// Lucene90CompressingTermVectorsReader mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsReader.
type Lucene90CompressingTermVectorsReader struct{}

// NewLucene90CompressingTermVectorsReader builds a
// Lucene90CompressingTermVectorsReader.
func NewLucene90CompressingTermVectorsReader() *Lucene90CompressingTermVectorsReader {
	return &Lucene90CompressingTermVectorsReader{}
}

// Lucene90CompressingTermVectorsWriter mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsWriter.
type Lucene90CompressingTermVectorsWriter struct{}

// NewLucene90CompressingTermVectorsWriter builds a
// Lucene90CompressingTermVectorsWriter.
func NewLucene90CompressingTermVectorsWriter() *Lucene90CompressingTermVectorsWriter {
	return &Lucene90CompressingTermVectorsWriter{}
}
