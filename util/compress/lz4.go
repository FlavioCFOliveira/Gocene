// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.
//
// The LZ4 algorithm itself is:
//
//   LZ4 Library
//   Copyright (c) 2011-2016, Yann Collet
//   All rights reserved.
//   BSD 2-Clause License.

package compress

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// LZ4 compression and decompression routines.
//
// References:
//
//   - https://github.com/lz4/lz4/tree/dev/lib
//   - http://fastcompression.blogspot.fr/p/lz4.html
//
// The high-compression option is a simpler version of the one of the original
// algorithm, and only retains a better hash table that remembers about more
// occurrences of a previous 4-bytes sequence, and removes all the logic about
// handling of the case when overlapping matches are found.
//
// This is the Go port of org.apache.lucene.util.compress.LZ4 from Apache
// Lucene 10.4.0. The on-disk byte format is identical to the Java reference
// (4-bit literal-len nibble | 4-bit match-len nibble, 0xFF-run continuation
// bytes, and a 16-bit little-endian match offset).

// MaxDistance is the window size: the maximum supported distance between two
// strings so that LZ4 can replace the second one by a reference to the first
// one.
const MaxDistance = 1 << 16 // 65536

// Internal compression tuning constants. Names mirror the Lucene Java source
// so the algorithm reads identically when compared side-by-side.
const (
	memoryUsage      = 14
	minMatch         = 4  // minimum length of a match
	lastLiterals     = 5  // the last 5 bytes must be encoded as literals
	hashLogHC        = 15 // log size of the dictionary for high-compression
	hashTableSizeHC  = 1 << hashLogHC
	maxAttemptsHC    = 256
	maskHC           = MaxDistance - 1
	hcChainEmpty     = 0xFFFF // sentinel in the HC chain table (unsigned 16-bit -1)
	hcHashTableEmpty = -1     // sentinel in the HC hash table
)

// hashI returns the LZ4 hash of i clamped to hashBits bits. The multiplier is
// the same magic constant used by the reference LZ4 implementation; the
// shift is computed against an unsigned 32-bit interpretation to match
// Java's `>>>` (logical right shift) semantics.
func hashI(i int32, hashBits int) int {
	// Java: (i * -1640531535) >>> (32 - hashBits)
	// In Go we cast through uint32 to get the same logical-shift behaviour.
	u := uint32(i) * 2654435769 // 2654435769 == uint32(-1640531535)
	return int(u >> uint(32-hashBits))
}

func hashHC(i int32) int { return hashI(i, hashLogHC) }

// readInt32LE reads 4 bytes from b starting at offset i and interprets them as
// a signed little-endian 32-bit integer. According to LZ4's algorithm the
// endianness of the read does not matter for correctness of compression
// itself, only that it is consistent within a single Compress invocation.
// The Java code uses VH_NATIVE_INT (LITTLE_ENDIAN in the Lucene module), so
// we mirror that exactly to keep hash-table collision sets identical and
// thus the encoded byte stream identical when matches are picked.
func readInt32LE(b []byte, i int) int32 {
	return int32(binary.LittleEndian.Uint32(b[i:]))
}

// commonBytes returns the number of bytes that are equal in b starting from
// offsets o1 and o2, up to the given limit (exclusive end of b's region to
// inspect for the longer of the two starts).
//
// Mirrors `Arrays.mismatch(b, o1, limit, b, o2, limit)` in the Java source:
// the comparison continues until either end is reached or a mismatch is
// found.
func commonBytes(b []byte, o1, o2, limit int) int {
	// Java's Arrays.mismatch on two slices of identical effective range
	// returns the number of matching elements before the first mismatch
	// (and the full common length if they match entirely).
	maxLen := limit - o2
	if other := limit - o1; other < maxLen {
		maxLen = other
	}
	n := 0
	for n < maxLen && b[o1+n] == b[o2+n] {
		n++
	}
	return n
}

