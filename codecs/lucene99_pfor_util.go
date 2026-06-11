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
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Source: lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/
//
//	lucene99/PForUtil.java
//
// Purpose: 128-wide Patched Frame-of-Reference encode/decode/skip for the
// Lucene 9.9 backward-codecs postings format, using the long-based ForUtil.

package codecs

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene99PForMaxExceptions is the maximum number of exception values that
// can be patched per block. Mirrors PForUtil.MAX_EXCEPTIONS.
const lucene99PForMaxExceptions = 7

// lucene99PForUtil provides 128-wide Patched-FOR encode/decode using the
// long-based ForUtil. Mirrors backward_codecs.lucene99.PForUtil.
type lucene99PForUtil struct {
	forUtil *lucene99ForUtil
}

// newLucene99PForUtil creates a PForUtil backed by the given long-based
// 128-wide ForUtil. Mirrors PForUtil(ForUtil).
func newLucene99PForUtil(forUtil *lucene99ForUtil) *lucene99PForUtil {
	return &lucene99PForUtil{forUtil: forUtil}
}

// l99LongHeap is a 1-indexed min-heap of int64 values, used to track the
// top (largest) values in a block. Mirrors LongHeap.
type l99LongHeap struct {
	heap []int64 // 1-indexed; index 0 is unused
	sz   int     // number of elements currently in the heap
}

// newL99LongHeap creates a heap with the given maximum capacity.
func newL99LongHeap(maxSize int) *l99LongHeap {
	return &l99LongHeap{
		heap: make([]int64, maxSize+1),
	}
}

// push adds v to the heap. Mirrors LongHeap.push.
func (h *l99LongHeap) push(v int64) {
	h.sz++
	i := h.sz
	h.heap[i] = v
	for i > 1 && h.heap[i] < h.heap[i>>1] {
		h.heap[i], h.heap[i>>1] = h.heap[i>>1], h.heap[i]
		i >>= 1
	}
}

// top returns the minimum element in the heap. Mirrors LongHeap.top().
func (h *l99LongHeap) top() int64 {
	return h.heap[1]
}

// updateTop replaces the minimum element with v and re-heapifies downward.
// It returns the new minimum element. Mirrors LongHeap.updateTop.
func (h *l99LongHeap) updateTop(v int64) int64 {
	h.heap[1] = v
	i := 1
	for {
		smallest := i
		left := i << 1
		right := left + 1
		if left <= h.sz && h.heap[left] < h.heap[smallest] {
			smallest = left
		}
		if right <= h.sz && h.heap[right] < h.heap[smallest] {
			smallest = right
		}
		if smallest == i {
			break
		}
		h.heap[i], h.heap[smallest] = h.heap[smallest], h.heap[i]
		i = smallest
	}
	return h.heap[1]
}

// size returns the number of elements in the heap. Mirrors LongHeap.size().
func (h *l99LongHeap) size() int {
	return h.sz
}

// get returns the element at 1-indexed position i. Mirrors LongHeap.get(i).
func (h *l99LongHeap) get(i int) int64 {
	return h.heap[i]
}

// l99PForAllEqual reports whether every value in longs equals longs[0].
// Mirrors PForUtil.allEqual.
func l99PForAllEqual(longs []int64) bool {
	for i := 1; i < lucene99BlockSize; i++ {
		if longs[i] != longs[0] {
			return false
		}
	}
	return true
}

