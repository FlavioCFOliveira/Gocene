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

package bkd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrPointWriterClosed is returned by HeapPointWriter operations that
// require the writer to still be open. Mirrors the IllegalStateException
// thrown by Lucene's HeapPointWriter after close().
var ErrPointWriterClosed = errors.New("bkd: point writer is already closed")

// HeapPointWriter writes new points into an on-heap byte slab. Port of
// org.apache.lucene.util.bkd.HeapPointWriter (Lucene 10.4.0).
//
// A single byte slab of size MaxPoints * BytesPerDoc is allocated up
// front. Each appended point occupies a fixed-size slot
// [i*bytesPerDoc, (i+1)*bytesPerDoc). The slot layout matches Java:
// PackedBytesLength bytes of dimension data followed by the docID as a
// big-endian 32-bit unsigned integer.
//
// HeapPointWriter is not safe for concurrent use.
type HeapPointWriter struct {
	block                []byte
	scratch              []byte
	config               BKDConfig
	size                 int
	dataDimsAndDocLength int
	nextWrite            int
	closed               bool
	pointValue           *heapPointValue
}

// NewHeapPointWriter allocates a new HeapPointWriter for at most size
// points using the supplied configuration. Mirrors the Java constructor:
// when size is zero, no reusable PointValue view is allocated.
func NewHeapPointWriter(config BKDConfig, size int) *HeapPointWriter {
	bytesPerDoc := config.BytesPerDoc()
	w := &HeapPointWriter{
		block:                make([]byte, bytesPerDoc*size),
		scratch:              make([]byte, bytesPerDoc),
		config:               config,
		size:                 size,
		dataDimsAndDocLength: bytesPerDoc - config.PackedIndexBytesLength(),
	}
	if size > 0 {
		w.pointValue = newHeapPointValue(config, w.block)
	}
	return w
}

// Size returns the maximum number of points the writer can hold. Mirrors
// the package-private final field {@code size} in Lucene's reference.
func (w *HeapPointWriter) Size() int { return w.size }

// GetPackedValueSlice returns the reusable PointValue view pointing at
// the slot for {@code index}. The returned pointer is stable across
// calls; consumers must not retain it past the next call. Java:
// {@code getPackedValueSlice(int)}.
func (w *HeapPointWriter) GetPackedValueSlice(index int) PointValue {
	if index < 0 || index >= w.nextWrite {
		panic(fmt.Sprintf("bkd: index out of range (index=%d nextWrite=%d)", index, w.nextWrite))
	}
	w.pointValue.setOffset(index * w.config.BytesPerDoc())
	return w.pointValue
}

// Append writes a point composed of {@code packedValue} and {@code docID}.
// Java: {@code append(byte[] packedValue, int docID)}.
func (w *HeapPointWriter) Append(packedValue []byte, docID int) error {
	if w.closed {
		return ErrPointWriterClosed
	}
	if len(packedValue) != w.config.PackedBytesLength() {
		return fmt.Errorf("bkd: packedValue must have length %d but was %d",
			w.config.PackedBytesLength(), len(packedValue))
	}
	if w.nextWrite >= w.size {
		return fmt.Errorf("bkd: writer is full (nextWrite=%d size=%d)", w.nextWrite+1, w.size)
	}
	position := w.nextWrite * w.config.BytesPerDoc()
	copy(w.block[position:position+w.config.PackedBytesLength()], packedValue)
	binary.BigEndian.PutUint32(w.block[position+w.config.PackedBytesLength():], uint32(docID))
	w.nextWrite++
	return nil
}

// AppendPointValue writes a point sourced from another PointValue. The
// PointValue's PackedValueDocIDBytes must have length equal to
// BytesPerDoc. Java: {@code append(PointValue pointValue)}.
func (w *HeapPointWriter) AppendPointValue(pointValue PointValue) error {
	if w.closed {
		return ErrPointWriterClosed
	}
	if w.nextWrite >= w.size {
		return fmt.Errorf("bkd: writer is full (nextWrite=%d size=%d)", w.nextWrite+1, w.size)
	}
	combo := pointValue.PackedValueDocIDBytes()
	if combo.Length != w.config.BytesPerDoc() {
		return fmt.Errorf("bkd: packedValueDocID must have length %d but was %d",
			w.config.BytesPerDoc(), combo.Length)
	}
	position := w.nextWrite * w.config.BytesPerDoc()
	copy(w.block[position:position+w.config.BytesPerDoc()],
		combo.Bytes[combo.Offset:combo.Offset+combo.Length])
	w.nextWrite++
	return nil
}

