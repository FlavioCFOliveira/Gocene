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
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of org.apache.lucene.util.bkd.DocIdsWriter (Lucene 10.4.0).
//
// DocIdsWriter encodes the docIDs found in a single leaf block using
// one of several layouts and writes them through a DataOutput, then
// decodes them back through an IndexInput. The encoding chosen
// depends on the docIDs themselves: continuous runs, sorted ids with
// low cardinality, deltas that fit in 16 bits, and so on. The
// dispatch byte at the head of each block identifies which layout
// follows.
//
// Wire-format compatibility with Lucene 10.4.0 requires the multi-
// byte integers to be encoded little-endian. In Gocene's store
// package some DataOutput implementations (specifically
// ByteBuffersIndexOutput and BufferedIndexOutput) emit big-endian
// shorts/ints/longs through their WriteShort/WriteInt/WriteLong
// methods. To stay wire-compatible with the Java reference we write
// every multi-byte integer in this file by emitting its bytes
// explicitly in little-endian order via WriteByte. The matching
// IndexInput implementations already decode little-endian.
//
// Layout markers (must match BKDWriter.VERSION_* and the Lucene
// constants byte-for-byte).
const (
	docIDsContinuous  byte = 0xFE // (byte) -2
	docIDsBitSet      byte = 0xFF // (byte) -1
	docIDsDeltaBPV16  byte = 16
	docIDsBPV21       byte = 21
	docIDsBPV24       byte = 24
	docIDsBPV32       byte = 32
	docIDsLegacyDelta byte = 0
)

// BKD-format version constants used by DocIdsWriter to choose between
// the legacy scalar BPV24 layout and the newer vectorised layout that
// also enables BPV21 compression. Mirrors BKDWriter.VERSION_* in
// Lucene 10.4.0.
const (
	// BKDVersionMetaFile (=9) is the first version that splits index
	// and data into separate files.
	BKDVersionMetaFile = 9

	// BKDVersionVectorizeBPV24AndIntroduceBPV21 (=10) introduces the
	// vectorisable BPV24 encoding and the new BPV21 encoding.
	BKDVersionVectorizeBPV24AndIntroduceBPV21 = 10

	// BKDVersionCurrent is the version produced by the current writer.
	BKDVersionCurrent = BKDVersionVectorizeBPV24AndIntroduceBPV21
)

// DocIdsWriter encodes and decodes leaf-block docIDs. A single
// instance must be configured for the maximum number of docIDs per
// leaf (the scratch buffer is sized to fit that worst case) and a
// specific BKD version (chooses the encoding flavour).
//
// The instance is reused across many leaf blocks and is not safe for
// concurrent use.
type DocIdsWriter struct {
	scratch      []int32
	scratchLongs []int64
	version      int
}

// NewDocIdsWriter constructs a DocIdsWriter sized for the given
// maximum number of points per leaf block. The version selects the
// encoding flavour as in BKDWriter: pass BKDVersionMetaFile for
// backward-compatible scalar BPV24, or BKDVersionCurrent for the
// vectorised BPV24/BPV21 mix.
func NewDocIdsWriter(maxPointsInLeaf, version int) *DocIdsWriter {
	return &DocIdsWriter{
		scratch: make([]int32, maxPointsInLeaf),
		version: version,
	}
}

