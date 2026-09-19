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
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/lucene50/compressing/FieldsIndexReader.java

package compressing

import (
	"fmt"

	bcpacked "github.com/FlavioCFOliveira/Gocene/backward_codecs/packed"
	bstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	// fieldsIndexVersionStart mirrors {@code static final int VERSION_START = 0}.
	fieldsIndexVersionStart = 0
	// fieldsIndexVersionCurrent mirrors {@code static final int VERSION_CURRENT = 0}.
	fieldsIndexVersionCurrent = 0
)

// fieldsIndexReader is the off-heap fields index written from Lucene 8.5
// onwards: two DirectMonotonic runs, one mapping block to first doc and one
// mapping block to start pointer.
//
// Mirrors {@code final class FieldsIndexReader extends FieldsIndex} (Lucene
// 10.5.0, package-private).
type fieldsIndexReader struct {
	maxDoc            int
	blockShift        int
	numChunks         int
	docsMeta          *bcpacked.LegacyDirectMonotonicMeta
	startPointersMeta *bcpacked.LegacyDirectMonotonicMeta
	indexInput        store.IndexInput

	docsStartPointer          int64
	docsEndPointer            int64
	startPointersStartPointer int64
	startPointersEndPointer   int64

	docs          *bcpacked.LegacyDirectMonotonicReader
	startPointers *bcpacked.LegacyDirectMonotonicReader
	maxPointer    int64
}

// newFieldsIndexReader reproduces the package-private constructor
//
//	FieldsIndexReader(Directory dir, String name, String suffix, String extension,
//	    String codecName, byte[] id, IndexInput metaIn)
func newFieldsIndexReader(
	dir store.Directory,
	name string,
	suffix string,
	extension string,
	codecName string,
	id []byte,
	metaIn store.IndexInput,
) (*fieldsIndexReader, error) {
	r := &fieldsIndexReader{}

	maxDoc, err := metaIn.ReadInt()
	if err != nil {
		return nil, err
	}
	r.maxDoc = int(maxDoc)
	blockShift, err := metaIn.ReadInt()
	if err != nil {
		return nil, err
	}
	r.blockShift = int(blockShift)
	numChunks, err := metaIn.ReadInt()
	if err != nil {
		return nil, err
	}
	r.numChunks = int(numChunks)
	r.docsStartPointer, err = metaIn.ReadLong()
	if err != nil {
		return nil, err
	}
	r.docsMeta, err = bcpacked.LegacyDirectMonotonicReaderLoadMeta(metaIn, int64(r.numChunks), r.blockShift)
	if err != nil {
		return nil, err
	}
	r.docsEndPointer, err = metaIn.ReadLong()
	if err != nil {
		return nil, err
	}
	r.startPointersStartPointer = r.docsEndPointer
	r.startPointersMeta, err = bcpacked.LegacyDirectMonotonicReaderLoadMeta(metaIn, int64(r.numChunks), r.blockShift)
	if err != nil {
		return nil, err
	}
	r.startPointersEndPointer, err = metaIn.ReadLong()
	if err != nil {
		return nil, err
	}
	r.maxPointer, err = metaIn.ReadLong()
	if err != nil {
		return nil, err
	}

	indexInput, err := bstore.OpenInput(dir, store.SegmentFileName(name, suffix, extension), spi.IOContextDefault)
	if err != nil {
		return nil, err
	}
	r.indexInput = indexInput
	success := false
	defer func() {
		if !success {
			_ = indexInput.Close()
		}
	}()
	if _, err := codecs.CheckIndexHeader(
		indexInput, codecName+"Idx", fieldsIndexVersionStart, fieldsIndexVersionCurrent, id, suffix,
	); err != nil {
		return nil, err
	}
	if _, err := codecs.RetrieveChecksum(indexInput); err != nil {
		return nil, err
	}
	success = true

	if err := r.openReaders(indexInput); err != nil {
		_ = indexInput.Close()
		return nil, err
	}
	return r, nil
}

