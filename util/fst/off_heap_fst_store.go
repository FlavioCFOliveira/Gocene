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

package fst

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// OffHeapFSTStore stores the FST bytes off-heap inside a RandomAccessInput
// (typically an IndexInput slice). It is the Go port of
// org.apache.lucene.util.fst.OffHeapFSTStore.
//
// NOTE on slicing: Lucene's reference calls in.randomAccessSlice(offset,
// numBytes) to obtain a virtual slice and then reads at relative
// positions. Gocene's store package does not yet expose a
// randomAccessSlice helper, so this port keeps an explicit base offset
// and translates positions when handing out a reverse reader.
type OffHeapFSTStore struct {
	in       store.RandomAccessInput
	offset   int64
	numBytes int64
}

// NewOffHeapFSTStore constructs an off-heap store backed by the given
// RandomAccessInput. The store reads numBytes starting at offset.
func NewOffHeapFSTStore(in store.RandomAccessInput, offset, numBytes int64) (*OffHeapFSTStore, error) {
	if in == nil {
		return nil, errors.New("OffHeapFSTStore: nil RandomAccessInput")
	}
	if offset < 0 || numBytes < 0 {
		return nil, errors.New("OffHeapFSTStore: negative offset or numBytes")
	}
	return &OffHeapFSTStore{in: in, offset: offset, numBytes: numBytes}, nil
}

// Size returns the byte count of this store.
func (s *OffHeapFSTStore) Size() int64 { return s.numBytes }

// GetReverseBytesReader implements FSTReader.
//
// The returned reader's positions are absolute within the backing
// RandomAccessInput (Lucene uses a virtual slice, so its positions
// start at zero — here they start at offset+numBytes-1).
func (s *OffHeapFSTStore) GetReverseBytesReader() BytesReader {
	r := NewReverseRandomAccessReader(s.in)
	r.SetPosition(s.offset + s.numBytes - 1)
	return r
}

// WriteTo implements FSTReader. Mirrors Lucene's behaviour: off-heap
// stores do not support writing back to a DataOutput.
func (s *OffHeapFSTStore) WriteTo(_ store.DataOutput) error {
	return errors.New("OffHeapFSTStore: WriteTo is not supported")
}

// RAMBytesUsed implements FSTReader. Off-heap stores hold a constant
// amount of on-heap state.
func (s *OffHeapFSTStore) RAMBytesUsed() int64 {
	const baseBytes = 24
	return baseBytes
}
