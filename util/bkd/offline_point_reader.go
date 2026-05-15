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
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrOfflinePointReaderClosed is returned by OfflinePointReader operations
// invoked after Close.
var ErrOfflinePointReaderClosed = errors.New("bkd: offline point reader is already closed")

// OfflinePointReader reads points from disk in the fixed-width format
// produced by OfflinePointWriter. Port of
// org.apache.lucene.util.bkd.OfflinePointReader (Lucene 10.4.0).
//
// On-disk layout: a contiguous run of points, each BytesPerDoc bytes
// long (packed value followed by the docID encoded as 4 big-endian
// bytes), followed by a 16-byte codec footer. The reader iterates a
// caller-specified half-open range [start, start+length) of points,
// using a reusable buffer to amortise IndexInput reads.
//
// When the requested range covers the whole file the reader opens it
// through a ChecksumIndexInput and verifies the footer on Close, in
// line with Lucene's best-effort checksumming behaviour.
//
// OfflinePointReader is not safe for concurrent use.
type OfflinePointReader struct {
	in             store.IndexInput
	checksumIn     *store.ChecksumIndexInput // non-nil iff checksumming is active
	config         BKDConfig
	onHeapBuffer   []byte
	maxPointOnHeap int
	pointsInBuffer int
	offset         int
	countLeft      int64
	name           string
	checked        bool
	closed         bool
	pointValue     *offlinePointValue
}

// NewOfflinePointReader builds an OfflinePointReader iterating
// {@code length} points starting at point index {@code start} in
// {@code tempFileName}. The supplied {@code reusableBuffer} must be at
// least BytesPerDoc bytes long and is retained for the lifetime of the
// reader. Mirrors {@code OfflinePointReader(BKDConfig, Directory, String,
// long, long, byte[])} in Lucene.
func NewOfflinePointReader(
	config BKDConfig,
	tempDir store.Directory,
	tempFileName string,
	start, length int64,
	reusableBuffer []byte,
) (*OfflinePointReader, error) {
	if reusableBuffer == nil {
		return nil, errors.New("bkd: [reusableBuffer] cannot be null")
	}
	if len(reusableBuffer) < config.BytesPerDoc() {
		return nil, fmt.Errorf("bkd: length of [reusableBuffer] must be bigger than %d",
			config.BytesPerDoc())
	}
	if start < 0 || length < 0 {
		return nil, fmt.Errorf("bkd: start and length must be non-negative (start=%d length=%d)",
			start, length)
	}

	fileLength, err := tempDir.FileLength(tempFileName)
	if err != nil {
		return nil, err
	}

	requiredBytes := (start+length)*int64(config.BytesPerDoc()) + int64(codecs.FooterLength())
	if requiredBytes > fileLength {
		return nil, fmt.Errorf(
			"bkd: requested slice is beyond the length of this file: start=%d length=%d bytesPerDoc=%d fileLength=%d tempFileName=%s",
			start, length, config.BytesPerDoc(), fileLength, tempFileName)
	}

	maxPointOnHeap := len(reusableBuffer) / config.BytesPerDoc()

	// Best-effort checksumming: open via ChecksumIndexInput only when
	// reading the entire file. Mirrors the Java reader's heuristic.
	rawIn, err := tempDir.OpenInput(tempFileName, store.IOContextReadOnce)
	if err != nil {
		return nil, err
	}

	var (
		readerIn   store.IndexInput
		checksumIn *store.ChecksumIndexInput
	)
	if start == 0 && length*int64(config.BytesPerDoc()) == fileLength-int64(codecs.FooterLength()) {
		checksumIn = store.NewChecksumIndexInput(rawIn)
		readerIn = checksumIn
	} else {
		readerIn = rawIn
	}

	seekFP := start * int64(config.BytesPerDoc())
	if err := readerIn.SetPosition(seekFP); err != nil {
		_ = readerIn.Close()
		return nil, err
	}

	r := &OfflinePointReader{
		in:             readerIn,
		checksumIn:     checksumIn,
		config:         config,
		onHeapBuffer:   reusableBuffer,
		maxPointOnHeap: maxPointOnHeap,
		countLeft:      length,
		name:           tempFileName,
	}
	r.pointValue = newOfflinePointValue(config, reusableBuffer)
	return r, nil
}

