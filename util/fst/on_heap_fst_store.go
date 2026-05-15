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
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// OnHeapFSTStore stores the FST bytes in a single on-heap byte slice.
// This is the Go port of org.apache.lucene.util.fst.OnHeapFSTStore.
//
// NOTE on the maxBlockBits behaviour: Lucene's reference splits very
// large FSTs into multiple pages governed by maxBlockBits. The Go
// port currently materialises the entire FST as a single byte slice
// (Go can address byte slices well beyond 1 GiB on 64-bit systems);
// this matches the small-FST branch in the Java code. Multi-page
// support is intentionally deferred and tracked as future work.
type OnHeapFSTStore struct {
	bytes []byte
}

// NewOnHeapFSTStoreFromDataInput reads numBytes from in into a fresh
// on-heap byte slice. Mirrors Lucene's OnHeapFSTStore(int, DataInput,
// long) constructor.
func NewOnHeapFSTStoreFromDataInput(maxBlockBits int, in store.DataInput, numBytes int64) (*OnHeapFSTStore, error) {
	if maxBlockBits < 1 || maxBlockBits > 30 {
		return nil, fmt.Errorf("OnHeapFSTStore: maxBlockBits should be 1..30; got %d", maxBlockBits)
	}
	if numBytes < 0 {
		return nil, errors.New("OnHeapFSTStore: negative numBytes")
	}
	buf := make([]byte, numBytes)
	if numBytes > 0 {
		if err := in.ReadBytes(buf); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("OnHeapFSTStore: only read partial bytes, expected %d", numBytes)
			}
			return nil, err
		}
	}
	return &OnHeapFSTStore{bytes: buf}, nil
}

// NewOnHeapFSTStoreFromBytes wraps an existing byte slice as a store.
// The slice is not copied; callers must not mutate it after the
// store is constructed.
func NewOnHeapFSTStoreFromBytes(bytes []byte) *OnHeapFSTStore {
	return &OnHeapFSTStore{bytes: bytes}
}

// GetReverseBytesReader implements FSTReader.
func (s *OnHeapFSTStore) GetReverseBytesReader() BytesReader {
	return NewReverseBytesReader(s.bytes)
}

// WriteTo implements FSTReader.
func (s *OnHeapFSTStore) WriteTo(out store.DataOutput) error {
	return out.WriteBytesN(s.bytes, len(s.bytes))
}

// RAMBytesUsed implements FSTReader.
func (s *OnHeapFSTStore) RAMBytesUsed() int64 {
	const baseBytes = 16 // approx shallow size
	return int64(baseBytes + len(s.bytes))
}

// Size returns the number of bytes in the store.
func (s *OnHeapFSTStore) Size() int64 { return int64(len(s.bytes)) }