// openReaders reproduces the tail shared by both constructors:
//
//	final RandomAccessInput docsSlice =
//	    indexInput.randomAccessSlice(docsStartPointer, docsEndPointer - docsStartPointer);
//	final RandomAccessInput startPointersSlice =
//	    indexInput.randomAccessSlice(
//	        startPointersStartPointer, startPointersEndPointer - startPointersStartPointer);
//	docs = LegacyDirectMonotonicReader.getInstance(docsMeta, docsSlice);
//	startPointers = LegacyDirectMonotonicReader.getInstance(startPointersMeta, startPointersSlice);
func (r *fieldsIndexReader) openReaders(indexInput store.IndexInput) error {
	slicer, ok := indexInput.(interface {
		RandomAccessSlice(offset, length int64) (store.RandomAccessInput, error)
	})
	if !ok {
		return fmt.Errorf("lucene50/compressing: %T does not support randomAccessSlice", indexInput)
	}
	docsSlice, err := slicer.RandomAccessSlice(r.docsStartPointer, r.docsEndPointer-r.docsStartPointer)
	if err != nil {
		return err
	}
	startPointersSlice, err := slicer.RandomAccessSlice(
		r.startPointersStartPointer, r.startPointersEndPointer-r.startPointersStartPointer)
	if err != nil {
		return err
	}
	r.docs, err = bcpacked.LegacyDirectMonotonicReaderGetInstance(r.docsMeta, docsSlice)
	if err != nil {
		return err
	}
	r.startPointers, err = bcpacked.LegacyDirectMonotonicReaderGetInstance(r.startPointersMeta, startPointersSlice)
	return err
}

// cloneFieldsIndexReader reproduces the private copy constructor
// {@code FieldsIndexReader(FieldsIndexReader other)}.
func (r *fieldsIndexReader) cloneFieldsIndexReader() (*fieldsIndexReader, error) {
	cp := &fieldsIndexReader{
		maxDoc:                    r.maxDoc,
		numChunks:                 r.numChunks,
		blockShift:                r.blockShift,
		docsMeta:                  r.docsMeta,
		startPointersMeta:         r.startPointersMeta,
		indexInput:                r.indexInput.Clone(),
		docsStartPointer:          r.docsStartPointer,
		docsEndPointer:            r.docsEndPointer,
		startPointersStartPointer: r.startPointersStartPointer,
		startPointersEndPointer:   r.startPointersEndPointer,
		maxPointer:                r.maxPointer,
	}
	if err := cp.openReaders(cp.indexInput); err != nil {
		return nil, err
	}
	return cp, nil
}

// Close reproduces {@code indexInput.close();}.
func (r *fieldsIndexReader) Close() error { return r.indexInput.Close() }

// GetStartPointer reproduces
//
//	Objects.checkIndex(docID, maxDoc);
//	long blockIndex = docs.binarySearch(0, numChunks, docID);
//	if (blockIndex < 0) blockIndex = -2 - blockIndex;
//	return startPointers.get(blockIndex);
func (r *fieldsIndexReader) GetStartPointer(docID int) (int64, error) {
	if docID < 0 || docID >= r.maxDoc {
		return 0, fmt.Errorf("lucene50/compressing: docID %d out of range [0, %d)", docID, r.maxDoc)
	}
	blockIndex, err := r.docs.BinarySearch(0, int64(r.numChunks), int64(docID))
	if err != nil {
		return 0, err
	}
	if blockIndex < 0 {
		blockIndex = -2 - blockIndex
	}
	return r.startPointers.Get(blockIndex)
}

// Clone reproduces
//
//	try { return new FieldsIndexReader(this); }
//	catch (IOException e) { throw new UncheckedIOException(e); }
//
// Java wraps the IOException because FieldsIndex.clone() declares none; the Go
// rendering returns nil on that failure, which every caller treats as "no
// index".
func (r *fieldsIndexReader) Clone() FieldsIndex {
	cp, err := r.cloneFieldsIndexReader()
	if err != nil {
		return nil
	}
	return cp
}

// GetMaxPointer reproduces {@code public long getMaxPointer()}.
func (r *fieldsIndexReader) GetMaxPointer() int64 { return r.maxPointer }

// CheckIntegrity reproduces {@code CodecUtil.checksumEntireFile(indexInput);}.
func (r *fieldsIndexReader) CheckIntegrity() error {
	_, err := codecs.ChecksumEntireFile(r.indexInput)
	return err
}

var _ FieldsIndex = (*fieldsIndexReader)(nil)
