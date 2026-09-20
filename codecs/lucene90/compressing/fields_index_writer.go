// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
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
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

const (
	// fieldsIndexWriterVersionStart is FieldsIndexWriter.VERSION_START
	// (FieldsIndexWriter.java:47).
	fieldsIndexWriterVersionStart = int32(0)

	// fieldsIndexWriterVersionCurrent is FieldsIndexWriter.VERSION_CURRENT
	// (FieldsIndexWriter.java:48).
	fieldsIndexWriterVersionCurrent = int32(0)
)

// FieldsIndexWriter is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.FieldsIndexWriter
// (Lucene 10.5.0, FieldsIndexWriter.java, 201 lines), whose javadoc reads:
//
//	Efficient index format for block-based Codecs.
//
//	For each block of compressed stored fields, this stores the first
//	document of the block and the start pointer of the block in a
//	DirectMonotonicWriter. At read time, the docID is binary-searched in the
//	DirectMonotonicReader that records doc IDS, and the returned index is
//	used to look up the start pointer in the DirectMonotonicReader that
//	records start pointers.
//
// # Divergence: the two scratch streams
//
// Java buffers the per-chunk doc counts and start-pointer deltas in two
// temporary files obtained from Directory#createTempOutput, each framed with
// its own header and footer, and reads them back through
// Directory#openChecksumInput in finish() before deleting them. Gocene's
// spi.Directory declares neither createTempOutput nor openChecksumInput, so
// the two sequences are buffered in memory instead.
//
// This changes nothing observable: the temp files never belong to the
// segment, are deleted before finish() returns, and contribute no byte to
// either the .fdx or the .fdm. The values written to the DirectMonotonicWriters
// are the same values in the same order, so both artefacts are byte-identical
// to Lucene's. What is lost is Lucene's own corruption check on its scratch
// data (checkHeader/checkFooter over the temp files); the two invariants those
// guarded — "Docs don't add up" and "File pointers don't add up" — are
// reproduced below against the in-memory sequences.
type FieldsIndexWriter struct {
	dir        store.Directory
	name       string
	suffix     string
	extension  string
	codecName  string
	id         []byte
	blockShift int
	ioContext  store.IOContext

	// docsOut and filePointersOut stand in for the two Java temp streams.
	// docsOut holds one VInt-equivalent doc count per chunk;
	// filePointersOut holds one VLong-equivalent start-pointer delta.
	docsOut         []int32
	filePointersOut []int64

	totalDocs   int32
	totalChunks int
	previousFP  int64

	closed bool
}

// NewFieldsIndexWriter is the FieldsIndexWriter sole constructor
// (FieldsIndexWriter.java:64-92). Java creates and frames the two temporary
// streams here; the Go form has nothing to open, for the reason given on the
// type.
func NewFieldsIndexWriter(
	dir store.Directory,
	name, suffix, extension, codecName string,
	id []byte,
	blockShift int,
	ioContext store.IOContext,
) (*FieldsIndexWriter, error) {
	return &FieldsIndexWriter{
		dir:        dir,
		name:       name,
		suffix:     suffix,
		extension:  extension,
		codecName:  codecName,
		id:         id,
		blockShift: blockShift,
		ioContext:  ioContext,
	}, nil
}

// WriteIndex is FieldsIndexWriter#writeIndex(int, long)
// (FieldsIndexWriter.java:94-101):
//
//	assert startPointer >= previousFP;
//	docsOut.writeVInt(numDocs);
//	filePointersOut.writeVLong(startPointer - previousFP);
//	previousFP = startPointer;
//	totalDocs += numDocs;
//	totalChunks++;
//
// Java's assert is disabled in production builds; here the condition is a
// returned error, because a negative delta would be written as an enormous
// VLong and silently corrupt the index rather than trip an assertion.
func (w *FieldsIndexWriter) WriteIndex(numDocs int, startPointer int64) error {
	if startPointer < w.previousFP {
		return fmt.Errorf(
			"lucene90/compressing: FieldsIndexWriter: startPointer %d < previousFP %d",
			startPointer, w.previousFP,
		)
	}
	w.docsOut = append(w.docsOut, int32(numDocs))
	w.filePointersOut = append(w.filePointersOut, startPointer-w.previousFP)
	w.previousFP = startPointer
	w.totalDocs += int32(numDocs)
	w.totalChunks++
	return nil
}

