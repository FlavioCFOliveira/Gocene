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

// fieldsIndexReader is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.FieldsIndexReader
// (Lucene 10.5.0, FieldsIndexReader.java, 162 lines), the sole concrete
// FieldsIndex: a pair of DirectMonotonicReaders over the .fdx blobs, with
// their metadata read from the .fdm stream the caller supplies.
//
// Field-for-field against the Java class. Note in particular that numChunks
// holds what the writer stored, which is totalChunks + 1 (see
// FieldsIndexWriter.finish: `metaOut.writeInt(totalChunks + 1)`) — the name
// is Lucene's and the off-by-one is part of the contract, because the
// monotonic arrays carry one sentinel entry past the last chunk.
type fieldsIndexReader struct {
	maxDoc     int32
	blockShift int32
	numChunks  int32

	docsMeta          *packed.DirectMonotonicMeta
	startPointersMeta *packed.DirectMonotonicMeta

	indexInput store.IndexInput

	docsStartPointer          int64
	docsEndPointer            int64
	startPointersStartPointer int64
	startPointersEndPointer   int64

	docs          *packed.DirectMonotonicReader
	startPointers *packed.DirectMonotonicReader

	maxPointer int64
}

// Compile-time guarantee that the port satisfies the abstract class.
var _ fieldsIndex = (*fieldsIndexReader)(nil)

// newFieldsIndexReader is the FieldsIndexReader sole constructor
// (FieldsIndexReader.java:48-89).
//
// The meta stream is positioned by the caller immediately after the
// Lucene90FieldsIndexMeta header and the chunkSize VInt, exactly as
// Lucene90CompressingStoredFieldsReader leaves it.
func newFieldsIndexReader(
	dir store.Directory,
	name, suffix, extension, codecName string,
	id []byte,
	metaIn store.IndexInput,
	ctx store.IOContext,
) (*fieldsIndexReader, error) {
	maxDoc, err := metaIn.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader read maxDoc: %w", err)
	}
	blockShift, err := metaIn.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader read blockShift: %w", err)
	}
	numChunks, err := metaIn.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader read numChunks: %w", err)
	}
	docsStartPointer, err := metaIn.ReadLong()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader read docsStartPointer: %w", err)
	}
	docsMeta, err := packed.LoadDirectMonotonicMeta(metaIn, int64(numChunks), int(blockShift))
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader load docs meta: %w", err)
	}
	// Java: docsEndPointer = startPointersStartPointer = metaIn.readLong();
	docsEndPointer, err := metaIn.ReadLong()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader read docsEndPointer: %w", err)
	}
	startPointersStartPointer := docsEndPointer
	startPointersMeta, err := packed.LoadDirectMonotonicMeta(metaIn, int64(numChunks), int(blockShift))
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader load startPointers meta: %w", err)
	}
	startPointersEndPointer, err := metaIn.ReadLong()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader read startPointersEndPointer: %w", err)
	}
	maxPointer, err := metaIn.ReadLong()
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader read maxPointer: %w", err)
	}

	// indexInput = dir.openInput(IndexFileNames.segmentFileName(name, suffix,
	//                            extension), context.withHints(FileTypeHint.INDEX));
	fdxName := store.SegmentFileName(name, suffix, extension)
	indexInput, err := dir.OpenInput(fdxName, ctx)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader open %s: %w", fdxName, err)
	}
	success := false
	defer func() {
		if !success {
			_ = indexInput.Close()
		}
	}()

	if _, err := gcodecs.CheckIndexHeader(
		indexInput,
		codecName+"Idx",
		fieldsIndexWriterVersionStart,
		fieldsIndexWriterVersionCurrent,
		id,
		suffix,
	); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader check %s header: %w", fdxName, err)
	}
	if _, err := gcodecs.RetrieveChecksum(indexInput); err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader retrieve %s checksum: %w", fdxName, err)
	}

	docsSlice, err := randomAccessSlice(indexInput, "docs", docsStartPointer, docsEndPointer-docsStartPointer)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader slice docs: %w", err)
	}
	startPointersSlice, err := randomAccessSlice(
		indexInput,
		"startPointers",
		startPointersStartPointer,
		startPointersEndPointer-startPointersStartPointer,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader slice startPointers: %w", err)
	}
	docs, err := packed.NewDirectMonotonicReader(docsMeta, docsSlice)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader docs reader: %w", err)
	}
	startPointers, err := packed.NewDirectMonotonicReader(startPointersMeta, startPointersSlice)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader startPointers reader: %w", err)
	}

	success = true
	return &fieldsIndexReader{
		maxDoc:                    maxDoc,
		blockShift:                blockShift,
		numChunks:                 numChunks,
		docsMeta:                  docsMeta,
		startPointersMeta:         startPointersMeta,
		indexInput:                indexInput,
		docsStartPointer:          docsStartPointer,
		docsEndPointer:            docsEndPointer,
		startPointersStartPointer: startPointersStartPointer,
		startPointersEndPointer:   startPointersEndPointer,
		docs:                      docs,
		startPointers:             startPointers,
		maxPointer:                maxPointer,
	}, nil
}