// WriteDocIds encodes count docIDs from docIds[start:start+count]
// using the most compact applicable layout and writes them through
// out. Mirrors DocIdsWriter.writeDocIds(int[], int, int, DataOutput)
// in Lucene 10.4.0.
func (w *DocIdsWriter) WriteDocIds(docIds []int32, start, count int, out store.DataOutput) error {
	// docs can be sorted either when all docs in a block have the
	// same value or when a segment is sorted.
	strictlySorted := true
	minID := docIds[start]
	maxID := docIds[start]
	for i := 1; i < count; i++ {
		last := docIds[start+i-1]
		current := docIds[start+i]
		if last >= current {
			strictlySorted = false
		}
		if current < minID {
			minID = current
		}
		if current > maxID {
			maxID = current
		}
	}

	min2max := int(maxID-minID) + 1
	if strictlySorted {
		if min2max == count {
			// Continuous ids, typically happens when segment is sorted.
			if err := out.WriteByte(docIDsContinuous); err != nil {
				return err
			}
			return store.WriteVInt(out, docIds[start])
		}
		if min2max <= (count << 4) {
			// Only trigger bitset optimization when max - min + 1 <= 16 * count
			// in order to avoid expanding too much storage. A field with
			// lower cardinality will have higher probability to trigger
			// this optimisation.
			if err := out.WriteByte(docIDsBitSet); err != nil {
				return err
			}
			return writeIdsAsBitSet(docIds, start, count, out)
		}
	}

	if min2max <= 0xFFFF {
		if err := out.WriteByte(docIDsDeltaBPV16); err != nil {
			return err
		}
		for i := 0; i < count; i++ {
			w.scratch[i] = docIds[start+i] - minID
		}
		if err := store.WriteVInt(out, minID); err != nil {
			return err
		}
		halfLen := count >> 1
		for i := 0; i < halfLen; i++ {
			w.scratch[i] = w.scratch[halfLen+i] | (w.scratch[i] << 16)
		}
		for i := 0; i < halfLen; i++ {
			if err := writeIntLE(out, w.scratch[i]); err != nil {
				return err
			}
		}
		if (count & 1) == 1 {
			if err := writeShortLE(out, int16(w.scratch[count-1])); err != nil {
				return err
			}
		}
		return nil
	}

	if maxID <= 0x1FFFFF && w.version >= BKDVersionVectorizeBPV24AndIntroduceBPV21 {
		if err := out.WriteByte(docIDsBPV21); err != nil {
			return err
		}
		oneThird := floorToMultipleOf16(count / 3)
		numInts := oneThird * 2
		for i := 0; i < numInts; i++ {
			w.scratch[i] = docIds[i+start] << 11
		}
		for i := 0; i < oneThird; i++ {
			longIdx := i + numInts + start
			w.scratch[i] |= docIds[longIdx] & 0x7FF
			w.scratch[i+oneThird] |= (int32(uint32(docIds[longIdx]) >> 11)) & 0x7FF
		}
		for i := 0; i < numInts; i++ {
			if err := writeIntLE(out, w.scratch[i]); err != nil {
				return err
			}
		}
		i := oneThird * 3
		for ; i < count-2; i += 3 {
			packed := int64(docIds[start+i]) |
				(int64(docIds[start+i+1]) << 21) |
				(int64(docIds[start+i+2]) << 42)
			if err := writeLongLE(out, packed); err != nil {
				return err
			}
		}
		for ; i < count; i++ {
			if err := writeShortLE(out, int16(docIds[start+i])); err != nil {
				return err
			}
			if err := out.WriteByte(byte(uint32(docIds[start+i]) >> 16)); err != nil {
				return err
			}
		}
		return nil
	}

	if maxID <= 0xFFFFFF {
		if err := out.WriteByte(docIDsBPV24); err != nil {
			return err
		}
		if w.version < BKDVersionVectorizeBPV24AndIntroduceBPV21 {
			return writeScalarInts24(docIds, start, count, out)
		}
		// Vectorisable BPV24 layout.
		quarter := count >> 2
		numInts := quarter * 3
		for i := 0; i < numInts; i++ {
			w.scratch[i] = docIds[i+start] << 8
		}
		for i := 0; i < quarter; i++ {
			longIdx := i + numInts + start
			w.scratch[i] |= docIds[longIdx] & 0xFF
			w.scratch[i+quarter] |= int32(uint32(docIds[longIdx])>>8) & 0xFF
			w.scratch[i+quarter*2] |= int32(uint32(docIds[longIdx]) >> 16)
		}
		for i := 0; i < numInts; i++ {
			if err := writeIntLE(out, w.scratch[i]); err != nil {
				return err
			}
		}
		for i := quarter << 2; i < count; i++ {
			if err := writeShortLE(out, int16(docIds[start+i])); err != nil {
				return err
			}
			if err := out.WriteByte(byte(uint32(docIds[start+i]) >> 16)); err != nil {
				return err
			}
		}
		return nil
	}

	// Full 32-bit fallback.
	if err := out.WriteByte(docIDsBPV32); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if err := writeIntLE(out, docIds[start+i]); err != nil {
			return err
		}
	}
	return nil
}

