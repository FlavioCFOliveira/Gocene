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
)

// ErrOfflinePointWriterClosed is returned by OfflinePointWriter
// operations attempted after Close. It is the Go analogue of the
// {@code "Point writer is already closed"} assertion in Lucene.
var ErrOfflinePointWriterClosed = errors.New("bkd: offline point writer is already closed")

// ErrOfflinePointWriterOpen is returned by GetReader when invoked
// before Close. It mirrors Lucene's {@code "point writer is still
// open and trying to get a reader"} assertion.
var ErrOfflinePointWriterOpen = errors.New("bkd: offline point writer is still open and trying to get a reader")

// tempOutputCreator is the segregated capability required by
// OfflinePointWriter on top of store.Directory. Concrete directories
// such as ByteBuffersDirectory and FSDirectory implement it; the
// abstract Directory interface does not advertise it.
type tempOutputCreator interface {
	CreateTempOutput(prefix, suffix string, ctx store.IOContext) (store.IndexOutput, error)
}

// OfflinePointWriter writes points to a temporary file on disk in
// the same fixed-width format produced by Lucene's OfflinePointWriter
// and consumed by OfflinePointReader. Port of
// org.apache.lucene.util.bkd.OfflinePointWriter (Lucene 10.4.0).
//
// On-disk layout: a contiguous run of points, each BytesPerDoc bytes
// long, made up of the packed value followed by the docID encoded as
// 4 big-endian bytes; the stream is terminated by a 16-byte codec
// footer. The byte stream is identical to the one produced by the
// Java reference for the same inputs.
//
// OfflinePointWriter is not safe for concurrent use.
type OfflinePointWriter struct {
	tempDir       store.Directory
	out           store.IndexOutput          // raw output (held for Close even when wrapped)
	checksumOut   *store.ChecksumIndexOutput // wrapper used for all data writes
	name          string
	config        BKDConfig
	count         int64
	expectedCount int64
	scratchDocID  [4]byte // reusable buffer for docID encoding; avoids per-Append allocation
	closed        bool
}

// NewOfflinePointWriter creates a new writer backed by a temporary
// file in tempDir. The file name follows Lucene's convention
// {@code <prefix>_bkd_<desc>_<n>.tmp}: the leading {@code bkd_} tag
// is folded into the suffix passed to {@code CreateTempOutput} so
// the on-disk naming stays consistent with the Java reference.
//
// expectedCount is the maximum number of points the caller intends
// to append. A value of zero disables the upper-bound check, exactly
// as in Java. If tempDir does not implement {@code CreateTempOutput}
// the call fails with a typed error.
func NewOfflinePointWriter(
	config BKDConfig,
	tempDir store.Directory,
	tempFileNamePrefix string,
	desc string,
	expectedCount int64,
) (*OfflinePointWriter, error) {
	if tempDir == nil {
		return nil, errors.New("bkd: tempDir cannot be nil")
	}
	if expectedCount < 0 {
		return nil, fmt.Errorf("bkd: expectedCount must be non-negative; got %d", expectedCount)
	}
	creator, ok := tempDir.(tempOutputCreator)
	if !ok {
		return nil, fmt.Errorf("bkd: directory %T does not support CreateTempOutput", tempDir)
	}

	// Java passes IOContext.DEFAULT; the closest Go analogue for a
	// new sink is IOContextWrite (no MergeInfo, no FlushInfo).
	raw, err := creator.CreateTempOutput(tempFileNamePrefix, "bkd_"+desc, store.IOContextWrite)
	if err != nil {
		return nil, err
	}

	return &OfflinePointWriter{
		tempDir:       tempDir,
		out:           raw,
		checksumOut:   store.NewChecksumIndexOutput(raw),
		name:          raw.GetName(),
		config:        config,
		expectedCount: expectedCount,
	}, nil
}

// Name returns the name of the underlying temporary file. Mirrors
// the package-private {@code name} field exposed in Java.
func (w *OfflinePointWriter) Name() string { return w.name }