// HashTable is a record of previous occurrences of sequences of 4 bytes used
// by Compress to find matches. Two implementations are provided in this file:
// FastCompressionHashTable (small memory footprint, lossy) and
// HighCompressionHashTable (denser, slower).
//
// HashTable values must not be shared across concurrent Compress calls but
// can be safely reused sequentially.
type HashTable interface {
	// Reset resets this hash table in order to compress the given content.
	Reset(b []byte, off, length int)
	// InitDictionary initialises the first dictLen bytes of the buffer
	// passed to Reset as a preset dictionary.
	InitDictionary(dictLen int)
	// Get advances the cursor to off and returns an index that stores the
	// same 4 bytes as b[off:off+4]. This may only be called on strictly
	// increasing sequences of offsets. A return value of -1 indicates that
	// no other index could be found.
	Get(off int) int
	// Previous returns an index that is less than off and stores the same
	// 4 bytes as b[off:off+4]. Unlike Get, it does not need to be called
	// on increasing offsets. A return value of -1 indicates no match.
	Previous(off int) int
	// assertReset is a test-only invariant check.
	assertReset() bool
}

// table abstracts a fixed-size mapping of hash slots to previous offsets. In
// the Java source this is a sealed inner class hierarchy (Table16, Table32);
// in Go we model it as a small interface to keep the same dispatch.
type table interface {
	set(index, value int)
	getAndSet(index, value int) int
	bitsPerValue() int
	size() int
}

// table16 stores 16 bits per offset. This is by far the most commonly used
// table since it gets used whenever compressing inputs whose size is <= 64kB.
type table16 struct {
	t []uint16
}

func newTable16(size int) *table16 { return &table16{t: make([]uint16, size)} }

func (t *table16) set(index, value int) {
	// value must fit in 16 bits; the caller (FastCompressionHashTable.Get)
	// guarantees this because offsets relative to base are < MaxDistance
	// (which is exactly 1<<16) when the bits-per-offset is 16.
	t.t[index] = uint16(value)
}

func (t *table16) getAndSet(index, value int) int {
	prev := int(t.t[index])
	t.t[index] = uint16(value)
	return prev
}

func (*table16) bitsPerValue() int { return 16 }
func (t *table16) size() int       { return len(t.t) }

// table32 stores 32 bits per offset. Used only when inputs exceed 64kB.
type table32 struct {
	t []int32
}

func newTable32(size int) *table32 { return &table32{t: make([]int32, size)} }

func (t *table32) set(index, value int) { t.t[index] = int32(value) }
func (t *table32) getAndSet(index, value int) int {
	prev := int(t.t[index])
	t.t[index] = int32(value)
	return prev
}
func (*table32) bitsPerValue() int { return 32 }
func (t *table32) size() int       { return len(t.t) }

// FastCompressionHashTable is a simple lossy HashTable that only stores the
// last occurrence for each hash on 2^14 bytes of memory. This is the default
// choice for fast (real-time-ish) compression scenarios.
type FastCompressionHashTable struct {
	bytes   []byte
	base    int
	lastOff int
	end     int
	hashLog int
	tbl     table
}

// NewFastCompressionHashTable returns a fresh FastCompressionHashTable.
func NewFastCompressionHashTable() *FastCompressionHashTable {
	return &FastCompressionHashTable{}
}

// Reset initialises this hash table for a new compression run over
// b[off:off+length].
func (h *FastCompressionHashTable) Reset(b []byte, off, length int) {
	if off < 0 || length < 0 || off+length > len(b) {
		// Mirror Objects.checkFromIndexSize: a programming error from the
		// caller, not a recoverable runtime condition.
		panic(fmt.Sprintf("FastCompressionHashTable.Reset: off=%d length=%d len(b)=%d", off, length, len(b)))
	}
	h.bytes = b
	h.base = off
	h.end = off + length

	var bitsPerOffset int
	if length-lastLiterals < 1<<16 {
		bitsPerOffset = 16
	} else {
		bitsPerOffset = 32
	}
	// bitsPerOffsetLog = 32 - leadingZeros(bitsPerOffset - 1).
	// For bitsPerOffset = 16 this is 4; for 32 it is 5.
	bitsPerOffsetLog := bitsForValue(bitsPerOffset - 1)
	h.hashLog = memoryUsage + 3 - bitsPerOffsetLog

	wantSize := 1 << h.hashLog
	if h.tbl == nil || h.tbl.size() < wantSize || h.tbl.bitsPerValue() < bitsPerOffset {
		if bitsPerOffset > 16 {
			h.tbl = newTable32(wantSize)
		} else {
			h.tbl = newTable16(wantSize)
		}
	}
	// Note: we intentionally avoid clearing the table. Get() checks that the
	// returned reference is < the current offset before accepting it as a
	// match, which makes residual values from previous Reset calls harmless.

	h.lastOff = off - 1
}