// swap exchanges the point stored at index i with the one stored at
// index j. Package-private in Java; intended for the BKD radix sorters
// (which live in the same Go package).
func (w *HeapPointWriter) swap(i, j int) {
	bytesPerDoc := w.config.BytesPerDoc()
	indexI := i * bytesPerDoc
	indexJ := j * bytesPerDoc
	copy(w.scratch, w.block[indexI:indexI+bytesPerDoc])
	copy(w.block[indexI:indexI+bytesPerDoc], w.block[indexJ:indexJ+bytesPerDoc])
	copy(w.block[indexJ:indexJ+bytesPerDoc], w.scratch)
}

// byteAt returns the unsigned byte at position k inside the slot for
// point i. Mirrors Java's {@code byteAt(int i, int k)}.
func (w *HeapPointWriter) byteAt(i, k int) int {
	return int(w.block[i*w.config.BytesPerDoc()+k])
}

// copyDim copies the bytes of dimension dim from the point at index i
// into dst starting at offset. Mirrors Java's
// {@code copyDim(int i, int dim, byte[] bytes, int offset)}.
func (w *HeapPointWriter) copyDim(i, dim int, dst []byte, offset int) {
	src := i*w.config.BytesPerDoc() + dim
	copy(dst[offset:offset+w.config.BytesPerDim()], w.block[src:src+w.config.BytesPerDim()])
}

// copyDataDimsAndDoc copies the data-dimension bytes and the docID
// portion of the point at index i into dst at offset. Mirrors
// {@code copyDataDimsAndDoc(int i, byte[] bytes, int offset)}.
func (w *HeapPointWriter) copyDataDimsAndDoc(i int, dst []byte, offset int) {
	src := i*w.config.BytesPerDoc() + w.config.PackedIndexBytesLength()
	copy(dst[offset:offset+w.dataDimsAndDocLength], w.block[src:src+w.dataDimsAndDocLength])
}

// compareDim compares the {@code dim} byte range of the points at
// indices i and j. Mirrors {@code compareDim(int i, int j, int dim)}.
func (w *HeapPointWriter) compareDim(i, j, dim int) int {
	bytesPerDoc := w.config.BytesPerDoc()
	bytesPerDim := w.config.BytesPerDim()
	iOffset := i*bytesPerDoc + dim
	jOffset := j*bytesPerDoc + dim
	return bytes.Compare(
		w.block[iOffset:iOffset+bytesPerDim],
		w.block[jOffset:jOffset+bytesPerDim],
	)
}

// compareDimWithValue compares the {@code dim} byte range of the point
// at index j against the provided slice starting at offset. Mirrors
// {@code compareDim(int j, byte[] dimValue, int offset, int dim)}.
func (w *HeapPointWriter) compareDimWithValue(j int, dimValue []byte, offset, dim int) int {
	bytesPerDim := w.config.BytesPerDim()
	jOffset := j*w.config.BytesPerDoc() + dim
	return bytes.Compare(
		dimValue[offset:offset+bytesPerDim],
		w.block[jOffset:jOffset+bytesPerDim],
	)
}

// compareDataDimsAndDoc compares the data dimensions and the docID
// suffix of the points at indices i and j. Mirrors
// {@code compareDataDimsAndDoc(int i, int j)}.
func (w *HeapPointWriter) compareDataDimsAndDoc(i, j int) int {
	bytesPerDoc := w.config.BytesPerDoc()
	indexBytes := w.config.PackedIndexBytesLength()
	iOffset := i*bytesPerDoc + indexBytes
	jOffset := j*bytesPerDoc + indexBytes
	return bytes.Compare(
		w.block[iOffset:iOffset+w.dataDimsAndDocLength],
		w.block[jOffset:jOffset+w.dataDimsAndDocLength],
	)
}

// compareDataDimsAndDocWithValue compares the data dimensions and the
// docID suffix of the point at index j against the supplied slice.
// Mirrors {@code compareDataDimsAndDoc(int j, byte[] dataDimsAndDocs, int offset)}.
func (w *HeapPointWriter) compareDataDimsAndDocWithValue(j int, dataDimsAndDocs []byte, offset int) int {
	jOffset := j*w.config.BytesPerDoc() + w.config.PackedIndexBytesLength()
	return bytes.Compare(
		dataDimsAndDocs[offset:offset+w.dataDimsAndDocLength],
		w.block[jOffset:jOffset+w.dataDimsAndDocLength],
	)
}