// Append writes a single point composed of {@code packedValue} and
// {@code docID}. The packed value length must equal
// {@code config.PackedBytesLength()}. The docID is encoded as four
// big-endian bytes (matching Lucene's {@code writeInt(reverseBytes)}
// pattern on a little-endian {@code IndexOutput}). Implements
// {@link PointWriter#Append}.
func (w *OfflinePointWriter) Append(packedValue []byte, docID int) error {
	if w.closed {
		return ErrOfflinePointWriterClosed
	}
	if len(packedValue) != w.config.PackedBytesLength() {
		return fmt.Errorf("bkd: [packedValue] must have length %d but was %d",
			w.config.PackedBytesLength(), len(packedValue))
	}
	if err := w.checksumOut.WriteBytes(packedValue); err != nil {
		return err
	}
	binary.BigEndian.PutUint32(w.scratchDocID[:], uint32(int32(docID)))
	if err := w.checksumOut.WriteBytes(w.scratchDocID[:]); err != nil {
		return err
	}
	w.count++
	if w.expectedCount != 0 && w.count > w.expectedCount {
		return fmt.Errorf("bkd: expectedCount=%d vs count=%d", w.expectedCount, w.count)
	}
	return nil
}

// AppendPointValue writes a single point sourced from a PointValue.
// {@code pointValue.PackedValueDocIDBytes()} must span exactly
// {@code config.BytesPerDoc()} bytes. Implements
// {@link PointWriter#AppendPointValue}.
func (w *OfflinePointWriter) AppendPointValue(pointValue PointValue) error {
	if w.closed {
		return ErrOfflinePointWriterClosed
	}
	combo := pointValue.PackedValueDocIDBytes()
	if combo.Length != w.config.BytesPerDoc() {
		return fmt.Errorf("bkd: [packedValue and docID] must have length %d but was %d",
			w.config.BytesPerDoc(), combo.Length)
	}
	if err := w.checksumOut.WriteBytes(
		combo.Bytes[combo.Offset : combo.Offset+combo.Length],
	); err != nil {
		return err
	}
	w.count++
	if w.expectedCount != 0 && w.count > w.expectedCount {
		return fmt.Errorf("bkd: expectedCount=%d vs count=%d", w.expectedCount, w.count)
	}
	return nil
}

// Count returns the number of points appended so far. Implements
// {@link PointWriter#Count}.
func (w *OfflinePointWriter) Count() int64 { return w.count }

// GetReader returns a PointReader that iterates the half-open range
// {@code [start, start+length)} of the points previously written.
// The writer must already be closed. Implements
// {@link PointWriter#GetReader}.
func (w *OfflinePointWriter) GetReader(start, length int64) (PointReader, error) {
	buffer := make([]byte, w.config.BytesPerDoc())
	return w.getReaderWithBuffer(start, length, buffer)
}

// getReaderWithBuffer mirrors the protected
// {@code getReader(long, long, byte[])} overload in Java, used by
// downstream BKD code that pools reusable buffers.
func (w *OfflinePointWriter) getReaderWithBuffer(start, length int64, reusableBuffer []byte) (*OfflinePointReader, error) {
	if !w.closed {
		return nil, ErrOfflinePointWriterOpen
	}
	if start < 0 || length < 0 {
		return nil, fmt.Errorf("bkd: start and length must be non-negative (start=%d length=%d)", start, length)
	}
	if start+length > w.count {
		return nil, fmt.Errorf("bkd: start=%d length=%d count=%d", start, length, w.count)
	}
	if w.expectedCount != 0 && w.count != w.expectedCount {
		return nil, fmt.Errorf("bkd: expectedCount=%d vs count=%d", w.expectedCount, w.count)
	}
	return NewOfflinePointReader(w.config, w.tempDir, w.name, start, length, reusableBuffer)
}

// Close writes the codec footer and closes the underlying
// IndexOutput. Calling Close twice is a no-op (Lucene's behaviour).
// Implements {@link io.Closer}.
func (w *OfflinePointWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	footerErr := codecs.WriteFooter(w.checksumOut)
	closeErr := w.out.Close()
	if footerErr != nil {
		return footerErr
	}
	return closeErr
}

// Destroy deletes the temporary file backing this writer. Implements
// {@link PointWriter#Destroy}.
func (w *OfflinePointWriter) Destroy() error {
	return w.tempDir.DeleteFile(w.name)
}

// String returns Lucene's debug representation:
// {@code OfflinePointWriter(count=N tempFileName=...)}.
func (w *OfflinePointWriter) String() string {
	return fmt.Sprintf("OfflinePointWriter(count=%d tempFileName=%s)", w.count, w.name)
}

// IsClosed reports whether the writer has been closed. Exposed for
// tests; Java keeps the flag private but checks it via assertions.
func (w *OfflinePointWriter) IsClosed() bool { return w.closed }

// Ensure interface conformance at compile time.
var (
	_ PointWriter = (*OfflinePointWriter)(nil)
	_ io.Closer   = (*OfflinePointWriter)(nil)
)