// InitDictionary primes the hash table with the dictLen bytes starting at
// h.base. Must be called after Reset.
func (h *FastCompressionHashTable) InitDictionary(dictLen int) {
	for i := 0; i < dictLen; i++ {
		v := readInt32LE(h.bytes, h.base+i)
		hh := hashI(v, h.hashLog)
		h.tbl.set(hh, i)
	}
	h.lastOff += dictLen
}

// Get returns the previous offset (in h.bytes) that stores the same 4 bytes
// as b[off:off+4], or -1 if no such match exists within MaxDistance.
func (h *FastCompressionHashTable) Get(off int) int {
	if off <= h.lastOff {
		panic(fmt.Sprintf("FastCompressionHashTable.Get: off=%d <= lastOff=%d", off, h.lastOff))
	}
	if off >= h.end {
		panic(fmt.Sprintf("FastCompressionHashTable.Get: off=%d >= end=%d", off, h.end))
	}

	v := readInt32LE(h.bytes, off)
	hh := hashI(v, h.hashLog)

	ref := h.base + h.tbl.getAndSet(hh, off-h.base)
	h.lastOff = off

	if ref < off && off-ref < MaxDistance && readInt32LE(h.bytes, ref) == v {
		return ref
	}
	return -1
}

// Previous always returns -1 for the fast variant: it has no chain.
func (*FastCompressionHashTable) Previous(int) int { return -1 }

func (*FastCompressionHashTable) assertReset() bool { return true }

// HighCompressionHashTable is a higher-precision HashTable. It stores up to
// MaxAttemptsHC (256) occurrences of 4-bytes sequences in the last 2^16
// bytes, which makes it much more likely to find matches than
// FastCompressionHashTable at the cost of more CPU and memory.
type HighCompressionHashTable struct {
	bytes      []byte
	base       int
	next       int
	end        int
	hashTable  []int32  // length hashTableSizeHC
	chainTable []uint16 // length MaxDistance, ring-buffer indexed by off & maskHC
	attempts   int
}

// NewHighCompressionHashTable returns a fresh HighCompressionHashTable with
// both tables initialised to their empty sentinels.
func NewHighCompressionHashTable() *HighCompressionHashTable {
	h := &HighCompressionHashTable{
		hashTable:  make([]int32, hashTableSizeHC),
		chainTable: make([]uint16, MaxDistance),
	}
	for i := range h.hashTable {
		h.hashTable[i] = hcHashTableEmpty
	}
	for i := range h.chainTable {
		h.chainTable[i] = hcChainEmpty
	}
	return h
}

// Reset initialises this hash table for a new compression run.
func (h *HighCompressionHashTable) Reset(b []byte, off, length int) {
	if off < 0 || length < 0 || off+length > len(b) {
		panic(fmt.Sprintf("HighCompressionHashTable.Reset: off=%d length=%d len(b)=%d", off, length, len(b)))
	}
	// Mirror the Java optimisation: when the previous run covered < 64kB we
	// can skip clearing the hash table and only sweep the parts of the chain
	// table actually touched. The Java condition compares the *previous*
	// run's extent — which we encode as (h.end - h.base) before mutating.
	prevExtent := h.end - h.base
	if prevExtent < len(h.chainTable) {
		startOffset := h.base & maskHC
		var endOffset int
		if h.end == 0 {
			endOffset = 0
		} else {
			endOffset = ((h.end - 1) & maskHC) + 1
		}
		if startOffset < endOffset {
			for i := startOffset; i < endOffset; i++ {
				h.chainTable[i] = hcChainEmpty
			}
		} else {
			for i := 0; i < endOffset; i++ {
				h.chainTable[i] = hcChainEmpty
			}
			for i := startOffset; i < len(h.chainTable); i++ {
				h.chainTable[i] = hcChainEmpty
			}
		}
	} else {
		for i := range h.hashTable {
			h.hashTable[i] = hcHashTableEmpty
		}
		for i := range h.chainTable {
			h.chainTable[i] = hcChainEmpty
		}
	}
	h.bytes = b
	h.base = off
	h.next = off
	h.end = off + length
}

