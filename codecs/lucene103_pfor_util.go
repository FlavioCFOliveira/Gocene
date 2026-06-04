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
//	lucene103/PForUtil.java
//
// Purpose: 128-wide Patched Frame-of-Reference encode/decode/skip for the
// read-only Lucene 10.3 postings format (freqs, payload lengths, offset
// start-deltas and offset lengths).

package codecs

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene103PForMaxExceptions is the maximum number of patch exceptions.
// Mirrors PForUtil.MAX_EXCEPTIONS.
const lucene103PForMaxExceptions = 7

// lucene103PForUtil provides 128-wide Patched-FOR encode/decode.
// Mirrors backward_codecs.lucene103.PForUtil.
type lucene103PForUtil struct {
	forUtil *lucene103ForUtil
}

// newLucene103PForUtil allocates a PForUtil backed by its own 128-wide ForUtil.
// Mirrors PForUtil()'s implicit "new ForUtil()".
func newLucene103PForUtil() *lucene103PForUtil {
	return &lucene103PForUtil{forUtil: newLucene103ForUtil()}
}

// l103AllEqual reports whether every value in l[0:128] equals l[0].
// Mirrors PForUtil.allEqual.
func l103AllEqual(l []int32) bool {
	for i := 1; i < lucene103BlockSizeConst; i++ {
		if l[i] != l[0] {
			return false
		}
	}
	return true
}

// encode encodes 128 integers from ints into out. Mirrors PForUtil.encode.
func (p *lucene103PForUtil) encode(ints []int32, out store.IndexOutput) error {
	if len(ints) < lucene103BlockSizeConst {
		return errors.New("lucene103 PForUtil.encode: ints must have at least 128 elements")
	}

	// Histogram of bit widths.
	var histogram [32]int
	maxBitsRequired := 0
	for i := 0; i < lucene103BlockSizeConst; i++ {
		b := bitsRequired(int64(ints[i]))
		histogram[b]++
		if b > maxBitsRequired {
			maxBitsRequired = b
		}
	}

	// We store patch on a byte, so we can't decrease bits by more than 8.
	minBits := maxBitsRequired - 8
	if minBits < 0 {
		minBits = 0
	}
	cumulativeExceptions := 0
	patchedBitsRequired := maxBitsRequired
	numExceptions := 0
	for b := maxBitsRequired; b >= minBits; b-- {
		if cumulativeExceptions > lucene103PForMaxExceptions {
			break
		}
		patchedBitsRequired = b
		numExceptions = cumulativeExceptions
		cumulativeExceptions += histogram[b]
	}

	maxUnpatchedValue := int32((1 << patchedBitsRequired) - 1)
	exceptions := make([]byte, numExceptions*2)

	// Work on a mutable copy: masking exceptions in-place must not corrupt the
	// caller's slice (matches Java, where ints is the writer's own buffer but
	// Gocene callers may reuse the slice).
	intsCopy := make([]int32, lucene103BlockSizeConst)
	copy(intsCopy, ints[:lucene103BlockSizeConst])

	if numExceptions > 0 {
		exceptionCount := 0
		for i := 0; i < lucene103BlockSizeConst; i++ {
			if intsCopy[i] > maxUnpatchedValue {
				exceptions[exceptionCount*2] = byte(i)
				exceptions[exceptionCount*2+1] = byte(int32(uint32(intsCopy[i]) >> uint(patchedBitsRequired)))
				intsCopy[i] &= maxUnpatchedValue
				exceptionCount++
			}
		}
		if exceptionCount != numExceptions {
			return errors.New("lucene103 PForUtil.encode: exception count mismatch")
		}
	}

	if l103AllEqual(intsCopy) && maxBitsRequired <= 8 {
		for i := 0; i < numExceptions; i++ {
			exceptions[2*i+1] = byte(int(uint8(exceptions[2*i+1])) << patchedBitsRequired)
		}
		if err := out.WriteByte(byte(numExceptions << 5)); err != nil {
			return err
		}
		if err := store.WriteVInt(out, intsCopy[0]); err != nil {
			return err
		}
	} else {
		token := (numExceptions << 5) | patchedBitsRequired
		if err := out.WriteByte(byte(token)); err != nil {
			return err
		}
		if err := p.forUtil.encode(intsCopy, patchedBitsRequired, out); err != nil {
			return err
		}
	}

	return out.WriteBytes(exceptions)
}

// decode decodes 128 integers from in into ints. Mirrors PForUtil.decode.
func (p *lucene103PForUtil) decode(in store.IndexInput, ints []int64) error {
	if len(ints) < lucene103BlockSizeConst {
		return errors.New("lucene103 PForUtil.decode: ints must have at least 128 elements")
	}
	tokenByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	token := int(tokenByte)
	bitsPerValue := token & 0x1F
	if bitsPerValue == 0 {
		val, err2 := store.ReadVInt(in)
		if err2 != nil {
			return err2
		}
		for i := 0; i < lucene103BlockSizeConst; i++ {
			ints[i] = int64(val)
		}
	} else {
		if err2 := p.forUtil.decode(bitsPerValue, in, ints); err2 != nil {
			return err2
		}
	}
	numExceptions := token >> 5
	for i := 0; i < numExceptions; i++ {
		idxByte, err2 := in.ReadByte()
		if err2 != nil {
			return err2
		}
		valByte, err3 := in.ReadByte()
		if err3 != nil {
			return err3
		}
		ints[int(idxByte)] |= int64(valByte) << uint(bitsPerValue)
	}
	return nil
}

// lucene103PForUtilSkip skips one 128-integer block in in.
// Mirrors PForUtil.skip(DataInput).
func lucene103PForUtilSkip(in store.IndexInput) error {
	tokenByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	token := int(tokenByte)
	bitsPerValue := token & 0x1F
	numExceptions := token >> 5
	if bitsPerValue == 0 {
		if _, err2 := store.ReadVLong(in); err2 != nil {
			return err2
		}
		return skipBytesInput(in, int64(numExceptions<<1))
	}
	return skipBytesInput(in, int64(lucene103ForNumBytes(bitsPerValue)+(numExceptions<<1)))
}
