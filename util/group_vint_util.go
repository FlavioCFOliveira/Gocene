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

package util

import "fmt"

// GroupVInt is the Go port of org.apache.lucene.util.GroupVIntUtil.
//
// Group-varint encoding packs four 32-bit unsigned integers into a
// "group". The group begins with a single control byte that stores the
// byte-length minus one of each of the four packed integers in 2-bit
// fields (so each length is in [1, 4]). The four little-endian payloads
// follow concatenated. A 32-bit value therefore costs between 5 bytes
// (four 1-byte values) and 17 bytes (four 4-byte values).
//
// The control byte layout (LSB first):
//
//	bits 0-1 : len(value[0]) - 1
//	bits 2-3 : len(value[1]) - 1
//	bits 4-5 : len(value[2]) - 1
//	bits 6-7 : len(value[3]) - 1
//
// Values that exceed 32 bits cannot be encoded by GroupVInt; callers
// must ensure values fit in uint32. Values that are not multiples of
// four in count must be encoded in two passes: complete groups via
// GroupVIntEncode, then the trailing 1-3 values via a regular VInt.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/GroupVIntUtil.java
const GroupVIntMaxBytesPerValue = 4

// GroupVIntEncode appends the group-varint encoding of exactly four
// uint32 values from src starting at index off into dst, returning the
// new dst slice. Callers must supply exactly four values per call.
func GroupVIntEncode(dst []byte, src []uint32, off int) ([]byte, error) {
	if off+4 > len(src) {
		return nil, fmt.Errorf("source slice too short: off=%d len=%d need 4 values", off, len(src))
	}
	lens := [4]int{
		vintLen32(src[off]),
		vintLen32(src[off+1]),
		vintLen32(src[off+2]),
		vintLen32(src[off+3]),
	}
	control := byte((lens[0]-1)&0x3) |
		byte((lens[1]-1)&0x3)<<2 |
		byte((lens[2]-1)&0x3)<<4 |
		byte((lens[3]-1)&0x3)<<6
	dst = append(dst, control)
	for i := 0; i < 4; i++ {
		v := src[off+i]
		l := lens[i]
		for b := 0; b < l; b++ {
			dst = append(dst, byte(v>>uint(8*b)))
		}
	}
	return dst, nil
}

// GroupVIntDecode reads one group-varint group from src starting at off,
// writes the four decoded uint32 values into dst at off, and returns
// the number of bytes consumed from src.
func GroupVIntDecode(dst []uint32, src []byte, srcOff, dstOff int) (int, error) {
	if srcOff >= len(src) {
		return 0, fmt.Errorf("source slice exhausted: off=%d len=%d", srcOff, len(src))
	}
	if dstOff+4 > len(dst) {
		return 0, fmt.Errorf("destination too short: off=%d len=%d need 4 slots", dstOff, len(dst))
	}
	control := src[srcOff]
	lens := [4]int{
		int(control&0x3) + 1,
		int((control>>2)&0x3) + 1,
		int((control>>4)&0x3) + 1,
		int((control>>6)&0x3) + 1,
	}
	total := 1
	for i := 0; i < 4; i++ {
		total += lens[i]
	}
	if srcOff+total > len(src) {
		return 0, fmt.Errorf("source slice truncated: have %d, need %d",
			len(src)-srcOff, total)
	}
	p := srcOff + 1
	for i := 0; i < 4; i++ {
		var v uint32
		for b := 0; b < lens[i]; b++ {
			v |= uint32(src[p+b]) << uint(8*b)
		}
		dst[dstOff+i] = v
		p += lens[i]
	}
	return total, nil
}

// vintLen32 returns the number of bytes required to encode v using the
// group-varint length convention (1..4 bytes per uint32).
func vintLen32(v uint32) int {
	switch {
	case v < 1<<8:
		return 1
	case v < 1<<16:
		return 2
	case v < 1<<24:
		return 3
	default:
		return 4
	}
}

// GroupVIntMaxBytes returns the maximum number of bytes that a group of
// n values can occupy when encoded with the group-varint scheme.
// A complete group of four values occupies at most 1 + 4*4 = 17 bytes.
// Trailing values (count not divisible by 4) are conventionally encoded
// via a regular VInt and are not accounted for here.
func GroupVIntMaxBytes(n int) int {
	groups := n / 4
	return groups * 17
}