// InitDictionary primes the hash table with the dictLen bytes starting at
// h.base. Must be called after Reset.
func (h *HighCompressionHashTable) InitDictionary(dictLen int) {
	if h.next != h.base {
		panic("HighCompressionHashTable.InitDictionary: next != base")
	}
	for i := 0; i < dictLen; i++ {
		h.addHash(h.base + i)
	}
	h.next += dictLen
}

// Get returns the previous offset that stores the same 4 bytes as b[off:],
// walking the chain up to maxAttemptsHC links back, or -1 if no match is
// found within MaxDistance.
func (h *HighCompressionHashTable) Get(off int) int {
	if off < h.next {
		panic(fmt.Sprintf("HighCompressionHashTable.Get: off=%d < next=%d", off, h.next))
	}
	if off >= h.end {
		panic(fmt.Sprintf("HighCompressionHashTable.Get: off=%d >= end=%d", off, h.end))
	}

	for ; h.next < off; h.next++ {
		h.addHash(h.next)
	}

	v := readInt32LE(h.bytes, off)
	hh := hashHC(v)

	h.attempts = 0
	ref := int(h.hashTable[hh])
	if ref >= off {
		// Stale residue from a previous Compress call: ignore.
		return -1
	}
	min := h.base
	if floor := off - MaxDistance + 1; floor > min {
		min = floor
	}
	for ; ref >= min && h.attempts < maxAttemptsHC; h.attempts++ {
		if readInt32LE(h.bytes, ref) == v {
			return ref
		}
		ref -= int(h.chainTable[ref&maskHC])
	}
	return -1
}

// addHash inserts off into the chain at its hash slot. Both the head
// (hashTable) and the prev-delta link (chainTable) are updated.
func (h *HighCompressionHashTable) addHash(off int) {
	v := readInt32LE(h.bytes, off)
	hh := hashHC(v)
	delta := off - int(h.hashTable[hh])
	if delta <= 0 || delta >= MaxDistance {
		delta = MaxDistance - 1
	}
	h.chainTable[off&maskHC] = uint16(delta)
	h.hashTable[hh] = int32(off)
}

// Previous walks the chain starting from off backwards, looking for a prior
// offset whose 4 bytes match b[off:off+4]. It contributes to the same
// attempts budget as Get (the budget is reset by Get).
func (h *HighCompressionHashTable) Previous(off int) int {
	v := readInt32LE(h.bytes, off)
	ref := off - int(h.chainTable[off&maskHC])
	for ; ref >= h.base && h.attempts < maxAttemptsHC; h.attempts++ {
		if readInt32LE(h.bytes, ref) == v {
			return ref
		}
		ref -= int(h.chainTable[ref&maskHC])
	}
	return -1
}

func (h *HighCompressionHashTable) assertReset() bool {
	for i, v := range h.chainTable {
		if v != hcChainEmpty {
			panic(fmt.Sprintf("HighCompressionHashTable.assertReset: chainTable[%d]=%#x", i, v))
		}
	}
	return true
}

// ErrInvalidOffset is returned by Decompress when the encoded stream
// references a match with offset 0 (which is never produced by Compress).
var ErrInvalidOffset = errors.New("lz4: offset 0 is invalid")