// Next advances the iterator to the next point. Returns false once the
// requested range is fully consumed. Implements PointReader.Next.
func (r *OfflinePointReader) Next() (bool, error) {
	if r.closed {
		return false, ErrOfflinePointReaderClosed
	}
	if r.pointsInBuffer == 0 {
		// countLeft is always >= 0 in the Go port (the constructor
		// rejects negatives). The Java variant carried a defensive
		// branch for countLeft == -1 that masked EOFExceptions; we
		// surface them as real errors instead.
		if r.countLeft == 0 {
			return false, nil
		}
		var toRead int
		if r.countLeft > int64(r.maxPointOnHeap) {
			toRead = r.maxPointOnHeap
		} else {
			toRead = int(r.countLeft)
		}
		readBytes := toRead * r.config.BytesPerDoc()
		if err := r.in.ReadBytes(r.onHeapBuffer[:readBytes]); err != nil {
			return false, err
		}
		r.pointsInBuffer = toRead - 1
		r.countLeft -= int64(toRead)
		r.offset = 0
	} else {
		r.pointsInBuffer--
		r.offset += r.config.BytesPerDoc()
	}
	return true, nil
}

// PointValue returns the current point. The returned value is reusable
// and is valid only until the next Next call. Implements
// PointReader.PointValue.
func (r *OfflinePointReader) PointValue() PointValue {
	r.pointValue.setOffset(r.offset)
	return r.pointValue
}

// Close releases the underlying IndexInput. When the reader was opened
// for whole-file iteration and all points have been consumed, the codec
// footer is verified via CheckFooter, propagating any checksum
// mismatch. Calling Close twice is a no-op.
func (r *OfflinePointReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true

	var verifyErr error
	if r.checksumIn != nil && r.countLeft == 0 && !r.checked {
		r.checked = true
		if _, err := codecs.CheckFooter(r.checksumIn); err != nil {
			verifyErr = err
		}
	}
	closeErr := r.in.Close()
	if verifyErr != nil {
		return verifyErr
	}
	return closeErr
}

// Ensure interface conformance at compile time.
var (
	_ PointReader = (*OfflinePointReader)(nil)
	_ io.Closer   = (*OfflinePointReader)(nil)
)

// offlinePointValue is the reusable PointValue view sharing the
// reader's on-heap buffer. Port of Lucene's
// {@code OfflinePointReader.OfflinePointValue}.
type offlinePointValue struct {
	packedValue       *util.BytesRef
	packedValueDocID  *util.BytesRef
	packedValueLength int
}

func newOfflinePointValue(config BKDConfig, value []byte) *offlinePointValue {
	packedLen := config.PackedBytesLength()
	return &offlinePointValue{
		packedValueLength: packedLen,
		packedValue:       &util.BytesRef{Bytes: value, Offset: 0, Length: packedLen},
		packedValueDocID:  &util.BytesRef{Bytes: value, Offset: 0, Length: config.BytesPerDoc()},
	}
}

// setOffset re-points both views at the slot starting at offset.
func (v *offlinePointValue) setOffset(offset int) {
	v.packedValue.Offset = offset
	v.packedValueDocID.Offset = offset
}

// PackedValue implements PointValue.
func (v *offlinePointValue) PackedValue() *util.BytesRef { return v.packedValue }

// DocID implements PointValue. The docID is encoded as 4 big-endian
// bytes immediately after the packed value, matching the encoding
// used by both OfflinePointWriter and HeapPointWriter.
func (v *offlinePointValue) DocID() int {
	position := v.packedValueDocID.Offset + v.packedValueLength
	return int(int32(binary.BigEndian.Uint32(v.packedValueDocID.Bytes[position : position+4])))
}

// PackedValueDocIDBytes implements PointValue.
func (v *offlinePointValue) PackedValueDocIDBytes() *util.BytesRef { return v.packedValueDocID }
