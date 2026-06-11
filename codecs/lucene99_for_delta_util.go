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
//	lucene99/ForDeltaUtil.java
//
// Purpose: 128-wide FOR-delta encode/decode for the Lucene 9.9 backward-codecs
// postings format (long-based, BLOCK_SIZE=128). encodeDeltas writes the bit
// width followed by packed FOR data; decodeAndPrefixSum fuses the FOR decode
// of doc-delta blocks with the prefix-sum that turns deltas back into absolute
// doc IDs. This is the int64 variant used by the Lucene99 postings format,
// distinct from the int32-based Lucene103 ForDeltaUtil.

package codecs

import (
	"errors"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// l99identityPlusOne is a pre-computed table where entry i == int64(i + 1).
// Used by decodeAndPrefixSum when the encoded deltas are all 1 (dense
// postings — every doc ID is exactly one greater than the previous).
var l99identityPlusOne [lucene99BlockSize]int64

func init() {
	for i := 0; i < lucene99BlockSize; i++ {
		l99identityPlusOne[i] = int64(i + 1)
	}
}

// allEqual64 returns true iff all 128 values in vals are equal.
// The caller MUST ensure vals has at least 128 elements.
// Mirrors PForUtil.allEqual(long[]).
func allEqual64(vals []int64) bool {
	for i := 1; i < lucene99BlockSize; i++ {
		if vals[i] != vals[0] {
			return false
		}
	}
	return true
}

// l99bitsRequired returns the minimum number of bits required to represent v
// as an unsigned value. Returns at least 1.
//
// Under the hood: bits.Len64(uint64(v)) gives the position of the highest set
// bit (0->0, 1->1, 2->2, 3->2, 4->3, ...). Java's unsignedBitsRequired does
// Math.max(1, 64 - Long.numberOfLeadingZeros(v)), which is identical except
// for v=0 (bits.Len64 returns 0; we clamp to 1).
//
// Mirrors PackedInts.bitsRequired(long) / unsignedBitsRequired(long).
func l99bitsRequired(v int64) int {
	n := bits.Len64(uint64(v))
	if n == 0 {
		return 1
	}
	return n
}

// lucene99ForDeltaUtil wraps lucene99ForUtil for delta encoding/decoding of
// 128-long blocks. Mirrors backward_codecs.lucene99.ForDeltaUtil.
type lucene99ForDeltaUtil struct {
	forUtil *lucene99ForUtil
}

// newLucene99ForDeltaUtil creates a new delta utility that delegates to the
// given ForUtil for the underlying FOR encode/decode operations.
func newLucene99ForDeltaUtil(forUtil *lucene99ForUtil) *lucene99ForDeltaUtil {
	return &lucene99ForDeltaUtil{forUtil: forUtil}
}

// encodeDeltas encodes 128 delta values from longs into out.
//
// If all deltas are exactly 1 (very dense postings), a single zero byte is
// written as a sentinel. Otherwise the bit width is computed via OR-reduction
// and written as a byte, followed by the packed FOR-encoded data.
//
// Mirrors ForDeltaUtil.encodeDeltas.
func (f *lucene99ForDeltaUtil) encodeDeltas(longs []int64, out store.IndexOutput) error {
	if len(longs) < lucene99BlockSize {
		return errors.New("lucene99 ForDeltaUtil.encodeDeltas: longs must have at least 128 elements")
	}
	if longs[0] == 1 && allEqual64(longs) {
		return out.WriteByte(0)
	}
	var or int64
	for _, l := range longs[:lucene99BlockSize] {
		or |= l
	}
	if or == 0 {
		return errors.New("lucene99 ForDeltaUtil: all deltas are zero")
	}
	bitsPerValue := l99bitsRequired(or)
	if err := out.WriteByte(byte(bitsPerValue)); err != nil {
		return err
	}
	return f.forUtil.encode(longs, bitsPerValue, out)
}

// decodeAndPrefixSum decodes 128 delta-encoded values from in into longs,
// then computes the prefix sum with the given base so that longs holds
// absolute values on return.
//
// When bitsPerValue is 0 (meaning the deltas were all 1 during encoding),
// it fills longs with the sequence [base+1, base+2, ..., base+128] directly
// without calling into ForUtil.
//
// Mirrors ForDeltaUtil.decodeAndPrefixSum.
func (f *lucene99ForDeltaUtil) decodeAndPrefixSum(in store.IndexInput, base int64, longs []int64) error {
	if len(longs) < lucene99BlockSize {
		return errors.New("lucene99 ForDeltaUtil.decodeAndPrefixSum: longs must have at least 128 elements")
	}
	b, err := in.ReadByte()
	if err != nil {
		return err
	}
	bitsPerValue := int(b)
	if bitsPerValue == 0 {
		copy(longs[:lucene99BlockSize], l99identityPlusOne[:])
		for i := 0; i < lucene99BlockSize; i++ {
			longs[i] += base
		}
		return nil
	}
	return f.forUtil.decodeAndPrefixSum(bitsPerValue, in, base, longs)
}