// writeScalarInts24 emits count docIDs assuming each fits in 24 bits,
// using the legacy unvectorisable layout of three int64 words per
// eight docIDs followed by a 3-byte tail for the remainder. Mirrors
// DocIdsWriter.writeScalarInts24 in Lucene 10.4.0.
func writeScalarInts24(docIds []int32, start, count int, out store.DataOutput) error {
	i := 0
	for ; i < count-7; i += 8 {
		doc1 := int64(docIds[start+i]) & 0xFFFFFF
		doc2 := int64(docIds[start+i+1]) & 0xFFFFFF
		doc3 := int64(docIds[start+i+2]) & 0xFFFFFF
		doc4 := int64(docIds[start+i+3]) & 0xFFFFFF
		doc5 := int64(docIds[start+i+4]) & 0xFFFFFF
		doc6 := int64(docIds[start+i+5]) & 0xFFFFFF
		doc7 := int64(docIds[start+i+6]) & 0xFFFFFF
		doc8 := int64(docIds[start+i+7]) & 0xFFFFFF

		l1 := (doc1 << 40) | (doc2 << 16) | ((doc3 >> 8) & 0xFFFF)
		// Java uses `(doc6 >> 16) & 0xff` where doc6 is the full int,
		// matching arithmetic shift on int. With our masked value
		// guaranteed in [0, 2^24) the result is the same and avoids
		// surprises with negative ids.
		l2 := ((doc3 & 0xFF) << 56) |
			(doc4 << 32) |
			(doc5 << 8) |
			((doc6 >> 16) & 0xFF)
		l3 := ((doc6 & 0xFFFF) << 48) | (doc7 << 24) | doc8

		if err := writeLongLE(out, l1); err != nil {
			return err
		}
		if err := writeLongLE(out, l2); err != nil {
			return err
		}
		if err := writeLongLE(out, l3); err != nil {
			return err
		}
	}
	for ; i < count; i++ {
		if err := writeShortLE(out, int16(uint32(docIds[start+i])>>8)); err != nil {
			return err
		}
		if err := out.WriteByte(byte(docIds[start+i])); err != nil {
			return err
		}
	}
	return nil
}

// writeIdsAsBitSet encodes the docIDs from start to start+count as a
// bitset spanning [min..max], written as offsetWords (VInt) +
// totalWordCount (VInt) + totalWordCount little-endian 64-bit words.
// Mirrors DocIdsWriter.writeIdsAsBitSet in Lucene 10.4.0.
func writeIdsAsBitSet(docIds []int32, start, count int, out store.DataOutput) error {
	minID := docIds[start]
	maxID := docIds[start+count-1]

	offsetWords := int(uint32(minID) >> 6) // signed >> 6 is safe for non-negative ids; Lucene uses signed >>
	offsetBits := offsetWords << 6
	totalWordCount := util.Bits2Words(int64(maxID-int32(offsetBits)) + 1)
	var currentWord int64
	currentWordIndex := 0

	if err := store.WriteVInt(out, int32(offsetWords)); err != nil {
		return err
	}
	if err := store.WriteVInt(out, int32(totalWordCount)); err != nil {
		return err
	}
	// Build bit set streaming.
	for i := 0; i < count; i++ {
		index := int(docIds[start+i]) - offsetBits
		nextWordIndex := index >> 6
		if currentWordIndex > nextWordIndex {
			return fmt.Errorf("bkd: docIds not sorted at index %d", i)
		}
		if currentWordIndex < nextWordIndex {
			if err := writeLongLE(out, currentWord); err != nil {
				return err
			}
			currentWord = 0
			currentWordIndex++
			for currentWordIndex < nextWordIndex {
				currentWordIndex++
				if err := writeLongLE(out, 0); err != nil {
					return err
				}
			}
		}
		// Java's `1L << index` applies index & 63 implicitly for long
		// shifts; Go does not, so we mask explicitly to stay
		// byte-compatible.
		currentWord |= int64(1) << (uint(index) & 63)
	}
	if err := writeLongLE(out, currentWord); err != nil {
		return err
	}
	if currentWordIndex+1 != totalWordCount {
		return fmt.Errorf("bkd: bitset short-write: wrote %d words, expected %d", currentWordIndex+1, totalWordCount)
	}
	return nil
}