// LZ4Decompress decodes at least decompressedLen bytes from compressed into
// dest[dOff:]. dest must be large enough to hold all decompressed data
// (meaning that you need to know the total decompressed length).
//
// If the given bytes were compressed using a preset dictionary then the
// same dictionary must be provided in dest[dOff-dictLen:dOff].
//
// Returns the offset in dest that points just past the last byte written.
//
// This is the Go equivalent of org.apache.lucene.util.compress.LZ4.decompress.
func LZ4Decompress(compressed store.DataInput, decompressedLen int, dest []byte, dOff int) (int, error) {
	destEnd := dOff + decompressedLen
	if destEnd > len(dest) {
		return dOff, fmt.Errorf("lz4: dest buffer too small: need %d, have %d", destEnd, len(dest))
	}

	for {
		// literals
		tokenByte, err := compressed.ReadByte()
		if err != nil {
			return dOff, err
		}
		token := int(tokenByte) & 0xFF
		literalLen := token >> 4

		if literalLen != 0 {
			if literalLen == 0x0F {
				for {
					lenByte, err := compressed.ReadByte()
					if err != nil {
						return dOff, err
					}
					if lenByte != 0xFF {
						literalLen += int(lenByte) & 0xFF
						break
					}
					literalLen += 0xFF
				}
			}
			if dOff+literalLen > len(dest) {
				return dOff, fmt.Errorf("lz4: literal run overflows dest (off=%d len=%d dest=%d)", dOff, literalLen, len(dest))
			}
			if err := compressed.ReadBytes(dest[dOff : dOff+literalLen]); err != nil {
				return dOff, err
			}
			dOff += literalLen
		}

		if dOff >= destEnd {
			break
		}

		// match
		matchDecShort, err := compressed.ReadShort()
		if err != nil {
			return dOff, err
		}
		matchDec := int(uint16(matchDecShort))
		if matchDec == 0 {
			return dOff, ErrInvalidOffset
		}

		matchLen := token & 0x0F
		if matchLen == 0x0F {
			for {
				lenByte, err := compressed.ReadByte()
				if err != nil {
					return dOff, err
				}
				if lenByte != 0xFF {
					matchLen += int(lenByte) & 0xFF
					break
				}
				matchLen += 0xFF
			}
		}
		matchLen += minMatch

		// Copying a multiple of 8 bytes can make decompression up to 10%
		// faster when the regions don't overlap. We mirror the heuristic
		// from the Java source.
		fastLen := (matchLen + 7) & 0x7FFFFFF8
		if matchDec < matchLen || dOff+fastLen > destEnd {
			// overlap or not enough headroom for the rounded-up copy:
			// naive incremental byte-by-byte copy is required because
			// later bytes of the match may depend on bytes copied earlier
			// in this same pass.
			ref := dOff - matchDec
			end := dOff + matchLen
			for ; dOff < end; ref, dOff = ref+1, dOff+1 {
				dest[dOff] = dest[ref]
			}
		} else {
			copy(dest[dOff:dOff+fastLen], dest[dOff-matchDec:])
			dOff += matchLen
		}

		if dOff >= destEnd {
			break
		}
	}

	return dOff, nil
}

// encodeLen writes the continuation-byte tail of a length field after the
// 4-bit nibble in the token has already been set to 0x0F. The remainder is
// emitted as a run of 0xFF bytes followed by the final residue byte.
func encodeLen(l int, out store.DataOutput) error {
	for l >= 0xFF {
		if err := out.WriteByte(0xFF); err != nil {
			return err
		}
		l -= 0xFF
	}
	return out.WriteByte(byte(l))
}

// encodeLiterals writes the token byte, the continuation bytes for the
// literal length (if any), and the literals themselves.
func encodeLiterals(b []byte, token, anchor, literalLen int, out store.DataOutput) error {
	if err := out.WriteByte(byte(token)); err != nil {
		return err
	}
	if literalLen >= 0x0F {
		if err := encodeLen(literalLen-0x0F, out); err != nil {
			return err
		}
	}
	if literalLen > 0 {
		if err := out.WriteBytes(b[anchor : anchor+literalLen]); err != nil {
			return err
		}
	}
	return nil
}

// encodeLastLiterals writes the trailing literal-only sequence at the end of
// the stream (no match follows, so only the high nibble of the token is
// populated).
func encodeLastLiterals(b []byte, anchor, literalLen int, out store.DataOutput) error {
	hi := literalLen
	if hi > 0x0F {
		hi = 0x0F
	}
	token := hi << 4
	return encodeLiterals(b, token, anchor, literalLen, out)
}

// writeShortLE writes v as a little-endian 16-bit value. We bypass
// DataOutput.WriteShort because some buffered implementations in this code
// base emit big-endian shorts; for LZ4 wire compatibility with Lucene
// 10.4.0 the match offset MUST be little-endian.
func writeShortLE(out store.DataOutput, v uint16) error {
	if err := out.WriteByte(byte(v)); err != nil {
		return err
	}
	return out.WriteByte(byte(v >> 8))
}