// encode encodes 128 int64 values from longs into out. Mirrors PForUtil.encode.
func (p *lucene99PForUtil) encode(longs []int64, out store.IndexOutput) error {
	if len(longs) < lucene99BlockSize {
		return errors.New("lucene99 PForUtil.encode: longs must have at least 128 elements")
	}

	// Build a min-heap containing the 8 largest values in the block.
	top := newL99LongHeap(lucene99PForMaxExceptions + 1)
	for i := 0; i <= lucene99PForMaxExceptions; i++ {
		top.push(longs[i])
	}
	topValue := top.top()
	for i := lucene99PForMaxExceptions + 1; i < lucene99BlockSize; i++ {
		if longs[i] > topValue {
			topValue = top.updateTop(longs[i])
		}
	}

	// Find the maximum value among the top 8.
	var max int64
	for i := 1; i <= top.size(); i++ {
		if top.get(i) > max {
			max = top.get(i)
		}
	}
	maxBitsRequired := bitsRequired(max)

	// Compute patched bits required: we need at least enough bits for
	// topValue (the 8th largest), and we cannot decrease by more than 8
	// because patches are stored as bytes.
	patchedBitsRequired := bitsRequired(topValue)
	if d := maxBitsRequired - 8; d > patchedBitsRequired {
		patchedBitsRequired = d
	}

	maxUnpatchedValue := (int64(1) << uint(patchedBitsRequired)) - 1

	// Pre-count exceptions from the heap (excluding the minimum element at
	// index 1, which is guaranteed <= maxUnpatchedValue by construction).
	numExceptions := 0
	for i := 2; i <= top.size(); i++ {
		if top.get(i) > maxUnpatchedValue {
			numExceptions++
		}
	}

	exceptions := make([]byte, numExceptions*2)

	// Work on a mutable copy: masking exceptions in-place must not corrupt
	// the caller's slice (matches Java's copy-free convention for internal
	// buffers, but Go callers may reuse the slice).
	longsCopy := make([]int64, lucene99BlockSize)
	copy(longsCopy, longs[:lucene99BlockSize])

	if numExceptions > 0 {
		exceptionCount := 0
		for i := 0; i < lucene99BlockSize; i++ {
			if longsCopy[i] > maxUnpatchedValue {
				exceptions[exceptionCount*2] = byte(i)
				exceptions[exceptionCount*2+1] = byte(uint64(longsCopy[i]) >> uint(patchedBitsRequired))
				longsCopy[i] &= maxUnpatchedValue
				exceptionCount++
			}
		}
		if exceptionCount != numExceptions {
			return errors.New("lucene99 PForUtil.encode: exception count mismatch")
		}
	}

	if l99PForAllEqual(longsCopy) && maxBitsRequired <= 8 {
		// All values are equal and small: encode as a constant VLong.
		// Pre-shift the exception high-bytes so that decode can OR them at
		// shift 0 (bitsPerValue == 0 in the token).
		for i := 0; i < numExceptions; i++ {
			exceptions[2*i+1] = byte(int(uint8(exceptions[2*i+1])) << patchedBitsRequired)
		}
		if err := out.WriteByte(byte(numExceptions << 5)); err != nil {
			return err
		}
		if err := store.WriteVLong(out, longsCopy[0]); err != nil {
			return err
		}
	} else {
		token := (numExceptions << 5) | patchedBitsRequired
		if err := out.WriteByte(byte(token)); err != nil {
			return err
		}
		if err := p.forUtil.encode(longsCopy, patchedBitsRequired, out); err != nil {
			return err
		}
	}

	return out.WriteBytes(exceptions)
}

// decode decodes 128 int64 values from in into longs. Mirrors PForUtil.decode.
func (p *lucene99PForUtil) decode(in store.IndexInput, longs []int64) error {
	if len(longs) < lucene99BlockSize {
		return errors.New("lucene99 PForUtil.decode: longs must have at least 128 elements")
	}

	tokenByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	token := int(tokenByte)
	bitsPerValue := token & 0x1F
	numExceptions := token >> 5

	if bitsPerValue == 0 {
		// All values are the same constant.
		val, err := store.ReadVLong(in)
		if err != nil {
			return err
		}
		for i := 0; i < lucene99BlockSize; i++ {
			longs[i] = val
		}
	} else {
		if err := p.forUtil.decode(bitsPerValue, in, longs); err != nil {
			return err
		}
	}

	// Apply exception patches: for each exception, OR the exception byte at
	// the stored index, shifted left by bitsPerValue.
	for i := 0; i < numExceptions; i++ {
		idxByte, err := in.ReadByte()
		if err != nil {
			return err
		}
		valByte, err := in.ReadByte()
		if err != nil {
			return err
		}
		longs[int(idxByte)] |= int64(valByte) << uint(bitsPerValue)
	}

	return nil
}

// skip skips one 128-int64 block in the input. Mirrors PForUtil.skip.
func (p *lucene99PForUtil) skip(in store.IndexInput) error {
	tokenByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	token := int(tokenByte)
	bitsPerValue := token & 0x1F
	numExceptions := token >> 5

	if bitsPerValue == 0 {
		// Constant value: skip the VLong and the exception bytes.
		if _, err := store.ReadVLong(in); err != nil {
			return err
		}
		if numExceptions > 0 {
			return skipBytesInput(in, int64(numExceptions<<1))
		}
		return nil
	}

	// ForUtil-encoded block: skip encoded bytes plus exception bytes.
	n := p.forUtil.l99forNumBytes(bitsPerValue) + (numExceptions << 1)
	if n <= 0 {
		return nil
	}
	return skipBytesInput(in, int64(n))
}