// ReadInts decodes count docIDs from in into docIDs[0:count].
// Mirrors DocIdsWriter.readInts(IndexInput, int, int[]).
func (w *DocIdsWriter) ReadInts(in store.IndexInput, count int, docIDs []int32) error {
	bpv, err := in.ReadByte()
	if err != nil {
		return err
	}
	switch bpv {
	case docIDsContinuous:
		return readContinuousIds(in, count, docIDs)
	case docIDsBitSet:
		return w.readBitSet(in, count, docIDs)
	case docIDsDeltaBPV16:
		return readDelta16(in, count, docIDs)
	case docIDsBPV21:
		return w.readInts21(in, count, docIDs)
	case docIDsBPV24:
		if w.version < BKDVersionVectorizeBPV24AndIntroduceBPV21 {
			return readScalarInts24(in, count, docIDs)
		}
		return w.readInts24(in, count, docIDs)
	case docIDsBPV32:
		return readInts32(in, count, docIDs)
	case docIDsLegacyDelta:
		return readLegacyDeltaVInts(in, count, docIDs)
	default:
		return fmt.Errorf("bkd: unsupported number of bits per value: %d", bpv)
	}
}

// ReadIntsVisitor decodes count docIDs from in and feeds them
// individually to visitor. The buffer is used as the scratch area for
// the modes that decode in bulk before visiting (BPV21 and BPV24);
// callers should pass a slice of length >= count to avoid
// reallocation. Mirrors DocIdsWriter.readInts(IndexInput, int,
// IntersectVisitor, int[]) but lifts the IntersectVisitor signature
// from the codecs package to keep the BKD utility free of cyclic
// dependencies on heavier types.
//
// The visitor type is intentionally a minimal contract: only the
// per-doc Visit hook is invoked here. Higher-level callers wrap a
// codec IntersectVisitor that implements Visit(int) error.
func (w *DocIdsWriter) ReadIntsVisitor(in store.IndexInput, count int, visitor DocIDVisitor, buffer []int32) error {
	bpv, err := in.ReadByte()
	if err != nil {
		return err
	}
	switch bpv {
	case docIDsContinuous:
		return readContinuousIdsVisitor(in, count, visitor)
	case docIDsBitSet:
		return w.readBitSetVisitor(in, count, visitor)
	case docIDsDeltaBPV16:
		return w.readDelta16Visitor(in, count, visitor)
	case docIDsBPV21:
		return w.readInts21Visitor(in, count, visitor, buffer)
	case docIDsBPV24:
		if w.version < BKDVersionVectorizeBPV24AndIntroduceBPV21 {
			return w.readScalarInts24Visitor(in, count, visitor)
		}
		return w.readInts24Visitor(in, count, visitor, buffer)
	case docIDsBPV32:
		return w.readInts32Visitor(in, count, visitor)
	case docIDsLegacyDelta:
		return readLegacyDeltaVIntsVisitor(in, count, visitor)
	default:
		return fmt.Errorf("bkd: unsupported number of bits per value: %d", bpv)
	}
}

// DocIDVisitor is the narrow visitor surface used by ReadIntsVisitor.
// Concrete IntersectVisitor implementations in higher layers satisfy
// this contract by exposing the per-doc Visit method.
type DocIDVisitor interface {
	// Visit is called for each decoded doc ID.
	Visit(docID int) error
}

// readContinuousIds decodes a CONTINUOUS_IDS block: a single VInt
// starting docID, followed by count-1 implicit increments.
func readContinuousIds(in store.IndexInput, count int, docIDs []int32) error {
	startVal, err := store.ReadVInt(in)
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		docIDs[i] = startVal + int32(i)
	}
	return nil
}

func readContinuousIdsVisitor(in store.IndexInput, count int, visitor DocIDVisitor) error {
	startVal, err := store.ReadVInt(in)
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if err := visitor.Visit(int(startVal) + i); err != nil {
			return err
		}
	}
	return nil
}

// readLegacyDeltaVInts decodes the pre-VINT16 legacy format kept for
// backward compatibility with old segments.
func readLegacyDeltaVInts(in store.IndexInput, count int, docIDs []int32) error {
	var doc int32
	for i := 0; i < count; i++ {
		delta, err := store.ReadVInt(in)
		if err != nil {
			return err
		}
		doc += delta
		docIDs[i] = doc
	}
	return nil
}