// cloneFieldsIndexReader is the private copy constructor
// FieldsIndexReader(FieldsIndexReader other) (FieldsIndexReader.java:91-109).
// It clones the IndexInput and rebuilds both DirectMonotonicReaders over the
// clone, so the copy can be positioned independently of the original.
func cloneFieldsIndexReader(other *fieldsIndexReader) (*fieldsIndexReader, error) {
	indexInput := other.indexInput.Clone()

	docsSlice, err := randomAccessSlice(
		indexInput, "docs", other.docsStartPointer, other.docsEndPointer-other.docsStartPointer,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader clone slice docs: %w", err)
	}
	startPointersSlice, err := randomAccessSlice(
		indexInput,
		"startPointers",
		other.startPointersStartPointer,
		other.startPointersEndPointer-other.startPointersStartPointer,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader clone slice startPointers: %w", err)
	}
	docs, err := packed.NewDirectMonotonicReader(other.docsMeta, docsSlice)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader clone docs reader: %w", err)
	}
	startPointers, err := packed.NewDirectMonotonicReader(other.startPointersMeta, startPointersSlice)
	if err != nil {
		return nil, fmt.Errorf("lucene90/compressing: FieldsIndexReader clone startPointers reader: %w", err)
	}

	return &fieldsIndexReader{
		maxDoc:                    other.maxDoc,
		numChunks:                 other.numChunks,
		blockShift:                other.blockShift,
		docsMeta:                  other.docsMeta,
		startPointersMeta:         other.startPointersMeta,
		indexInput:                indexInput,
		docsStartPointer:          other.docsStartPointer,
		docsEndPointer:            other.docsEndPointer,
		startPointersStartPointer: other.startPointersStartPointer,
		startPointersEndPointer:   other.startPointersEndPointer,
		maxPointer:                other.maxPointer,
		docs:                      docs,
		startPointers:             startPointers,
	}, nil
}

// Close is FieldsIndexReader#close (FieldsIndexReader.java:111-114).
func (r *fieldsIndexReader) Close() error {
	return r.indexInput.Close()
}

// getBlockID is FieldsIndexReader#getBlockID (FieldsIndexReader.java:116-124).
//
//	Objects.checkIndex(docID, maxDoc);
//	long blockIndex = docs.binarySearch(0, numChunks, docID);
//	if (blockIndex < 0) {
//	  blockIndex = -2 - blockIndex;
//	}
//	return blockIndex;
func (r *fieldsIndexReader) getBlockID(docID int) (int64, error) {
	// Objects.checkIndex(docID, maxDoc) throws IndexOutOfBoundsException.
	if docID < 0 || int32(docID) >= r.maxDoc {
		return 0, fmt.Errorf(
			"lucene90/compressing: FieldsIndexReader: Index %d out of bounds for length %d",
			docID, r.maxDoc,
		)
	}
	blockIndex, err := r.docs.BinarySearch(0, int64(r.numChunks), int64(docID))
	if err != nil {
		return 0, err
	}
	if blockIndex < 0 {
		blockIndex = -2 - blockIndex
	}
	return blockIndex, nil
}

// getBlockStartPointer is FieldsIndexReader#getBlockStartPointer
// (FieldsIndexReader.java:126-129).
func (r *fieldsIndexReader) getBlockStartPointer(blockIndex int64) (int64, error) {
	return r.startPointers.Get(blockIndex)
}

// getBlockLength is FieldsIndexReader#getBlockLength
// (FieldsIndexReader.java:131-140).
//
//	final long endPointer;
//	if (blockIndex == numChunks - 1) {
//	  endPointer = maxPointer;
//	} else {
//	  endPointer = startPointers.get(blockIndex + 1);
//	}
//	return endPointer - getBlockStartPointer(blockIndex);
func (r *fieldsIndexReader) getBlockLength(blockIndex int64) (int64, error) {
	var endPointer int64
	if blockIndex == int64(r.numChunks)-1 {
		endPointer = r.maxPointer
	} else {
		var err error
		endPointer, err = r.startPointers.Get(blockIndex + 1)
		if err != nil {
			return 0, err
		}
	}
	start, err := r.getBlockStartPointer(blockIndex)
	if err != nil {
		return 0, err
	}
	return endPointer - start, nil
}

// clone is FieldsIndexReader#clone (FieldsIndexReader.java:142-149). Java
// catches the IOException and rethrows it as an UncheckedIOException; Go
// returns it.
func (r *fieldsIndexReader) clone() (fieldsIndex, error) {
	return cloneFieldsIndexReader(r)
}

// getMaxPointer is FieldsIndexReader#getMaxPointer
// (FieldsIndexReader.java:151-153).
func (r *fieldsIndexReader) getMaxPointer() int64 {
	return r.maxPointer
}

// checkIntegrity is FieldsIndexReader#checkIntegrity
// (FieldsIndexReader.java:155-158): CodecUtil.checksumEntireFile(indexInput).
func (r *fieldsIndexReader) checkIntegrity() error {
	_, err := gcodecs.ChecksumEntireFile(r.indexInput)
	return err
}

// randomAccessSlice stands in for IndexInput#randomAccessSlice(long, long),
// which Java's IndexInput declares and every concrete implementation
// provides. Gocene's store.IndexInput exposes Slice; when the resulting
// implementation already answers random-access reads it is returned as is,
// and otherwise the window is read into memory and wrapped, which is what
// Lucene's own ByteBuffer-backed slices amount to.
func randomAccessSlice(in store.IndexInput, desc string, offset, length int64) (packed.RandomAccessInput, error) {
	if length == 0 {
		return store.NewByteArrayRandomAccessInput(nil), nil
	}
	sub, err := in.Slice(desc, offset, length)
	if err != nil {
		return nil, err
	}
	if ra, ok := sub.(packed.RandomAccessInput); ok {
		return ra, nil
	}
	buf := make([]byte, length)
	if err := sub.ReadBytes(buf, 0, len(buf)); err != nil {
		return nil, fmt.Errorf("randomAccessSlice: read %d bytes at %d: %w", length, offset, err)
	}
	return store.NewByteArrayRandomAccessInput(buf), nil
}