// encodeSequence writes a literal+match pair: token, literal-len
// continuation bytes, literals, 2-byte little-endian match offset, and any
// match-length continuation bytes.
func encodeSequence(b []byte, anchor, matchRef, matchOff, matchLen int, out store.DataOutput) error {
	literalLen := matchOff - anchor
	if matchLen < minMatch {
		return fmt.Errorf("lz4: encodeSequence matchLen=%d < %d", matchLen, minMatch)
	}
	hi := literalLen
	if hi > 0x0F {
		hi = 0x0F
	}
	lo := matchLen - minMatch
	if lo > 0x0F {
		lo = 0x0F
	}
	token := (hi << 4) | lo
	if err := encodeLiterals(b, token, anchor, literalLen, out); err != nil {
		return err
	}

	matchDec := matchOff - matchRef
	if matchDec <= 0 || matchDec >= MaxDistance {
		return fmt.Errorf("lz4: encodeSequence matchDec=%d out of (0,%d)", matchDec, MaxDistance)
	}
	if err := writeShortLE(out, uint16(matchDec)); err != nil {
		return err
	}

	if matchLen >= minMatch+0x0F {
		if err := encodeLen(matchLen-0x0F-minMatch, out); err != nil {
			return err
		}
	}
	return nil
}

// LZ4Compress encodes bytes[off:off+length] into out using at most 16kB of
// memory. ht must not be shared across concurrent calls but can safely be
// reused sequentially.
//
// This is the Go equivalent of org.apache.lucene.util.compress.LZ4.compress.
func LZ4Compress(bytes []byte, off, length int, out store.DataOutput, ht HashTable) error {
	return LZ4CompressWithDictionary(bytes, off, 0, length, out, ht)
}

// LZ4CompressWithDictionary encodes
// bytes[dictOff+dictLen:dictOff+dictLen+length] into out using at most 16kB
// of memory. bytes[dictOff:dictOff+dictLen] is used as a preset dictionary.
// dictLen must not be greater than MaxDistance (64kB), the maximum window
// size.
//
// ht must not be shared across concurrent calls but can safely be reused
// sequentially.
//
// This is the Go equivalent of
// org.apache.lucene.util.compress.LZ4.compressWithDictionary.
func LZ4CompressWithDictionary(bytes []byte, dictOff, dictLen, length int, out store.DataOutput, ht HashTable) error {
	if dictOff < 0 || dictLen < 0 || dictOff+dictLen > len(bytes) {
		return fmt.Errorf("lz4: dictOff=%d dictLen=%d len(bytes)=%d (out of bounds)", dictOff, dictLen, len(bytes))
	}
	if length < 0 || dictOff+dictLen+length > len(bytes) {
		return fmt.Errorf("lz4: length=%d dictOff=%d dictLen=%d len(bytes)=%d (out of bounds)", length, dictOff, dictLen, len(bytes))
	}
	if dictLen > MaxDistance {
		return fmt.Errorf("lz4: dictLen must not be greater than 64kB, got %d", dictLen)
	}

	end := dictOff + dictLen + length

	offset := dictOff + dictLen
	anchor := offset

	if length > lastLiterals+minMatch {
		limit := end - lastLiterals
		matchLimit := limit - minMatch
		ht.Reset(bytes, dictOff, dictLen+length)
		ht.InitDictionary(dictLen)

	mainLoop:
		for offset <= limit {
			// Find a match.
			var ref int
			for {
				if offset >= matchLimit {
					break mainLoop
				}
				ref = ht.Get(offset)
				if ref != -1 {
					break
				}
				offset++
			}

			// Compute match length.
			matchLen := minMatch + commonBytes(bytes, ref+minMatch, offset+minMatch, limit)

			// Try to find a better match by walking the chain (HC only;
			// FastCompressionHashTable.Previous always returns -1).
			min := offset - MaxDistance + 1
			if min < dictOff {
				min = dictOff
			}
			for r := ht.Previous(ref); r >= min; r = ht.Previous(r) {
				rMatchLen := minMatch + commonBytes(bytes, r+minMatch, offset+minMatch, limit)
				if rMatchLen > matchLen {
					ref = r
					matchLen = rMatchLen
				}
			}

			if err := encodeSequence(bytes, anchor, ref, offset, matchLen, out); err != nil {
				return err
			}
			offset += matchLen
			anchor = offset
		}
	}

	// Last literals.
	literalLen := end - anchor
	return encodeLastLiterals(bytes, anchor, literalLen, out)
}

// bitsForValue returns 32 - leadingZeros32(v). For v == 0 it returns 0,
// matching Java's Integer.numberOfLeadingZeros semantics. This is the
// minimum number of bits needed to represent v.
func bitsForValue(v int) int {
	if v == 0 {
		return 0
	}
	n := 0
	for v != 0 {
		n++
		v >>= 1
	}
	return n
}