func readLegacyDeltaVIntsVisitor(in store.IndexInput, count int, visitor DocIDVisitor) error {
	var doc int32
	for i := 0; i < count; i++ {
		delta, err := store.ReadVInt(in)
		if err != nil {
			return err
		}
		doc += delta
		if err := visitor.Visit(int(doc)); err != nil {
			return err
		}
	}
	return nil
}

// readBitSet reads a BITSET_IDS block (offsetWords + length + LE
// uint64 words) and materialises the set bits as docIDs.
func (w *DocIdsWriter) readBitSet(in store.IndexInput, count int, docIDs []int32) error {
	iter, err := w.readBitSetIterator(in, count)
	if err != nil {
		return err
	}
	pos := 0
	for {
		docID, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if docID == util.NO_MORE_DOCS {
			break
		}
		docIDs[pos] = int32(docID)
		pos++
	}
	if pos != count {
		return fmt.Errorf("bkd: bitset decode produced %d docs, expected %d", pos, count)
	}
	return nil
}

func (w *DocIdsWriter) readBitSetVisitor(in store.IndexInput, count int, visitor DocIDVisitor) error {
	iter, err := w.readBitSetIterator(in, count)
	if err != nil {
		return err
	}
	for {
		docID, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if docID == util.NO_MORE_DOCS {
			return nil
		}
		if err := visitor.Visit(docID); err != nil {
			return err
		}
	}
}

// readBitSetIterator materialises the bitset payload into a
// FixedBitSet wrapped in a DocBaseBitSetIterator. The scratchLongs
// buffer is grown without copying to amortise allocation across many
// leaf blocks (the previously-loaded values are not reused).
func (w *DocIdsWriter) readBitSetIterator(in store.IndexInput, count int) (*util.DocBaseBitSetIterator, error) {
	offsetWords, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	longLen, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	if longLen < 0 {
		return nil, fmt.Errorf("bkd: bitset longLen negative: %d", longLen)
	}
	if cap(w.scratchLongs) < int(longLen) {
		w.scratchLongs = make([]int64, longLen)
	} else {
		w.scratchLongs = w.scratchLongs[:longLen]
		// Make ghost bits clear for FixedBitSet — match Lucene's
		// Arrays.fill(scratchLongs.longs, longLen, scratchLongs.longs.length, 0).
		// In Go we never expose stale capacity below len, so this is a
		// no-op; the slice is reused with the correct logical length.
	}
	for i := 0; i < int(longLen); i++ {
		v, err := in.ReadLong()
		if err != nil {
			return nil, err
		}
		w.scratchLongs[i] = v
	}
	// Reinterpret the int64 slice as []uint64 for FixedBitSet without
	// copying. The two share the same underlying memory and have the
	// same element size.
	bits := int64SliceAsUint64(w.scratchLongs)
	fbs, err := util.NewFixedBitSetOfBits(bits, len(bits)<<6)
	if err != nil {
		return nil, err
	}
	return util.NewDocBaseBitSetIterator(fbs, int64(count), int(offsetWords)<<6)
}

// int64SliceAsUint64 reinterprets a []int64 as []uint64 by element
// copy. We deliberately copy rather than alias via unsafe to keep the
// port free of unsafe pointer reinterpretation; the slice is at most
// a few KiB in practice (one entry per 64 doc IDs in the leaf).
func int64SliceAsUint64(in []int64) []uint64 {
	out := make([]uint64, len(in))
	for i, v := range in {
		out[i] = uint64(v)
	}
	return out
}

// readDelta16 decodes the DELTA_BPV_16 block: VInt min, count/2 ints
// holding two packed 16-bit deltas each, optional trailing short.
func readDelta16(in store.IndexInput, count int, docIds []int32) error {
	minVal, err := store.ReadVInt(in)
	if err != nil {
		return err
	}
	half := count >> 1
	for i := 0; i < half; i++ {
		v, err := in.ReadInt()
		if err != nil {
			return err
		}
		docIds[i] = v
	}
	decode16(docIds, half, minVal)
	// Read the remaining doc if count is odd.
	for i := half << 1; i < count; i++ {
		s, err := in.ReadShort()
		if err != nil {
			return err
		}
		docIds[i] = int32(uint16(s)) + minVal
	}
	return nil
}