// Finish is FieldsIndexWriter#finish(int, long, IndexOutput)
// (FieldsIndexWriter.java:103-175). It writes the .fdx data file in full and
// appends this index's metadata to the caller's meta stream.
func (w *FieldsIndexWriter) Finish(numDocs int, maxPointer int64, metaOut store.IndexOutput) error {
	if int(w.totalDocs) != numDocs {
		// Java: throw new IllegalStateException("Expected " + numDocs + " docs, but got " + totalDocs);
		return fmt.Errorf(
			"lucene90/compressing: FieldsIndexWriter: Expected %d docs, but got %d",
			numDocs, w.totalDocs,
		)
	}

	// try (IndexOutput dataOut = dir.createOutput(
	//        IndexFileNames.segmentFileName(name, suffix, extension), ioContext)) {
	//
	// The output is wrapped in a checksum tracker because Gocene's directory
	// outputs do not maintain a running CRC of their own, and CodecUtil's
	// footer has to record one. The wrapper adds no byte to the stream.
	fdxName := store.SegmentFileName(w.name, w.suffix, w.extension)
	raw, err := w.dir.CreateOutput(fdxName, w.ioContext)
	if err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter create %s: %w", fdxName, err)
	}
	dataOut := store.NewChecksumIndexOutput(raw)
	dataOutClosed := false
	defer func() {
		if !dataOutClosed {
			_ = dataOut.Close()
		}
	}()

	if err := gcodecs.WriteIndexHeader(
		dataOut, w.codecName+"Idx", fieldsIndexWriterVersionCurrent, w.id, w.suffix,
	); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter write %s header: %w", fdxName, err)
	}

	if err := metaOut.WriteInt(int32(numDocs)); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter meta numDocs: %w", err)
	}
	if err := metaOut.WriteInt(int32(w.blockShift)); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter meta blockShift: %w", err)
	}
	if err := metaOut.WriteInt(int32(w.totalChunks + 1)); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter meta numChunks: %w", err)
	}
	if err := metaOut.WriteLong(dataOut.GetFilePointer()); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter meta docsStartPointer: %w", err)
	}

	// --- doc IDs ---
	docs, err := packed.NewDirectMonotonicWriter(metaOut, dataOut, int64(w.totalChunks+1), w.blockShift)
	if err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter docs monotonic writer: %w", err)
	}
	var doc int64
	if err := docs.Add(doc); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter docs add: %w", err)
	}
	for i := 0; i < w.totalChunks; i++ {
		doc += int64(w.docsOut[i])
		if err := docs.Add(doc); err != nil {
			return fmt.Errorf("lucene90/compressing: FieldsIndexWriter docs add: %w", err)
		}
	}
	if err := docs.Finish(); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter docs finish: %w", err)
	}
	if doc != int64(w.totalDocs) {
		// Java: throw new CorruptIndexException("Docs don't add up", docsIn);
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter: Docs don't add up (%d != %d)", doc, w.totalDocs)
	}
	w.docsOut = nil

	if err := metaOut.WriteLong(dataOut.GetFilePointer()); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter meta startPointersStartPointer: %w", err)
	}

	// --- start pointers ---
	filePointers, err := packed.NewDirectMonotonicWriter(metaOut, dataOut, int64(w.totalChunks+1), w.blockShift)
	if err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter filePointers monotonic writer: %w", err)
	}
	var fp int64
	for i := 0; i < w.totalChunks; i++ {
		fp += w.filePointersOut[i]
		if err := filePointers.Add(fp); err != nil {
			return fmt.Errorf("lucene90/compressing: FieldsIndexWriter filePointers add: %w", err)
		}
	}
	if maxPointer < fp {
		// Java: throw new CorruptIndexException("File pointers don't add up", filePointersIn);
		return fmt.Errorf(
			"lucene90/compressing: FieldsIndexWriter: File pointers don't add up (maxPointer %d < %d)",
			maxPointer, fp,
		)
	}
	if err := filePointers.Add(maxPointer); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter filePointers add maxPointer: %w", err)
	}
	if err := filePointers.Finish(); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter filePointers finish: %w", err)
	}
	w.filePointersOut = nil

	if err := metaOut.WriteLong(dataOut.GetFilePointer()); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter meta startPointersEndPointer: %w", err)
	}
	if err := metaOut.WriteLong(maxPointer); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter meta maxPointer: %w", err)
	}

	if err := gcodecs.WriteFooter(dataOut); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter write %s footer: %w", fdxName, err)
	}

	dataOutClosed = true
	if err := dataOut.Close(); err != nil {
		return fmt.Errorf("lucene90/compressing: FieldsIndexWriter close %s: %w", fdxName, err)
	}
	return nil
}

// Close is FieldsIndexWriter#close (FieldsIndexWriter.java:177-198). Java
// closes the two temp streams and deletes their files; the Go form releases
// the in-memory sequences that stand in for them.
func (w *FieldsIndexWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	w.docsOut = nil
	w.filePointersOut = nil
	return nil
}

// Compile-time guarantee that FieldsIndexWriter is Closeable, as the Java
// class declares.
var _ interface{ Close() error } = (*FieldsIndexWriter)(nil)