// ComputeCardinality returns the number of distinct points in the
// half-open range [from, to), considering only the bytes beyond
// commonPrefixLengths for each dimension. Mirrors Java's
// {@code computeCardinality(int from, int to, int[] commonPrefixLengths)}.
//
// Lucene scans the data dimensions; the Go port preserves that exact
// scan order so the returned value matches byte-for-byte.
func (w *HeapPointWriter) ComputeCardinality(from, to int, commonPrefixLengths []int) int {
	leafCardinality := 1
	bytesPerDoc := w.config.BytesPerDoc()
	bytesPerDim := w.config.BytesPerDim()
	numDims := w.config.NumDims()
	for i := from + 1; i < to; i++ {
		pointOffset := (i - 1) * bytesPerDoc
		nextPointOffset := pointOffset + bytesPerDoc
		for dim := 0; dim < numDims; dim++ {
			start := dim*bytesPerDim + commonPrefixLengths[dim]
			end := dim*bytesPerDim + bytesPerDim
			if !bytes.Equal(
				w.block[nextPointOffset+start:nextPointOffset+end],
				w.block[pointOffset+start:pointOffset+end],
			) {
				leafCardinality++
				break
			}
		}
	}
	return leafCardinality
}

// Count returns the number of points currently stored. Implements
// {@link PointWriter#count()}.
func (w *HeapPointWriter) Count() int64 { return int64(w.nextWrite) }

// GetReader returns a PointReader iterating the half-open range
// [start, start+length) of the stored points. The writer must be closed
// before calling. Implements {@link PointWriter#getReader(long, long)}.
func (w *HeapPointWriter) GetReader(start, length int64) (PointReader, error) {
	if !w.closed {
		return nil, errors.New("bkd: point writer is still open and trying to get a reader")
	}
	if start < 0 || length < 0 {
		return nil, fmt.Errorf("bkd: start and length must be non-negative (start=%d length=%d)", start, length)
	}
	if start+length > int64(w.size) {
		return nil, fmt.Errorf("bkd: start=%d length=%d docIDs.length=%d", start, length, w.size)
	}
	if start+length > int64(w.nextWrite) {
		return nil, fmt.Errorf("bkd: start=%d length=%d nextWrite=%d", start, length, w.nextWrite)
	}
	end := start + length
	return NewHeapPointReader(func(i int) PointValue { return w.GetPackedValueSlice(i) }, int(start), int(end)), nil
}

// Close marks the writer as closed. Subsequent Append / AppendPointValue
// calls return ErrPointWriterClosed. Implements {@link io.Closeable}.
func (w *HeapPointWriter) Close() error {
	w.closed = true
	return nil
}

// IsClosed reports whether the writer has been closed. Exposed for
// tests; Java keeps the flag private but checks it via assertions.
func (w *HeapPointWriter) IsClosed() bool { return w.closed }

// Destroy is a no-op for the heap writer; the slab is owned by the GC.
// Implements {@link PointWriter#destroy()}.
func (w *HeapPointWriter) Destroy() error { return nil }

// String returns the {@code HeapPointWriter(count=N size=M)} debug
// string used by Lucene's toString().
func (w *HeapPointWriter) String() string {
	return fmt.Sprintf("HeapPointWriter(count=%d size=%d)", w.nextWrite, w.size)
}

// heapPointValue is the reusable PointValue view sharing the writer's
// byte slab. Port of Lucene's private nested class
// {@code HeapPointWriter.HeapPointValue}.
type heapPointValue struct {
	packedValue       *util.BytesRef
	packedValueDocID  *util.BytesRef
	packedValueLength int
}

func newHeapPointValue(config BKDConfig, value []byte) *heapPointValue {
	packedLen := config.PackedBytesLength()
	return &heapPointValue{
		packedValueLength: packedLen,
		packedValue:       &util.BytesRef{Bytes: value, Offset: 0, Length: packedLen},
		packedValueDocID:  &util.BytesRef{Bytes: value, Offset: 0, Length: config.BytesPerDoc()},
	}
}

// setOffset re-points both views at the slot starting at offset.
func (h *heapPointValue) setOffset(offset int) {
	h.packedValue.Offset = offset
	h.packedValueDocID.Offset = offset
}

// PackedValue implements PointValue.
func (h *heapPointValue) PackedValue() *util.BytesRef { return h.packedValue }

// DocID implements PointValue. Reads the BE-encoded docID from the
// slot's trailing 4 bytes.
func (h *heapPointValue) DocID() int {
	position := h.packedValueDocID.Offset + h.packedValueLength
	return int(int32(binary.BigEndian.Uint32(h.packedValueDocID.Bytes[position : position+4])))
}

// PackedValueDocIDBytes implements PointValue.
func (h *heapPointValue) PackedValueDocIDBytes() *util.BytesRef { return h.packedValueDocID }