func (w *DocIdsWriter) readDelta16Visitor(in store.IndexInput, count int, visitor DocIDVisitor) error {
	if err := readDelta16(in, count, w.scratch); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if err := visitor.Visit(int(w.scratch[i])); err != nil {
			return err
		}
	}
	return nil
}

// decode16 unpacks half packed-pair ints into the two halves of docIDs.
func decode16(docIDs []int32, half int, minVal int32) {
	for i := 0; i < half; i++ {
		l := docIDs[i]
		docIDs[i] = int32(uint32(l)>>16) + minVal
		docIDs[i+half] = (l & 0xFFFF) + minVal
	}
}

// floorToMultipleOf16 returns the largest multiple of 16 not greater
// than n. Mirrors the helper of the same name in DocIdsWriter.
func floorToMultipleOf16(n int) int {
	return n & ^15
}

// readInts21 decodes a BPV_21 block.
func (w *DocIdsWriter) readInts21(in store.IndexInput, count int, docIDs []int32) error {
	oneThird := floorToMultipleOf16(count / 3)
	numInts := oneThird << 1
	for i := 0; i < numInts; i++ {
		v, err := in.ReadInt()
		if err != nil {
			return err
		}
		w.scratch[i] = v
	}
	decode21(docIDs, w.scratch, oneThird, numInts)
	i := oneThird * 3
	for ; i < count-2; i += 3 {
		l, err := in.ReadLong()
		if err != nil {
			return err
		}
		ul := uint64(l)
		docIDs[i] = int32(ul & 0x1FFFFF)
		docIDs[i+1] = int32((ul >> 21) & 0x1FFFFF)
		docIDs[i+2] = int32(ul >> 42)
	}
	for ; i < count; i++ {
		s, err := in.ReadShort()
		if err != nil {
			return err
		}
		b, err := in.ReadByte()
		if err != nil {
			return err
		}
		docIDs[i] = int32(uint16(s)) | int32(b)<<16
	}
	return nil
}

func (w *DocIdsWriter) readInts21Visitor(in store.IndexInput, count int, visitor DocIDVisitor, buffer []int32) error {
	if cap(buffer) < count {
		buffer = make([]int32, count)
	} else {
		buffer = buffer[:count]
	}
	if err := w.readInts21(in, count, buffer); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if err := visitor.Visit(int(buffer[i])); err != nil {
			return err
		}
	}
	return nil
}

func decode21(docIds, scratch []int32, oneThird, numInts int) {
	for i := 0; i < numInts; i++ {
		docIds[i] = int32(uint32(scratch[i]) >> 11)
	}
	for i := 0; i < oneThird; i++ {
		docIds[i+numInts] = (scratch[i] & 0x7FF) | ((scratch[i+oneThird] & 0x7FF) << 11)
	}
}

// readInts24 decodes a BPV_24 block in the vectorisable layout.
func (w *DocIdsWriter) readInts24(in store.IndexInput, count int, docIDs []int32) error {
	quarter := count >> 2
	numInts := quarter * 3
	for i := 0; i < numInts; i++ {
		v, err := in.ReadInt()
		if err != nil {
			return err
		}
		w.scratch[i] = v
	}
	decode24(docIDs, w.scratch, quarter, numInts)
	// Now read the remaining 0, 1, 2 or 3 values.
	for i := quarter << 2; i < count; i++ {
		s, err := in.ReadShort()
		if err != nil {
			return err
		}
		b, err := in.ReadByte()
		if err != nil {
			return err
		}
		docIDs[i] = int32(uint16(s)) | int32(b)<<16
	}
	return nil
}

func (w *DocIdsWriter) readInts24Visitor(in store.IndexInput, count int, visitor DocIDVisitor, buffer []int32) error {
	if cap(buffer) < count {
		buffer = make([]int32, count)
	} else {
		buffer = buffer[:count]
	}
	if err := w.readInts24(in, count, buffer); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if err := visitor.Visit(int(buffer[i])); err != nil {
			return err
		}
	}
	return nil
}

func decode24(docIDs, scratch []int32, quarter, numInts int) {
	for i := 0; i < numInts; i++ {
		docIDs[i] = int32(uint32(scratch[i]) >> 8)
	}
	for i := 0; i < quarter; i++ {
		docIDs[i+numInts] =
			(scratch[i] & 0xFF) |
				((scratch[i+quarter] & 0xFF) << 8) |
				((scratch[i+quarter*2] & 0xFF) << 16)
	}
}

// readScalarInts24 decodes a BPV_24 block in the legacy unvectorised
// layout (pre-VERSION_VECTORIZE_BPV24_AND_INTRODUCE_BPV21).
func readScalarInts24(in store.IndexInput, count int, docIDs []int32) error {
	i := 0
	for ; i < count-7; i += 8 {
		l1, err := in.ReadLong()
		if err != nil {
			return err
		}
		l2, err := in.ReadLong()
		if err != nil {
			return err
		}
		l3, err := in.ReadLong()
		if err != nil {
			return err
		}
		ul1, ul2, ul3 := uint64(l1), uint64(l2), uint64(l3)
		docIDs[i] = int32(ul1 >> 40)
		docIDs[i+1] = int32(ul1>>16) & 0xFFFFFF
		docIDs[i+2] = int32(((ul1 & 0xFFFF) << 8) | (ul2 >> 56))
		docIDs[i+3] = int32(ul2>>32) & 0xFFFFFF
		docIDs[i+4] = int32(ul2>>8) & 0xFFFFFF
		docIDs[i+5] = int32(((ul2 & 0xFF) << 16) | (ul3 >> 48))
		docIDs[i+6] = int32(ul3>>24) & 0xFFFFFF
		docIDs[i+7] = int32(ul3) & 0xFFFFFF
	}
	for ; i < count; i++ {
		s, err := in.ReadShort()
		if err != nil {
			return err
		}
		b, err := in.ReadByte()
		if err != nil {
			return err
		}
		docIDs[i] = (int32(uint16(s)) << 8) | int32(b)
	}
	return nil
}

func (w *DocIdsWriter) readScalarInts24Visitor(in store.IndexInput, count int, visitor DocIDVisitor) error {
	if err := readScalarInts24(in, count, w.scratch); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if err := visitor.Visit(int(w.scratch[i])); err != nil {
			return err
		}
	}
	return nil
}

// readInts32 decodes a BPV_32 block.
func readInts32(in store.IndexInput, count int, docIDs []int32) error {
	for i := 0; i < count; i++ {
		v, err := in.ReadInt()
		if err != nil {
			return err
		}
		docIDs[i] = v
	}
	return nil
}

func (w *DocIdsWriter) readInts32Visitor(in store.IndexInput, count int, visitor DocIDVisitor) error {
	for i := 0; i < count; i++ {
		v, err := in.ReadInt()
		if err != nil {
			return err
		}
		w.scratch[i] = v
	}
	for i := 0; i < count; i++ {
		if err := visitor.Visit(int(w.scratch[i])); err != nil {
			return err
		}
	}
	return nil
}

// writeShortLE writes v as a little-endian 16-bit value via raw bytes.
// We bypass DataOutput.WriteShort because some buffered
// implementations in the store package emit big-endian shorts; for
// wire compatibility with Lucene 10.4.0 the BKD docID encodings MUST
// be little-endian.
func writeShortLE(out store.DataOutput, v int16) error {
	uv := uint16(v)
	if err := out.WriteByte(byte(uv)); err != nil {
		return err
	}
	return out.WriteByte(byte(uv >> 8))
}

// writeIntLE writes v as a little-endian 32-bit value via raw bytes.
func writeIntLE(out store.DataOutput, v int32) error {
	uv := uint32(v)
	if err := out.WriteByte(byte(uv)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 8)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 16)); err != nil {
		return err
	}
	return out.WriteByte(byte(uv >> 24))
}

// writeLongLE writes v as a little-endian 64-bit value via raw bytes.
func writeLongLE(out store.DataOutput, v int64) error {
	uv := uint64(v)
	if err := out.WriteByte(byte(uv)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 8)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 16)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 24)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 32)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 40)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 48)); err != nil {
		return err
	}
	return out.WriteByte(byte(uv >> 56))
}
